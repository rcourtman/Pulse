package learning

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultLearningStoreConfig(t *testing.T) {
	cfg := DefaultLearningStoreConfig()
	if cfg.MaxRecords != 10000 {
		t.Fatalf("expected MaxRecords 10000, got %d", cfg.MaxRecords)
	}
	if cfg.RetentionDays != 90 {
		t.Fatalf("expected RetentionDays 90, got %d", cfg.RetentionDays)
	}
}

func TestRecordFeedback_GeneratesIDTimestampAndSignal(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f-1",
		ResourceID: "vm-1",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionThumbsDown,
	})

	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.feedbackRecords) != 1 {
		t.Fatalf("expected 1 feedback record, got %d", len(store.feedbackRecords))
	}

	for _, record := range store.feedbackRecords {
		if record.ID == "" {
			t.Fatalf("expected record ID to be generated")
		}
		if record.Timestamp.IsZero() {
			t.Fatalf("expected timestamp to be set")
		}
		if !record.Signal.IsFalsePositive {
			t.Fatalf("expected thumbs down to mark false positive")
		}
	}
}

func TestComputeFeedbackSignal_Default(t *testing.T) {
	signal := computeFeedbackSignal(UserAction("unknown"))
	if signal.Confidence != 0.5 {
		t.Fatalf("expected default confidence 0.5, got %.2f", signal.Confidence)
	}
}

func TestResourcePreferences_NotesTrim(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	for i := 0; i < 12; i++ {
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "f" + intToStr(i),
			ResourceID: "vm-1",
			Category:   "performance",
			Severity:   "warning",
			Action:     ActionDismissExpected,
			UserNote:   "note-" + intToStr(i),
		})
	}

	pref := store.GetResourcePreference("vm-1")
	if pref == nil {
		t.Fatalf("expected resource preference to exist")
	}
	if len(pref.Notes) != 10 {
		t.Fatalf("expected 10 notes after trimming, got %d", len(pref.Notes))
	}
	if pref.Notes[0] != "note-2" {
		t.Fatalf("expected oldest notes to be trimmed, got %s", pref.Notes[0])
	}
	if pref.Notes[len(pref.Notes)-1] != "note-11" {
		t.Fatalf("expected last note to be retained")
	}
}

func TestShouldSuppress_SeverityThreshold(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f1",
		ResourceID: "vm-1",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionDismissNotAnIssue,
	})

	if !store.ShouldSuppress("vm-1", "performance", "info") {
		t.Fatalf("expected info severity to be suppressed for thresholded category")
	}
	if store.ShouldSuppress("vm-1", "performance", "critical") {
		t.Fatalf("expected critical severity not to be suppressed")
	}
}

func TestCategoryPreferences_RollingAverage(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	store.RecordFeedback(FeedbackRecord{
		FindingID:    "f1",
		ResourceID:   "vm-1",
		Category:     "capacity",
		Severity:     "warning",
		Action:       ActionQuickFix,
		TimeToAction: 10 * time.Minute,
	})
	store.RecordFeedback(FeedbackRecord{
		FindingID:    "f2",
		ResourceID:   "vm-2",
		Category:     "capacity",
		Severity:     "warning",
		Action:       ActionQuickFix,
		TimeToAction: 20 * time.Minute,
	})

	pref := store.GetCategoryPreference("capacity")
	if pref == nil {
		t.Fatalf("expected category preference to exist")
	}
	expected := 15 * time.Minute
	if pref.AverageTimeToAction != expected {
		t.Fatalf("expected rolling average %s, got %s", expected, pref.AverageTimeToAction)
	}
}

func TestFormatForContext_Details(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})

	for i := 0; i < 6; i++ {
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "f" + intToStr(i),
			ResourceID: "vm-1",
			Category:   "performance",
			Severity:   "warning",
			Action:     ActionDismissNotAnIssue,
			UserNote:   "note-" + intToStr(i),
		})
	}
	for i := 0; i < 12; i++ {
		action := ActionQuickFix
		if i%5 == 0 {
			action = ActionDismissNotAnIssue
		}
		store.RecordFeedback(FeedbackRecord{
			FindingID:  "c" + intToStr(i),
			ResourceID: "vm-" + intToStr(i),
			Category:   "capacity",
			Severity:   "warning",
			Action:     action,
		})
	}

	context := store.FormatForContext()
	if context == "" {
		t.Fatalf("expected context to be populated")
	}
	if !containsStr(context, "vm-1") {
		t.Fatalf("expected resource preference details in context")
	}
	if !containsStr(context, "Category value") {
		t.Fatalf("expected category section in context")
	}
}

func TestCleanup_TrimMaxRecords(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{MaxRecords: 2})

	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f1",
		ResourceID: "vm-1",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionQuickFix,
	})
	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f2",
		ResourceID: "vm-2",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionQuickFix,
	})
	store.RecordFeedback(FeedbackRecord{
		FindingID:  "f3",
		ResourceID: "vm-3",
		Category:   "performance",
		Severity:   "warning",
		Action:     ActionQuickFix,
	})

	removed := store.Cleanup()
	if removed == 0 {
		t.Fatalf("expected Cleanup to trim records when over max")
	}
	stats := store.GetStatistics()
	if stats.TotalFeedbackRecords > 2 {
		t.Fatalf("expected records trimmed to max, got %d", stats.TotalFeedbackRecords)
	}
}

func TestSaveAndLoadLearningStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_learning.json")

	payload := struct {
		FeedbackRecords     map[string]*FeedbackRecord     `json:"feedback_records"`
		ResourcePreferences map[string]*ResourcePreference `json:"resource_preferences"`
		CategoryPreferences map[string]*CategoryPreference `json:"category_preferences"`
	}{
		FeedbackRecords: map[string]*FeedbackRecord{
			"fb-1": {
				ID:        "fb-1",
				FindingID: "finding-1",
				Category:  "performance",
				Action:    ActionQuickFix,
				Timestamp: time.Now(),
			},
		},
		ResourcePreferences: map[string]*ResourcePreference{
			"vm-1": {
				ResourceID:     "vm-1",
				TotalFindings:  3,
				ActionedCount:  2,
				DismissedCount: 1,
			},
		},
		CategoryPreferences: map[string]*CategoryPreference{
			"performance": {
				Category:      "performance",
				TotalFindings: 5,
				ActionedCount: 3,
			},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	loaded := NewLearningStore(LearningStoreConfig{DataDir: dir})
	stats := loaded.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("expected 1 feedback record, got %d", stats.TotalFeedbackRecords)
	}
	if stats.ResourcePreferences != 1 || stats.CategoryPreferences != 1 {
		t.Fatalf("expected resource and category prefs to load")
	}
}

func TestForceSave_PersistsData(t *testing.T) {
	dir := t.TempDir()
	store := NewLearningStore(LearningStoreConfig{DataDir: dir})

	store.mu.Lock()
	store.feedbackRecords["fb-1"] = &FeedbackRecord{
		ID:        "fb-1",
		FindingID: "finding-1",
		Category:  "capacity",
		Action:    ActionQuickFix,
		Timestamp: time.Now(),
	}
	store.mu.Unlock()

	if err := store.ForceSave(); err != nil {
		t.Fatalf("force save failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ai_learning.json"))
	if err != nil {
		t.Fatalf("expected saved file to exist: %v", err)
	}
	if !containsStr(string(data), "fb-1") {
		t.Fatalf("expected saved data to contain record id")
	}
}

func TestSaveIfDirty_WritesFile(t *testing.T) {
	dir := t.TempDir()
	store := NewLearningStore(LearningStoreConfig{DataDir: dir})

	store.mu.Lock()
	store.feedbackRecords["fb-1"] = &FeedbackRecord{
		ID:        "fb-1",
		FindingID: "finding-1",
		Category:  "capacity",
		Action:    ActionQuickFix,
		Timestamp: time.Now(),
	}
	store.dirty = true
	store.mu.Unlock()

	store.saveIfDirty()

	if _, err := os.Stat(filepath.Join(dir, "ai_learning.json")); err != nil {
		t.Fatalf("expected learning file to exist: %v", err)
	}
}

func TestSaveToDisk_NoDir(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})
	if err := store.saveToDisk(); err != nil {
		t.Fatalf("expected saveToDisk to no-op without DataDir, got %v", err)
	}
}

func TestSaveIfDirty_SaveError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	store := NewLearningStore(LearningStoreConfig{DataDir: filePath})
	store.mu.Lock()
	store.feedbackRecords["fb-1"] = &FeedbackRecord{
		ID:        "fb-1",
		FindingID: "finding-1",
		Category:  "capacity",
		Action:    ActionQuickFix,
		Timestamp: time.Now(),
	}
	store.dirty = true
	store.mu.Unlock()

	store.saveIfDirty()

	store.mu.RLock()
	dirty := store.dirty
	store.mu.RUnlock()
	if !dirty {
		t.Fatalf("expected dirty to remain true on save error")
	}
}

func TestComputeFeedbackSignal_AdditionalActions(t *testing.T) {
	signal := computeFeedbackSignal(ActionDismissWillFixLater)
	if !signal.WasActionable || signal.Confidence <= 0 {
		t.Fatalf("expected actionable signal for dismiss will fix later")
	}

	signal = computeFeedbackSignal(ActionAcknowledge)
	if !signal.WasActionable || signal.Confidence <= 0 {
		t.Fatalf("expected actionable signal for acknowledge")
	}
}

func TestGetPreferences_NotFound(t *testing.T) {
	store := NewLearningStore(LearningStoreConfig{})
	if store.GetResourcePreference("missing") != nil {
		t.Fatalf("expected nil for missing resource preference")
	}
	if store.GetCategoryPreference("missing") != nil {
		t.Fatalf("expected nil for missing category preference")
	}
}
