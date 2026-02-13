package models

import (
	"testing"
	"time"
)

func TestMetricPoint_ZeroValue(t *testing.T) {
	var point MetricPoint

	// Zero value should have zero time
	if !point.Timestamp.IsZero() {
		t.Error("Zero MetricPoint should have zero timestamp")
	}
	if point.Value != 0 {
		t.Errorf("Zero MetricPoint Value = %v, want 0", point.Value)
	}
}

func TestIOMetrics_Fields(t *testing.T) {
	now := time.Now()
	metrics := IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  now,
	}

	if metrics.DiskRead != 1000 {
		t.Errorf("DiskRead = %v, want 1000", metrics.DiskRead)
	}
	if metrics.DiskWrite != 2000 {
		t.Errorf("DiskWrite = %v, want 2000", metrics.DiskWrite)
	}
	if metrics.NetworkIn != 3000 {
		t.Errorf("NetworkIn = %v, want 3000", metrics.NetworkIn)
	}
	if metrics.NetworkOut != 4000 {
		t.Errorf("NetworkOut = %v, want 4000", metrics.NetworkOut)
	}
	if !metrics.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", metrics.Timestamp, now)
	}
}

func TestIOMetrics_ZeroValue(t *testing.T) {
	var metrics IOMetrics

	if metrics.DiskRead != 0 {
		t.Error("Zero IOMetrics should have zero DiskRead")
	}
	if metrics.DiskWrite != 0 {
		t.Error("Zero IOMetrics should have zero DiskWrite")
	}
	if metrics.NetworkIn != 0 {
		t.Error("Zero IOMetrics should have zero NetworkIn")
	}
	if metrics.NetworkOut != 0 {
		t.Error("Zero IOMetrics should have zero NetworkOut")
	}
	if !metrics.Timestamp.IsZero() {
		t.Error("Zero IOMetrics should have zero Timestamp")
	}
}
