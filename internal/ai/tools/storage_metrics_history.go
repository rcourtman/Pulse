package tools

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// storageMetricHistorySource exposes the monitor-owned history selection used
// by charts. The source label distinguishes generated fixtures from observations.
type storageMetricHistorySource interface {
	GetStorageMetricHistory(string, time.Duration) (map[string][]RawMetricPoint, string, error)
}

func (a *MetricsHistoryToolAdapter) GetStorageMetricHistory(target unifiedresources.MetricsTarget, period time.Duration) ([]RawMetricPoint, string, error) {
	if source, ok := a.metricsSource.(storageMetricHistorySource); ok {
		series, provenance, err := source.GetStorageMetricHistory(target.ResourceID, period)
		return series["usage"], provenance, err
	}
	// Compatibility for adapters without the monitor chart boundary. Keep the
	// requested retained window and propagate store errors, just like guest reads.
	if a.retainedMetrics == nil {
		return nil, "unavailable", nil
	}
	end := time.Now()
	points, err := a.retainedMetrics.Query("storage", target.ResourceID, "usage", end.Add(-period), end, 0)
	if err != nil {
		return nil, "", err
	}
	result := make([]RawMetricPoint, 0, len(points))
	for _, point := range points {
		result = append(result, RawMetricPoint{Timestamp: point.Timestamp, Value: point.Value})
	}
	return result, "store", nil
}

func (e *PulseToolExecutor) storageMetricsHistory(resourceID string, target unifiedresources.MetricsTarget, period string, duration time.Duration) (CallToolResult, error) {
	provider, ok := e.metricsHistory.(interface {
		GetStorageMetricHistory(unifiedresources.MetricsTarget, time.Duration) ([]RawMetricPoint, string, error)
	})
	if !ok {
		return NewErrorResult(fmt.Errorf("storage metrics history provider is not available for resource %s", resourceID)), nil
	}
	points, source, err := provider.GetStorageMetricHistory(target, duration)
	if err != nil {
		return NewErrorResult(err), nil
	}
	originalCount := len(points)
	if len(points) > maxMetricPoints {
		values := make([]MetricPoint, len(points))
		for i, point := range points {
			values[i] = MetricPoint{Timestamp: point.Timestamp, Disk: point.Value}
		}
		values = downsampleMetrics(values, maxMetricPoints)
		points = make([]RawMetricPoint, len(values))
		for i, value := range values {
			points[i] = RawMetricPoint{Timestamp: value.Timestamp, Value: value.Disk}
		}
	}
	if points == nil {
		points = []RawMetricPoint{}
	}
	// A storage series has no CPU/memory measurements. Do not manufacture zero
	// observations by projecting it through the guest performance point shape.
	return NewJSONResult(struct {
		ResourceID    string           `json:"resource_id"`
		ResourceType  string           `json:"resource_type"`
		Metric        string           `json:"metric"`
		Unit          string           `json:"unit"`
		Period        string           `json:"period"`
		Source        string           `json:"source"`
		Points        []RawMetricPoint `json:"points"`
		Downsampled   bool             `json:"downsampled"`
		OriginalCount int              `json:"original_count"`
	}{resourceID, "storage", "usage", "percent", period, source, points, originalCount > len(points), originalCount}), nil
}
