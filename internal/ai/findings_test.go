package ai

import (
	"strings"
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

func TestFindingsStore_Acknowledge(t *testing.T) {
	store := NewFindingsStore()
	finding := &Finding{ID: "f1", Severity: FindingSeverityWarning, ResourceID: "r1", ResourceName: "res1", Title: "Test"}
	store.Add(finding)

	if !store.Acknowledge("f1") {
		t.Fatal("Acknowledge should return true")
	}

	f := store.Get("f1")
	if f.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt should be set")
	}

	// Double acknowledge
	if !store.Acknowledge("f1") {
		t.Error("Acknowledge should still return true (noop)")
	}
}

func TestFindingsStore_Dismiss(t *testing.T) {
	store := NewFindingsStore()
	finding := &Finding{ID: "f1", Severity: FindingSeverityWarning, ResourceID: "r1", ResourceName: "res1", Title: "Test", Category: FindingCategoryPerformance}
	store.Add(finding)

	if !store.Dismiss("f1", "not_an_issue", "Custom note") {
		t.Fatal("Dismiss should return true")
	}

	f := store.Get("f1")
	if !f.IsDismissed() {
		t.Error("Finding should be dismissed")
	}
	if f.DismissedReason != "not_an_issue" {
		t.Errorf("Expected reason not_an_issue, got %s", f.DismissedReason)
	}
	if f.UserNote != "Custom note" {
		t.Errorf("Expected note 'Custom note', got %s", f.UserNote)
	}

	// Verify it's not active anymore
	active := store.GetActive(FindingSeverityInfo)
	if len(active) != 0 {
		t.Error("Dismissed finding should not be active")
	}
}

func TestFindingsStore_SetUserNote(t *testing.T) {
	store := NewFindingsStore()
	finding := &Finding{ID: "f1", Severity: FindingSeverityWarning, ResourceID: "r1", ResourceName: "res1", Title: "Test"}
	store.Add(finding)

	if !store.SetUserNote("f1", "New note") {
		t.Fatal("SetUserNote should return true")
	}

	f := store.Get("f1")
	if f.UserNote != "New note" {
		t.Errorf("Expected note 'New note', got %s", f.UserNote)
	}
}

func TestFindingsStore_Suppression(t *testing.T) {
	store := NewFindingsStore()
	
	// Add a finding
	finding := &Finding{
		ID:           "f1",
		Severity:     FindingSeverityWarning,
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		Title:        "High CPU",
		Category:     FindingCategoryPerformance,
	}
	store.Add(finding)

	// Suppress it
	if !store.Suppress("f1") {
		t.Fatal("Suppress should return true")
	}

	// Verify it's suppressed and dismissed
	f := store.Get("f1")
	if !f.IsDismissed() || f.DismissedReason != "suppressed" {
		t.Error("Finding should be auto-dismissed when suppressed")
	}

	if !store.IsSuppressed("res-1", FindingCategoryPerformance) {
		t.Error("Provider+Category should be reported as suppressed")
	}

	// Add a NEW finding for the SAME resource and category
	finding2 := &Finding{
		ID:           "f2",
		Severity:     FindingSeverityWarning,
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		Title:        "Another High CPU",
		Category:     FindingCategoryPerformance,
	}
	isNew := store.Add(finding2)
	
	if isNew {
		t.Error("Expected Add to return false for suppressed finding")
	}

	f2 := store.Get("f2")
	if f2 != nil {
		t.Error("Suppressed finding should not be stored")
	}

	// Test clearing suppression
	rules := store.GetSuppressionRules()
	if len(rules) == 0 {
		t.Fatal("Should have at least one suppression rule")
	}
	
	if !store.DeleteSuppressionRule(rules[0].ID) {
		t.Fatal("DeleteSuppressionRule should return true")
	}

	if store.IsSuppressed("res-1", FindingCategoryPerformance) {
		t.Error("Should no longer be suppressed after rule deletion")
	}
}

func TestFindingsStore_AddSuppressionRule(t *testing.T) {
	store := NewFindingsStore()
	
	rule := store.AddSuppressionRule("res-1", "Res 1", FindingCategoryCapacity, "Manual suppression")
	if rule == nil {
		t.Fatal("AddSuppressionRule returned nil")
	}

	if !store.IsSuppressed("res-1", FindingCategoryCapacity) {
		t.Error("Resource+Category should be suppressed")
	}

	// Verify rules list
	rules := store.GetSuppressionRules()
	found := false
	for _, r := range rules {
		if r.ID == rule.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Manual rule not found in GetSuppressionRules")
	}
}

func TestFindingsStore_Cleanup(t *testing.T) {
	store := NewFindingsStore()
	
	now := time.Now()
	
	// Add an active finding (should NOT be cleaned up)
	store.Add(&Finding{ID: "active", Title: "Active"})
	
	// Add an old resolved finding (should BE cleaned up)
	resolvedOld := &Finding{
		ID:         "resolved-old",
		Title:      "Resolved Old",
		ResolvedAt: timePtr(now.Add(-48 * time.Hour)),
	}
	store.Add(resolvedOld)
	
	// Add a recent resolved finding (should NOT be cleaned up if maxAge is 24h)
	resolvedRecent := &Finding{
		ID:         "resolved-recent",
		Title:      "Resolved Recent",
		ResolvedAt: timePtr(now.Add(-1 * time.Hour)),
	}
	store.Add(resolvedRecent)

	removed := store.Cleanup(24 * time.Hour)
	if removed != 1 {
		t.Errorf("Expected 1 finding removed, got %d", removed)
	}

	if store.Get("resolved-old") != nil {
		t.Error("resolved-old should have been removed")
	}
	if store.Get("active") == nil {
		t.Error("active should NOT have been removed")
	}
	if store.Get("resolved-recent") == nil {
		t.Error("resolved-recent should NOT have been removed")
	}
}

func TestFindingsStore_GetDismissedForContext(t *testing.T) {
	store := NewFindingsStore()
	
	// Add dismissed finding
	finding1 := &Finding{
		ID:              "f1",
		Title:           "High CPU on web-1",
		ResourceID:      "web-1",
		ResourceName:    "web-1",
		Category:        FindingCategoryPerformance,
		DismissedReason: "not_an_issue",
		UserNote:        "Expected during backup",
	}
	store.Add(finding1)
	store.Dismiss("f1", "not_an_issue", "Expected during backup")

	// Add suppressed finding
	finding2 := &Finding{
		ID:           "f2",
		Title:        "Disk nearly full on db-1",
		ResourceID:   "db-1",
		ResourceName: "db-1",
		Category:     FindingCategoryCapacity,
	}
	store.Add(finding2)
	store.Suppress("f2")

	ctx := store.GetDismissedForContext()
	
	if !strings.Contains(ctx, "High CPU on web-1") {
		t.Error("Context should contain dismissed finding title")
	}
	if !strings.Contains(ctx, "Disk nearly full on db-1") {
		t.Error("Context should contain suppressed finding title")
	}
	if !strings.Contains(ctx, "Expected during backup") {
		t.Error("Context should contain user note")
	}
	if !strings.Contains(ctx, "Permanently Suppressed") {
		t.Error("Context should label suppressed findings")
	}
}

func TestFindingsStore_Persistence(t *testing.T) {
	store := NewFindingsStore()
	mockP := &mockPersistence{}
	
	err := store.SetPersistence(mockP)
	if err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	// Add a finding - should trigger save (debounced, but we can ForceSave)
	store.Add(&Finding{ID: "f1", Title: "Persist me"})
	
	err = store.ForceSave()
	if err != nil {
		t.Fatalf("ForceSave failed: %v", err)
	}

	if mockP.savedCount != 1 {
		t.Errorf("Expected 1 save, got %d", mockP.savedCount)
	}
}

func TestFindingsStore_Add_UpdateExisting(t *testing.T) {
	store := NewFindingsStore()

	// Add initial finding
	f1 := &Finding{
		ID:          "f1",
		ResourceID:  "res-1",
		Severity:    FindingSeverityWarning,
		Title:       "Initial Title",
		Description: "Initial Description",
	}
	isNew := store.Add(f1)
	if !isNew {
		t.Error("Expected new finding on first add")
	}

	// Add same finding again (should update, not create new)
	f1Updated := &Finding{
		ID:          "f1",
		ResourceID:  "res-1",
		Severity:    FindingSeverityWarning,
		Title:       "Updated Title",
		Description: "Updated Description",
	}
	isNew = store.Add(f1Updated)
	if isNew {
		t.Error("Expected update, not new finding")
	}

	// Verify update happened
	stored := store.Get("f1")
	if stored.Title != "Updated Title" {
		t.Errorf("Expected 'Updated Title', got %s", stored.Title)
	}
	if stored.TimesRaised != 1 {
		t.Errorf("Expected TimesRaised=1, got %d", stored.TimesRaised)
	}
}

func TestFindingsStore_Add_SeverityEscalation(t *testing.T) {
	store := NewFindingsStore()

	// Add and dismiss a warning finding
	f1 := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Warning",
	}
	store.Add(f1)
	store.Dismiss("f1", "not_an_issue", "It's fine")

	// Verify it's dismissed
	dismissed := store.Get("f1")
	if !dismissed.IsDismissed() {
		t.Fatal("Finding should be dismissed")
	}

	// Now add the same finding with HIGHER severity (critical)
	f1Escalated := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityCritical,
		Title:      "Now Critical!",
	}
	store.Add(f1Escalated)

	// Severity escalation should clear the dismissal
	reactivated := store.Get("f1")
	if reactivated.IsDismissed() {
		t.Error("Finding should be reactivated after severity escalation")
	}
	if reactivated.Severity != FindingSeverityCritical {
		t.Error("Severity should be updated to critical")
	}
}

func TestFindingsStore_Add_SuppressedExisting(t *testing.T) {
	store := NewFindingsStore()

	// Add and suppress a finding
	f1 := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Suppressed Finding",
		Category:   FindingCategoryPerformance,
	}
	store.Add(f1)
	store.Suppress("f1")

	// Verify it's suppressed
	suppressed := store.Get("f1")
	if !suppressed.Suppressed {
		t.Fatal("Finding should be suppressed")
	}

	// Try to add the same finding again with SAME severity (should stay suppressed)
	f1Updated := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning, // Same severity - should stay suppressed
		Title:      "Updated Title",
		Category:   FindingCategoryPerformance,
	}
	isNew := store.Add(f1Updated)

	// Should not be treated as new
	if isNew {
		t.Error("Suppressed finding should not be treated as new")
	}

	// Verify the finding title wasn't updated (stayed suppressed)
	stillSuppressed := store.Get("f1")
	if stillSuppressed.Title != "Suppressed Finding" {
		t.Error("Suppressed finding title should not change with same severity")
	}

	// But TimesRaised should still increment
	if stillSuppressed.TimesRaised < 1 {
		t.Error("TimesRaised should increment even for suppressed findings")
	}

	// NOTE: Severity ESCALATION (e.g., warning -> critical) would legitimately
	// reactivate the finding for safety reasons - tested in TestFindingsStore_Add_SeverityEscalation
}

func TestFindingsStore_Add_DismissedSameSeverity(t *testing.T) {
	store := NewFindingsStore()

	// Add and dismiss a finding
	f1 := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Warning",
	}
	store.Add(f1)
	store.Dismiss("f1", "expected_behavior", "Known issue")

	// Add same finding with SAME severity
	f1Same := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Still Warning",
	}
	store.Add(f1Same)

	// Should stay dismissed (same severity doesn't reactivate)
	stillDismissed := store.Get("f1")
	if !stillDismissed.IsDismissed() {
		t.Error("Finding should stay dismissed with same severity")
	}
	// But TimesRaised should increment
	if stillDismissed.TimesRaised < 1 {
		t.Error("TimesRaised should increment even for dismissed findings")
	}
}

func TestFindingsStore_GetByResource(t *testing.T) {
	store := NewFindingsStore()

	// Add findings for different resources
	f1 := &Finding{
		ID:           "f1",
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		Severity:     FindingSeverityWarning,
		Title:        "Finding 1",
	}
	f2 := &Finding{
		ID:           "f2",
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		Severity:     FindingSeverityCritical,
		Title:        "Finding 2",
	}
	f3 := &Finding{
		ID:           "f3",
		ResourceID:   "res-2",
		ResourceName: "Resource 2",
		Severity:     FindingSeverityWarning,
		Title:        "Finding 3",
	}

	store.Add(f1)
	store.Add(f2)
	store.Add(f3)

	// Get findings for res-1
	res1Findings := store.GetByResource("res-1")
	if len(res1Findings) != 2 {
		t.Errorf("Expected 2 findings for res-1, got %d", len(res1Findings))
	}

	// Get findings for res-2
	res2Findings := store.GetByResource("res-2")
	if len(res2Findings) != 1 {
		t.Errorf("Expected 1 finding for res-2, got %d", len(res2Findings))
	}

	// Get findings for non-existent resource
	noFindings := store.GetByResource("non-existent")
	if len(noFindings) != 0 {
		t.Errorf("Expected 0 findings for non-existent resource, got %d", len(noFindings))
	}

	// Resolve one finding and verify it's excluded from GetByResource
	store.Resolve("f1", false)
	res1FindingsAfterResolve := store.GetByResource("res-1")
	if len(res1FindingsAfterResolve) != 1 {
		t.Errorf("Expected 1 active finding for res-1 after resolve, got %d", len(res1FindingsAfterResolve))
	}
}

func TestFindingsStore_GetAll(t *testing.T) {
	store := NewFindingsStore()

	now := time.Now()

	// Add findings at different times
	f1 := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Title:      "Finding 1",
		DetectedAt: now.Add(-48 * time.Hour),
	}
	f2 := &Finding{
		ID:         "f2",
		ResourceID: "res-1",
		Title:      "Finding 2",
		DetectedAt: now.Add(-12 * time.Hour),
	}
	f3 := &Finding{
		ID:         "f3",
		ResourceID: "res-2",
		Title:      "Finding 3",
		DetectedAt: now.Add(-1 * time.Hour),
	}

	store.Add(f1)
	store.Add(f2)
	store.Add(f3)

	// Get all findings without filter
	allFindings := store.GetAll(nil)
	if len(allFindings) != 3 {
		t.Errorf("Expected 3 findings, got %d", len(allFindings))
	}

	// Get findings after 24 hours ago
	startTime := now.Add(-24 * time.Hour)
	recentFindings := store.GetAll(&startTime)
	if len(recentFindings) != 2 {
		t.Errorf("Expected 2 findings after 24h ago, got %d", len(recentFindings))
	}

	// Resolve a finding and verify it's still in GetAll (includes resolved)
	store.Resolve("f1", false)
	allAfterResolve := store.GetAll(nil)
	if len(allAfterResolve) != 3 {
		t.Errorf("Expected 3 findings (including resolved), got %d", len(allAfterResolve))
	}
}

type mockPersistence struct {
	savedCount int
}

func (m *mockPersistence) SaveFindings(findings map[string]*Finding) error {
	m.savedCount++
	return nil
}

func (m *mockPersistence) LoadFindings() (map[string]*Finding, error) {
	return make(map[string]*Finding), nil
}

func TestFindingsSummary_HasIssues(t *testing.T) {
	tests := []struct {
		name     string
		summary  FindingsSummary
		expected bool
	}{
		{
			name:     "no issues",
			summary:  FindingsSummary{Info: 1, Watch: 1},
			expected: false,
		},
		{
			name:     "has warning",
			summary:  FindingsSummary{Warning: 1},
			expected: true,
		},
		{
			name:     "has critical",
			summary:  FindingsSummary{Critical: 1},
			expected: true,
		},
		{
			name:     "has both",
			summary:  FindingsSummary{Critical: 2, Warning: 3},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.summary.HasIssues(); got != tt.expected {
				t.Errorf("HasIssues() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFindingsStore_GetDismissedFindings(t *testing.T) {
	store := NewFindingsStore()

	// Add normal finding
	f1 := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Title:      "Normal Finding",
		Severity:   FindingSeverityWarning,
	}
	store.Add(f1)

	// Add dismissed finding
	f2 := &Finding{
		ID:         "f2",
		ResourceID: "res-2",
		Title:      "Dismissed Finding",
		Severity:   FindingSeverityWarning,
	}
	store.Add(f2)
	store.Dismiss("f2", "not_an_issue", "Ignore this")

	// Add suppressed finding
	f3 := &Finding{
		ID:         "f3",
		ResourceID: "res-3",
		Title:      "Suppressed Finding",
		Severity:   FindingSeverityWarning,
	}
	store.Add(f3)
	store.Suppress("f3")

	dismissed := store.GetDismissedFindings()
	if len(dismissed) != 2 {
		t.Errorf("Expected 2 dismissed/suppressed findings, got %d", len(dismissed))
	}

	// Verify the dismissed findings are the right ones
	dismissedIDs := make(map[string]bool)
	for _, f := range dismissed {
		dismissedIDs[f.ID] = true
	}

	if !dismissedIDs["f2"] {
		t.Error("Expected f2 to be in dismissed findings")
	}
	if !dismissedIDs["f3"] {
		t.Error("Expected f3 to be in dismissed findings")
	}
	if dismissedIDs["f1"] {
		t.Error("f1 should NOT be in dismissed findings")
	}
}

func TestFindingsStore_MatchesSuppressionRule(t *testing.T) {
	store := NewFindingsStore()

	// Add a suppression rule for specific resource+category
	store.AddSuppressionRule("res-1", "Resource 1", FindingCategoryPerformance, "Known issue")

	// Should match
	if !store.MatchesSuppressionRule("res-1", FindingCategoryPerformance) {
		t.Error("Should match exact resource+category")
	}

	// Should not match different resource
	if store.MatchesSuppressionRule("res-2", FindingCategoryPerformance) {
		t.Error("Should NOT match different resource")
	}

	// Should not match different category
	if store.MatchesSuppressionRule("res-1", FindingCategoryCapacity) {
		t.Error("Should NOT match different category")
	}

	// Add a wildcard rule (any resource for capacity)
	store.AddSuppressionRule("", "Any", FindingCategoryCapacity, "All capacity")

	// Should match any resource for capacity
	if !store.MatchesSuppressionRule("any-resource", FindingCategoryCapacity) {
		t.Error("Wildcard resource rule should match any resource")
	}

	// Add a wildcard rule (res-3 for any category)
	store.AddSuppressionRule("res-3", "Resource 3", "", "All categories for res-3")

	// Should match res-3 for any category
	if !store.MatchesSuppressionRule("res-3", FindingCategoryReliability) {
		t.Error("Wildcard category rule should match any category for that resource")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestFindingsStore_DeleteSuppressionRule_NotFound(t *testing.T) {
	store := NewFindingsStore()
	
	// Try to delete non-existent rule
	if store.DeleteSuppressionRule("nonexistent-id") {
		t.Error("DeleteSuppressionRule should return false for non-existent rule")
	}
}

func TestFindingsStore_SetPersistence_Nil(t *testing.T) {
	store := NewFindingsStore()
	
	// Setting nil persistence should succeed (disables persistence)
	err := store.SetPersistence(nil)
	if err != nil {
		t.Errorf("SetPersistence(nil) should succeed, got: %v", err)
	}
}

func TestFindingsStore_Acknowledge_NotFound(t *testing.T) {
	store := NewFindingsStore()
	
	if store.Acknowledge("nonexistent") {
		t.Error("Acknowledge should return false for non-existent finding")
	}
}

func TestFindingsStore_Dismiss_NotFound(t *testing.T) {
	store := NewFindingsStore()
	
	if store.Dismiss("nonexistent", "reason", "note") {
		t.Error("Dismiss should return false for non-existent finding")
	}
}

func TestFindingsStore_SetUserNote_NotFound(t *testing.T) {
	store := NewFindingsStore()
	
	if store.SetUserNote("nonexistent", "note") {
		t.Error("SetUserNote should return false for non-existent finding")
	}
}

func TestFindingsStore_Suppress_NotFound(t *testing.T) {
	store := NewFindingsStore()
	
	if store.Suppress("nonexistent") {
		t.Error("Suppress should return false for non-existent finding")
	}
}

func TestFindingsStore_Resolve_NotFound(t *testing.T) {
	store := NewFindingsStore()
	
	if store.Resolve("nonexistent", false) {
		t.Error("Resolve should return false for non-existent finding")
	}
}

func TestFindingsStore_Resolve_AlreadyResolved(t *testing.T) {
	store := NewFindingsStore()
	
	finding := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Test",
	}
	store.Add(finding)
	store.Resolve("f1", false)
	
	// Verify it's resolved
	f := store.Get("f1")
	if !f.IsResolved() {
		t.Error("Finding should be resolved after Resolve call")
	}
	
	// Try to resolve again - should return false
	if store.Resolve("f1", false) {
		t.Error("Resolve should return false for already-resolved finding")
	}
}

func TestFindingsStore_GetActive_Empty(t *testing.T) {
	store := NewFindingsStore()
	
	active := store.GetActive(FindingSeverityInfo)
	if len(active) != 0 {
		t.Errorf("Expected 0 active findings from empty store, got %d", len(active))
	}
}

func TestFindingsStore_GetSummary(t *testing.T) {
	store := NewFindingsStore()
	
	// Add findings of each severity
	store.Add(&Finding{ID: "crit", Severity: FindingSeverityCritical, ResourceID: "r1", Title: "Critical"})
	store.Add(&Finding{ID: "warn", Severity: FindingSeverityWarning, ResourceID: "r2", Title: "Warning"})
	store.Add(&Finding{ID: "watch", Severity: FindingSeverityWatch, ResourceID: "r3", Title: "Watch"})
	store.Add(&Finding{ID: "info", Severity: FindingSeverityInfo, ResourceID: "r4", Title: "Info"})
	
	summary := store.GetSummary()
	
	if summary.Critical != 1 {
		t.Errorf("Expected 1 critical, got %d", summary.Critical)
	}
	if summary.Warning != 1 {
		t.Errorf("Expected 1 warning, got %d", summary.Warning)
	}
	if summary.Watch != 1 {
		t.Errorf("Expected 1 watch, got %d", summary.Watch)
	}
	if summary.Info != 1 {
		t.Errorf("Expected 1 info, got %d", summary.Info)
	}
	if summary.Total != 4 {
		t.Errorf("Expected 4 total, got %d", summary.Total)
	}
	if !summary.HasIssues() {
		t.Error("HasIssues should be true with critical/warning findings")
	}
}

func TestFindingsStore_GetDismissedForContext_Empty(t *testing.T) {
	store := NewFindingsStore()
	
	ctx := store.GetDismissedForContext()
	if ctx != "" {
		t.Errorf("Expected empty context for empty store, got: %s", ctx)
	}
}

func TestFinding_Status(t *testing.T) {
	// Test IsResolved
	resolved := Finding{
		ID:         "resolved",
		ResolvedAt: timePtr(time.Now()),
	}
	if !resolved.IsResolved() {
		t.Error("Finding with ResolvedAt set should be resolved")
	}
	
	notResolved := Finding{ID: "not-resolved"}
	if notResolved.IsResolved() {
		t.Error("Finding without ResolvedAt should not be resolved")
	}
	
	// Test IsDismissed
	dismissed := Finding{
		ID:              "dismissed",
		DismissedReason: "not_an_issue",
	}
	if !dismissed.IsDismissed() {
		t.Error("Finding with DismissedReason should be dismissed")
	}
}

// Note: Add(nil) panics in current implementation - not testing

func TestFindingsStore_Add_EmptyID(t *testing.T) {
	store := NewFindingsStore()
	
	finding := &Finding{
		ResourceID: "res-1",
		Title:      "Test",
	}
	
	// Should generate ID if empty
	store.Add(finding)
	
	// Verify something was added
	all := store.GetAll(nil)
	if len(all) != 1 {
		t.Error("Finding should be added even with empty ID")
	}
}

func TestFindingsStore_GetSuppressionRules_Empty(t *testing.T) {
	store := NewFindingsStore()
	
	rules := store.GetSuppressionRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules from new store, got %d", len(rules))
	}
}

// TestFindingsStore_Dismiss_DifferentReasons tests the dismissal behavior based on reason:
// - "not_an_issue": permanent suppression (true false positive)
// - "expected_behavior": 30-day snooze (user accepts risk, but re-check later)
// - "will_fix_later": just marked dismissed, no snooze or suppression
func TestFindingsStore_Dismiss_DifferentReasons(t *testing.T) {
	t.Run("not_an_issue creates permanent suppression", func(t *testing.T) {
		store := NewFindingsStore()
		f := &Finding{
			ID:         "f1",
			ResourceID: "res-1",
			Severity:   FindingSeverityWarning,
			Title:      "False Positive",
			Category:   FindingCategoryPerformance,
		}
		store.Add(f)

		store.Dismiss("f1", "not_an_issue", "Detection bug")

		finding := store.Get("f1")
		if !finding.Suppressed {
			t.Error("Expected finding to be suppressed for 'not_an_issue'")
		}
		if finding.SnoozedUntil != nil {
			t.Error("Should not set snooze for 'not_an_issue'")
		}
		if finding.DismissedReason != "not_an_issue" {
			t.Errorf("Expected DismissedReason 'not_an_issue', got %s", finding.DismissedReason)
		}
	})

	t.Run("expected_behavior acknowledges but stays visible", func(t *testing.T) {
		store := NewFindingsStore()
		f := &Finding{
			ID:         "f2",
			ResourceID: "res-2",
			Severity:   FindingSeverityWarning,
			Title:      "Expected Behavior",
			Category:   FindingCategoryCapacity,
		}
		store.Add(f)

		store.Dismiss("f2", "expected_behavior", "Disk fills during backup")

		finding := store.Get("f2")
		if finding.Suppressed {
			t.Error("Should NOT set Suppressed for 'expected_behavior'")
		}
		if finding.SnoozedUntil != nil {
			t.Error("Should NOT set snooze for 'expected_behavior' (just acknowledges)")
		}
		if finding.AcknowledgedAt == nil {
			t.Error("Expected AcknowledgedAt to be set for 'expected_behavior'")
		}
		if finding.DismissedReason != "expected_behavior" {
			t.Errorf("Expected DismissedReason 'expected_behavior', got %s", finding.DismissedReason)
		}
		if finding.UserNote != "Disk fills during backup" {
			t.Errorf("Expected note to be saved, got %s", finding.UserNote)
		}

		// Finding is marked dismissed, so IsDismissed() returns true
		if !finding.IsDismissed() {
			t.Error("Finding should be marked as dismissed")
		}
	})

	t.Run("will_fix_later acknowledges but stays visible", func(t *testing.T) {
		store := NewFindingsStore()
		f := &Finding{
			ID:         "f3",
			ResourceID: "res-3",
			Severity:   FindingSeverityWarning,
			Title:      "Will Fix Later",
			Category:   FindingCategoryReliability,
		}
		store.Add(f)

		store.Dismiss("f3", "will_fix_later", "Low priority")

		finding := store.Get("f3")
		if finding.Suppressed {
			t.Error("Should NOT suppress for 'will_fix_later'")
		}
		if finding.SnoozedUntil != nil {
			t.Error("Should NOT snooze for 'will_fix_later'")
		}
		if finding.AcknowledgedAt == nil {
			t.Error("Expected AcknowledgedAt to be set")
		}
		if finding.DismissedReason != "will_fix_later" {
			t.Errorf("Expected DismissedReason 'will_fix_later', got %s", finding.DismissedReason)
		}
		// Finding is marked dismissed
		if !finding.IsDismissed() {
			t.Error("Finding should be marked as dismissed")
		}
	})
}
