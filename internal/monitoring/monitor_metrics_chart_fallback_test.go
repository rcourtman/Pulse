package monitoring

import (
	"testing"
	"time"

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

	inMemoryKey := "host:host-1"
	monitor.metricsHistory.AddGuestMetric(inMemoryKey, "cpu", 15, now.Add(-4*time.Minute))
	monitor.metricsHistory.AddGuestMetric(inMemoryKey, "cpu", 18, now.Add(-1*time.Minute))

	writeRawMetricBatch(t, monitor.metricsStore, "host", "host-1", "cpu", []MetricPoint{
		{Timestamp: now.Add(-58 * time.Minute), Value: 41},
		{Timestamp: now.Add(-30 * time.Minute), Value: 43},
		{Timestamp: now.Add(-1 * time.Minute), Value: 46},
	})

	result := monitor.GetGuestMetricsForChart(inMemoryKey, "host", "host-1", duration)
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
