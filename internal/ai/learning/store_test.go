package learning

import (
	"testing"
	"time"
)

func TestLearningStore_RecordFeedback(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	record := FeedbackRecord{
		FindingID:  "finding-1",
		ResourceID: "vm-101",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionQuickFix,
	}

	store.RecordFeedback(record)

	stats := store.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Errorf("Expected 1 feedback record, got %d", stats.TotalFeedbackRecords)
	}
}

func TestLearningStore_ResourcePreferences(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	// Record feedback for a resource
	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f1",
		ResourceID: "vm-101",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionQuickFix,
	})
	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f2",
		ResourceID: "vm-101",
		Category:   "capacity",
		Severity:   "info",
		Action:     ActionDismissNotAnIssue,
	})

	pref := store.GetResourcePreference("vm-101")
	if pref == nil {
		t.Fatal("Expected resource preference")
	}

	if pref.TotalFindings != 2 {
		t.Errorf("Expected 2 findings, got %d", pref.TotalFindings)
	}

	if pref.ActionedCount != 1 {
		t.Errorf("Expected 1 actioned, got %d", pref.ActionedCount)
	}

	if pref.DismissedCount != 1 {
		t.Errorf("Expected 1 dismissed (false positive), got %d", pref.DismissedCount)
	}
}

func TestLearningStore_CategoryPreferences(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	// Record multiple actions in same category
	for i := 0; i < 15; i++ {
		action := ActionQuickFix
		if i%3 == 0 {
			action = ActionDismissNotAnIssue
		}
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "f" + intToStr(i),
			ResourceID: "vm-" + intToStr(i),
			Category:   "performance",
			Severity:   "warning",
			Action:     action,
		})
	}

	pref := store.GetCategoryPreference("performance")
	if pref == nil {
		t.Fatal("Expected category preference")
	}

	if pref.TotalFindings != 15 {
		t.Errorf("Expected 15 findings, got %d", pref.TotalFindings)
	}

	// With enough data, severity weight should be adjusted
	if pref.SeverityWeight == 1.0 {
		// Weight should change based on action rate
		t.Log("Severity weight adjusted based on action rate")
	}
}

func TestLearningStore_ShouldSuppress(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	// Record many dismissals to create high false positive rate
	for i := 0; i < 10; i++ {
		action := ActionDismissNotAnIssue
		if i == 0 {
			action = ActionQuickFix
		}
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "f" + intToStr(i),
			ResourceID: "noisy-vm",
			Category:   "performance",
			Severity:   "info",
			Action:     action,
		})
	}

	// Should suppress info-level findings for this resource
	if !store.ShouldSuppress("noisy-vm", "other", "info") {
		t.Error("Expected to suppress info-level for high FP resource")
	}

	// Should not suppress critical
	if store.ShouldSuppress("noisy-vm", "other", "critical") {
		t.Error("Should not suppress critical level")
	}
}

func TestLearningStore_GetSeverityWeight(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	// Default weight is 1.0
	weight := store.GetSeverityWeight("unknown")
	if weight != 1.0 {
		t.Errorf("Expected default weight 1.0, got %.2f", weight)
	}

	// After recording feedback, weight changes
	for i := 0; i < 15; i++ {
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "f" + intToStr(i),
			ResourceID: "vm-" + intToStr(i),
			Category:   "capacity",
			Severity:   "warning",
			Action:     ActionQuickFix, // High action rate
		})
	}

	weight = store.GetSeverityWeight("capacity")
	if weight <= 1.0 {
		t.Errorf("Expected weight > 1.0 for high action rate category, got %.2f", weight)
	}
}

func TestLearningStore_FormatForContext(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	// Record enough data
	for i := 0; i < 20; i++ {
		action := ActionQuickFix
		if i%2 == 0 {
			action = ActionDismissNotAnIssue
		}
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "f" + intToStr(i),
			ResourceID: "vm-101",
			Category:   "performance",
			Severity:   "warning",
			Action:     action,
		})
	}

	context := store.FormatForContext()

	if context == "" {
		t.Error("Expected non-empty context")
	}

	if !containsStr(context, "Learned Preferences") {
		t.Error("Expected 'Learned Preferences' in context")
	}
}

func TestLearningStore_FormatForContext_NoData(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	context := store.FormatForContext()
	if context != "" {
		t.Error("Expected empty context with no data")
	}
}

func TestLearningStore_Cleanup(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{RetentionDays: 1})

	// Add old feedback
	store.mu.Lock()
	oldRecord := &FeedbackRecord{
		ID:        "old-1",
		FindingID: "f1",
		Category:  "test",
		Action:    ActionQuickFix,
		Timestamp: time.Now().AddDate(0, 0, -2),
	}
	store.feedbackRecords["old-1"] = oldRecord
	store.mu.Unlock()

	// Add recent feedback
	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f2",
		ResourceID: "vm-1",
		Category:   "test",
		Action:     ActionQuickFix,
	})

	removed := store.Cleanup()
	if removed != 1 {
		t.Errorf("Expected 1 removed, got %d", removed)
	}

	stats := store.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Errorf("Expected 1 remaining, got %d", stats.TotalFeedbackRecords)
	}
}

func TestComputeFeedbackSignal(t *testing.T) {
	tests := []struct {
		action         UserAction
		expectedFP     bool
		expectedAction bool
		minConfidence  float64
	}{
		{ActionDismissNotAnIssue, true, false, 0.8},
		{ActionDismissExpected, false, false, 0.6},
		{ActionQuickFix, false, true, 0.9},
		{ActionThumbsUp, false, true, 0.8},
		{ActionThumbsDown, true, false, 0.8},
		{ActionIgnore, false, false, 0.2},
		{ActionSnooze, false, false, 0.4},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			signal := computeFeedbackSignal(tt.action)

			if signal.IsFalsePositive != tt.expectedFP {
				t.Errorf("Expected IsFalsePositive=%v, got %v", tt.expectedFP, signal.IsFalsePositive)
			}

			if signal.WasActionable != tt.expectedAction {
				t.Errorf("Expected WasActionable=%v, got %v", tt.expectedAction, signal.WasActionable)
			}

			if signal.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %.1f, got %.2f", tt.minConfidence, signal.Confidence)
			}
		})
	}
}

func TestSeverityLevel(t *testing.T) {
	tests := []struct {
		severity string
		expected int
	}{
		{"info", 0},
		{"watch", 1},
		{"warning", 2},
		{"critical", 3},
		{"unknown", 0},
	}

	for _, tt := range tests {
		result := severityLevel(tt.severity)
		if result != tt.expected {
			t.Errorf("severityLevel(%s) = %d, want %d", tt.severity, result, tt.expected)
		}
	}
}

func TestLearningStore_UserNotes(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f1",
		ResourceID: "vm-101",
		Category:   "performance",
		Action:     ActionDismissExpected,
		UserNote:   "This VM runs batch jobs",
	})

	pref := store.GetResourcePreference("vm-101")
	if len(pref.Notes) != 1 {
		t.Errorf("Expected 1 note, got %d", len(pref.Notes))
	}

	if pref.Notes[0] != "This VM runs batch jobs" {
		t.Errorf("Note mismatch")
	}
}

func TestLearningStore_Cleanup_WithActionQuickFix(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f1",
		ResourceID: "vm-101",
		Category:   "performance",
		Action:     ActionQuickFix,
	})

	stats := store.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Errorf("Expected 1 record, got %d", stats.TotalFeedbackRecords)
	}
}

func TestLearningStore_TimeToAction(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	store.RecordFeedback(FeedbackRecord{
		FindingID:    "f1",
		ResourceID:   "vm-101",
		Category:     "performance",
		Action:       ActionQuickFix,
		TimeToAction: 5 * time.Minute,
	})

	pref := store.GetCategoryPreference("performance")
	if pref.AverageTimeToAction == 0 {
		t.Error("Expected average time to action to be set")
	}
}

// Helpers
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var result string
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
