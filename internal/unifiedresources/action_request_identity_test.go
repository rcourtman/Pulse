package unifiedresources

import (
	"errors"
	"testing"
)

func TestActionRequestIdentityAcrossStores(t *testing.T) {
	for _, kind := range []string{"memory", "sqlite"} {
		t.Run(kind, func(t *testing.T) {
			var store ResourceStore = NewMemoryStore()
			if kind == "sqlite" {
				s, err := NewSQLiteResourceStore(t.TempDir(), "default")
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				store = s
			}
			original := atomicLifecycleTestRecord("accepted", ActionStatePending)
			if _, created, err := store.CreateActionAudit(original, atomicLifecycleInitialEvents(original)); err != nil || !created {
				t.Fatalf("initial created=%v err=%v", created, err)
			}
			replay := original
			replay.ID = "different-snapshot"
			replay.Plan.ActionID = replay.ID
			replay.Plan.PlanHash = "different-plan-hash"
			replay.Plan.PolicyVersion = "changed-policy"
			replay.Plan.ResourceVersion = "changed-resource"
			got, created, err := store.CreateActionAudit(replay, atomicLifecycleInitialEvents(replay))
			if err != nil || created || got.ID != original.ID || got.Plan.PlanHash != original.Plan.PlanHash {
				t.Fatalf("replay created=%v err=%v got=%#v", created, err, got)
			}
			if _, found, err := store.GetActionAudit(replay.ID); err != nil || found {
				t.Fatalf("tentative replay persisted: found=%v err=%v", found, err)
			}
			changed := replay
			changed.Request.Reason = "a different requested change"
			if _, _, err := store.CreateActionAudit(changed, nil); !errors.Is(err, ErrActionIdentityConflict) {
				t.Fatalf("conflicting request: %v", err)
			}
			changed = original
			changed.Origin = &ActionOrigin{Surface: "patrol", FindingID: "other", InvestigationID: "other", ProposalID: "other"}
			if _, _, err := store.GetActionAuditByRequest(changed.Request, changed.Origin); !errors.Is(err, ErrActionIdentityConflict) {
				t.Fatalf("conflicting origin: %v", err)
			}
			isolated := replay
			isolated.Request.Actor.CredentialID = "another-credential"
			if _, created, err := store.CreateActionAudit(isolated, atomicLifecycleInitialEvents(isolated)); err != nil || !created {
				t.Fatalf("actor isolation created=%v err=%v", created, err)
			}
		})
	}
}

func TestActionRequestIdentityConcurrentSQLiteInstancesAndReopen(t *testing.T) {
	dir := t.TempDir()
	first, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	original := atomicLifecycleTestRecord("candidate-a", ActionStatePending)
	alternate := original
	alternate.ID = "candidate-b"
	alternate.Plan.ActionID = alternate.ID
	alternate.Plan.PlanHash = "changed-snapshot"
	type result struct {
		record  ActionAuditRecord
		created bool
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i, store := range []*SQLiteResourceStore{first, second} {
		record := original
		if i == 1 {
			record = alternate
		}
		go func() {
			<-start
			r, c, e := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record))
			results <- result{r, c, e}
		}()
	}
	close(start)
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || a.created == b.created || a.record.ID != b.record.ID {
		t.Fatalf("concurrent results: %#v %#v", a, b)
	}
	first.Close()
	second.Close()
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record, found, err := reopened.GetActionAuditByRequest(original.Request, original.Origin)
	if err != nil || !found || record.ID != a.record.ID {
		t.Fatalf("reopened identity: found=%v err=%v id=%s", found, err, record.ID)
	}
	var count int
	if err := reopened.db.QueryRow("SELECT count(*) FROM action_audits").Scan(&count); err != nil || count != 1 {
		t.Fatalf("persisted actions=%d err=%v", count, err)
	}
}

// Existing databases can contain several action IDs for a single caller key.
// Do not arbitrarily select one, delete history, or add a third accepted plan.
func TestActionRequestIdentityLegacyDuplicatesRefuse(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first := atomicLifecycleTestRecord("legacy-first", ActionStatePending)
	second := first
	second.ID = "legacy-second"
	second.Plan.ActionID = second.ID
	second.Plan.PlanHash = "second-plan"
	for _, record := range []ActionAuditRecord{first, second} {
		if _, err := insertActionAuditSQL(store.db, record); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, err := store.GetActionAuditByRequest(first.Request, first.Origin); !errors.Is(err, ErrActionIdentityConflict) || found {
		t.Fatalf("ambiguous legacy lookup found=%v err=%v", found, err)
	}
	third := first
	third.ID = "attempted-third"
	third.Plan.ActionID = third.ID
	third.Plan.PlanHash = "third-plan"
	if _, created, err := store.CreateActionAudit(third, nil); !errors.Is(err, ErrActionIdentityConflict) || created {
		t.Fatalf("ambiguous creation created=%v err=%v", created, err)
	}
	var count int
	if err := store.db.QueryRow("SELECT count(*) FROM action_audits").Scan(&count); err != nil || count != 2 {
		t.Fatalf("legacy history changed: rows=%d err=%v", count, err)
	}
}

func TestActionRequestIdentityDoesNotEquateRedactedInputs(t *testing.T) {
	for _, kind := range []string{"memory", "sqlite"} {
		t.Run(kind, func(t *testing.T) {
			var store ResourceStore = NewMemoryStore()
			if kind == "sqlite" {
				s, err := NewSQLiteResourceStore(t.TempDir(), "default")
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				store = s
			}
			original := atomicLifecycleTestRecord("redacted-original", ActionStatePending)
			original.Request.Reason = "operator pasted token=test-first-value"
			if _, created, err := store.CreateActionAudit(original, nil); err != nil || !created {
				t.Fatalf("initial created=%v err=%v", created, err)
			}
			changed := original
			changed.ID = "redacted-replacement"
			changed.Plan.ActionID = changed.ID
			changed.Plan.PlanHash = "changed-hash"
			changed.Request.Reason = "operator pasted token=test-second-value"
			if RedactAuditText(original.Request.Reason) != RedactAuditText(changed.Request.Reason) {
				t.Fatal("fixture must collide only after redaction")
			}
			if _, created, err := store.CreateActionAudit(changed, nil); !errors.Is(err, ErrActionIdentityConflict) || created {
				t.Fatalf("different redacted inputs treated as replay: created=%v err=%v", created, err)
			}
			if _, _, err := store.GetActionAuditByRequest(original.Request, original.Origin); !errors.Is(err, ErrActionIdentityConflict) {
				t.Fatalf("lost original intent must remain unverifiable: %v", err)
			}
		})
	}
}
