package investigation

import (
	"testing"
	"time"
)

func TestStore_CreateGetAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	session := store.Create("finding-1", "session-1")
	if session == nil || session.ID == "" {
		t.Fatalf("expected session to be created")
	}

	retrieved := store.Get(session.ID)
	if retrieved == nil || retrieved.ID != session.ID {
		t.Fatalf("expected to retrieve session")
	}

	store.sessions[session.ID].ProposedFix = &Fix{ID: "fix-1"}
	copy := store.Get(session.ID)
	if copy.ProposedFix == nil || copy.ProposedFix.ID != "fix-1" {
		t.Fatalf("expected fix to be copied")
	}
	if copy.ProposedFix == store.sessions[session.ID].ProposedFix {
		t.Fatalf("expected fix to be deep copied")
	}

	if err := store.ForceSave(); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	loaded := NewStore(dir)
	if err := loaded.LoadFromDisk(); err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if loaded.Get(session.ID) == nil {
		t.Fatalf("expected loaded session")
	}
}

func TestStore_ByFindingAndLatest(t *testing.T) {
	store := NewStore("")

	first := store.Create("finding-1", "session-1")
	second := store.Create("finding-1", "session-2")

	store.sessions[first.ID].StartedAt = time.Now().Add(-time.Hour)
	store.sessions[second.ID].StartedAt = time.Now()

	sessions := store.GetByFinding("finding-1")
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	latest := store.GetLatestByFinding("finding-1")
	if latest == nil || latest.ID != second.ID {
		t.Fatalf("expected latest session")
	}
}

func TestStore_RunningCounts(t *testing.T) {
	store := NewStore("")
	first := store.Create("finding-1", "session-1")
	second := store.Create("finding-2", "session-2")

	store.UpdateStatus(first.ID, StatusRunning)
	store.UpdateStatus(second.ID, StatusRunning)

	if store.CountRunning() != 2 {
		t.Fatalf("expected 2 running sessions")
	}
	if len(store.GetRunning()) != 2 {
		t.Fatalf("expected running sessions")
	}
}

func TestStore_UpdateAndStatus(t *testing.T) {
	store := NewStore("")
	session := store.Create("finding-1", "session-1")
	session.Status = StatusRunning
	session.ProposedFix = &Fix{ID: "fix-1"}

	if !store.Update(session) {
		t.Fatalf("expected update to succeed")
	}
	retrieved := store.Get(session.ID)
	if retrieved.Status != StatusRunning {
		t.Fatalf("expected status update")
	}
	if retrieved.ProposedFix == nil || retrieved.ProposedFix.ID != "fix-1" {
		t.Fatalf("expected fix update")
	}
	if !store.UpdateStatus(session.ID, StatusCompleted) {
		t.Fatalf("expected status update")
	}
}

func TestStore_CompleteFailAndCounts(t *testing.T) {
	store := NewStore("")
	session := store.Create("finding-1", "session-1")

	if !store.Complete(session.ID, OutcomeFixExecuted, "summary", &Fix{ID: "fix-1"}) {
		t.Fatalf("expected complete")
	}
	updated := store.Get(session.ID)
	if updated.Outcome != OutcomeFixExecuted || updated.Status != StatusCompleted {
		t.Fatalf("expected completed outcome")
	}

	session2 := store.Create("finding-2", "session-2")
	if !store.Fail(session2.ID, "error") {
		t.Fatalf("expected fail")
	}
	if store.CountFixed() != 1 {
		t.Fatalf("expected fixed count 1")
	}
}

func TestStore_IncrementAndApproval(t *testing.T) {
	store := NewStore("")
	session := store.Create("finding-1", "session-1")

	if count := store.IncrementTurnCount(session.ID); count != 1 {
		t.Fatalf("expected turn count 1")
	}
	if !store.SetApprovalID(session.ID, "approval-1") {
		t.Fatalf("expected approval id set")
	}
}

func TestStore_GetAllAndCleanup(t *testing.T) {
	store := NewStore("")
	session := store.Create("finding-1", "session-1")
	session2 := store.Create("finding-2", "session-2")

	all := store.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 sessions")
	}

	old := time.Now().Add(-2 * time.Hour)
	store.sessions[session.ID].CompletedAt = &old
	removed := store.Cleanup(time.Hour)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if store.Get(session.ID) != nil {
		t.Fatalf("expected session removed")
	}
	if store.Get(session2.ID) == nil {
		t.Fatalf("expected remaining session")
	}
}

func TestStore_ForceSave_NoDir(t *testing.T) {
	store := NewStore("")
	if err := store.ForceSave(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
