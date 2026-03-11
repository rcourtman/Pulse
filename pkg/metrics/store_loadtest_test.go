package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// 500-Node Deployment Simulation Load Tests
//
// These tests simulate a production deployment with 500 Proxmox nodes, each
// reporting 4 metric types (cpu, memory, disk_read, disk_write). The goal is
// to validate that the metrics store can handle ingestion, query, and rollup
// workloads at this scale without unacceptable latency or contention.
//
// Scale parameters:
//   - 500 nodes × 4 metrics = 2000 metric series
//   - Ingestion: 100 metrics/batch, simulating 5-second recording intervals
//   - Query: concurrent dashboard loads (QueryAll for multiple resources)
//   - Rollup: minute-tier aggregation of raw data
// =============================================================================

const (
	loadTestNodes      = 500
	loadTestMetrics    = 4 // cpu, memory, disk_read, disk_write
	loadTestSeries     = loadTestNodes * loadTestMetrics
	loadTestSeedPoints = 20 // per metric series (simulates ~100s of data at 5s intervals)
)

var loadTestMetricTypes = [loadTestMetrics]string{"cpu", "memory", "disk_read", "disk_write"}

// seedLoadTestData populates the store with realistic 500-node data.
// Returns the time range of the seeded data.
func seedLoadTestData(b *testing.B, store *Store) (start, end time.Time) {
	b.Helper()
	base := time.Now().Add(-30 * time.Minute)

	// Batch all seed data in a single WriteBatchSync for efficiency.
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

	return base.Add(-time.Second), base.Add(time.Duration(loadTestSeedPoints) * 5 * time.Second)
}

// BenchmarkLoadTest500Nodes_WriteBatch measures batch-write throughput at
// 500-node scale. Each batch contains 100 metrics from mixed resources,
// mirroring the production recording tick that collects all node metrics
// and writes them in one batch.
func BenchmarkLoadTest500Nodes_WriteBatch(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	base := time.Now()

	// Precompute resource IDs and batches.
	nodeIDs := make([]string, loadTestNodes)
	for n := range nodeIDs {
		nodeIDs[n] = fmt.Sprintf("node-%d", n)
	}

	// Each batch covers 25 nodes × 4 metrics = 100 entries, cycling through
	// all 500 nodes across successive batches (20 batches = full coverage).
	const nodesPerBatch = 25
	const batchSize = nodesPerBatch * loadTestMetrics
	batches := make([][]WriteMetric, b.N)
	for n := range batches {
		batch := make([]WriteMetric, batchSize)
		nodeOffset := (n * nodesPerBatch) % loadTestNodes
		ts := base.Add(time.Duration(n) * 5 * time.Second)
		for i := 0; i < nodesPerBatch; i++ {
			for m := 0; m < loadTestMetrics; m++ {
				batch[i*loadTestMetrics+m] = WriteMetric{
					ResourceType: "node",
					ResourceID:   nodeIDs[(nodeOffset+i)%loadTestNodes],
					MetricType:   loadTestMetricTypes[m],
					Value:        float64((nodeOffset + i + m) % 100),
					Timestamp:    ts,
					Tier:         TierRaw,
				}
			}
		}
		batches[n] = batch
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store.WriteBatchSync(batches[i])
	}
}

// BenchmarkLoadTest500Nodes_QuerySingle measures single-resource query latency
// when the table contains data for 500 nodes. Validates that index isolation
// prevents full table scans at scale.
func BenchmarkLoadTest500Nodes_QuerySingle(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	start, end := seedLoadTestData(b, store)

	// Precompute resource IDs to avoid fmt.Sprintf in the timed loop.
	nodeIDs := make([]string, loadTestNodes)
	for n := range nodeIDs {
		nodeIDs[n] = fmt.Sprintf("node-%d", n)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pts, err := store.Query("node", nodeIDs[i%loadTestNodes], "cpu", start, end, 0)
		if err != nil {
			b.Fatalf("Query: %v", err)
		}
		if len(pts) != loadTestSeedPoints {
			b.Fatalf("expected %d points, got %d", loadTestSeedPoints, len(pts))
		}
	}
}

// BenchmarkLoadTest500Nodes_QueryAllDashboard measures QueryAll latency
// (all 4 metric types for one resource) at 500-node scale. This is the
// hot path when a user opens a node's detail view.
func BenchmarkLoadTest500Nodes_QueryAllDashboard(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	start, end := seedLoadTestData(b, store)

	nodeIDs := make([]string, loadTestNodes)
	for n := range nodeIDs {
		nodeIDs[n] = fmt.Sprintf("node-%d", n)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := store.QueryAll("node", nodeIDs[i%loadTestNodes], start, end, 0)
		if err != nil {
			b.Fatalf("QueryAll: %v", err)
		}
		if len(result) != loadTestMetrics {
			b.Fatalf("expected %d metric types, got %d", loadTestMetrics, len(result))
		}
		for _, mt := range loadTestMetricTypes {
			if len(result[mt]) != loadTestSeedPoints {
				b.Fatalf("expected %d points for %s, got %d", loadTestSeedPoints, mt, len(result[mt]))
			}
		}
	}
}

// BenchmarkLoadTest500Nodes_ConcurrentDashboardLoad simulates 10 concurrent
// users each loading a different node's dashboard (QueryAll) while a background
// writer saturates the store with unbounded write pressure. This measures
// worst-case query latency under maximum write contention at 500-node scale
// (same stress pattern as BenchmarkConcurrentReadWrite).
func BenchmarkLoadTest500Nodes_ConcurrentDashboardLoad(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	start, end := seedLoadTestData(b, store)

	nodeIDs := make([]string, loadTestNodes)
	for n := range nodeIDs {
		nodeIDs[n] = fmt.Sprintf("node-%d", n)
	}

	// Background writer simulating continuous ingestion: 5 nodes × 4 metrics
	// per batch, cycling through all 500 nodes across ticks.
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
			batch := make([]WriteMetric, writerBatchSize)
			nodeOffset := (tick * writerNodesPerBatch) % loadTestNodes
			ts := writeBase.Add(time.Duration(tick) * 5 * time.Second)
			for n := 0; n < writerNodesPerBatch; n++ {
				for m := 0; m < loadTestMetrics; m++ {
					batch[n*loadTestMetrics+m] = WriteMetric{
						ResourceType: "node",
						ResourceID:   nodeIDs[(nodeOffset+n)%loadTestNodes],
						MetricType:   loadTestMetricTypes[m],
						Value:        float64((tick + n + m) % 100),
						Timestamp:    ts,
						Tier:         TierRaw,
					}
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
	b.Cleanup(func() {
		close(stop)
		<-writerDone
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Simulate 10 concurrent dashboard loads.
		var wg sync.WaitGroup
		var queryErrors atomic.Int32
		for u := 0; u < 10; u++ {
			wg.Add(1)
			go func(userIdx int) {
				defer wg.Done()
				nodeIdx := (i*10 + userIdx) % loadTestNodes
				_, err := store.QueryAll("node", nodeIDs[nodeIdx], start, end, 0)
				if err != nil {
					queryErrors.Add(1)
				}
			}(u)
		}
		wg.Wait()
		if errs := queryErrors.Load(); errs > 0 {
			b.Fatalf("%d query errors in iteration %d", errs, i)
		}
	}
}

// BenchmarkLoadTest500Nodes_RollupAtScale measures rollup aggregation
// throughput with 500-node data volume. Exercises the rollupCandidate
// path that runs once per resource/metric pair every 5 minutes in production.
func BenchmarkLoadTest500Nodes_RollupAtScale(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)

	// Seed data with minute-aligned timestamps for clean rollup boundaries.
	rawBase := time.Now().Add(-30 * time.Minute).Unix()
	base := time.Unix((rawBase/60)*60, 0)

	batch := make([]WriteMetric, 0, loadTestNodes*loadTestSeedPoints)
	for n := 0; n < loadTestNodes; n++ {
		nodeID := fmt.Sprintf("node-%d", n)
		for p := 0; p < loadTestSeedPoints; p++ {
			batch = append(batch, WriteMetric{
				ResourceType: "node",
				ResourceID:   nodeID,
				MetricType:   "cpu",
				Value:        float64((n + p) % 100),
				Timestamp:    base.Add(time.Duration(p) * 5 * time.Second),
				Tier:         TierRaw,
			})
		}
	}
	store.WriteBatchSync(batch)

	bucketSecs := int64(60)
	startTs := (base.Unix() / bucketSecs) * bucketSecs
	rawEnd := base.Add(time.Duration(loadTestSeedPoints) * 5 * time.Second).Unix()
	endTs := ((rawEnd + bucketSecs - 1) / bucketSecs) * bucketSecs

	// Precompute a random subset of nodes to rollup (benchmarking one node
	// per iteration at 500-node scale, cycling through all nodes).
	nodeIDs := make([]string, loadTestNodes)
	for n := range nodeIDs {
		nodeIDs[n] = fmt.Sprintf("node-%d", n)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store.rollupCandidate("node", nodeIDs[i%loadTestNodes], "cpu", TierRaw, TierMinute, bucketSecs, startTs, endTs)
	}
}

// BenchmarkLoadTest500Nodes_RollupTierBatched measures the production batched
// rollupTier path at 500-node scale. This is the real aggregation path used in
// the store: one INSERT...SELECT...GROUP BY across all node/metric series.
func BenchmarkLoadTest500Nodes_RollupTierBatched(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)

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

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = store.setMetaInt(metaKey, 0)
		store.rollupTier(TierRaw, TierMinute, time.Minute, 0)
	}
}

// BenchmarkLoadTest500Nodes_QueryAllBatchDashboard measures QueryAllBatch
// latency at 500-node scale — the production hot path when the dashboard
// loads charts for all nodes in a single batched SQL call. Contrasts with
// BenchmarkLoadTest500Nodes_QueryAllDashboard which queries one node at a time.
func BenchmarkLoadTest500Nodes_QueryAllBatchDashboard(b *testing.B) {
	suppressLogs(b)
	store := newBenchStore(b)
	start, end := seedLoadTestData(b, store)

	// Build full ID list (all 500 nodes) and smaller subsets.
	allIDs := make([]string, loadTestNodes)
	for n := range allIDs {
		allIDs[n] = fmt.Sprintf("node-%d", n)
	}

	for _, count := range []int{10, 50, 100, 500} {
		ids := allIDs[:count]
		b.Run(fmt.Sprintf("%d-nodes", count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := store.QueryAllBatch("node", ids, start, end, 0)
				if err != nil {
					b.Fatalf("QueryAllBatch: %v", err)
				}
				if len(result) != count {
					b.Fatalf("expected %d resources, got %d", count, len(result))
				}
				// Spot-check one resource per iteration (rotating) for metric completeness.
				checkID := ids[i%count]
				resMetrics := result[checkID]
				if len(resMetrics) != loadTestMetrics {
					b.Fatalf("resource %s: expected %d metric types, got %d", checkID, loadTestMetrics, len(resMetrics))
				}
				for _, mt := range loadTestMetricTypes {
					if len(resMetrics[mt]) != loadTestSeedPoints {
						b.Fatalf("resource %s metric %s: expected %d points, got %d", checkID, mt, loadTestSeedPoints, len(resMetrics[mt]))
					}
				}
			}
		})
	}
}

// TestLoadTest500Nodes_WriteAndQueryIntegration is a non-benchmark integration
// test that verifies correctness at 500-node scale: seeds data, queries every
// resource, and validates result counts. Uses t.Parallel() for CI efficiency.
func TestLoadTest500Nodes_WriteAndQueryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 500-node load test in short mode")
	}
	t.Parallel()
	suppressLogs(t)

	store := func() *Store {
		t.Helper()
		dir := t.TempDir()
		cfg := DefaultConfig(dir)
		cfg.FlushInterval = time.Hour
		cfg.WriteBufferSize = 50_000
		s, err := NewStore(cfg)
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	}()

	base := time.Now().Add(-30 * time.Minute)
	const pointsPerSeries = 20

	// Seed all 500 nodes × 4 metrics.
	batch := make([]WriteMetric, 0, loadTestSeries*pointsPerSeries)
	for n := 0; n < loadTestNodes; n++ {
		nodeID := fmt.Sprintf("node-%d", n)
		for _, mt := range loadTestMetricTypes {
			for p := 0; p < pointsPerSeries; p++ {
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

	start := base.Add(-time.Second)
	end := base.Add(time.Duration(pointsPerSeries) * 5 * time.Second)

	// Verify all 500 nodes to ensure complete correctness at scale.
	for idx := 0; idx < loadTestNodes; idx++ {
		nodeID := fmt.Sprintf("node-%d", idx)
		result, err := store.QueryAll("node", nodeID, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAll(%s): %v", nodeID, err)
		}
		if len(result) != loadTestMetrics {
			t.Fatalf("QueryAll(%s): expected %d metric types, got %d", nodeID, loadTestMetrics, len(result))
		}
		for _, mt := range loadTestMetricTypes {
			pts := result[mt]
			if len(pts) != pointsPerSeries {
				t.Fatalf("QueryAll(%s)[%s]: expected %d points, got %d", nodeID, mt, pointsPerSeries, len(pts))
			}
		}
	}
}
