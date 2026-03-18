package monitoring

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// newChartBatchBenchMonitor creates a Monitor with a real SQLite metrics store
// for benchmarking batch chart query paths. Each call creates an isolated
// database to avoid cross-benchmark data collisions.
func newChartBatchBenchMonitor(b *testing.B) *Monitor {
	b.Helper()

	orig := log.Logger
	log.Logger = zerolog.Nop()
	b.Cleanup(func() { log.Logger = orig })

	cfg := metrics.DefaultConfig(b.TempDir())
	cfg.RetentionRaw = 90 * 24 * time.Hour
	cfg.RetentionMinute = 90 * 24 * time.Hour
	cfg.RetentionHourly = 90 * 24 * time.Hour
	cfg.RetentionDaily = 90 * 24 * time.Hour
	cfg.WriteBufferSize = 10_000
	cfg.FlushInterval = time.Hour // disable background flushes

	store, err := metrics.NewStore(cfg)
	if err != nil {
		b.Fatalf("failed to create metrics store: %v", err)
	}
	b.Cleanup(func() { _ = store.Close() })

	return &Monitor{
		metricsHistory: NewMetricsHistory(4096, 24*time.Hour),
		metricsStore:   store,
	}
}

// seedBenchGuestMetrics writes numResources guests × numPoints per metric type
// into the store at TierMinute (the primary tier for 4h queries). Returns the
// resource IDs.
func seedBenchGuestMetrics(b *testing.B, store *metrics.Store, resourceType string, numResources, numPoints int) []string {
	b.Helper()

	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	now := time.Now().UTC().Truncate(time.Second)
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

// seedBenchNodeMetrics writes numNodes × numPoints per metric type into the
// store at TierMinute (the primary tier for 4h queries). Returns the node IDs.
func seedBenchNodeMetrics(b *testing.B, store *metrics.Store, numNodes, numPoints int) []string {
	b.Helper()

	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	now := time.Now().UTC().Truncate(time.Second)
	ids := make([]string, numNodes)

	batch := make([]metrics.WriteMetric, 0, numNodes*numPoints*len(metricTypes))
	for n := 0; n < numNodes; n++ {
		id := fmt.Sprintf("node-bench-%d", n)
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

// BenchmarkGetGuestMetricsForChartBatch measures the batch guest chart query
// path that replaced the per-resource N+1 pattern. The benchmark uses a
// duration beyond inMemoryChartThreshold to force the store batch path.
func BenchmarkGetGuestMetricsForChartBatch(b *testing.B) {
	duration := 4 * time.Hour // beyond inMemoryChartThreshold → forces store path

	b.Run("10-guests", func(b *testing.B) {
		monitor := newChartBatchBenchMonitor(b)
		ids := seedBenchGuestMetrics(b, monitor.metricsStore, "vm", 10, 240)
		requests := make([]GuestChartRequest, len(ids))
		for i, id := range ids {
			requests[i] = GuestChartRequest{InMemoryKey: id, SQLResourceID: id}
		}

		// Sanity: verify store path returns real data before timing loop.
		sanity := monitor.GetGuestMetricsForChartBatch("vm", requests, duration)
		for _, id := range ids {
			if cpuPts := sanity[id]["cpu"]; len(cpuPts) == 0 {
				b.Fatalf("sanity: guest %s has no cpu points from store", id)
			}
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := monitor.GetGuestMetricsForChartBatch("vm", requests, duration)
			if len(result) != len(ids) {
				b.Fatalf("expected %d results, got %d", len(ids), len(result))
			}
		}
	})

	b.Run("50-guests", func(b *testing.B) {
		monitor := newChartBatchBenchMonitor(b)
		ids := seedBenchGuestMetrics(b, monitor.metricsStore, "container", 50, 240)
		requests := make([]GuestChartRequest, len(ids))
		for i, id := range ids {
			requests[i] = GuestChartRequest{InMemoryKey: id, SQLResourceID: id}
		}

		// Sanity: verify store path returns real data before timing loop.
		sanity := monitor.GetGuestMetricsForChartBatch("container", requests, duration)
		for _, id := range ids {
			if cpuPts := sanity[id]["cpu"]; len(cpuPts) == 0 {
				b.Fatalf("sanity: guest %s has no cpu points from store", id)
			}
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := monitor.GetGuestMetricsForChartBatch("container", requests, duration)
			if len(result) != len(ids) {
				b.Fatalf("expected %d results, got %d", len(ids), len(result))
			}
		}
	})
}

// BenchmarkGetNodeMetricsForChartBatch measures the batch node chart query
// path that replaced the per-node per-metric N×5 pattern.
func BenchmarkGetNodeMetricsForChartBatch(b *testing.B) {
	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	duration := 4 * time.Hour

	b.Run("5-nodes", func(b *testing.B) {
		monitor := newChartBatchBenchMonitor(b)
		ids := seedBenchNodeMetrics(b, monitor.metricsStore, 5, 240)

		// Sanity: verify store path returns real data before timing loop.
		sanity := monitor.GetNodeMetricsForChartBatch(ids, metricTypes, duration)
		for _, id := range ids {
			if cpuPts := sanity[id]["cpu"]; len(cpuPts) == 0 {
				b.Fatalf("sanity: node %s has no cpu points from store", id)
			}
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := monitor.GetNodeMetricsForChartBatch(ids, metricTypes, duration)
			if len(result) != len(ids) {
				b.Fatalf("expected %d results, got %d", len(ids), len(result))
			}
		}
	})

	b.Run("20-nodes", func(b *testing.B) {
		monitor := newChartBatchBenchMonitor(b)
		ids := seedBenchNodeMetrics(b, monitor.metricsStore, 20, 240)

		// Sanity: verify store path returns real data before timing loop.
		sanity := monitor.GetNodeMetricsForChartBatch(ids, metricTypes, duration)
		for _, id := range ids {
			if cpuPts := sanity[id]["cpu"]; len(cpuPts) == 0 {
				b.Fatalf("sanity: node %s has no cpu points from store", id)
			}
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := monitor.GetNodeMetricsForChartBatch(ids, metricTypes, duration)
			if len(result) != len(ids) {
				b.Fatalf("expected %d results, got %d", len(ids), len(result))
			}
		}
	})
}
