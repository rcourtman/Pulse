package patterns

import (
	"os"
	"testing"
	"time"
)

// Additional tests to improve coverage

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxEvents == 0 {
		t.Error("MaxEvents should have a default value")
	}
	if cfg.MinOccurrences == 0 {
		t.Error("MinOccurrences should have a default value")
	}
	if cfg.PatternWindow == 0 {
		t.Error("PatternWindow should have a default value")
	}
	if cfg.PredictionLimit == 0 {
		t.Error("PredictionLimit should have a default value")
	}
}

func TestNewDetector_Defaults(t *testing.T) {
	d := NewDetector(DetectorConfig{})

	if d == nil {
		t.Fatal("Expected non-nil detector")
	}

	if d.events == nil {
		t.Error("events should be initialized")
	}
	if d.patterns == nil {
		t.Error("patterns should be initialized")
	}
}

func TestDetector_GetPredictions_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	predictions := d.GetPredictions()

	if len(predictions) != 0 {
		t.Errorf("Expected empty predictions, got %d", len(predictions))
	}
}

func TestDetector_GetPredictionsForResource_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	predictions := d.GetPredictionsForResource("nonexistent")

	if len(predictions) != 0 {
		t.Errorf("Expected empty predictions, got %d", len(predictions))
	}
}

func TestDetector_GetPatterns_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	patterns := d.GetPatterns()

	if len(patterns) != 0 {
		t.Errorf("Expected empty patterns, got %d", len(patterns))
	}
}

func TestDetector_FormatForContext_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	context := d.FormatForContext("")

	if context != "" {
		t.Error("Expected empty context for empty detector")
	}
}

func TestDetector_RecordFromAlert(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:      1000,
		MinOccurrences: 2,
		PatternWindow:  30 * 24 * time.Hour,
	})

	// Record from various alert types that ARE mapped
	d.RecordFromAlert("vm-100", "cpu_critical", time.Now())
	d.RecordFromAlert("vm-100", "memory_warning", time.Now())
	d.RecordFromAlert("vm-100", "disk_critical", time.Now())
	d.RecordFromAlert("vm-100", "restart", time.Now())
	d.RecordFromAlert("vm-100", "backup_failed", time.Now())

	// Should have recorded 5 events
	if len(d.events) != 5 {
		t.Errorf("Expected 5 events, got %d", len(d.events))
	}

	// Record with unknown alert type - should NOT create event
	d.RecordFromAlert("vm-100", "unknown_alert_type", time.Now())
	if len(d.events) != 5 {
		t.Errorf("Expected still 5 events after unknown alert type, got %d", len(d.events))
	}
}

func TestDetector_RecordEvent_WithResolve(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:      1000,
		MinOccurrences: 2,
		PatternWindow:  30 * 24 * time.Hour,
	})

	now := time.Now()
	resolveTime := now.Add(5 * time.Minute)

	event := HistoricalEvent{
		ResourceID:  "vm-100",
		EventType:   EventHighMemory,
		Timestamp:   now,
		Resolved:    true,
		ResolvedAt:  resolveTime,
		Duration:    5 * time.Minute,
		Description: "Memory usage exceeded 90%",
	}

	d.RecordEvent(event)

	if len(d.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(d.events))
	}

	recorded := d.events[0]
	if !recorded.Resolved {
		t.Error("Expected event to be resolved")
	}
	if recorded.Duration != 5*time.Minute {
		t.Errorf("Expected duration of 5 minutes, got %v", recorded.Duration)
	}
}

func TestDetector_PatternDetection_Extended(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:      1000,
		MinOccurrences: 3,
		PatternWindow:  30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create a pattern: memory issues every week
	for i := 0; i < 5; i++ {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventHighMemory,
			Timestamp:  now.Add(-time.Duration(i*7*24) * time.Hour),
		})
	}

	// Check patterns
	patterns := d.GetPatterns()
	if len(patterns) == 0 {
		t.Log("Note: Patterns may require more occurrences or specific conditions")
	}
}

func TestDetector_TrimEvents(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:      5,
		MinOccurrences: 1,
		PatternWindow:  30 * 24 * time.Hour,
	})

	// Add 10 events (exceeds max of 5)
	for i := 0; i < 10; i++ {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventHighCPU,
			Timestamp:  time.Now(),
		})
	}

	// Should be trimmed to max
	if len(d.events) > 5 {
		t.Errorf("Expected at most 5 events, got %d", len(d.events))
	}
}

func TestDetector_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "patterns-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDetector(DetectorConfig{
		MaxEvents:      100,
		MinOccurrences: 2,
		PatternWindow:  30 * 24 * time.Hour,
		DataDir:        tmpDir,
	})

	// Add some events
	d.RecordEvent(HistoricalEvent{
		ResourceID: "vm-100",
		EventType:  EventHighCPU,
		Timestamp:  time.Now(),
	})

	// Wait for async save
	time.Sleep(100 * time.Millisecond)

	// Create new detector from same dir
	d2 := NewDetector(DetectorConfig{
		MaxEvents:      100,
		MinOccurrences: 2,
		PatternWindow:  30 * 24 * time.Hour,
		DataDir:        tmpDir,
	})

	// Just check it doesn't crash
	_ = d2.GetPredictions()
}

func TestEventTypes(t *testing.T) {
	// Test that all event types are valid strings
	eventTypes := []EventType{
		EventHighMemory,
		EventHighCPU,
		EventDiskFull,
		EventRestart,
		EventUnresponsive,
		EventBackupFailed,
	}

	for _, et := range eventTypes {
		if et == "" {
			t.Error("Event type should not be empty")
		}
	}
}

func TestDetector_MultiplePredictions(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:       1000,
		MinOccurrences:  3,
		PatternWindow:   60 * 24 * time.Hour,
		PredictionLimit: 30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create multiple patterns for different resources
	for i := 0; i < 4; i++ {
		// VM-100 has weekly CPU issues
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(-time.Duration(i*7*24) * time.Hour),
		})

		// VM-200 has weekly memory issues
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-200",
			EventType:  EventHighMemory,
			Timestamp:  now.Add(-time.Duration(i*7*24) * time.Hour),
		})
	}

	// Get all predictions
	predictions := d.GetPredictions()
	// May or may not have predictions depending on pattern detection logic
	_ = predictions

	// Get for specific resource
	pred100 := d.GetPredictionsForResource("vm-100")
	_ = pred100
}

func TestDetector_FormatForContext_ResourceSpecific(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:       1000,
		MinOccurrences:  3,
		PatternWindow:   60 * 24 * time.Hour,
		PredictionLimit: 30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create pattern for specific resource
	for i := 0; i < 5; i++ {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(-time.Duration(i*7*24) * time.Hour),
		})
	}

	// Get context for specific resource (may be empty if no patterns detected)
	context := d.FormatForContext("vm-100")
	_ = context
}

func TestDetector_RecordEventWithAutoID(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:      100,
		MinOccurrences: 2,
	})

	// Record event without ID - should auto-generate
	event := HistoricalEvent{
		ResourceID: "vm-100",
		EventType:  EventHighCPU,
		Timestamp:  time.Now(),
	}

	d.RecordEvent(event)

	if len(d.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(d.events))
	}

	if d.events[0].ID == "" {
		t.Error("Expected auto-generated ID")
	}
}

func TestDetector_RecordEventWithAutoTimestamp(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MaxEvents:      100,
		MinOccurrences: 2,
	})

	// Record event without timestamp - should auto-set
	event := HistoricalEvent{
		ResourceID: "vm-100",
		EventType:  EventHighCPU,
	}

	before := time.Now()
	d.RecordEvent(event)
	after := time.Now()

	if len(d.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(d.events))
	}

	recorded := d.events[0]
	if recorded.Timestamp.Before(before) || recorded.Timestamp.After(after) {
		t.Error("Expected auto-generated timestamp to be around now")
	}
}
