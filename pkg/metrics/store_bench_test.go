package metrics

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// suppressLogs disables the zerolog global logger for the duration of a
// benchmark/test and restores it on cleanup. This prevents log I/O from
// skewing benchmark results while leaving other tests in the package unaffected.
func suppressLogs(tb testing.TB) {
	tb.Helper()
	orig := log.Logger
	log.Logger = zerolog.Nop()
	tb.Cleanup(func() { log.Logger = orig })
}

// newBenchStore creates an ephemeral metrics store suitable for benchmarks.
// It disables automatic background flushes so callers control timing.
func newBenchStore(b *testing.B) *Store {
	b.Helper()
	dir := b.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "bench.db")
	cfg.FlushInterval = time.Hour // prevent background flushes
	cfg.WriteBufferSize = 10_000  // large buffer so Write() doesn't auto-flush
	store, err := NewStore(cfg)
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	b.Cleanup(func() { store.Close() })
	return store
}

// BenchmarkWriteBatchSync measures raw SQLite insert throughput via the
// synchronous batch-write path (the hot path during metrics recording).
// Batch sizes mirror real-world usage: 100 metrics per flush.
func BenchmarkWriteBatchSync(b *testing.B) {
	suppressLogs(b)
	for _, batchSize := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			store := newBenchStore(b)
			base := time.Now()

			// Precompute all batches so timestamp construction is outside the timed loop.
			batches := make([][]WriteMetric, b.N)
			for n := range batches {
				batch := make([]WriteMetric, batchSize)
				offset := time.Duration(n*batchSize) * time.Second
				for i := range batch {
					batch[i] = WriteMetric{
						ResourceType: "vm",
						ResourceID:   fmt.Sprintf("vm-%d", i%50),
						MetricType:   "cpu",
						Value:        float64(i % 100),
						Timestamp:    base.Add(offset + time.Duration(i)*time.Second),
						Tier:         TierRaw,
					}
				}
				batches[n] = batch
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				store.WriteBatchSync(batches[i])
			}
		})
	}
}

// BenchmarkQuery measures single-metric query latency over a pre-populated
// dataset of 1000 raw data points — representative of a 2-hour window at
// ~7-second intervals.
func BenchmarkQuery(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	base := time.Now().Add(-2 * time.Hour)

	// Seed 1000 raw points for one resource.
	const numPoints = 1000
	batch := make([]WriteMetric, numPoints)
	for i := range batch {
		batch[i] = WriteMetric{
			ResourceType: "vm",
			ResourceID:   "vm-bench",
			MetricType:   "cpu",
			Value:        float64(i % 100),
			Timestamp:    base.Add(time.Duration(i) * 7 * time.Second),
			Tier:         TierRaw,
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(numPoints) * 7 * time.Second)

	b.Run("no-downsample", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pts, err := store.Query("vm", "vm-bench", "cpu", start, end, 0)
			if err != nil {
				b.Fatalf("Query: %v", err)
			}
			if len(pts) != numPoints {
				b.Fatalf("expected %d points, got %d", numPoints, len(pts))
			}
		}
	})

	b.Run("downsample-60s", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := store.Query("vm", "vm-bench", "cpu", start, end, 60)
			if err != nil {
				b.Fatalf("Query: %v", err)
			}
		}
	})
}

// BenchmarkQueryAll measures multi-metric query latency. Seeds 4 metric types
// with 500 points each — representative of a dashboard loading all metrics for
// one resource.
func BenchmarkQueryAll(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	base := time.Now().Add(-2 * time.Hour)

	metricTypes := []string{"cpu", "memory", "disk_read", "disk_write"}
	const pointsPerMetric = 500

	batch := make([]WriteMetric, 0, len(metricTypes)*pointsPerMetric)
	for _, mt := range metricTypes {
		for i := 0; i < pointsPerMetric; i++ {
			batch = append(batch, WriteMetric{
				ResourceType: "vm",
				ResourceID:   "vm-bench",
				MetricType:   mt,
				Value:        float64(i % 100),
				Timestamp:    base.Add(time.Duration(i) * 14 * time.Second),
				Tier:         TierRaw,
			})
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(pointsPerMetric) * 14 * time.Second)

	b.Run("4-metrics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			result, err := store.QueryAll("vm", "vm-bench", start, end, 0)
			if err != nil {
				b.Fatalf("QueryAll: %v", err)
			}
			if len(result) != len(metricTypes) {
				b.Fatalf("expected %d metric types, got %d", len(metricTypes), len(result))
			}
			for _, mt := range metricTypes {
				if len(result[mt]) != pointsPerMetric {
					b.Fatalf("expected %d points for %s, got %d", pointsPerMetric, mt, len(result[mt]))
				}
			}
		}
	})

	b.Run("4-metrics-downsample-60s", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := store.QueryAll("vm", "vm-bench", start, end, 60)
			if err != nil {
				b.Fatalf("QueryAll: %v", err)
			}
		}
	})
}

// BenchmarkWriteBuffered measures the in-memory buffer path (Write → buffer
// append). This isolates the mutex-protected hot path that every live metrics
// tick goes through. The buffer capacity is set high enough that writes don't
// trigger flushes, and the buffer is periodically drained (outside timing) to
// bound memory usage.
func BenchmarkWriteBuffered(b *testing.B) {
	suppressLogs(b)

	const drainEvery = 100_000 // drain buffer every 100k writes to bound memory

	dir := b.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "bench.db")
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = drainEvery + 1 // larger than drain interval → no auto-flush
	store, err := NewStore(cfg)
	if err != nil {
		b.Fatalf("NewStore: %v", err)
	}
	b.Cleanup(func() {
		store.bufferMu.Lock()
		store.buffer = store.buffer[:0]
		store.bufferMu.Unlock()
		store.Close()
	})

	base := time.Now()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i > 0 && i%drainEvery == 0 {
			b.StopTimer()
			store.bufferMu.Lock()
			store.buffer = store.buffer[:0]
			store.bufferMu.Unlock()
			b.StartTimer()
		}
		store.Write("vm", "vm-101", "cpu", 42.0, base.Add(time.Duration(i)*time.Second))
	}
}

// BenchmarkRollupCandidate measures the core rollup aggregation path — an
// INSERT SELECT that reads raw-tier data, aggregates into minute buckets, and
// writes the result. This is the inner loop called once per resource/metric
// pair every 5 minutes. At 500-node scale (4 metrics × ~500 resources), this
// runs ~2000 times per rollup cycle.
//
// After the first iteration, INSERT OR IGNORE detects existing minute-tier rows
// and skips re-insertion. The SELECT + GROUP BY aggregation (the expensive part)
// still executes every iteration, so this accurately measures scan/aggregation
// cost which dominates rollup latency.
func BenchmarkRollupCandidate(b *testing.B) {
	suppressLogs(b)

	for _, numPoints := range []int{100, 1000} {
		b.Run(fmt.Sprintf("points=%d", numPoints), func(b *testing.B) {
			store := newBenchStore(b)

			// Place all data points well in the past so even the largest
			// sub-benchmark (1000 points at 1-second spacing = ~17 minutes)
			// never extends past "now". 30 minutes of headroom is sufficient.
			// Floor to minute boundary so startTs alignment doesn't trim
			// leading points, keeping the effective input size deterministic.
			raw := time.Now().Add(-30 * time.Minute).Unix()
			base := time.Unix((raw/60)*60, 0)
			batch := make([]WriteMetric, numPoints)
			for i := range batch {
				batch[i] = WriteMetric{
					ResourceType: "vm",
					ResourceID:   "vm-rollup",
					MetricType:   "cpu",
					Value:        float64(i % 100),
					Timestamp:    base.Add(time.Duration(i) * time.Second),
					Tier:         TierRaw,
				}
			}
			store.WriteBatchSync(batch)

			// Bucket-align boundaries to match production rollupTier behavior.
			// startTs is floored (base is already aligned). endTs is ceiled so
			// all seeded points fall within the [startTs, endTs) window.
			bucketSecs := int64(60)
			startTs := (base.Unix() / bucketSecs) * bucketSecs
			rawEnd := base.Add(time.Duration(numPoints) * time.Second).Unix()
			endTs := ((rawEnd + bucketSecs - 1) / bucketSecs) * bucketSecs

			// Sanity check: verify rollup actually produces data on first call.
			store.rollupCandidate("vm", "vm-rollup", "cpu", TierRaw, TierMinute, bucketSecs, startTs, endTs)
			var minuteCount int
			if err := store.db.QueryRow(
				`SELECT COUNT(*) FROM metrics WHERE tier = 'minute' AND resource_id = 'vm-rollup'`,
			).Scan(&minuteCount); err != nil {
				b.Fatalf("sanity check query: %v", err)
			}
			if minuteCount == 0 {
				b.Fatal("sanity check: rollupCandidate produced no minute-tier rows")
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				store.rollupCandidate("vm", "vm-rollup", "cpu", TierRaw, TierMinute, bucketSecs, startTs, endTs)
			}
		})
	}
}

// BenchmarkQueryManyResources measures query latency when the metrics table
// contains data for many distinct resources — simulating a 100-resource
// deployment where index isolation matters.
func BenchmarkQueryManyResources(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	base := time.Now().Add(-time.Hour)

	const numResources = 100
	const pointsPerResource = 20
	batch := make([]WriteMetric, 0, numResources*pointsPerResource)
	for r := 0; r < numResources; r++ {
		for p := 0; p < pointsPerResource; p++ {
			batch = append(batch, WriteMetric{
				ResourceType: "node",
				ResourceID:   fmt.Sprintf("node-%d", r),
				MetricType:   "cpu",
				Value:        float64(p * 5),
				Timestamp:    base.Add(time.Duration(p) * 3 * time.Minute),
				Tier:         TierRaw,
			})
		}
	}
	store.WriteBatchSync(batch)

	// Precompute resource IDs to avoid fmt.Sprintf in the timed loop.
	resourceIDs := make([]string, numResources)
	for r := 0; r < numResources; r++ {
		resourceIDs[r] = fmt.Sprintf("node-%d", r)
	}

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(pointsPerResource) * 3 * time.Minute)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Query a single resource — index should isolate it from the 2000 total rows.
		pts, err := store.Query("node", resourceIDs[i%numResources], "cpu", start, end, 0)
		if err != nil {
			b.Fatalf("Query: %v", err)
		}
		if len(pts) != pointsPerResource {
			b.Fatalf("expected %d points, got %d", pointsPerResource, len(pts))
		}
	}
}
