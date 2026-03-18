package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func newChartFallbackTestMonitor(t *testing.T) *Monitor {
	t.Helper()

	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 90 * 24 * time.Hour
	cfg.RetentionMinute = 90 * 24 * time.Hour
	cfg.RetentionHourly = 90 * 24 * time.Hour
	cfg.RetentionDaily = 90 * 24 * time.Hour
	cfg.WriteBufferSize = 64

	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	return &Monitor{
		metricsHistory: NewMetricsHistory(1024, 24*time.Hour),
		metricsStore:   store,
	}
}

func writeRawMetricBatch(t *testing.T, store *metrics.Store, resourceType, resourceID, metricType string, points []MetricPoint) {
	t.Helper()

	records := make([]metrics.WriteMetric, len(points))
	for i, point := range points {
		records[i] = metrics.WriteMetric{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			MetricType:   metricType,
			Value:        point.Value,
			Timestamp:    point.Timestamp,
			Tier:         metrics.TierRaw,
		}
	}
	store.WriteBatchSync(records)
}

func TestGetGuestMetricsForChart_ShortRangeFallsBackToStoreWhenInMemoryCoverageShallow(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	inMemoryKey := "agent:host-1"
	monitor.metricsHistory.AddGuestMetric(inMemoryKey, "cpu", 15, now.Add(-4*time.Minute))
	monitor.metricsHistory.AddGuestMetric(inMemoryKey, "cpu", 18, now.Add(-1*time.Minute))

	writeRawMetricBatch(t, monitor.metricsStore, "agent", "host-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-58 * time.Minute), Value: 41},
		{Timestamp: now.Add(-30 * time.Minute), Value: 43},
		{Timestamp: now.Add(-1 * time.Minute), Value: 46},
	})

	result := monitor.GetGuestMetricsForChart(inMemoryKey, "agent", "host-1", duration)
	if got, wantMin := chartMapCoverageSpan(result), 45*time.Minute; got < wantMin {
		t.Fatalf("expected store-backed coverage >= %s, got %s", wantMin, got)
	}
}

func TestGetNodeMetricsForChart_ShortRangeUsesInMemoryWhenCoverageSufficient(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	for _, offset := range []time.Duration{-59 * time.Minute, -30 * time.Minute, -1 * time.Minute} {
		monitor.metricsHistory.AddNodeMetric("node-1", "cpu", 11, now.Add(offset))
	}

	writeRawMetricBatch(t, monitor.metricsStore, "node", "node-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-59 * time.Minute), Value: 99},
		{Timestamp: now.Add(-30 * time.Minute), Value: 99},
		{Timestamp: now.Add(-1 * time.Minute), Value: 99},
	})

	result := monitor.GetNodeMetricsForChart("node-1", "cpu", duration)
	if len(result) == 0 {
		t.Fatal("expected non-empty node chart series")
	}
	if result[0].Value != 11 {
		t.Fatalf("expected in-memory series to be preferred, got first value %.2f", result[0].Value)
	}
}

func TestGetNodeMetricsForChart_ShortRangeFallsBackToStoreWhenInMemoryCoverageShallow(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	monitor.metricsHistory.AddNodeMetric("node-1", "cpu", 10, now.Add(-5*time.Minute))
	monitor.metricsHistory.AddNodeMetric("node-1", "cpu", 12, now.Add(-1*time.Minute))

	writeRawMetricBatch(t, monitor.metricsStore, "node", "node-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-57 * time.Minute), Value: 60},
		{Timestamp: now.Add(-25 * time.Minute), Value: 62},
		{Timestamp: now.Add(-1 * time.Minute), Value: 65},
	})

	result := monitor.GetNodeMetricsForChart("node-1", "cpu", duration)
	if got, wantMin := chartSeriesCoverageSpan(result), 45*time.Minute; got < wantMin {
		t.Fatalf("expected store-backed node coverage >= %s, got %s", wantMin, got)
	}
}

func TestGetStorageMetricsForChart_ShortRangeFallsBackToStoreWhenInMemoryCoverageShallow(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	monitor.metricsHistory.AddStorageMetric("storage-1", "usage", 20, now.Add(-3*time.Minute))
	monitor.metricsHistory.AddStorageMetric("storage-1", "usage", 21, now.Add(-1*time.Minute))

	writeRawMetricBatch(t, monitor.metricsStore, "storage", "storage-1", "usage", []MetricPoint{
		{Timestamp: now.Add(-56 * time.Minute), Value: 71},
		{Timestamp: now.Add(-28 * time.Minute), Value: 73},
		{Timestamp: now.Add(-1 * time.Minute), Value: 75},
	})

	result := monitor.GetStorageMetricsForChart("storage-1", duration)
	if got, wantMin := chartMapCoverageSpan(result), 45*time.Minute; got < wantMin {
		t.Fatalf("expected store-backed storage coverage >= %s, got %s", wantMin, got)
	}
}

func TestGetPhysicalDiskTemperatureCharts_UsesUnifiedReadStateDiskViews(t *testing.T) {
	t.Parallel()

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "disk-1",
				Node:        "node-a",
				Instance:    "lab-a",
				DevPath:     "/dev/nvme0n1",
				Model:       "Samsung PM9A3",
				Serial:      "SERIAL-DISK-1",
				Temperature: 42,
				LastChecked: time.Now().UTC(),
			},
		},
	})

	monitor := &Monitor{
		state:         models.NewState(),
		resourceStore: unifiedresources.NewMonitorAdapter(registry),
	}

	charts := monitor.GetPhysicalDiskTemperatureCharts(30 * time.Minute)
	entry, ok := charts["SERIAL-DISK-1"]
	if !ok {
		t.Fatalf("expected chart entry for canonical disk metric ID, got %#v", charts)
	}
	if entry.Name != "Samsung PM9A3" {
		t.Fatalf("chart name = %q, want Samsung PM9A3", entry.Name)
	}
	if entry.Node != "node-a" {
		t.Fatalf("chart node = %q, want node-a", entry.Node)
	}
	if entry.Instance != "lab-a" {
		t.Fatalf("chart instance = %q, want lab-a", entry.Instance)
	}
	if len(entry.Temperature) != 2 {
		t.Fatalf("expected padded 2-point sparkline series, got %d", len(entry.Temperature))
	}
	for _, point := range entry.Temperature {
		if point.Value != 42 {
			t.Fatalf("expected canonical disk temperature 42 in padded series, got %.2f", point.Value)
		}
	}
}

func TestGetGuestMetricsForChart_UsesGapFillLookbackWhenRequestedRangeIsEmpty(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	writeRawMetricBatch(t, monitor.metricsStore, "agent", "host-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-4 * time.Hour), Value: 37},
		{Timestamp: now.Add(-3*time.Hour - 30*time.Minute), Value: 39},
		{Timestamp: now.Add(-3 * time.Hour), Value: 42},
	})

	result := monitor.GetGuestMetricsForChart("agent:host-1", "agent", "host-1", duration)
	cpu := result["cpu"]
	if len(cpu) == 0 {
		t.Fatal("expected gap-fill fallback to return historical cpu points")
	}
	if cpu[len(cpu)-1].Timestamp.Before(now.Add(-4*time.Hour)) || cpu[len(cpu)-1].Timestamp.After(now.Add(-3*time.Hour+time.Minute)) {
		t.Fatalf("expected latest fallback point to come from older store data window, got %s", cpu[len(cpu)-1].Timestamp)
	}
}

func TestGetNodeMetricsForChart_UsesGapFillLookbackWhenRequestedRangeIsEmpty(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour

	writeRawMetricBatch(t, monitor.metricsStore, "node", "node-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-5 * time.Hour), Value: 21},
		{Timestamp: now.Add(-4*time.Hour - 30*time.Minute), Value: 24},
		{Timestamp: now.Add(-4 * time.Hour), Value: 26},
	})

	result := monitor.GetNodeMetricsForChart("node-1", "cpu", duration)
	if len(result) == 0 {
		t.Fatal("expected gap-fill fallback to return historical node cpu points")
	}
	if result[len(result)-1].Timestamp.Before(now.Add(-5*time.Hour)) || result[len(result)-1].Timestamp.After(now.Add(-4*time.Hour+time.Minute)) {
		t.Fatalf("expected latest fallback node point to come from older store data window, got %s", result[len(result)-1].Timestamp)
	}
}

// ---------- Batch method tests ----------

func TestGetGuestMetricsForChartBatch_BatchesStoreQueries(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour // beyond inMemoryChartThreshold → forces store path

	// Write store data for 3 guests.
	for _, id := range []string{"vm-1", "vm-2", "vm-3"} {
		writeRawMetricBatch(t, monitor.metricsStore, "vm", id, "cpu", []MetricPoint{
			{Timestamp: now.Add(-3 * time.Hour), Value: 10},
			{Timestamp: now.Add(-2 * time.Hour), Value: 20},
			{Timestamp: now.Add(-1 * time.Hour), Value: 30},
		})
		writeRawMetricBatch(t, monitor.metricsStore, "vm", id, "memory", []MetricPoint{
			{Timestamp: now.Add(-3 * time.Hour), Value: 50},
			{Timestamp: now.Add(-1 * time.Hour), Value: 60},
		})
	}

	requests := []GuestChartRequest{
		{InMemoryKey: "vm-1", SQLResourceID: "vm-1"},
		{InMemoryKey: "vm-2", SQLResourceID: "vm-2"},
		{InMemoryKey: "vm-3", SQLResourceID: "vm-3"},
	}
	result := monitor.GetGuestMetricsForChartBatch("vm", requests, duration)

	// Verify all 3 guests have data.
	for _, id := range []string{"vm-1", "vm-2", "vm-3"} {
		metrics, ok := result[id]
		if !ok {
			t.Fatalf("missing result for %s", id)
		}
		if len(metrics["cpu"]) == 0 {
			t.Errorf("%s: expected cpu points from store, got none", id)
		}
		if len(metrics["memory"]) == 0 {
			t.Errorf("%s: expected memory points from store, got none", id)
		}
	}
}

func TestGetGuestMetricsForChartBatch_PrefersInMemoryForShortRanges(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := time.Hour // short range

	// Seed sufficient in-memory coverage.
	for _, offset := range []time.Duration{-55 * time.Minute, -30 * time.Minute, -5 * time.Minute} {
		monitor.metricsHistory.AddGuestMetric("vm-1", "cpu", 11, now.Add(offset))
	}

	// Write different values to store to distinguish sources.
	writeRawMetricBatch(t, monitor.metricsStore, "vm", "vm-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-55 * time.Minute), Value: 99},
		{Timestamp: now.Add(-30 * time.Minute), Value: 99},
		{Timestamp: now.Add(-5 * time.Minute), Value: 99},
	})

	requests := []GuestChartRequest{{InMemoryKey: "vm-1", SQLResourceID: "vm-1"}}
	result := monitor.GetGuestMetricsForChartBatch("vm", requests, duration)

	cpuPoints := result["vm-1"]["cpu"]
	if len(cpuPoints) == 0 {
		t.Fatal("expected non-empty cpu series")
	}
	// In-memory values are 11; store values are 99. Should prefer in-memory.
	if cpuPoints[0].Value != 11 {
		t.Fatalf("expected in-memory value 11, got %.2f (store leak)", cpuPoints[0].Value)
	}
}

func TestGetGuestMetricsForChartBatch_EmptyRequests(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	result := monitor.GetGuestMetricsForChartBatch("vm", nil, time.Hour)
	if result != nil {
		t.Fatalf("expected nil for empty requests, got %v", result)
	}
}

func TestGetGuestMetricsForChartBatch_DifferentInMemoryAndSQLKeys(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour

	// Docker containers use "docker:<id>" as in-memory key but just "<id>" as SQL ID.
	monitor.metricsHistory.AddGuestMetric("docker:dc1", "cpu", 5, now.Add(-10*time.Minute))

	writeRawMetricBatch(t, monitor.metricsStore, "dockerContainer", "dc1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-3 * time.Hour), Value: 40},
		{Timestamp: now.Add(-2 * time.Hour), Value: 45},
		{Timestamp: now.Add(-1 * time.Hour), Value: 50},
	})

	requests := []GuestChartRequest{{InMemoryKey: "docker:dc1", SQLResourceID: "dc1"}}
	result := monitor.GetGuestMetricsForChartBatch("dockerContainer", requests, duration)

	cpuPoints := result["dc1"]["cpu"]
	if len(cpuPoints) == 0 {
		t.Fatal("expected store-backed cpu points for docker container")
	}
}

func TestGetGuestMetricsForChartBatch_PicksBestAliasCandidate(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour

	// Write sparse data under "dockerContainer" (canonical) and better data
	// under the legacy "docker" alias. The batch path should pick the alias
	// with better coverage, matching single-resource alias resolution.
	writeRawMetricBatch(t, monitor.metricsStore, "dockerContainer", "dc1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-10 * time.Minute), Value: 1},
	})
	writeRawMetricBatch(t, monitor.metricsStore, "docker", "dc1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-3 * time.Hour), Value: 40},
		{Timestamp: now.Add(-2 * time.Hour), Value: 45},
		{Timestamp: now.Add(-1 * time.Hour), Value: 50},
	})

	requests := []GuestChartRequest{{InMemoryKey: "docker:dc1", SQLResourceID: "dc1"}}
	result := monitor.GetGuestMetricsForChartBatch("dockerContainer", requests, duration)

	cpuPoints := result["dc1"]["cpu"]
	if len(cpuPoints) < 2 {
		t.Fatalf("expected multi-point series from best alias, got %d points", len(cpuPoints))
	}
	span := chartSeriesCoverageSpan(cpuPoints)
	if span < time.Hour {
		t.Fatalf("expected coverage span >= 1h from legacy alias, got %s", span)
	}
}

func TestGetNodeMetricsForChartBatch_BatchesMultipleNodesAndMetricTypes(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour

	// Write store data for 2 nodes with multiple metric types.
	for _, nid := range []string{"node-1", "node-2"} {
		for _, mt := range []string{"cpu", "memory", "disk"} {
			writeRawMetricBatch(t, monitor.metricsStore, "node", nid, mt, []MetricPoint{
				{Timestamp: now.Add(-3 * time.Hour), Value: 10},
				{Timestamp: now.Add(-2 * time.Hour), Value: 20},
				{Timestamp: now.Add(-1 * time.Hour), Value: 30},
			})
		}
	}

	metricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	result := monitor.GetNodeMetricsForChartBatch([]string{"node-1", "node-2"}, metricTypes, duration)

	for _, nid := range []string{"node-1", "node-2"} {
		nodeMetrics, ok := result[nid]
		if !ok {
			t.Fatalf("missing result for %s", nid)
		}
		for _, mt := range []string{"cpu", "memory", "disk"} {
			if len(nodeMetrics[mt]) == 0 {
				t.Errorf("%s/%s: expected points from store, got none", nid, mt)
			}
		}
	}
}

func TestGetNodeMetricsForChartBatch_EmptyNodeIDs(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	result := monitor.GetNodeMetricsForChartBatch(nil, []string{"cpu"}, time.Hour)
	if result != nil {
		t.Fatalf("expected nil for empty nodeIDs, got %v", result)
	}
}

func TestGetStorageMetricsForChartBatch_BatchesMultiplePools(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	duration := 4 * time.Hour

	for _, sid := range []string{"storage-1", "storage-2"} {
		writeRawMetricBatch(t, monitor.metricsStore, "storage", sid, "usage", []MetricPoint{
			{Timestamp: now.Add(-3 * time.Hour), Value: 60},
			{Timestamp: now.Add(-2 * time.Hour), Value: 65},
			{Timestamp: now.Add(-1 * time.Hour), Value: 70},
		})
	}

	result := monitor.GetStorageMetricsForChartBatch([]string{"storage-1", "storage-2"}, duration)

	for _, sid := range []string{"storage-1", "storage-2"} {
		metrics, ok := result[sid]
		if !ok {
			t.Fatalf("missing result for %s", sid)
		}
		if len(metrics["usage"]) == 0 {
			t.Errorf("%s: expected usage points from store, got none", sid)
		}
	}
}

func TestGetStorageMetricsForChartBatch_EmptyStorageIDs(t *testing.T) {
	t.Parallel()

	monitor := newChartFallbackTestMonitor(t)
	result := monitor.GetStorageMetricsForChartBatch(nil, time.Hour)
	if result != nil {
		t.Fatalf("expected nil for empty storageIDs, got %v", result)
	}
}
