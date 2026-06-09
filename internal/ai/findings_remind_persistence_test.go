package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestFindingsPersistenceAdapter_PreservesRemindAt locks the will_fix_later
// commitment loop's persistence: RemindAt and RemindCount must survive a
// save/load cycle. Before this guard, RemindAt was dropped on the floor at
// persistence time, so every restart silently forgot every operator
// "fix later" promise and the deadline never re-surfaced the finding.
func TestFindingsPersistenceAdapter_PreservesRemindAt(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	now := time.Now().UTC().Truncate(time.Second)
	remindAt := now.Add(72 * time.Hour)

	original := &Finding{
		ID:              "finding-will-fix-1",
		Key:             "storage-growth-high",
		Severity:        FindingSeverityWarning,
		Category:        FindingCategoryCapacity,
		ResourceID:      "node1-storage",
		ResourceName:    "tank",
		ResourceType:    "storage",
		Node:            "node1",
		Title:           "Storage growth trending toward full",
		Description:     "Pool tank approaching threshold",
		Source:          "ai-analysis",
		DetectedAt:      now.Add(-2 * time.Hour),
		LastSeenAt:      now,
		DismissedReason: DismissReasonWillFixLater,
		UserNote:        "Will run cleanup playbook next maintenance window",
		TimesRaised:     1,
		RemindAt:        &remindAt,
		RemindCount:     2,
	}

	if err := adapter.SaveFindings(map[string]*Finding{original.ID: original}); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := adapter.LoadFindings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := loaded[original.ID]
	if got == nil {
		t.Fatalf("finding %q not found after load", original.ID)
	}
	if got.DismissedReason != DismissReasonWillFixLater {
		t.Errorf("DismissedReason: got %q, want %q", got.DismissedReason, DismissReasonWillFixLater)
	}
	if got.RemindAt == nil {
		t.Fatal("RemindAt was dropped on persistence roundtrip")
	}
	if !got.RemindAt.Equal(remindAt) {
		t.Errorf("RemindAt mismatch: got %v, want %v", got.RemindAt, remindAt)
	}
	if got.RemindCount != 2 {
		t.Errorf("RemindCount mismatch: got %d, want 2", got.RemindCount)
	}
}
