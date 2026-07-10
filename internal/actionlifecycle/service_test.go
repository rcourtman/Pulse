package actionlifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubExecutor struct {
	result    *unified.ExecutionResult
	err       error
	calls     int
	received  unified.ActionAuditRecord
	readiness *unified.ResourceActionReadiness
}

func (s *stubExecutor) ExecuteAction(_ context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	s.calls++
	s.received = record
	return s.result, s.err
}

func (s *stubExecutor) CheckActionAvailable(_ context.Context, _ unified.ActionRequest, _ unified.Resource) unified.ResourceActionReadiness {
	if s.readiness == nil {
		return unified.ResourceActionReadiness{}
	}
	return *s.readiness
}

type serviceEnv struct {
	store     unified.ResourceStore
	registry  *unified.ResourceRegistry
	executor  *stubExecutor
	completed []unified.ActionAuditRecord
	service   *Service
}

func testResource(now time.Time, minimumApproval unified.ActionApprovalLevel) unified.Resource {
	return unified.Resource{
		ID:        "vm:42",
		Type:      unified.ResourceTypeVM,
		Name:      "web-42",
		Status:    unified.StatusWarning,
		LastSeen:  now,
		UpdatedAt: now,
		Sources:   []unified.DataSource{unified.SourceProxmox},
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: minimumApproval,
				InternalHandler:      "proxmox.vm.restart",
				Params: []unified.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			},
		},
	}
}

func newServiceEnv(t *testing.T, resource unified.Resource) *serviceEnv {
	t.Helper()
	env := &serviceEnv{
		store:    unified.NewMemoryStore(),
		executor: &stubExecutor{result: &unified.ExecutionResult{Success: true, Output: "restarted"}},
	}
	env.registry = unified.NewRegistry(env.store)
	env.registry.IngestResources([]unified.Resource{resource})
	env.service = &Service{
		Registry: func(orgID string) (*unified.ResourceRegistry, error) {
			if orgID != "default" {
				return nil, errors.New("unknown org")
			}
			return env.registry, nil
		},
		Store: func(orgID string) (Store, error) {
			if orgID != "default" {
				return nil, errors.New("unknown org")
			}
			return env.store, nil
		},
		Executor: env.executor,
		OnActionCompleted: func(record unified.ActionAuditRecord) {
			env.completed = append(env.completed, record)
		},
	}
	return env
}

func restartRequest() unified.ActionRequest {
	return unified.ActionRequest{
		RequestID:      "req-1",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Params:         map[string]any{"mode": "graceful"},
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:test",
	}
}

func TestPlanPersistsPendingAuditAndLifecycle(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !plan.RequiresApproval {
		t.Fatal("expected admin-gated capability to require approval")
	}

	record, ok, err := env.store.GetActionAudit(plan.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if record.State != unified.ActionStatePending {
		t.Fatalf("audit state = %q, want pending_approval", record.State)
	}
	events, err := env.store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("lifecycle events = %d, want planned + pending", len(events))
	}

	// Idempotent replan must not duplicate lifecycle events.
	if _, err := env.service.Plan(context.Background(), "default", restartRequest()); err != nil {
		t.Fatalf("replan: %v", err)
	}
	events, err = env.store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents after replan: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("replan duplicated lifecycle events: %d", len(events))
	}
}

func TestPlanFailsClosedOnUnknownResourceAndCapability(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	missing := restartRequest()
	missing.ResourceID = "vm:404"
	var notFound *ResourceNotFoundError
	if _, err := env.service.Plan(context.Background(), "default", missing); !errors.As(err, &notFound) {
		t.Fatalf("unknown resource error = %v, want ResourceNotFoundError", err)
	}

	unknownCap := restartRequest()
	unknownCap.CapabilityName = "detonate"
	_, err := env.service.Plan(context.Background(), "default", unknownCap)
	var capErr *CapabilityNotFoundError
	if !errors.As(err, &capErr) || !errors.Is(err, actionplanner.ErrCapabilityNotFound) {
		t.Fatalf("unknown capability error = %v, want CapabilityNotFoundError wrapping ErrCapabilityNotFound", err)
	}
	if capErr.ResourceID != "vm:42" || capErr.CapabilityName != "detonate" {
		t.Fatalf("capability error detail = %#v", capErr)
	}

	empty := restartRequest()
	empty.ResourceID = "  "
	var validation *actionplanner.ValidationError
	if _, err := env.service.Plan(context.Background(), "default", empty); !errors.As(err, &validation) {
		t.Fatalf("empty resource id error = %v, want ValidationError", err)
	}

	audits, err := env.store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("failed plans must not persist audits, got %d", len(audits))
	}
}

func TestPlanAvailabilityRefusalPersistsNothing(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))
	env.executor.readiness = &unified.ResourceActionReadiness{
		Name:       "restart",
		Available:  false,
		ReasonCode: "agent_disconnected",
		Reason:     "no connected command agent",
	}

	_, err := env.service.Plan(context.Background(), "default", restartRequest())
	var refused *AvailabilityRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want AvailabilityRefusedError", err)
	}
	if refused.Readiness.ReasonCode != "agent_disconnected" {
		t.Fatalf("readiness = %#v", refused.Readiness)
	}
	audits, err := env.store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("refused availability must not persist audits, got %d", len(audits))
	}
}

func TestDecideApprovesPendingAction(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	updated, err := env.service.Decide(context.Background(), "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor:   "operator@example.com",
		Method:  unified.MethodAPI,
		Outcome: unified.OutcomeApproved,
		Reason:  "confirmed outage",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if updated.State != unified.ActionStateApproved {
		t.Fatalf("state = %q, want approved", updated.State)
	}
	if len(updated.Approvals) != 1 || updated.Approvals[0].Actor != "operator@example.com" {
		t.Fatalf("approvals = %#v", updated.Approvals)
	}

	var notFound *ActionNotFoundError
	if _, err := env.service.Decide(context.Background(), "default", "act_missing", unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	}); !errors.As(err, &notFound) {
		t.Fatalf("unknown action error = %v, want ActionNotFoundError", err)
	}
}

func TestExecuteRunsApprovedActionToTerminalAudit(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := env.service.Decide(context.Background(), "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	completed, err := env.service.Execute(context.Background(), "default", plan.ActionID, "operator@example.com", "approved restart")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if completed.State != unified.ActionStateCompleted {
		t.Fatalf("state = %q, want completed", completed.State)
	}
	if env.executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", env.executor.calls)
	}
	if len(env.completed) != 1 || env.completed[0].ID != plan.ActionID {
		t.Fatalf("completion publisher observed %#v", env.completed)
	}

	record, ok, err := env.store.GetActionAudit(plan.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if record.State != unified.ActionStateCompleted || record.Result == nil || !record.Result.Success {
		t.Fatalf("terminal audit = state %q result %#v", record.State, record.Result)
	}
}

func TestExecuteRefusesUnapprovedActionWithoutDispatch(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, "operator@example.com", ""); !errors.Is(err, unified.ErrActionNotApproved) {
		t.Fatalf("error = %v, want ErrActionNotApproved", err)
	}
	if env.executor.calls != 0 {
		t.Fatalf("executor must not run for unapproved actions, calls = %d", env.executor.calls)
	}
}

func TestExecuteRefusesRemediationLockedResource(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := env.store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:42",
		NeverAutoRemediate: true,
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	failed, err := env.service.Execute(context.Background(), "default", plan.ActionID, "agent:test", "")
	if !errors.Is(err, unified.ErrResourceRemediationLocked) {
		t.Fatalf("error = %v, want ErrResourceRemediationLocked", err)
	}
	if env.executor.calls != 0 {
		t.Fatalf("executor must not run on locked resources, calls = %d", env.executor.calls)
	}
	if failed.State != unified.ActionStateFailed {
		t.Fatalf("refusal must persist a terminal failed audit, state = %q", failed.State)
	}
	if len(env.completed) != 1 || env.completed[0].State != unified.ActionStateFailed {
		t.Fatalf("refusal must publish the failed record, got %#v", env.completed)
	}
}

func TestExecuteRefusesDriftedPlan(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// The capability policy tightens between plan and dispatch: the
	// replanned contract no longer matches the recorded plan hash.
	drifted := testResource(now, unified.ApprovalAdmin)
	env.registry = unified.NewRegistry(env.store)
	env.registry.IngestResources([]unified.Resource{drifted})

	failed, err := env.service.Execute(context.Background(), "default", plan.ActionID, "agent:test", "")
	if !errors.Is(err, unified.ErrActionPlanDrift) {
		t.Fatalf("error = %v, want ErrActionPlanDrift", err)
	}
	if env.executor.calls != 0 {
		t.Fatalf("executor must not run on drifted plans, calls = %d", env.executor.calls)
	}
	if failed.State != unified.ActionStateFailed {
		t.Fatalf("drift refusal must persist a terminal failed audit, state = %q", failed.State)
	}
}

func TestExecuteFailsClosedWithoutExecutor(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	env.service.Executor = nil
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, "agent:test", ""); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("error = %v, want ErrExecutorUnavailable", err)
	}
}

func TestLifecycleFailsClosedWithoutStore(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))
	env.service.Store = func(string) (Store, error) { return nil, errors.New("db offline") }

	if _, err := env.service.Plan(context.Background(), "default", restartRequest()); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("Plan error = %v, want ErrStoreUnavailable", err)
	}
	if _, err := env.service.Decide(context.Background(), "default", "act_x", unified.ActionApprovalRecord{Outcome: unified.OutcomeApproved}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("Decide error = %v, want ErrStoreUnavailable", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", "act_x", "agent:test", ""); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("Execute error = %v, want ErrStoreUnavailable", err)
	}
}

func TestCapabilitiesUsesSameRegistryResolutionAsPlanning(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	capabilities, err := env.service.Capabilities(context.Background(), "default", "vm:42")
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if len(capabilities) != 1 || capabilities[0].Name != "restart" {
		t.Fatalf("capabilities = %#v", capabilities)
	}

	var notFound *ResourceNotFoundError
	if _, err := env.service.Capabilities(context.Background(), "default", "vm:404"); !errors.As(err, &notFound) {
		t.Fatalf("unknown resource error = %v, want ResourceNotFoundError", err)
	}
}

func TestPlanWithOptionsPersistsOriginAcrossLifecycle(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	origin := &unified.ActionOrigin{
		Surface:         "patrol",
		FindingID:       "finding-9",
		InvestigationID: "inv-9",
		ProposalID:      "prop-9",
	}
	plan, err := env.service.PlanWithOptions(context.Background(), "default", restartRequest(), PlanOptions{Origin: origin})
	if err != nil {
		t.Fatalf("PlanWithOptions: %v", err)
	}

	record, ok, err := env.store.GetActionAudit(plan.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if record.Origin == nil || record.Origin.FindingID != "finding-9" {
		t.Fatalf("plan audit origin = %#v", record.Origin)
	}

	// Origin must survive the decision transition: the decision persists
	// the record loaded from the store, so the broker-owned metadata
	// stays reconcilable at approval time.
	updated, err := env.service.Decide(context.Background(), "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if updated.Origin == nil || updated.Origin.ProposalID != "prop-9" {
		t.Fatalf("decision record origin = %#v", updated.Origin)
	}

	// Plain Plan must never stamp an origin.
	plain := restartRequest()
	plain.RequestID = "req-plain"
	plainPlan, err := env.service.Plan(context.Background(), "default", plain)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	plainRecord, ok, err := env.store.GetActionAudit(plainPlan.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit(plain): ok=%v err=%v", ok, err)
	}
	if plainRecord.Origin != nil {
		t.Fatalf("plain plan must not carry an origin, got %#v", plainRecord.Origin)
	}
}

func TestOnActionTransitionFiresAfterEachPersistedState(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))
	var transitions []unified.ActionState
	env.service.OnActionTransition = func(record unified.ActionAuditRecord) {
		transitions = append(transitions, record.State)
	}

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := env.service.Decide(context.Background(), "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, "operator@example.com", ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := []unified.ActionState{unified.ActionStatePending, unified.ActionStateApproved, unified.ActionStateCompleted}
	if len(transitions) != len(want) {
		t.Fatalf("transitions = %v, want %v", transitions, want)
	}
	for i := range want {
		if transitions[i] != want[i] {
			t.Fatalf("transitions = %v, want %v", transitions, want)
		}
	}
}

func TestOnActionTransitionFiresForPersistedRefusals(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))
	var transitions []unified.ActionState
	env.service.OnActionTransition = func(record unified.ActionAuditRecord) {
		transitions = append(transitions, record.State)
	}

	plan, err := env.service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := env.store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:42",
		NeverAutoRemediate: true,
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, "agent:test", ""); !errors.Is(err, unified.ErrResourceRemediationLocked) {
		t.Fatalf("error = %v, want ErrResourceRemediationLocked", err)
	}

	want := []unified.ActionState{unified.ActionStatePlanned, unified.ActionStateFailed}
	if len(transitions) != len(want) || transitions[0] != want[0] || transitions[1] != want[1] {
		t.Fatalf("transitions = %v, want %v", transitions, want)
	}
}
