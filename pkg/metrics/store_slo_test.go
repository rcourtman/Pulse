package metrics

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Data-layer SLO targets for metrics store operations.
//
// These define the maximum acceptable p95 latencies for core store operations
// under representative conditions. Targets include generous headroom for CI
// runners (~10x local Apple M4 measurements) to avoid flaky failures while
// still catching genuine regressions (O(n²) scans, missing indices, etc.).
//
// Baseline measurements (Apple M4, March 2026):
//   - WriteBatchSync(100): ~2ms   → SLO 20ms
//   - Query(1000 pts):     ~400µs → SLO 5ms
//   - QueryAll(4×500 pts): ~1.8ms → SLO 15ms
//   - QueryAllBatch(50×4×100): ~18ms → SLO 40ms
//   - QueryAllBatch downsampled (50×4×100, 60s): ~31ms → SLO 55ms
//   - QueryAllBatch chunked (500×4×20): ~29ms → SLO 60ms
//   - rollupTier(50×2×20): ~2.1ms → SLO 15ms
//   - rollupTier fleet-scale (500×4×20): ~70ms → SLO 75ms
//   - Query under write contention: ~400µs → SLO 5ms
//   - 500-node concurrent dashboard load: ~2.9ms → SLO 15ms
//   - QueryManyResources:  ~22µs  → SLO 500µs
const (
	// SLOWriteBatchP95 is the p95 target for WriteBatchSync with 100 metrics —
	// the hot path during periodic buffer flushes.
	SLOWriteBatchP95 = 20 * time.Millisecond

	// SLOQuerySingleP95 is the p95 target for Query (single metric, 1000 raw
	// points, no downsampling) — the most common dashboard chart query.
	SLOQuerySingleP95 = 5 * time.Millisecond

	// SLOQueryAllP95 is the p95 target for QueryAll (4 metric types × 500
	// points each) — dashboard loading all metrics for one resource.
	SLOQueryAllP95 = 15 * time.Millisecond

	// SLOQueryAllBatchP95 is the p95 target for QueryAllBatch (50 resources ×
	// 4 metric types × 100 points each) — the batched dashboard chart path.
	SLOQueryAllBatchP95 = 40 * time.Millisecond

	// SLOQueryAllBatchDownsampledP95 is the p95 target for QueryAllBatch with
	// 60-second downsampling (50 resources × 4 metrics × 100 raw points). This
	// matches the grouped long-range dashboard path that relies on bucketed SQL.
	// The budget is set from the measured p95 rather than the mean benchmark
	// latency so it remains strict without flaking under normal CI variance.
	SLOQueryAllBatchDownsampledP95 = 55 * time.Millisecond

	// SLOQueryAllBatchChunkedP95 is the p95 target for QueryAllBatch at
	// 500-resource scale, where the implementation must split requests into
	// multiple SQL chunks to stay within SQLite parameter limits.
	SLOQueryAllBatchChunkedP95 = 60 * time.Millisecond

	// SLOQueryManyResourcesP95 is the p95 target for Query with 100 resources
	// in the table — validates that index isolation prevents full table scans.
	SLOQueryManyResourcesP95 = 500 * time.Microsecond

	// SLORollupTierBatchedP95 is the p95 target for the production batched
	// rollupTier path (50 resources × 2 metrics × 20 raw points), which must
	// stay fast enough to prevent periodic aggregation from becoming a backlog.
	SLORollupTierBatchedP95 = 15 * time.Millisecond

	// SLORollupTierBatchedFleetP95 is the p95 target for the production
	// batched rollupTier path at 500-resource scale (500 nodes × 4 metrics × 20
	// raw points). This guards the real fleet-scale aggregation workload.
	SLORollupTierBatchedFleetP95 = 75 * time.Millisecond

	// SLOConcurrentReadWriteP95 is the p95 target for single-resource Query
	// while a background writer continuously appends batches on the same SQLite
	// connection pool. This guards dashboard read latency under live ingestion.
	SLOConcurrentReadWriteP95 = 5 * time.Millisecond

	// SLOConcurrentDashboardLoadP95 is the p95 target for a 500-node scenario
	// where 10 concurrent dashboard loads each issue QueryAll while background
	// ingestion continues. This guards fleet-scale read fan-out under write load.
	SLOConcurrentDashboardLoadP95 = 15 * time.Millisecond
)

const sloIterations = 200

// skipUnderRace skips the test when the race detector is enabled, since the
// 2-10x overhead makes latency measurements meaningless.
func skipUnderRace(t *testing.T) {
	t.Helper()
	if raceEnabled {
		t.Skip("skipping SLO latency test under -race (overhead makes measurements unreliable)")
	}
}

// suppressTestLogs disables zerolog for the duration of a test.
func suppressTestLogs(t *testing.T) {
	t.Helper()
	orig := log.Logger
	log.Logger = zerolog.Nop()
	t.Cleanup(func() { log.Logger = orig })
}

// newSLOStore creates an ephemeral metrics store suitable for SLO tests.
func newSLOStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "slo.db")
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = 10_000
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// measureLatencies runs fn sloIterations times with a warmup phase and returns
// the measured latency durations.
func measureLatencies(t *testing.T, fn func()) []time.Duration {
	t.Helper()
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

// pct returns the value at the given percentile (0.0–1.0).
func pct(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// TestSLO_WriteBatchSync validates that WriteBatchSync with 100 metrics meets
// the write throughput SLO. This is the hot path during periodic buffer flushes.
func TestSLO_WriteBatchSync(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now()

	// Pre-build all batches outside the measurement loop.
	const batchSize = 100
	batches := make([][]WriteMetric, sloIterations+20)
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

	iter := 0
	latencies := measureLatencies(t, func() {
		store.WriteBatchSync(batches[iter%len(batches)])
		iter++
	})

	p95 := pct(latencies, 0.95)
	t.Logf("WriteBatchSync(100) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOWriteBatchP95)

	if p95 > SLOWriteBatchP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOWriteBatchP95)
	}

	// Post-measurement sanity: verify writes actually persisted.
	// Each batch writes 2 entries for vm-0 (indices 0 and 50 out of 100).
	// Over iter iterations we expect 2*iter points for vm-0.
	end := base.Add(time.Duration(iter*batchSize) * time.Second)
	pts, err := store.Query("vm", "vm-0", "cpu", base.Add(-time.Second), end, 0)
	if err != nil {
		t.Fatalf("post-write sanity Query: %v", err)
	}
	expectedMin := 2 * iter // 2 entries per batch for vm-0 (indices 0 and 50)
	if len(pts) < expectedMin {
		t.Fatalf("post-write sanity: expected at least %d persisted points for vm-0, got %d — writes may have silently failed", expectedMin, len(pts))
	}
}

// TestSLO_QuerySingle validates that a single-metric Query over 1000 points
// meets the read latency SLO.
func TestSLO_QuerySingle(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-2 * time.Hour)

	const numPoints = 1000
	batch := make([]WriteMetric, numPoints)
	for i := range batch {
		batch[i] = WriteMetric{
			ResourceType: "vm",
			ResourceID:   "vm-slo-query",
			MetricType:   "cpu",
			Value:        float64(i % 100),
			Timestamp:    base.Add(time.Duration(i) * 7 * time.Second),
			Tier:         TierRaw,
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(numPoints) * 7 * time.Second)

	// Sanity check.
	pts, err := store.Query("vm", "vm-slo-query", "cpu", start, end, 0)
	if err != nil {
		t.Fatalf("sanity Query: %v", err)
	}
	if len(pts) != numPoints {
		t.Fatalf("sanity: expected %d points, got %d", numPoints, len(pts))
	}

	latencies := measureLatencies(t, func() {
		_, err := store.Query("vm", "vm-slo-query", "cpu", start, end, 0)
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("Query(1000pts) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOQuerySingleP95)

	if p95 > SLOQuerySingleP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOQuerySingleP95)
	}
}

// TestSLO_QueryAll validates that QueryAll (4 metrics × 500 points) meets the
// multi-metric read latency SLO — the dashboard chart loading path.
func TestSLO_QueryAll(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-2 * time.Hour)

	metricTypes := []string{"cpu", "memory", "disk_read", "disk_write"}
	const pointsPerMetric = 500

	batch := make([]WriteMetric, 0, len(metricTypes)*pointsPerMetric)
	for _, mt := range metricTypes {
		for i := 0; i < pointsPerMetric; i++ {
			batch = append(batch, WriteMetric{
				ResourceType: "vm",
				ResourceID:   "vm-slo-queryall",
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

	// Sanity check: verify all metric types returned with expected point counts.
	result, err := store.QueryAll("vm", "vm-slo-queryall", start, end, 0)
	if err != nil {
		t.Fatalf("sanity QueryAll: %v", err)
	}
	if len(result) != len(metricTypes) {
		t.Fatalf("sanity: expected %d metric types, got %d", len(metricTypes), len(result))
	}
	for _, mt := range metricTypes {
		if len(result[mt]) != pointsPerMetric {
			t.Fatalf("sanity: expected %d points for %s, got %d", pointsPerMetric, mt, len(result[mt]))
		}
	}

	latencies := measureLatencies(t, func() {
		_, err := store.QueryAll("vm", "vm-slo-queryall", start, end, 0)
		if err != nil {
			t.Fatalf("QueryAll: %v", err)
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("QueryAll(4×500) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOQueryAllP95)

	if p95 > SLOQueryAllP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOQueryAllP95)
	}
}

// TestSLO_QueryAllBatch validates that QueryAllBatch meets the dashboard
// multi-resource latency budget and stays on the anti-N+1 path.
func TestSLO_QueryAllBatch(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-2 * time.Hour)

	const numResources = 50
	const pointsPerMetric = 100
	metricTypes := []string{"cpu", "memory", "disk_read", "disk_write"}

	batch := make([]WriteMetric, 0, numResources*len(metricTypes)*pointsPerMetric)
	resourceIDs := make([]string, numResources)
	for r := 0; r < numResources; r++ {
		resourceIDs[r] = fmt.Sprintf("vm-batch-%d", r)
		for _, mt := range metricTypes {
			for p := 0; p < pointsPerMetric; p++ {
				batch = append(batch, WriteMetric{
					ResourceType: "vm",
					ResourceID:   resourceIDs[r],
					MetricType:   mt,
					Value:        float64((r + p) % 100),
					Timestamp:    base.Add(time.Duration(p) * 72 * time.Second),
					Tier:         TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(pointsPerMetric) * 72 * time.Second)

	result, err := store.QueryAllBatch("vm", resourceIDs, start, end, 0)
	if err != nil {
		t.Fatalf("sanity QueryAllBatch: %v", err)
	}
	if len(result) != numResources {
		t.Fatalf("sanity: expected %d resources, got %d", numResources, len(result))
	}
	for _, id := range resourceIDs {
		if len(result[id]) != len(metricTypes) {
			t.Fatalf("sanity: expected %d metric types for %s, got %d", len(metricTypes), id, len(result[id]))
		}
	}

	latencies := measureLatencies(t, func() {
		_, err := store.QueryAllBatch("vm", resourceIDs, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch: %v", err)
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("QueryAllBatch(50×4×100) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOQueryAllBatchP95)

	if p95 > SLOQueryAllBatchP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOQueryAllBatchP95)
	}
}

// TestSLO_QueryAllBatchDownsampled validates the grouped QueryAllBatch path
// used for longer-range dashboard windows where bucketed SQL is required.
func TestSLO_QueryAllBatchDownsampled(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-2 * time.Hour)

	const numResources = 50
	const pointsPerMetric = 100
	metricTypes := []string{"cpu", "memory", "disk_read", "disk_write"}

	batch := make([]WriteMetric, 0, numResources*len(metricTypes)*pointsPerMetric)
	resourceIDs := make([]string, numResources)
	for r := 0; r < numResources; r++ {
		resourceIDs[r] = fmt.Sprintf("vm-batch-ds-%d", r)
		for _, mt := range metricTypes {
			for p := 0; p < pointsPerMetric; p++ {
				batch = append(batch, WriteMetric{
					ResourceType: "vm",
					ResourceID:   resourceIDs[r],
					MetricType:   mt,
					Value:        float64((r + p) % 100),
					Timestamp:    base.Add(time.Duration(p) * 72 * time.Second),
					Tier:         TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(pointsPerMetric) * 72 * time.Second)

	result, err := store.QueryAllBatch("vm", resourceIDs, start, end, 60)
	if err != nil {
		t.Fatalf("sanity QueryAllBatch downsampled: %v", err)
	}
	if len(result) != numResources {
		t.Fatalf("sanity: expected %d resources, got %d", numResources, len(result))
	}
	for _, id := range resourceIDs {
		if len(result[id]) != len(metricTypes) {
			t.Fatalf("sanity: expected %d metric types for %s, got %d", len(metricTypes), id, len(result[id]))
		}
	}

	latencies := measureLatencies(t, func() {
		_, err := store.QueryAllBatch("vm", resourceIDs, start, end, 60)
		if err != nil {
			t.Fatalf("QueryAllBatch downsampled: %v", err)
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("QueryAllBatchDownsampled(50x4x100,60s) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOQueryAllBatchDownsampledP95)

	if p95 > SLOQueryAllBatchDownsampledP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOQueryAllBatchDownsampledP95)
	}
}

// TestSLO_QueryAllBatchChunked validates the fleet-scale QueryAllBatch path
// where resource lists exceed a single SQL IN-clause chunk.
func TestSLO_QueryAllBatchChunked(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-30 * time.Minute)

	batch := make([]WriteMetric, 0, loadTestSeries*loadTestSeedPoints)
	resourceIDs := make([]string, loadTestNodes)
	for n := 0; n < loadTestNodes; n++ {
		resourceIDs[n] = fmt.Sprintf("node-%d", n)
		for _, mt := range loadTestMetricTypes {
			for p := 0; p < loadTestSeedPoints; p++ {
				batch = append(batch, WriteMetric{
					ResourceType: "node",
					ResourceID:   resourceIDs[n],
					MetricType:   mt,
					Value:        float64((n + p) % 100),
					Timestamp:    base.Add(time.Duration(p) * 5 * time.Second),
					Tier:         TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(loadTestSeedPoints) * 5 * time.Second)

	result, err := store.QueryAllBatch("node", resourceIDs, start, end, 0)
	if err != nil {
		t.Fatalf("sanity QueryAllBatch chunked: %v", err)
	}
	if len(result) != loadTestNodes {
		t.Fatalf("sanity: expected %d resources, got %d", loadTestNodes, len(result))
	}
	for _, id := range []string{"node-0", fmt.Sprintf("node-%d", queryAllBatchChunkSize-1), fmt.Sprintf("node-%d", loadTestNodes-1)} {
		if len(result[id]) != loadTestMetrics {
			t.Fatalf("sanity: expected %d metric types for %s, got %d", loadTestMetrics, id, len(result[id]))
		}
	}

	latencies := measureLatencies(t, func() {
		_, err := store.QueryAllBatch("node", resourceIDs, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch chunked: %v", err)
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("QueryAllBatchChunked(500x4x20) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOQueryAllBatchChunkedP95)

	if p95 > SLOQueryAllBatchChunkedP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOQueryAllBatchChunkedP95)
	}
}

// TestSLO_QueryManyResources validates that single-resource Query latency
// remains low when the table contains data for 100 resources — verifying that
// index isolation prevents full table scans.
func TestSLO_QueryManyResources(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
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

	resourceIDs := make([]string, numResources)
	for r := 0; r < numResources; r++ {
		resourceIDs[r] = fmt.Sprintf("node-%d", r)
	}

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(pointsPerResource) * 3 * time.Minute)

	// Sanity check.
	pts, err := store.Query("node", resourceIDs[0], "cpu", start, end, 0)
	if err != nil {
		t.Fatalf("sanity Query: %v", err)
	}
	if len(pts) != pointsPerResource {
		t.Fatalf("sanity: expected %d points, got %d", pointsPerResource, len(pts))
	}

	iter := 0
	latencies := measureLatencies(t, func() {
		_, err := store.Query("node", resourceIDs[iter%numResources], "cpu", start, end, 0)
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		iter++
	})

	p95 := pct(latencies, 0.95)
	t.Logf("QueryManyResources(100) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOQueryManyResourcesP95)

	if p95 > SLOQueryManyResourcesP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOQueryManyResourcesP95)
	}
}

// TestSLO_RollupTierBatched validates the production rollupTier path that
// aggregates all resource/metric combinations in a single SQL statement.
func TestSLO_RollupTierBatched(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	rawBase := time.Now().Add(-30 * time.Minute).Unix()
	base := time.Unix((rawBase/60)*60, 0)

	const numResources = 50
	const metricsPerResource = 2
	const pointsPerMetric = 20
	metricTypes := []string{"cpu", "mem"}

	batch := make([]WriteMetric, 0, numResources*metricsPerResource*pointsPerMetric)
	for r := 0; r < numResources; r++ {
		for _, mt := range metricTypes[:metricsPerResource] {
			for p := 0; p < pointsPerMetric; p++ {
				batch = append(batch, WriteMetric{
					ResourceType: "vm",
					ResourceID:   fmt.Sprintf("vm-%d", r),
					MetricType:   mt,
					Value:        float64((r + p) % 100),
					Timestamp:    base.Add(time.Duration(p) * time.Second),
					Tier:         TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)

	metaKey := "rollup:raw:minute"
	store.rollupTier(TierRaw, TierMinute, time.Minute, 0)

	var minuteCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics WHERE tier = ?`, string(TierMinute)).Scan(&minuteCount); err != nil {
		t.Fatalf("sanity minute-tier count query: %v", err)
	}
	if minuteCount == 0 {
		t.Fatal("sanity: expected minute-tier rows after rollupTier")
	}
	if checkpoint, ok := store.getMetaInt(metaKey); !ok || checkpoint <= 0 {
		t.Fatalf("sanity: expected rollup checkpoint for %s to advance, got %d (ok=%v)", metaKey, checkpoint, ok)
	}

	latencies := measureLatencies(t, func() {
		_ = store.setMetaInt(metaKey, 0)
		store.rollupTier(TierRaw, TierMinute, time.Minute, 0)
	})

	p95 := pct(latencies, 0.95)
	t.Logf("rollupTier(50x2x20) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLORollupTierBatchedP95)

	if p95 > SLORollupTierBatchedP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLORollupTierBatchedP95)
	}
}

// TestSLO_ConcurrentReadWrite validates query latency under continuous write
// contention on the single SQLite connection pool used in production.
func TestSLO_ConcurrentReadWrite(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-time.Hour)

	const seedPoints = 500
	seed := make([]WriteMetric, seedPoints)
	for i := range seed {
		seed[i] = WriteMetric{
			ResourceType: "vm",
			ResourceID:   "vm-crw",
			MetricType:   "cpu",
			Value:        float64(i % 100),
			Timestamp:    base.Add(time.Duration(i) * 7 * time.Second),
			Tier:         TierRaw,
		}
	}
	store.WriteBatchSync(seed)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(seedPoints) * 7 * time.Second)

	pts, err := store.Query("vm", "vm-crw", "cpu", start, end, 0)
	if err != nil {
		t.Fatalf("sanity Query: %v", err)
	}
	if len(pts) != seedPoints {
		t.Fatalf("sanity: expected %d points, got %d", seedPoints, len(pts))
	}

	stop := make(chan struct{})
	writerDone := make(chan struct{})
	started := make(chan struct{})
	go func() {
		defer close(writerDone)
		writeBase := end
		tick := 0
		for {
			select {
			case <-stop:
				return
			default:
			}

			batch := make([]WriteMetric, 10)
			for j := range batch {
				batch[j] = WriteMetric{
					ResourceType: "vm",
					ResourceID:   fmt.Sprintf("vm-crw-live-%d", tick%50),
					MetricType:   "cpu",
					Value:        float64((tick + j) % 100),
					Timestamp:    writeBase.Add(time.Duration(tick*10+j) * 2 * time.Second),
					Tier:         TierRaw,
				}
			}

			store.WriteBatchSync(batch)
			if tick == 0 {
				close(started)
			}
			tick++
		}
	}()

	<-started
	t.Cleanup(func() {
		close(stop)
		<-writerDone
	})

	latencies := measureLatencies(t, func() {
		pts, err := store.Query("vm", "vm-crw", "cpu", start, end, 0)
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(pts) < seedPoints {
			t.Fatalf("expected at least %d points, got %d", seedPoints, len(pts))
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("ConcurrentReadWrite p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOConcurrentReadWriteP95)

	if p95 > SLOConcurrentReadWriteP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOConcurrentReadWriteP95)
	}
}

// TestSLO_RollupTierBatchedFleet validates the production batched rollupTier
// path at 500-node scale, where all node/metric series are aggregated in a
// single grouped SQL statement.
func TestSLO_RollupTierBatchedFleet(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	rawBase := time.Now().Add(-30 * time.Minute).Unix()
	base := time.Unix((rawBase/60)*60, 0)

	batch := make([]WriteMetric, 0, loadTestSeries*loadTestSeedPoints)
	for n := 0; n < loadTestNodes; n++ {
		nodeID := fmt.Sprintf("node-%d", n)
		for _, mt := range loadTestMetricTypes {
			for p := 0; p < loadTestSeedPoints; p++ {
				batch = append(batch, WriteMetric{
					ResourceType: "node",
					ResourceID:   nodeID,
					MetricType:   mt,
					Value:        float64((n + p) % 100),
					Timestamp:    base.Add(time.Duration(p) * 5 * time.Second),
					Tier:         TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)

	metaKey := "rollup:raw:minute"
	store.rollupTier(TierRaw, TierMinute, time.Minute, 0)

	var minuteCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics WHERE tier = ?`, string(TierMinute)).Scan(&minuteCount); err != nil {
		t.Fatalf("sanity minute-tier count query: %v", err)
	}
	if minuteCount == 0 {
		t.Fatal("sanity: expected minute-tier rows after fleet rollupTier")
	}
	if checkpoint, ok := store.getMetaInt(metaKey); !ok || checkpoint <= 0 {
		t.Fatalf("sanity: expected rollup checkpoint for %s to advance, got %d (ok=%v)", metaKey, checkpoint, ok)
	}

	latencies := measureLatencies(t, func() {
		_ = store.setMetaInt(metaKey, 0)
		store.rollupTier(TierRaw, TierMinute, time.Minute, 0)
	})

	p95 := pct(latencies, 0.95)
	t.Logf("rollupTierFleet(500x4x20) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLORollupTierBatchedFleetP95)

	if p95 > SLORollupTierBatchedFleetP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLORollupTierBatchedFleetP95)
	}
}

// TestSLO_ConcurrentDashboardLoad validates fleet-scale QueryAll latency when
// 10 concurrent dashboard loads race with continuous ingestion on the same
// SQLite connection pool.
func TestSLO_ConcurrentDashboardLoad(t *testing.T) {
	skipUnderRace(t)
	suppressTestLogs(t)

	store := newSLOStore(t)
	base := time.Now().Add(-30 * time.Minute)

	batch := make([]WriteMetric, 0, loadTestSeries*loadTestSeedPoints)
	nodeIDs := make([]string, loadTestNodes)
	for n := 0; n < loadTestNodes; n++ {
		nodeIDs[n] = fmt.Sprintf("node-%d", n)
		for _, mt := range loadTestMetricTypes {
			for p := 0; p < loadTestSeedPoints; p++ {
				batch = append(batch, WriteMetric{
					ResourceType: "node",
					ResourceID:   nodeIDs[n],
					MetricType:   mt,
					Value:        float64((n + p) % 100),
					Timestamp:    base.Add(time.Duration(p) * 5 * time.Second),
					Tier:         TierRaw,
				})
			}
		}
	}
	store.WriteBatchSync(batch)

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(loadTestSeedPoints) * 5 * time.Second)

	const writerNodesPerBatch = 5
	const writerBatchSize = writerNodesPerBatch * loadTestMetrics
	stop := make(chan struct{})
	writerDone := make(chan struct{})
	started := make(chan struct{})
	go func() {
		defer close(writerDone)
		writeBase := end
		tick := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			liveBatch := make([]WriteMetric, writerBatchSize)
			nodeOffset := (tick * writerNodesPerBatch) % loadTestNodes
			ts := writeBase.Add(time.Duration(tick) * 5 * time.Second)
			for n := 0; n < writerNodesPerBatch; n++ {
				for m := 0; m < loadTestMetrics; m++ {
					liveBatch[n*loadTestMetrics+m] = WriteMetric{
						ResourceType: "node",
						ResourceID:   nodeIDs[(nodeOffset+n)%loadTestNodes],
						MetricType:   loadTestMetricTypes[m],
						Value:        float64((tick + n + m) % 100),
						Timestamp:    ts,
						Tier:         TierRaw,
					}
				}
			}
			store.WriteBatchSync(liveBatch)
			if tick == 0 {
				close(started)
			}
			tick++
		}
	}()
	<-started
	t.Cleanup(func() {
		close(stop)
		<-writerDone
	})

	latencies := measureLatencies(t, func() {
		var wg sync.WaitGroup
		var queryErrors atomic.Int32
		for u := 0; u < 10; u++ {
			wg.Add(1)
			go func(userIdx int) {
				defer wg.Done()
				nodeIdx := userIdx % loadTestNodes
				result, err := store.QueryAll("node", nodeIDs[nodeIdx], start, end, 0)
				if err != nil {
					queryErrors.Add(1)
					return
				}
				if len(result) != loadTestMetrics {
					queryErrors.Add(1)
				}
			}(u)
		}
		wg.Wait()
		if errs := queryErrors.Load(); errs > 0 {
			t.Fatalf("%d concurrent QueryAll errors", errs)
		}
	})

	p95 := pct(latencies, 0.95)
	t.Logf("ConcurrentDashboardLoad(500nodes,10users) p50=%v p95=%v p99=%v SLO=%v",
		pct(latencies, 0.50), p95, pct(latencies, 0.99), SLOConcurrentDashboardLoadP95)

	if p95 > SLOConcurrentDashboardLoadP95 {
		t.Errorf("SLO VIOLATION: p95=%v exceeds target %v", p95, SLOConcurrentDashboardLoadP95)
	}
}
