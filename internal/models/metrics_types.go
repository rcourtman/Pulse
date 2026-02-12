package models

import "time"

// MetricPoint represents a single metric value at a point in time
type MetricPoint struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// GetTimestamp returns the timestamp for the metric point
func (m MetricPoint) GetTimestamp() time.Time {
	return m.Timestamp
}

// IOMetrics represents I/O metrics at a point in time
type IOMetrics struct {
	DiskRead   int64
	DiskWrite  int64
	NetworkIn  int64
	NetworkOut int64
	Timestamp  time.Time
}
