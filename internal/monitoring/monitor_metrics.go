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
	if duration <= inMemoryChartThreshold || m.metricsStore == nil {
		return m.metricsHistory.GetAllGuestMetrics(inMemoryKey, duration)
	}
	end := time.Now()
	start := end.Add(-duration)
	sqlResult, err := m.metricsStore.QueryAll(sqlResourceType, sqlResourceID, start, end, 0)
	if err != nil || len(sqlResult) == 0 {
		return m.metricsHistory.GetAllGuestMetrics(inMemoryKey, duration)
	}
	return convertAndDownsample(sqlResult, chartDownsampleTarget)
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetNodeMetricsForChart returns node metrics for a single metric type,
// falling back to SQLite + LTTB for longer ranges.
func (m *Monitor) GetNodeMetricsForChart(nodeID, metricType string, duration time.Duration) []MetricPoint {
	if duration <= inMemoryChartThreshold || m.metricsStore == nil {
		return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
	}
	end := time.Now()
	start := end.Add(-duration)
	sqlPoints, err := m.metricsStore.Query("node", nodeID, metricType, start, end, 0)
	if err != nil || len(sqlPoints) == 0 {
		return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
	}
	converted := make([]MetricPoint, len(sqlPoints))
	for i, p := range sqlPoints {
		converted[i] = MetricPoint{Value: p.Value, Timestamp: p.Timestamp}
	}
	return lttb(converted, chartDownsampleTarget)
}

// GetStorageMetrics returns historical metrics for storage
func (m *Monitor) GetStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
}

// GetStorageMetricsForChart returns storage metrics, falling back to SQLite +
// LTTB for longer ranges.
func (m *Monitor) GetStorageMetricsForChart(storageID string, duration time.Duration) map[string][]MetricPoint {
	if duration <= inMemoryChartThreshold || m.metricsStore == nil {
		return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
	}
	end := time.Now()
	start := end.Add(-duration)
	sqlResult, err := m.metricsStore.QueryAll("storage", storageID, start, end, 0)
	if err != nil || len(sqlResult) == 0 {
		return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
	}
	return convertAndDownsample(sqlResult, chartDownsampleTarget)
}

// convertAndDownsample converts pkg/metrics.MetricPoint slices to
// internal/types.MetricPoint slices and applies LTTB downsampling.
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
