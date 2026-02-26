package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
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
	})
}

// TestTrueNASHandlers_HandleAdd_NoLimitForTrueNAS verifies that TrueNAS
// connection additions are never blocked by the agent limit (agents-only model).
func TestTrueNASHandlers_HandleAdd_NoLimitForTrueNAS(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setMockModeForTrueNASTest(t, false)
	setMaxAgentsLicenseForTests(t, 1)

	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
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

	// Under agents-only model, TrueNAS connections are not limited.
	if rec.Code == http.StatusPaymentRequired {
		t.Fatalf("TrueNAS add should not be blocked by agent limit, got 402")
	}

	stored, err := persistence.LoadTrueNASConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected both TrueNAS instances to be saved, got %d", len(stored))
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

func newTrueNASHandlersForTest(t *testing.T, cfg *config.Config) (*TrueNASHandlers, *config.ConfigPersistence, *monitoring.Monitor) {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}

	persistence := config.NewConfigPersistence(cfg.DataPath)
	var mon *monitoring.Monitor

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
