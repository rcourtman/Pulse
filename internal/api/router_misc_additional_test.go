package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

// syncTestResourceStore populates a MonitorAdapter (ResourceRegistry) from the
// legacy state so that GetUnifiedReadState() returns a valid ReadState.
// Call this after setting state.VMs, state.Nodes, etc.
func syncTestResourceStore(t *testing.T, monitor *monitoring.Monitor, state *models.State) {
	t.Helper()
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(state.GetSnapshot())
	setUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestNormalizeMetricsHistoryResourceType_ContainerCanonicalTypes(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantResponse string
		wantRuntime  string
		wantStore    []string
	}{
		{
			name:         "system container",
			input:        "system-container",
			wantResponse: "system-container",
			wantRuntime:  "system-container",
			wantStore:    []string{"container"},
		},
		{
			name:         "oci container",
			input:        "oci-container",
			wantResponse: "oci-container",
			wantRuntime:  "oci-container",
			wantStore:    []string{"container"},
		},
		{
			name:         "app container",
			input:        "app-container",
			wantResponse: "app-container",
			wantRuntime:  "app-container",
			wantStore:    []string{"dockerContainer", "docker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseType, runtimeType, storeTypes, err := normalizeMetricsHistoryResourceType(tt.input)
			if err != nil {
				t.Fatalf("normalizeMetricsHistoryResourceType(%q) error = %v", tt.input, err)
			}
			if responseType != tt.wantResponse {
				t.Fatalf("responseType = %q, want %q", responseType, tt.wantResponse)
			}
			if runtimeType != tt.wantRuntime {
				t.Fatalf("runtimeType = %q, want %q", runtimeType, tt.wantRuntime)
			}
			if len(storeTypes) != len(tt.wantStore) {
				t.Fatalf("storeTypes len = %d, want %d (%v)", len(storeTypes), len(tt.wantStore), storeTypes)
			}
			for i := range storeTypes {
				if storeTypes[i] != tt.wantStore[i] {
					t.Fatalf("storeTypes[%d] = %q, want %q (all=%v)", i, storeTypes[i], tt.wantStore[i], storeTypes)
				}
			}
		})
	}
}

func TestNormalizeMetricsHistoryResourceType_RejectsLegacyContainerAlias(t *testing.T) {
	_, _, _, err := normalizeMetricsHistoryResourceType("container")
	if err == nil {
		t.Fatal("expected error for legacy container alias")
	}
	if !strings.Contains(err.Error(), `unsupported resourceType "container"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeMetricsHistoryResourceType_DockerHostCanonicalType(t *testing.T) {
	responseType, runtimeType, storeTypes, err := normalizeMetricsHistoryResourceType("docker-host")
	if err != nil {
		t.Fatalf("normalizeMetricsHistoryResourceType(docker-host) error = %v", err)
	}
	if responseType != "docker-host" {
		t.Fatalf("responseType = %q, want %q", responseType, "docker-host")
	}
	if runtimeType != "docker-host" {
		t.Fatalf("runtimeType = %q, want %q", runtimeType, "docker-host")
	}
	if len(storeTypes) != 1 || storeTypes[0] != "dockerHost" {
		t.Fatalf("storeTypes = %v, want [dockerHost]", storeTypes)
	}
}

func TestNormalizeMetricsHistoryResourceType_AgentQueriesNodeHistoryAsFallback(t *testing.T) {
	responseType, runtimeType, storeTypes, err := normalizeMetricsHistoryResourceType("agent")
	if err != nil {
		t.Fatalf("normalizeMetricsHistoryResourceType(agent) error = %v", err)
	}
	if responseType != "agent" {
		t.Fatalf("responseType = %q, want %q", responseType, "agent")
	}
	if runtimeType != "agent" {
		t.Fatalf("runtimeType = %q, want %q", runtimeType, "agent")
	}
	if len(storeTypes) != 2 || storeTypes[0] != "agent" || storeTypes[1] != "node" {
		t.Fatalf("storeTypes = %v, want [agent node]", storeTypes)
	}
}

func TestNormalizeMetricsHistoryResourceType_RejectsLegacyAliases(t *testing.T) {
	legacyTypes := []string{"host", "guest", "docker", "dockerhost", "dockercontainer", "system_container"}
	for _, legacyType := range legacyTypes {
		_, _, _, err := normalizeMetricsHistoryResourceType(legacyType)
		if err == nil {
			t.Fatalf("expected error for legacy alias %q", legacyType)
		}
		if !strings.Contains(err.Error(), fmt.Sprintf(`unsupported resourceType %q`, legacyType)) {
			t.Fatalf("unexpected error for %q: %v", legacyType, err)
		}
	}
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

func TestHandleSchedulerHealth_UsesCanonicalEmptyCollections(t *testing.T) {
	router := &Router{monitor: &monitoring.Monitor{}}
	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/health", nil)
	rec := httptest.NewRecorder()

	router.handleSchedulerHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"breakers":[]`) {
		t.Fatalf("expected breakers array to be retained, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"staleness":[]`) {
		t.Fatalf("expected staleness array to be retained, got %s", rec.Body.String())
	}
}

func TestHandleHealth_NoMonitor(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.handleHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != "unhealthy" {
		t.Fatalf("expected unhealthy status, got %q", response.Status)
	}

	if response.Dependencies["monitor"] {
		t.Fatalf("expected monitor dependency to be unhealthy")
	}
}

func TestHandleHealth_WithMonitor(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Fatalf("expected healthy status, got %q", response.Status)
	}
	if !response.Dependencies["monitor"] {
		t.Fatalf("expected monitor dependency to be healthy")
	}
	if !response.Dependencies["scheduler"] {
		t.Fatalf("expected scheduler dependency to be healthy")
	}
	if response.Dependencies["websocket"] {
		t.Fatalf("expected websocket dependency to be false when ws hub is not configured")
	}
}

func TestHealthResponse_UsesCanonicalEmptyDependencies(t *testing.T) {
	payload, err := json.Marshal(EmptyHealthResponse())
	if err != nil {
		t.Fatalf("marshal empty health response: %v", err)
	}
	if !strings.Contains(string(payload), `"dependencies":{}`) {
		t.Fatalf("expected empty health response to retain dependencies map, got %s", payload)
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

func TestEstablishSession(t *testing.T) {
	InitPersistentAuthStores(t.TempDir())
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
	InitPersistentAuthStores(t.TempDir())
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	oidc := &OIDCTokenInfo{
		RefreshToken:   "refresh",
		AccessTokenExp: time.Now().Add(1 * time.Hour),
		Issuer:         "issuer",
		ClientID:       "client",
	}

	if err := router.establishOIDCSession(rec, req, "admin", "admin", oidc); err != nil {
		t.Fatalf("establishOIDCSession error: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected session and csrf cookies, got %d", len(cookies))
	}
}

func TestLearnBaselines_WithData(t *testing.T) {
	monitor, state, history := newTestMonitor(t)
	state.Nodes = []models.Node{{ID: "node-1", Name: "node"}}
	state.VMs = []models.VM{{ID: "vm-1", Name: "vm", Status: "running"}}
	state.Containers = []models.Container{{ID: "ct-1", Name: "ct", Status: "running"}}

	// Sync ReadState from legacy state — learnBaselines uses ReadState exclusively.
	syncTestResourceStore(t, monitor, state)

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
