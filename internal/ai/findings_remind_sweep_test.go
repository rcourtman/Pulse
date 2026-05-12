package ai

import (
	"testing"
	"time"
)

// TestSweepWillFixLaterReminders_PromotesPastDeadline verifies the sweep
// converges an overdue will_fix_later finding onto the same in-memory
// state the Add() re-detection path produces when the deadline passes:
// dismissal cleared, RemindAt cleared, RemindCount bumped, LoopState
// back to detected, and a "reminded" lifecycle event recorded. Future-
// dated commitments must stay untouched.
func TestSweepWillFixLaterReminders_PromotesPastDeadline(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()
	past := now.Add(-30 * time.Minute)
	future := now.Add(2 * time.Hour)

	overdue := &Finding{
		ID:              "overdue-1",
		Severity:        FindingSeverityWarning,
		Category:        FindingCategoryCapacity,
		ResourceID:      "node1-storage",
		ResourceName:    "tank",
		ResourceType:    "storage",
		Title:           "Storage growth trending toward full",
		DetectedAt:      now.Add(-3 * time.Hour),
		LastSeenAt:      now.Add(-3 * time.Hour),
		DismissedReason: DismissReasonWillFixLater,
		UserNote:        "Will run cleanup next maintenance window",
		LoopState:       string(FindingLoopStateDismissed),
		RemindAt:        &past,
		RemindCount:     0,
	}
	pending := &Finding{
		ID:              "pending-1",
		Severity:        FindingSeverityInfo,
		Category:        FindingCategoryPerformance,
		ResourceID:      "node1-vm-200",
		ResourceName:    "appserver",
		ResourceType:    "vm",
		Title:           "Elevated CPU on appserver",
		DetectedAt:      now.Add(-1 * time.Hour),
		LastSeenAt:      now.Add(-1 * time.Hour),
		DismissedReason: DismissReasonWillFixLater,
		LoopState:       string(FindingLoopStateDismissed),
		RemindAt:        &future,
		RemindCount:     0,
	}

	store.findings[overdue.ID] = overdue
	store.findings[pending.ID] = pending

	swept := store.SweepWillFixLaterReminders(now)
	if swept != 1 {
		t.Fatalf("swept count: got %d, want 1", swept)
	}

	got := store.findings[overdue.ID]
	if got.DismissedReason != "" {
		t.Errorf("DismissedReason: got %q, want \"\"", got.DismissedReason)
	}
	if got.RemindAt != nil {
		t.Errorf("RemindAt: got %v, want nil", got.RemindAt)
	}
	if got.RemindCount != 1 {
		t.Errorf("RemindCount: got %d, want 1", got.RemindCount)
	}
	if got.LoopState != string(FindingLoopStateDetected) {
		t.Errorf("LoopState: got %q, want %q", got.LoopState, FindingLoopStateDetected)
	}
	if got.UserNote != "Will run cleanup next maintenance window" {
		t.Errorf("UserNote should be preserved on remind wake, got %q", got.UserNote)
	}
	if len(got.Lifecycle) == 0 {
		t.Fatal("expected a lifecycle event to be appended")
	}
	last := got.Lifecycle[len(got.Lifecycle)-1]
	if last.Type != "reminded" {
		t.Errorf("last lifecycle event Type: got %q, want \"reminded\"", last.Type)
	}
	if last.From != string(FindingLoopStateDismissed) || last.To != string(FindingLoopStateDetected) {
		t.Errorf("lifecycle transition: from=%q to=%q, want dismissed->detected", last.From, last.To)
	}
	if last.Metadata["trigger"] != "sweep" {
		t.Errorf("lifecycle metadata trigger: got %q, want \"sweep\"", last.Metadata["trigger"])
	}

	pendingAfter := store.findings[pending.ID]
	if pendingAfter.DismissedReason != DismissReasonWillFixLater {
		t.Errorf("pending finding should still be dismissed, got %q", pendingAfter.DismissedReason)
	}
	if pendingAfter.RemindAt == nil || !pendingAfter.RemindAt.Equal(future) {
		t.Errorf("pending RemindAt should be untouched, got %v", pendingAfter.RemindAt)
	}
	if pendingAfter.RemindCount != 0 {
		t.Errorf("pending RemindCount should be 0, got %d", pendingAfter.RemindCount)
	}
	if len(pendingAfter.Lifecycle) != 0 {
		t.Errorf("pending finding should not have a new lifecycle event, got %d", len(pendingAfter.Lifecycle))
	}
}
