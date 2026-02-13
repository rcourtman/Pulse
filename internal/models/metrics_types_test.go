package models

import (
	"encoding/json"
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

func TestIOMetrics_JSONSerializationUsesCamelCaseFields(t *testing.T) {
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	metrics := IOMetrics{
		DiskRead:   1000,
		DiskWrite:  2000,
		NetworkIn:  3000,
		NetworkOut: 4000,
		Timestamp:  now,
	}

	payload, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded["diskRead"] != float64(1000) {
		t.Errorf("diskRead = %v, want 1000", decoded["diskRead"])
	}
	if decoded["diskWrite"] != float64(2000) {
		t.Errorf("diskWrite = %v, want 2000", decoded["diskWrite"])
	}
	if decoded["networkIn"] != float64(3000) {
		t.Errorf("networkIn = %v, want 3000", decoded["networkIn"])
	}
	if decoded["networkOut"] != float64(4000) {
		t.Errorf("networkOut = %v, want 4000", decoded["networkOut"])
	}
	if decoded["timestamp"] != now.Format(time.RFC3339Nano) {
		t.Errorf("timestamp = %v, want %v", decoded["timestamp"], now.Format(time.RFC3339Nano))
	}

	if _, ok := decoded["DiskRead"]; ok {
		t.Error("expected DiskRead key to be absent")
	}
	if _, ok := decoded["DiskWrite"]; ok {
		t.Error("expected DiskWrite key to be absent")
	}
	if _, ok := decoded["NetworkIn"]; ok {
		t.Error("expected NetworkIn key to be absent")
	}
	if _, ok := decoded["NetworkOut"]; ok {
		t.Error("expected NetworkOut key to be absent")
	}
	if _, ok := decoded["Timestamp"]; ok {
		t.Error("expected Timestamp key to be absent")
	}
}
