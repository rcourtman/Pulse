package patterns

import (
	"testing"
	"time"
)

func TestDetector_RecordEvent(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 2})

	// Record first event
	d.RecordEvent(HistoricalEvent{
		ResourceID: "vm-100",
		EventType:  EventHighMemory,
		Timestamp:  time.Now().Add(-10 * 24 * time.Hour),
	})

	if len(d.events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(d.events))
	}
}

func TestDetector_PatternDetection(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 3, PatternWindow: 365 * 24 * time.Hour})

	// Record events with 10-day interval
	now := time.Now()
	for i := 5; i >= 0; i-- {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventHighMemory,
			Timestamp:  now.Add(-time.Duration(i*10) * 24 * time.Hour),
		})
	}

	// Check that pattern was detected
	patterns := d.GetPatterns()
	key := patternKey("vm-100", EventHighMemory)
	pattern, ok := patterns[key]
	if !ok {
		t.Fatal("Expected pattern to be detected")
	}

	if pattern.Occurrences != 6 {
		t.Errorf("Expected 6 occurrences, got %d", pattern.Occurrences)
	}

	// Average interval should be ~10 days
	avgDays := pattern.AverageInterval.Hours() / 24
	if avgDays < 9 || avgDays > 11 {
		t.Errorf("Expected ~10 day interval, got %.1f days", avgDays)
	}
}

func TestDetector_GetPredictions(t *testing.T) {
	d := NewDetector(DetectorConfig{
		MinOccurrences:  3,
		PatternWindow:   365 * 24 * time.Hour,
		PredictionLimit: 30 * 24 * time.Hour,
	})

	// Record events with regular interval
	now := time.Now()
	for i := 3; i >= 0; i-- {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventOOM,
			Timestamp:  now.Add(-time.Duration(i*7) * 24 * time.Hour), // 7-day interval
		})
	}

	predictions := d.GetPredictions()
	
	// Should have a prediction for OOM
	found := false
	for _, p := range predictions {
		if p.ResourceID == "vm-100" && p.EventType == EventOOM {
			found = true
			// Should predict in ~7 days
			if p.DaysUntil < 5 || p.DaysUntil > 9 {
				t.Errorf("Expected prediction in ~7 days, got %.1f days", p.DaysUntil)
			}
			break
		}
	}
	
	if !found {
		t.Error("Expected OOM prediction for vm-100")
	}
}

func TestDetector_GetPredictionsForResource(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 3, PatternWindow: 365 * 24 * time.Hour})

	now := time.Now()
	// Add pattern for vm-100
	for i := 3; i >= 0; i-- {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventRestart,
			Timestamp:  now.Add(-time.Duration(i*14) * 24 * time.Hour),
		})
	}
	// Add pattern for vm-200
	for i := 3; i >= 0; i-- {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-200",
			EventType:  EventHighCPU,
			Timestamp:  now.Add(-time.Duration(i*5) * 24 * time.Hour),
		})
	}

	// Get predictions for vm-100 only
	predictions := d.GetPredictionsForResource("vm-100")
	for _, p := range predictions {
		if p.ResourceID != "vm-100" {
			t.Errorf("Got prediction for wrong resource: %s", p.ResourceID)
		}
	}
}

func TestDetector_Confidence(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 3, PatternWindow: 365 * 24 * time.Hour})

	now := time.Now()
	// Add very consistent pattern (every 7 days exactly)
	for i := 5; i >= 0; i-- {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "consistent-vm",
			EventType:  EventHighMemory,
			Timestamp:  now.Add(-time.Duration(i*7*24) * time.Hour),
		})
	}

	patterns := d.GetPatterns()
	pattern := patterns[patternKey("consistent-vm", EventHighMemory)]
	if pattern == nil {
		t.Fatal("Expected pattern")
	}

	// Consistent pattern should have high confidence
	if pattern.Confidence < 0.5 {
		t.Errorf("Expected high confidence for consistent pattern, got %.2f", pattern.Confidence)
	}
}

func TestDetector_FormatForContext(t *testing.T) {
	d := NewDetector(DetectorConfig{MinOccurrences: 3, PatternWindow: 365 * 24 * time.Hour})

	now := time.Now()
	for i := 3; i >= 0; i-- {
		d.RecordEvent(HistoricalEvent{
			ResourceID: "vm-100",
			EventType:  EventOOM,
			Timestamp:  now.Add(-time.Duration(i*10) * 24 * time.Hour),
		})
	}

	context := d.FormatForContext("vm-100")
	if context == "" {
		t.Error("Expected non-empty context")
	}
	
	if !contains(context, "OOM") && !contains(context, "oom") {
		t.Errorf("Expected context to mention OOM: %s", context)
	}
}

func TestMapAlertToEventType(t *testing.T) {
	tests := []struct {
		alertType string
		expected  EventType
	}{
		{"memory_warning", EventHighMemory},
		{"memory_critical", EventHighMemory},
		{"cpu_warning", EventHighCPU},
		{"cpu_critical", EventHighCPU},
		{"disk_warning", EventDiskFull},
		{"disk_critical", EventDiskFull},
		{"oom", EventOOM},
		{"restart", EventRestart},
		{"unresponsive", EventUnresponsive},
		{"backup_failed", EventBackupFailed},
		{"unknown_alert", ""},
	}

	for _, tc := range tests {
		result := mapAlertToEventType(tc.alertType)
		if result != tc.expected {
			t.Errorf("mapAlertToEventType(%q) = %q, want %q", tc.alertType, result, tc.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
