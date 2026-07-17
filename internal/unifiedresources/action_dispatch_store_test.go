package unifiedresources

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteActionDispatchBindingMigrationKeepsLegacyReceiptPendingInert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resources", "unified_resources.db")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE action_dispatch_attempts(attempt_id TEXT PRIMARY KEY,action_id TEXT NOT NULL UNIQUE,state TEXT NOT NULL,created_at DATETIME NOT NULL,updated_at DATETIME NOT NULL,lease_owner TEXT,lease_expires_at DATETIME,dispatch_count INTEGER NOT NULL DEFAULT 0);INSERT INTO action_dispatch_attempts(attempt_id,action_id,state,created_at,updated_at,dispatch_count)VALUES('legacy.dispatch.1','legacy','receipt_pending',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,1)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	attempt, found, err := store.GetActionDispatchAttempt("legacy")
	if err != nil || !found {
		t.Fatalf("attempt=%#v found=%v err=%v", attempt, found, err)
	}
	if attempt.State != ActionDispatchReceiptPending || attempt.OperationKind != "" || attempt.OperationVersion != 0 || attempt.RequestDigest != "" || attempt.AgentID != "" {
		t.Fatalf("migrated attempt=%#v", attempt)
	}
	if _, claimed, err := store.ClaimActionDispatch("legacy", "worker", time.Now(), time.Minute); err != nil || claimed {
		t.Fatalf("claimed=%v err=%v", claimed, err)
	}
}

func admitDispatchTestAction(t *testing.T, store ResourceStore, id string, now time.Time) ActionDispatchAttempt {
	t.Helper()
	record := atomicLifecycleTestRecord(id, ActionStatePlanned)
	record.CreatedAt = now
	record.UpdatedAt = now
	record.Plan.PlannedAt = now
	record.Plan.ExpiresAt = now.Add(time.Hour)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	started, event, err := BeginActionExecution(record, "operator", now)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := NewActionDispatchAttempt(id, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordActionExecutionAdmission(started, event, attempt); err != nil {
		t.Fatal(err)
	}
	return attempt
}

func TestMemoryStoreActionStateQueryDeepCopiesPolicyProvenance(t *testing.T) {
	store := NewMemoryStore()
	record := policyProvenanceTestRecord(t, "act_state_policy_copy")
	if _, created, err := store.CreateActionAudit(record, policyProvenanceInitialEvents(record)); err != nil || !created {
		t.Fatalf("CreateActionAudit: created=%v err=%v", created, err)
	}
	first, err := store.GetActionAuditsByStates([]ActionState{ActionStatePending}, 10)
	if err != nil || len(first) != 1 {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	first[0].Plan.PolicyDecision.Authorities[0].ReasonCodes[0] = PolicyReasonCapabilityApprovalMFA
	second, err := store.GetActionAuditsByStates([]ActionState{ActionStatePending}, 10)
	if err != nil || len(second) != 1 || second[0].Plan.PolicyDecision.Authorities[0].ReasonCodes[0] != PolicyReasonCapabilityApprovalAdmin {
		t.Fatalf("state query shared mutable provenance: second=%#v err=%v", second, err)
	}
}

func runActionDispatchCrashBoundaries(t *testing.T, store ResourceStore) {
	t.Helper()
	now := time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC)
	attempt := admitDispatchTestAction(t, store, "act_dispatch_crash", now)
	claimed, ok, err := store.ClaimActionDispatch(attempt.ActionID, "worker-a", now, time.Second)
	if err != nil || !ok || claimed.DispatchCount != 0 {
		t.Fatalf("initial claim=%#v ok=%v err=%v", claimed, ok, err)
	}
	// Crash after claim but before MarkActionDispatchStarted: the same durable
	// attempt is safely reclaimed and dispatch count remains zero.
	reclaimed, ok, err := store.ClaimActionDispatch(attempt.ActionID, "worker-b", now.Add(2*time.Second), time.Second)
	if err != nil || !ok || reclaimed.ID != attempt.ID || reclaimed.DispatchCount != 0 {
		t.Fatalf("reclaim=%#v ok=%v err=%v", reclaimed, ok, err)
	}
	started, err := store.MarkActionDispatchStarted(attempt.ID, "worker-b", now.Add(2*time.Second))
	if err != nil || started.State != ActionDispatchReceiptPending || started.DispatchCount != 1 {
		t.Fatalf("started=%#v err=%v", started, err)
	}
	if _, err := store.MarkActionDispatchStarted(attempt.ID, "worker-b", now.Add(3*time.Second)); !errors.Is(err, ErrActionDispatchNotClaimable) {
		t.Fatalf("duplicate pre-send boundary error=%v", err)
	}
	// Crash after the pre-send boundary: recovery cannot claim or resend.
	resumed, ok, err := store.RecoverActionDispatch(attempt.ActionID, now.Add(time.Hour))
	if err != nil || ok || resumed.State != ActionDispatchReceiptPending || resumed.DispatchCount != 1 {
		t.Fatalf("post-start recovery=%#v ok=%v err=%v", resumed, ok, err)
	}
}

func TestMemoryStoreActionDispatchCrashBoundaries(t *testing.T) {
	runActionDispatchCrashBoundaries(t, NewMemoryStore())
}

func TestSQLiteActionDispatchCrashBoundaries(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runActionDispatchCrashBoundaries(t, store)
}

func runConcurrentActionDispatchClaimHasOneWinner(t *testing.T, store ResourceStore) {
	t.Helper()
	now := time.Now().UTC()
	attempt := admitDispatchTestAction(t, store, "act_dispatch_claim", now)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for _, owner := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			<-start
			_, claimed, claimErr := store.ClaimActionDispatch(attempt.ActionID, owner, now, time.Minute)
			if claimErr != nil {
				t.Errorf("ClaimActionDispatch: %v", claimErr)
			}
			if claimed {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}(owner)
	}
	close(start)
	wg.Wait()
	if winners != 1 {
		t.Fatalf("claim winners=%d, want 1", winners)
	}
}

func TestMemoryStoreConcurrentActionDispatchClaimHasOneWinner(t *testing.T) {
	runConcurrentActionDispatchClaimHasOneWinner(t, NewMemoryStore())
}

func TestSQLiteConcurrentActionDispatchClaimHasOneWinner(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runConcurrentActionDispatchClaimHasOneWinner(t, store)
}

func TestSQLiteActionDispatchAdmissionRollsBackAuditAndOutboxTogether(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	record := atomicLifecycleTestRecord("act_dispatch_rollback", ActionStatePlanned)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER fail_dispatch_outbox BEFORE INSERT ON action_dispatch_outbox BEGIN SELECT RAISE(FAIL, 'outbox failure'); END`); err != nil {
		t.Fatal(err)
	}
	started, event, _ := BeginActionExecution(record, "operator", now)
	attempt, _ := NewActionDispatchAttempt(record.ID, now)
	if err := store.RecordActionExecutionAdmission(started, event, attempt); err == nil {
		t.Fatal("expected admission failure")
	}
	current, found, err := store.GetActionAudit(record.ID)
	if err != nil || !found || current.State != ActionStatePlanned {
		t.Fatalf("current=%#v found=%v err=%v", current, found, err)
	}
	if _, found, err := store.GetActionDispatchAttempt(record.ID); err != nil || found {
		t.Fatalf("attempt found=%v err=%v", found, err)
	}
}

func TestSQLiteActionDispatchReceiptIsIdempotentAndCorrelatedAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	attempt := admitDispatchTestAction(t, store, "act_dispatch_receipt", now)
	if _, ok, err := store.ClaimActionDispatch(attempt.ActionID, "worker", now, time.Minute); err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if _, err := store.MarkActionDispatchStarted(attempt.ID, "worker", now); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: attempt.ActionID, TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Minute)}
	if _, err := store.RecordActionDispatchReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordActionDispatchReceipt(receipt); err != nil {
		t.Fatalf("duplicate receipt: %v", err)
	}
	receipt.TransportRequestID = "different"
	if _, err := store.RecordActionDispatchReceipt(receipt); !errors.Is(err, ErrActionDispatchReceiptConflict) {
		t.Fatalf("conflicting receipt error=%v", err)
	}
}

func runEarlyActionDispatchReceiptIsRejected(t *testing.T, store ResourceStore) {
	t.Helper()
	now := time.Now().UTC()
	attempt := admitDispatchTestAction(t, store, "act_dispatch_early_receipt", now)
	if _, ok, err := store.ClaimActionDispatch(attempt.ActionID, "worker", now, time.Minute); err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: attempt.ActionID, TransportRequestID: attempt.ID, ReceivedAt: now}
	if _, err := store.RecordActionDispatchReceipt(receipt); !errors.Is(err, ErrActionDispatchReceiptConflict) {
		t.Fatalf("early receipt error=%v", err)
	}
	current, found, err := store.GetActionDispatchAttempt(attempt.ActionID)
	if err != nil || !found || current.State != ActionDispatchClaimed || current.DispatchCount != 0 {
		t.Fatalf("current=%#v found=%v err=%v", current, found, err)
	}
}

func TestMemoryStoreEarlyActionDispatchReceiptIsRejected(t *testing.T) {
	runEarlyActionDispatchReceiptIsRejected(t, NewMemoryStore())
}

func TestSQLiteEarlyActionDispatchReceiptIsRejected(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runEarlyActionDispatchReceiptIsRejected(t, store)
}

func prepareCorrelatedDispatchCompletion(t *testing.T, store ResourceStore, id string, now time.Time) (ActionDispatchAttempt, ActionAuditRecord, ActionLifecycleEvent, ActionDispatchReceipt) {
	t.Helper()
	attempt := admitDispatchTestAction(t, store, id, now)
	if _, claimed, err := store.ClaimActionDispatch(id, "worker", now, time.Minute); err != nil || !claimed {
		t.Fatalf("claim claimed=%v err=%v", claimed, err)
	}
	if _, err := store.MarkActionDispatchStarted(attempt.ID, "worker", now); err != nil {
		t.Fatal(err)
	}
	executing, found, err := store.GetActionAudit(id)
	if err != nil || !found {
		t.Fatalf("executing found=%v err=%v", found, err)
	}
	completed, event, err := CompleteActionExecution(executing, &ExecutionResult{Success: true, Output: "correlated"}, "operator", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: id, TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Second)}
	return attempt, completed, event, receipt
}

func TestSQLiteCorrelatedDispatchCompletionRollsBackReceiptOnTerminalWriteFailure(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	attempt, completed, event, receipt := prepareCorrelatedDispatchCompletion(t, store, "act_dispatch_atomic_rollback", now)
	if _, err := store.db.Exec(`CREATE TRIGGER fail_terminal_event BEFORE INSERT ON action_lifecycle_events WHEN NEW.state IN ('completed', 'failed') BEGIN SELECT RAISE(FAIL, 'terminal event failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordActionDispatchCompletion(receipt, completed, event); err == nil {
		t.Fatal("expected atomic completion failure")
	}
	if _, found, err := store.GetActionDispatchReceipt(attempt.ID); err != nil || found {
		t.Fatalf("receipt found=%v err=%v", found, err)
	}
	currentAttempt, found, err := store.GetActionDispatchAttempt(attempt.ActionID)
	if err != nil || !found || currentAttempt.State != ActionDispatchReceiptPending {
		t.Fatalf("attempt=%#v found=%v err=%v", currentAttempt, found, err)
	}
	currentAudit, found, err := store.GetActionAudit(attempt.ActionID)
	if err != nil || !found || currentAudit.State != ActionStateExecuting || currentAudit.Result != nil {
		t.Fatalf("audit=%#v found=%v err=%v", currentAudit, found, err)
	}
}

func TestSQLiteCorrelatedDispatchCompletionSurvivesRestartWithoutReceiptOnlyState(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	attempt, completed, event, receipt := prepareCorrelatedDispatchCompletion(t, store, "act_dispatch_atomic_restart", now)
	if err := store.RecordActionDispatchCompletion(receipt, completed, event); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	storedReceipt, found, err := store.GetActionDispatchReceipt(attempt.ID)
	if err != nil || !found || storedReceipt.AttemptID != attempt.ID {
		t.Fatalf("receipt=%#v found=%v err=%v", storedReceipt, found, err)
	}
	storedAudit, found, err := store.GetActionAudit(attempt.ActionID)
	if err != nil || !found || storedAudit.State != ActionStateCompleted || storedAudit.Result == nil || !storedAudit.Result.Success {
		t.Fatalf("audit=%#v found=%v err=%v", storedAudit, found, err)
	}
}
