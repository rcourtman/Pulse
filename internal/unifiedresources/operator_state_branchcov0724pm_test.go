package unifiedresources

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// branchcov0724pmBareStore satisfies the ResourceStore interface (by
// embedding it) but deliberately does NOT implement the unexported
// resourceOperatorStateLifecycleStore interface — its two methods are not
// part of ResourceStore. The free lifecycle adapters must therefore refuse
// it via the !ok type-assertion arm. No method is ever invoked on a value of
// this type, so the nil embedded interface never panics.
type branchcov0724pmBareStore struct {
	ResourceStore
}

// TestBranchcov0724pmSetOperatorStateLifecycleAdapter covers the free
// SetResourceOperatorStateWithMaintenanceLifecycle adapter: the delegation
// arm (store implements the lifecycle interface) and the refusal arm
// (store does not).
func TestBranchcov0724pmSetOperatorStateLifecycleAdapter(t *testing.T) {
	t.Run("delegates to store with atomic lifecycle support", func(t *testing.T) {
		store := NewMemoryStore()
		start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		end := start.Add(2 * time.Hour)
		setAt := start.Add(-time.Hour)
		state := ResourceOperatorState{
			CanonicalID:        " vm:101 ",
			MaintenanceStartAt: timePointer(start),
			MaintenanceEndAt:   timePointer(end),
			MaintenanceReason:  " kernel upgrade ",
			SetAt:              setAt,
			SetBy:              " operator:richard ",
		}

		got, err := SetResourceOperatorStateWithMaintenanceLifecycle(store, state)
		if err != nil {
			t.Fatalf("delegated set failed: %v", err)
		}
		// The store normalizes the row before persisting.
		if got.CanonicalID != "vm:101" {
			t.Errorf("canonical id = %q want vm:101 (trimmed)", got.CanonicalID)
		}
		if got.MaintenanceReason != "kernel upgrade" {
			t.Errorf("reason = %q want kernel upgrade (trimmed)", got.MaintenanceReason)
		}
		if got.SetBy != "operator:richard" {
			t.Errorf("setBy = %q want operator:richard (trimmed)", got.SetBy)
		}

		// The persisted row is observable through the plain GET path.
		persisted, found, err := store.GetResourceOperatorState("vm:101")
		if err != nil || !found {
			t.Fatalf("expected persisted entry after delegated set: found=%v err=%v", found, err)
		}
		if persisted.MaintenanceStartAt == nil || !persisted.MaintenanceStartAt.Equal(start) {
			t.Errorf("window start did not round-trip; got %v", persisted.MaintenanceStartAt)
		}
		if persisted.MaintenanceEndAt == nil || !persisted.MaintenanceEndAt.Equal(end) {
			t.Errorf("window end did not round-trip; got %v", persisted.MaintenanceEndAt)
		}

		// A scheduled maintenance-window lifecycle change is projected.
		changes, err := store.GetRecentChanges("vm:101", time.Time{}, 10)
		if err != nil {
			t.Fatalf("get changes: %v", err)
		}
		if len(changes) != 1 {
			t.Fatalf("expected exactly 1 lifecycle change, got %d", len(changes))
		}
		if changes[0].Metadata["activityType"] != MaintenanceWindowLifecycleEventScheduled {
			t.Errorf("activityType = %#v want %q", changes[0].Metadata["activityType"], MaintenanceWindowLifecycleEventScheduled)
		}
	})

	t.Run("refuses store lacking atomic lifecycle support", func(t *testing.T) {
		bare := branchcov0724pmBareStore{}
		_, err := SetResourceOperatorStateWithMaintenanceLifecycle(bare, ResourceOperatorState{CanonicalID: "vm:101"})
		if err == nil {
			t.Fatal("expected an error when the store lacks lifecycle support")
		}
		if !strings.Contains(err.Error(), "atomic store support") {
			t.Errorf("error should mention atomic store support; got %v", err)
		}
	})
}

// TestBranchcov0724pmClearOperatorStateLifecycleAdapter covers the free
// ClearResourceOperatorStateWithMaintenanceLifecycle adapter: the delegation
// arm and the refusal arm.
func TestBranchcov0724pmClearOperatorStateLifecycleAdapter(t *testing.T) {
	t.Run("delegates to store with atomic lifecycle support", func(t *testing.T) {
		store := NewMemoryStore()
		start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		end := start.Add(2 * time.Hour)
		setAt := start.Add(-time.Hour)
		if _, err := SetResourceOperatorStateWithMaintenanceLifecycle(store, ResourceOperatorState{
			CanonicalID:        "vm:101",
			MaintenanceStartAt: timePointer(start),
			MaintenanceEndAt:   timePointer(end),
			SetAt:              setAt,
			SetBy:              "operator:richard",
		}); err != nil {
			t.Fatalf("seed set failed: %v", err)
		}

		observed := end.Add(time.Minute)
		if err := ClearResourceOperatorStateWithMaintenanceLifecycle(store, "vm:101", observed, "operator:richard"); err != nil {
			t.Fatalf("delegated clear failed: %v", err)
		}
		if _, found, _ := store.GetResourceOperatorState("vm:101"); found {
			t.Error("entry must be removed after delegated clear")
		}

		// The clear projected a cleared lifecycle change.
		changes, err := store.GetRecentChanges("vm:101", time.Time{}, 10)
		if err != nil {
			t.Fatalf("get changes: %v", err)
		}
		var cleared bool
		for _, c := range changes {
			if c.Metadata["activityType"] == MaintenanceWindowLifecycleEventCleared {
				cleared = true
				break
			}
		}
		if !cleared {
			t.Errorf("expected a cleared lifecycle change; got %d changes", len(changes))
		}
	})

	t.Run("refuses store lacking atomic lifecycle support", func(t *testing.T) {
		bare := branchcov0724pmBareStore{}
		err := ClearResourceOperatorStateWithMaintenanceLifecycle(bare, "vm:101", time.Now(), "x")
		if err == nil {
			t.Fatal("expected an error when the store lacks lifecycle support")
		}
		if !strings.Contains(err.Error(), "atomic store support") {
			t.Errorf("error should mention atomic store support; got %v", err)
		}
	})
}

// TestBranchcov0724pmMemorystoreSetOperatorStateLifecycle covers every branch
// of MemoryStore.SetResourceOperatorStateWithMaintenanceLifecycle:
//   - validation failure (returns error, nothing persisted)
//   - nil operator-state map initialization on a zero-value MemoryStore
//   - new entry WITH a maintenance window -> scheduled lifecycle change
//   - existing entry updated to a different window -> updated lifecycle change
//   - entry WITHOUT a maintenance window -> no lifecycle change projected
func TestBranchcov0724pmMemorystoreSetOperatorStateLifecycle(t *testing.T) {
	t.Run("rejects invalid state before persisting", func(t *testing.T) {
		store := NewMemoryStore()
		// Empty canonical id (after trim) is invalid.
		_, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{CanonicalID: "   "})
		if !errors.Is(err, ErrResourceOperatorStateInvalid) {
			t.Fatalf("expected ErrResourceOperatorStateInvalid, got %v", err)
		}
		// A half-set maintenance window is also invalid.
		start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		_, err = store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{
			CanonicalID:        "vm:101",
			MaintenanceStartAt: timePointer(start),
		})
		if !errors.Is(err, ErrResourceOperatorStateInvalid) {
			t.Fatalf("expected ErrResourceOperatorStateInvalid for half-set window, got %v", err)
		}
		if _, found, _ := store.GetResourceOperatorState("vm:101"); found {
			t.Error("invalid state must not be persisted")
		}
	})

	t.Run("initializes nil map on zero value MemoryStore", func(t *testing.T) {
		// &MemoryStore{} has a nil resourceOperatorState map and a usable
		// zero-value mutex; the Set path must lazily initialize the map.
		store := &MemoryStore{}
		start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		end := start.Add(time.Hour)
		got, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{
			CanonicalID:        "vm:202",
			MaintenanceStartAt: timePointer(start),
			MaintenanceEndAt:   timePointer(end),
			SetAt:              start.Add(-time.Hour),
			SetBy:              "operator:alice",
		})
		if err != nil {
			t.Fatalf("set on zero-value store failed: %v", err)
		}
		if got.CanonicalID != "vm:202" {
			t.Errorf("canonical id = %q want vm:202", got.CanonicalID)
		}
		persisted, found, err := store.GetResourceOperatorState("vm:202")
		if err != nil || !found {
			t.Fatalf("entry not persisted after nil-map init: found=%v err=%v", found, err)
		}
		if persisted.MaintenanceStartAt == nil || !persisted.MaintenanceStartAt.Equal(start) {
			t.Errorf("window start did not round-trip; got %v", persisted.MaintenanceStartAt)
		}
	})

	t.Run("schedules and updates maintenance window with lifecycle side effects", func(t *testing.T) {
		store := NewMemoryStore()
		start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		end := start.Add(2 * time.Hour)
		setAt1 := start.Add(-time.Hour)

		first := ResourceOperatorState{
			CanonicalID:        "vm:303",
			MaintenanceStartAt: timePointer(start),
			MaintenanceEndAt:   timePointer(end),
			MaintenanceReason:  "kernel patch",
			SetAt:              setAt1,
			SetBy:              "operator:richard",
		}
		if _, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(first); err != nil {
			t.Fatalf("first set failed: %v", err)
		}

		// Scheduled lifecycle side effects on the projected change.
		changes := mustRecentChanges(t, store, "vm:303")
		if len(changes) != 1 {
			t.Fatalf("expected 1 change after schedule, got %d", len(changes))
		}
		scheduled := changes[0]
		if scheduled.Metadata["activityType"] != MaintenanceWindowLifecycleEventScheduled {
			t.Errorf("activityType = %#v want %q", scheduled.Metadata["activityType"], MaintenanceWindowLifecycleEventScheduled)
		}
		if scheduled.ResourceID != "vm:303" {
			t.Errorf("resource id = %q want vm:303", scheduled.ResourceID)
		}
		if scheduled.Actor != "operator:richard" {
			t.Errorf("actor = %q want operator:richard", scheduled.Actor)
		}
		// observedAt derives from state.SetAt (non-zero -> UTC).
		if !scheduled.ObservedAt.Equal(setAt1) {
			t.Errorf("observedAt = %v want %v", scheduled.ObservedAt, setAt1)
		}
		if scheduled.Metadata["maintenanceStartAt"] != start.Format(time.RFC3339) {
			t.Errorf("maintenanceStartAt metadata = %#v want %q", scheduled.Metadata["maintenanceStartAt"], start.Format(time.RFC3339))
		}
		if scheduled.Metadata["maintenanceEndAt"] != end.Format(time.RFC3339) {
			t.Errorf("maintenanceEndAt metadata = %#v want %q", scheduled.Metadata["maintenanceEndAt"], end.Format(time.RFC3339))
		}
		if scheduled.Metadata["maintenanceReason"] != "kernel patch" {
			t.Errorf("maintenanceReason metadata = %#v want kernel patch", scheduled.Metadata["maintenanceReason"])
		}
		if scheduled.From != "no maintenance window" {
			t.Errorf("from = %q want no maintenance window", scheduled.From)
		}
		if !strings.Contains(scheduled.To, "kernel patch") {
			t.Errorf("to = %q should contain the reason", scheduled.To)
		}

		// Updating the window end projects an updated lifecycle change.
		newEnd := end.Add(time.Hour)
		setAt2 := setAt1.Add(time.Minute)
		second := first
		second.MaintenanceEndAt = timePointer(newEnd)
		second.SetAt = setAt2
		if _, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(second); err != nil {
			t.Fatalf("second set failed: %v", err)
		}
		changes = mustRecentChanges(t, store, "vm:303")
		if len(changes) != 2 {
			t.Fatalf("expected 2 changes after update, got %d", len(changes))
		}
		// GetRecentChanges returns newest-first.
		updated := changes[0]
		if updated.Metadata["activityType"] != MaintenanceWindowLifecycleEventUpdated {
			t.Errorf("activityType = %#v want %q", updated.Metadata["activityType"], MaintenanceWindowLifecycleEventUpdated)
		}
		if updated.Metadata["previousMaintenanceEndAt"] != end.Format(time.RFC3339) {
			t.Errorf("previousMaintenanceEndAt metadata = %#v want %q", updated.Metadata["previousMaintenanceEndAt"], end.Format(time.RFC3339))
		}
		if updated.Metadata["maintenanceEndAt"] != newEnd.Format(time.RFC3339) {
			t.Errorf("maintenanceEndAt metadata = %#v want %q", updated.Metadata["maintenanceEndAt"], newEnd.Format(time.RFC3339))
		}
	})

	t.Run("state without a maintenance window projects no lifecycle change", func(t *testing.T) {
		store := NewMemoryStore()
		setAt := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
		// A note-only state carries operator intent but no window.
		got, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{
			CanonicalID: "vm:404",
			Note:        "archived",
			Criticality: CriticalityHigh,
			SetAt:       setAt,
			SetBy:       "operator:bob",
		})
		if err != nil {
			t.Fatalf("set failed: %v", err)
		}
		if got.Note != "archived" || got.Criticality != CriticalityHigh {
			t.Errorf("non-window fields must still round-trip: %+v", got)
		}
		// Persisted, but no timeline change recorded.
		if _, found, _ := store.GetResourceOperatorState("vm:404"); !found {
			t.Error("non-window state must still be persisted")
		}
		if changes, err := store.GetRecentChanges("vm:404", time.Time{}, 10); err != nil {
			t.Fatalf("get changes: %v", err)
		} else if len(changes) != 0 {
			t.Errorf("expected 0 lifecycle changes for windowless state, got %d", len(changes))
		}
	})
}

// TestBranchcov0724pmMemorystoreClearOperatorStateLifecycle covers every
// branch of MemoryStore.ClearResourceOperatorStateWithMaintenanceLifecycle:
//   - empty canonical id -> idempotent no-op (entry is NOT removed)
//   - clear an entry WITH a maintenance window -> cleared lifecycle change
//   - clear an entry WITHOUT a maintenance window -> no lifecycle change
//   - clear a non-existent entry -> idempotent nil, no lifecycle change
func TestBranchcov0724pmMemorystoreClearOperatorStateLifecycle(t *testing.T) {
	t.Run("empty canonical id is a no-op", func(t *testing.T) {
		store := NewMemoryStore()
		if _, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{
			CanonicalID: "vm:505",
			Note:        "keep me",
			SetAt:       time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC),
			SetBy:       "operator:alice",
		}); err != nil {
			t.Fatalf("seed set failed: %v", err)
		}
		// Whitespace-only canonical id trims to empty and is ignored.
		if err := store.ClearResourceOperatorStateWithMaintenanceLifecycle("   ", time.Now(), "x"); err != nil {
			t.Fatalf("empty-id clear must succeed: %v", err)
		}
		// The real entry is untouched.
		if state, found, _ := store.GetResourceOperatorState("vm:505"); !found {
			t.Error("entry must remain after empty-id clear")
		} else if state.Note != "keep me" {
			t.Errorf("entry note = %q want keep me", state.Note)
		}
	})

	t.Run("clear entry with window projects cleared change", func(t *testing.T) {
		store := NewMemoryStore()
		start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		end := start.Add(2 * time.Hour)
		setAt := start.Add(-time.Hour)
		if _, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{
			CanonicalID:        "vm:606",
			MaintenanceStartAt: timePointer(start),
			MaintenanceEndAt:   timePointer(end),
			MaintenanceReason:  "firmware flash",
			SetAt:              setAt,
			SetBy:              "operator:richard",
		}); err != nil {
			t.Fatalf("seed set failed: %v", err)
		}

		observed := end.Add(time.Minute)
		if err := store.ClearResourceOperatorStateWithMaintenanceLifecycle("vm:606", observed, "operator:carol"); err != nil {
			t.Fatalf("clear failed: %v", err)
		}
		if _, found, _ := store.GetResourceOperatorState("vm:606"); found {
			t.Error("entry must be removed after clear")
		}

		// Newest-first: the cleared event comes before the scheduled one.
		changes := mustRecentChanges(t, store, "vm:606")
		if len(changes) != 2 {
			t.Fatalf("expected 2 changes (scheduled + cleared), got %d", len(changes))
		}
		cleared := changes[0]
		if cleared.Metadata["activityType"] != MaintenanceWindowLifecycleEventCleared {
			t.Errorf("activityType = %#v want %q", cleared.Metadata["activityType"], MaintenanceWindowLifecycleEventCleared)
		}
		if cleared.Actor != "operator:carol" {
			t.Errorf("actor = %q want operator:carol", cleared.Actor)
		}
		// observedAt is converted to UTC from the supplied non-zero time.
		if !cleared.ObservedAt.Equal(observed.UTC()) {
			t.Errorf("observedAt = %v want %v", cleared.ObservedAt, observed.UTC())
		}
		if cleared.To != "no maintenance window" {
			t.Errorf("to = %q want no maintenance window", cleared.To)
		}
		if cleared.Metadata["previousMaintenanceReason"] != "firmware flash" {
			t.Errorf("previousMaintenanceReason metadata = %#v want firmware flash", cleared.Metadata["previousMaintenanceReason"])
		}
	})

	t.Run("clear entry without window projects no change", func(t *testing.T) {
		store := NewMemoryStore()
		setAt := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
		if _, err := store.SetResourceOperatorStateWithMaintenanceLifecycle(ResourceOperatorState{
			CanonicalID: "vm:707",
			Note:        "no window here",
			SetAt:       setAt,
			SetBy:       "operator:dave",
		}); err != nil {
			t.Fatalf("seed set failed: %v", err)
		}
		// No lifecycle change existed before, so none should be projected now.
		beforeCount := len(mustRecentChanges(t, store, "vm:707"))
		if beforeCount != 0 {
			t.Fatalf("expected 0 changes before clear, got %d", beforeCount)
		}
		if err := store.ClearResourceOperatorStateWithMaintenanceLifecycle("vm:707", time.Now(), "operator:dave"); err != nil {
			t.Fatalf("clear failed: %v", err)
		}
		if _, found, _ := store.GetResourceOperatorState("vm:707"); found {
			t.Error("entry must still be removed even without a lifecycle change")
		}
		if afterCount := len(mustRecentChanges(t, store, "vm:707")); afterCount != 0 {
			t.Errorf("expected 0 lifecycle changes after windowless clear, got %d", afterCount)
		}
	})

	t.Run("clear non-existent entry is idempotent", func(t *testing.T) {
		store := NewMemoryStore()
		if err := store.ClearResourceOperatorStateWithMaintenanceLifecycle("vm:808", time.Now(), "operator:nobody"); err != nil {
			t.Fatalf("clearing a non-existent entry must not error: %v", err)
		}
		if changes, err := store.GetRecentChanges("vm:808", time.Time{}, 10); err != nil {
			t.Fatalf("get changes: %v", err)
		} else if len(changes) != 0 {
			t.Errorf("expected 0 changes after clearing non-existent entry, got %d", len(changes))
		}
	})
}

// TestBranchcov0724pmMemorystoreClose asserts the MemoryStore closer is a
// safe no-op returning nil.
func TestBranchcov0724pmMemorystoreClose(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Close(); err != nil {
		t.Errorf("Close must return nil; got %v", err)
	}
	// Close is idempotent and safe on a zero-value store too.
	if err := (&MemoryStore{}).Close(); err != nil {
		t.Errorf("Close on zero-value store must return nil; got %v", err)
	}
}

// mustRecentChanges is a small helper that fetches all lifecycle changes for a
// canonical id (no since-filter) and fails the test on store error.
func mustRecentChanges(t *testing.T, store *MemoryStore, canonicalID string) []ResourceChange {
	t.Helper()
	changes, err := store.GetRecentChanges(canonicalID, time.Time{}, 50)
	if err != nil {
		t.Fatalf("GetRecentChanges(%q): %v", canonicalID, err)
	}
	return changes
}
