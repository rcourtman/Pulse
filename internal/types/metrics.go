package types

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
	DiskRead   int64     `json:"diskRead"`
	DiskWrite  int64     `json:"diskWrite"`
	NetworkIn  int64     `json:"networkIn"`
	NetworkOut int64     `json:"networkOut"`
	Timestamp  time.Time `json:"timestamp"`
}
