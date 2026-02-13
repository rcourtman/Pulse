package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// inMemoryChartThreshold is the max duration reliably covered by the in-memory
// metrics buffer. With 1000 data-points at a 10 s polling interval the buffer
// holds ~2.8 h of data; 2 h is a safe conservative cut-off.
const inMemoryChartThreshold = 2 * time.Hour

// chartDownsampleTarget is the number of points returned per metric when
// falling back to the persistent store. 500 is more than enough for any
// sparkline or thumbnail chart.
const chartDownsampleTarget = 500

const (
	// When the requested chart window has no data (for example after running in
	// mock mode for a while), query a broader historical window so sparklines
	// can render immediately instead of waiting for enough fresh samples.
	chartGapFillLookbackMultiplier = 12
	chartGapFillLookbackMin        = 6 * time.Hour
	chartGapFillLookbackMax        = 7 * 24 * time.Hour
)

// For short chart ranges (<= inMemoryChartThreshold), we prefer in-memory data
// for freshness unless coverage is too shallow (for example after a restart).
// In that case we fall back to SQLite history to avoid mostly-empty charts.
const (
	shortRangeCoverageRatioNum = 3
	shortRangeCoverageRatioDen = 4
	shortRangeCoverageMaxSlack = 2 * time.Minute
)

// GetGuestMetrics returns historical metrics for a guest
func (m *Monitor) GetGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllGuestMetrics(guestID, duration)
}

// GetGuestMetricsForChart returns guest metrics optimised for chart display.
// Short ranges are served from the in-memory ring buffer; longer ranges fall
// back to the persistent SQLite store with LTTB downsampling.
//
// inMemoryKey is the key used in the in-memory buffer (e.g. "docker:abc123").
// sqlResourceType/sqlResourceID are the type/id used in the SQLite store
// (e.g. "dockerContainer"/"abc123").
func (m *Monitor) GetGuestMetricsForChart(inMemoryKey, sqlResourceType, sqlResourceID string, duration time.Duration) map[string][]MetricPoint {
	inMemoryResult := m.metricsHistory.GetAllGuestMetrics(inMemoryKey, duration)
	if m.metricsStore == nil {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && hasSufficientChartMapCoverage(inMemoryResult, duration) {
		return inMemoryResult
	}

	converted, ok := m.queryStoreMetricMapWithGapFill(sqlResourceType, sqlResourceID, duration)
	if !ok {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && chartMapCoverageSpan(converted) <= chartMapCoverageSpan(inMemoryResult) {
		return inMemoryResult
	}
	return converted
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetNodeMetricsForChart returns node metrics for a single metric type,
// falling back to SQLite + LTTB for longer ranges.
func (m *Monitor) GetNodeMetricsForChart(nodeID, metricType string, duration time.Duration) []MetricPoint {
	inMemoryPoints := m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
	if m.metricsStore == nil {
		return inMemoryPoints
	}
	if duration <= inMemoryChartThreshold && hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		return inMemoryPoints
	}

	downsampled, ok := m.queryStoreMetricSeriesWithGapFill("node", nodeID, metricType, duration)
	if !ok {
		return inMemoryPoints
	}
	if duration <= inMemoryChartThreshold && chartSeriesCoverageSpan(downsampled) <= chartSeriesCoverageSpan(inMemoryPoints) {
		return inMemoryPoints
	}
	return downsampled
}

// GetStorageMetrics returns historical metrics for storage
func (m *Monitor) GetStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
}

// GetStorageMetricsForChart returns storage metrics, falling back to SQLite +
// LTTB for longer ranges.
func (m *Monitor) GetStorageMetricsForChart(storageID string, duration time.Duration) map[string][]MetricPoint {
	inMemoryResult := m.metricsHistory.GetAllStorageMetrics(storageID, duration)
	if m.metricsStore == nil {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && hasSufficientChartMapCoverage(inMemoryResult, duration) {
		return inMemoryResult
	}

	converted, ok := m.queryStoreMetricMapWithGapFill("storage", storageID, duration)
	if !ok {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && chartMapCoverageSpan(converted) <= chartMapCoverageSpan(inMemoryResult) {
		return inMemoryResult
	}
	return converted
}

func chartGapFillLookbackWindow(duration time.Duration) time.Duration {
	lookback := duration * chartGapFillLookbackMultiplier
	if lookback < chartGapFillLookbackMin {
		lookback = chartGapFillLookbackMin
	}
	if lookback > chartGapFillLookbackMax {
		lookback = chartGapFillLookbackMax
	}
	return lookback
}

func convertSeriesAndDownsample(points []metrics.MetricPoint, target int) []MetricPoint {
	converted := make([]MetricPoint, len(points))
	for i, p := range points {
		converted[i] = MetricPoint{Value: p.Value, Timestamp: p.Timestamp}
	}
	return lttb(converted, target)
}

func (m *Monitor) queryStoreMetricMapWithGapFill(resourceType, resourceID string, duration time.Duration) (map[string][]MetricPoint, bool) {
	if m == nil || m.metricsStore == nil {
		return nil, false
	}

	end := time.Now()
	start := end.Add(-duration)

	query := func(from time.Time) (map[string][]MetricPoint, bool) {
		sqlResult, err := m.metricsStore.QueryAll(resourceType, resourceID, from, end, 0)
		if err != nil || len(sqlResult) == 0 {
			return nil, false
		}
		return convertAndDownsample(sqlResult, chartDownsampleTarget), true
	}

	if result, ok := query(start); ok {
		return result, true
	}

	return query(end.Add(-chartGapFillLookbackWindow(duration)))
}

func (m *Monitor) queryStoreMetricSeriesWithGapFill(resourceType, resourceID, metricType string, duration time.Duration) ([]MetricPoint, bool) {
	if m == nil || m.metricsStore == nil {
		return nil, false
	}

	end := time.Now()
	start := end.Add(-duration)

	query := func(from time.Time) ([]MetricPoint, bool) {
		sqlPoints, err := m.metricsStore.Query(resourceType, resourceID, metricType, from, end, 0)
		if err != nil || len(sqlPoints) == 0 {
			return nil, false
		}
		return convertSeriesAndDownsample(sqlPoints, chartDownsampleTarget), true
	}

	if points, ok := query(start); ok {
		return points, true
	}

	return query(end.Add(-chartGapFillLookbackWindow(duration)))
}

// convertAndDownsample converts pkg/metrics.MetricPoint slices to
// internal/monitoring.MetricPoint slices and applies LTTB downsampling.
func convertAndDownsample(sqlResult map[string][]metrics.MetricPoint, target int) map[string][]MetricPoint {
	result := make(map[string][]MetricPoint, len(sqlResult))
	for metric, points := range sqlResult {
		converted := make([]MetricPoint, len(points))
		for i, p := range points {
			converted[i] = MetricPoint{Value: p.Value, Timestamp: p.Timestamp}
		}
		result[metric] = lttb(converted, target)
	}
	return result
}

func chartCoverageRequiredSpan(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}

	required := (duration * shortRangeCoverageRatioNum) / shortRangeCoverageRatioDen
	slack := duration / 10
	if slack > shortRangeCoverageMaxSlack {
		slack = shortRangeCoverageMaxSlack
	}
	if required <= slack {
		return 0
	}
	return required - slack
}

func chartSeriesCoverageSpan(points []MetricPoint) time.Duration {
	if len(points) < 2 {
		return 0
	}

	oldest := points[0].Timestamp
	newest := points[0].Timestamp
	for i := 1; i < len(points); i++ {
		ts := points[i].Timestamp
		if ts.Before(oldest) {
			oldest = ts
		}
		if ts.After(newest) {
			newest = ts
		}
	}
	if !newest.After(oldest) {
		return 0
	}
	return newest.Sub(oldest)
}

func chartMapCoverageSpan(metrics map[string][]MetricPoint) time.Duration {
	var best time.Duration
	for _, points := range metrics {
		span := chartSeriesCoverageSpan(points)
		if span > best {
			best = span
		}
	}
	return best
}

func hasSufficientChartSeriesCoverage(points []MetricPoint, duration time.Duration) bool {
	return chartSeriesCoverageSpan(points) >= chartCoverageRequiredSpan(duration)
}

func hasSufficientChartMapCoverage(metrics map[string][]MetricPoint, duration time.Duration) bool {
	return chartMapCoverageSpan(metrics) >= chartCoverageRequiredSpan(duration)
}
