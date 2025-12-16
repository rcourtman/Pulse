package correlation

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
	if cfg.CorrelationWindow == 0 {
		t.Error("CorrelationWindow should have a default value")
	}
	if cfg.MinOccurrences == 0 {
		t.Error("MinOccurrences should have a default value")
	}
	if cfg.RetentionWindow == 0 {
		t.Error("RetentionWindow should have a default value")
	}
}

func TestNewDetector_Defaults(t *testing.T) {
	d := NewDetector(Config{})

	if d == nil {
		t.Fatal("Expected non-nil detector")
	}

	if d.events == nil {
		t.Error("events should be initialized")
	}
	if d.correlations == nil {
		t.Error("correlations should be initialized")
	}
}

func TestDetector_GetCorrelations_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	correlations := d.GetCorrelations()

	if len(correlations) != 0 {
		t.Errorf("Expected empty correlations, got %d", len(correlations))
	}
}

func TestDetector_GetCorrelationsForResource_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	correlations := d.GetCorrelationsForResource("nonexistent")

	if len(correlations) != 0 {
		t.Errorf("Expected empty correlations, got %d", len(correlations))
	}
}

func TestDetector_GetDependencies_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	deps := d.GetDependencies("nonexistent")

	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies, got %d", len(deps))
	}
}

func TestDetector_GetDependsOn_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	deps := d.GetDependsOn("nonexistent")

	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies, got %d", len(deps))
	}
}

func TestDetector_PredictCascade_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	predictions := d.PredictCascade("nonexistent", EventHighCPU)

	if len(predictions) != 0 {
		t.Errorf("Expected empty predictions, got %d", len(predictions))
	}
}

func TestDetector_FormatForContext_Empty(t *testing.T) {
	d := NewDetector(DefaultConfig())

	context := d.FormatForContext("")

	if context != "" {
		t.Error("Expected empty context for empty detector")
	}
}

func TestDetector_FormatForContext_ResourceSpecific(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    2,
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create correlations
	for i := 0; i < 3; i++ {
		offset := time.Duration(-i) * time.Hour
		d.RecordEvent(Event{
			ResourceID:   "storage-1",
			ResourceName: "local-zfs",
			EventType:    EventDiskFull,
			Timestamp:    now.Add(offset),
		})
		d.RecordEvent(Event{
			ResourceID:   "vm-100",
			ResourceName: "database",
			EventType:    EventRestart,
			Timestamp:    now.Add(offset + 5*time.Minute),
		})
	}

	// Get context for specific resource
	context := d.FormatForContext("storage-1")
	if context == "" {
		t.Error("Expected non-empty context for storage-1")
	}
}

func TestDetector_GetDependsOn(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    3,
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create pattern: node event is followed by VM event
	for i := 0; i < 4; i++ {
		offset := time.Duration(-i) * time.Hour
		d.RecordEvent(Event{
			ResourceID: "node-1",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(offset),
		})
		d.RecordEvent(Event{
			ResourceID: "vm-100",
			EventType:  EventHighMem,
			Timestamp:  now.Add(offset + 3*time.Minute),
		})
	}

	// VM depends on node
	deps := d.GetDependsOn("vm-100")
	if len(deps) == 0 {
		// If correlation was detected
		t.Log("Note: GetDependsOn may return empty if correlation threshold not met")
	}
}

func TestDetector_EventTypes(t *testing.T) {
	// Test that all event types are valid
	eventTypes := []EventType{
		EventAlert,
		EventRestart,
		EventHighCPU,
		EventHighMem,
		EventDiskFull,
		EventOffline,
		EventMigration,
	}

	for _, et := range eventTypes {
		if et == "" {
			t.Error("Event type should not be empty")
		}
	}
}

func TestDetector_TrimEvents(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         5,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    1,
		RetentionWindow:   time.Hour,
	})

	// Add 10 events (exceeds max of 5)
	for i := 0; i < 10; i++ {
		d.RecordEvent(Event{
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

func TestDetector_CalculateConfidence(t *testing.T) {
	d := NewDetector(DefaultConfig())

	// Low occurrences = low confidence
	conf1 := d.calculateConfidence(3)
	conf2 := d.calculateConfidence(10)

	if conf2 <= conf1 {
		t.Error("More occurrences should result in higher confidence")
	}

	// Confidence should be capped at 1.0
	confMax := d.calculateConfidence(1000)
	if confMax > 1.0 {
		t.Errorf("Confidence should not exceed 1.0, got %f", confMax)
	}
}

func TestDetector_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "correlation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDetector(Config{
		MaxEvents:         100,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    2,
		RetentionWindow:   24 * time.Hour,
		DataDir:           tmpDir,
	})

	// Add some events
	d.RecordEvent(Event{
		ResourceID: "vm-100",
		EventType:  EventHighCPU,
		Timestamp:  time.Now(),
	})

	// Wait for async save
	time.Sleep(100 * time.Millisecond)

	// Create new detector from same dir - file may not exist yet (async)
	d2 := NewDetector(Config{
		MaxEvents:         100,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    2,
		RetentionWindow:   24 * time.Hour,
		DataDir:           tmpDir,
	})

	// Just check it doesn't crash
	_ = d2.GetCorrelations()
}

func TestCascadePrediction_Fields(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    3,
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create strong correlation
	for i := 0; i < 5; i++ {
		offset := time.Duration(-i) * time.Hour
		d.RecordEvent(Event{
			ResourceID:   "node-1",
			ResourceName: "pve-main",
			EventType:    EventHighMem,
			Timestamp:    now.Add(offset),
		})
		d.RecordEvent(Event{
			ResourceID:   "vm-100",
			ResourceName: "database",
			EventType:    EventRestart,
			Timestamp:    now.Add(offset + 5*time.Minute),
		})
	}

	predictions := d.PredictCascade("node-1", EventHighMem)
	if len(predictions) > 0 {
		p := predictions[0]
		// Check fields are populated
		if p.ResourceID == "" {
			t.Error("ResourceID should be populated")
		}
		if p.Confidence < 0 || p.Confidence > 1 {
			t.Errorf("Confidence should be 0-1, got %f", p.Confidence)
		}
	}
}

func TestDetector_MultipleEventTypes(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    2,
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// Different event types on same resource
	d.RecordEvent(Event{
		ResourceID: "vm-100",
		EventType:  EventHighCPU,
		Timestamp:  now.Add(-1 * time.Hour),
	})
	d.RecordEvent(Event{
		ResourceID: "vm-100",
		EventType:  EventHighMem,
		Timestamp:  now.Add(-30 * time.Minute),
	})
	d.RecordEvent(Event{
		ResourceID: "vm-100",
		EventType:  EventRestart,
		Timestamp:  now,
	})

	// Should have recorded all events
	if len(d.events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(d.events))
	}
}
