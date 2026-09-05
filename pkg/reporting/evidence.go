package reporting

import (
	"context"
	"fmt"
	"time"
)

// MetricEvidenceProvider exposes the same retained metrics as reports without
// invoking a narrator or assigning health, urgency, or recommended actions.
type MetricEvidenceProvider interface {
	MetricEvidenceFor(context.Context, MetricReportRequest) (*MetricEvidence, error)
}

type MetricEvidence struct {
	ResourceID   string                         `json:"resource_id"`
	ResourceType string                         `json:"resource_type"`
	GeneratedAt  time.Time                      `json:"generated_at"`
	WindowStart  time.Time                      `json:"window_start"`
	WindowEnd    time.Time                      `json:"window_end"`
	Metrics      map[string]RetainedMetricStats `json:"metrics"`
}

// RetainedMetricStats describes returned points, which may already be retention
// aggregates. Their count and time span do not prove continuous observation.
type RetainedMetricStats struct {
	Unit           string    `json:"unit,omitempty"`
	RetainedPoints int       `json:"retained_points"`
	Min            float64   `json:"min"`
	Max            float64   `json:"max"`
	Mean           float64   `json:"mean"`
	Latest         float64   `json:"latest"`
	MinBucketAt    time.Time `json:"min_bucket_at"`
	MaxBucketAt    time.Time `json:"max_bucket_at"`
	FirstAt        time.Time `json:"first_at"`
	LastAt         time.Time `json:"last_at"`
	MaxGapSeconds  float64   `json:"max_gap_seconds"`
}

func (e *ReportEngine) MetricEvidenceFor(ctx context.Context, req MetricReportRequest) (*MetricEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e.getMetricsStore() == nil {
		return nil, fmt.Errorf("metrics store not initialized")
	}
	data, err := e.queryMetrics(req)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	evidence := &MetricEvidence{
		ResourceID: data.ResourceID, ResourceType: data.ResourceType,
		GeneratedAt: data.GeneratedAt, WindowStart: data.Start, WindowEnd: data.End,
		Metrics: make(map[string]RetainedMetricStats, len(data.Metrics)),
	}
	for name, points := range data.Metrics {
		if len(points) == 0 {
			continue
		}
		stats := data.Summary.ByMetric[name]
		entry := RetainedMetricStats{
			Unit: GetMetricUnit(name), RetainedPoints: len(points),
			Min: points[0].Value, Max: points[0].Value, Mean: stats.Avg, Latest: stats.Current,
			MinBucketAt: points[0].Timestamp, MaxBucketAt: points[0].Timestamp,
			FirstAt: points[0].Timestamp, LastAt: points[len(points)-1].Timestamp,
		}
		for i, point := range points {
			// Retention buckets preserve extrema separately from their mean.
			low, high := point.Value, point.Value
			if point.Min <= point.Value && point.Max >= point.Value {
				low, high = point.Min, point.Max
			}
			if low < entry.Min {
				entry.Min, entry.MinBucketAt = low, point.Timestamp
			}
			if high > entry.Max {
				entry.Max, entry.MaxBucketAt = high, point.Timestamp
			}
			if i > 0 {
				entry.MaxGapSeconds = max(entry.MaxGapSeconds, point.Timestamp.Sub(points[i-1].Timestamp).Seconds())
			}
		}
		evidence.Metrics[name] = entry
	}
	return evidence, nil
}
