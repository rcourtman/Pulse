package ai

import (
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

func TestFindingsPersistenceAdapter_PreservesAllFields(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	now := time.Now()
	resolved := now.Add(time.Hour)
	acked := now.Add(30 * time.Minute)
	snoozed := now.Add(24 * time.Hour)

	originalFinding := &Finding{
		ID:             "test-finding",
		Key:            "test-key",
		Severity:       FindingSeverityCritical,
		Category:       FindingCategorySecurity,
		ResourceID:     "resource-123",
		ResourceName:   "Test Resource",
		ResourceType:   "vm",
		Node:           "node1",
		Title:          "Test Title",
		Description:    "Test Description",
		Recommendation: "Test Recommendation",
		Evidence:       "Test Evidence",
		DetectedAt:     now,
		LastSeenAt:     now,
		ResolvedAt:     &resolved,
		AutoResolved:   true,
		AcknowledgedAt: &acked,
		SnoozedUntil:   &snoozed,
		AlertID:        "alert-456",
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
}
