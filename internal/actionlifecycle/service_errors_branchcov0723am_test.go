package actionlifecycle

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestBranchcov0723Am_AvailabilityCheckError covers both the Error() format
// and the Unwrap() cause-reachability for AvailabilityCheckError, including
// the populated-cause, nil-cause (zero-value), and errors.Is/errors.As paths.
// The other typed errors in service.go are already exercised by
// TestTypedError_Error_FormatsFields / TestTypedError_Unwrap in
// dispatch_context_branchcov0718_test.go; AvailabilityCheckError is the one
// gap those tests leave.
func TestBranchcov0723Am_AvailabilityCheckError(t *testing.T) {
	cause := errors.New("registry probe timed out")

	t.Run("populated_cause_formats_message_and_unwraps", func(t *testing.T) {
		err := &AvailabilityCheckError{Err: cause}
		want := "action execution availability check: registry probe timed out"
		if got := err.Error(); got != want {
			t.Fatalf("Error()=%q; want %q", got, want)
		}
		if unwrapped := err.Unwrap(); unwrapped != cause {
			t.Fatalf("Unwrap()=%v; want %v", unwrapped, cause)
		}
		if !errors.Is(err, cause) {
			t.Fatalf("errors.Is(err, cause)=false; want true")
		}
		var target *AvailabilityCheckError
		if !errors.As(err, &target) || target.Err != cause {
			t.Fatalf("errors.As target=%#v; want wrapped cause", target)
		}
	})

	t.Run("zero_value_renders_nil_cause_and_unwraps_to_nil", func(t *testing.T) {
		err := &AvailabilityCheckError{}
		// %v on a nil interface renders as "<nil>".
		want := "action execution availability check: <nil>"
		if got := err.Error(); got != want {
			t.Fatalf("Error()=%q; want %q", got, want)
		}
		if unwrapped := err.Unwrap(); unwrapped != nil {
			t.Fatalf("Unwrap()=%v; want nil", unwrapped)
		}
		// errors.Is against a non-nil sentinel must be false when the chain
		// has no matching target.
		if errors.Is(err, cause) {
			t.Fatalf("errors.Is(zero-value err, cause)=true; want false")
		}
	})
}

// failingCreateStore embeds a real ResourceStore and forces CreateActionAudit
// to return the configured error, mirroring the barrierDecisionStore /
// delayedCreateStore wrapper pattern used elsewhere in this package.
type failingCreateStore struct {
	unified.ResourceStore
	createErr error
}

func (s *failingCreateStore) CreateActionAudit(record unified.ActionAuditRecord, events []unified.ActionLifecycleEvent) (unified.ActionAuditRecord, bool, error) {
	return unified.ActionAuditRecord{}, false, s.createErr
}

// TestBranchcov0723Am_PersistPlanAudit drives PersistPlanAudit through every
// branch of persistPlanAudit: the store-error path, the success path for both
// PlannedActionState arms (single planned event vs. appended pending event),
// idempotent replan (created=false, no error), a zero-value request, and a
// minimal plan with no blast-radius/preflight payload. Audit fields are read
// back through the same store rather than asserted from the return value
// alone.
func TestBranchcov0723Am_PersistPlanAudit(t *testing.T) {
	plannedAt := time.Date(2026, 7, 23, 9, 30, 0, 0, time.UTC)

	t.Run("store_error_is_propagated_unchanged", func(t *testing.T) {
		storeErr := errors.New("disk write failed")
		store := &failingCreateStore{ResourceStore: unified.NewMemoryStore(), createErr: storeErr}
		req := unified.ActionRequest{RequestID: "req-1", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "agent:test"}
		plan := unified.ActionPlan{ActionID: "act-1", RequestID: "req-1", PlannedAt: plannedAt}
		err := PersistPlanAudit(store, req, plan)
		if !errors.Is(err, storeErr) {
			t.Fatalf("PersistPlanAudit err=%v; want %v", err, storeErr)
		}
		if got := err.Error(); got != storeErr.Error() {
			t.Fatalf("error message=%q; want %q (must not be swallowed)", got, storeErr.Error())
		}
		// No audit may have been persisted when the create failed.
		if _, found, qerr := store.GetActionAudit("act-1"); qerr != nil || found {
			t.Fatalf("audit persisted despite error: found=%v err=%v", found, qerr)
		}
	})

	t.Run("success_planned_state_records_single_event", func(t *testing.T) {
		store := unified.NewMemoryStore()
		req := unified.ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Params:         map[string]any{"mode": "graceful"},
			Reason:         "Recover after confirmed outage",
			RequestedBy:    "agent:test",
		}
		plan := unified.ActionPlan{
			ActionID:         "act-planned",
			RequestID:        "req-1",
			Allowed:          true,
			RequiresApproval: false,
			PlannedAt:        plannedAt,
			PlanHash:         "sha256:abc",
		}
		if err := PersistPlanAudit(store, req, plan); err != nil {
			t.Fatalf("PersistPlanAudit: %v", err)
		}
		record, found, err := store.GetActionAudit("act-planned")
		if err != nil || !found {
			t.Fatalf("GetActionAudit found=%v err=%v", found, err)
		}
		// Verify the audit fields PersistPlanAudit derives from req+plan.
		if record.ID != "act-planned" {
			t.Fatalf("record.ID=%q; want act-planned", record.ID)
		}
		if record.State != unified.ActionStatePlanned {
			t.Fatalf("record.State=%q; want %q", record.State, unified.ActionStatePlanned)
		}
		if record.CreatedAt != plannedAt || record.UpdatedAt != plannedAt {
			t.Fatalf("timestamps created=%v updated=%v; want %v", record.CreatedAt, record.UpdatedAt, plannedAt)
		}
		if !reflect.DeepEqual(record.Request, req) {
			t.Fatalf("record.Request=%#v; want %#v", record.Request, req)
		}
		// PersistPlanAudit copies the plan verbatim into the record; the store
		// then enriches default policy/TTL/preflight fields around it. Assert
		// only the identity-bearing fields PersistPlanAudit is responsible for
		// so the test stays scoped to this function's behaviour.
		if record.Plan.ActionID != plan.ActionID || record.Plan.PlannedAt != plan.PlannedAt ||
			record.Plan.PlanHash != plan.PlanHash || record.Plan.RequiresApproval != plan.RequiresApproval ||
			record.Plan.RequestID != plan.RequestID {
			t.Fatalf("record.Plan identity fields mismatch:\n got =%#v\n want=%#v", record.Plan, plan)
		}
		if record.Origin != nil {
			t.Fatalf("record.Origin=%#v; want nil (no origin supplied)", record.Origin)
		}
		events, err := store.GetActionLifecycleEvents("act-planned", time.Time{}, 100)
		if err != nil {
			t.Fatalf("GetActionLifecycleEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("len(events)=%d; want 1 (no second event for planned state)", len(events))
		}
		ev := events[0]
		wantEv := unified.ActionLifecycleEvent{
			ActionID: "act-planned", Timestamp: plannedAt,
			State: unified.ActionStatePlanned, Actor: "agent:test",
			Message: "Action plan created.",
		}
		if ev.ActionID != wantEv.ActionID || ev.Timestamp != wantEv.Timestamp ||
			ev.State != wantEv.State || ev.Actor != wantEv.Actor || ev.Message != wantEv.Message {
			t.Fatalf("event=%#v; want %#v", ev, wantEv)
		}
	})

	t.Run("success_pending_state_appends_waiting_for_approval_event", func(t *testing.T) {
		store := unified.NewMemoryStore()
		req := unified.ActionRequest{RequestID: "req-2", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "operator:1"}
		// RequiresApproval=true forces PlannedActionState to ActionStatePending,
		// exercising the "state != Planned" branch that appends the second event.
		plan := unified.ActionPlan{ActionID: "act-pending", RequestID: "req-2", RequiresApproval: true, PlannedAt: plannedAt}
		if err := PersistPlanAudit(store, req, plan); err != nil {
			t.Fatalf("PersistPlanAudit: %v", err)
		}
		record, found, err := store.GetActionAudit("act-pending")
		if err != nil || !found {
			t.Fatalf("GetActionAudit found=%v err=%v", found, err)
		}
		if record.State != unified.ActionStatePending {
			t.Fatalf("record.State=%q; want %q", record.State, unified.ActionStatePending)
		}
		events, err := store.GetActionLifecycleEvents("act-pending", time.Time{}, 100)
		if err != nil {
			t.Fatalf("GetActionLifecycleEvents: %v", err)
		}
		// GetActionLifecycleEvents returns events newest-first, so the
		// waiting-for-approval event precedes the planned transition.
		if len(events) != 2 {
			t.Fatalf("len(events)=%d; want 2 (planned + waiting-for-approval)", len(events))
		}
		pending := events[0]
		if pending.State != unified.ActionStatePending ||
			pending.Message != "Action is waiting for approval before execution." ||
			pending.Actor != "operator:1" || pending.Timestamp != plannedAt || pending.ActionID != "act-pending" {
			t.Fatalf("waiting-for-approval event=%#v", pending)
		}
		planned := events[1]
		if planned.State != unified.ActionStatePlanned || planned.Message != "Action plan created." {
			t.Fatalf("planned transition event=%#v; want planned transition", planned)
		}
	})

	t.Run("idempotent_replan_returns_nil_without_new_events", func(t *testing.T) {
		store := unified.NewMemoryStore()
		req := unified.ActionRequest{RequestID: "req-3", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "agent:test"}
		plan := unified.ActionPlan{ActionID: "act-replan", RequestID: "req-3", PlannedAt: plannedAt}
		if err := PersistPlanAudit(store, req, plan); err != nil {
			t.Fatalf("first PersistPlanAudit: %v", err)
		}
		// Replaying the identical plan must not error (created=false is silent).
		if err := PersistPlanAudit(store, req, plan); err != nil {
			t.Fatalf("second PersistPlanAudit: %v", err)
		}
		events, err := store.GetActionLifecycleEvents("act-replan", time.Time{}, 100)
		if err != nil {
			t.Fatalf("GetActionLifecycleEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("len(events)=%d after replan; want 1 (idempotent)", len(events))
		}
	})

	t.Run("zero_value_request_propagates_store_validation_error", func(t *testing.T) {
		// PersistPlanAudit performs no request validation of its own; it
		// delegates to the store. A fully zero-value request is rejected by
		// the store's NormalizeActionAuditRecord, and that error must flow
		// back to the caller unchanged (not swallowed, not transformed).
		store := unified.NewMemoryStore()
		plan := unified.ActionPlan{ActionID: "act-emptyreq", RequestID: "req-4", PlannedAt: plannedAt}
		err := PersistPlanAudit(store, unified.ActionRequest{}, plan)
		if err == nil {
			t.Fatalf("PersistPlanAudit(zero-value req)=nil; want store validation error")
		}
		if !strings.Contains(err.Error(), "action request resource id required") {
			t.Fatalf("err=%q; want substring %q (store validation propagated verbatim)", err.Error(), "action request resource id required")
		}
		// Nothing was persisted.
		if _, found, qerr := store.GetActionAudit("act-emptyreq"); qerr != nil || found {
			t.Fatalf("audit persisted despite validation error: found=%v err=%v", found, qerr)
		}
	})

	t.Run("request_with_only_required_fields_is_persisted_faithfully", func(t *testing.T) {
		// A request carrying just the store-required fields (ResourceID,
		// CapabilityName, RequestedBy) but leaving Reason/Params empty must
		// round-trip those empty fields verbatim — proving PersistPlanAudit
		// does not synthesize or alter request content.
		store := unified.NewMemoryStore()
		req := unified.ActionRequest{
			RequestID:      "req-4b",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			RequestedBy:    "agent:test",
		}
		plan := unified.ActionPlan{ActionID: "act-thinreq", RequestID: "req-4b", PlannedAt: plannedAt}
		if err := PersistPlanAudit(store, req, plan); err != nil {
			t.Fatalf("PersistPlanAudit: %v", err)
		}
		record, found, err := store.GetActionAudit("act-thinreq")
		if err != nil || !found {
			t.Fatalf("GetActionAudit found=%v err=%v", found, err)
		}
		if record.Request.Reason != "" {
			t.Fatalf("record.Request.Reason=%q; want empty (faithful pass-through)", record.Request.Reason)
		}
		if record.Request.RequestedBy != "agent:test" || record.Request.ResourceID != "vm:42" ||
			record.Request.CapabilityName != "restart" {
			t.Fatalf("record.Request=%#v; want %#v", record.Request, req)
		}
	})

	t.Run("minimal_plan_with_no_blast_radius_or_preflight", func(t *testing.T) {
		store := unified.NewMemoryStore()
		// "No steps": a bare plan carrying only the durable identity and
		// timestamp, with no PlanHash, PredictedBlastRadius, Preflight, or
		// PolicyDecision payload (Allowed is even false). PersistPlanAudit
		// must still persist a valid planned audit and preserve the identity
		// fields it copies from the input. (The store enriches default
		// policy/TTL/preflight around the plan; that is the store's job, not
		// PersistPlanAudit's, so only identity fields are asserted here.)
		plan := unified.ActionPlan{ActionID: "act-bare", RequestID: "req-5", PlannedAt: plannedAt}
		req := unified.ActionRequest{RequestID: "req-5", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "agent:test"}
		if err := PersistPlanAudit(store, req, plan); err != nil {
			t.Fatalf("PersistPlanAudit: %v", err)
		}
		record, found, err := store.GetActionAudit("act-bare")
		if err != nil || !found {
			t.Fatalf("GetActionAudit found=%v err=%v", found, err)
		}
		if record.Plan.ActionID != "act-bare" || record.Plan.PlannedAt != plannedAt ||
			record.Plan.RequestID != "req-5" || record.Plan.PlanHash != "" {
			t.Fatalf("record.Plan identity fields mismatch:\n got =%#v\n want ActionID/PlannedAt/RequestID preserved, empty PlanHash", record.Plan)
		}
		if record.State != unified.ActionStatePlanned {
			t.Fatalf("record.State=%q; want %q (no approval required)", record.State, unified.ActionStatePlanned)
		}
		events, _ := store.GetActionLifecycleEvents("act-bare", time.Time{}, 100)
		if len(events) != 1 {
			t.Fatalf("len(events)=%d; want 1 (single planned event)", len(events))
		}
	})
}

// TestBranchcov0723Am_WithPolicyMutation covers both arms of the nil-guard,
// the lock-held invocation of the write callback, and the faithful return of
// the callback's result.
func TestBranchcov0723Am_WithPolicyMutation(t *testing.T) {
	t.Run("nil_callback_returns_unavailable_error", func(t *testing.T) {
		s := &Service{}
		err := s.WithPolicyMutation(nil)
		if err == nil {
			t.Fatalf("WithPolicyMutation(nil)=nil; want error")
		}
		if got := err.Error(); !strings.Contains(got, "policy mutation unavailable") {
			t.Fatalf("err=%q; want substring %q", got, "policy mutation unavailable")
		}
	})

	t.Run("nil_service_returns_unavailable_error_without_panicking", func(t *testing.T) {
		var s *Service // typed nil receiver; the s==nil guard must short-circuit.
		err := s.WithPolicyMutation(func() error { return nil })
		if err == nil || !strings.Contains(err.Error(), "policy mutation unavailable") {
			t.Fatalf("nil-service err=%v; want policy mutation unavailable", err)
		}
	})

	t.Run("callback_returning_nil_is_invoked_exactly_once", func(t *testing.T) {
		s := &Service{}
		calls := 0
		err := s.WithPolicyMutation(func() error {
			calls++
			return nil
		})
		if err != nil {
			t.Fatalf("WithPolicyMutation err=%v; want nil", err)
		}
		if calls != 1 {
			t.Fatalf("callback invoked %d times; want exactly 1", calls)
		}
	})

	t.Run("callback_returning_error_is_propagated_unchanged", func(t *testing.T) {
		s := &Service{}
		writeErr := errors.New("write rejected by policy store")
		err := s.WithPolicyMutation(func() error {
			return writeErr
		})
		if !errors.Is(err, writeErr) {
			t.Fatalf("WithPolicyMutation err=%v; want %v (unchanged)", err, writeErr)
		}
	})

	t.Run("concurrent_mutation_is_serialized_by_admission_lock", func(t *testing.T) {
		// A shared Service must serialize two WithPolicyMutation calls:
		// while one holds the admission write lock, the other blocks. We
		// prove serialization by detecting overlap via a non-atomic counter
		// guarded only by the lock: if both callbacks ever ran at once the
		// in-flight count would briefly reach 2.
		s := &Service{}
		inFlight := 0
		maxInFlight := 0
		done := make(chan struct{})
		writer := func() {
			defer func() { done <- struct{}{} }()
			_ = s.WithPolicyMutation(func() error {
				inFlight++
				if inFlight > maxInFlight {
					maxInFlight = inFlight
				}
				// Yield to invite a race; the lock must prevent one.
				time.Sleep(10 * time.Millisecond)
				inFlight--
				return nil
			})
		}
		go writer()
		go writer()
		<-done
		<-done
		if maxInFlight != 1 {
			t.Fatalf("maxInFlight=%d; want 1 (mutations must be serialized)", maxInFlight)
		}
	})
}
