package ai

import (
	"testing"
	"time"
)

func TestFindingInvestigationReplayPreservesResolutionHistory(t *testing.T) {
	store := NewFindingsStore()
	store.Add(&Finding{ID: "replay", ResourceID: "resource", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, Title: "Failure"})
	observed := time.Now().Add(-time.Hour)
	store.UpdateInvestigation("replay", "session", "completed", string(InvestigationOutcomeFixVerified), &observed, 1)
	before := store.Get("replay")
	if before.ResolvedAt == nil {
		t.Fatal("first verified result did not resolve finding")
	}
	for i := 0; i < 3; i++ {
		store.UpdateInvestigationOutcome("replay", string(InvestigationOutcomeFixVerified))
		store.UpdateInvestigation("replay", "session", "completed", string(InvestigationOutcomeFixVerified), &observed, 1)
	}
	after := store.Get("replay")
	if !after.ResolvedAt.Equal(*before.ResolvedAt) {
		t.Fatalf("resolution moved from %v to %v", before.ResolvedAt, after.ResolvedAt)
	}
	if len(after.Lifecycle) != len(before.Lifecycle) {
		t.Fatalf("replay appended lifecycle events: %d -> %d", len(before.Lifecycle), len(after.Lifecycle))
	}
	if got := store.activeCounts[FindingSeverityWarning]; got != 0 {
		t.Fatalf("active count = %d", got)
	}
	// A newly observed regression is a different event and must still resolve.
	store.mu.Lock()
	store.findings["replay"].ResolvedAt = nil
	store.activeCounts[FindingSeverityWarning] = 1
	store.mu.Unlock()
	store.UpdateInvestigationOutcome("replay", string(InvestigationOutcomeFixVerified))
	regressed := store.Get("replay")
	if regressed.ResolvedAt == nil || len(regressed.Lifecycle) != len(before.Lifecycle)+1 {
		t.Fatal("verified regression did not create a new resolution")
	}
}

func TestFindingInvestigationReplayDoesNotDuplicateNonResolutionHistory(t *testing.T) {
	store := NewFindingsStore()
	store.Add(&Finding{ID: "replay", ResourceID: "resource", Severity: FindingSeverityWarning, Title: "Failure"})
	store.UpdateInvestigation("replay", "session", "completed", string(InvestigationOutcomeNeedsAttention), nil, 1)
	before := store.Get("replay")
	store.UpdateInvestigationOutcome("replay", string(InvestigationOutcomeNeedsAttention))
	store.UpdateInvestigation("replay", "session", "completed", string(InvestigationOutcomeNeedsAttention), nil, 1)
	if after := store.Get("replay"); len(after.Lifecycle) != len(before.Lifecycle) {
		t.Fatal("replay appended outcome history")
	}
}
