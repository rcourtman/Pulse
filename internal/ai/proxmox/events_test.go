package proxmox

import (
	"testing"
	"time"
)

func TestEventCorrelator_RecordEvent(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	event := ProxmoxEvent{
		Type:       EventMigrationStart,
		Node:       "pve1",
		ResourceID: "vm-101",
		TargetNode: "pve2",
		Timestamp:  time.Now(),
	}

	correlator.RecordEvent(event)

	events := correlator.GetRecentEvents(1 * time.Hour)
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].Type != EventMigrationStart {
		t.Errorf("Expected migration start event")
	}
}

func TestEventCorrelator_OperationWindow(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	// Start a migration
	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventMigrationStart,
		Node:       "pve1",
		ResourceID: "vm-101",
		TargetNode: "pve2",
	})

	activeOps := correlator.GetActiveOperations()
	if len(activeOps) != 1 {
		t.Errorf("Expected 1 active operation, got %d", len(activeOps))
	}

	if activeOps[0].EventType != EventMigrationStart {
		t.Errorf("Expected migration operation")
	}
}

func TestEventCorrelator_RecordAnomaly(t *testing.T) {
	cfg := DefaultEventCorrelatorConfig()
	cfg.CorrelationWindow = 5 * time.Minute
	correlator := NewEventCorrelator(cfg)

	// Record a migration event
	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventMigrationStart,
		Node:       "pve1",
		ResourceID: "vm-101",
		TargetNode: "pve2",
		Timestamp:  time.Now(),
	})

	// Record an anomaly shortly after
	anomaly := MetricAnomaly{
		ResourceID: "vm-101",
		Metric:     "cpu",
		Value:      95,
		Baseline:   50,
		Deviation:  4.5,
		Timestamp:  time.Now().Add(30 * time.Second),
	}

	correlation := correlator.RecordAnomaly(anomaly)

	// Should find correlation with migration
	if correlation == nil {
		t.Error("Expected correlation to be found")
	}

	if correlation != nil && correlation.Event.Type != EventMigrationStart {
		t.Errorf("Expected correlation with migration event")
	}
}

func TestEventCorrelator_NoCorrelation(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	// Record anomaly with no recent events
	anomaly := MetricAnomaly{
		ResourceID: "vm-101",
		Metric:     "cpu",
		Value:      95,
		Baseline:   50,
		Timestamp:  time.Now(),
	}

	correlation := correlator.RecordAnomaly(anomaly)

	if correlation != nil {
		t.Error("Expected no correlation when no events exist")
	}
}

func TestEventCorrelator_GetEventsForResource(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventBackupStart,
		ResourceID: "vm-101",
		Storage:    "local-zfs",
	})
	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventVMStart,
		ResourceID: "vm-102",
	})
	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventSnapshotCreate,
		ResourceID: "vm-101",
		Storage:    "local-zfs",
	})

	events := correlator.GetEventsForResource("vm-101", 10)
	if len(events) != 2 {
		t.Errorf("Expected 2 events for vm-101, got %d", len(events))
	}
}

func TestEventCorrelator_GetCorrelations(t *testing.T) {
	cfg := DefaultEventCorrelatorConfig()
	correlator := NewEventCorrelator(cfg)

	// Create event and correlated anomaly
	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventBackupStart,
		ResourceID: "vm-101",
		Storage:    "local-zfs",
	})

	correlator.RecordAnomaly(MetricAnomaly{
		ResourceID: "vm-101",
		Metric:     "io",
		Value:      100,
		Baseline:   20,
		Timestamp:  time.Now(),
	})

	correlations := correlator.GetCorrelations(10)
	if len(correlations) != 1 {
		t.Errorf("Expected 1 correlation, got %d", len(correlations))
	}
}

func TestEventCorrelator_FormatForPatrol(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	correlator.RecordEvent(ProxmoxEvent{
		Type:         EventBackupStart,
		ResourceID:   "vm-101",
		ResourceName: "web-server",
		Storage:      "local-zfs",
		Status:       "running",
	})

	context := correlator.FormatForPatrol(1 * time.Hour)

	if context == "" {
		t.Error("Expected non-empty context")
	}

	if !containsStr(context, "Proxmox Operations") {
		t.Error("Expected 'Proxmox Operations' in context")
	}

	if !containsStr(context, "Backup") {
		t.Error("Expected 'Backup' in context")
	}
}

func TestEventCorrelator_FormatForPatrol_NoEvents(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	context := correlator.FormatForPatrol(1 * time.Hour)

	if context != "" {
		t.Error("Expected empty context with no events")
	}
}

func TestEventCorrelator_FormatForResource(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	correlator.RecordEvent(ProxmoxEvent{
		Type:       EventMigrationStart,
		ResourceID: "vm-101",
		Node:       "pve1",
		TargetNode: "pve2",
	})

	context := correlator.FormatForResource("vm-101")

	if context == "" {
		t.Error("Expected non-empty context")
	}
}

func TestEventCorrelator_CorrelationConfidence(t *testing.T) {
	correlator := NewEventCorrelator(DefaultEventCorrelatorConfig())

	event := ProxmoxEvent{
		Type:       EventMigrationStart,
		ResourceID: "vm-101",
		Node:       "pve1",
	}

	// Direct match should have higher confidence
	anomalyDirect := MetricAnomaly{
		ResourceID: "vm-101", // Same as event
		Metric:     "cpu",
		Timestamp:  time.Now(),
	}

	confidenceDirect := correlator.calculateCorrelationConfidence(event, anomalyDirect)

	// Indirect match
	anomalyIndirect := MetricAnomaly{
		ResourceID: "vm-999", // Different resource
		Metric:     "disk",   // Unexpected metric
		Timestamp:  time.Now(),
	}

	confidenceIndirect := correlator.calculateCorrelationConfidence(event, anomalyIndirect)

	if confidenceIndirect >= confidenceDirect {
		t.Errorf("Expected direct match to have higher confidence: direct=%.2f, indirect=%.2f",
			confidenceDirect, confidenceIndirect)
	}
}

func TestIsOngoingOperation(t *testing.T) {
	tests := []struct {
		eventType ProxmoxEventType
		expected  bool
	}{
		{EventMigrationStart, true},
		{EventBackupStart, true},
		{EventSnapshotCreate, true},
		{EventMigrationEnd, false},
		{EventVMStart, false},
	}

	for _, tt := range tests {
		result := isOngoingOperation(tt.eventType)
		if result != tt.expected {
			t.Errorf("isOngoingOperation(%s) = %v, want %v", tt.eventType, result, tt.expected)
		}
	}
}

func TestIsEndOperation(t *testing.T) {
	tests := []struct {
		eventType ProxmoxEventType
		expected  bool
	}{
		{EventMigrationEnd, true},
		{EventBackupEnd, true},
		{EventMigrationStart, false},
		{EventVMStop, false},
	}

	for _, tt := range tests {
		result := isEndOperation(tt.eventType)
		if result != tt.expected {
			t.Errorf("isEndOperation(%s) = %v, want %v", tt.eventType, result, tt.expected)
		}
	}
}

func TestEstimateOperationDuration(t *testing.T) {
	tests := []struct {
		eventType ProxmoxEventType
		minDur    time.Duration
	}{
		{EventMigrationStart, 10 * time.Minute},
		{EventBackupStart, 30 * time.Minute},
		{EventSnapshotCreate, 5 * time.Minute},
	}

	for _, tt := range tests {
		result := estimateOperationDuration(tt.eventType)
		if result < tt.minDur {
			t.Errorf("estimateOperationDuration(%s) = %v, want >= %v", tt.eventType, result, tt.minDur)
		}
	}
}

func TestGetExpectedMetrics(t *testing.T) {
	metrics := getExpectedMetrics(EventMigrationStart)
	if len(metrics) == 0 {
		t.Error("Expected some metrics for migration")
	}

	foundCPU := false
	for _, m := range metrics {
		if m == "cpu" {
			foundCPU = true
		}
	}
	if !foundCPU {
		t.Error("Expected 'cpu' metric for migration")
	}
}

func TestFormatEventType(t *testing.T) {
	tests := []struct {
		eventType ProxmoxEventType
		expected  string
	}{
		{EventMigrationStart, "Migration started"},
		{EventBackupEnd, "Backup completed"},
		{EventHAFailover, "HA failover"},
	}

	for _, tt := range tests {
		result := formatEventType(tt.eventType)
		if result != tt.expected {
			t.Errorf("formatEventType(%s) = %s, want %s", tt.eventType, result, tt.expected)
		}
	}
}

func TestSortEventsByTimestamp(t *testing.T) {
	now := time.Now()
	events := []ProxmoxEvent{
		{ID: "1", Timestamp: now.Add(-2 * time.Hour)},
		{ID: "2", Timestamp: now},
		{ID: "3", Timestamp: now.Add(-1 * time.Hour)},
	}

	SortEventsByTimestamp(events)

	// Should be newest first
	if events[0].ID != "2" {
		t.Errorf("Expected newest first, got ID=%s", events[0].ID)
	}
}

// Helper
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
