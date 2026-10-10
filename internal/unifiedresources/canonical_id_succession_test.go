package unifiedresources

import (
	"fmt"
	"testing"
	"time"
)

// ApplyCanonicalIDSuccessions must re-key operator state and action-audit
// history rows to the successor canonical ID, drop the superseded identity
// pin row, and never clobber rows already present under the successor.
func TestSQLiteApplyCanonicalIDSuccessions(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	defer store.Close()

	oldID := buildHashID(ResourceTypeAgent, "cluster:prod-swarm:cloud")
	newID := buildHashID(ResourceTypeAgent, "cluster:prod-swarm:cloud.a")

	if err := store.UpsertResourceIdentityPins([]ResourceIdentityPin{{
		CanonicalID:  oldID,
		ResourceType: ResourceTypeAgent,
		ClusterName:  "prod-swarm",
		Hostname:     "cloud",
	}}); err != nil {
		t.Fatalf("seed identity pin: %v", err)
	}
	if err := store.SetResourceOperatorState(ResourceOperatorState{
		CanonicalID:        oldID,
		NeverAutoRemediate: true,
		Note:               "flaky PSU, hands off",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}
	// Seed the audit history row directly: only the canonical_id index
	// column participates in succession, the audit artifact itself is
	// opaque here.
	now := time.Now().UTC()
	if _, err := store.db.Exec(`INSERT INTO action_audits
		(id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"audit-1", "audit-1", oldID, "req-1", now.Add(-time.Hour), now.Add(-time.Hour),
		string(ActionStatePending), `{"resourceId":"`+oldID+`"}`, `{}`,
	); err != nil {
		t.Fatalf("seed action audit: %v", err)
	}

	if err := store.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{
		OldCanonicalID: oldID,
		NewCanonicalID: newID,
	}}); err != nil {
		t.Fatalf("ApplyCanonicalIDSuccessions: %v", err)
	}

	if _, found, err := store.GetResourceOperatorState(oldID); err != nil || found {
		t.Fatalf("operator state still keyed by superseded ID (found=%v, err=%v)", found, err)
	}
	state, found, err := store.GetResourceOperatorState(newID)
	if err != nil || !found {
		t.Fatalf("operator state missing under successor ID (found=%v, err=%v)", found, err)
	}
	if !state.NeverAutoRemediate || state.Note != "flaky PSU, hands off" {
		t.Fatalf("operator state mutated across succession: %+v", state)
	}

	oldAudits, err := store.GetActionAudits(oldID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits(old): %v", err)
	}
	if len(oldAudits) != 0 {
		t.Fatalf("action audits still keyed by superseded ID: %d rows", len(oldAudits))
	}
	newAudits, err := store.GetActionAudits(newID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits(new): %v", err)
	}
	if len(newAudits) != 1 || newAudits[0].ID != "audit-1" {
		t.Fatalf("action audit history missing under successor ID: %+v", newAudits)
	}

	pins, err := store.ListResourceIdentityPins()
	if err != nil {
		t.Fatalf("ListResourceIdentityPins: %v", err)
	}
	for _, pin := range pins {
		if pin.CanonicalID == oldID {
			t.Fatalf("superseded identity pin row still present")
		}
	}
}

// When the successor ID already has an operator-state row, the succession
// must keep it (it is fresher) rather than overwrite it with the superseded
// row.
func TestSQLiteCanonicalIDSuccessionKeepsExistingTargetRow(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	defer store.Close()

	oldID := buildHashID(ResourceTypeAgent, "cluster:prod-swarm:cloud")
	newID := buildHashID(ResourceTypeAgent, "cluster:prod-swarm:cloud.a")

	if err := store.SetResourceOperatorState(ResourceOperatorState{
		CanonicalID: oldID,
		Note:        "stale note from the collapsed era",
	}); err != nil {
		t.Fatalf("seed old operator state: %v", err)
	}
	if err := store.SetResourceOperatorState(ResourceOperatorState{
		CanonicalID:        newID,
		NeverAutoRemediate: true,
		Note:               "fresh note under the successor",
	}); err != nil {
		t.Fatalf("seed new operator state: %v", err)
	}

	if err := store.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{
		OldCanonicalID: oldID,
		NewCanonicalID: newID,
	}}); err != nil {
		t.Fatalf("ApplyCanonicalIDSuccessions: %v", err)
	}

	state, found, err := store.GetResourceOperatorState(newID)
	if err != nil || !found {
		t.Fatalf("successor operator state missing (found=%v, err=%v)", found, err)
	}
	if !state.NeverAutoRemediate || state.Note != "fresh note under the successor" {
		t.Fatalf("succession overwrote the fresher successor row: %+v", state)
	}
}

// The successor's row wins over the superseded one (it is fresher), but a
// remediation lock is the operator's "never touch this" and must not be shed
// by whichever row happened to be written second. The successor keeps every
// setting of its own and gains only the block.
func TestCanonicalIDSuccessionCarriesRemediationLockOntoExistingSuccessorRow(t *testing.T) {
	stores := map[string]func(t *testing.T) ResourceStore{
		"memory": func(t *testing.T) ResourceStore { return NewMemoryStore() },
		"sqlite": func(t *testing.T) ResourceStore {
			store, err := NewSQLiteResourceStore(t.TempDir(), "default")
			if err != nil {
				t.Fatalf("NewSQLiteResourceStore: %v", err)
			}
			t.Cleanup(func() { _ = store.Close() })
			return store
		},
	}
	locks := map[string]ResourceOperatorState{
		"never auto-remediate": {NeverAutoRemediate: true},
		"retired":              {LifecycleState: LifecycleStateRetired},
	}
	for storeName, newStore := range stores {
		for lockName, lock := range locks {
			t.Run(storeName+"/"+lockName, func(t *testing.T) {
				store := newStore(t)
				succeeder, ok := store.(canonicalIDSuccessor)
				if !ok {
					t.Fatalf("%T cannot apply canonical ID successions", store)
				}
				oldID := buildHashID(ResourceTypeAgent, "cluster:prod-swarm:vault")
				newID := buildHashID(ResourceTypeAgent, "cluster:prod-swarm:vault.a")
				now := time.Now().UTC()

				predecessor := lock
				predecessor.CanonicalID = oldID
				predecessor.SetAt = now.Add(-72 * time.Hour)
				predecessor.SetBy = "operator"
				if err := store.SetResourceOperatorState(predecessor); err != nil {
					t.Fatalf("seed predecessor: %v", err)
				}
				if err := store.SetResourceOperatorState(ResourceOperatorState{
					CanonicalID:    newID,
					MonitoringMode: MonitoringModeExpectedOffline,
					Criticality:    CriticalityHigh,
					Note:           "set under the new ID before the era change was declared",
					SetAt:          now.Add(-time.Hour),
					SetBy:          "operator",
				}); err != nil {
					t.Fatalf("seed successor: %v", err)
				}

				if err := succeeder.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{OldCanonicalID: oldID, NewCanonicalID: newID}}); err != nil {
					t.Fatalf("ApplyCanonicalIDSuccessions: %v", err)
				}

				got, found, err := store.GetResourceOperatorState(newID)
				if err != nil || !found {
					t.Fatalf("successor state missing: found=%v err=%v", found, err)
				}
				if !got.BlocksRemediation() {
					t.Fatalf("the successor's unlocked row shadowed the predecessor's lock: %+v", got)
				}
				if got.MonitoringMode != MonitoringModeExpectedOffline || got.Criticality != CriticalityHigh ||
					got.Note != "set under the new ID before the era change was declared" || got.LifecycleState != LifecycleStateActive {
					t.Fatalf("carrying the lock rewrote the successor's own settings: %+v", got)
				}

				// The block is the operator's to lift on the successor. Declaring
				// the same succession again (every steady-state rebuild does) must
				// not bring it back.
				got.NeverAutoRemediate = false
				if err := store.SetResourceOperatorState(got); err != nil {
					t.Fatalf("operator lifts the lock: %v", err)
				}
				if err := succeeder.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{OldCanonicalID: oldID, NewCanonicalID: newID}}); err != nil {
					t.Fatalf("ApplyCanonicalIDSuccessions (repeat): %v", err)
				}
				if again, _, err := store.GetResourceOperatorState(newID); err != nil || again.BlocksRemediation() {
					t.Fatalf("a repeated succession re-locked the successor the operator cleared: err=%v state=%+v", err, again)
				}

				// Clearing the row outright is the other way to lift it, and the
				// repeat must not move the shadowed predecessor's row back in.
				if err := store.ClearResourceOperatorState(newID); err != nil {
					t.Fatalf("operator clears the successor: %v", err)
				}
				if err := succeeder.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{OldCanonicalID: oldID, NewCanonicalID: newID}}); err != nil {
					t.Fatalf("ApplyCanonicalIDSuccessions (repeat after clear): %v", err)
				}
				if again, found, err := store.GetResourceOperatorState(newID); err != nil || found {
					t.Fatalf("a repeated succession restored a row the operator cleared: found=%v err=%v state=%+v", found, err, again)
				}
			})
		}
	}
}

// The registry keeps one member of a manual link, and a succession re-keys the
// link onto the successor. The successor's lock (whether it kept its own row
// or took the predecessor's whole row) has to reach the member that survives.
func TestCanonicalIDSuccessionLockReachesLinkedSurvivor(t *testing.T) {
	stores := map[string]func(t *testing.T) ResourceStore{
		"memory": func(t *testing.T) ResourceStore { return NewMemoryStore() },
		"sqlite": func(t *testing.T) ResourceStore {
			store, err := NewSQLiteResourceStore(t.TempDir(), "default")
			if err != nil {
				t.Fatalf("NewSQLiteResourceStore: %v", err)
			}
			t.Cleanup(func() { _ = store.Close() })
			return store
		},
	}
	for name, newStore := range stores {
		for _, successorHasRow := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/successor row %v", name, successorHasRow), func(t *testing.T) {
				store := newStore(t)
				succeeder, ok := store.(canonicalIDSuccessor)
				if !ok {
					t.Fatalf("%T cannot apply canonical ID successions", store)
				}
				const guestID = "vm-linked-survivor"
				oldID := buildHashID(ResourceTypeAgent, "linked:old-era")
				newID := buildHashID(ResourceTypeAgent, "linked:new-era")
				now := time.Now().UTC()

				// The link names the superseded ID, so the succession re-keys it.
				if err := store.AddLink(ResourceLink{ResourceA: guestID, ResourceB: oldID, PrimaryID: guestID}); err != nil {
					t.Fatalf("link: %v", err)
				}
				if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: oldID, NeverAutoRemediate: true, SetAt: now}); err != nil {
					t.Fatalf("lock predecessor: %v", err)
				}
				if successorHasRow {
					if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: newID, Note: "unlocked", SetAt: now}); err != nil {
						t.Fatalf("seed successor: %v", err)
					}
				}
				if err := succeeder.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{OldCanonicalID: oldID, NewCanonicalID: newID}}); err != nil {
					t.Fatalf("ApplyCanonicalIDSuccessions: %v", err)
				}

				for _, id := range []string{newID, guestID} {
					state, found, err := store.GetResourceOperatorState(id)
					if err != nil || !found || !state.BlocksRemediation() {
						t.Fatalf("%s lost the lock across the succession: found=%v err=%v state=%+v", id, found, err, state)
					}
				}
			})
		}
	}
}

// A succession keeps the IDs its links joined together. The lock follows a
// chain hop by hop as each succession applies, a link that a newer exclusion
// supersedes once it is re-keyed still hands its lock to the member it leaves
// behind (on either side of the succession), and a predecessor linked to its
// own successor still has its whole row move across.
func TestCanonicalIDSuccessionKeepsLinkedMembersTogether(t *testing.T) {
	stores := map[string]func(t *testing.T) ResourceStore{
		"memory": func(t *testing.T) ResourceStore { return NewMemoryStore() },
		"sqlite": func(t *testing.T) ResourceStore {
			store, err := NewSQLiteResourceStore(t.TempDir(), "default")
			if err != nil {
				t.Fatalf("NewSQLiteResourceStore: %v", err)
			}
			t.Cleanup(func() { _ = store.Close() })
			return store
		},
	}
	now := time.Now().UTC()
	locked := func(t *testing.T, store ResourceStore, id string) bool {
		t.Helper()
		state, found, err := store.GetResourceOperatorState(id)
		if err != nil {
			t.Fatalf("read operator state of %s: %v", id, err)
		}
		return found && state.BlocksRemediation()
	}
	apply := func(t *testing.T, store ResourceStore, oldID, newID string) {
		t.Helper()
		if err := store.(canonicalIDSuccessor).ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{OldCanonicalID: oldID, NewCanonicalID: newID}}); err != nil {
			t.Fatalf("ApplyCanonicalIDSuccessions %s -> %s: %v", oldID, newID, err)
		}
	}
	for name, newStore := range stores {
		t.Run(name+"/chain across two batches", func(t *testing.T) {
			store := newStore(t)
			for id, state := range map[string]ResourceOperatorState{
				"hop-a": {NeverAutoRemediate: true},
				"hop-b": {Note: "unlocked"},
				"hop-c": {Note: "unlocked"},
			} {
				state.CanonicalID, state.SetAt = id, now
				if err := store.SetResourceOperatorState(state); err != nil {
					t.Fatalf("seed %s: %v", id, err)
				}
			}
			apply(t, store, "hop-a", "hop-b")
			apply(t, store, "hop-b", "hop-c")
			for _, id := range []string{"hop-b", "hop-c"} {
				if !locked(t, store, id) {
					t.Fatalf("%s did not get the lock of the chain's first predecessor", id)
				}
			}
		})
		t.Run(name+"/predecessor's link superseded once re-keyed", func(t *testing.T) {
			store := newStore(t)
			if err := store.AddLink(ResourceLink{ResourceA: "split-a", ResourceB: "split-b", PrimaryID: "split-a", CreatedAt: now.Add(-2 * time.Hour)}); err != nil {
				t.Fatalf("link: %v", err)
			}
			// An exclusion naming the successor, newer than the link: once the
			// link is re-keyed onto the successor this exclusion supersedes it.
			if err := store.AddExclusion(ResourceExclusion{ResourceA: "split-a2", ResourceB: "split-b", CreatedAt: now.Add(-time.Hour)}); err != nil {
				t.Fatalf("exclude: %v", err)
			}
			if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "split-a", NeverAutoRemediate: true, SetAt: now}); err != nil {
				t.Fatalf("operator locks the merged resource: %v", err)
			}
			apply(t, store, "split-a", "split-a2")
			for _, id := range []string{"split-a2", "split-b"} {
				if !locked(t, store, id) {
					t.Fatalf("%s lost the lock when the succession split the link", id)
				}
			}
		})
		t.Run(name+"/successor's link superseded once re-keyed", func(t *testing.T) {
			store := newStore(t)
			if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "side-b", Note: "unlocked", SetAt: now}); err != nil {
				t.Fatalf("seed successor: %v", err)
			}
			if err := store.AddLink(ResourceLink{ResourceA: "side-b", ResourceB: "side-x", PrimaryID: "side-x", CreatedAt: now.Add(-2 * time.Hour)}); err != nil {
				t.Fatalf("link: %v", err)
			}
			if err := store.AddExclusion(ResourceExclusion{ResourceA: "side-a", ResourceB: "side-x", CreatedAt: now.Add(-time.Hour)}); err != nil {
				t.Fatalf("exclude: %v", err)
			}
			if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "side-x", NeverAutoRemediate: true, SetAt: now}); err != nil {
				t.Fatalf("operator locks the merged resource: %v", err)
			}
			apply(t, store, "side-a", "side-b")
			for _, id := range []string{"side-b", "side-x"} {
				if !locked(t, store, id) {
					t.Fatalf("%s lost the lock when the succession split the successor's link", id)
				}
			}
		})
		t.Run(name+"/star batch: sharing waits until every predecessor's row has moved", func(t *testing.T) {
			store := newStore(t)
			if err := store.AddLink(ResourceLink{ResourceA: "star-b", ResourceB: "star-x", PrimaryID: "star-x"}); err != nil {
				t.Fatalf("link: %v", err)
			}
			if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "star-x", NeverAutoRemediate: true, SetAt: now}); err != nil {
				t.Fatalf("operator locks the linked resource: %v", err)
			}
			if err := store.SetResourceOperatorState(ResourceOperatorState{
				CanonicalID: "star-old2", Note: "keep", Criticality: CriticalityHigh, SetAt: now.Add(-time.Hour), SetBy: "alice",
			}); err != nil {
				t.Fatalf("seed second predecessor: %v", err)
			}
			if err := store.(canonicalIDSuccessor).ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{
				{OldCanonicalID: "star-old1", NewCanonicalID: "star-b"},
				{OldCanonicalID: "star-old2", NewCanonicalID: "star-b"},
			}); err != nil {
				t.Fatalf("ApplyCanonicalIDSuccessions: %v", err)
			}
			got, found, err := store.GetResourceOperatorState("star-b")
			if err != nil || !found {
				t.Fatalf("successor row missing: found=%v err=%v", found, err)
			}
			if got.Note != "keep" || got.Criticality != CriticalityHigh {
				t.Fatalf("a block-only row made for the first predecessor shadowed the second one's whole row: %+v", got)
			}
			if !got.BlocksRemediation() {
				t.Fatalf("the successor did not get the lock of the resource its links joined: %+v", got)
			}
		})
		t.Run(name+"/forward chain in one batch, link superseded once re-keyed", func(t *testing.T) {
			store := newStore(t)
			if err := store.AddLink(ResourceLink{ResourceA: "fwd-a", ResourceB: "fwd-x", PrimaryID: "fwd-x", CreatedAt: now.Add(-2 * time.Hour)}); err != nil {
				t.Fatalf("link: %v", err)
			}
			if err := store.AddExclusion(ResourceExclusion{ResourceA: "fwd-b", ResourceB: "fwd-x", CreatedAt: now.Add(-time.Hour)}); err != nil {
				t.Fatalf("exclude: %v", err)
			}
			if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "fwd-x", NeverAutoRemediate: true, SetAt: now}); err != nil {
				t.Fatalf("operator locks the linked resource: %v", err)
			}
			if err := store.(canonicalIDSuccessor).ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{
				{OldCanonicalID: "fwd-a", NewCanonicalID: "fwd-b"},
				{OldCanonicalID: "fwd-b", NewCanonicalID: "fwd-c"},
			}); err != nil {
				t.Fatalf("ApplyCanonicalIDSuccessions: %v", err)
			}
			for _, id := range []string{"fwd-c", "fwd-x"} {
				if !locked(t, store, id) {
					t.Fatalf("%s lost the lock across a chain declared in one batch", id)
				}
			}
			if state, found, err := store.GetResourceOperatorState("fwd-b"); err != nil || found {
				t.Fatalf("the re-keyed intermediate ID was given a row: found=%v err=%v state=%+v", found, err, state)
			}
		})
		t.Run(name+"/predecessor linked to its successor keeps its whole row", func(t *testing.T) {
			store := newStore(t)
			if err := store.AddLink(ResourceLink{ResourceA: "own-a", ResourceB: "own-b", PrimaryID: "own-a"}); err != nil {
				t.Fatalf("link: %v", err)
			}
			if err := store.SetResourceOperatorState(ResourceOperatorState{
				CanonicalID: "own-a", LifecycleState: LifecycleStateRetired, Note: "keep", SetAt: now.Add(-time.Hour), SetBy: "alice",
			}); err != nil {
				t.Fatalf("seed predecessor: %v", err)
			}
			apply(t, store, "own-a", "own-b")
			got, found, err := store.GetResourceOperatorState("own-b")
			if err != nil || !found {
				t.Fatalf("successor row missing: found=%v err=%v", found, err)
			}
			if got.LifecycleState != LifecycleStateRetired || got.Note != "keep" || got.SetBy != "alice" {
				t.Fatalf("the predecessor's whole row did not move across: %+v", got)
			}
		})
	}
}

// Link rows may leave the optional reason and author empty (NULL) or hold a
// time the driver cannot read. Reading links to settle a lock must not stop an
// unrelated succession, or the upgrade carry, from completing.
func TestSQLiteSuccessionAndUpgradeCarryIgnoreMalformedLinkRows(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	// A row without reason or author, one with an integer where the driver
	// expects a time, and one with no time at all.
	for _, row := range []string{
		`INSERT INTO resource_links (resource_a, resource_b, primary_id) VALUES ('unrelated', 'unrelated', 'unrelated')`,
		`INSERT INTO resource_links (resource_a, resource_b, primary_id, reason, created_by, created_at) VALUES ('z1', 'z2', 'z1', '', '', 123)`,
		`INSERT INTO resource_links (resource_a, resource_b, primary_id, created_at) VALUES ('z3', 'z4', 'z3', NULL)`,
	} {
		if _, err := store.db.Exec(row); err != nil {
			t.Fatalf("seed malformed link row: %v", err)
		}
	}
	if err := store.SetResourceOperatorState(ResourceOperatorState{CanonicalID: "null-old", NeverAutoRemediate: true, Note: "moves", SetAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed predecessor: %v", err)
	}
	if err := store.ApplyCanonicalIDSuccessions([]CanonicalIDSuccession{{OldCanonicalID: "null-old", NewCanonicalID: "null-new"}}); err != nil {
		t.Fatalf("succession stopped by an unrelated link row: %v", err)
	}
	if got, found, err := store.GetResourceOperatorState("null-new"); err != nil || !found || got.Note != "moves" {
		t.Fatalf("predecessor's row did not move: found=%v err=%v state=%+v", found, err, got)
	}
	if _, err := store.db.Exec(`DELETE FROM resource_store_migrations`); err != nil {
		t.Fatalf("forget the carry marker: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store.Close()
	var recorded int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM resource_store_migrations`).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("the upgrade carry did not complete past the unrelated link row: markers=%d err=%v", recorded, err)
	}
}
