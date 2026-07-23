package unifiedresources

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Branch-coverage tests for currently-0.0%-covered MemoryStore methods in
// internal/unifiedresources:
//   - MemoryStore.RecordActionExpiry                  (action_dispatch_store.go)
//   - MemoryStore.GetActionDispatchReceipt            (action_dispatch_store.go)
//   - MemoryStore.RecordActionDispatchCompletion      (action_dispatch_store.go)
//   - MemoryStore.ExpireActionAudits                  (action_dispatch_store.go)
//   - MemoryStore.RecordActionExecutionRefusal        (store.go)
//   - MemoryStore.RecordActionLifecycleEvent          (store.go)
//   - MemoryStore.RecordActionPolicyExecutionAdmission (action_dispatch_store.go)
//
// Every subtest constructs its OWN MemoryStore so it passes when run alone via
// -run. The constructor, record, and event helpers reused here
// (NewMemoryStore, atomicLifecycleTestRecord, atomicLifecycleInitialEvents,
// admitDispatchTestAction, testBoundActionApproval) come from sibling _test.go
// files in this same package.

// branchcov0723amNow is the deterministic "current time" used across these
// subtests so expiry comparisons can be reasoned about by hand.
var branchcov0723amNow = time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)

// branchcov0723amFutureRecord returns an audit record whose Plan.ExpiresAt is
// 2h AFTER branchcov0723amNow. This is required by helpers like
// BeginPolicyActionExecution and ApplyActionDecision, which call
// ValidateActionExecutionStart and reject any record whose ExpiresAt is not
// strictly in the future relative to the "now" they are invoked with. Using a
// record whose ExpiresAt equals branchcov0723amNow + 2h lets callers pass
// branchcov0723amNow (or any value up to +2h) as the decision/admission time.
func branchcov0723amFutureRecord(id string, state ActionState) ActionAuditRecord {
	r := atomicLifecycleTestRecord(id, state)
	r.CreatedAt = branchcov0723amNow
	r.UpdatedAt = branchcov0723amNow
	r.Plan.PlannedAt = branchcov0723amNow
	r.Plan.ExpiresAt = branchcov0723amNow.Add(2 * time.Hour)
	return r
}

// branchcov0723amLeaseFor builds a ValidateActionPolicyAuthorizationLease-valid
// lease for record. Every field ValidateActionPolicyAuthorizationLease inspects
// is sourced from the record (so BeginPolicyActionExecution accepts it); the
// digest is recomputed last via the canonical helper.
func branchcov0723amLeaseFor(record ActionAuditRecord, now time.Time) ActionPolicyAuthorizationLease {
	lease := ActionPolicyAuthorizationLease{
		Version:                 1,
		OrgID:                   record.Request.Actor.OrgID,
		ActionID:                record.ID,
		ResourceID:              CanonicalResourceID(record.Request.ResourceID),
		CapabilityName:          strings.TrimSpace(record.Request.CapabilityName),
		PlanHash:                record.Plan.PlanHash,
		CapabilityPolicyVersion: record.Plan.PolicyVersion,
		TenantPolicyVersion:     "tenant:branchcov0723am",
		ResourcePolicyVersion:   "resource:branchcov0723am",
		LicenseAllowsAutoFix:    true,
		IssuedAt:                now,
		ExpiresAt:               now.Add(time.Hour),
	}
	lease.Digest = ActionPolicyAuthorizationDigest(lease)
	return lease
}

// branchcov0723amAdmitToReceiptPending drives an action all the way through
// admission, claim, and MarkActionDispatchStarted so that the attempt ends in
// ActionDispatchReceiptPending and the audit ends in ActionStateExecuting. It
// returns the persisted attempt ID and the executing audit so callers can build
// the terminal (Completed/Failed) record/event pair for completion tests.
func branchcov0723amAdmitToReceiptPending(t *testing.T, store *MemoryStore, id string, now time.Time) (ActionDispatchAttempt, ActionAuditRecord) {
	t.Helper()
	attempt := admitDispatchTestAction(t, store, id, now)
	if _, claimed, err := store.ClaimActionDispatch(id, "worker", now, time.Minute); err != nil || !claimed {
		t.Fatalf("ClaimActionDispatch claimed=%v err=%v", claimed, err)
	}
	if _, err := store.MarkActionDispatchStarted(attempt.ID, "worker", now); err != nil {
		t.Fatalf("MarkActionDispatchStarted: %v", err)
	}
	executing, found, err := store.GetActionAudit(id)
	if err != nil || !found || executing.State != ActionStateExecuting {
		t.Fatalf("executing audit not in executing state: found=%v err=%v state=%q", found, err, executing.State)
	}
	started, startedFound, err := store.GetActionDispatchAttempt(id)
	if err != nil || !startedFound || started.State != ActionDispatchReceiptPending {
		t.Fatalf("attempt not receipt_pending: found=%v state=%q err=%v", startedFound, started.State, err)
	}
	return started, executing
}

// ---------------------------------------------------------------------------
// MemoryStore.RecordActionExpiry
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_RecordActionExpiry(t *testing.T) {
	// happyPlannedToExpired: audit currently Planned -> Expired stored.
	t.Run("happy_planned_to_expired", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-planned", ActionStatePlanned)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		expired, event, err := ExpireAction(record, "system:expiry", branchcov0723amNow.Add(time.Hour))
		if err != nil {
			t.Fatalf("ExpireAction: %v", err)
		}
		if err := store.RecordActionExpiry(expired, event); err != nil {
			t.Fatalf("RecordActionExpiry: %v", err)
		}
		got, found, err := store.GetActionAudit(record.ID)
		if err != nil || !found {
			t.Fatalf("GetActionAudit found=%v err=%v", found, err)
		}
		if got.State != ActionStateExpired {
			t.Fatalf("state=%q want %q", got.State, ActionStateExpired)
		}
		if !got.UpdatedAt.Equal(branchcov0723amNow.Add(time.Hour)) {
			t.Fatalf("UpdatedAt=%v want %v", got.UpdatedAt, branchcov0723amNow.Add(time.Hour))
		}
		// The expiry lifecycle event must be persisted too.
		events, err := store.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
		if err != nil {
			t.Fatalf("GetActionLifecycleEvents: %v", err)
		}
		var sawExpired bool
		for _, e := range events {
			if e.State == ActionStateExpired {
				sawExpired = true
			}
		}
		if !sawExpired {
			t.Fatalf("expiry lifecycle event was not appended: %#v", events)
		}
	})

	// happyPendingToExpired: audit currently Pending -> Expired stored.
	t.Run("happy_pending_to_expired", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-pending", ActionStatePending)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		expired, event, err := ExpireAction(record, "system:expiry", branchcov0723amNow.Add(2*time.Hour))
		if err != nil {
			t.Fatalf("ExpireAction: %v", err)
		}
		if err := store.RecordActionExpiry(expired, event); err != nil {
			t.Fatalf("RecordActionExpiry: %v", err)
		}
		got, _, _ := store.GetActionAudit(record.ID)
		if got.State != ActionStateExpired {
			t.Fatalf("state=%q want %q", got.State, ActionStateExpired)
		}
	})

	// happyApprovedToExpired: audit currently Approved -> Expired stored.
	t.Run("happy_approved_to_expired", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-approved", ActionStateApproved)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		expired, event, err := ExpireAction(record, "system:expiry", branchcov0723amNow.Add(3*time.Hour))
		if err != nil {
			t.Fatalf("ExpireAction: %v", err)
		}
		if err := store.RecordActionExpiry(expired, event); err != nil {
			t.Fatalf("RecordActionExpiry: %v", err)
		}
		got, _, _ := store.GetActionAudit(record.ID)
		if got.State != ActionStateExpired {
			t.Fatalf("state=%q want %q", got.State, ActionStateExpired)
		}
	})

	// conflictWhenCurrentlyExecuting: default arm -> ErrActionExecutionFinal is
	// the fallback from actionTransitionConflict for an Executing current state
	// when desired is Expired.
	t.Run("conflict_when_currently_executing", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-conflict-exec", ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		// Build a hypothetical Expired transition for the same id (does not
		// need to be produced by ExpireAction — RecordActionExpiry only
		// requires normalization + matching id).
		expiredRecord := record
		expiredRecord.State = ActionStateExpired
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: branchcov0723amNow, State: ActionStateExpired, Actor: "system:expiry"}
		err := store.RecordActionExpiry(expiredRecord, event)
		if !errors.Is(err, ErrActionNotExecuting) {
			t.Fatalf("err=%v want %v", err, ErrActionNotExecuting)
		}
		// Audit must be unchanged.
		got, _, _ := store.GetActionAudit(record.ID)
		if got.State != ActionStateExecuting {
			t.Fatalf("state was mutated: %q", got.State)
		}
	})

	// conflictWhenCurrentlyCompleted: terminal current state.
	t.Run("conflict_when_currently_completed", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-conflict-done", ActionStateCompleted)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		expiredRecord := record
		expiredRecord.State = ActionStateExpired
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: branchcov0723amNow, State: ActionStateExpired, Actor: "system:expiry"}
		err := store.RecordActionExpiry(expiredRecord, event)
		if !errors.Is(err, ErrActionExecutionFinal) {
			t.Fatalf("err=%v want %v", err, ErrActionExecutionFinal)
		}
	})

	// notFound: pass a record whose audit was never created.
	t.Run("not_found", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-missing", ActionStatePlanned)
		expiredRecord := record
		expiredRecord.State = ActionStateExpired
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: branchcov0723amNow, State: ActionStateExpired, Actor: "system:expiry"}
		err := store.RecordActionExpiry(expiredRecord, event)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err=%v want 'not found'", err)
		}
	})

	// invalidRecordEmptyID: NormalizeActionAuditRecord fails before any locking.
	t.Run("invalid_record_empty_id", func(t *testing.T) {
		store := NewMemoryStore()
		bad := ActionAuditRecord{State: ActionStateExpired}
		event := ActionLifecycleEvent{ActionID: "anything", Timestamp: branchcov0723amNow, State: ActionStateExpired}
		err := store.RecordActionExpiry(bad, event)
		if err == nil {
			t.Fatal("expected normalization error for empty record id")
		}
	})

	// invalidEventEmptyActionID: NormalizeActionLifecycleEvent fails after the
	// record normalizes successfully.
	t.Run("invalid_event_empty_action_id", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-expiry-bad-event", ActionStateExpired)
		badEvent := ActionLifecycleEvent{Timestamp: branchcov0723amNow, State: ActionStateExpired}
		err := store.RecordActionExpiry(record, badEvent)
		if err == nil {
			t.Fatal("expected normalization error for empty event action id")
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.GetActionDispatchReceipt
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_GetActionDispatchReceipt(t *testing.T) {
	// emptyStoreReturnsNotFound: zero-value receipt, ok=false, err=nil.
	t.Run("empty_store_returns_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		got, ok, err := store.GetActionDispatchReceipt("anything.dispatch.1")
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if ok {
			t.Fatal("ok=true want false")
		}
		if got.AttemptID != "" || got.ActionID != "" || got.TransportRequestID != "" {
			t.Fatalf("zero-value expected, got %#v", got)
		}
	})

	// returnsPersistedReceipt: drive the full completion path, then read back
	// the persisted receipt and assert its concrete values.
	t.Run("returns_persisted_receipt", func(t *testing.T) {
		store := NewMemoryStore()
		now := branchcov0723amNow
		attempt, executing := branchcov0723amAdmitToReceiptPending(t, store, "act-receipt-get", now)
		completed, event, err := CompleteActionExecution(executing, &ExecutionResult{Success: true, Output: "ok"}, "operator", now.Add(time.Second))
		if err != nil {
			t.Fatalf("CompleteActionExecution: %v", err)
		}
		receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-receipt-get", TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Second)}
		if err := store.RecordActionDispatchCompletion(receipt, completed, event); err != nil {
			t.Fatalf("RecordActionDispatchCompletion: %v", err)
		}
		got, ok, err := store.GetActionDispatchReceipt(attempt.ID)
		if err != nil || !ok {
			t.Fatalf("GetActionDispatchReceipt ok=%v err=%v", ok, err)
		}
		if got.AttemptID != attempt.ID || got.ActionID != "act-receipt-get" || got.TransportRequestID != attempt.ID {
			t.Fatalf("receipt=%#v", got)
		}
		if !got.ReceivedAt.Equal(now.Add(time.Second)) {
			t.Fatalf("ReceivedAt=%v want %v", got.ReceivedAt, now.Add(time.Second))
		}
	})

	// whitespaceAttemptIDStillMatches: TrimSpace path on the lookup key.
	t.Run("whitespace_attempt_id_still_matches", func(t *testing.T) {
		store := NewMemoryStore()
		now := branchcov0723amNow
		attempt, executing := branchcov0723amAdmitToReceiptPending(t, store, "act-receipt-ws", now)
		completed, event, err := CompleteActionExecution(executing, &ExecutionResult{Success: true}, "operator", now.Add(time.Second))
		if err != nil {
			t.Fatalf("CompleteActionExecution: %v", err)
		}
		receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-receipt-ws", TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Second)}
		if err := store.RecordActionDispatchCompletion(receipt, completed, event); err != nil {
			t.Fatalf("RecordActionDispatchCompletion: %v", err)
		}
		got, ok, err := store.GetActionDispatchReceipt("  " + attempt.ID + "  ")
		if err != nil || !ok || got.AttemptID != attempt.ID {
			t.Fatalf("whitespace lookup: got=%#v ok=%v err=%v", got, ok, err)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.RecordActionDispatchCompletion
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_RecordActionDispatchCompletion(t *testing.T) {
	now := branchcov0723amNow

	// happyPathNoExistingReceipt: attempt in ReceiptPending, no receipt yet,
	// audit Executing -> success and persisted state.
	t.Run("happy_no_existing_receipt", func(t *testing.T) {
		store := NewMemoryStore()
		attempt, executing := branchcov0723amAdmitToReceiptPending(t, store, "act-completion-happy", now)
		completed, event, err := CompleteActionExecution(executing, &ExecutionResult{Success: true, Output: "ok"}, "operator", now.Add(time.Second))
		if err != nil {
			t.Fatalf("CompleteActionExecution: %v", err)
		}
		receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-completion-happy", TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Second)}
		if err := store.RecordActionDispatchCompletion(receipt, completed, event); err != nil {
			t.Fatalf("RecordActionDispatchCompletion: %v", err)
		}
		gotAttempt, found, err := store.GetActionDispatchAttempt("act-completion-happy")
		if err != nil || !found || gotAttempt.State != ActionDispatchReceiptRecorded {
			t.Fatalf("attempt=%#v found=%v err=%v", gotAttempt, found, err)
		}
		gotReceipt, found, err := store.GetActionDispatchReceipt(attempt.ID)
		if err != nil || !found || gotReceipt.TransportRequestID != attempt.ID {
			t.Fatalf("receipt=%#v found=%v err=%v", gotReceipt, found, err)
		}
		gotAudit, _, _ := store.GetActionAudit("act-completion-happy")
		if gotAudit.State != ActionStateCompleted || gotAudit.Result == nil || !gotAudit.Result.Success {
			t.Fatalf("audit=%#v", gotAudit)
		}
	})

	// happyPathExistingReceipt: persist the receipt first via
	// RecordActionDispatchReceipt, then call completion. The completion path
	// takes the receiptExists && matching-transport && state==ReceiptRecorded
	// branch and succeeds.
	t.Run("happy_existing_receipt_matching_transport", func(t *testing.T) {
		store := NewMemoryStore()
		attempt, executing := branchcov0723amAdmitToReceiptPending(t, store, "act-completion-existing", now)
		// Move the attempt from ReceiptPending to ReceiptRecorded by recording
		// the receipt once. This also stores the receipt row.
		_, err := store.RecordActionDispatchReceipt(ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-completion-existing", TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Second)})
		if err != nil {
			t.Fatalf("RecordActionDispatchReceipt: %v", err)
		}
		completed, event, completeErr := CompleteActionExecution(executing, &ExecutionResult{Success: true, Output: "ok"}, "operator", now.Add(2*time.Second))
		if completeErr != nil {
			t.Fatalf("CompleteActionExecution: %v", completeErr)
		}
		// Same TransportRequestID as the persisted receipt -> existing-receipt
		// branch must succeed.
		receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-completion-existing", TransportRequestID: attempt.ID, ReceivedAt: now.Add(time.Second)}
		if err := store.RecordActionDispatchCompletion(receipt, completed, event); err != nil {
			t.Fatalf("RecordActionDispatchCompletion: %v", err)
		}
		gotAudit, _, _ := store.GetActionAudit("act-completion-existing")
		if gotAudit.State != ActionStateCompleted {
			t.Fatalf("audit state=%q", gotAudit.State)
		}
	})

	// failAttemptNotFound: no attempt was created -> ErrActionDispatchNotFound.
	t.Run("fail_attempt_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		// Audit exists but no attempt was ever inserted for this action.
		record := atomicLifecycleTestRecord("act-completion-no-attempt", ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		completed := record
		completed.State = ActionStateCompleted
		completed.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: ActionDispatchAttemptID(record.ID), ActionID: record.ID, TransportRequestID: ActionDispatchAttemptID(record.ID), ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, completed, event)
		if !errors.Is(err, ErrActionDispatchNotFound) {
			t.Fatalf("err=%v want %v", err, ErrActionDispatchNotFound)
		}
	})

	// failAttemptIDMismatch: attempt exists for the action but its .ID differs
	// from receipt.AttemptID. Only reachable by direct map manipulation since
	// attempts are normally keyed by action ID with id == ActionDispatchAttemptID.
	t.Run("fail_attempt_id_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		actionID := "act-completion-id-mismatch"
		store.mu.Lock()
		store.actionDispatchAttempts[actionID] = ActionDispatchAttempt{
			ID:        "different.attempt.id",
			ActionID:  actionID,
			State:     ActionDispatchReceiptPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.mu.Unlock()
		record := atomicLifecycleTestRecord(actionID, ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		completed := record
		completed.State = ActionStateCompleted
		completed.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: ActionDispatchAttemptID(actionID), ActionID: actionID, TransportRequestID: ActionDispatchAttemptID(actionID), ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, completed, event)
		if !errors.Is(err, ErrActionDispatchNotFound) {
			t.Fatalf("err=%v want %v", err, ErrActionDispatchNotFound)
		}
	})

	// failExistingReceiptTransportMismatch: receipt persisted with
	// TransportRequestID="A"; completion called with "B" for the same
	// AttemptID -> ErrActionDispatchReceiptConflict.
	t.Run("fail_existing_receipt_transport_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		attempt, executing := branchcov0723amAdmitToReceiptPending(t, store, "act-completion-transport-mismatch", now)
		// Persist a receipt with one transport request id.
		_, err := store.RecordActionDispatchReceipt(ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-completion-transport-mismatch", TransportRequestID: "transport-A", ReceivedAt: now.Add(time.Second)})
		if err != nil {
			t.Fatalf("RecordActionDispatchReceipt: %v", err)
		}
		completed, event, completeErr := CompleteActionExecution(executing, &ExecutionResult{Success: true}, "operator", now.Add(2*time.Second))
		if completeErr != nil {
			t.Fatalf("CompleteActionExecution: %v", completeErr)
		}
		// Now call completion with a DIFFERENT transport request id.
		receipt := ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: "act-completion-transport-mismatch", TransportRequestID: "transport-B", ReceivedAt: now.Add(time.Second)}
		err = store.RecordActionDispatchCompletion(receipt, completed, event)
		if !errors.Is(err, ErrActionDispatchReceiptConflict) {
			t.Fatalf("err=%v want %v", err, ErrActionDispatchReceiptConflict)
		}
	})

	// failExistingReceiptAttemptStateNotRecorded: corner case where a receipt
	// exists but attempt.State is not ReceiptRecorded (only reachable via
	// direct map setup).
	t.Run("fail_existing_receipt_attempt_state_not_recorded", func(t *testing.T) {
		store := NewMemoryStore()
		actionID := "act-completion-state-recorded"
		attemptID := ActionDispatchAttemptID(actionID)
		// Set up: receipt exists, attempt is still queued (not ReceiptRecorded).
		store.mu.Lock()
		store.actionDispatchAttempts[actionID] = ActionDispatchAttempt{
			ID:        attemptID,
			ActionID:  actionID,
			State:     ActionDispatchQueued,
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.actionDispatchReceipts[attemptID] = ActionDispatchReceipt{AttemptID: attemptID, ActionID: actionID, TransportRequestID: attemptID, ReceivedAt: now}
		store.mu.Unlock()
		record := atomicLifecycleTestRecord(actionID, ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		completed := record
		completed.State = ActionStateCompleted
		completed.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: attemptID, ActionID: actionID, TransportRequestID: attemptID, ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, completed, event)
		if !errors.Is(err, ErrActionDispatchReceiptConflict) {
			t.Fatalf("err=%v want %v", err, ErrActionDispatchReceiptConflict)
		}
	})

	// failNoExistingReceiptAttemptStateNotPending: no receipt but attempt.State
	// is not ReceiptPending -> ErrActionDispatchReceiptConflict.
	t.Run("fail_no_existing_receipt_attempt_state_not_pending", func(t *testing.T) {
		store := NewMemoryStore()
		actionID := "act-completion-not-pending"
		attemptID := ActionDispatchAttemptID(actionID)
		store.mu.Lock()
		store.actionDispatchAttempts[actionID] = ActionDispatchAttempt{
			ID:        attemptID,
			ActionID:  actionID,
			State:     ActionDispatchQueued, // not ReceiptPending, no receipt
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.mu.Unlock()
		record := atomicLifecycleTestRecord(actionID, ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		completed := record
		completed.State = ActionStateCompleted
		completed.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: attemptID, ActionID: actionID, TransportRequestID: attemptID, ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, completed, event)
		if !errors.Is(err, ErrActionDispatchReceiptConflict) {
			t.Fatalf("err=%v want %v", err, ErrActionDispatchReceiptConflict)
		}
	})

	// failAuditNotFound: attempt exists but no audit was created.
	t.Run("fail_audit_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		actionID := "act-completion-no-audit"
		attemptID := ActionDispatchAttemptID(actionID)
		store.mu.Lock()
		store.actionDispatchAttempts[actionID] = ActionDispatchAttempt{
			ID:        attemptID,
			ActionID:  actionID,
			State:     ActionDispatchReceiptPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.mu.Unlock()
		// Build a synthetic completed record (no audit was created).
		synthetic := ActionAuditRecord{
			ID:        actionID,
			CreatedAt: now,
			UpdatedAt: now,
			State:     ActionStateCompleted,
			Request:   ActionRequest{RequestID: "req-" + actionID, ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "agent:test", Actor: ActionActor{SubjectID: "agent:test", Kind: ActionActorService, CredentialID: "service:test", OrgID: "default"}},
			Plan:      ActionPlan{ActionID: actionID, RequestID: "req-" + actionID, PlanHash: "sha256:" + actionID},
			Result:    &ExecutionResult{Success: true},
		}
		event := ActionLifecycleEvent{ActionID: actionID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: attemptID, ActionID: actionID, TransportRequestID: attemptID, ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, synthetic, event)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err=%v want 'not found'", err)
		}
	})

	// failAuditStateNotExecuting: audit is in a non-Executing state.
	t.Run("fail_audit_state_not_executing", func(t *testing.T) {
		store := NewMemoryStore()
		actionID := "act-completion-not-exec"
		attemptID := ActionDispatchAttemptID(actionID)
		store.mu.Lock()
		store.actionDispatchAttempts[actionID] = ActionDispatchAttempt{
			ID:        attemptID,
			ActionID:  actionID,
			State:     ActionDispatchReceiptPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.mu.Unlock()
		// Audit in Planned state (not Executing).
		record := atomicLifecycleTestRecord(actionID, ActionStatePlanned)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		completed := record
		completed.State = ActionStateCompleted
		completed.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: attemptID, ActionID: actionID, TransportRequestID: attemptID, ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, completed, event)
		if !errors.Is(err, ErrActionNotExecuting) {
			t.Fatalf("err=%v want %v", err, ErrActionNotExecuting)
		}
	})

	// failAuditIdentityMismatch: audit exists with a different PlanHash than
	// the proposed record -> ErrActionIdentityConflict.
	t.Run("fail_audit_identity_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		actionID := "act-completion-id-mismatch"
		attemptID := ActionDispatchAttemptID(actionID)
		// Persisted audit uses one PlanHash.
		persisted := atomicLifecycleTestRecord(actionID, ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(persisted, atomicLifecycleInitialEvents(persisted)); err != nil {
			t.Fatal(err)
		}
		store.mu.Lock()
		store.actionDispatchAttempts[actionID] = ActionDispatchAttempt{
			ID:        attemptID,
			ActionID:  actionID,
			State:     ActionDispatchReceiptPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.mu.Unlock()
		// Proposed record claims a DIFFERENT PlanHash.
		proposed := persisted
		proposed.Plan.PlanHash = "sha256:different"
		proposed.State = ActionStateCompleted
		proposed.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: actionID, Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		receipt := ActionDispatchReceipt{AttemptID: attemptID, ActionID: actionID, TransportRequestID: attemptID, ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, proposed, event)
		if !errors.Is(err, ErrActionIdentityConflict) {
			t.Fatalf("err=%v want %v", err, ErrActionIdentityConflict)
		}
	})

	// failNormalizeIdentitiesMismatch: receipt.ActionID != record.ID trips
	// normalizeActionDispatchCompletion before any locking.
	t.Run("fail_normalize_identities_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("action-X", ActionStateCompleted)
		record.Result = &ExecutionResult{Success: true}
		event := ActionLifecycleEvent{ActionID: "action-X", Timestamp: now, State: ActionStateCompleted, Actor: "operator"}
		// Receipt claims a different action than the record.
		receipt := ActionDispatchReceipt{AttemptID: ActionDispatchAttemptID("action-Y"), ActionID: "action-Y", TransportRequestID: ActionDispatchAttemptID("action-Y"), ReceivedAt: now}
		err := store.RecordActionDispatchCompletion(receipt, record, event)
		if err == nil {
			t.Fatal("expected normalization error for receipt/record identity mismatch")
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.ExpireActionAudits
// ---------------------------------------------------------------------------

// branchcov0723amExpirable builds an audit in the requested pre-terminal state
// with ExpiresAt set to expiresAt and the creation timestamps coherent with
// the global test "now".
func branchcov0723amExpirable(id string, state ActionState, expiresAt time.Time) ActionAuditRecord {
	r := atomicLifecycleTestRecord(id, state)
	r.Plan.ExpiresAt = expiresAt
	return r
}

func TestBranchcov0723Am_ExpireActionAudits(t *testing.T) {
	now := branchcov0723amNow
	pastExpiry := now.Add(-time.Hour)
	futureExpiry := now.Add(time.Hour)

	t.Run("empty_store_returns_empty_no_error", func(t *testing.T) {
		store := NewMemoryStore()
		out, err := store.ExpireActionAudits(now, 100)
		if err != nil {
			t.Fatalf("err=%v want nil", err)
		}
		if len(out) != 0 {
			t.Fatalf("out=%#v want empty", out)
		}
	})

	t.Run("nothing_eligible_all_terminal_state", func(t *testing.T) {
		store := NewMemoryStore()
		for i, state := range []ActionState{ActionStateExpired, ActionStateCompleted, ActionStateFailed} {
			id := "act-expire-terminal-" + string(rune('a'+i))
			r := branchcov0723amExpirable(id, state, pastExpiry)
			if _, _, err := store.CreateActionAudit(r, atomicLifecycleInitialEvents(r)); err != nil {
				t.Fatal(err)
			}
		}
		out, err := store.ExpireActionAudits(now, 100)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 0 {
			t.Fatalf("out=%d want 0", len(out))
		}
	})

	t.Run("nothing_eligible_zero_expires_at", func(t *testing.T) {
		// NormalizeActionAuditRecord backfills a zero ExpiresAt to
		// PlannedAt+5min, so this corner of ExpireActionAudits's
		// `!r.Plan.ExpiresAt.IsZero()` condition is NOT reachable via the
		// public CreateActionAudit API. Drive it directly by appending a
		// record with a zero ExpiresAt to the store's internal slice.
		store := NewMemoryStore()
		store.mu.Lock()
		r := atomicLifecycleTestRecord("act-expire-zero", ActionStatePlanned)
		r.Plan.ExpiresAt = time.Time{}
		store.actionAudits = append(store.actionAudits, r)
		store.mu.Unlock()
		out, err := store.ExpireActionAudits(now, 100)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 0 {
			t.Fatalf("out=%d want 0 (zero ExpiresAt must be ineligible)", len(out))
		}
		// Audit must remain in its original state.
		got, _, _ := store.GetActionAudit(r.ID)
		if got.State != ActionStatePlanned {
			t.Fatalf("state=%q want %q", got.State, ActionStatePlanned)
		}
	})

	t.Run("nothing_eligible_now_before_expiry", func(t *testing.T) {
		store := NewMemoryStore()
		r := branchcov0723amExpirable("act-expire-future", ActionStatePlanned, futureExpiry)
		if _, _, err := store.CreateActionAudit(r, atomicLifecycleInitialEvents(r)); err != nil {
			t.Fatal(err)
		}
		out, err := store.ExpireActionAudits(now, 100)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 0 {
			t.Fatalf("out=%d want 0", len(out))
		}
		got, _, _ := store.GetActionAudit(r.ID)
		if got.State != ActionStatePlanned {
			t.Fatalf("state=%q want %q (record was expired despite future ExpiresAt)", got.State, ActionStatePlanned)
		}
	})

	t.Run("some_eligible_some_not_only_eligible_returned_and_ineligible_preserved", func(t *testing.T) {
		store := NewMemoryStore()
		// Two eligible: Planned + Approved with past ExpiresAt.
		eligible1 := branchcov0723amExpirable("act-expire-elig-1", ActionStatePlanned, pastExpiry)
		eligible2 := branchcov0723amExpirable("act-expire-elig-2", ActionStateApproved, pastExpiry)
		// One ineligible: Planned with future ExpiresAt.
		ineligible := branchcov0723amExpirable("act-expire-inelig", ActionStatePending, futureExpiry)
		for _, r := range []ActionAuditRecord{eligible1, eligible2, ineligible} {
			r := r
			if _, _, err := store.CreateActionAudit(r, atomicLifecycleInitialEvents(r)); err != nil {
				t.Fatal(err)
			}
		}
		out, err := store.ExpireActionAudits(now, 100)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 2 {
			t.Fatalf("out=%d want 2 (only eligible records)", len(out))
		}
		// Ineligible record must remain Pending in the store.
		got, _, _ := store.GetActionAudit(ineligible.ID)
		if got.State != ActionStatePending {
			t.Fatalf("ineligible state=%q want %q (it should NOT have been expired)", got.State, ActionStatePending)
		}
		// Eligible records must be Expired now.
		for _, id := range []string{eligible1.ID, eligible2.ID} {
			g, _, _ := store.GetActionAudit(id)
			if g.State != ActionStateExpired {
				t.Fatalf("eligible %q state=%q want %q", id, g.State, ActionStateExpired)
			}
		}
	})

	t.Run("limit_smaller_than_eligible_honours_limit", func(t *testing.T) {
		store := NewMemoryStore()
		// Three eligible records; insertion order is deterministic because
		// MemoryStore.actionAudits is an ordered slice.
		ids := []string{"act-expire-limit-a", "act-expire-limit-b", "act-expire-limit-c"}
		for _, id := range ids {
			r := branchcov0723amExpirable(id, ActionStatePlanned, pastExpiry)
			if _, _, err := store.CreateActionAudit(r, atomicLifecycleInitialEvents(r)); err != nil {
				t.Fatal(err)
			}
		}
		out, err := store.ExpireActionAudits(now, 2)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 2 {
			t.Fatalf("out=%d want 2 (limit honours)", len(out))
		}
		// The first two inserted records must be the ones expired; the third
		// must remain Planned.
		outIDs := map[string]bool{out[0].ID: true, out[1].ID: true}
		if !outIDs[ids[0]] || !outIDs[ids[1]] {
			t.Fatalf("expected first two records to be expired, got %#v", outIDs)
		}
		third, _, _ := store.GetActionAudit(ids[2])
		if third.State != ActionStatePlanned {
			t.Fatalf("third record state=%q want %q (limit must leave it untouched)", third.State, ActionStatePlanned)
		}
	})

	t.Run("limit_zero_no_truncation", func(t *testing.T) {
		store := NewMemoryStore()
		for _, id := range []string{"act-expire-lim0-a", "act-expire-lim0-b"} {
			r := branchcov0723amExpirable(id, ActionStatePlanned, pastExpiry)
			if _, _, err := store.CreateActionAudit(r, atomicLifecycleInitialEvents(r)); err != nil {
				t.Fatal(err)
			}
		}
		out, err := store.ExpireActionAudits(now, 0)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 2 {
			t.Fatalf("out=%d want 2 (limit 0 means no truncation)", len(out))
		}
	})

	t.Run("limit_negative_no_truncation", func(t *testing.T) {
		store := NewMemoryStore()
		for _, id := range []string{"act-expire-limneg-a", "act-expire-limneg-b"} {
			r := branchcov0723amExpirable(id, ActionStatePlanned, pastExpiry)
			if _, _, err := store.CreateActionAudit(r, atomicLifecycleInitialEvents(r)); err != nil {
				t.Fatal(err)
			}
		}
		out, err := store.ExpireActionAudits(now, -5)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(out) != 2 {
			t.Fatalf("out=%d want 2 (negative limit means no truncation)", len(out))
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.RecordActionExecutionRefusal
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_RecordActionExecutionRefusal(t *testing.T) {
	now := branchcov0723amNow
	refuse := func(t *testing.T, record ActionAuditRecord) (ActionAuditRecord, ActionLifecycleEvent) {
		t.Helper()
		refused, event, err := RefuseActionExecution(record, ErrResourceRemediationLocked, "operator", now.Add(time.Hour))
		if err != nil {
			t.Fatalf("RefuseActionExecution: %v", err)
		}
		return refused, event
	}

	// happyFromPlanned/Pending/Approved: each pre-terminal current state must
	// transition to Failed and persist the refusal event.
	for _, initial := range []ActionState{ActionStatePlanned, ActionStatePending, ActionStateApproved} {
		initial := initial
		t.Run("happy_from_"+string(initial), func(t *testing.T) {
			store := NewMemoryStore()
			record := atomicLifecycleTestRecord("act-refuse-"+string(initial), initial)
			if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
				t.Fatal(err)
			}
			refused, event := refuse(t, record)
			if err := store.RecordActionExecutionRefusal(refused, event); err != nil {
				t.Fatalf("RecordActionExecutionRefusal: %v", err)
			}
			got, found, err := store.GetActionAudit(record.ID)
			if err != nil || !found {
				t.Fatalf("GetActionAudit found=%v err=%v", found, err)
			}
			if got.State != ActionStateFailed {
				t.Fatalf("state=%q want %q", got.State, ActionStateFailed)
			}
			if got.Result == nil || got.Result.Success {
				t.Fatalf("result=%#v want non-success", got.Result)
			}
			events, err := store.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
			if err != nil {
				t.Fatalf("GetActionLifecycleEvents: %v", err)
			}
			var sawFailedEvent bool
			for _, e := range events {
				if e.State == ActionStateFailed {
					sawFailedEvent = true
				}
			}
			if !sawFailedEvent {
				t.Fatalf("refusal lifecycle event not appended: %#v", events)
			}
		})
	}

	t.Run("fail_invalid_record_state_not_failed", func(t *testing.T) {
		store := NewMemoryStore()
		// record.State is Planned but event.State is Failed; the precondition
		// record.State == Failed fails.
		record := atomicLifecycleTestRecord("act-refuse-bad-state", ActionStatePlanned)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		notFailed := record // state still Planned
		event := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateFailed, Actor: "operator"}
		err := store.RecordActionExecutionRefusal(notFailed, event)
		if err == nil || !strings.Contains(err.Error(), "matching failed state") {
			t.Fatalf("err=%v want 'matching failed state'", err)
		}
	})

	t.Run("fail_invalid_event_state_not_failed", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-refuse-bad-event-state", ActionStatePlanned)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		failedRecord := record
		failedRecord.State = ActionStateFailed
		// event.State is Planned, not Failed.
		badEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStatePlanned, Actor: "operator"}
		err := store.RecordActionExecutionRefusal(failedRecord, badEvent)
		if err == nil || !strings.Contains(err.Error(), "matching failed state") {
			t.Fatalf("err=%v want 'matching failed state'", err)
		}
	})

	t.Run("fail_event_action_id_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-refuse-mismatch", ActionStateFailed)
		// event.ActionID != record.ID.
		event := ActionLifecycleEvent{ActionID: "different-id", Timestamp: now, State: ActionStateFailed, Actor: "operator"}
		err := store.RecordActionExecutionRefusal(record, event)
		if err == nil || !strings.Contains(err.Error(), "matching failed state") {
			t.Fatalf("err=%v want 'matching failed state'", err)
		}
	})

	t.Run("fail_record_normalization_empty_id", func(t *testing.T) {
		store := NewMemoryStore()
		bad := ActionAuditRecord{State: ActionStateFailed}
		event := ActionLifecycleEvent{ActionID: "any", Timestamp: now, State: ActionStateFailed}
		err := store.RecordActionExecutionRefusal(bad, event)
		if err == nil {
			t.Fatal("expected normalization error for empty record id")
		}
	})

	t.Run("fail_event_normalization_empty_action_id", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-refuse-bad-event", ActionStateFailed)
		badEvent := ActionLifecycleEvent{Timestamp: now, State: ActionStateFailed}
		err := store.RecordActionExecutionRefusal(record, badEvent)
		if err == nil {
			t.Fatal("expected normalization error for empty event action id")
		}
	})

	t.Run("fail_identity_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		persisted := atomicLifecycleTestRecord("act-refuse-idconflict", ActionStatePlanned)
		if _, _, err := store.CreateActionAudit(persisted, atomicLifecycleInitialEvents(persisted)); err != nil {
			t.Fatal(err)
		}
		// Proposed record claims a different PlanHash.
		proposed := persisted
		proposed.Plan.PlanHash = "sha256:different"
		refused, event := refuse(t, proposed)
		err := store.RecordActionExecutionRefusal(refused, event)
		if !errors.Is(err, ErrActionIdentityConflict) {
			t.Fatalf("err=%v want %v", err, ErrActionIdentityConflict)
		}
	})

	t.Run("fail_current_terminal", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-refuse-terminal", ActionStateCompleted)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		refused, event := refuse(t, record)
		err := store.RecordActionExecutionRefusal(refused, event)
		if !errors.Is(err, ErrActionExecutionFinal) {
			t.Fatalf("err=%v want %v", err, ErrActionExecutionFinal)
		}
	})

	t.Run("fail_current_executing", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-refuse-executing", ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		refused, event := refuse(t, record)
		err := store.RecordActionExecutionRefusal(refused, event)
		if !errors.Is(err, ErrActionNotExecuting) {
			t.Fatalf("err=%v want %v", err, ErrActionNotExecuting)
		}
	})

	t.Run("fail_record_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		record := atomicLifecycleTestRecord("act-refuse-missing", ActionStatePlanned)
		refused, event := refuse(t, record)
		err := store.RecordActionExecutionRefusal(refused, event)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err=%v want 'not found'", err)
		}
	})

	// scansPastUnrelatedAudits: with two unrelated audits in the store, the
	// refusal scan must skip past the unrelated one (exercising the `continue`
	// arm of the loop) and still find the target audit.
	t.Run("scans_past_unrelated_audits", func(t *testing.T) {
		store := NewMemoryStore()
		other := atomicLifecycleTestRecord("act-refuse-unrelated", ActionStatePlanned)
		if _, _, err := store.CreateActionAudit(other, atomicLifecycleInitialEvents(other)); err != nil {
			t.Fatal(err)
		}
		target := atomicLifecycleTestRecord("act-refuse-target", ActionStateApproved)
		if _, _, err := store.CreateActionAudit(target, atomicLifecycleInitialEvents(target)); err != nil {
			t.Fatal(err)
		}
		refused, event := refuse(t, target)
		if err := store.RecordActionExecutionRefusal(refused, event); err != nil {
			t.Fatalf("RecordActionExecutionRefusal: %v", err)
		}
		// Unrelated audit must remain Planned; target must be Failed.
		gotOther, _, _ := store.GetActionAudit(other.ID)
		if gotOther.State != ActionStatePlanned {
			t.Fatalf("unrelated state=%q want %q (must be untouched)", gotOther.State, ActionStatePlanned)
		}
		gotTarget, _, _ := store.GetActionAudit(target.ID)
		if gotTarget.State != ActionStateFailed {
			t.Fatalf("target state=%q want %q", gotTarget.State, ActionStateFailed)
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.RecordActionLifecycleEvent
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_RecordActionLifecycleEvent(t *testing.T) {
	now := branchcov0723amNow

	t.Run("happy_appends_new_transition", func(t *testing.T) {
		store := NewMemoryStore()
		event := ActionLifecycleEvent{ActionID: "act-event-1", Timestamp: now, State: ActionStatePlanned, Actor: "system"}
		if err := store.RecordActionLifecycleEvent(event); err != nil {
			t.Fatalf("RecordActionLifecycleEvent: %v", err)
		}
		events, err := store.GetActionLifecycleEvents("act-event-1", time.Time{}, 10)
		if err != nil {
			t.Fatalf("GetActionLifecycleEvents: %v", err)
		}
		if len(events) != 1 || events[0].State != ActionStatePlanned || events[0].ActionID != "act-event-1" {
			t.Fatalf("events=%#v", events)
		}
	})

	t.Run("fail_duplicate_transition_same_action_state", func(t *testing.T) {
		store := NewMemoryStore()
		event := ActionLifecycleEvent{ActionID: "act-event-dup", Timestamp: now, State: ActionStatePlanned, Actor: "system"}
		if err := store.RecordActionLifecycleEvent(event); err != nil {
			t.Fatalf("first RecordActionLifecycleEvent: %v", err)
		}
		// Re-recording the SAME (action, state) transition must be rejected.
		err := store.RecordActionLifecycleEvent(event)
		if err == nil || !strings.Contains(err.Error(), "already recorded") {
			t.Fatalf("err=%v want 'already recorded'", err)
		}
		// The duplicate must NOT have been appended.
		events, _ := store.GetActionLifecycleEvents("act-event-dup", time.Time{}, 10)
		if len(events) != 1 {
			t.Fatalf("events=%d want 1 (duplicate was appended)", len(events))
		}
	})

	t.Run("ok_transition_different_state_same_action", func(t *testing.T) {
		store := NewMemoryStore()
		first := ActionLifecycleEvent{ActionID: "act-event-multi", Timestamp: now, State: ActionStatePlanned, Actor: "system"}
		second := ActionLifecycleEvent{ActionID: "act-event-multi", Timestamp: now.Add(time.Second), State: ActionStateApproved, Actor: "operator"}
		if err := store.RecordActionLifecycleEvent(first); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordActionLifecycleEvent(second); err != nil {
			t.Fatalf("second RecordActionLifecycleEvent: %v", err)
		}
		events, _ := store.GetActionLifecycleEvents("act-event-multi", time.Time{}, 10)
		if len(events) != 2 {
			t.Fatalf("events=%d want 2", len(events))
		}
	})

	t.Run("ok_transition_different_action_same_state", func(t *testing.T) {
		store := NewMemoryStore()
		a := ActionLifecycleEvent{ActionID: "act-event-A", Timestamp: now, State: ActionStatePlanned, Actor: "system"}
		b := ActionLifecycleEvent{ActionID: "act-event-B", Timestamp: now, State: ActionStatePlanned, Actor: "system"}
		if err := store.RecordActionLifecycleEvent(a); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordActionLifecycleEvent(b); err != nil {
			t.Fatalf("different action with same state must be allowed: %v", err)
		}
	})

	t.Run("fail_duplicate_decision_same_revision", func(t *testing.T) {
		store := NewMemoryStore()
		// Use ApplyActionDecision to derive a valid normalized decision event.
		record := branchcov0723amFutureRecord("act-event-decision", ActionStatePending)
		approval := testBoundActionApproval(record, "operator@example.com", MethodSession, OutcomeApproved, "approved", now)
		updated, decisionEvent, err := ApplyActionDecision(record, approval, now)
		if err != nil {
			t.Fatalf("ApplyActionDecision: %v", err)
		}
		if err := store.RecordActionLifecycleEvent(decisionEvent); err != nil {
			t.Fatalf("first decision RecordActionLifecycleEvent: %v", err)
		}
		// Re-recording the SAME (action, decisionRevision) must be rejected.
		err = store.RecordActionLifecycleEvent(decisionEvent)
		if err == nil || !strings.Contains(err.Error(), "already recorded") {
			t.Fatalf("err=%v want 'already recorded'", err)
		}
		// The duplicate must NOT have been appended (only one event for this action).
		events, _ := store.GetActionLifecycleEvents(updated.ID, time.Time{}, 10)
		if len(events) != 1 {
			t.Fatalf("events=%d want 1 (duplicate decision was appended)", len(events))
		}
	})

	t.Run("ok_decision_for_different_action_same_revision", func(t *testing.T) {
		store := NewMemoryStore()
		// Two different actions can each carry a decision at the same revision.
		mkDecision := func(actionID string) ActionLifecycleEvent {
			record := branchcov0723amFutureRecord(actionID, ActionStatePending)
			approval := testBoundActionApproval(record, "operator@example.com", MethodSession, OutcomeApproved, "approved", now)
			_, event, err := ApplyActionDecision(record, approval, now)
			if err != nil {
				t.Fatalf("ApplyActionDecision(%s): %v", actionID, err)
			}
			return event
		}
		first := mkDecision("act-event-decision-A")
		second := mkDecision("act-event-decision-B")
		if err := store.RecordActionLifecycleEvent(first); err != nil {
			t.Fatal(err)
		}
		if err := store.RecordActionLifecycleEvent(second); err != nil {
			t.Fatalf("decision for different action with same revision must be allowed: %v", err)
		}
	})

	t.Run("fail_normalize_empty_action_id", func(t *testing.T) {
		store := NewMemoryStore()
		bad := ActionLifecycleEvent{Timestamp: now, State: ActionStatePlanned}
		err := store.RecordActionLifecycleEvent(bad)
		if err == nil {
			t.Fatal("expected normalization error for empty event action id")
		}
	})
}

// ---------------------------------------------------------------------------
// MemoryStore.RecordActionPolicyExecutionAdmission
// ---------------------------------------------------------------------------

func TestBranchcov0723Am_RecordActionPolicyExecutionAdmission(t *testing.T) {
	now := branchcov0723amNow

	// happyFromPending: audit currently Pending -> Executing after admission.
	t.Run("happy_from_pending", func(t *testing.T) {
		store := NewMemoryStore()
		record := branchcov0723amFutureRecord("act-policy-pending", ActionStatePending)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		lease := branchcov0723amLeaseFor(record, now.Add(time.Minute))
		approval := ActionApprovalRecord{Actor: "policy:auto", Method: MethodPolicy}
		execRecord, approvedEvent, startedEvent, err := BeginPolicyActionExecution(record, approval, lease, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("BeginPolicyActionExecution: %v", err)
		}
		attempt, err := NewActionDispatchAttempt(record.ID, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecordActionPolicyExecutionAdmission(execRecord, approvedEvent, startedEvent, attempt); err != nil {
			t.Fatalf("RecordActionPolicyExecutionAdmission: %v", err)
		}
		gotAudit, found, err := store.GetActionAudit(record.ID)
		if err != nil || !found || gotAudit.State != ActionStateExecuting {
			t.Fatalf("audit found=%v state=%q err=%v", found, gotAudit.State, err)
		}
		gotAttempt, found, err := store.GetActionDispatchAttempt(record.ID)
		if err != nil || !found || gotAttempt.State != ActionDispatchQueued {
			t.Fatalf("attempt found=%v state=%q err=%v", found, gotAttempt.State, err)
		}
		// Both the approval and the executing events must be persisted.
		events, _ := store.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
		states := map[ActionState]bool{}
		for _, e := range events {
			states[e.State] = true
		}
		if !states[ActionStateApproved] || !states[ActionStateExecuting] {
			t.Fatalf("missing approved/executing event: %#v", states)
		}
	})

	// happyFromPlanned: audit currently Planned (approval-free path) ->
	// Executing after admission.
	t.Run("happy_from_planned", func(t *testing.T) {
		store := NewMemoryStore()
		record := branchcov0723amFutureRecord("act-policy-planned", ActionStatePlanned)
		record.Plan.RequiresApproval = false
		record.Plan.ApprovalPolicy = ApprovalNone
		record.Plan.ApprovalRequirement = ApprovalRequirementForFloor(ApprovalNone)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		lease := branchcov0723amLeaseFor(record, now.Add(time.Minute))
		approval := ActionApprovalRecord{Actor: "policy:auto", Method: MethodPolicy}
		execRecord, approvedEvent, startedEvent, err := BeginPolicyActionExecution(record, approval, lease, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("BeginPolicyActionExecution: %v", err)
		}
		attempt, err := NewActionDispatchAttempt(record.ID, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecordActionPolicyExecutionAdmission(execRecord, approvedEvent, startedEvent, attempt); err != nil {
			t.Fatalf("RecordActionPolicyExecutionAdmission: %v", err)
		}
		gotAudit, _, _ := store.GetActionAudit(record.ID)
		if gotAudit.State != ActionStateExecuting {
			t.Fatalf("state=%q want %q", gotAudit.State, ActionStateExecuting)
		}
	})

	// failValidateAdmissionStatesMismatch: validateExecutionAdmission rejects
	// an event with the wrong state.
	t.Run("fail_validate_admission_event_state_mismatch", func(t *testing.T) {
		store := NewMemoryStore()
		record := branchcov0723amFutureRecord("act-policy-bad-event", ActionStateExecuting)
		// event.State is Pending (not Executing).
		badEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStatePending, Actor: "operator"}
		approvalEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateApproved, Actor: "operator"}
		attempt, err := NewActionDispatchAttempt(record.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		err = store.RecordActionPolicyExecutionAdmission(record, approvalEvent, badEvent, attempt)
		if err == nil {
			t.Fatal("expected validateExecutionAdmission to reject mismatched event state")
		}
	})

	// failNormalizeApprovalEventEmptyActionID: approvalEvent fails normalization.
	t.Run("fail_normalize_approval_event_empty_action_id", func(t *testing.T) {
		store := NewMemoryStore()
		// Build a valid Executing record + executionEvent + attempt first.
		record := branchcov0723amFutureRecord("act-policy-bad-approval", ActionStatePending)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		lease := branchcov0723amLeaseFor(record, now.Add(time.Minute))
		approval := ActionApprovalRecord{Actor: "policy:auto", Method: MethodPolicy}
		execRecord, _, startedEvent, err := BeginPolicyActionExecution(record, approval, lease, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("BeginPolicyActionExecution: %v", err)
		}
		attempt, err := NewActionDispatchAttempt(record.ID, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		// approvalEvent with empty ActionID cannot normalize.
		badApprovalEvent := ActionLifecycleEvent{Timestamp: now, State: ActionStateApproved}
		err = store.RecordActionPolicyExecutionAdmission(execRecord, badApprovalEvent, startedEvent, attempt)
		if err == nil {
			t.Fatal("expected normalization error for empty approvalEvent action id")
		}
	})

	// failRecordNotFound: no audit was created for this action.
	t.Run("fail_record_not_found", func(t *testing.T) {
		store := NewMemoryStore()
		record := branchcov0723amFutureRecord("act-policy-missing", ActionStatePending)
		lease := branchcov0723amLeaseFor(record, now.Add(time.Minute))
		approval := ActionApprovalRecord{Actor: "policy:auto", Method: MethodPolicy}
		execRecord, approvedEvent, startedEvent, err := BeginPolicyActionExecution(record, approval, lease, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("BeginPolicyActionExecution: %v", err)
		}
		attempt, err := NewActionDispatchAttempt(record.ID, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		err = store.RecordActionPolicyExecutionAdmission(execRecord, approvedEvent, startedEvent, attempt)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err=%v want 'not found'", err)
		}
	})

	// failCurrentStateNotAdmissible: audit currently Executing (not Planned or
	// Pending) -> actionTransitionConflict returns ErrActionAlreadyExecuting.
	t.Run("fail_current_state_not_admissible", func(t *testing.T) {
		store := NewMemoryStore()
		record := branchcov0723amFutureRecord("act-policy-executing", ActionStateExecuting)
		if _, _, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil {
			t.Fatal(err)
		}
		// Build the executing record/event pair directly. BeginPolicyActionExecution
		// rejects records that are already Executing, so construct manually to
		// exercise the MemoryStore-side state check.
		execRecord := record
		execRecord.State = ActionStateExecuting
		startedEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now.Add(time.Minute), State: ActionStateExecuting, Actor: "policy:auto"}
		approvedEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now.Add(time.Minute), State: ActionStateApproved, Actor: "policy:auto"}
		attempt, err := NewActionDispatchAttempt(record.ID, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		err = store.RecordActionPolicyExecutionAdmission(execRecord, approvedEvent, startedEvent, attempt)
		if !errors.Is(err, ErrActionAlreadyExecuting) {
			t.Fatalf("err=%v want %v", err, ErrActionAlreadyExecuting)
		}
		// Audit must remain Executing.
		got, _, _ := store.GetActionAudit(record.ID)
		if got.State != ActionStateExecuting {
			t.Fatalf("state=%q want %q (must be unchanged)", got.State, ActionStateExecuting)
		}
	})
}
