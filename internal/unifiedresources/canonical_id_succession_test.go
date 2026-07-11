package unifiedresources

import (
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
