package ai

import (
	"testing"
	"time"
)

func TestFinding_InvestigationMethods(t *testing.T) {
	f := &Finding{
		ID:    "finding-1",
		Title: "Test Finding",
	}

	// 1. Set Session ID
	f.SetInvestigationSessionID("session-123")
	if f.GetInvestigationSessionID() != "session-123" {
		t.Errorf("Expected session ID 'session-123', got %s", f.GetInvestigationSessionID())
	}

	// 2. Set Status
	f.SetInvestigationStatus(string(InvestigationStatusRunning))
	if f.GetInvestigationStatus() != string(InvestigationStatusRunning) {
		t.Errorf("Expected status 'running', got %s", f.GetInvestigationStatus())
	}

	// 3. Set Outcome
	f.SetInvestigationOutcome(string(InvestigationOutcomeNeedsAttention))
	if f.GetInvestigationOutcome() != string(InvestigationOutcomeNeedsAttention) {
		t.Errorf("Expected outcome 'needs_attention', got %s", f.GetInvestigationOutcome())
	}

	// 4. Set LastInvestigatedAt
	now := time.Now()
	f.SetLastInvestigatedAt(&now)
	if !f.GetLastInvestigatedAt().Equal(now) {
		t.Error("Expected LastInvestigatedAt to be set")
	}

	// 5. Set Attempts
	f.SetInvestigationAttempts(3)
	if f.GetInvestigationAttempts() != 3 {
		t.Errorf("Expected attempts 3, got %d", f.GetInvestigationAttempts())
	}

	// 6. Test Getters for other fields
	f.Severity = FindingSeverityCritical
	if f.GetSeverity() != string(FindingSeverityCritical) {
		t.Error("GetSeverity mismatch")
	}
	f.Category = FindingCategoryPerformance
	if f.GetCategory() != string(FindingCategoryPerformance) {
		t.Error("GetCategory mismatch")
	}
	f.ResourceID = "res-1"
	if f.GetResourceID() != "res-1" {
		t.Error("GetResourceID mismatch")
	}
}

func TestFindingsStore_UpdateInvestigation(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:    "finding-1",
		Title: "Test Finding",
	}
	store.Add(f)

	now := time.Now()
	// Update all investigation fields
	if !store.UpdateInvestigation("finding-1", "session-new", string(InvestigationStatusCompleted), string(InvestigationOutcomeResolved), &now, 1) {
		t.Fatal("UpdateInvestigation failed")
	}

	updated := store.Get("finding-1")
	if updated.InvestigationSessionID != "session-new" {
		t.Errorf("Expected session 'session-new', got %s", updated.InvestigationSessionID)
	}
	if updated.InvestigationStatus != string(InvestigationStatusCompleted) {
		t.Errorf("Expected status 'completed', got %s", updated.InvestigationStatus)
	}
	if updated.InvestigationOutcome != string(InvestigationOutcomeResolved) {
		t.Errorf("Expected outcome 'resolved', got %s", updated.InvestigationOutcome)
	}
	if updated.InvestigationAttempts != 1 {
		t.Errorf("Expected attempts 1, got %d", updated.InvestigationAttempts)
	}
	if !updated.LastInvestigatedAt.Equal(now) {
		t.Error("Expected LastInvestigatedAt to be updated")
	}

	// Update non-existent finding
	if store.UpdateInvestigation("non-existent", "", "", "", nil, 0) {
		t.Error("Expected false for non-existent finding")
	}
}

func TestFindingsStore_UpdateInvestigationOutcome(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:    "finding-1",
		Title: "Test Finding",
	}
	store.Add(f)

	// Update outcome
	if !store.UpdateInvestigationOutcome("finding-1", string(InvestigationOutcomeFixQueued)) {
		t.Fatal("UpdateInvestigationOutcome failed")
	}

	updated := store.Get("finding-1")
	if updated.InvestigationOutcome != string(InvestigationOutcomeFixQueued) {
		t.Errorf("Expected outcome 'fix_queued', got %s", updated.InvestigationOutcome)
	}

	// Update non-existent finding
	if store.UpdateInvestigationOutcome("non-existent", "val") {
		t.Error("Expected false for non-existent finding")
	}
}

func TestFinding_CanRetryInvestigation(t *testing.T) {
	// 1. Fresh finding - should be able to retry
	f1 := &Finding{ID: "f1", Title: "Fresh", InvestigationAttempts: 0}
	if !f1.CanRetryInvestigation() {
		t.Error("Fresh finding should be retryable")
	}

	// 2. Max attempts reached
	f2 := &Finding{ID: "f2", InvestigationAttempts: 3}
	if f2.CanRetryInvestigation() {
		t.Error("Should not retry if max attempts reached")
	}

	// 3. Recently investigated (cooldown 1h not met)
	now := time.Now()
	f3 := &Finding{
		ID:                    "f3",
		InvestigationAttempts: 1,
		LastInvestigatedAt:    &now, // Just now
	}
	if f3.CanRetryInvestigation() {
		t.Error("Should not retry if cooldown not met")
	}

	// 4. Cooldown met
	past := now.Add(-2 * time.Hour)
	f4 := &Finding{
		ID:                    "f4",
		InvestigationAttempts: 1,
		LastInvestigatedAt:    &past,
	}
	if !f4.CanRetryInvestigation() {
		t.Error("Should retry if cooldown met")
	}

	// 5. Currently running
	f5 := &Finding{
		ID:                  "f5",
		InvestigationStatus: string(InvestigationStatusRunning),
	}
	if f5.CanRetryInvestigation() {
		t.Error("Should not retry if currently running")
	}
}
