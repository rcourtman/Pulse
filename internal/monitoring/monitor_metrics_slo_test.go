package monitoring

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Monitoring-layer SLO targets for batched chart retrieval.
//
// These protect the chart aggregation layer above the raw metrics store. The
// store benchmarks already validate SQL performance; these tests cover the
// additional alias resolution, gap-fill retry, conversion, and downsampling
// work that powers /api/charts.
//
// Baseline measurements (Apple M4, March 2026):
//   - GetGuestMetricsForChartBatch(50 guests × 5 metrics × 240 points): ~42ms
//   - GetNodeMetricsForChartBatch(20 nodes × 5 metrics × 240 points):  ~16ms
//   - GitHub-hosted runners on the April 9, 2026 RC dry run reached ~337ms
//     and ~153ms p95 respectively, so CI keeps separate hosted-runner budgets
//     while preserving strict local thresholds.
const (
	SLOGuestChartBatchP95              = 80 * time.Millisecond
	SLONodeChartBatchP95               = 35 * time.Millisecond
	SLOGuestChartBatchGitHubActionsP95 = 400 * time.Millisecond
	SLONodeChartBatchGitHubActionsP95  = 180 * time.Millisecond
	SLOPhysicalDiskChartFallbackP95    = 30 * time.Millisecond
	SLOPhysicalDiskChartFallbackGHA    = 120 * time.Millisecond
	SLOGuestChartFallbackP95           = 30 * time.Millisecond
	SLOGuestChartFallbackGHA           = 120 * time.Millisecond

	monitoringSLOIterations = 120
)

func skipMonitoringSLOUnderRace(t *testing.T) {
	t.Helper()
	if raceEnabled {
		t.Skip("skipping monitoring SLO latency test under -race")
	}
}

func suppressMonitoringTestLogs(t *testing.T) {
	t.Helper()
	orig := log.Logger
	log.Logger = zerolog.Nop()
	t.Cleanup(func() { log.Logger = orig })
}

func newChartBatchSLOMonitor(t *testing.T) *Monitor {
	t.Helper()

	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 90 * 24 * time.Hour
	cfg.RetentionMinute = 90 * 24 * time.Hour
	cfg.RetentionHourly = 90 * 24 * time.Hour
	cfg.RetentionDaily = 90 * 24 * time.Hour
	cfg.WriteBufferSize = 10_000
	cfg.FlushInterval = time.Hour

	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return &Monitor{
		metricsHistory: NewMetricsHistory(4096, 24*time.Hour),
		metricsStore:   store,
	}
}

func measureMonitoringLatencies(t *testing.T, fn func()) []time.Duration {
	t.Helper()
	for i := 0; i < 20; i++ {
		fn()
	}

	latencies := make([]time.Duration, monitoringSLOIterations)
	for i := 0; i < monitoringSLOIterations; i++ {
		start := time.Now()
		fn()
		latencies[i] = time.Since(start)
	}
	return latencies
}

func monitoringPercentile(durations []time.Duration, pct float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * pct)
	return sorted[idx]
}

func effectiveMonitoringSLOTarget(localTarget, githubActionsTarget time.Duration) time.Duration {
	if githubActionsTarget > 0 && os.Getenv("GITHUB_ACTIONS") == "true" {
		return githubActionsTarget
	}
	return localTarget
}

func seedSLOGuestMetrics(t *testing.T, store *metrics.Store, resourceType string, numResources, numPoints int) []string {
	t.Helper()

	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	now := time.Now().UTC().Truncate(time.Second)
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
					Value:        float64(p%100) + float64(r%10),
					Timestamp:    now.Add(-time.Duration(numPoints-p) * time.Minute),
					Tier:         metrics.TierMinute,
				})
			}
		}
	}
	store.WriteBatchSync(batch)
	return ids
}

func seedSLONodeMetrics(t *testing.T, store *metrics.Store, numNodes, numPoints int) []string {
	t.Helper()

	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	now := time.Now().UTC().Truncate(time.Second)
	ids := make([]string, numNodes)

	batch := make([]metrics.WriteMetric, 0, numNodes*numPoints*len(metricTypes))
	for n := 0; n < numNodes; n++ {
		id := fmt.Sprintf("node-slo-%d", n)
		ids[n] = id
		for _, mt := range metricTypes {
			for p := 0; p < numPoints; p++ {
				batch = append(batch, metrics.WriteMetric{
					ResourceType: "node",
					ResourceID:   id,
					MetricType:   mt,
					Value:        float64(p%100) + float64(n%10),
					Timestamp:    now.Add(-time.Duration(numPoints-p) * time.Minute),
					Tier:         metrics.TierMinute,
				})
			}
		}
	}
	store.WriteBatchSync(batch)
	return ids
}

func TestSLO_GetGuestMetricsForChartBatch(t *testing.T) {
	skipMonitoringSLOUnderRace(t)
	suppressMonitoringTestLogs(t)

	monitor := newChartBatchSLOMonitor(t)
	duration := 4 * time.Hour
	ids := seedSLOGuestMetrics(t, monitor.metricsStore, "container", 50, 240)
	requests := make([]GuestChartRequest, len(ids))
	for i, id := range ids {
		requests[i] = GuestChartRequest{InMemoryKey: id, SQLResourceID: id}
	}

	sanity := monitor.GetGuestMetricsForChartBatch("container", requests, duration)
	if len(sanity) != len(ids) {
		t.Fatalf("sanity: expected %d guests, got %d", len(ids), len(sanity))
	}
	for _, id := range ids {
		if cpuPts := sanity[id]["cpu"]; len(cpuPts) == 0 {
			t.Fatalf("sanity: guest %s has no cpu points", id)
		}
	}

	latencies := measureMonitoringLatencies(t, func() {
		result := monitor.GetGuestMetricsForChartBatch("container", requests, duration)
		if len(result) != len(ids) {
			t.Fatalf("expected %d guests, got %d", len(ids), len(result))
		}
	})

	target := effectiveMonitoringSLOTarget(SLOGuestChartBatchP95, SLOGuestChartBatchGitHubActionsP95)
	p95 := monitoringPercentile(latencies, 0.95)
	t.Logf("GetGuestMetricsForChartBatch(50×5×240) p50=%v p95=%v p99=%v SLO=%v",
		monitoringPercentile(latencies, 0.50), p95, monitoringPercentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

func TestSLO_GetNodeMetricsForChartBatch(t *testing.T) {
	skipMonitoringSLOUnderRace(t)
	suppressMonitoringTestLogs(t)

	monitor := newChartBatchSLOMonitor(t)
	duration := 4 * time.Hour
	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	ids := seedSLONodeMetrics(t, monitor.metricsStore, 20, 240)

	sanity := monitor.GetNodeMetricsForChartBatch(ids, metricTypes, duration)
	if len(sanity) != len(ids) {
		t.Fatalf("sanity: expected %d nodes, got %d", len(ids), len(sanity))
	}
	for _, id := range ids {
		if cpuPts := sanity[id]["cpu"]; len(cpuPts) == 0 {
			t.Fatalf("sanity: node %s has no cpu points", id)
		}
	}

	latencies := measureMonitoringLatencies(t, func() {
		result := monitor.GetNodeMetricsForChartBatch(ids, metricTypes, duration)
		if len(result) != len(ids) {
			t.Fatalf("expected %d nodes, got %d", len(ids), len(result))
		}
	})

	target := effectiveMonitoringSLOTarget(SLONodeChartBatchP95, SLONodeChartBatchGitHubActionsP95)
	p95 := monitoringPercentile(latencies, 0.95)
	t.Logf("GetNodeMetricsForChartBatch(20×5×240) p50=%v p95=%v p99=%v SLO=%v",
		monitoringPercentile(latencies, 0.50), p95, monitoringPercentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

func TestSLO_GetPhysicalDiskTemperatureCharts_WithNativeHistoryFallback(t *testing.T) {
	skipMonitoringSLOUnderRace(t)
	suppressMonitoringTestLogs(t)

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "disk-1",
				Node:        "truenas-main",
				DevPath:     "/dev/sda",
				Model:       "Seagate Exos X18",
				Serial:      "SERIAL-DISK-1",
				Temperature: 34,
				LastChecked: now,
			},
		},
	})
	monitor.resourceStore = unifiedresources.NewMonitorAdapter(registry)
	monitor.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: &stubDiskTemperatureHistoryProvider{
			history: map[string][]MetricPoint{
				"SERIAL-DISK-1": {
					{Timestamp: now.Add(-2 * time.Hour), Value: 29},
					{Timestamp: now.Add(-1 * time.Hour), Value: 31},
					{Timestamp: now, Value: 34},
				},
			},
		},
	}

	sanity := monitor.GetPhysicalDiskTemperatureCharts(4 * time.Hour)
	entry, ok := sanity["SERIAL-DISK-1"]
	if !ok {
		t.Fatalf("sanity: expected chart entry for canonical disk metric id, got %#v", sanity)
	}
	if len(entry.Temperature) != 3 {
		t.Fatalf("sanity: expected native history points instead of padded fallback, got %+v", entry.Temperature)
	}

	latencies := measureMonitoringLatencies(t, func() {
		result := monitor.GetPhysicalDiskTemperatureCharts(4 * time.Hour)
		entry, ok := result["SERIAL-DISK-1"]
		if !ok {
			t.Fatalf("expected chart entry for canonical disk metric id, got %#v", result)
		}
		if len(entry.Temperature) != 3 {
			t.Fatalf("expected native history points instead of padded fallback, got %+v", entry.Temperature)
		}
	})

	target := effectiveMonitoringSLOTarget(SLOPhysicalDiskChartFallbackP95, SLOPhysicalDiskChartFallbackGHA)
	p95 := monitoringPercentile(latencies, 0.95)
	t.Logf("GetPhysicalDiskTemperatureCharts(native-history fallback) p50=%v p95=%v p99=%v SLO=%v",
		monitoringPercentile(latencies, 0.50), p95, monitoringPercentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

func TestSLO_GetDiskMetricsForChart_WithNativeStoreFallback(t *testing.T) {
	skipMonitoringSLOUnderRace(t)
	suppressMonitoringTestLogs(t)

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour

	writeRawMetricBatch(t, monitor.metricsStore, "disk", "SERIAL-DISK-IO-1", "diskread", []MetricPoint{
		{Timestamp: now.Add(-2 * time.Hour), Value: 1.25 * 1024 * 1024},
		{Timestamp: now.Add(-1 * time.Hour), Value: 2.5 * 1024 * 1024},
		{Timestamp: now, Value: 3.75 * 1024 * 1024},
	})

	sanity := monitor.GetDiskMetricsForChart("SERIAL-DISK-IO-1", "diskread", duration)
	if got := len(sanity); got != 3 {
		t.Fatalf("sanity: expected native diskread history, got %+v", sanity)
	}

	latencies := measureMonitoringLatencies(t, func() {
		result := monitor.GetDiskMetricsForChart("SERIAL-DISK-IO-1", "diskread", duration)
		if got := len(result); got != 3 {
			t.Fatalf("expected native diskread history, got %+v", result)
		}
	})

	target := effectiveMonitoringSLOTarget(SLOPhysicalDiskChartFallbackP95, SLOPhysicalDiskChartFallbackGHA)
	p95 := monitoringPercentile(latencies, 0.95)
	t.Logf("GetDiskMetricsForChart(native-store fallback) p50=%v p95=%v p99=%v SLO=%v",
		monitoringPercentile(latencies, 0.50), p95, monitoringPercentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

func TestSLO_GetGuestMetricsForChart_WithNativeHistoryFallback(t *testing.T) {
	skipMonitoringSLOUnderRace(t)
	suppressMonitoringTestLogs(t)

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour

	writeRawMetricBatch(t, monitor.metricsStore, "agent", "truenas-main", "cpu", []MetricPoint{
		{Timestamp: now.Add(-5 * time.Minute), Value: 41},
	})
	monitor.supplementalProviders = map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider{
		unifiedresources.SourceTrueNAS: &stubGuestMetricHistoryProvider{
			history: map[string]map[string][]MetricPoint{
				"truenas-main": {
					"cpu": {
						{Timestamp: now.Add(-2 * time.Hour), Value: 20},
						{Timestamp: now.Add(-1 * time.Hour), Value: 28},
						{Timestamp: now, Value: 34},
					},
					"memory": {
						{Timestamp: now.Add(-2 * time.Hour), Value: 45},
						{Timestamp: now, Value: 62},
					},
				},
			},
		},
	}

	sanity := monitor.GetGuestMetricsForChart("agent:truenas-main", "agent", "truenas-main", duration)
	if got := len(sanity["cpu"]); got != 3 {
		t.Fatalf("sanity: expected native cpu history, got %+v", sanity["cpu"])
	}

	latencies := measureMonitoringLatencies(t, func() {
		result := monitor.GetGuestMetricsForChart("agent:truenas-main", "agent", "truenas-main", duration)
		if got := len(result["cpu"]); got != 3 {
			t.Fatalf("expected native cpu history, got %+v", result["cpu"])
		}
		if got := len(result["memory"]); got != 2 {
			t.Fatalf("expected native memory history, got %+v", result["memory"])
		}
	})

	target := effectiveMonitoringSLOTarget(SLOGuestChartFallbackP95, SLOGuestChartFallbackGHA)
	p95 := monitoringPercentile(latencies, 0.95)
	t.Logf("GetGuestMetricsForChart(native-history fallback) p50=%v p95=%v p99=%v SLO=%v",
		monitoringPercentile(latencies, 0.50), p95, monitoringPercentile(latencies, 0.99), target)

	if p95 > target {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, target)
	}
}

func TestSLO_GetGuestMetricsForChartBatch_DoesNotStitchSparseStoreTailOntoCoveredInMemorySeries(t *testing.T) {
	previous := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(previous) })

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	inMemorySeries := []MetricPoint{
		{Timestamp: now.Add(-58 * time.Minute), Value: 74},
		{Timestamp: now.Add(-36 * time.Minute), Value: 75},
		{Timestamp: now.Add(-14 * time.Minute), Value: 77},
		{Timestamp: now.Add(-2 * time.Minute), Value: 78},
	}
	for _, point := range inMemorySeries {
		monitor.metricsHistory.AddGuestMetric("vm-1", "memory", point.Value, point.Timestamp)
	}

	// This sparse late store point models the old stitched-tail behavior. In
	// mock mode the batch chart path must ignore persisted history entirely and
	// serve the canonical sampler timeline instead.
	writeRawMetricBatch(t, monitor.metricsStore, "vm", "vm-1", "memory", []MetricPoint{
		{Timestamp: now.Add(-1 * time.Minute), Value: 21},
	})

	result := monitor.GetGuestMetricsForChartBatch("vm", []GuestChartRequest{{
		InMemoryKey:   "vm-1",
		SQLResourceID: "vm-1",
	}}, duration)

	memoryPoints := result["vm-1"]["memory"]
	if len(memoryPoints) < 2 {
		t.Fatalf("expected canonical mock memory series, got %+v", memoryPoints)
	}
	checkIndices := []int{0, len(memoryPoints) / 2, len(memoryPoints) - 1}
	seen := map[int]struct{}{}
	for _, idx := range checkIndices {
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}

		want := mock.SampleMetric("vm", "vm-1", "memory", memoryPoints[idx].Timestamp)
		if memoryPoints[idx].Value != want {
			t.Fatalf("memoryPoints[%d] = %+v, want canonical %.12f", idx, memoryPoints[idx], want)
		}
	}
}
