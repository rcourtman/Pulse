package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func newTestMonitor(t *testing.T) (*monitoring.Monitor, *models.State, *monitoring.MetricsHistory) {
	t.Helper()

	monitor := &monitoring.Monitor{}
	state := models.NewState()
	metricsHistory := monitoring.NewMetricsHistory(10, time.Hour)

	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", metricsHistory)

	return monitor, state, metricsHistory
}

func TestHandleSchedulerHealth_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/scheduler/health", nil)
	rec := httptest.NewRecorder()

	router.handleSchedulerHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleSchedulerHealth_NoMonitor(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/health", nil)
	rec := httptest.NewRecorder()

	router.handleSchedulerHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleChangePassword_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/change-password", nil)
	rec := httptest.NewRecorder()

	router.handleChangePassword(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleLogout_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/logout", nil)
	rec := httptest.NewRecorder()

	router.handleLogout(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleLogout_Post(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rec := httptest.NewRecorder()

	router.handleLogout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := payload["success"].(bool); !ok {
		t.Fatalf("expected success=true, got %#v", payload["success"])
	}
}

func TestHandleAgentVersion_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/agent/version", nil)
	rec := httptest.NewRecorder()

	router.handleAgentVersion(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleAgentVersion_Get(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/agent/version", nil)
	rec := httptest.NewRecorder()

	router.handleAgentVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["version"] == "" {
		t.Fatalf("expected version in response, got %#v", payload)
	}
}

func TestHandleStorage_MissingID(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/storage/", nil)
	rec := httptest.NewRecorder()

	router.handleStorage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleStorage_NotFound(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/storage/store-1", nil)
	rec := httptest.NewRecorder()

	router.handleStorage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleStorage_Success(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Storage = []models.Storage{{ID: "store-1", Name: "Store One"}}
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/storage/store-1", nil)
	rec := httptest.NewRecorder()

	router.handleStorage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data field, got %#v", payload)
	}
	if data["id"] != "store-1" {
		t.Fatalf("expected storage id store-1, got %#v", data["id"])
	}
}

func TestHandleCharts_Success(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.VMs = []models.VM{{ID: "vm-1", Name: "vm-one", CPU: 0.2}}
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/charts?range=5m", nil)
	rec := httptest.NewRecorder()

	router.handleCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestHandleStorageCharts_Success(t *testing.T) {
	monitor, state, metricsHistory := newTestMonitor(t)
	state.Storage = []models.Storage{{ID: "store-1", Name: "Store One"}}
	metricsHistory.AddStorageMetric("store-1", "usage", 0.4, time.Now())

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/storage/charts?range=30", nil)
	rec := httptest.NewRecorder()

	router.handleStorageCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestEstablishSession(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	if err := router.establishSession(rec, req, "admin"); err != nil {
		t.Fatalf("establishSession error: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected session and csrf cookies, got %d", len(cookies))
	}
}

func TestEstablishOIDCSession(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	oidc := &OIDCTokenInfo{
		RefreshToken:   "refresh",
		AccessTokenExp: time.Now().Add(1 * time.Hour),
		Issuer:         "issuer",
		ClientID:       "client",
	}

	if err := router.establishOIDCSession(rec, req, "admin", oidc); err != nil {
		t.Fatalf("establishOIDCSession error: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected session and csrf cookies, got %d", len(cookies))
	}
}

func TestLearnBaselines_NoMonitor(t *testing.T) {
	router := &Router{}
	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})
	history := monitoring.NewMetricsHistory(10, time.Hour)

	router.learnBaselines(store, history)
}

func TestLearnBaselines_WithData(t *testing.T) {
	monitor, state, history := newTestMonitor(t)
	state.Nodes = []models.Node{{ID: "node-1", Name: "node"}}
	state.VMs = []models.VM{{ID: "vm-1", Name: "vm", Status: "running"}}
	state.Containers = []models.Container{{ID: "ct-1", Name: "ct", Status: "running"}}

	now := time.Now()
	history.AddNodeMetric("node-1", "cpu", 0.5, now)
	history.AddGuestMetric("vm-1", "cpu", 0.2, now)
	history.AddGuestMetric("ct-1", "cpu", 0.3, now)

	router := &Router{monitor: monitor}
	store := ai.NewBaselineStore(ai.BaselineConfig{MinSamples: 1})

	router.learnBaselines(store, history)

	if store.ResourceCount() == 0 {
		t.Fatalf("expected baselines to be learned")
	}
}

func TestWireAlertTriggeredAI_EarlyReturns(t *testing.T) {
	router := &Router{}
	router.WireAlertTriggeredAI()

	router.aiSettingsHandler = &AISettingsHandler{legacyAIService: ai.NewService(nil, nil)}
	router.WireAlertTriggeredAI()
}
