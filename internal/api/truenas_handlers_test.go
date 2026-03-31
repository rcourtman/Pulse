package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type fakeTrueNASClient struct {
	testConnection func(context.Context) error
}

func (c *fakeTrueNASClient) TestConnection(ctx context.Context) error {
	if c == nil || c.testConnection == nil {
		return nil
	}
	return c.testConnection(ctx)
}

func (c *fakeTrueNASClient) Close() {}

func TestTrueNASHandlers_HandleAdd_Success(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, false)

	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)

	body := marshalTrueNASRequest(t, map[string]any{
		"name":   "nas",
		"host":   "nas.local",
		"apiKey": "super-secret",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created config.TrueNASInstance
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected generated ID, got empty")
	}
	if created.APIKey != "********" {
		t.Fatalf("expected API key redacted, got %q", created.APIKey)
	}

	stored, err := persistence.LoadTrueNASConfig()
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 saved instance, got %d", len(stored))
	}
	if stored[0].APIKey != "super-secret" {
		t.Fatalf("expected unredacted API key persisted, got %q", stored[0].APIKey)
	}
}

func TestTrueNASHandlers_HandleAdd_ValidationAndFeatureGate(t *testing.T) {
	t.Run("missing host", func(t *testing.T) {
		setTrueNASFeatureForTest(t, true)
		setMockModeForTrueNASTest(t, false)
		handler, _, _ := newTrueNASHandlersForTest(t, nil)

		body := marshalTrueNASRequest(t, map[string]any{
			"apiKey": "token",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("feature disabled", func(t *testing.T) {
		setTrueNASFeatureForTest(t, false)
		setMockModeForTrueNASTest(t, false)
		handler, _, _ := newTrueNASHandlersForTest(t, nil)

		body := marshalTrueNASRequest(t, map[string]any{
			"host":   "nas.local",
			"apiKey": "token",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "explicitly disabled") {
			t.Fatalf("expected explicit disable message, got %s", rec.Body.String())
		}
	})
}

func TestTrueNASHandlers_HandleAdd_BlocksNewCountedSystemAtLimit(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, false)
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	handler, persistence, monitor := newTrueNASHandlersForTest(t, nil)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "existing-host",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "existing-host",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"existing-host"},
				},
			},
		},
	})
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{
		{ID: "existing", Host: "nas-a.local", APIKey: "a"},
	}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}

	body := marshalTrueNASRequest(t, map[string]any{
		"host":   "nas-b.local",
		"apiKey": "b",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 once monitored-system cap is full, got %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := persistence.LoadTrueNASConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected blocked TrueNAS add not to persist, got %d instances", len(stored))
	}
}

func TestTrueNASHandlers_HandleList_RedactsSensitiveFields(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{
		{
			ID:       "a",
			Name:     "api-key-auth",
			Host:     "nas-a.local",
			APIKey:   "key-a",
			UseHTTPS: true,
			Enabled:  true,
		},
		{
			ID:       "b",
			Name:     "password-auth",
			Host:     "nas-b.local",
			Username: "admin",
			Password: "pw-b",
			UseHTTPS: true,
			Enabled:  true,
		},
	}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []config.TrueNASInstance
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 listed instances, got %d", len(listed))
	}
	if listed[0].APIKey != "********" {
		t.Fatalf("expected API key to be redacted, got %q", listed[0].APIKey)
	}
	if listed[1].Password != "********" {
		t.Fatalf("expected password to be redacted, got %q", listed[1].Password)
	}
}

func TestTrueNASHandlers_HandleList_ReturnsMockConnectionsInMockMode(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, true)

	handler, _, _ := newTrueNASHandlersForTest(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []trueNASConnectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 mock TrueNAS connection, got %d", len(listed))
	}
	if listed[0].Host != "truenas-main" {
		t.Fatalf("expected mock TrueNAS host truenas-main, got %q", listed[0].Host)
	}
	if listed[0].Poll == nil || listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected mock poll summary, got %+v", listed[0].Poll)
	}
	if listed[0].Observed == nil || listed[0].Observed.Systems != 1 || listed[0].Observed.StoragePools == 0 {
		t.Fatalf("expected populated mock observed summary, got %+v", listed[0].Observed)
	}
}

func TestTrueNASHandlers_HandleList_IncludesPollAndObservedSummary(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	cfg := &config.Config{DataPath: t.TempDir()}
	handler, persistence, _ := newTrueNASHandlersForTest(t, cfg)
	multiTenant := config.NewMultiTenantPersistence(cfg.DataPath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			_, _ = w.Write([]byte(`{"hostname":"tower","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER-tower"}`))
		case "/api/v2.0/pool":
			_, _ = w.Write([]byte(`[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`))
		case "/api/v2.0/pool/dataset":
			_, _ = w.Write([]byte(`[{"id":"tank/apps","name":"tank/apps","pool":"tank","used":{"rawvalue":"12345","parsed":12345},"available":{"rawvalue":"555","parsed":555},"mountpoint":"/mnt/tank/apps","readonly":{"rawvalue":"off","parsed":false},"mounted":true}]`))
		case "/api/v2.0/disk":
			_, _ = w.Write([]byte(`[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"}]`))
		case "/api/v2.0/alert/list":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	connection := trueNASInstanceFromRawURL(t, "tower-1", server.URL, true)
	connection.PollIntervalSecs = 60
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}

	poller := monitoring.NewTrueNASPoller(multiTenant, 50*time.Millisecond, nil)
	poller.Start(context.Background())
	t.Cleanup(poller.Stop)
	handler.getPoller = func(context.Context) *monitoring.TrueNASPoller { return poller }

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		summaries := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})
		if summary, ok := summaries[connection.ID]; ok && summary.Poll != nil && summary.Poll.LastSuccessAt != nil && summary.Observed != nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []trueNASConnectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed instance, got %d", len(listed))
	}
	if listed[0].Poll == nil {
		t.Fatalf("expected poll summary, got nil")
	}
	if listed[0].Poll.IntervalSeconds != 60 {
		t.Fatalf("expected poll interval 60, got %d", listed[0].Poll.IntervalSeconds)
	}
	if listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected last success timestamp in poll summary")
	}
	if listed[0].Observed == nil {
		t.Fatalf("expected observed summary, got nil")
	}
	if listed[0].Observed.Host != "tower" || listed[0].Observed.ResourceID != "tower" {
		t.Fatalf("unexpected observed host summary: %+v", listed[0].Observed)
	}
	if listed[0].Observed.StoragePools != 1 || listed[0].Observed.Datasets != 1 || listed[0].Observed.Disks != 1 {
		t.Fatalf("unexpected observed counts: %+v", listed[0].Observed)
	}
}

func TestTrueNASHandlers_HandleDelete_RemovesAndHandlesUnknownID(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, false)

	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{
		{ID: "alpha", Host: "nas-a.local", APIKey: "a"},
		{ID: "beta", Host: "nas-b.local", APIKey: "b"},
	}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/truenas/connections/alpha", nil)
	deleteRec := httptest.NewRecorder()
	handler.HandleDelete(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	stored, err := persistence.LoadTrueNASConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 || stored[0].ID != "beta" {
		t.Fatalf("expected only beta to remain, got %+v", stored)
	}

	missingReq := httptest.NewRequest(http.MethodDelete, "/api/truenas/connections/missing", nil)
	missingRec := httptest.NewRecorder()
	handler.HandleDelete(missingRec, missingReq)

	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing ID, got %d: %s", missingRec.Code, missingRec.Body.String())
	}
}

func TestTrueNASHandlers_HandleUpdate_PreservesMaskedSecretsAndReplacesFields(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, false)

	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{
		{
			ID:                 "alpha",
			Name:               "old-name",
			Host:               "old.local",
			Port:               443,
			APIKey:             "super-secret",
			UseHTTPS:           true,
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}

	body := marshalTrueNASRequest(t, map[string]any{
		"id":                 "ignored-id",
		"name":               "new-name",
		"host":               "new.local",
		"port":               8443,
		"apiKey":             "********",
		"useHttps":           true,
		"insecureSkipVerify": true,
		"enabled":            true,
	})

	req := httptest.NewRequest(http.MethodPut, "/api/truenas/connections/alpha", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated config.TrueNASInstance
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.ID != "alpha" {
		t.Fatalf("expected path ID to win, got %q", updated.ID)
	}
	if updated.APIKey != "********" {
		t.Fatalf("expected api key to remain redacted, got %q", updated.APIKey)
	}

	stored, err := persistence.LoadTrueNASConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored instance, got %d", len(stored))
	}
	if stored[0].Host != "new.local" || stored[0].Port != 8443 {
		t.Fatalf("expected updated endpoint to persist, got %+v", stored[0])
	}
	if stored[0].APIKey != "super-secret" {
		t.Fatalf("expected masked api key to preserve stored secret, got %q", stored[0].APIKey)
	}
	if !stored[0].InsecureSkipVerify {
		t.Fatalf("expected insecureSkipVerify update to persist")
	}
}

func TestTrueNASHandlers_HandleUpdate_UnknownID(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, false)

	handler, _, _ := newTrueNASHandlersForTest(t, nil)

	body := marshalTrueNASRequest(t, map[string]any{
		"host":   "missing.local",
		"apiKey": "secret",
	})

	req := httptest.NewRequest(http.MethodPut, "/api/truenas/connections/missing", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTrueNASHandlers_HandleTestConnection_SuccessAndFailure(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	handler, _, _ := newTrueNASHandlersForTest(t, nil)
	var gotConfig truenas.ClientConfig
	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		gotConfig = cfg
		return &fakeTrueNASClient{}, nil
	}

	successBody := marshalTrueNASRequest(t, map[string]any{
		"host":     "nas.local",
		"port":     80,
		"useHttps": false,
		"apiKey":   "key",
	})
	successReq := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/test", bytes.NewReader(successBody))
	successRec := httptest.NewRecorder()
	handler.HandleTestConnection(successRec, successReq)

	if successRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", successRec.Code, successRec.Body.String())
	}
	if gotConfig.Host != "nas.local" || gotConfig.Port != 80 || gotConfig.UseHTTPS {
		t.Fatalf("unexpected client config: %+v", gotConfig)
	}

	// For invalid hosts, rely on the real TrueNAS client constructor so we exercise the
	// same endpoint parsing and validation logic as production.
	handler.newClient = nil

	failureBody := marshalTrueNASRequest(t, map[string]any{
		"host":   "http://127.0.0.1:65536",
		"apiKey": "key",
	})
	failureReq := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/test", bytes.NewReader(failureBody))
	failureRec := httptest.NewRecorder()
	handler.HandleTestConnection(failureRec, failureReq)

	if failureRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad host, got %d: %s", failureRec.Code, failureRec.Body.String())
	}

	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		return &fakeTrueNASClient{
			testConnection: func(context.Context) error { return errors.New("boom") },
		}, nil
	}
	errorBody := marshalTrueNASRequest(t, map[string]any{
		"host":   "nas.local",
		"port":   80,
		"apiKey": "key",
	})
	errorReq := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/test", bytes.NewReader(errorBody))
	errorRec := httptest.NewRecorder()
	handler.HandleTestConnection(errorRec, errorReq)

	if errorRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for failing connection, got %d: %s", errorRec.Code, errorRec.Body.String())
	}
}

func TestTrueNASHandlers_HandleTestSavedConnection_UsesStoredSecrets(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	connection := config.TrueNASInstance{
		ID:                 "conn-1",
		Name:               "tower",
		Host:               "truenas.local",
		Port:               443,
		APIKey:             "super-secret",
		UseHTTPS:           true,
		InsecureSkipVerify: false,
		Enabled:            true,
	}
	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}
	poller := monitoring.NewTrueNASPoller(nil, time.Second, nil)
	handler.getPoller = func(context.Context) *monitoring.TrueNASPoller { return poller }

	var gotConfig truenas.ClientConfig
	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		gotConfig = cfg
		return &fakeTrueNASClient{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotConfig.Host != "truenas.local" || gotConfig.APIKey != "super-secret" || !gotConfig.UseHTTPS {
		t.Fatalf("unexpected saved client config: %+v", gotConfig)
	}
	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved retest to update poll summary success state, got %+v", summary.Poll)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/missing/test", nil)
	missingRec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(missingRec, missingReq)

	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing saved connection, got %d: %s", missingRec.Code, missingRec.Body.String())
	}
}

func TestTrueNASHandlers_HandleTestSavedConnection_UpdatesPollSummaryFailure(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	connection := config.TrueNASInstance{
		ID:                 "conn-1",
		Name:               "tower",
		Host:               "truenas.local",
		Port:               443,
		APIKey:             "super-secret",
		UseHTTPS:           true,
		InsecureSkipVerify: false,
		Enabled:            true,
	}
	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}
	poller := monitoring.NewTrueNASPoller(nil, time.Second, nil)
	handler.getPoller = func(context.Context) *monitoring.TrueNASPoller { return poller }
	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		return &fakeTrueNASClient{
			testConnection: func(context.Context) error { return errors.New("authentication failed") },
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastError == nil {
		t.Fatalf("expected saved retest failure to update poll summary, got %+v", summary.Poll)
	}
	if summary.Poll.LastError.Message != "authentication failed" {
		t.Fatalf("expected failure message preserved, got %+v", summary.Poll.LastError)
	}
}

func TestTrueNASHandlers_HandleTestSavedConnection_MergesEditedPayloadWithStoredSecrets(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{
		{
			ID:                 "conn-1",
			Name:               "tower",
			Host:               "truenas.local",
			Port:               443,
			APIKey:             "super-secret",
			UseHTTPS:           true,
			InsecureSkipVerify: false,
			Fingerprint:        "sha256:old",
			Enabled:            true,
			PollIntervalSecs:   60,
		},
	}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}

	var gotConfig truenas.ClientConfig
	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		gotConfig = cfg
		return &fakeTrueNASClient{}, nil
	}

	body := marshalTrueNASRequest(t, map[string]any{
		"name":                "Tower Edited",
		"host":                "tower-edited.local",
		"port":                8443,
		"apiKey":              "********",
		"useHttps":            true,
		"insecureSkipVerify":  true,
		"fingerprint":         "sha256:new",
		"enabled":             true,
		"pollIntervalSeconds": 120,
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/truenas/connections/conn-1/test",
		bytes.NewReader(body),
	)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotConfig.Host != "tower-edited.local" {
		t.Fatalf("expected edited host, got %+v", gotConfig)
	}
	if gotConfig.Port != 8443 {
		t.Fatalf("expected edited port, got %+v", gotConfig)
	}
	if gotConfig.APIKey != "super-secret" {
		t.Fatalf("expected stored API key to be reused, got %+v", gotConfig)
	}
	if !gotConfig.InsecureSkipVerify || gotConfig.Fingerprint != "sha256:new" {
		t.Fatalf("expected edited TLS settings, got %+v", gotConfig)
	}
}

func newTrueNASHandlersForTest(t *testing.T, cfg *config.Config) (*TrueNASHandlers, *config.ConfigPersistence, *monitoring.Monitor) {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}

	persistence := config.NewConfigPersistence(cfg.DataPath)
	mon := &monitoring.Monitor{}

	handler := &TrueNASHandlers{
		getPersistence: func(context.Context) *config.ConfigPersistence { return persistence },
		getConfig:      func(context.Context) *config.Config { return cfg },
		getMonitor:     func(context.Context) *monitoring.Monitor { return mon },
	}

	return handler, persistence, mon
}

func setTrueNASFeatureForTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := truenas.IsFeatureEnabled()
	truenas.SetFeatureEnabled(enabled)
	t.Cleanup(func() {
		truenas.SetFeatureEnabled(previous)
	})
}

func setMockModeForTrueNASTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := mock.IsMockEnabled()
	mock.SetEnabled(enabled)
	t.Cleanup(func() {
		mock.SetEnabled(previous)
	})
}

func marshalTrueNASRequest(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return body
}

func trueNASInstanceFromRawURL(t *testing.T, id string, rawURL string, enabled bool) config.TrueNASInstance {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parse port from %q: %v", rawURL, err)
	}

	return config.TrueNASInstance{
		ID:               id,
		Name:             "connection-" + id,
		Host:             parsed.Hostname(),
		Port:             port,
		APIKey:           "test-api-key",
		UseHTTPS:         strings.EqualFold(parsed.Scheme, "https"),
		Enabled:          enabled,
		PollIntervalSecs: 60,
	}
}
