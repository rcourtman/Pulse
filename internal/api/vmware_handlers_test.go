package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

type fakeVMwareClient struct {
	testConnection func(context.Context) (*vmware.InventorySummary, error)
}

func (c *fakeVMwareClient) TestConnection(ctx context.Context) (*vmware.InventorySummary, error) {
	if c == nil || c.testConnection == nil {
		return &vmware.InventorySummary{}, nil
	}
	return c.testConnection(ctx)
}

func (c *fakeVMwareClient) Close() {}

func TestVMwareHandlers_HandleAdd_Success(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)

	handler, persistence := newVMwareHandlersForTest(t)

	body := marshalVMwareRequest(t, map[string]any{
		"name":     "lab-vcenter",
		"host":     "vcsa.lab.local",
		"port":     443,
		"username": "administrator@vsphere.local",
		"password": "super-secret",
		"enabled":  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created config.VMwareVCenterInstance
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected generated ID, got empty")
	}
	if created.Password != "********" {
		t.Fatalf("expected password redacted, got %q", created.Password)
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 saved instance, got %d", len(stored))
	}
	if stored[0].Password != "super-secret" {
		t.Fatalf("expected unredacted password persisted, got %q", stored[0].Password)
	}
}

func TestVMwareHandlers_HandleAdd_ValidationAndFeatureGate(t *testing.T) {
	t.Run("missing host", func(t *testing.T) {
		setVMwareFeatureForTest(t, true)
		setMockModeForVMwareTest(t, false)
		handler, _ := newVMwareHandlersForTest(t)

		body := marshalVMwareRequest(t, map[string]any{
			"username": "administrator@vsphere.local",
			"password": "super-secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("feature disabled", func(t *testing.T) {
		setVMwareFeatureForTest(t, false)
		setMockModeForVMwareTest(t, false)
		handler, _ := newVMwareHandlersForTest(t)

		body := marshalVMwareRequest(t, map[string]any{
			"host":     "vcsa.lab.local",
			"username": "administrator@vsphere.local",
			"password": "super-secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
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

func TestVMwareHandlers_HandleList_RedactsSensitiveFieldsAndIncludesRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:                 "vc-1",
			Name:               "lab-a",
			Host:               "vcsa-a.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "secret-a",
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	recordedAt := time.Date(2026, 3, 30, 10, 11, 12, 0, time.UTC)
	poller.RecordConnectionTestSuccess("default", "vc-1", &vmware.InventorySummary{
		Hosts:      3,
		VMs:        42,
		Datastores: 6,
		VIRelease:  "8.0.3",
	}, recordedAt)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []vmwareConnectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed instance, got %d", len(listed))
	}
	if listed[0].Password != "********" {
		t.Fatalf("expected password to be redacted, got %q", listed[0].Password)
	}
	if listed[0].Poll == nil || listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved test runtime summary, got %+v", listed[0].Poll)
	}
	if listed[0].Observed == nil {
		t.Fatalf("expected observed summary, got nil")
	}
	if listed[0].Observed.Hosts != 3 || listed[0].Observed.VMs != 42 || listed[0].Observed.Datastores != 6 {
		t.Fatalf("unexpected observed counts: %+v", listed[0].Observed)
	}
	if listed[0].Observed.VIRelease != "8.0.3" {
		t.Fatalf("unexpected VI release: %+v", listed[0].Observed)
	}
}

func TestVMwareHandlers_HandleDelete_RemovesAndClearsRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)

	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{ID: "alpha", Host: "vcsa-a.lab.local", Username: "admin", Password: "a"},
		{ID: "beta", Host: "vcsa-b.lab.local", Username: "admin", Password: "b"},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}
	handler.recordTestSuccess("alpha", &vmware.InventorySummary{Hosts: 1}, time.Now().UTC())

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/vmware/connections/alpha", nil)
	deleteRec := httptest.NewRecorder()
	handler.HandleDelete(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 || stored[0].ID != "beta" {
		t.Fatalf("expected only beta to remain, got %+v", stored)
	}
	if status := handler.runtimeStatus("alpha"); status.Poll != nil || status.Observed != nil {
		t.Fatalf("expected runtime summary to be cleared, got %+v", status)
	}
}

func TestVMwareHandlers_HandleUpdate_PreservesMaskedSecretsAndReplacesFields(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)

	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:                 "alpha",
			Name:               "old-name",
			Host:               "old.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "super-secret",
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	body := marshalVMwareRequest(t, map[string]any{
		"id":                 "ignored-id",
		"name":               "new-name",
		"host":               "new.lab.local",
		"port":               8443,
		"username":           "operator@vsphere.local",
		"password":           "********",
		"insecureSkipVerify": true,
		"enabled":            true,
	})

	req := httptest.NewRequest(http.MethodPut, "/api/vmware/connections/alpha", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated config.VMwareVCenterInstance
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.ID != "alpha" {
		t.Fatalf("expected path ID to win, got %q", updated.ID)
	}
	if updated.Password != "********" {
		t.Fatalf("expected password to remain redacted, got %q", updated.Password)
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored instance, got %d", len(stored))
	}
	if stored[0].Host != "new.lab.local" || stored[0].Port != 8443 {
		t.Fatalf("expected updated endpoint to persist, got %+v", stored[0])
	}
	if stored[0].Password != "super-secret" {
		t.Fatalf("expected masked password to preserve stored secret, got %q", stored[0].Password)
	}
	if !stored[0].InsecureSkipVerify {
		t.Fatalf("expected insecureSkipVerify update to persist")
	}
}

func TestVMwareHandlers_HandleTestConnection_SuccessAndFailure(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, _ := newVMwareHandlersForTest(t)
	var gotConfig vmware.ClientConfig
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		gotConfig = cfg
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 1, VMs: 2, Datastores: 3, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	successBody := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"port":     8443,
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	successReq := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(successBody))
	successRec := httptest.NewRecorder()
	handler.HandleTestConnection(successRec, successReq)

	if successRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", successRec.Code, successRec.Body.String())
	}
	if gotConfig.Host != "vcsa.lab.local" || gotConfig.Port != 8443 {
		t.Fatalf("unexpected client config: %+v", gotConfig)
	}

	handler.newClient = nil
	failureBody := marshalVMwareRequest(t, map[string]any{
		"host":     "http://127.0.0.1/path",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	failureReq := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(failureBody))
	failureRec := httptest.NewRecorder()
	handler.HandleTestConnection(failureRec, failureReq)

	if failureRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad host, got %d: %s", failureRec.Code, failureRec.Body.String())
	}

	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return nil, errors.New("boom")
			},
		}, nil
	}
	errorBody := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	errorReq := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(errorBody))
	errorRec := httptest.NewRecorder()
	handler.HandleTestConnection(errorRec, errorReq)

	if errorRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for failing connection, got %d: %s", errorRec.Code, errorRec.Body.String())
	}
}

func TestVMwareHandlers_HandleTestConnection_PreservesUnsupportedVersionCategory(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, _ := newVMwareHandlersForTest(t)
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return nil, &vmware.ConnectionError{
					Category: "unsupported_version",
					Message:  "VMware vCenter version is outside the implemented VI JSON probe floor; Pulse currently probes 9.0.0.0, 8.0.3, 8.0.2.0, 8.0.1.0",
				}
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTestConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Code != "vmware_connection_failed" {
		t.Fatalf("unexpected code: %+v", payload)
	}
	if payload.Details["category"] != "unsupported_version" {
		t.Fatalf("expected unsupported_version category, got %+v", payload.Details)
	}
}

func TestVMwareHandlers_HandleTestSavedConnection_UsesStoredSecretsAndUpdatesRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:                 "conn-1",
		Name:               "lab-vcenter",
		Host:               "vcsa.lab.local",
		Port:               443,
		Username:           "administrator@vsphere.local",
		Password:           "super-secret",
		InsecureSkipVerify: false,
		Enabled:            true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	var gotConfig vmware.ClientConfig
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		gotConfig = cfg
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 4, VMs: 25, Datastores: 5, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotConfig.Host != "vcsa.lab.local" || gotConfig.Password != "super-secret" {
		t.Fatalf("unexpected saved client config: %+v", gotConfig)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	listRec := httptest.NewRecorder()
	handler.HandleList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listed []vmwareConnectionResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 || listed[0].Poll == nil || listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved retest to update runtime status, got %+v", listed)
	}
	if listed[0].Observed == nil || listed[0].Observed.VMs != 25 {
		t.Fatalf("expected saved retest to update observed summary, got %+v", listed[0].Observed)
	}
}

func TestVMwareHandlers_HandleTestSavedConnection_UpdatesRuntimeSummaryFailure(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Host:     "vcsa.lab.local",
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return nil, &vmware.ConnectionError{Category: "auth", Message: "authentication failed"}
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastError == nil {
		t.Fatalf("expected saved retest failure to update runtime summary, got %+v", summary.Poll)
	}
	if summary.Poll.LastError.Message != "authentication failed" || summary.Poll.LastError.Category != "auth" {
		t.Fatalf("unexpected failure summary: %+v", summary.Poll.LastError)
	}
}

func TestVMwareHandlers_HandleTestSavedConnection_MergesEditedPayloadWithoutOverwritingRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:                 "conn-1",
			Name:               "lab-vcenter",
			Host:               "vcsa.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "super-secret",
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	previousAt := time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)
	poller.RecordConnectionTestSuccess("default", "conn-1", &vmware.InventorySummary{Hosts: 1, VMs: 2, Datastores: 3, VIRelease: "8.0.2.0"}, previousAt)

	var gotConfig vmware.ClientConfig
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		gotConfig = cfg
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 9, VMs: 99, Datastores: 12, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"name":               "edited-vcenter",
		"host":               "edited.lab.local",
		"port":               8443,
		"username":           "operator@vsphere.local",
		"password":           "********",
		"insecureSkipVerify": true,
		"enabled":            true,
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/vmware/connections/conn-1/test",
		bytes.NewReader(body),
	)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotConfig.Host != "edited.lab.local" || gotConfig.Port != 8443 {
		t.Fatalf("expected edited endpoint, got %+v", gotConfig)
	}
	if gotConfig.Password != "super-secret" {
		t.Fatalf("expected stored password to be reused, got %+v", gotConfig)
	}

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{{
		ID:                 "conn-1",
		Name:               "lab-vcenter",
		Host:               "vcsa.lab.local",
		Port:               443,
		Username:           "administrator@vsphere.local",
		Password:           "super-secret",
		InsecureSkipVerify: false,
		Enabled:            true,
	}})["conn-1"]
	if summary.Observed == nil || summary.Observed.VMs != 2 || summary.Observed.VIRelease != "8.0.2.0" {
		t.Fatalf("expected existing runtime summary to remain unchanged, got %+v", summary.Observed)
	}
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil || !summary.Poll.LastSuccessAt.Equal(previousAt) {
		t.Fatalf("expected existing last success timestamp to remain unchanged, got %+v", summary.Poll)
	}
}

func newVMwareHandlersForTest(t *testing.T) (*VMwareHandlers, *config.ConfigPersistence) {
	t.Helper()

	persistence := config.NewConfigPersistence(t.TempDir())
	handler := &VMwareHandlers{
		getPersistence: func(context.Context) *config.ConfigPersistence { return persistence },
	}

	return handler, persistence
}

func setVMwareFeatureForTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(enabled)
	t.Cleanup(func() {
		vmware.SetFeatureEnabled(previous)
	})
}

func setMockModeForVMwareTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := mock.IsMockEnabled()
	mock.SetEnabled(enabled)
	t.Cleanup(func() {
		mock.SetEnabled(previous)
	})
}

func marshalVMwareRequest(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return body
}
