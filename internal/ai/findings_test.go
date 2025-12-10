package ai

import (
	"testing"
	"time"
)

func TestFinding_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		finding  Finding
		expected bool
	}{
		{
			name:     "active finding",
			finding:  Finding{ID: "test-1"},
			expected: true,
		},
		{
			name: "resolved finding",
			finding: Finding{
				ID:         "test-2",
				ResolvedAt: timePtr(time.Now()),
			},
			expected: false,
		},
		{
			name: "snoozed finding",
			finding: Finding{
				ID:           "test-3",
				SnoozedUntil: timePtr(time.Now().Add(1 * time.Hour)),
			},
			expected: false,
		},
		{
			name: "expired snooze finding",
			finding: Finding{
				ID:           "test-4",
				SnoozedUntil: timePtr(time.Now().Add(-1 * time.Hour)),
			},
			expected: true, // Snooze expired, should be active again
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.finding.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFinding_IsSnoozed(t *testing.T) {
	tests := []struct {
		name     string
		finding  Finding
		expected bool
	}{
		{
			name:     "not snoozed",
			finding:  Finding{ID: "test-1"},
			expected: false,
		},
		{
			name: "actively snoozed",
			finding: Finding{
				ID:           "test-2",
				SnoozedUntil: timePtr(time.Now().Add(1 * time.Hour)),
			},
			expected: true,
		},
		{
			name: "snooze expired",
			finding: Finding{
				ID:           "test-3",
				SnoozedUntil: timePtr(time.Now().Add(-1 * time.Hour)),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.finding.IsSnoozed(); got != tt.expected {
				t.Errorf("IsSnoozed() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFindingsStore_Snooze(t *testing.T) {
	store := NewFindingsStore()

	// Add a finding
	finding := &Finding{
		ID:           "finding-1",
		Severity:     FindingSeverityWarning,
		ResourceID:   "res-1",
		ResourceName: "test-resource",
		Title:        "Test Finding",
	}
	store.Add(finding)

	// Verify it's active
	activeFindings := store.GetActive(FindingSeverityInfo)
	if len(activeFindings) != 1 {
		t.Fatalf("Expected 1 active finding, got %d", len(activeFindings))
	}

	// Snooze the finding for 1 hour
	if !store.Snooze("finding-1", 1*time.Hour) {
		t.Fatal("Snooze should return true for existing finding")
	}

	// Verify it's no longer in active list
	activeFindings = store.GetActive(FindingSeverityInfo)
	if len(activeFindings) != 0 {
		t.Fatalf("Expected 0 active findings after snooze, got %d", len(activeFindings))
	}

	// Verify summary counts decreased
	summary := store.GetSummary()
	if summary.Warning != 0 {
		t.Errorf("Expected 0 warning count, got %d", summary.Warning)
	}

	// Snooze non-existent finding
	if store.Snooze("non-existent", 1*time.Hour) {
		t.Error("Snooze should return false for non-existent finding")
	}
}

func TestFindingsStore_Unsnooze(t *testing.T) {
	store := NewFindingsStore()

	// Add and snooze a finding
	finding := &Finding{
		ID:           "finding-1",
		Severity:     FindingSeverityCritical,
		ResourceID:   "res-1",
		ResourceName: "test-resource",
		Title:        "Test Finding",
	}
	store.Add(finding)
	store.Snooze("finding-1", 1*time.Hour)

	// Verify it's snoozed
	activeFindings := store.GetActive(FindingSeverityInfo)
	if len(activeFindings) != 0 {
		t.Fatalf("Expected 0 active findings when snoozed, got %d", len(activeFindings))
	}

	// Unsnooze
	if !store.Unsnooze("finding-1") {
		t.Fatal("Unsnooze should return true for snoozed finding")
	}

	// Verify it's active again
	activeFindings = store.GetActive(FindingSeverityInfo)
	if len(activeFindings) != 1 {
		t.Fatalf("Expected 1 active finding after unsnooze, got %d", len(activeFindings))
	}

	// Verify summary counts increased
	summary := store.GetSummary()
	if summary.Critical != 1 {
		t.Errorf("Expected 1 critical count, got %d", summary.Critical)
	}

	// Unsnooze non-existent finding
	if store.Unsnooze("non-existent") {
		t.Error("Unsnooze should return false for non-existent finding")
	}
}

func TestFindingsStore_SnoozeResolvedFinding(t *testing.T) {
	store := NewFindingsStore()

	// Add a finding
	finding := &Finding{
		ID:           "finding-1",
		Severity:     FindingSeverityWarning,
		ResourceID:   "res-1",
		ResourceName: "test-resource",
		Title:        "Test Finding",
	}
	store.Add(finding)

	// Resolve it
	store.Resolve("finding-1", false)

	// Try to snooze a resolved finding - should fail
	if store.Snooze("finding-1", 1*time.Hour) {
		t.Error("Should not be able to snooze a resolved finding")
	}
}

func TestFindingsStore_SummaryConsistentWithActive(t *testing.T) {
	store := NewFindingsStore()

	// Add mixed severity findings
	store.Add(&Finding{ID: "f1", Severity: FindingSeverityCritical, ResourceID: "r1", ResourceName: "res1", Title: "Critical"})
	store.Add(&Finding{ID: "f2", Severity: FindingSeverityWarning, ResourceID: "r2", ResourceName: "res2", Title: "Warning"})
	store.Add(&Finding{ID: "f3", Severity: FindingSeverityWatch, ResourceID: "r3", ResourceName: "res3", Title: "Watch"})

	// Verify initial consistency
	active := store.GetActive(FindingSeverityInfo)
	summary := store.GetSummary()

	if len(active) != summary.Critical+summary.Warning+summary.Watch+summary.Info {
		t.Errorf("Mismatch: %d active findings, summary totals %d", len(active), summary.Critical+summary.Warning+summary.Watch+summary.Info)
	}

	// Resolve the warning finding
	store.Resolve("f2", false)

	// Verify consistency after resolution
	active = store.GetActive(FindingSeverityInfo)
	summary = store.GetSummary()

	if len(active) != 2 {
		t.Fatalf("Expected 2 active findings after resolution, got %d", len(active))
	}

	if summary.Warning != 0 {
		t.Errorf("Summary shows %d warnings but should be 0 after resolution", summary.Warning)
	}

	if summary.Critical != 1 {
		t.Errorf("Summary shows %d critical but should be 1", summary.Critical)
	}

	// Snooze the critical finding
	store.Snooze("f1", 1*time.Hour)

	// Verify consistency after snooze
	active = store.GetActive(FindingSeverityInfo)
	summary = store.GetSummary()

	if len(active) != 1 {
		t.Errorf("Expected 1 active finding after snooze, got %d", len(active))
	}

	if summary.Critical != 0 {
		t.Errorf("Summary shows %d critical but should be 0 after snooze", summary.Critical)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
