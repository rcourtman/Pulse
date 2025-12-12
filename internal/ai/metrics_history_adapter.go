package ai

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// MetricsHistoryAdapter adapts monitoring.MetricsHistory to the MetricsHistoryProvider interface
// This allows the patrol service to use the monitoring package's metrics history
// without creating a direct package dependency
type MetricsHistoryAdapter struct {
	history *monitoring.MetricsHistory
}

// NewMetricsHistoryAdapter creates an adapter for the monitoring.MetricsHistory
func NewMetricsHistoryAdapter(history *monitoring.MetricsHistory) *MetricsHistoryAdapter {
	if history == nil {
		return nil
	}
	return &MetricsHistoryAdapter{history: history}
}

// GetNodeMetrics returns historical metrics for a node
func (a *MetricsHistoryAdapter) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	if a.history == nil {
		return nil
	}
	points := a.history.GetNodeMetrics(nodeID, metricType, duration)
	return convertMetricPoints(points)
}

// GetGuestMetrics returns historical metrics for a guest
func (a *MetricsHistoryAdapter) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []MetricPoint {
	if a.history == nil {
		return nil
	}
	points := a.history.GetGuestMetrics(guestID, metricType, duration)
	return convertMetricPoints(points)
}

// GetAllGuestMetrics returns all metrics for a guest
func (a *MetricsHistoryAdapter) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	if a.history == nil {
		return nil
	}
	metricsMap := a.history.GetAllGuestMetrics(guestID, duration)
	return convertMetricsMap(metricsMap)
}

// GetAllStorageMetrics returns all metrics for storage
func (a *MetricsHistoryAdapter) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	if a.history == nil {
		return nil
	}
	metricsMap := a.history.GetAllStorageMetrics(storageID, duration)
	return convertMetricsMap(metricsMap)
}

// convertMetricPoints converts from monitoring.MetricPoint to ai.MetricPoint
func convertMetricPoints(points []monitoring.MetricPoint) []MetricPoint {
	if points == nil {
		return nil
	}
	result := make([]MetricPoint, len(points))
	for i, p := range points {
		result[i] = MetricPoint{
			Value:     p.Value,
			Timestamp: p.Timestamp,
		}
	}
	return result
}

// convertMetricsMap converts a map of metric types to their points
func convertMetricsMap(metricsMap map[string][]monitoring.MetricPoint) map[string][]MetricPoint {
	if metricsMap == nil {
		return nil
	}
	result := make(map[string][]MetricPoint, len(metricsMap))
	for key, points := range metricsMap {
		result[key] = convertMetricPoints(points)
	}
	return result
}
