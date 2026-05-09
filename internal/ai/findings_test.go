package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
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

func TestFindingsStore_SuppressedPersistsInContextAndCleanup(t *testing.T) {
	store := NewFindingsStore()
	old := time.Now().Add(-60 * 24 * time.Hour)

	store.findings["suppressed"] = &Finding{
		ID:              "suppressed",
		Title:           "Suppressed Finding",
		ResourceName:    "host-1",
		Suppressed:      true,
		DismissedReason: "not_an_issue",
		LastSeenAt:      old,
	}

	removed := store.Cleanup(24 * time.Hour)
	if removed != 0 {
		t.Errorf("Expected 0 findings removed, got %d", removed)
	}
	if store.Get("suppressed") == nil {
		t.Error("suppressed finding should NOT have been removed")
	}

	ctx := store.GetDismissedForContext()
	if !strings.Contains(ctx, "Suppressed Finding") {
		t.Error("expected suppressed finding to remain in context")
	}
}

func TestFindingsStore_Cleanup_RemovesOldDismissed(t *testing.T) {
	store := NewFindingsStore()
	old := time.Now().Add(-31 * 24 * time.Hour)

	store.findings["dismissed"] = &Finding{
		ID:              "dismissed",
		Title:           "Dismissed Finding",
		ResourceName:    "host-2",
		DismissedReason: "expected_behavior",
		LastSeenAt:      old,
	}

	removed := store.Cleanup(24 * time.Hour)
	if removed != 1 {
		t.Errorf("Expected 1 finding removed, got %d", removed)
	}
	if store.Get("dismissed") != nil {
		t.Error("dismissed finding should have been removed")
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

func TestFindingsStore_GetDismissedForContextForResources(t *testing.T) {
	store := NewFindingsStore()

	inScope := &Finding{
		ID:              "f-in",
		Title:           "High CPU on web-1",
		ResourceID:      "web-1",
		ResourceName:    "web-1",
		Category:        FindingCategoryPerformance,
		DismissedReason: "not_an_issue",
		LastSeenAt:      time.Now(),
	}
	outOfScope := &Finding{
		ID:              "f-out",
		Title:           "Disk nearly full on db-1",
		ResourceID:      "db-1",
		ResourceName:    "db-1",
		Category:        FindingCategoryCapacity,
		DismissedReason: "expected_behavior",
		LastSeenAt:      time.Now(),
	}
	store.Add(inScope)
	store.Add(outOfScope)

	ctx := store.GetDismissedForContextForResources(map[string]bool{"web-1": true})

	if !strings.Contains(ctx, "High CPU on web-1") {
		t.Fatal("expected scoped dismissed finding in context")
	}
	if strings.Contains(ctx, "Disk nearly full on db-1") {
		t.Fatal("expected out-of-scope dismissed finding to be omitted")
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
	if stored.TimesRaised != 2 {
		t.Errorf("Expected TimesRaised=2, got %d", stored.TimesRaised)
	}
}

// TestFindingsStore_GetTrustSummary covers the snapshot counts that answer
// the operator-facing "do I trust Patrol?" question. It verifies the bucket
// boundaries for each finding outcome the summary tracks.
func TestFindingsStore_GetTrustSummary(t *testing.T) {
	store := NewFindingsStore()

	// Active finding (no resolution, no dismissal)
	store.Add(&Finding{
		ID: "f-active", ResourceID: "r1", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Active",
	})

	// Auto-resolved finding (resolved + auto_resolved=true)
	store.Add(&Finding{
		ID: "f-auto", ResourceID: "r2", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Auto",
	})
	store.Resolve("f-auto", true)

	// Fix-verified finding (investigation_outcome = fix_verified)
	store.Add(&Finding{
		ID: "f-verified", ResourceID: "r3", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Verified",
	})
	store.UpdateInvestigationOutcome("f-verified", string(aicontracts.OutcomeFixVerified))

	// Fix-failed finding
	store.Add(&Finding{
		ID: "f-failed", ResourceID: "r4", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Failed",
	})
	store.UpdateInvestigationOutcome("f-failed", string(aicontracts.OutcomeFixFailed))

	// Dismissed as noise
	store.Add(&Finding{
		ID: "f-noise", ResourceID: "r5", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Noise",
	})
	store.Dismiss("f-noise", "not_an_issue", "")

	// Dismissed as expected
	store.Add(&Finding{
		ID: "f-expected", ResourceID: "r6", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Expected",
	})
	store.Dismiss("f-expected", "expected_behavior", "Maintenance window")

	// Regressed at least once: setup via the regression branch
	store.Add(&Finding{
		ID: "f-regressed", ResourceID: "r7", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Regressed",
	})
	store.Resolve("f-regressed", true)
	store.Add(&Finding{
		ID: "f-regressed", ResourceID: "r7", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: "Regressed",
	})

	got := store.GetTrustSummary()
	if got.Tracked != 7 {
		t.Errorf("Tracked = %d, want 7", got.Tracked)
	}
	// AutoResolved counts findings that were resolved without operator action.
	// Both Resolve(auto=true) and UpdateInvestigationOutcome(fix_verified) set
	// the AutoResolved flag, so f-auto and f-verified both contribute.
	if got.AutoResolved != 2 {
		t.Errorf("AutoResolved = %d, want 2", got.AutoResolved)
	}
	if got.FixVerified != 1 {
		t.Errorf("FixVerified = %d, want 1", got.FixVerified)
	}
	if got.FixFailed != 1 {
		t.Errorf("FixFailed = %d, want 1", got.FixFailed)
	}
	if got.DismissedAsNoise != 1 {
		t.Errorf("DismissedAsNoise = %d, want 1", got.DismissedAsNoise)
	}
	if got.DismissedAsExpected != 1 {
		t.Errorf("DismissedAsExpected = %d, want 1", got.DismissedAsExpected)
	}
	if got.RegressedAtLeastOnce != 1 {
		t.Errorf("RegressedAtLeastOnce = %d, want 1", got.RegressedAtLeastOnce)
	}
}

// TestFindingsStore_Add_CapturesPreviousResolvedFixOnRegression covers the
// regression branch: when a finding that had a resolved investigation with a
// proposed fix is re-detected, the prior fix description must be preserved on
// PreviousResolvedFixSummary as operational memory before InvestigationRecord
// is cleared. Without this capture the next investigation cannot reason about
// what worked last time.
func TestFindingsStore_Add_CapturesPreviousResolvedFixOnRegression(t *testing.T) {
	store := NewFindingsStore()

	// Initial detection
	store.Add(&Finding{
		ID:          "f-regress",
		ResourceID:  "vm-100",
		Severity:    FindingSeverityWarning,
		Category:    FindingCategoryReliability,
		Title:       "Service stalled",
		Description: "Service stopped responding",
	})

	// Mark the finding as resolved with an investigation record carrying a
	// proposed fix, mimicking a successful Patrol-then-fix loop. UpdateInvestigationRecord
	// and ResolveWithReason mutate the canonical stored finding (Get returns a copy).
	store.UpdateInvestigationRecord("f-regress", &aicontracts.InvestigationRecord{
		ID:        "investigation-1",
		FindingID: "f-regress",
		Status:    aicontracts.InvestigationStatusCompleted,
		Outcome:   aicontracts.OutcomeFixVerified,
		ProposedFix: &aicontracts.InvestigationRecordFix{
			ID:          "fix-1",
			Description: "Restart the workload service after backup window clears",
			Commands:    []string{"systemctl restart workload.service"},
		},
		Verification: []string{"Service is responsive within 60s"},
		ToolsUsed:    []string{"ssh.exec"},
	})
	store.ResolveWithReason("f-regress", "verified-recovery")

	// Re-detection of the same finding triggers the regression branch.
	store.Add(&Finding{
		ID:          "f-regress",
		ResourceID:  "vm-100",
		Severity:    FindingSeverityWarning,
		Category:    FindingCategoryReliability,
		Title:       "Service stalled",
		Description: "Service stopped responding again",
	})

	regressed := store.Get("f-regress")
	if regressed.RegressionCount != 1 {
		t.Fatalf("expected RegressionCount=1, got %d", regressed.RegressionCount)
	}
	if regressed.InvestigationRecord != nil {
		t.Fatalf("expected InvestigationRecord cleared on regression, got %#v", regressed.InvestigationRecord)
	}
	want := "Restart the workload service after backup window clears"
	if regressed.PreviousResolvedFixSummary != want {
		t.Fatalf("expected PreviousResolvedFixSummary=%q, got %q", want, regressed.PreviousResolvedFixSummary)
	}
}

// TestFindingsStore_Add_PropagatesImpactFieldOnUpdate covers the dedup-merge
// path where a re-detected finding carries newly-authored Impact text. The
// existing finding may have been persisted before Impact was a contract field,
// so the store must overwrite Impact alongside Description/Recommendation
// rather than preserve the stale empty value.
func TestFindingsStore_Add_PropagatesImpactFieldOnUpdate(t *testing.T) {
	store := NewFindingsStore()

	store.Add(&Finding{
		ID:          "f-impact",
		ResourceID:  "res-1",
		Severity:    FindingSeverityWarning,
		Title:       "Initial",
		Description: "Initial",
		// Impact not authored on first add (e.g., persisted by older binary)
	})

	store.Add(&Finding{
		ID:          "f-impact",
		ResourceID:  "res-1",
		Severity:    FindingSeverityWarning,
		Title:       "Initial",
		Description: "Initial",
		Impact:      "Workload stalls until pressure clears.",
	})

	stored := store.Get("f-impact")
	if stored == nil {
		t.Fatalf("expected finding to be present after re-add")
	}
	if stored.Impact != "Workload stalls until pressure clears." {
		t.Fatalf("expected Impact to overwrite on update, got %q", stored.Impact)
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

func TestFindingsStore_Add_ReactivatesResolved(t *testing.T) {
	store := NewFindingsStore()

	f1 := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Finding 1",
	}
	store.Add(f1)
	store.Resolve("f1", false)

	activeAfterResolve := store.GetActive(FindingSeverityWarning)
	if len(activeAfterResolve) != 0 {
		t.Fatalf("Expected 0 active findings after resolve, got %d", len(activeAfterResolve))
	}

	f1Repeat := &Finding{
		ID:         "f1",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Finding 1 reoccurred",
	}
	store.Add(f1Repeat)

	active := store.GetActive(FindingSeverityWarning)
	if len(active) != 1 {
		t.Fatalf("Expected 1 active finding after re-add, got %d", len(active))
	}
	if active[0].ResolvedAt != nil {
		t.Fatal("Expected resolved finding to be reactivated")
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

func TestFindingsStore_Dismiss_WillFixLater_DefaultsRemindAt(t *testing.T) {
	// will_fix_later is meant to be an operational commitment, not a silent
	// shut-up. Dismiss must populate RemindAt so the finding can wake itself
	// after the deadline; expected_behavior must NOT populate it (that's
	// the "acknowledged forever" semantic), and not_an_issue must clear it
	// (that's the "permanent suppression" semantic).
	t.Run("will_fix_later sets RemindAt ~7 days out", func(t *testing.T) {
		store := NewFindingsStore()
		f := &Finding{
			ID:         "f-wfl",
			ResourceID: "res-1",
			Severity:   FindingSeverityWarning,
			Title:      "Disk pressure",
		}
		store.Add(f)

		before := time.Now()
		store.Dismiss("f-wfl", DismissReasonWillFixLater, "Plan to upgrade Q3")
		after := time.Now()

		got := store.Get("f-wfl")
		if got.RemindAt == nil {
			t.Fatal("Expected RemindAt to be set for will_fix_later")
		}
		expectedMin := before.Add(DefaultWillFixLaterRemindAfter)
		expectedMax := after.Add(DefaultWillFixLaterRemindAfter)
		if got.RemindAt.Before(expectedMin) || got.RemindAt.After(expectedMax) {
			t.Errorf("RemindAt should be ~7 days out, got %v (expected window %v..%v)", got.RemindAt, expectedMin, expectedMax)
		}
	})

	t.Run("expected_behavior does not set RemindAt", func(t *testing.T) {
		store := NewFindingsStore()
		store.Add(&Finding{ID: "f-eb", ResourceID: "res-2", Severity: FindingSeverityWarning})
		store.Dismiss("f-eb", "expected_behavior", "Known")
		if got := store.Get("f-eb"); got.RemindAt != nil {
			t.Errorf("expected_behavior must not set RemindAt; got %v", got.RemindAt)
		}
	})

	t.Run("not_an_issue clears RemindAt", func(t *testing.T) {
		store := NewFindingsStore()
		store.Add(&Finding{ID: "f-nai", ResourceID: "res-3", Severity: FindingSeverityWarning})
		// Pre-stage a stale RemindAt to confirm not_an_issue clears it.
		store.Dismiss("f-nai", DismissReasonWillFixLater, "later")
		if store.Get("f-nai").RemindAt == nil {
			t.Fatal("setup: RemindAt should have been set by will_fix_later")
		}
		store.Undismiss("f-nai")
		store.Dismiss("f-nai", "not_an_issue", "false positive")
		if got := store.Get("f-nai"); got.RemindAt != nil {
			t.Errorf("not_an_issue must clear RemindAt; got %v", got.RemindAt)
		}
	})

	t.Run("Undismiss clears RemindAt", func(t *testing.T) {
		store := NewFindingsStore()
		store.Add(&Finding{ID: "f-un", ResourceID: "res-4", Severity: FindingSeverityWarning})
		store.Dismiss("f-un", DismissReasonWillFixLater, "later")
		store.Undismiss("f-un")
		if got := store.Get("f-un"); got.RemindAt != nil {
			t.Errorf("Undismiss must clear RemindAt; got %v", got.RemindAt)
		}
	})
}

func TestFindingsStore_Add_WillFixLater_StaysQuietBeforeRemindAt(t *testing.T) {
	// While the will_fix_later remind-at is still in the future, re-detection
	// at same severity must NOT reactivate the finding — the operator's
	// commitment is still "honored" and Pulse stays quiet.
	store := NewFindingsStore()
	store.Add(&Finding{
		ID:         "f-quiet",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Disk pressure",
	})
	store.Dismiss("f-quiet", DismissReasonWillFixLater, "Q3 upgrade")

	// Re-detect at same severity while RemindAt is still in the future.
	store.Add(&Finding{
		ID:         "f-quiet",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Disk pressure (again)",
	})

	got := store.Get("f-quiet")
	if !got.IsDismissed() {
		t.Error("Finding must stay dismissed before RemindAt fires")
	}
	if got.DismissedReason != DismissReasonWillFixLater {
		t.Errorf("DismissedReason should still be will_fix_later, got %s", got.DismissedReason)
	}
	if got.TimesRaised < 1 {
		t.Error("TimesRaised must still increment to track recurrences while quiet")
	}
}

func TestFindingsStore_Add_WillFixLater_WakesAfterRemindAt(t *testing.T) {
	// Once the will_fix_later remind-at has passed, the next re-detection
	// must clear the dismissal and surface the finding again with a
	// "reminded" lifecycle event so the operator sees their commitment lapsed.
	store := NewFindingsStore()
	store.Add(&Finding{
		ID:         "f-wake",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Disk pressure",
	})
	store.Dismiss("f-wake", DismissReasonWillFixLater, "Q3 upgrade")

	// Backdate RemindAt so it has already passed.
	store.mu.Lock()
	past := time.Now().Add(-1 * time.Hour)
	store.findings["f-wake"].RemindAt = &past
	store.mu.Unlock()

	// Re-detect at same severity — should now wake.
	store.Add(&Finding{
		ID:         "f-wake",
		ResourceID: "res-1",
		Severity:   FindingSeverityWarning,
		Title:      "Disk pressure (still)",
	})

	got := store.Get("f-wake")
	if got.IsDismissed() {
		t.Error("Finding must be un-dismissed once RemindAt has passed")
	}
	if got.DismissedReason != "" {
		t.Errorf("DismissedReason must be cleared after wake, got %s", got.DismissedReason)
	}
	if got.RemindAt != nil {
		t.Errorf("RemindAt must be cleared after wake, got %v", got.RemindAt)
	}
	// UserNote should be preserved on remind-at wake (operator's promise is still relevant context).
	if got.UserNote != "Q3 upgrade" {
		t.Errorf("UserNote must be preserved across remind-at wake; got %q", got.UserNote)
	}
	// Lifecycle must record the "reminded" event so the operator sees their commitment lapsed.
	foundReminded := false
	for _, ev := range got.Lifecycle {
		if ev.Type == "reminded" {
			foundReminded = true
			break
		}
	}
	if !foundReminded {
		t.Error("Lifecycle must contain a 'reminded' event after will_fix_later wake")
	}
}

func TestFinding_RemindAt_RoundTripsThroughJSON(t *testing.T) {
	// RemindAt must persist across save/load so the operator's commitment
	// survives a process restart.
	when := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	original := Finding{
		ID:              "f-json",
		ResourceID:      "res-1",
		Severity:        FindingSeverityWarning,
		DismissedReason: DismissReasonWillFixLater,
		RemindAt:        &when,
	}
	bytes, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if !strings.Contains(string(bytes), "\"remind_at\"") {
		t.Errorf("MarshalJSON output should contain remind_at field, got %s", string(bytes))
	}
	var roundTripped Finding
	if err := roundTripped.UnmarshalJSON(bytes); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if roundTripped.RemindAt == nil {
		t.Fatal("RemindAt must round-trip through JSON")
	}
	if !roundTripped.RemindAt.Equal(when) {
		t.Errorf("RemindAt round-trip mismatch: got %v want %v", roundTripped.RemindAt, when)
	}
}

func TestFindingsStore_Add_AutoAcknowledgesDuringMaintenanceWindow(t *testing.T) {
	// When the operator has set a maintenance window covering the
	// finding's resource, the new-finding path must auto-dismiss the
	// finding with reason "expected_behavior" and record a lifecycle
	// event naming the maintenance window. The finding still lands in
	// durable history (so the operator can audit what tripped during
	// the window) but does not surface as active.
	store := NewFindingsStore()
	now := time.Now()
	provider := ResourceOperatorStateProviderFunc(func(canonicalID string, queriedAt time.Time) (ResourceOperatorStateProjection, bool) {
		if canonicalID != "vm:101" {
			return ResourceOperatorStateProjection{}, false
		}
		return ResourceOperatorStateProjection{
			MaintenanceWindow: &ResourceOperatorStateMaintenanceWindow{
				StartAt: now.Add(-time.Hour),
				EndAt:   now.Add(time.Hour),
				Reason:  "Q3 storage upgrade",
			},
		}, true
	})
	store.SetResourceOperatorStateProvider(provider)

	store.Add(&Finding{
		ID:         "f1",
		ResourceID: "vm:101",
		Severity:   FindingSeverityWarning,
		Title:      "CPU saturated",
	})

	got := store.Get("f1")
	if got == nil {
		t.Fatal("finding must be persisted even when auto-dismissed; durable history matters")
	}
	if got.DismissedReason != "expected_behavior" {
		t.Errorf("expected DismissedReason=expected_behavior; got %q", got.DismissedReason)
	}
	if got.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt must be populated for auto-dismissed findings")
	}
	if !strings.Contains(got.UserNote, "operator-set maintenance window") {
		t.Errorf("UserNote should explain the maintenance suppression; got %q", got.UserNote)
	}
	if !strings.Contains(got.UserNote, "Q3 storage upgrade") {
		t.Errorf("UserNote should include the operator's reason; got %q", got.UserNote)
	}
	// Lifecycle must record the auto-dismiss with the canonical
	// operator_state_cause metadata so the timeline can attribute the
	// suppression to its source.
	foundMaintenanceEvent := false
	for _, ev := range got.Lifecycle {
		if ev.Type == "dismissed" && ev.Metadata["operator_state_cause"] == "maintenance_window" {
			foundMaintenanceEvent = true
			break
		}
	}
	if !foundMaintenanceEvent {
		t.Error("expected a 'dismissed' lifecycle event with operator_state_cause=maintenance_window")
	}
}

func TestFindingsStore_Add_DoesNotAutoAcknowledgeOutsideMaintenanceWindow(t *testing.T) {
	// A provider that reports no active window must not affect the
	// new-finding default path — the finding lands as active. Pin this
	// so the suppression branch can't drift into "always suppress when
	// provider is set".
	store := NewFindingsStore()
	provider := ResourceOperatorStateProviderFunc(func(canonicalID string, now time.Time) (ResourceOperatorStateProjection, bool) {
		return ResourceOperatorStateProjection{}, false
	})
	store.SetResourceOperatorStateProvider(provider)

	store.Add(&Finding{
		ID:         "f-active",
		ResourceID: "vm:101",
		Severity:   FindingSeverityWarning,
		Title:      "CPU saturated",
	})

	got := store.Get("f-active")
	if got == nil {
		t.Fatal("expected finding to be persisted")
	}
	if got.DismissedReason != "" {
		t.Errorf("finding must remain active when provider reports no window; got DismissedReason=%q", got.DismissedReason)
	}
	if !got.IsActive() {
		t.Error("finding must report IsActive=true outside maintenance windows")
	}
}

func TestFindingsStore_Add_AutoAcknowledgesOnIntentionallyOfflineResource(t *testing.T) {
	// When the operator has marked a resource as intentionally offline,
	// any new finding raised against it must be auto-dismissed as
	// expected_behavior with operator_state_cause=intentionally_offline.
	// The intent is an indefinite "shut up about this resource" — no
	// time window — so the lifecycle event must not include a
	// maintenance_end_at field.
	store := NewFindingsStore()
	provider := ResourceOperatorStateProviderFunc(func(canonicalID string, now time.Time) (ResourceOperatorStateProjection, bool) {
		if canonicalID != "vm:archived" {
			return ResourceOperatorStateProjection{}, false
		}
		return ResourceOperatorStateProjection{
			IntentionallyOffline: true,
		}, true
	})
	store.SetResourceOperatorStateProvider(provider)

	store.Add(&Finding{
		ID:         "f-archived",
		ResourceID: "vm:archived",
		Severity:   FindingSeverityWarning,
		Title:      "Resource is offline",
	})

	got := store.Get("f-archived")
	if got == nil {
		t.Fatal("finding must be persisted even when auto-dismissed")
	}
	if got.DismissedReason != "expected_behavior" {
		t.Errorf("expected DismissedReason=expected_behavior; got %q", got.DismissedReason)
	}
	if got.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt must be populated for auto-dismissed findings")
	}
	if !strings.Contains(got.UserNote, "intentionally offline") {
		t.Errorf("UserNote should explain the intentionally-offline suppression; got %q", got.UserNote)
	}
	foundEvent := false
	for _, ev := range got.Lifecycle {
		if ev.Type == "dismissed" && ev.Metadata["operator_state_cause"] == "intentionally_offline" {
			foundEvent = true
			if _, hasEnd := ev.Metadata["maintenance_end_at"]; hasEnd {
				t.Error("intentionally_offline event must not carry maintenance_end_at metadata; the suppression is indefinite")
			}
			break
		}
	}
	if !foundEvent {
		t.Error("expected a 'dismissed' lifecycle event with operator_state_cause=intentionally_offline")
	}
}

func TestFindingsStore_Add_MaintenanceWindowTakesPriorityOverIntentionallyOffline(t *testing.T) {
	// When both signals are active for the same resource, the
	// maintenance window must win because it's time-bounded — operators
	// will see the suppression auto-clear when the window ends, which
	// is more honest than the indefinite intentionally-offline branch.
	store := NewFindingsStore()
	now := time.Now()
	provider := ResourceOperatorStateProviderFunc(func(canonicalID string, queriedAt time.Time) (ResourceOperatorStateProjection, bool) {
		return ResourceOperatorStateProjection{
			IntentionallyOffline: true,
			MaintenanceWindow: &ResourceOperatorStateMaintenanceWindow{
				StartAt: now.Add(-time.Hour),
				EndAt:   now.Add(time.Hour),
				Reason:  "active maintenance",
			},
		}, true
	})
	store.SetResourceOperatorStateProvider(provider)

	store.Add(&Finding{
		ID:         "f-both",
		ResourceID: "vm:101",
		Severity:   FindingSeverityWarning,
		Title:      "CPU saturated",
	})

	got := store.Get("f-both")
	if got == nil {
		t.Fatal("finding must be persisted")
	}
	foundMaintenance := false
	for _, ev := range got.Lifecycle {
		if ev.Type == "dismissed" && ev.Metadata["operator_state_cause"] == "maintenance_window" {
			foundMaintenance = true
			break
		}
	}
	if !foundMaintenance {
		t.Error("maintenance window must take priority over intentionally_offline when both are active")
	}
}

func TestFindingsStore_Add_OperatorStateSuppressionIsOptIn(t *testing.T) {
	// The default store (no provider wired) must behave exactly as
	// before — the operator-state feature must not regress the
	// existing new-finding path for any deployment that hasn't opted
	// in yet.
	store := NewFindingsStore()
	store.Add(&Finding{
		ID:         "f-default",
		ResourceID: "vm:101",
		Severity:   FindingSeverityWarning,
		Title:      "CPU saturated",
	})

	got := store.Get("f-default")
	if got == nil {
		t.Fatal("expected finding to be persisted on default store")
	}
	if got.DismissedReason != "" {
		t.Errorf("default-store finding must remain active; got DismissedReason=%q", got.DismissedReason)
	}
}

func TestFindingsStore_OperatorStateSuppressedFindingsDoNotTriggerInvestigation(t *testing.T) {
	// Cross-slice contract: when slice 31/32's operator-state suppression
	// fires (maintenance window or intentionally offline), the resulting
	// auto-dismissed finding must also be ineligible for autonomous
	// investigation. This is currently delivered by the chain
	// findings.Add → DismissedReason="expected_behavior" → ShouldInvestigate
	// returns false on the line `f.DismissedReason != ""` check, but the
	// relationship is implicit. Pinning it here so a future refactor of
	// either auto-dismiss or ShouldInvestigate cannot silently waste
	// investigation budget on operator-suppressed findings.

	cases := []struct {
		name      string
		signal    ResourceOperatorStateProjection
		wantCause string
	}{
		{
			name: "intentionally_offline",
			signal: ResourceOperatorStateProjection{
				IntentionallyOffline: true,
			},
			wantCause: "intentionally_offline",
		},
		{
			name: "maintenance_window",
			signal: ResourceOperatorStateProjection{
				MaintenanceWindow: &ResourceOperatorStateMaintenanceWindow{
					StartAt: time.Now().Add(-time.Hour),
					EndAt:   time.Now().Add(time.Hour),
				},
			},
			wantCause: "maintenance_window",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewFindingsStore()
			signal := tc.signal // capture for closure
			store.SetResourceOperatorStateProvider(
				ResourceOperatorStateProviderFunc(func(_ string, _ time.Time) (ResourceOperatorStateProjection, bool) {
					return signal, true
				}),
			)

			store.Add(&Finding{
				ID:         "f-suppressed",
				ResourceID: "vm:101",
				Severity:   FindingSeverityWarning,
				Title:      "CPU saturated",
			})

			got := store.Get("f-suppressed")
			if got == nil {
				t.Fatal("finding must be persisted even when auto-dismissed")
			}
			if got.DismissedReason != "expected_behavior" {
				t.Fatalf("auto-dismiss must run; got DismissedReason=%q", got.DismissedReason)
			}

			// The cross-slice contract: an auto-dismissed finding must
			// not be investigated. The autonomy level is irrelevant
			// here — even at "full" autonomy, investigation must skip
			// operator-suppressed findings because the operator has
			// already told Pulse to stay quiet.
			for _, autonomy := range []string{"approval", "assisted", "full"} {
				if got.ShouldInvestigate(autonomy) {
					t.Errorf("ShouldInvestigate(%q) must be false on operator-suppressed finding (cause=%s)", autonomy, tc.wantCause)
				}
			}

			// Sanity-check the lifecycle attribution still names the cause
			// — protects against the auto-dismiss branch silently changing
			// to a generic 'dismissed' that other tooling might
			// mis-categorize as operator-driven rather than operator-state
			// driven.
			foundCause := false
			for _, ev := range got.Lifecycle {
				if ev.Type == "dismissed" && ev.Metadata["operator_state_cause"] == tc.wantCause {
					foundCause = true
					break
				}
			}
			if !foundCause {
				t.Errorf("lifecycle must carry operator_state_cause=%s metadata", tc.wantCause)
			}
		})
	}
}
