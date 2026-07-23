package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newBranchcov0723pmLog constructs a RemediationLog through the package's own
// constructor, pointing persistence at a per-test temp directory so the
// background saveToDisk goroutine spawned by Log/MarkRolledBack writes somewhere
// harmless.
//
// Log and MarkRolledBack persist via a fire-and-forget goroutine. t.TempDir's
// auto-cleanup (RemoveAll) would otherwise race those goroutines and fail with
// "directory not empty". We therefore register a Cleanup that runs *before*
// TempDir's RemoveAll (cleanups are LIFO, and TempDir registered first) and
// waits for the persisted file to quiesce. This mirrors the package's existing
// poll-for-file convention (see TestIncidentStore_SaveAsyncAndPersistence).
func newBranchcov0723pmLog(t *testing.T) *RemediationLog {
	t.Helper()
	dataDir := t.TempDir()
	rl := NewRemediationLog(RemediationLogConfig{DataDir: dataDir})
	t.Cleanup(func() { waitForRemediationSaveQuiescence(t, dataDir) })
	return rl
}

// waitForRemediationSaveQuiescence blocks until the remediation history file in
// dataDir has stopped changing (no background save goroutine is still writing),
// or until the deadline expires. It never fails the test; its only job is to
// keep the temp directory stable long enough for RemoveAll to succeed.
func waitForRemediationSaveQuiescence(t *testing.T, dataDir string) {
	t.Helper()
	path := filepath.Join(dataDir, remediationHistoryFileName)
	deadline := time.Now().Add(2 * time.Second)
	var lastMod time.Time
	stableChecks := 0
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err == nil && info.ModTime().Equal(lastMod) {
			stableChecks++
			if stableChecks >= 3 { // ~30ms with no mutation: writes have drained
				return
			}
		} else if err == nil {
			lastMod = info.ModTime()
			stableChecks = 0
		} else {
			stableChecks = 0
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBranchcov0723pmGetByID(t *testing.T) {
	t.Run("HitReturnsRightRecord", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		want := RemediationRecord{
			ID:         "rec-hit",
			ResourceID: "vm-1",
			Problem:    "disk full",
			Action:     "rm -rf /tmp/x",
			Outcome:    OutcomeResolved,
			Timestamp:  time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		}
		if err := rl.Log(want); err != nil {
			t.Fatalf("Log: %v", err)
		}
		// Decoy to confirm the scan picks the right element, not just the only one.
		if err := rl.Log(RemediationRecord{ID: "rec-other", Problem: "p", Action: "a", Timestamp: time.Now()}); err != nil {
			t.Fatalf("Log decoy: %v", err)
		}

		got, ok := rl.GetByID("rec-hit")
		if !ok {
			t.Fatalf("expected ok=true for existing id")
		}
		if got == nil {
			t.Fatalf("expected non-nil record for existing id")
		}
		if got.ID != "rec-hit" {
			t.Fatalf("ID = %q, want %q", got.ID, "rec-hit")
		}
		if got.ResourceID != "vm-1" || got.Problem != "disk full" || got.Action != "rm -rf /tmp/x" || got.Outcome != OutcomeResolved {
			t.Fatalf("returned record = %+v, want the logged record", got)
		}
		if !got.Timestamp.Equal(want.Timestamp) {
			t.Fatalf("Timestamp = %v, want %v", got.Timestamp, want.Timestamp)
		}
	})

	t.Run("MissReturnsNilFalse", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		if err := rl.Log(RemediationRecord{ID: "rec-present", Problem: "p", Action: "a", Timestamp: time.Now()}); err != nil {
			t.Fatalf("Log: %v", err)
		}

		got, ok := rl.GetByID("does-not-exist")
		if ok {
			t.Fatalf("expected ok=false for unknown id, got true")
		}
		if got != nil {
			t.Fatalf("expected nil record for unknown id, got %+v", got)
		}
	})

	t.Run("EmptyStoreAndEmptyID", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		// Empty store: nothing to find.
		if got, ok := rl.GetByID("anything"); ok || got != nil {
			t.Fatalf("empty store: expected (nil,false), got (%+v,%v)", got, ok)
		}
		// Empty id against a store where no record carries an empty id -> miss.
		if err := rl.Log(RemediationRecord{ID: "has-id", Problem: "p", Action: "a", Timestamp: time.Now()}); err != nil {
			t.Fatalf("Log: %v", err)
		}
		if got, ok := rl.GetByID(""); ok || got != nil {
			t.Fatalf("empty id lookup: expected (nil,false), got (%+v,%v)", got, ok)
		}
	})
}

func TestBranchcov0723pmMarkRolledBack(t *testing.T) {
	t.Run("UnknownIDReturnsErrorAndLeavesRecordsUntouched", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		if err := rl.Log(RemediationRecord{ID: "rec-a", Problem: "p", Action: "a", Timestamp: time.Now()}); err != nil {
			t.Fatalf("Log: %v", err)
		}

		err := rl.MarkRolledBack("missing-id", "rb-1", "alice")
		if err == nil {
			t.Fatalf("expected error for unknown id, got nil")
		}
		if !strings.Contains(err.Error(), "missing-id") {
			t.Fatalf("expected error to mention id %q, got %q", "missing-id", err.Error())
		}

		// Existing record must be untouched on the error path.
		got, ok := rl.GetByID("rec-a")
		if !ok || got == nil {
			t.Fatalf("expected rec-a to still exist after failed rollback")
		}
		if got.Rollback != nil {
			t.Fatalf("expected rec-a Rollback to remain nil, got %+v", got.Rollback)
		}
	})

	t.Run("NilRollbackCreatesStructAndSetsEveryField", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		if err := rl.Log(RemediationRecord{ID: "rec-nil", Problem: "p", Action: "a", Timestamp: time.Now()}); err != nil {
			t.Fatalf("Log: %v", err)
		}
		// Precondition: Rollback is nil before the call.
		if pre, _ := rl.GetByID("rec-nil"); pre.Rollback != nil {
			t.Fatalf("precondition: expected nil Rollback, got %+v", pre.Rollback)
		}

		before := time.Now()
		if err := rl.MarkRolledBack("rec-nil", "rb-99", "bob"); err != nil {
			t.Fatalf("MarkRolledBack: %v", err)
		}
		after := time.Now()

		got, ok := rl.GetByID("rec-nil")
		if !ok || got == nil {
			t.Fatalf("expected rec-nil to exist after rollback")
		}
		rb := got.Rollback
		if rb == nil {
			t.Fatalf("expected Rollback struct to be created")
		}
		// Fields the function assigns.
		if !rb.RolledBack {
			t.Errorf("expected RolledBack=true, got false")
		}
		if rb.RolledBackBy != "bob" {
			t.Errorf("expected RolledBackBy=%q, got %q", "bob", rb.RolledBackBy)
		}
		if rb.RollbackID != "rb-99" {
			t.Errorf("expected RollbackID=%q, got %q", "rb-99", rb.RollbackID)
		}
		if rb.RolledBackAt == nil {
			t.Fatalf("expected RolledBackAt to be set")
		}
		if rb.RolledBackAt.Before(before) || rb.RolledBackAt.After(after) {
			t.Errorf("RolledBackAt=%v not within [%v,%v]", rb.RolledBackAt, before, after)
		}
		// Fields the function does NOT touch: must remain at the zero value of
		// a freshly-created RollbackInfo{}.
		if rb.Reversible {
			t.Errorf("expected Reversible to remain false on newly-created struct, got true")
		}
		if rb.RollbackCmd != "" {
			t.Errorf("expected RollbackCmd to remain empty, got %q", rb.RollbackCmd)
		}
		if rb.PreState != "" {
			t.Errorf("expected PreState to remain empty, got %q", rb.PreState)
		}
	})

	t.Run("ExistingRollbackOverwritesTrackedFieldsPreservesRest", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := rl.Log(RemediationRecord{
			ID:        "rec-existing",
			Problem:   "p",
			Action:    "a",
			Timestamp: time.Now(),
			Rollback: &RollbackInfo{
				Reversible:   true,
				RollbackCmd:  "undo-cmd",
				PreState:     `{"cpu":4}`,
				RolledBack:   false,
				RolledBackAt: &oldTime,
				RolledBackBy: "prev-user",
				RollbackID:   "prev-rb",
			},
		}); err != nil {
			t.Fatalf("Log: %v", err)
		}

		before := time.Now()
		if err := rl.MarkRolledBack("rec-existing", "new-rb", "carol"); err != nil {
			t.Fatalf("MarkRolledBack: %v", err)
		}
		after := time.Now()

		got, ok := rl.GetByID("rec-existing")
		if !ok || got == nil {
			t.Fatalf("expected rec-existing to exist after rollback")
		}
		rb := got.Rollback
		if rb == nil {
			t.Fatalf("expected Rollback to still be present")
		}
		// Overwritten tracked fields.
		if !rb.RolledBack {
			t.Errorf("expected RolledBack overwritten to true, got false")
		}
		if rb.RolledBackBy != "carol" {
			t.Errorf("expected RolledBackBy overwritten to %q, got %q", "carol", rb.RolledBackBy)
		}
		if rb.RollbackID != "new-rb" {
			t.Errorf("expected RollbackID overwritten to %q, got %q", "new-rb", rb.RollbackID)
		}
		if rb.RolledBackAt == nil {
			t.Fatalf("expected RolledBackAt overwritten to a new time")
		}
		if rb.RolledBackAt.Equal(oldTime) {
			t.Errorf("expected RolledBackAt to be replaced, still equals oldTime")
		}
		if rb.RolledBackAt.Before(before) || rb.RolledBackAt.After(after) {
			t.Errorf("RolledBackAt=%v not within [%v,%v]", rb.RolledBackAt, before, after)
		}
		// Preserved fields (not touched by MarkRolledBack).
		if !rb.Reversible {
			t.Errorf("expected Reversible preserved as true, got false")
		}
		if rb.RollbackCmd != "undo-cmd" {
			t.Errorf("expected RollbackCmd preserved as %q, got %q", "undo-cmd", rb.RollbackCmd)
		}
		if rb.PreState != `{"cpu":4}` {
			t.Errorf("expected PreState preserved, got %q", rb.PreState)
		}
	})
}

func TestBranchcov0723pmGetRollbackable(t *testing.T) {
	// eligibleRecord builds a record that satisfies GetRollbackable's
	// eligibility condition: Rollback != nil, Reversible, not rolled back,
	// and not itself a rollback record.
	eligibleRecord := func(id string, ts time.Time) RemediationRecord {
		return RemediationRecord{
			ID:        id,
			Problem:   "p",
			Action:    "a",
			Timestamp: ts,
			Rollback:  &RollbackInfo{Reversible: true},
		}
	}

	t.Run("EmptyStoreAndNonPositiveLimit", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		// Empty store: loop body never executes.
		if got := rl.GetRollbackable(5); len(got) != 0 {
			t.Fatalf("empty store: expected 0 results, got %d", len(got))
		}
		// limit <= 0 makes the loop guard `len(result) < limit` false on the
		// first evaluation, so even an eligible record is excluded.
		if err := rl.Log(eligibleRecord("r1", time.Now())); err != nil {
			t.Fatalf("Log: %v", err)
		}
		if got := rl.GetRollbackable(0); len(got) != 0 {
			t.Fatalf("limit=0: expected 0 results, got %d", len(got))
		}
		if got := rl.GetRollbackable(-3); len(got) != 0 {
			t.Fatalf("limit=-3: expected 0 results, got %d", len(got))
		}
	})

	t.Run("FewerThanLimitReturnsAllNewestFirst", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		base := time.Now()
		// Insert oldest -> newest (slice order, which is what the reverse
		// iteration walks).
		ids := []string{"old", "mid", "new"}
		for i, id := range ids {
			if err := rl.Log(eligibleRecord(id, base.Add(time.Duration(i)*time.Hour))); err != nil {
				t.Fatalf("Log %s: %v", id, err)
			}
		}

		got := rl.GetRollbackable(10)
		if len(got) != 3 {
			t.Fatalf("expected all 3 eligible records, got %d", len(got))
		}
		// Ordering is reverse slice order: newest first.
		wantOrder := []string{"new", "mid", "old"}
		for i, w := range wantOrder {
			if got[i].ID != w {
				t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, w)
			}
		}
	})

	t.Run("LimitHonouredAndOrdered", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		base := time.Now()
		ids := []string{"e1", "e2", "e3", "e4", "e5"}
		for i, id := range ids {
			if err := rl.Log(eligibleRecord(id, base.Add(time.Duration(i)*time.Hour))); err != nil {
				t.Fatalf("Log %s: %v", id, err)
			}
		}

		got := rl.GetRollbackable(3)
		if len(got) != 3 {
			t.Fatalf("expected limit honoured at 3, got %d", len(got))
		}
		// The 3 most recent, newest first.
		wantOrder := []string{"e5", "e4", "e3"}
		for i, w := range wantOrder {
			if got[i].ID != w {
				t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, w)
			}
		}
	})

	t.Run("NoneEligibleAcrossEveryIneligibilityReason", func(t *testing.T) {
		rl := newBranchcov0723pmLog(t)
		ts := time.Now()
		// One record per falsy arm of the eligibility condition.
		records := []RemediationRecord{
			{ID: "nil-rollback", Timestamp: ts, Rollback: nil},                                                 // Rollback == nil
			{ID: "not-reversible", Timestamp: ts, Rollback: &RollbackInfo{Reversible: false}},                  // !Reversible
			{ID: "already-rolled", Timestamp: ts, Rollback: &RollbackInfo{Reversible: true, RolledBack: true}}, // RolledBack
			{ID: "is-rollback", Timestamp: ts, Rollback: &RollbackInfo{Reversible: true}, IsRollback: true},    // IsRollback
		}
		for _, rec := range records {
			if err := rl.Log(rec); err != nil {
				t.Fatalf("Log %s: %v", rec.ID, err)
			}
		}

		if got := rl.GetRollbackable(10); len(got) != 0 {
			t.Fatalf("expected 0 eligible across all ineligibility reasons, got %d: %+v", len(got), got)
		}

		// Adding a single eligible record must surface only it.
		if err := rl.Log(eligibleRecord("the-eligible", ts)); err != nil {
			t.Fatalf("Log eligible: %v", err)
		}
		got := rl.GetRollbackable(10)
		if len(got) != 1 {
			t.Fatalf("expected exactly 1 eligible after adding one, got %d", len(got))
		}
		if got[0].ID != "the-eligible" {
			t.Errorf("expected only the-eligible, got %q", got[0].ID)
		}
	})
}
