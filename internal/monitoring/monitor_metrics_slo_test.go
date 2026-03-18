package monitoring

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

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
const (
	SLOGuestChartBatchP95              = 80 * time.Millisecond
	SLONodeChartBatchP95               = 35 * time.Millisecond
	SLOGuestChartBatchGitHubActionsP95 = 220 * time.Millisecond
	SLONodeChartBatchGitHubActionsP95  = 140 * time.Millisecond

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
