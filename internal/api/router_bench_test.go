package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// suppressBenchLogs disables zerolog for the duration of a benchmark to prevent
// log I/O from skewing results.
func suppressBenchLogs(b *testing.B) {
	b.Helper()
	orig := log.Logger
	log.Logger = zerolog.Nop()
	b.Cleanup(func() { log.Logger = orig })
}

// setBenchUnexportedField sets an unexported field on a struct via reflection.
func setBenchUnexportedField(b *testing.B, target interface{}, field string, value interface{}) {
	b.Helper()
	v := reflect.ValueOf(target).Elem()
	f := v.FieldByName(field)
	if !f.IsValid() {
		b.Fatalf("field %q not found", field)
	}
	ptr := unsafe.Pointer(f.UnsafeAddr())
	reflect.NewAt(f.Type(), ptr).Elem().Set(reflect.ValueOf(value))
}

// newBenchMetricsStore creates an ephemeral metrics store for benchmarks with
// background flushes disabled.
func newBenchMetricsStore(b *testing.B) *metrics.Store {
	b.Helper()
	dir := b.TempDir()
	cfg := metrics.DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "bench.db")
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = 10_000
	store, err := metrics.NewStore(cfg)
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	b.Cleanup(func() { store.Close() })
	return store
}

func syncBenchResourceStore(b *testing.B, monitor *monitoring.Monitor, state *models.State) {
	b.Helper()
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(state.GetSnapshot())
	setBenchUnexportedField(b, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))
}

// seedBenchMetricsMulti writes numResources × numPoints × len(metricTypes)
// metrics to the store. Data spans from now-50min to now, ensuring all points
// fall within a 1h query window.
func seedBenchMetricsMulti(b *testing.B, store *metrics.Store, resourceType string, metricTypes []string, numResources, numPoints int) []string {
	b.Helper()
	base := time.Now().Add(-50 * time.Minute)
	ids := make([]string, numResources)

	batch := make([]metrics.WriteMetric, 0, numResources*numPoints*len(metricTypes))
	for r := 0; r < numResources; r++ {
		id := fmt.Sprintf("%s-bench-%d", resourceType, r)
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

// BenchmarkHandleMetricsHistory_StoreQuery measures end-to-end latency of the
// /api/metrics-store/history handler when serving from the SQLite store — the
// primary hot path for chart rendering. Data is seeded within the last 50
// minutes so all points are returned by both 1h and 24h query windows.
func BenchmarkHandleMetricsHistory_StoreQuery(b *testing.B) {
	suppressBenchLogs(b)

	store := newBenchMetricsStore(b)
	const numPoints = 500
	metricTypes := []string{"cpu", "memory", "disk", "netin"}
	ids := seedBenchMetricsMulti(b, store, "vm", metricTypes, 10, numPoints)

	state := models.NewState()
	monitor := &monitoring.Monitor{}
	setBenchUnexportedField(b, monitor, "state", state)
	setBenchUnexportedField(b, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setBenchUnexportedField(b, monitor, "metricsStore", store)

	tempDir := b.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		b.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	// Sanity check: verify store path returns non-empty data.
	req := httptest.NewRequest(http.MethodGet,
		"/api/metrics-store/history?resourceType=vm&resourceId="+ids[0]+"&metric=cpu&range=1h", nil)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		b.Fatalf("sanity check failed: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	var check metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		b.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if check.Source != "store" {
		b.Fatalf("expected source store, got %q", check.Source)
	}
	if len(check.Points) == 0 {
		b.Fatalf("sanity check: expected non-empty points from store, got 0")
	}

	// Precompute per-resource URLs to avoid string building in timed loops.
	urls1h := make([]string, len(ids))
	urls24h := make([]string, len(ids))
	urlsMaxPts := make([]string, len(ids))
	urlsAll := make([]string, len(ids))
	for i, id := range ids {
		urls1h[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&metric=cpu&range=1h"
		urls24h[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&metric=cpu&range=24h"
		urlsMaxPts[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&metric=cpu&range=1h&maxPoints=100"
		urlsAll[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + id + "&range=1h"
	}

	b.Run("single-metric-1h", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, urls1h[i%len(ids)], nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("single-metric-24h", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, urls24h[i%len(ids)], nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("single-metric-maxPoints-100", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, urlsMaxPts[i%len(ids)], nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("all-metrics-1h", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, urlsAll[i%len(ids)], nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})
}

// BenchmarkHandleMetricsHistory_MemoryFallback measures latency of the
// in-memory fallback path (used when the SQLite store has no data for the
// requested resource). This path falls through to MetricsHistory and then to
// live state.
func BenchmarkHandleMetricsHistory_MemoryFallback(b *testing.B) {
	suppressBenchLogs(b)

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
	setBenchUnexportedField(b, monitor, "state", state)
	setBenchUnexportedField(b, monitor, "metricsHistory", mh)

	router := &Router{monitor: monitor}

	// Sanity check: verify memory fallback path returns non-empty data.
	req := httptest.NewRequest(http.MethodGet,
		"/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:100&metric=cpu&range=1h", nil)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		b.Fatalf("sanity check failed: status %d, body: %s", rec.Code, rec.Body.String())
	}
	var check metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		b.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if len(check.Points) == 0 {
		b.Fatalf("sanity check: expected non-empty points from memory fallback, got 0")
	}

	// Precompute URLs.
	vmURLsSingle := make([]string, 10)
	vmURLsAll := make([]string, 10)
	for i := 0; i < 10; i++ {
		vmID := fmt.Sprintf("pve1:node1:%d", 100+i)
		vmURLsSingle[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + vmID + "&metric=cpu&range=1h"
		vmURLsAll[i] = "/api/metrics-store/history?resourceType=vm&resourceId=" + vmID + "&range=1h"
	}

	b.Run("single-metric", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, vmURLsSingle[i%10], nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("all-metrics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, vmURLsAll[i%10], nil)
			rec := httptest.NewRecorder()
			router.handleMetricsHistory(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})
}

// BenchmarkHandleWorkloadCharts_StoreBacked measures the workloads charts
// endpoint under a store-backed 4h range. This is the workloads summary
// sparkline hot path and should avoid per-workload N+1 queries.
func BenchmarkHandleWorkloadCharts_StoreBacked(b *testing.B) {
	suppressBenchLogs(b)

	store := newBenchMetricsStore(b)
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	setBenchUnexportedField(b, monitor, "state", state)
	setBenchUnexportedField(b, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setBenchUnexportedField(b, monitor, "metricsStore", store)

	const (
		vmCount           = 30
		containerCount    = 20
		dockerHostCount   = 10
		containersPerHost = 2
		pointsPerMetric   = 240
	)
	base := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)

	nodes := make([]models.Node, 5)
	for i := range nodes {
		nodes[i] = models.Node{
			ID:       fmt.Sprintf("node-bench-%d", i),
			Name:     fmt.Sprintf("node-%d", i),
			Instance: "pve1",
			Status:   "online",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 64 << 30, Used: 32 << 30},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 500 << 30, Used: 250 << 30},
		}
	}
	state.UpdateNodesForInstance("pve1", nodes)

	vms := make([]models.VM, vmCount)
	vmIDs := make([]string, vmCount)
	for i := range vms {
		vmIDs[i] = fmt.Sprintf("vm-bench-%d", i)
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
		containerIDs[i] = fmt.Sprintf("ct-bench-%d", i)
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
		hostID := fmt.Sprintf("docker-host-bench-%d", i)
		hostContainers := make([]models.DockerContainer, containersPerHost)
		for j := range hostContainers {
			containerID := fmt.Sprintf("docker-container-bench-%d-%d", i, j)
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
	syncBenchResourceStore(b, monitor, state)

	seedSeries := func(resourceType string, ids []string, metricTypes []string) {
		batch := make([]metrics.WriteMetric, 0, len(ids)*len(metricTypes)*pointsPerMetric)
		for r, id := range ids {
			for _, mt := range metricTypes {
				for p := 0; p < pointsPerMetric; p++ {
					batch = append(batch, metrics.WriteMetric{
						ResourceType: resourceType,
						ResourceID:   id,
						MetricType:   mt,
						Value:        float64((r + p) % 100),
						Timestamp:    base.Add(time.Duration(p) * time.Minute),
						Tier:         metrics.TierMinute,
					})
				}
			}
		}
		store.WriteBatchSync(batch)
	}

	seedSeries("vm", vmIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedSeries("container", containerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedSeries("dockerContainer", dockerContainerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})

	router := &Router{monitor: monitor}
	url := "/api/charts/workloads?range=4h&maxPoints=120"

	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)
	if rec.Code != http.StatusOK {
		b.Fatalf("sanity check failed: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	var check WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		b.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if len(check.ChartData) != vmCount+containerCount {
		b.Fatalf("expected %d guest chart entries, got %d", vmCount+containerCount, len(check.ChartData))
	}
	if len(check.DockerData) != len(dockerContainerIDs) {
		b.Fatalf("expected %d docker chart entries, got %d", len(dockerContainerIDs), len(check.DockerData))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleWorkloadCharts(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rec.Code)
		}
	}
}

// BenchmarkHandleWorkloadsSummaryCharts_StoreBacked measures the workloads
// summary endpoint under a store-backed 4h range. This path should also avoid
// per-workload N+1 queries while computing aggregate p50/p95 sparklines.
func BenchmarkHandleWorkloadsSummaryCharts_StoreBacked(b *testing.B) {
	suppressBenchLogs(b)

	store := newBenchMetricsStore(b)
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	setBenchUnexportedField(b, monitor, "state", state)
	setBenchUnexportedField(b, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setBenchUnexportedField(b, monitor, "metricsStore", store)

	const (
		vmCount           = 30
		containerCount    = 20
		dockerHostCount   = 10
		containersPerHost = 2
		pointsPerMetric   = 240
	)
	base := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)

	nodes := make([]models.Node, 5)
	for i := range nodes {
		nodes[i] = models.Node{
			ID:       fmt.Sprintf("node-summary-bench-%d", i),
			Name:     fmt.Sprintf("node-%d", i),
			Instance: "pve1",
			Status:   "online",
		}
	}
	state.UpdateNodesForInstance("pve1", nodes)

	vms := make([]models.VM, vmCount)
	vmIDs := make([]string, vmCount)
	for i := range vms {
		vmIDs[i] = fmt.Sprintf("vm-summary-bench-%d", i)
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
		containerIDs[i] = fmt.Sprintf("ct-summary-bench-%d", i)
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
		hostID := fmt.Sprintf("docker-host-summary-bench-%d", i)
		hostContainers := make([]models.DockerContainer, containersPerHost)
		for j := range hostContainers {
			containerID := fmt.Sprintf("docker-container-summary-bench-%d-%d", i, j)
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
	syncBenchResourceStore(b, monitor, state)

	seedSeries := func(resourceType string, ids []string, metricTypes []string) {
		batch := make([]metrics.WriteMetric, 0, len(ids)*len(metricTypes)*pointsPerMetric)
		for r, id := range ids {
			for _, mt := range metricTypes {
				for p := 0; p < pointsPerMetric; p++ {
					batch = append(batch, metrics.WriteMetric{
						ResourceType: resourceType,
						ResourceID:   id,
						MetricType:   mt,
						Value:        float64((r + p) % 100),
						Timestamp:    base.Add(time.Duration(p) * time.Minute),
						Tier:         metrics.TierMinute,
					})
				}
			}
		}
		store.WriteBatchSync(batch)
	}

	seedSeries("vm", vmIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedSeries("container", containerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})
	seedSeries("dockerContainer", dockerContainerIDs, []string{"cpu", "memory", "disk", "netin", "netout"})

	router := &Router{monitor: monitor}
	url := "/api/charts/workloads-summary?range=4h"

	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(rec, req)
	if rec.Code != http.StatusOK {
		b.Fatalf("sanity check failed: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	var check WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		b.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if check.GuestCounts.Total != vmCount+containerCount+len(dockerContainerIDs) {
		b.Fatalf("expected %d total guests, got %d", vmCount+containerCount+len(dockerContainerIDs), check.GuestCounts.Total)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		router.handleWorkloadsSummaryCharts(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rec.Code)
		}
	}
}

// BenchmarkHandleMetricsStoreStats measures latency of the /api/metrics-store/stats
// endpoint, which performs lightweight SQLite stat queries.
func BenchmarkHandleMetricsStoreStats(b *testing.B) {
	suppressBenchLogs(b)

	store := newBenchMetricsStore(b)
	seedBenchMetricsMulti(b, store, "node", []string{"cpu"}, 5, 100)

	monitor := &monitoring.Monitor{}
	setBenchUnexportedField(b, monitor, "state", models.NewState())
	setBenchUnexportedField(b, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))
	setBenchUnexportedField(b, monitor, "metricsStore", store)

	router := &Router{monitor: monitor}

	// Sanity check.
	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/stats", nil)
	rec := httptest.NewRecorder()
	router.handleMetricsStoreStats(rec, req)
	if rec.Code != http.StatusOK {
		b.Fatalf("sanity check failed: status %d", rec.Code)
	}
	var statsCheck map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &statsCheck); err != nil {
		b.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	if enabled, _ := statsCheck["enabled"].(bool); !enabled {
		b.Fatalf("sanity check: expected enabled=true")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/stats", nil)
		rec := httptest.NewRecorder()
		router.handleMetricsStoreStats(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rec.Code)
		}
	}
}

// BenchmarkHandleListResources measures end-to-end latency of the GET
// /api/resources handler with a pre-populated registry. After the first
// request builds the registry, subsequent requests serve from cache (the
// production-realistic hot path). This covers: cache lookup, snapshot
// comparison, filtering, sorting, pagination, pruning, and JSON serialization.
func BenchmarkHandleListResources(b *testing.B) {
	suppressBenchLogs(b)

	state := models.NewState()
	nodes := make([]models.Node, 5)
	for i := range nodes {
		nodes[i] = models.Node{
			ID:       fmt.Sprintf("pve1:node%d", i),
			Name:     fmt.Sprintf("node%d", i),
			Instance: "pve1",
			Status:   "online",
			CPU:      float64(i%80+10) / 100.0,
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 64 * 1024 * 1024 * 1024, Used: 32 * 1024 * 1024 * 1024},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 500 * 1024 * 1024 * 1024, Used: 250 * 1024 * 1024 * 1024},
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
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 4 * 1024 * 1024 * 1024, Used: 2 * 1024 * 1024 * 1024},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 50 * 1024 * 1024 * 1024, Used: 25 * 1024 * 1024 * 1024},
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
			Memory:   models.Memory{Usage: float64(i%60 + 20), Total: 2 * 1024 * 1024 * 1024, Used: 1 * 1024 * 1024 * 1024},
			Disk:     models.Disk{Usage: float64(i%40 + 30), Total: 20 * 1024 * 1024 * 1024, Used: 10 * 1024 * 1024 * 1024},
		}
	}
	state.UpdateContainersForInstance("pve1", containers)

	tempDir := b.TempDir()
	cfg := &config.Config{DataPath: tempDir}
	handlers := NewResourceHandlers(cfg)
	handlers.SetStateProvider(&benchStateProvider{state: state})

	// Sanity check: verify handler returns non-empty resource data.
	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rec := httptest.NewRecorder()
	handlers.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		b.Fatalf("sanity check failed: status %d, body: %s", rec.Code, rec.Body.String())
	}
	var checkResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &checkResp); err != nil {
		b.Fatalf("sanity check: unmarshal failed: %v", err)
	}
	data, _ := checkResp["data"].([]interface{})
	if len(data) == 0 {
		b.Fatalf("sanity check: expected resources in response, got none")
	}

	b.Run("unfiltered", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
			rec := httptest.NewRecorder()
			handlers.HandleListResources(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("filtered-type-vm", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/resources?type=vm", nil)
			rec := httptest.NewRecorder()
			handlers.HandleListResources(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("paginated-page1-limit20", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/resources?page=1&limit=20", nil)
			rec := httptest.NewRecorder()
			handlers.HandleListResources(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})

	b.Run("sorted-by-cpu", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/resources?sort=cpu&order=desc", nil)
			rec := httptest.NewRecorder()
			handlers.HandleListResources(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("unexpected status %d", rec.Code)
			}
		}
	})
}

// benchStateProvider implements StateProvider for benchmarks by returning a
// snapshot of the given State.
type benchStateProvider struct {
	state *models.State
}

func (p *benchStateProvider) ReadSnapshot() models.StateSnapshot {
	return p.state.GetSnapshot()
}
