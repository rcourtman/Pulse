package correlation

import (
	"testing"
	"time"
)

func TestDetector_RecordEvent(t *testing.T) {
	d := NewDetector(DefaultConfig())

	d.RecordEvent(Event{
		ResourceID:   "vm-100",
		ResourceName: "web-server",
		EventType:    EventHighMem,
		Timestamp:    time.Now(),
	})

	if len(d.events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(d.events))
	}
}

func TestDetector_CorrelationDetection(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    2, // Lower threshold for testing
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// Simulate pattern: when storage has high usage, database VM restarts
	// Occurrence 1
	d.RecordEvent(Event{
		ResourceID:   "storage-1",
		ResourceName: "local-zfs",
		ResourceType: "storage",
		EventType:    EventDiskFull,
		Timestamp:    now.Add(-2 * time.Hour),
	})
	d.RecordEvent(Event{
		ResourceID:   "vm-100",
		ResourceName: "database",
		ResourceType: "vm",
		EventType:    EventRestart,
		Timestamp:    now.Add(-2*time.Hour + 5*time.Minute),
	})

	// Occurrence 2
	d.RecordEvent(Event{
		ResourceID:   "storage-1",
		ResourceName: "local-zfs",
		ResourceType: "storage",
		EventType:    EventDiskFull,
		Timestamp:    now.Add(-1 * time.Hour),
	})
	d.RecordEvent(Event{
		ResourceID:   "vm-100",
		ResourceName: "database",
		ResourceType: "vm",
		EventType:    EventRestart,
		Timestamp:    now.Add(-1*time.Hour + 5*time.Minute),
	})

	// Occurrence 3
	d.RecordEvent(Event{
		ResourceID:   "storage-1",
		ResourceName: "local-zfs",
		ResourceType: "storage",
		EventType:    EventDiskFull,
		Timestamp:    now,
	})
	d.RecordEvent(Event{
		ResourceID:   "vm-100",
		ResourceName: "database",
		ResourceType: "vm",
		EventType:    EventRestart,
		Timestamp:    now.Add(5 * time.Minute),
	})

	// Check correlations
	correlations := d.GetCorrelations()
	
	found := false
	for _, c := range correlations {
		if c.SourceID == "storage-1" && c.TargetID == "vm-100" {
			found = true
			if c.Occurrences < 2 {
				t.Errorf("Expected at least 2 occurrences, got %d", c.Occurrences)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected correlation between storage-1 and vm-100")
	}
}

func TestDetector_GetCorrelationsForResource(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    2,
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// Create correlation pattern
	for i := 0; i < 3; i++ {
		offset := time.Duration(-i) * time.Hour
		d.RecordEvent(Event{
			ResourceID: "node-1",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(offset),
		})
		d.RecordEvent(Event{
			ResourceID: "vm-100",
			EventType:  EventHighMem,
			Timestamp:  now.Add(offset + 2*time.Minute),
		})
	}

	// Get correlations for node-1
	correlations := d.GetCorrelationsForResource("node-1")
	if len(correlations) == 0 {
		t.Error("Expected correlations for node-1")
	}
}

func TestDetector_GetDependencies(t *testing.T) {
	d := NewDetector(Config{
		MaxEvents:         1000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    3,
		RetentionWindow:   30 * 24 * time.Hour,
	})

	now := time.Now()

	// When storage is full, both VM and container have issues
	for i := 0; i < 4; i++ {
		offset := time.Duration(-i) * time.Hour
		d.RecordEvent(Event{
			ResourceID: "storage-1",
			EventType:  EventDiskFull,
			Timestamp:  now.Add(offset),
		})
		d.RecordEvent(Event{
			ResourceID: "vm-100",
			EventType:  EventRestart,
			Timestamp:  now.Add(offset + 3*time.Minute),
		})
		d.RecordEvent(Event{
			ResourceID: "ct-200",
			EventType:  EventRestart,
			Timestamp:  now.Add(offset + 5*time.Minute),
		})
	}

	deps := d.GetDependencies("storage-1")
	if len(deps) < 2 {
		t.Errorf("Expected at least 2 dependencies, got %d", len(deps))
	}
}

func TestDetector_PredictCascade(t *testing.T) {
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
	if len(predictions) == 0 {
		t.Error("Expected cascade predictions")
	}
}

func TestDetector_FormatForContext(t *testing.T) {
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

	context := d.FormatForContext("")
	if context == "" {
		t.Error("Expected non-empty context")
	}
	
	if !contains(context, "Correlation") {
		t.Errorf("Expected context to mention correlations: %s", context)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "seconds",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			expected: "1 minute",
		},
		{
			name:     "multiple minutes",
			duration: 30 * time.Minute,
			expected: "30 minutes",
		},
		{
			name:     "just under an hour",
			duration: 59 * time.Minute,
			expected: "59 minutes",
		},
		{
			name:     "one hour",
			duration: 1 * time.Hour,
			expected: "1 hour",
		},
		{
			name:     "multiple hours",
			duration: 5 * time.Hour,
			expected: "5 hours",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatConfidence(t *testing.T) {
	tests := []struct {
		confidence float64
		expected   string
	}{
		{0.0, "0%"},
		{0.5, "50%"},
		{0.75, "75%"},
		{1.0, "100%"},
		{0.333, "33%"},
	}

	for _, tt := range tests {
		result := formatConfidence(tt.confidence)
		if result != tt.expected {
			t.Errorf("formatConfidence(%.2f) = %q, want %q", tt.confidence, result, tt.expected)
		}
	}
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{999, "999"},
		{12345, "12345"},
	}

	for _, tt := range tests {
		result := intToStr(tt.input)
		if result != tt.expected {
			t.Errorf("intToStr(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

