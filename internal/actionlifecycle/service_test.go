package actionlifecycle

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type delayedCreateStore struct {
	unified.ResourceStore
	calls         atomic.Int32
	secondArrived chan struct{}
	releaseSecond chan struct{}
}

func (s *delayedCreateStore) CreateActionAudit(record unified.ActionAuditRecord, events []unified.ActionLifecycleEvent) (unified.ActionAuditRecord, bool, error) {
	if s.calls.Add(1) == 2 {
		close(s.secondArrived)
		<-s.releaseSecond
	}
	return s.ResourceStore.CreateActionAudit(record, events)
}

type blockingExecutor struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (e *blockingExecutor) ExecuteAction(_ context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	if e.calls.Add(1) == 1 {
		close(e.entered)
	}
	<-e.release
	return &unified.ExecutionResult{Success: true, Output: record.ID}, nil
}

func serviceForStore(t *testing.T, store unified.ResourceStore, resource unified.Resource, executor Executor) *Service {
	t.Helper()
	registry := unified.NewRegistry(store)
	registry.IngestResources([]unified.Resource{resource})
	return &Service{
		Registry: func(string) (*unified.ResourceRegistry, error) { return registry, nil },
		Store:    func(string) (Store, error) { return store, nil },
		Executor: executor,
	}
}

func runConcurrentPlanReplayCannotRewindTerminalAction(t *testing.T, store unified.ResourceStore) {
	t.Helper()
	delayed := &delayedCreateStore{ResourceStore: store, secondArrived: make(chan struct{}), releaseSecond: make(chan struct{})}
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, delayed, testResource(time.Now().UTC(), unified.ApprovalAdmin), executor)
	firstPlan, err := service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatal(err)
	}
	secondResult := make(chan error, 1)
	go func() { _, err := service.Plan(context.Background(), "default", restartRequest()); secondResult <- err }()
	<-delayed.secondArrived
	if _, err := service.Decide(context.Background(), "default", firstPlan.ActionID, unified.ActionApprovalRecord{Actor: "operator", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", firstPlan.ActionID, "operator", "proof"); err != nil {
		t.Fatal(err)
	}
	close(delayed.releaseSecond)
	if err := <-secondResult; err != nil {
		t.Fatal(err)
	}
	current, found, err := store.GetActionAudit(firstPlan.ActionID)
	if err != nil || !found || current.State != unified.ActionStateCompleted {
		t.Fatalf("current=%#v found=%v err=%v", current, found, err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls=%d, want 1", executor.calls)
	}
}

func TestConcurrentPlanReplayCannotRewindTerminalActionMemoryStore(t *testing.T) {
	runConcurrentPlanReplayCannotRewindTerminalAction(t, unified.NewMemoryStore())
}

func TestConcurrentPlanReplayCannotRewindTerminalActionSQLiteStore(t *testing.T) {
	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runConcurrentPlanReplayCannotRewindTerminalAction(t, store)
}

func runConcurrentExecuteAdmitsExecutorExactlyOnce(t *testing.T, store unified.ResourceStore) {
	t.Helper()
	executor := &blockingExecutor{entered: make(chan struct{}), release: make(chan struct{})}
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan error, 1)
	go func() {
		_, err := service.Execute(context.Background(), "default", plan.ActionID, "operator", "first")
		first <- err
	}()
	<-executor.entered
	secondRecord, secondErr := service.Execute(context.Background(), "default", plan.ActionID, "operator", "second")
	if secondErr != nil {
		t.Fatalf("duplicate Execute: %v", secondErr)
	}
	if secondRecord.State != unified.ActionStateExecuting {
		t.Fatalf("duplicate state=%q", secondRecord.State)
	}
	close(executor.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if executor.calls.Load() != 1 {
		t.Fatalf("executor calls=%d, want 1", executor.calls.Load())
	}
}

func TestConcurrentExecuteAdmitsExecutorExactlyOnceMemoryStore(t *testing.T) {
	runConcurrentExecuteAdmitsExecutorExactlyOnce(t, unified.NewMemoryStore())
}

func TestConcurrentExecuteAdmitsExecutorExactlyOnceSQLiteStore(t *testing.T) {
	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runConcurrentExecuteAdmitsExecutorExactlyOnce(t, store)
}

func TestPlanReplayReturnsAuthoritativeApprovedExecutingAndTerminalRecords(t *testing.T) {
	states := []unified.ActionState{unified.ActionStateApproved, unified.ActionStateExecuting, unified.ActionStateRejected, unified.ActionStateCompleted, unified.ActionStateFailed}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			store := unified.NewMemoryStore()
			service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
			plan, err := service.Plan(context.Background(), "default", restartRequest())
			if err != nil {
				t.Fatal(err)
			}
			record, _, _ := store.GetActionAudit(plan.ActionID)
			decisionOutcome := unified.OutcomeApproved
			if state == unified.ActionStateRejected {
				decisionOutcome = unified.OutcomeRejected
			}
			decided, decisionEvent, err := unified.ApplyActionDecision(record, unified.ActionApprovalRecord{Actor: "operator", Outcome: decisionOutcome}, time.Now().UTC())
			if err != nil || store.RecordActionDecision(decided, decisionEvent) != nil {
				t.Fatalf("decision: %v", err)
			}
			if state != unified.ActionStateApproved && state != unified.ActionStateRejected {
				started, startEvent, err := unified.BeginActionExecution(decided, "operator", time.Now().UTC())
				if err != nil || store.RecordActionExecutionStart(started, startEvent) != nil {
					t.Fatalf("start: %v", err)
				}
				if state == unified.ActionStateCompleted || state == unified.ActionStateFailed {
					result := &unified.ExecutionResult{Success: state == unified.ActionStateCompleted}
					completed, doneEvent, err := unified.CompleteActionExecution(started, result, "operator", time.Now().UTC())
					if err != nil || store.RecordActionExecutionResult(completed, doneEvent) != nil {
						t.Fatalf("complete: %v", err)
					}
				}
			}
			returned, err := service.Plan(context.Background(), "default", restartRequest())
			if err != nil {
				t.Fatal(err)
			}
			if returned.ActionID != plan.ActionID {
				t.Fatalf("action id=%q", returned.ActionID)
			}
			current, _, _ := store.GetActionAudit(plan.ActionID)
			if current.State != state {
				t.Fatalf("state=%q want=%q", current.State, state)
			}
		})
	}
}

func TestPlanReplayRejectsConflictingOriginForDeterministicActionID(t *testing.T) {
	store := unified.NewMemoryStore()
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
	first := PlanOptions{Origin: &unified.ActionOrigin{Surface: "patrol", FindingID: "finding-1", InvestigationID: "inv-1", ProposalID: "proposal-1"}}
	if _, err := service.PlanWithOptions(context.Background(), "default", restartRequest(), first); err != nil {
		t.Fatal(err)
	}
	conflict := first
	conflict.Origin = &unified.ActionOrigin{Surface: "patrol", FindingID: "finding-2", InvestigationID: "inv-2", ProposalID: "proposal-1"}
	if _, err := service.PlanWithOptions(context.Background(), "default", restartRequest(), conflict); !errors.Is(err, unified.ErrActionIdentityConflict) {
		t.Fatalf("error=%v", err)
	}
}

func TestExecutePublishesPersistedExecutingTransition(t *testing.T) {
	store := unified.NewMemoryStore()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	var states []unified.ActionState
	service.OnActionTransition = func(_ string, record unified.ActionAuditRecord) { states = append(states, record.State) }
	plan, err := service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", plan.ActionID, "operator", ""); err != nil {
		t.Fatal(err)
	}
	want := []unified.ActionState{unified.ActionStatePlanned, unified.ActionStateExecuting, unified.ActionStateCompleted}
	if len(states) != len(want) {
		t.Fatalf("states=%v", states)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("states=%v want=%v", states, want)
		}
	}
}

func TestExecuteAfterSQLiteRestartDoesNotReadmitExecutingAction(t *testing.T) {
	dir := t.TempDir()
	first, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, first, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest())
	if err != nil {
		t.Fatal(err)
	}
	record, _, _ := first.GetActionAudit(plan.ActionID)
	started, event, _ := unified.BeginActionExecution(record, "operator", time.Now().UTC())
	if err := first.RecordActionExecutionStart(started, event); err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	service = serviceForStore(t, second, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	current, err := service.Execute(context.Background(), "default", plan.ActionID, "operator", "retry")
	if err != nil || current.State != unified.ActionStateExecuting || executor.calls != 0 {
		t.Fatalf("current=%#v err=%v calls=%d", current, err, executor.calls)
	}
}

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
	var transitionOrgs []string
	env.service.OnActionTransition = func(orgID string, record unified.ActionAuditRecord) {
		transitions = append(transitions, record.State)
		transitionOrgs = append(transitionOrgs, orgID)
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

	want := []unified.ActionState{unified.ActionStatePending, unified.ActionStateApproved, unified.ActionStateExecuting, unified.ActionStateCompleted}
	if len(transitions) != len(want) {
		t.Fatalf("transitions = %v, want %v", transitions, want)
	}
	for i := range want {
		if transitions[i] != want[i] {
			t.Fatalf("transitions = %v, want %v", transitions, want)
		}
		if transitionOrgs[i] != "default" {
			t.Fatalf("transition org = %q, want default", transitionOrgs[i])
		}
	}
}

func TestOnActionTransitionFiresForPersistedRefusals(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))
	var transitions []unified.ActionState
	env.service.OnActionTransition = func(orgID string, record unified.ActionAuditRecord) {
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
