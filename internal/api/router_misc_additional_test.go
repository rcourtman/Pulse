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

func TestHandleCharts_StatsDebugMetadata(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.VMs = []models.VM{{ID: "vm-1", Name: "vm-one", CPU: 0.2}}
	state.Nodes = []models.Node{{ID: "node-1", Name: "node-one", CPU: 0.1}}
	state.Storage = []models.Storage{{ID: "store-1", Name: "Store One", Used: 50, Total: 100}}
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/charts?range=5m", nil)
	rec := httptest.NewRecorder()

	router.handleCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.Bytes()

	var decoded ChartResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal ChartResponse: %v", err)
	}

	if decoded.Stats.Range != "5m" {
		t.Fatalf("expected stats.range=5m, got %q", decoded.Stats.Range)
	}
	if decoded.Stats.RangeSeconds != 300 {
		t.Fatalf("expected stats.rangeSeconds=300, got %d", decoded.Stats.RangeSeconds)
	}
	if decoded.Stats.MetricsStoreEnabled {
		t.Fatalf("expected stats.metricsStoreEnabled=false in test monitor, got true")
	}
	if decoded.Stats.PrimarySourceHint != "memory" {
		t.Fatalf("expected stats.primarySourceHint=memory, got %q", decoded.Stats.PrimarySourceHint)
	}
	if decoded.Stats.InMemoryThresholdSecs != 7200 {
		t.Fatalf("expected stats.inMemoryThresholdSecs=7200, got %d", decoded.Stats.InMemoryThresholdSecs)
	}
	if decoded.Stats.OldestDataTimestamp <= 0 {
		t.Fatalf("expected stats.oldestDataTimestamp to be set, got %d", decoded.Stats.OldestDataTimestamp)
	}
	if decoded.Stats.OldestDataTimestamp > decoded.Timestamp {
		t.Fatalf(
			"expected stats.oldestDataTimestamp <= timestamp, got oldest=%d timestamp=%d",
			decoded.Stats.OldestDataTimestamp,
			decoded.Timestamp,
		)
	}

	// With no history in the test monitor, handleCharts falls back to synthetic points:
	// guests: cpu/memory/disk/diskread/diskwrite/netin/netout (7)
	// nodes: cpu/memory/disk (3)
	// storage: disk (1)
	if decoded.Stats.PointCounts.Guests != 7 {
		t.Fatalf("expected stats.pointCounts.guests=7, got %d", decoded.Stats.PointCounts.Guests)
	}
	if decoded.Stats.PointCounts.Nodes != 3 {
		t.Fatalf("expected stats.pointCounts.nodes=3, got %d", decoded.Stats.PointCounts.Nodes)
	}
	if decoded.Stats.PointCounts.Storage != 1 {
		t.Fatalf("expected stats.pointCounts.storage=1, got %d", decoded.Stats.PointCounts.Storage)
	}
	if decoded.Stats.PointCounts.DockerContainers != 0 || decoded.Stats.PointCounts.DockerHosts != 0 || decoded.Stats.PointCounts.Hosts != 0 {
		t.Fatalf(
			"expected dockerContainers/dockerHosts/hosts all 0, got dc=%d dh=%d hosts=%d",
			decoded.Stats.PointCounts.DockerContainers,
			decoded.Stats.PointCounts.DockerHosts,
			decoded.Stats.PointCounts.Hosts,
		)
	}

	sum := decoded.Stats.PointCounts.Guests +
		decoded.Stats.PointCounts.Nodes +
		decoded.Stats.PointCounts.Storage +
		decoded.Stats.PointCounts.DockerContainers +
		decoded.Stats.PointCounts.DockerHosts +
		decoded.Stats.PointCounts.Hosts
	if decoded.Stats.PointCounts.Total != sum {
		t.Fatalf("expected stats.pointCounts.total=%d, got %d", sum, decoded.Stats.PointCounts.Total)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal raw JSON: %v", err)
	}
	stats, ok := raw["stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stats object in JSON response")
	}
	if _, ok := stats["pointCounts"]; !ok {
		t.Fatalf("expected stats.pointCounts to be present in JSON response")
	}
}

func TestHandleInfrastructureSummaryCharts_Lightweight(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:     "node-1",
		Name:   "node-one",
		Status: "online",
		CPU:    0.1,
		Memory: models.Memory{Usage: 12.0},
		Disk:   models.Disk{Usage: 34.0},
	}}
	state.DockerHosts = []models.DockerHost{{
		ID:       "docker-host-1",
		Runtime:  "docker",
		CPUUsage: 23.0,
		Memory:   models.Memory{Usage: 45.0},
		Disks:    []models.Disk{{Usage: 67.0}},
		Status:   "online",
	}}
	state.Hosts = []models.Host{{
		ID:       "host-1",
		Hostname: "host-one",
		CPUUsage: 11.0,
		Memory:   models.Memory{Usage: 22.0},
		Disks:    []models.Disk{{Usage: 33.0}},
		Status:   "online",
	}}
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/charts/infrastructure-summary?range=5m", nil)
	rec := httptest.NewRecorder()

	router.handleInfrastructureSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	body := rec.Body.Bytes()

	var decoded InfrastructureChartsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal InfrastructureChartsResponse: %v", err)
	}
	if decoded.Stats.Range != "5m" {
		t.Fatalf("expected stats.range=5m, got %q", decoded.Stats.Range)
	}
	if decoded.Stats.RangeSeconds != 300 {
		t.Fatalf("expected stats.rangeSeconds=300, got %d", decoded.Stats.RangeSeconds)
	}

	// With no history in the test monitor, handler falls back to synthetic points:
	// nodes: cpu/memory/disk (3)
	// dockerHosts: cpu/memory/disk (3)
	// hosts: cpu/memory/disk (3)
	if decoded.Stats.PointCounts.Nodes != 3 {
		t.Fatalf("expected stats.pointCounts.nodes=3, got %d", decoded.Stats.PointCounts.Nodes)
	}
	if decoded.Stats.PointCounts.DockerHosts != 3 {
		t.Fatalf("expected stats.pointCounts.dockerHosts=3, got %d", decoded.Stats.PointCounts.DockerHosts)
	}
	if decoded.Stats.PointCounts.Hosts != 3 {
		t.Fatalf("expected stats.pointCounts.hosts=3, got %d", decoded.Stats.PointCounts.Hosts)
	}
	sum := decoded.Stats.PointCounts.Nodes + decoded.Stats.PointCounts.DockerHosts + decoded.Stats.PointCounts.Hosts
	if decoded.Stats.PointCounts.Total != sum {
		t.Fatalf("expected stats.pointCounts.total=%d, got %d", sum, decoded.Stats.PointCounts.Total)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal raw JSON: %v", err)
	}
	for _, forbidden := range []string{"data", "storageData", "dockerData", "guestTypes"} {
		if _, ok := raw[forbidden]; ok {
			t.Fatalf("expected %q to be absent from infra summary response", forbidden)
		}
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
