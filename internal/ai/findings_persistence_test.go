package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestNewFindingsPersistenceAdapter(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)

	adapter := NewFindingsPersistenceAdapter(persistence)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.config != persistence {
		t.Fatal("expected config to match persistence")
	}
}

func TestFindingsPersistenceAdapter_SaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	now := time.Now()
	findings := map[string]*Finding{
		"finding-1": {
			ID:             "finding-1",
			Key:            "high-cpu",
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryPerformance,
			ResourceID:     "node1-100",
			ResourceName:   "test-vm",
			ResourceType:   "vm",
			Node:           "node1",
			Title:          "High CPU Usage",
			Description:    "CPU is at 95%",
			Recommendation: "Check running processes",
			Evidence:       "CPU: 95%",
			DetectedAt:     now,
			LastSeenAt:     now,
		},
		"finding-2": {
			ID:             "finding-2",
			Key:            "high-memory",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryCapacity,
			ResourceID:     "node1-101",
			ResourceName:   "test-container",
			ResourceType:   "container",
			Node:           "node1",
			Title:          "High Memory Usage",
			Description:    "Memory is at 90%",
			Recommendation: "Increase memory allocation",
			Evidence:       "Memory: 90%",
			DetectedAt:     now.Add(-time.Hour),
			LastSeenAt:     now,
			AlertID:        "alert-123",
		},
	}

	// Save findings
	err := adapter.SaveFindings(findings)
	if err != nil {
		t.Fatalf("failed to save findings: %v", err)
	}

	// Load findings back
	loaded, err := adapter.LoadFindings()
	if err != nil {
		t.Fatalf("failed to load findings: %v", err)
	}

	if len(loaded) != len(findings) {
		t.Fatalf("expected %d findings, got %d", len(findings), len(loaded))
	}

	// Verify first finding
	f1 := loaded["finding-1"]
	if f1 == nil {
		t.Fatal("finding-1 not found")
	}
	if f1.Key != "high-cpu" {
		t.Errorf("expected key 'high-cpu', got %q", f1.Key)
	}
	if f1.Severity != FindingSeverityWarning {
		t.Errorf("expected severity 'warning', got %q", f1.Severity)
	}
	if f1.Category != FindingCategoryPerformance {
		t.Errorf("expected category 'performance', got %q", f1.Category)
	}
	if f1.ResourceID != "node1-100" {
		t.Errorf("expected resource ID 'node1-100', got %q", f1.ResourceID)
	}

	// Verify second finding
	f2 := loaded["finding-2"]
	if f2 == nil {
		t.Fatal("finding-2 not found")
	}
	if f2.AlertID != "alert-123" {
		t.Errorf("expected alert ID 'alert-123', got %q", f2.AlertID)
	}
}

func TestFindingsPersistenceAdapter_LoadEmpty(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	// Load from empty persistence should return empty map, not error
	loaded, err := adapter.LoadFindings()
	if err != nil {
		t.Fatalf("expected no error for empty load, got: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty map, got %d findings", len(loaded))
	}
}

func TestFindingsPersistenceAdapter_SaveEmpty(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	// Save empty map should work
	err := adapter.SaveFindings(map[string]*Finding{})
	if err != nil {
		t.Fatalf("failed to save empty findings: %v", err)
	}
}

func TestFindingsPersistenceAdapter_LoadError(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	badPath := filepath.Join(tmp, "ai_findings.json")
	if err := os.Mkdir(badPath, 0700); err != nil {
		t.Fatalf("failed to create directory at %s: %v", badPath, err)
	}

	if _, err := adapter.LoadFindings(); err == nil {
		t.Fatal("expected error when findings path is a directory")
	}
}

func TestFindingsPersistenceAdapter_PreservesAllFields(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	now := time.Now()
	resolved := now.Add(time.Hour)
	acked := now.Add(30 * time.Minute)
	snoozed := now.Add(24 * time.Hour)

	originalFinding := &Finding{
		ID:              "test-finding",
		Key:             "test-key",
		Severity:        FindingSeverityCritical,
		Category:        FindingCategorySecurity,
		ResourceID:      "resource-123",
		ResourceName:    "Test Resource",
		ResourceType:    "vm",
		Node:            "node1",
		Title:           "Test Title",
		Description:     "Test Description",
		Recommendation:  "Test Recommendation",
		Evidence:        "Test Evidence",
		Source:          "ai-analysis",
		DetectedAt:      now,
		LastSeenAt:      now,
		ResolvedAt:      &resolved,
		AutoResolved:    true,
		AcknowledgedAt:  &acked,
		SnoozedUntil:    &snoozed,
		AlertID:         "alert-456",
		DismissedReason: "expected_behavior",
		UserNote:        "This is intentional for Frigate recordings",
		TimesRaised:     5,
		Suppressed:      false,
	}

	findings := map[string]*Finding{"test-finding": originalFinding}

	// Save and load
	if err := adapter.SaveFindings(findings); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := adapter.LoadFindings()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	f := loaded["test-finding"]
	if f == nil {
		t.Fatal("finding not found after load")
	}

	// Verify all fields preserved
	if f.ID != originalFinding.ID {
		t.Errorf("ID mismatch: got %q", f.ID)
	}
	if f.Key != originalFinding.Key {
		t.Errorf("Key mismatch: got %q", f.Key)
	}
	if f.Severity != originalFinding.Severity {
		t.Errorf("Severity mismatch: got %q", f.Severity)
	}
	if f.Category != originalFinding.Category {
		t.Errorf("Category mismatch: got %q", f.Category)
	}
	if f.ResourceID != originalFinding.ResourceID {
		t.Errorf("ResourceID mismatch: got %q", f.ResourceID)
	}
	if f.AutoResolved != originalFinding.AutoResolved {
		t.Errorf("AutoResolved mismatch: got %v", f.AutoResolved)
	}
	if f.AlertID != originalFinding.AlertID {
		t.Errorf("AlertID mismatch: got %q", f.AlertID)
	}
	if f.ResolvedAt == nil {
		t.Error("ResolvedAt should not be nil")
	}
	if f.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt should not be nil")
	}
	if f.SnoozedUntil == nil {
		t.Error("SnoozedUntil should not be nil")
	}

	// Verify user feedback fields are preserved (these were missing before the fix)
	if f.Source != originalFinding.Source {
		t.Errorf("Source mismatch: got %q, want %q", f.Source, originalFinding.Source)
	}
	if f.DismissedReason != originalFinding.DismissedReason {
		t.Errorf("DismissedReason mismatch: got %q, want %q", f.DismissedReason, originalFinding.DismissedReason)
	}
	if f.UserNote != originalFinding.UserNote {
		t.Errorf("UserNote mismatch: got %q, want %q", f.UserNote, originalFinding.UserNote)
	}
	if f.TimesRaised != originalFinding.TimesRaised {
		t.Errorf("TimesRaised mismatch: got %d, want %d", f.TimesRaised, originalFinding.TimesRaised)
	}
	if f.Suppressed != originalFinding.Suppressed {
		t.Errorf("Suppressed mismatch: got %v, want %v", f.Suppressed, originalFinding.Suppressed)
	}
}

// TestFindingsPersistenceAdapter_PreservesDismissals specifically tests that dismissed findings
// are properly persisted across restarts. This was a bug where DismissedReason, UserNote,
// TimesRaised, and Suppressed fields were not being saved to disk.
func TestFindingsPersistenceAdapter_PreservesDismissals(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	now := time.Now()
	acked := now.Add(-1 * time.Hour)

	// Simulate a finding that was dismissed as "expected_behavior"
	dismissedFinding := &Finding{
		ID:              "frigate-storage-123",
		Key:             "storage-growth-high",
		Severity:        FindingSeverityWarning,
		Category:        FindingCategoryCapacity,
		ResourceID:      "delly-frigate-storage",
		ResourceName:    "frigate-storage",
		ResourceType:    "storage",
		Node:            "delly",
		Title:           "Frigate storage growing at 56GB/day",
		Description:     "Storage will be full in 13 days",
		Recommendation:  "Reduce retention policy",
		Evidence:        "6.0% per day growth rate",
		Source:          "ai-analysis",
		DetectedAt:      now.Add(-5 * time.Hour),
		LastSeenAt:      now,
		DismissedReason: "expected_behavior",
		UserNote:        "24/7 recording is intentional, storage is sized for this",
		TimesRaised:     3,
		Suppressed:      false,
		AcknowledgedAt:  &acked,
	}

	// Also test a suppressed finding
	suppressedFinding := &Finding{
		ID:              "dev-vm-hot",
		Key:             "cpu-high",
		Severity:        FindingSeverityInfo,
		Category:        FindingCategoryPerformance,
		ResourceID:      "node1-105",
		ResourceName:    "dev-vm",
		ResourceType:    "vm",
		Node:            "node1",
		Title:           "Dev VM running hot",
		Description:     "CPU at 80%",
		Source:          "ai-analysis",
		DetectedAt:      now.Add(-24 * time.Hour),
		LastSeenAt:      now,
		DismissedReason: "suppressed",
		UserNote:        "Development workload, expected",
		TimesRaised:     10,
		Suppressed:      true,
		AcknowledgedAt:  &acked,
	}

	findings := map[string]*Finding{
		dismissedFinding.ID:  dismissedFinding,
		suppressedFinding.ID: suppressedFinding,
	}

	// Save
	if err := adapter.SaveFindings(findings); err != nil {
		t.Fatalf("failed to save findings: %v", err)
	}

	// Load (simulating restart)
	loaded, err := adapter.LoadFindings()
	if err != nil {
		t.Fatalf("failed to load findings: %v", err)
	}

	// Verify dismissed finding
	df := loaded[dismissedFinding.ID]
	if df == nil {
		t.Fatal("dismissed finding not found after load")
	}
	if df.DismissedReason != "expected_behavior" {
		t.Errorf("DismissedReason not preserved: got %q, want 'expected_behavior'", df.DismissedReason)
	}
	if df.UserNote != dismissedFinding.UserNote {
		t.Errorf("UserNote not preserved: got %q", df.UserNote)
	}
	if df.TimesRaised != 3 {
		t.Errorf("TimesRaised not preserved: got %d, want 3", df.TimesRaised)
	}
	if df.IsActive() {
		t.Error("dismissed finding should not be active")
	}

	// Verify suppressed finding
	sf := loaded[suppressedFinding.ID]
	if sf == nil {
		t.Fatal("suppressed finding not found after load")
	}
	if !sf.Suppressed {
		t.Error("Suppressed not preserved: should be true")
	}
	if sf.DismissedReason != "suppressed" {
		t.Errorf("DismissedReason not preserved for suppressed: got %q", sf.DismissedReason)
	}
	if sf.TimesRaised != 10 {
		t.Errorf("TimesRaised not preserved: got %d, want 10", sf.TimesRaised)
	}
	if sf.IsActive() {
		t.Error("suppressed finding should not be active")
	}
}
