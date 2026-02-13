package models

import "time"

// MetricPoint represents a single metric value at a point in time
type MetricPoint struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// IOMetrics represents I/O metrics at a point in time
type IOMetrics struct {
	DiskRead   int64
	DiskWrite  int64
	NetworkIn  int64
	NetworkOut int64
	Timestamp  time.Time
}
