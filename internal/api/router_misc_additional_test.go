package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
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

func TestHandleInfrastructureSummaryCharts_MockModeSynthesizesDenseSeries(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	monitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/charts/infrastructure-summary?range=5m", nil)
	rec := httptest.NewRecorder()
	router.handleInfrastructureSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded InfrastructureChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal InfrastructureChartsResponse: %v", err)
	}

	if len(decoded.NodeData) == 0 {
		t.Fatalf("expected nodeData entries in mock mode")
	}

	foundDenseSeries := false
	for _, metrics := range decoded.NodeData {
		if len(metrics["cpu"]) >= 24 && len(metrics["memory"]) >= 24 && len(metrics["disk"]) >= 24 {
			foundDenseSeries = true
			break
		}
	}
	if !foundDenseSeries {
		t.Fatalf("expected at least one node with dense synthetic series (>=24 points for cpu/memory/disk)")
	}
}

func TestHandleWorkloadsSummaryCharts_AggregatesAndCounts(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:       "node-pve-1",
		Name:     "pve-1",
		Instance: "pve",
	}}
	state.VMs = []models.VM{{
		ID:         "vm-101",
		Name:       "vm-101",
		Node:       "pve-1",
		Instance:   "pve",
		Status:     "running",
		CPU:        0.25,
		Memory:     models.Memory{Usage: 40.0},
		Disk:       models.Disk{Usage: 55.0},
		NetworkIn:  1200,
		NetworkOut: 800,
	}}
	state.Containers = []models.Container{{
		ID:         "ct-201",
		Name:       "ct-201",
		Node:       "pve-1",
		Instance:   "pve",
		Status:     "stopped",
		CPU:        0.10,
		Memory:     models.Memory{Usage: 30.0},
		Disk:       models.Disk{Usage: 45.0},
		NetworkIn:  400,
		NetworkOut: 600,
	}}
	state.DockerHosts = []models.DockerHost{{
		ID:      "docker-host-1",
		Runtime: "docker",
		Containers: []models.DockerContainer{{
			ID:                  "docker-1",
			Name:                "docker-1",
			State:               "running",
			CPUPercent:          35.0,
			MemoryPercent:       60.0,
			WritableLayerBytes:  10,
			RootFilesystemBytes: 100,
		}},
	}}
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=5m", nil)
	rec := httptest.NewRecorder()

	router.handleWorkloadsSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	body := rec.Body.Bytes()

	var decoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadsSummaryChartsResponse: %v", err)
	}
	if decoded.Stats.Range != "5m" {
		t.Fatalf("expected stats.range=5m, got %q", decoded.Stats.Range)
	}
	if decoded.Stats.RangeSeconds != 300 {
		t.Fatalf("expected stats.rangeSeconds=300, got %d", decoded.Stats.RangeSeconds)
	}

	if decoded.GuestCounts.Total != 3 {
		t.Fatalf("expected guestCounts.total=3, got %d", decoded.GuestCounts.Total)
	}
	if decoded.GuestCounts.Running != 2 {
		t.Fatalf("expected guestCounts.running=2, got %d", decoded.GuestCounts.Running)
	}
	if decoded.GuestCounts.Stopped != 1 {
		t.Fatalf("expected guestCounts.stopped=1, got %d", decoded.GuestCounts.Stopped)
	}

	for metricName, metric := range map[string]WorkloadsSummaryMetricData{
		"cpu":     decoded.CPU,
		"memory":  decoded.Memory,
		"disk":    decoded.Disk,
		"network": decoded.Network,
	} {
		if len(metric.P50) == 0 {
			t.Fatalf("expected %s p50 points to be present", metricName)
		}
		if len(metric.P95) == 0 {
			t.Fatalf("expected %s p95 points to be present", metricName)
		}
	}

	if decoded.Stats.PointCounts.Total <= 0 {
		t.Fatalf("expected stats.pointCounts.total to be > 0, got %d", decoded.Stats.PointCounts.Total)
	}
	if decoded.Stats.PointCounts.Guests <= 0 {
		t.Fatalf("expected stats.pointCounts.guests to be > 0, got %d", decoded.Stats.PointCounts.Guests)
	}

	if decoded.BlastRadius.CPU.Scope != "concentrated" {
		t.Fatalf("expected cpu blast radius concentrated, got %q", decoded.BlastRadius.CPU.Scope)
	}
	if decoded.BlastRadius.Network.Scope != "concentrated" {
		t.Fatalf("expected network blast radius concentrated, got %q", decoded.BlastRadius.Network.Scope)
	}

	// Node-scoped request should only include workloads that match the selected node.
	nodeReq := httptest.NewRequest(
		http.MethodGet,
		"/api/charts/workloads-summary?range=5m&node=node-pve-1",
		nil,
	)
	nodeRec := httptest.NewRecorder()

	router.handleWorkloadsSummaryCharts(nodeRec, nodeReq)

	if nodeRec.Code != http.StatusOK {
		t.Fatalf("node-scoped expected status %d, got %d", http.StatusOK, nodeRec.Code)
	}

	var nodeScoped WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(nodeRec.Body.Bytes(), &nodeScoped); err != nil {
		t.Fatalf("unmarshal node-scoped WorkloadsSummaryChartsResponse: %v", err)
	}
	if nodeScoped.GuestCounts.Total != 2 {
		t.Fatalf("expected node-scoped guestCounts.total=2, got %d", nodeScoped.GuestCounts.Total)
	}
	if nodeScoped.GuestCounts.Running != 1 {
		t.Fatalf("expected node-scoped guestCounts.running=1, got %d", nodeScoped.GuestCounts.Running)
	}
	if nodeScoped.GuestCounts.Stopped != 1 {
		t.Fatalf("expected node-scoped guestCounts.stopped=1, got %d", nodeScoped.GuestCounts.Stopped)
	}
	if nodeScoped.BlastRadius.CPU.Scope != "concentrated" {
		t.Fatalf("expected node-scoped cpu blast radius concentrated, got %q", nodeScoped.BlastRadius.CPU.Scope)
	}
}

func TestHandleWorkloadCharts_WorkloadOnlyPayloadAndNodeFilter(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:       "node-pve-1",
		Name:     "pve-1",
		Instance: "pve",
	}}
	state.VMs = []models.VM{
		{
			ID:       "vm-101",
			Name:     "vm-101",
			Node:     "pve-1",
			Instance: "pve",
			CPU:      0.2,
			Memory:   models.Memory{Usage: 55},
			Disk:     models.Disk{Usage: 40},
		},
		{
			ID:       "vm-202",
			Name:     "vm-202",
			Node:     "other",
			Instance: "other",
			CPU:      0.4,
			Memory:   models.Memory{Usage: 65},
			Disk:     models.Disk{Usage: 70},
		},
	}
	state.Containers = []models.Container{
		{
			ID:       "ct-301",
			Name:     "ct-301",
			Node:     "pve-1",
			Instance: "pve",
			CPU:      0.3,
			Memory:   models.Memory{Usage: 40},
			Disk:     models.Disk{Usage: 25},
		},
	}
	state.DockerHosts = []models.DockerHost{{
		ID:          "docker-host-1",
		Hostname:    "pve-1",
		DisplayName: "pve-1",
		Containers: []models.DockerContainer{{
			ID:            "docker-401",
			Name:          "docker-401",
			CPUPercent:    22,
			MemoryPercent: 30,
		}},
	}}

	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=5m", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadChartsResponse: %v", err)
	}

	if decoded.Stats.Range != "5m" {
		t.Fatalf("expected stats.range=5m, got %q", decoded.Stats.Range)
	}
	if decoded.Stats.RangeSeconds != 300 {
		t.Fatalf("expected stats.rangeSeconds=300, got %d", decoded.Stats.RangeSeconds)
	}
	if len(decoded.ChartData) != 3 {
		t.Fatalf("expected 3 workload chart entries, got %d", len(decoded.ChartData))
	}
	if len(decoded.DockerData) != 1 {
		t.Fatalf("expected 1 docker chart entry, got %d", len(decoded.DockerData))
	}
	if decoded.GuestTypes["vm-101"] != "vm" {
		t.Fatalf("expected vm guest type for vm-101, got %q", decoded.GuestTypes["vm-101"])
	}
	if decoded.GuestTypes["ct-301"] != "container" {
		t.Fatalf("expected container guest type for ct-301, got %q", decoded.GuestTypes["ct-301"])
	}
	if decoded.Stats.PointCounts.Total <= 0 {
		t.Fatalf("expected stats.pointCounts.total > 0, got %d", decoded.Stats.PointCounts.Total)
	}

	// Node-scoped request should only include workloads linked to node-pve-1
	// plus docker containers running on matching docker hosts.
	nodeReq := httptest.NewRequest(
		http.MethodGet,
		"/api/charts/workloads?range=5m&node=node-pve-1",
		nil,
	)
	nodeRec := httptest.NewRecorder()
	router.handleWorkloadCharts(nodeRec, nodeReq)

	if nodeRec.Code != http.StatusOK {
		t.Fatalf("node-scoped expected status %d, got %d", http.StatusOK, nodeRec.Code)
	}

	var scoped WorkloadChartsResponse
	if err := json.Unmarshal(nodeRec.Body.Bytes(), &scoped); err != nil {
		t.Fatalf("unmarshal node-scoped WorkloadChartsResponse: %v", err)
	}
	if len(scoped.ChartData) != 2 {
		t.Fatalf("expected 2 scoped workload chart entries, got %d", len(scoped.ChartData))
	}
	if _, ok := scoped.ChartData["vm-202"]; ok {
		t.Fatalf("expected vm-202 to be excluded by node scope")
	}
	if len(scoped.DockerData) != 1 {
		t.Fatalf("expected 1 scoped docker chart entry, got %d", len(scoped.DockerData))
	}
}

func TestHandleWorkloadCharts_MockModeSynthesizesDenseSeries(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:       "node-pve-1",
		Name:     "pve-1",
		Instance: "pve",
	}}
	state.VMs = []models.VM{{
		ID:       "vm-101",
		Name:     "vm-101",
		Node:     "pve-1",
		Instance: "pve",
		CPU:      0.3,
		Memory:   models.Memory{Usage: 42},
		Disk:     models.Disk{Usage: 55},
	}}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=5m", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadChartsResponse: %v", err)
	}
	if len(decoded.ChartData) == 0 {
		t.Fatalf("expected workload chart entries in mock mode")
	}
	for id, metricSeries := range decoded.ChartData {
		cpuSeries := metricSeries["cpu"]
		if len(cpuSeries) < 24 {
			t.Fatalf("expected at least 24 mock cpu points for %s, got %d", id, len(cpuSeries))
		}
	}
}

func TestHandleWorkloadCharts_IncludesKubernetesPods(t *testing.T) {
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(false)
	t.Cleanup(func() { mock.SetEnabled(prevMock) })

	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:       "node-pve-1",
		Name:     "pve-1",
		Instance: "pve",
	}}
	state.KubernetesClusters = []models.KubernetesCluster{{
		ID:     "cluster-k8s-summary-test",
		Name:   "cluster-k8s-summary-test",
		Status: "online",
		Pods: []models.KubernetesPod{
			{
				UID:       "pod-001",
				Name:      "api-0",
				Namespace: "default",
				NodeName:  "pve-1",
				Phase:     "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "api", Ready: true},
				},
			},
		},
	}}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=5m", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadChartsResponse: %v", err)
	}

	var metricID string
	for id, typ := range decoded.GuestTypes {
		if typ == "k8s" {
			metricID = id
			break
		}
	}
	if metricID == "" {
		t.Fatal("expected at least one kubernetes pod series")
	}

	series, ok := decoded.ChartData[metricID]
	if !ok {
		t.Fatalf("expected kubernetes pod series for %s", metricID)
	}
	if len(series["cpu"]) == 0 {
		t.Fatalf("expected kubernetes cpu points for %s", metricID)
	}
	if got := len(series["disk"]); got != 1 || series["disk"][0].Value != 0 {
		t.Fatalf("expected unsupported kubernetes pod disk metric fallback to single zero point; got len=%d value=%v", got, series["disk"])
	}
	if got := len(series["netin"]); got != 1 || series["netin"][0].Value != 0 {
		t.Fatalf("expected unsupported kubernetes pod network metric fallback to single zero point; got len=%d value=%v", got, series["netin"])
	}
}

func TestHandleWorkloadCharts_MockModeSynthesizesDenseSeriesForKubernetesPods(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	monitor, state, _ := newTestMonitor(t)
	_ = state

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=5m", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadChartsResponse: %v", err)
	}

	var foundID string
	for id, typ := range decoded.GuestTypes {
		if typ == "k8s" {
			foundID = id
			break
		}
	}
	if foundID == "" {
		t.Fatal("expected at least one kubernetes workload series in mock mode")
	}

	series, ok := decoded.ChartData[foundID]
	if !ok {
		t.Fatalf("expected kubernetes pod series for %s", foundID)
	}
	if got := len(series["cpu"]); got < 24 {
		t.Fatalf("expected dense mock kubernetes cpu series for %s, got %d points", foundID, got)
	}
	if got := len(series["memory"]); got < 24 {
		t.Fatalf("expected dense mock kubernetes memory series for %s, got %d points", foundID, got)
	}
	if got := len(series["disk"]); got < 24 {
		t.Fatalf("expected dense mock kubernetes disk series for %s, got %d points", foundID, got)
	}
	if got := len(series["netin"]); got < 24 {
		t.Fatalf("expected dense mock kubernetes netin series for %s, got %d points", foundID, got)
	}
	if got := len(series["netout"]); got < 24 {
		t.Fatalf("expected dense mock kubernetes netout series for %s, got %d points", foundID, got)
	}
}

func TestHandleWorkloadsSummaryCharts_IncludesKubernetesPods(t *testing.T) {
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(false)
	t.Cleanup(func() { mock.SetEnabled(prevMock) })

	monitor, state, _ := newTestMonitor(t)
	state.KubernetesClusters = []models.KubernetesCluster{{
		ID:     "cluster-alpha",
		Name:   "cluster-alpha",
		Status: "online",
		Pods: []models.KubernetesPod{
			{
				UID:       "pod-001",
				Name:      "api-0",
				Namespace: "default",
				Phase:     "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "api", Ready: true},
				},
			},
			{
				UID:       "pod-002",
				Name:      "batch-0",
				Namespace: "ops",
				Phase:     "Failed",
				Containers: []models.KubernetesPodContainer{
					{Name: "worker", Ready: false},
				},
			},
		},
	}}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=5m", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadsSummaryChartsResponse: %v", err)
	}

	if decoded.GuestCounts.Total != 2 {
		t.Fatalf("expected guestCounts.total=2, got %d", decoded.GuestCounts.Total)
	}
	if decoded.GuestCounts.Running != 1 {
		t.Fatalf("expected guestCounts.running=1, got %d", decoded.GuestCounts.Running)
	}
	if decoded.GuestCounts.Stopped != 1 {
		t.Fatalf("expected guestCounts.stopped=1, got %d", decoded.GuestCounts.Stopped)
	}
	if len(decoded.CPU.P50) == 0 {
		t.Fatal("expected cpu summary points for kubernetes workloads")
	}
}

func TestHandleWorkloadCharts_UnknownNodeFilterFallsBackToGlobalScope(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:       "node-pve-1",
		Name:     "pve-1",
		Instance: "pve",
	}}
	state.VMs = []models.VM{{
		ID:       "vm-101",
		Name:     "vm-101",
		Node:     "pve-1",
		Instance: "pve",
		CPU:      0.3,
		Memory:   models.Memory{Usage: 42},
		Disk:     models.Disk{Usage: 55},
	}}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=5m&node=missing-node-id", nil)
	rec := httptest.NewRecorder()

	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadChartsResponse: %v", err)
	}
	if len(decoded.ChartData) == 0 {
		t.Fatalf("expected fallback to global scope when node filter is stale")
	}
}

func TestHandleWorkloadsSummaryCharts_UnknownNodeFilterFallsBackToGlobalScope(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:       "node-pve-1",
		Name:     "pve-1",
		Instance: "pve",
	}}
	state.VMs = []models.VM{{
		ID:       "vm-101",
		Name:     "vm-101",
		Node:     "pve-1",
		Instance: "pve",
		CPU:      0.3,
		Memory:   models.Memory{Usage: 42},
		Disk:     models.Disk{Usage: 55},
		Status:   "running",
	}}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=5m&node=missing-node-id", nil)
	rec := httptest.NewRecorder()

	router.handleWorkloadsSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var decoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal WorkloadsSummaryChartsResponse: %v", err)
	}
	if decoded.GuestCounts.Total == 0 {
		t.Fatalf("expected fallback to global scope when summary node filter is stale")
	}
}

func TestBuildSyntheticWorkloadSeries_ProducesJaggedDeterministicSeries(t *testing.T) {
	now := time.Now().UnixMilli()
	seed := hashChartSeed("test", "jagged", "cpu")
	seriesA := buildSyntheticWorkloadSeries(now, time.Hour, 120, 48, 0, 100, seed)
	seriesB := buildSyntheticWorkloadSeries(now, time.Hour, 120, 48, 0, 100, seed)

	if len(seriesA) != 120 {
		t.Fatalf("expected 120 points, got %d", len(seriesA))
	}
	if len(seriesA) != len(seriesB) {
		t.Fatalf("expected deterministic length, got %d vs %d", len(seriesA), len(seriesB))
	}
	for i := range seriesA {
		if seriesA[i] != seriesB[i] {
			t.Fatalf("determinism failure at %d", i)
		}
		if seriesA[i].Value < 0 || seriesA[i].Value > 100 {
			t.Fatalf("value out of bounds at %d: %.3f", i, seriesA[i].Value)
		}
	}
	if seriesA[len(seriesA)-1].Value != 48 {
		t.Fatalf("expected anchored final value 48, got %.3f", seriesA[len(seriesA)-1].Value)
	}

	kinks := 0
	for i := 2; i < len(seriesA); i++ {
		s1 := seriesA[i-1].Value - seriesA[i-2].Value
		s2 := seriesA[i].Value - seriesA[i-1].Value
		if math.Abs(s2-s1) >= 0.8 {
			kinks++
		}
	}
	if kinks < 16 {
		t.Fatalf("expected jagged synthetic series with many inflection points; got %d", kinks)
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
