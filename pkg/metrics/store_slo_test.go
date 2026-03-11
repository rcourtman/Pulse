package metrics

import (
	"fmt"
	"path/filepath"
	"sort"
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
//   - rollupTier(50×2×20): ~2.1ms → SLO 15ms
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

	// SLOQueryManyResourcesP95 is the p95 target for Query with 100 resources
	// in the table — validates that index isolation prevents full table scans.
	SLOQueryManyResourcesP95 = 500 * time.Microsecond

	// SLORollupTierBatchedP95 is the p95 target for the production batched
	// rollupTier path (50 resources × 2 metrics × 20 raw points), which must
	// stay fast enough to prevent periodic aggregation from becoming a backlog.
	SLORollupTierBatchedP95 = 15 * time.Millisecond
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
