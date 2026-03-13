package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	sloInfrastructureChartsGitHubActionsP95   = 140 * time.Millisecond
	sloWorkloadChartsGitHubActionsP95         = 350 * time.Millisecond
	sloWorkloadsSummaryChartsGitHubActionsP95 = sloWorkloadChartsGitHubActionsP95
)

// suppressTestLogs disables zerolog for the duration of a test.
func suppressTestLogs(t *testing.T) {
	t.Helper()
	orig := log.Logger
	log.Logger = zerolog.Nop()
	t.Cleanup(func() { log.Logger = orig })
}

// setTestUnexportedField sets an unexported field on a struct via reflection.
func setTestUnexportedField(t *testing.T, target interface{}, field string, value interface{}) {
	t.Helper()
	v := reflect.ValueOf(target).Elem()
	f := v.FieldByName(field)
	if !f.IsValid() {
		t.Fatalf("field %q not found", field)
	}
	ptr := unsafe.Pointer(f.UnsafeAddr())
	reflect.NewAt(f.Type(), ptr).Elem().Set(reflect.ValueOf(value))
}

// TestSLO_MetricsHistoryStore validates that the metrics-store/history handler
// (SQLite path) meets SLOMetricsHistoryStoreP95 under benchmark conditions.
func TestSLO_MetricsHistoryStore(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newTestMetricsStore(t)
	const numPoints = 500
	metricTypes := []string{"cpu", "memory", "disk", "netin"}
	ids := seedTestMetrics(t, store, "vm", metricTypes, 10, numPoints)

	state := models.NewState()
	monitor := &monitoring.Monitor{}
	setTestUnexportedField(t, monitor, "state", state)
	setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setTestUnexportedField(t, monitor, "metricsStore", store)

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	url := "/api/metrics-store/history?resourceType=vm&resourceId=" + ids[0] + "&metric=cpu&range=1h"

	// Sanity check: verify the store path is exercised and returns expected data.
	sanityReq := httptest.NewRequest(http.MethodGet, url, nil)
	sanityRec := httptest.NewRecorder()
	router.handleMetricsHistory(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d, body: %s", sanityRec.Code, sanityRec.Body.String())
	}
	var sanityResp metricsHistoryResponse
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &sanityResp); err != nil {
		t.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if sanityResp.Source != "store" {
		t.Fatalf("sanity check: expected source=store, got %q", sanityResp.Source)
	}
	if len(sanityResp.Points) == 0 {
		t.Fatal("sanity check: expected non-empty points from store path")
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleMetricsHistory(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	target := effectiveAPISLOTarget(SLOMetricsHistoryStoreP95, 0)
	t.Logf("metrics-store/history (store) p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

// TestSLO_MetricsHistoryMemory validates the in-memory fallback path.
func TestSLO_MetricsHistoryMemory(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	state := models.NewState()
	vms := make([]models.VM, 10)
	for i := range vms {
		vms[i] = models.VM{
			ID:       fmt.Sprintf("pve1:node1:%d", 100+i),
			VMID:     100 + i,
			Name:     fmt.Sprintf("vm-%d", 100+i),
			Node:     "node1",
			Instance: "pve1",
			Status:   "running",
			Type:     "qemu",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20)},
			Disk:     models.Disk{Usage: float64(i%40 + 30)},
		}
	}
	state.UpdateVMsForInstance("pve1", vms)

	mh := monitoring.NewMetricsHistory(1000, time.Hour)
	now := time.Now()
	for _, vm := range vms {
		for j := 0; j < 60; j++ {
			ts := now.Add(time.Duration(-60+j) * time.Minute)
			mh.AddGuestMetric(vm.ID, "cpu", vm.CPU*100+float64(j%10), ts)
			mh.AddGuestMetric(vm.ID, "memory", vm.Memory.Usage+float64(j%5), ts)
		}
	}

	monitor := &monitoring.Monitor{}
	setTestUnexportedField(t, monitor, "state", state)
	setTestUnexportedField(t, monitor, "metricsHistory", mh)

	router := &Router{monitor: monitor}

	url := "/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:100&metric=cpu&range=1h"

	// Sanity check: verify the memory fallback path is exercised.
	sanityReq := httptest.NewRequest(http.MethodGet, url, nil)
	sanityRec := httptest.NewRecorder()
	router.handleMetricsHistory(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d, body: %s", sanityRec.Code, sanityRec.Body.String())
	}
	var sanityResp metricsHistoryResponse
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &sanityResp); err != nil {
		t.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if sanityResp.Source != "memory" {
		t.Fatalf("sanity check: expected source=memory, got %q", sanityResp.Source)
	}
	if len(sanityResp.Points) == 0 {
		t.Fatal("sanity check: expected non-empty points from memory fallback")
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleMetricsHistory(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	t.Logf("metrics-store/history (memory) p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), SLOMetricsHistoryMemoryP95)

	if p95 > SLOMetricsHistoryMemoryP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOMetricsHistoryMemoryP95)
	}
}

// TestSLO_MetricsStoreStats validates the /api/metrics-store/stats endpoint.
func TestSLO_MetricsStoreStats(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newTestMetricsStore(t)
	seedTestMetrics(t, store, "node", []string{"cpu"}, 5, 100)

	monitor := &monitoring.Monitor{}
	setTestUnexportedField(t, monitor, "state", models.NewState())
	setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setTestUnexportedField(t, monitor, "metricsStore", store)

	router := &Router{monitor: monitor}

	// Sanity check: verify stats endpoint returns valid data.
	sanityReq := httptest.NewRequest(http.MethodGet, "/api/metrics-store/stats", nil)
	sanityRec := httptest.NewRecorder()
	router.handleMetricsStoreStats(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d", sanityRec.Code)
	}
	var statsCheck map[string]interface{}
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &statsCheck); err != nil {
		t.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if enabled, _ := statsCheck["enabled"].(bool); !enabled {
		t.Fatal("sanity check: expected enabled=true in stats response")
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/stats", nil)
		rec := httptest.NewRecorder()
		router.handleMetricsStoreStats(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	t.Logf("metrics-store/stats p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), SLOMetricsStoreStatsP95)

	if p95 > SLOMetricsStoreStatsP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOMetricsStoreStatsP95)
	}
}

// TestSLO_ResourcesList validates the GET /api/resources endpoint with ~85
// resources in state (5 nodes + 50 VMs + 30 containers). The handler uses
// default pagination (limit=50), so response encodes up to 50 resources.
func TestSLO_ResourcesList(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	state := models.NewState()
	nodes := make([]models.Node, 5)
	for i := range nodes {
		nodes[i] = models.Node{
			ID:       fmt.Sprintf("pve1:node%d", i),
			Name:     fmt.Sprintf("node%d", i),
			Instance: "pve1",
			Status:   "online",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 64 << 30, Used: 32 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 500 << 30, Used: 250 << 30},
		}
	}
	state.UpdateNodesForInstance("pve1", nodes)

	vms := make([]models.VM, 50)
	for i := range vms {
		vms[i] = models.VM{
			ID:       fmt.Sprintf("pve1:node%d:%d", i%5, 100+i),
			VMID:     100 + i,
			Name:     fmt.Sprintf("vm-%d", 100+i),
			Node:     fmt.Sprintf("node%d", i%5),
			Instance: "pve1",
			Status:   "running",
			Type:     "qemu",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 4 << 30, Used: 2 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 50 << 30, Used: 25 << 30},
		}
	}
	state.UpdateVMsForInstance("pve1", vms)

	containers := make([]models.Container, 30)
	for i := range containers {
		containers[i] = models.Container{
			ID:       fmt.Sprintf("pve1:node%d:%d", i%5, 200+i),
			VMID:     200 + i,
			Name:     fmt.Sprintf("ct-%d", 200+i),
			Node:     fmt.Sprintf("node%d", i%5),
			Instance: "pve1",
			Status:   "running",
			Type:     "lxc",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 2 << 30, Used: 1 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 20 << 30, Used: 10 << 30},
		}
	}
	state.UpdateContainersForInstance("pve1", containers)

	cfg := &config.Config{DataPath: t.TempDir()}
	handlers := NewResourceHandlers(cfg)
	handlers.SetStateProvider(&sloTestStateProvider{state: state})

	// Warm the cache — first request populates it, subsequent requests hit cache.
	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rec := httptest.NewRecorder()
	handlers.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("warmup failed: status %d", rec.Code)
	}
	var checkResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &checkResp); err != nil {
		t.Fatalf("warmup unmarshal: %v", err)
	}
	data, _ := checkResp["data"].([]interface{})
	if len(data) == 0 {
		t.Fatalf("warmup: expected resources, got none")
	}
	// Verify the workload matches expectations: 50 items per page (default limit),
	// 85 total resources (5 nodes + 50 VMs + 30 containers).
	if len(data) != 50 {
		t.Fatalf("warmup: expected 50 resources in first page, got %d", len(data))
	}
	meta, _ := checkResp["meta"].(map[string]interface{})
	if total, _ := meta["total"].(float64); int(total) != 85 {
		t.Fatalf("warmup: expected total=85, got %v", total)
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
		rec := httptest.NewRecorder()
		handlers.HandleListResources(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	t.Logf("resources/list p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), SLOResourcesListP95)

	if p95 > SLOResourcesListP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOResourcesListP95)
	}
}

// TestSLO_InfrastructureCharts validates the lightweight infrastructure charts
// endpoint that drives infrastructure summary sparklines. The workload forces
// the store-backed batch path across nodes, docker hosts, and unified agents.
func TestSLO_InfrastructureCharts(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newTestMetricsStore(t)
	const (
		nodeCount       = 20
		dockerHostCount = 10
		agentCount      = 10
		pointsPerMetric = 240
	)
	base := time.Now().Add(-4 * time.Hour)

	seedBatchMetrics := func(resourceType string, ids []string, metricTypes []string) {
		batch := make([]metrics.WriteMetric, 0, len(ids)*len(metricTypes)*pointsPerMetric)
		for idx, id := range ids {
			for _, mt := range metricTypes {
				for p := 0; p < pointsPerMetric; p++ {
					batch = append(batch, metrics.WriteMetric{
						ResourceType: resourceType,
						ResourceID:   id,
						MetricType:   mt,
						Value:        float64((idx + p) % 100),
						Timestamp:    base.Add(time.Duration(p) * time.Minute),
						Tier:         metrics.TierMinute,
					})
				}
			}
		}
		store.WriteBatchSync(batch)
	}

	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	nodes := make([]models.Node, nodeCount)
	nodeIDs := make([]string, nodeCount)
	for i := range nodes {
		nodeIDs[i] = fmt.Sprintf("node-slo-%d", i)
		nodes[i] = models.Node{
			ID:       nodeIDs[i],
			Name:     fmt.Sprintf("node-%d", i),
			Instance: "pve1",
			Status:   "online",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 64 << 30, Used: 32 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 500 << 30, Used: 250 << 30},
		}
	}
	state.Nodes = nodes

	dockerHosts := make([]models.DockerHost, dockerHostCount)
	dockerHostIDs := make([]string, dockerHostCount)
	for i := range dockerHosts {
		dockerHostIDs[i] = fmt.Sprintf("docker-host-slo-%d", i)
		dockerHosts[i] = models.DockerHost{
			ID:       dockerHostIDs[i],
			Runtime:  "docker",
			Status:   "online",
			CPUUsage: float64(i%80 + 10),
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 32 << 30, Used: 16 << 30},
			Disks:    []models.Disk{{Usage: float64(i%40 + 30), Total: 200 << 30, Used: 100 << 30}},
		}
	}
	state.DockerHosts = dockerHosts

	hosts := make([]models.Host, agentCount)
	agentIDs := make([]string, agentCount)
	for i := range hosts {
		agentIDs[i] = fmt.Sprintf("agent-slo-%d", i)
		hosts[i] = models.Host{
			ID:       agentIDs[i],
			Hostname: fmt.Sprintf("agent-host-%d", i),
			Status:   "online",
			CPUUsage: float64(i%80 + 10),
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 32 << 30, Used: 16 << 30},
			Disks:    []models.Disk{{Usage: float64(i%40 + 30), Total: 200 << 30, Used: 100 << 30}},
		}
	}
	state.Hosts = hosts
	syncTestResourceStore(t, monitor, state)

	seedBatchMetrics("node", nodeIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedBatchMetrics("dockerHost", dockerHostIDs, []string{"cpu", "memory", "disk"})
	seedBatchMetrics("agent", agentIDs, []string{"cpu", "memory", "disk"})

	router := &Router{monitor: monitor}
	url := "/api/charts/infrastructure?range=4h"

	sanityReq := httptest.NewRequest(http.MethodGet, url, nil)
	sanityRec := httptest.NewRecorder()
	router.handleInfrastructureCharts(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d body=%s", sanityRec.Code, sanityRec.Body.String())
	}
	var sanityResp InfrastructureChartsResponse
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &sanityResp); err != nil {
		t.Fatalf("sanity unmarshal: %v", err)
	}
	if len(sanityResp.NodeData) != nodeCount {
		t.Fatalf("sanity: expected %d nodes, got %d", nodeCount, len(sanityResp.NodeData))
	}
	if len(sanityResp.DockerHostData) != dockerHostCount {
		t.Fatalf("sanity: expected %d docker hosts, got %d", dockerHostCount, len(sanityResp.DockerHostData))
	}
	if len(sanityResp.AgentData) != agentCount {
		t.Fatalf("sanity: expected %d agents, got %d", agentCount, len(sanityResp.AgentData))
	}
	if sanityResp.Stats.PrimarySourceHint != "store_or_memory_fallback" {
		t.Fatalf("sanity: expected store-backed source hint, got %q", sanityResp.Stats.PrimarySourceHint)
	}
	if SLOWorkloadsSummaryChartsP95 != SLOWorkloadChartsP95 {
		t.Fatalf(
			"sanity: workloads-summary SLO=%v, want alignment with workload charts SLO=%v",
			SLOWorkloadsSummaryChartsP95,
			SLOWorkloadChartsP95,
		)
	}
	if sloWorkloadsSummaryChartsGitHubActionsP95 != sloWorkloadChartsGitHubActionsP95 {
		t.Fatalf(
			"sanity: workloads-summary GitHub Actions SLO=%v, want alignment with workload charts GitHub Actions SLO=%v",
			sloWorkloadsSummaryChartsGitHubActionsP95,
			sloWorkloadChartsGitHubActionsP95,
		)
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleInfrastructureCharts(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	target := effectiveAPISLOTarget(SLOInfrastructureChartsP95, sloInfrastructureChartsGitHubActionsP95)
	t.Logf("charts/infrastructure p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

// TestSLO_WorkloadCharts validates the workload charts endpoint that powers
// workload summary sparklines. The workload forces the store-backed batch path
// across VMs, system containers, and docker containers.
func TestSLO_WorkloadCharts(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newTestMetricsStore(t)
	const (
		vmCount           = 30
		containerCount    = 20
		dockerHostCount   = 10
		containersPerHost = 2
		pointsPerMetric   = 240
	)
	base := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)

	seedBatchMetrics := func(resourceType string, ids []string, metricTypes []string) {
		batch := make([]metrics.WriteMetric, 0, len(ids)*len(metricTypes)*pointsPerMetric)
		for idx, id := range ids {
			for _, mt := range metricTypes {
				for p := 0; p < pointsPerMetric; p++ {
					batch = append(batch, metrics.WriteMetric{
						ResourceType: resourceType,
						ResourceID:   id,
						MetricType:   mt,
						Value:        float64((idx + p) % 100),
						Timestamp:    base.Add(time.Duration(p) * time.Minute),
						Tier:         metrics.TierMinute,
					})
				}
			}
		}
		store.WriteBatchSync(batch)
	}

	monitor := &monitoring.Monitor{}
	state := models.NewState()
	setTestUnexportedField(t, monitor, "state", state)
	setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setTestUnexportedField(t, monitor, "metricsStore", store)

	nodes := make([]models.Node, 5)
	for i := range nodes {
		nodes[i] = models.Node{
			ID:       fmt.Sprintf("node-slo-%d", i),
			Name:     fmt.Sprintf("node-%d", i),
			Instance: "pve1",
			Status:   "online",
		}
	}
	state.UpdateNodesForInstance("pve1", nodes)

	vms := make([]models.VM, vmCount)
	vmIDs := make([]string, vmCount)
	for i := range vms {
		vmIDs[i] = fmt.Sprintf("vm-slo-%d", i)
		vms[i] = models.VM{
			ID:       vmIDs[i],
			VMID:     100 + i,
			Name:     fmt.Sprintf("vm-%d", i),
			Node:     nodes[i%len(nodes)].Name,
			Instance: "pve1",
			Status:   "running",
			Type:     "qemu",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 4 << 30, Used: 2 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 50 << 30, Used: 25 << 30},
		}
	}
	state.UpdateVMsForInstance("pve1", vms)

	containers := make([]models.Container, containerCount)
	containerIDs := make([]string, containerCount)
	for i := range containers {
		containerIDs[i] = fmt.Sprintf("ct-slo-%d", i)
		containers[i] = models.Container{
			ID:       containerIDs[i],
			VMID:     200 + i,
			Name:     fmt.Sprintf("ct-%d", i),
			Node:     nodes[i%len(nodes)].Name,
			Instance: "pve1",
			Status:   "running",
			Type:     "lxc",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 2 << 30, Used: 1 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 20 << 30, Used: 10 << 30},
		}
	}
	state.UpdateContainersForInstance("pve1", containers)

	dockerHosts := make([]models.DockerHost, dockerHostCount)
	dockerContainerIDs := make([]string, 0, dockerHostCount*containersPerHost)
	for i := range dockerHosts {
		hostID := fmt.Sprintf("docker-host-slo-%d", i)
		hostContainers := make([]models.DockerContainer, containersPerHost)
		for j := range hostContainers {
			containerID := fmt.Sprintf("docker-container-slo-%d-%d", i, j)
			dockerContainerIDs = append(dockerContainerIDs, containerID)
			hostContainers[j] = models.DockerContainer{
				ID:            containerID,
				Name:          fmt.Sprintf("docker-%d-%d", i, j),
				State:         "running",
				Status:        "running",
				CPUPercent:    float64((i+j)%80 + 10),
				MemoryPercent: float64((i+j)%60 + 20),
				NetInRate:     float64((i+j)%50 + 5),
				NetOutRate:    float64((i+j)%50 + 7),
			}
		}
		dockerHosts[i] = models.DockerHost{
			ID:         hostID,
			AgentID:    hostID,
			Hostname:   nodes[i%len(nodes)].Name,
			Runtime:    "docker",
			Status:     "online",
			CPUUsage:   float64(i%80 + 10),
			Memory:     models.Memory{Usage: float64(i%60 + 20), Total: 32 << 30, Used: 16 << 30},
			Disks:      []models.Disk{{Usage: float64(i%40 + 30), Total: 200 << 30, Used: 100 << 30}},
			Containers: hostContainers,
		}
	}
	state.DockerHosts = dockerHosts
	syncTestResourceStore(t, monitor, state)

	seedBatchMetrics("vm", vmIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedBatchMetrics("container", containerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedBatchMetrics("dockerContainer", dockerContainerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})

	router := &Router{monitor: monitor}
	url := "/api/charts/workloads?range=4h&maxPoints=120"

	sanityReq := httptest.NewRequest(http.MethodGet, url, nil)
	sanityRec := httptest.NewRecorder()
	router.handleWorkloadCharts(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d body=%s", sanityRec.Code, sanityRec.Body.String())
	}
	var sanityResp WorkloadChartsResponse
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &sanityResp); err != nil {
		t.Fatalf("sanity unmarshal: %v", err)
	}
	if len(sanityResp.ChartData) != vmCount+containerCount {
		t.Fatalf("sanity: expected %d guest chart entries, got %d", vmCount+containerCount, len(sanityResp.ChartData))
	}
	if len(sanityResp.DockerData) != len(dockerContainerIDs) {
		t.Fatalf("sanity: expected %d docker chart entries, got %d", len(dockerContainerIDs), len(sanityResp.DockerData))
	}
	if sanityResp.Stats.PrimarySourceHint != "store_or_memory_fallback" {
		t.Fatalf("sanity: expected store-backed source hint, got %q", sanityResp.Stats.PrimarySourceHint)
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleWorkloadCharts(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	target := effectiveAPISLOTarget(SLOWorkloadChartsP95, sloWorkloadChartsGitHubActionsP95)
	t.Logf("charts/workloads p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

// TestSLO_WorkloadsSummaryCharts validates the aggregate workload summary
// endpoint that powers top-card sparklines and blast-radius summaries. The
// workload forces the store-backed batch path across VMs, system containers,
// Kubernetes pods, and docker containers.
func TestSLO_WorkloadsSummaryCharts(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newTestMetricsStore(t)
	const (
		vmCount           = 30
		containerCount    = 20
		podCount          = 10
		dockerHostCount   = 10
		containersPerHost = 2
		pointsPerMetric   = 240
	)
	base := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)

	seedBatchMetrics := func(resourceType string, ids []string, metricTypes []string) {
		batch := make([]metrics.WriteMetric, 0, len(ids)*len(metricTypes)*pointsPerMetric)
		for idx, id := range ids {
			for _, mt := range metricTypes {
				for p := 0; p < pointsPerMetric; p++ {
					batch = append(batch, metrics.WriteMetric{
						ResourceType: resourceType,
						ResourceID:   id,
						MetricType:   mt,
						Value:        float64((idx + p) % 100),
						Timestamp:    base.Add(time.Duration(p) * time.Minute),
						Tier:         metrics.TierMinute,
					})
				}
			}
		}
		store.WriteBatchSync(batch)
	}

	monitor := &monitoring.Monitor{}
	state := models.NewState()
	setTestUnexportedField(t, monitor, "state", state)
	setTestUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setTestUnexportedField(t, monitor, "metricsStore", store)

	nodes := make([]models.Node, 5)
	for i := range nodes {
		nodes[i] = models.Node{
			ID:       fmt.Sprintf("node-summary-slo-%d", i),
			Name:     fmt.Sprintf("node-%d", i),
			Instance: "pve1",
			Status:   "online",
		}
	}
	state.UpdateNodesForInstance("pve1", nodes)

	vms := make([]models.VM, vmCount)
	vmIDs := make([]string, vmCount)
	for i := range vms {
		vmIDs[i] = fmt.Sprintf("vm-summary-slo-%d", i)
		vms[i] = models.VM{
			ID:       vmIDs[i],
			VMID:     100 + i,
			Name:     fmt.Sprintf("vm-%d", i),
			Node:     nodes[i%len(nodes)].Name,
			Instance: "pve1",
			Status:   "running",
			Type:     "qemu",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 4 << 30, Used: 2 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 50 << 30, Used: 25 << 30},
		}
	}
	state.UpdateVMsForInstance("pve1", vms)

	containers := make([]models.Container, containerCount)
	containerIDs := make([]string, containerCount)
	for i := range containers {
		containerIDs[i] = fmt.Sprintf("ct-summary-slo-%d", i)
		containers[i] = models.Container{
			ID:       containerIDs[i],
			VMID:     200 + i,
			Name:     fmt.Sprintf("ct-%d", i),
			Node:     nodes[i%len(nodes)].Name,
			Instance: "pve1",
			Status:   "running",
			Type:     "lxc",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 2 << 30, Used: 1 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 20 << 30, Used: 10 << 30},
		}
	}
	state.UpdateContainersForInstance("pve1", containers)

	clusters := []models.KubernetesCluster{{
		ID:   "k8s-summary-slo",
		Name: "k8s-summary-slo",
		Pods: make([]models.KubernetesPod, podCount),
	}}
	podIDs := make([]string, podCount)
	for i := 0; i < podCount; i++ {
		podIDs[i] = fmt.Sprintf("pod:%s:%s", "default", fmt.Sprintf("pod-%d", i))
		clusters[0].Pods[i] = models.KubernetesPod{
			UID:                fmt.Sprintf("pod-summary-slo-%d", i),
			Name:               fmt.Sprintf("pod-%d", i),
			Namespace:          "default",
			NodeName:           nodes[i%len(nodes)].Name,
			Phase:              "Running",
			UsageCPUPercent:    float64((i % 80) + 10),
			UsageMemoryPercent: float64((i % 60) + 20),
			DiskUsagePercent:   float64((i % 40) + 30),
			NetInRate:          float64((i % 50) + 5),
			NetOutRate:         float64((i % 50) + 7),
		}
	}
	state.KubernetesClusters = clusters

	dockerHosts := make([]models.DockerHost, dockerHostCount)
	dockerContainerIDs := make([]string, 0, dockerHostCount*containersPerHost)
	for i := range dockerHosts {
		hostID := fmt.Sprintf("docker-host-summary-slo-%d", i)
		hostContainers := make([]models.DockerContainer, containersPerHost)
		for j := range hostContainers {
			containerID := fmt.Sprintf("docker-container-summary-slo-%d-%d", i, j)
			dockerContainerIDs = append(dockerContainerIDs, containerID)
			hostContainers[j] = models.DockerContainer{
				ID:            containerID,
				Name:          fmt.Sprintf("docker-%d-%d", i, j),
				State:         "running",
				Status:        "running",
				CPUPercent:    float64((i+j)%80 + 10),
				MemoryPercent: float64((i+j)%60 + 20),
				NetInRate:     float64((i+j)%50 + 5),
				NetOutRate:    float64((i+j)%50 + 7),
			}
		}
		dockerHosts[i] = models.DockerHost{
			ID:         hostID,
			AgentID:    hostID,
			Hostname:   nodes[i%len(nodes)].Name,
			Runtime:    "docker",
			Status:     "online",
			CPUUsage:   float64(i%80 + 10),
			Memory:     models.Memory{Usage: float64(i%60 + 20), Total: 32 << 30, Used: 16 << 30},
			Disks:      []models.Disk{{Usage: float64(i%40 + 30), Total: 200 << 30, Used: 100 << 30}},
			Containers: hostContainers,
		}
	}
	state.DockerHosts = dockerHosts
	syncTestResourceStore(t, monitor, state)

	seedBatchMetrics("vm", vmIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedBatchMetrics("container", containerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedBatchMetrics("k8s", podIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedBatchMetrics("dockerContainer", dockerContainerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})

	router := &Router{monitor: monitor}
	url := "/api/charts/workloads-summary?range=4h"

	sanityReq := httptest.NewRequest(http.MethodGet, url, nil)
	sanityRec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(sanityRec, sanityReq)
	if sanityRec.Code != http.StatusOK {
		t.Fatalf("sanity check failed: status %d body=%s", sanityRec.Code, sanityRec.Body.String())
	}
	var sanityResp WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(sanityRec.Body.Bytes(), &sanityResp); err != nil {
		t.Fatalf("sanity unmarshal: %v", err)
	}
	if sanityResp.GuestCounts.Total != vmCount+containerCount+podCount+len(dockerContainerIDs) {
		t.Fatalf("sanity: expected %d guests, got %d", vmCount+containerCount+podCount+len(dockerContainerIDs), sanityResp.GuestCounts.Total)
	}
	if sanityResp.Stats.PrimarySourceHint != "store_or_memory_fallback" {
		t.Fatalf("sanity: expected store-backed source hint, got %q", sanityResp.Stats.PrimarySourceHint)
	}

	latencies := measureEndpointLatencies(t, func() {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleWorkloadsSummaryCharts(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
	})

	p95 := percentile(latencies, 0.95)
	target := effectiveAPISLOTarget(SLOWorkloadsSummaryChartsP95, sloWorkloadsSummaryChartsGitHubActionsP95)
	t.Logf("charts/workloads-summary p50=%v p95=%v p99=%v SLO=%v",
		percentile(latencies, 0.50), p95, percentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

// --- Test helpers ---

// skipUnderRace skips the test when the race detector is enabled, since the
// 2-10x overhead makes latency measurements meaningless.
func skipUnderRace(t *testing.T) {
	t.Helper()
	if raceEnabled {
		t.Skip("skipping SLO latency test under -race (overhead makes measurements unreliable)")
	}
}

const sloIterations = 200

func effectiveAPISLOTarget(localTarget, githubActionsTarget time.Duration) time.Duration {
	if githubActionsTarget > 0 && os.Getenv("GITHUB_ACTIONS") == "true" {
		return githubActionsTarget
	}
	return localTarget
}

// measureEndpointLatencies runs fn sloIterations times with a warmup phase and
// returns the measured latency durations.
func measureEndpointLatencies(t *testing.T, fn func()) []time.Duration {
	t.Helper()

	// Warmup: run 20 iterations to stabilize allocations and caches.
	for i := 0; i < 20; i++ {
		fn()
	}

	latencies := make([]time.Duration, sloIterations)
	for i := 0; i < sloIterations; i++ {
		start := time.Now()
		fn()
		latencies[i] = time.Since(start)
	}
	return latencies
}

// percentile returns the value at the given percentile (0.0–1.0) from
// a slice of durations.
func percentile(durations []time.Duration, pct float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * pct)
	return sorted[idx]
}

// newTestMetricsStore creates an ephemeral metrics store for SLO tests.
func newTestMetricsStore(t *testing.T) *metrics.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := metrics.DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "slo-test.db")
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = 10_000
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// seedTestMetrics writes test data to the store (mirrors seedBenchMetricsMulti).
func seedTestMetrics(t *testing.T, store *metrics.Store, resourceType string, metricTypes []string, numResources, numPoints int) []string {
	t.Helper()
	base := time.Now().Add(-50 * time.Minute)
	ids := make([]string, numResources)

	batch := make([]metrics.WriteMetric, 0, numResources*numPoints*len(metricTypes))
	for r := 0; r < numResources; r++ {
		id := fmt.Sprintf("%s-slo-%d", resourceType, r)
		ids[r] = id
		for _, mt := range metricTypes {
			for p := 0; p < numPoints; p++ {
				batch = append(batch, metrics.WriteMetric{
					ResourceType: resourceType,
					ResourceID:   id,
					MetricType:   mt,
					Value:        float64(p % 100),
					Timestamp:    base.Add(time.Duration(p) * 6 * time.Second),
					Tier:         metrics.TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)
	return ids
}

// sloTestStateProvider implements StateProvider for SLO tests.
type sloTestStateProvider struct {
	state *models.State
}

func (p *sloTestStateProvider) ReadSnapshot() models.StateSnapshot {
	return p.state.GetSnapshot()
}
