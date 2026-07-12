package actionlifecycle

import (
	"context"
	"errors"
	"strings"
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

type claimBoundaryStore struct {
	unified.ResourceStore
	entered chan struct{}
	release chan struct{}
	once    atomic.Bool
}

func (s *claimBoundaryStore) ClaimActionDispatch(actionID, owner string, now time.Time, lease time.Duration) (unified.ActionDispatchAttempt, bool, error) {
	attempt, claimed, err := s.ResourceStore.ClaimActionDispatch(actionID, owner, now, lease)
	if claimed && s.once.CompareAndSwap(false, true) {
		close(s.entered)
		<-s.release
	}
	return attempt, claimed, err
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
		Registry:            func(string) (*unified.ResourceRegistry, error) { return registry, nil },
		Store:               func(string) (Store, error) { return store, nil },
		Executor:            executor,
		DecisionAuthorizer:  DecisionAuthorizerFunc(func(context.Context, string, unified.ActionAuditRecord, unified.ActionDecision) error { return nil }),
		ExecutionAuthorizer: ExecutionAuthorizerFunc(func(context.Context, string, unified.ActionAuditRecord, unified.ActionActor) error { return nil }),
	}
}

func testActionActor(subject, orgID string) unified.ActionActor {
	if strings.TrimSpace(subject) == "" {
		subject = "operator"
	}
	return unified.ActionActor{SubjectID: subject, Kind: unified.ActionActorUser, CredentialID: "session:test", OrgID: orgID}
}

func testActionDecision(t *testing.T, service *Service, orgID, actionID string, approval unified.ActionApprovalRecord) unified.ActionDecision {
	t.Helper()
	record, found, _ := service.Get(orgID, actionID)
	planHash := "sha256:test"
	if found {
		planHash = record.Plan.PlanHash
	}
	actor := testActionActor(approval.Actor, orgID)
	return unified.ActionDecision{
		Actor:   actor,
		Outcome: approval.Outcome,
		Reason:  approval.Reason,
		Evidence: unified.ApprovalEvidence{
			Version: 1, Method: unified.MethodSession, Actor: actor, OrgID: orgID,
			ActionID: actionID, PlanHash: planHash, Outcome: approval.Outcome, IssuedAt: time.Now().UTC(),
		},
	}
}

func testBoundLifecycleApproval(record unified.ActionAuditRecord, subject string, method unified.ApprovalMethod, outcome unified.ApprovalOutcome, at time.Time) unified.ActionApprovalRecord {
	kind := unified.ActionActorUser
	credential := "session:test"
	if method == unified.MethodPolicy {
		kind = unified.ActionActorPolicy
		credential = "policy:test"
	}
	actor := unified.ActionActor{SubjectID: subject, Kind: kind, CredentialID: credential, OrgID: "default"}
	evidence := unified.ApprovalEvidence{Version: 1, Method: method, Actor: actor, OrgID: "default", ActionID: record.ID, PlanHash: record.Plan.PlanHash, Outcome: outcome, IssuedAt: at}
	return unified.ActionApprovalRecord{Actor: subject, ActorBinding: actor, Method: method, Timestamp: at, Outcome: outcome, Evidence: &evidence}
}

func runConcurrentPlanReplayCannotRewindTerminalAction(t *testing.T, store unified.ResourceStore) {
	t.Helper()
	delayed := &delayedCreateStore{ResourceStore: store, secondArrived: make(chan struct{}), releaseSecond: make(chan struct{})}
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, delayed, testResource(time.Now().UTC(), unified.ApprovalAdmin), executor)
	firstPlan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	secondResult := make(chan error, 1)
	go func() {
		_, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
		secondResult <- err
	}()
	<-delayed.secondArrived
	if _, err := service.Decide(context.Background(), "default", firstPlan.ActionID, testActionDecision(t, service, "default", firstPlan.ActionID, unified.ActionApprovalRecord{Actor: "operator", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved})); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", firstPlan.ActionID, testActionActor("operator", "default"), "proof"); err != nil {
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
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan error, 1)
	go func() {
		_, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "first")
		first <- err
	}()
	<-executor.entered
	secondRecord, secondErr := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "second")
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

func runConcurrentDuplicateExecuteAtClaimBoundaryCallsExecutorOnce(t *testing.T, store unified.ResourceStore) {
	t.Helper()
	blocked := &claimBoundaryStore{ResourceStore: store, entered: make(chan struct{}), release: make(chan struct{})}
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, blocked, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan error, 1)
	go func() {
		_, executeErr := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "first")
		first <- executeErr
	}()
	<-blocked.entered
	second, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "duplicate")
	if err != nil || second.State != unified.ActionStateExecuting {
		t.Fatalf("duplicate=%#v err=%v", second, err)
	}
	close(blocked.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls=%d, want 1", executor.calls)
	}
}

func TestMemoryStoreConcurrentDuplicateExecuteAtClaimBoundaryCallsExecutorOnce(t *testing.T) {
	runConcurrentDuplicateExecuteAtClaimBoundaryCallsExecutorOnce(t, unified.NewMemoryStore())
}

func TestSQLiteConcurrentDuplicateExecuteAtClaimBoundaryCallsExecutorOnce(t *testing.T) {
	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runConcurrentDuplicateExecuteAtClaimBoundaryCallsExecutorOnce(t, store)
}

func TestPlanReplayReturnsAuthoritativeApprovedExecutingAndTerminalRecords(t *testing.T) {
	states := []unified.ActionState{unified.ActionStateApproved, unified.ActionStateExecuting, unified.ActionStateRejected, unified.ActionStateCompleted, unified.ActionStateFailed}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			store := unified.NewMemoryStore()
			service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
			plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
			if err != nil {
				t.Fatal(err)
			}
			record, _, _ := store.GetActionAudit(plan.ActionID)
			decisionOutcome := unified.OutcomeApproved
			if state == unified.ActionStateRejected {
				decisionOutcome = unified.OutcomeRejected
			}
			decisionAt := time.Now().UTC()
			decided, decisionEvent, err := unified.ApplyActionDecision(record, testBoundLifecycleApproval(record, "operator", unified.MethodSession, decisionOutcome, decisionAt), decisionAt)
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
			returned, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
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
	first := PlanOptions{Actor: testActionActor("requester", "default"), Origin: &unified.ActionOrigin{Surface: "patrol", FindingID: "finding-1", InvestigationID: "inv-1", ProposalID: "proposal-1"}}
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
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), ""); err != nil {
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
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
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
	current, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "retry")
	if err != nil || current.State != unified.ActionStateExecuting || executor.calls != 0 {
		t.Fatalf("current=%#v err=%v calls=%d", current, err, executor.calls)
	}
}

func TestExecuteTimeoutAfterSendWaitsForCorrelatedLateResponseWithoutResend(t *testing.T) {
	store := unified.NewMemoryStore()
	executor := &reconcilingExecutor{executeErr: context.DeadlineExceeded, result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "timeout proof")
	if !errors.Is(err, context.DeadlineExceeded) || current.State != unified.ActionStateExecuting {
		t.Fatalf("current=%#v err=%v", current, err)
	}
	attempt, found, err := store.GetActionDispatchAttempt(plan.ActionID)
	if err != nil || !found || attempt.State != unified.ActionDispatchReceiptPending || attempt.DispatchCount != 1 {
		t.Fatalf("attempt=%#v found=%v err=%v", attempt, found, err)
	}
	if _, found, err := store.GetActionDispatchReceipt(attempt.ID); err != nil || found {
		t.Fatalf("receipt found=%v err=%v", found, err)
	}
	// Duplicate execute is a query-only resume and cannot call ExecuteAction.
	current, err = service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "duplicate resume")
	if err != nil || current.State != unified.ActionStateExecuting || executor.executeCalls != 1 || executor.reconcileCalls != 1 {
		t.Fatalf("current=%#v err=%v execute=%d reconcile=%d", current, err, executor.executeCalls, executor.reconcileCalls)
	}
	executor.found = true
	current, err = service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "late response")
	if err != nil || current.State != unified.ActionStateCompleted || executor.executeCalls != 1 || executor.reconcileCalls != 2 {
		t.Fatalf("current=%#v err=%v execute=%d reconcile=%d", current, err, executor.executeCalls, executor.reconcileCalls)
	}
	if receipt, found, err := store.GetActionDispatchReceipt(attempt.ID); err != nil || !found || receipt.AttemptID != attempt.ID {
		t.Fatalf("receipt=%#v found=%v err=%v", receipt, found, err)
	}
}

func TestSQLiteRestartRecoveryReconcilesReceiptPendingWithoutResend(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	first, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	executor := &reconcilingExecutor{executeErr: context.DeadlineExceeded, result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, first, testResource(now, unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "before restart"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Execute error=%v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	executor.found = true
	service = serviceForStore(t, second, testResource(now, unified.ApprovalNone), executor)
	recovered, err := service.RecoverExecutingActions(context.Background(), "default", "system:restart-recovery", 100)
	if err != nil || len(recovered) != 1 || recovered[0].State != unified.ActionStateCompleted {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
	if executor.executeCalls != 1 || executor.reconcileCalls != 1 {
		t.Fatalf("execute=%d reconcile=%d", executor.executeCalls, executor.reconcileCalls)
	}
	current, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "duplicate after recovery")
	if err != nil || current.State != unified.ActionStateCompleted || executor.executeCalls != 1 {
		t.Fatalf("current=%#v err=%v execute=%d", current, err, executor.executeCalls)
	}
}

func TestSQLiteRestartRecoveryNotFoundRemainsReceiptPendingWithoutResend(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	store, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	executor := &reconcilingExecutor{executeErr: context.DeadlineExceeded, found: false}
	service := serviceForStore(t, store, testResource(now, unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "initial"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("execute err=%v", err)
	}
	for n := 0; n < 2; n++ {
		recovered, err := service.RecoverExecutingActions(context.Background(), "default", "system:restart-recovery", 100)
		if err != nil || len(recovered) != 1 || recovered[0].State != unified.ActionStateExecuting {
			t.Fatalf("pass=%d recovered=%#v err=%v", n, recovered, err)
		}
	}
	attempt, found, err := store.GetActionDispatchAttempt(plan.ActionID)
	if err != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("attempt=%#v found=%v err=%v", attempt, found, err)
	}
	if _, found, err := store.GetActionDispatchReceipt(attempt.ID); err != nil || found {
		t.Fatalf("receipt found=%v err=%v", found, err)
	}
	audit, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || audit.State != unified.ActionStateExecuting || audit.Result != nil {
		t.Fatalf("audit=%#v found=%v err=%v", audit, found, err)
	}
	if executor.executeCalls != 1 || executor.reconcileCalls != 2 {
		t.Fatalf("execute=%d reconcile=%d", executor.executeCalls, executor.reconcileCalls)
	}
}

func TestActionDetailAndInboxMaterializeExplicitExpiry(t *testing.T) {
	now := time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC)
	store := unified.NewMemoryStore()
	service := serviceForStore(t, store, testResource(now, unified.ApprovalAdmin), &stubExecutor{})
	service.Now = func() time.Time { return now }
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	service.Now = func() time.Time { return plan.ExpiresAt.Add(time.Second) }
	detail, found, err := service.Detail("default", plan.ActionID)
	if err != nil || !found || detail.Audit.State != unified.ActionStateExpired {
		t.Fatalf("detail=%#v found=%v err=%v", detail, found, err)
	}
	if len(detail.Events) == 0 || detail.Events[0].State != unified.ActionStateExpired {
		t.Fatalf("events=%#v", detail.Events)
	}
	pending, err := service.List("default", ActionListPending, 100)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending=%#v err=%v", pending, err)
	}
	settled, err := service.List("default", ActionListSettled, 100)
	if err != nil || len(settled) != 1 || settled[0].State != unified.ActionStateExpired {
		t.Fatalf("settled=%#v err=%v", settled, err)
	}
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, service, "default", plan.ActionID, unified.ActionApprovalRecord{Actor: "operator", Outcome: unified.OutcomeApproved})); !errors.Is(err, unified.ErrActionPlanExpired) {
		t.Fatalf("Decide error=%v", err)
	}
}

func policyTestLease(record unified.ActionAuditRecord, now time.Time) unified.ActionPolicyAuthorizationLease {
	lease := unified.ActionPolicyAuthorizationLease{
		Version: 1, OrgID: "default", ActionID: record.ID, ResourceID: record.Request.ResourceID,
		CapabilityName: record.Request.CapabilityName, PlanHash: record.Plan.PlanHash,
		CapabilityPolicyVersion: record.Plan.PolicyVersion, AutoAuthorization: unified.AutoAuthorizeLowRisk,
		ApprovalPolicy: record.Plan.ApprovalPolicy, TenantPolicyVersion: "tenant:v1",
		EffectiveAutonomyLevel: "assisted", LicenseAllowsAutoFix: true,
		ResourcePolicyVersion: "resource:v1", CapabilityNames: []string{"restart"},
		IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	lease.Digest = unified.ActionPolicyAuthorizationDigest(lease)
	return lease
}

func runPolicyAdmissionCommitsAtomically(t *testing.T, store unified.ResourceStore) {
	now := time.Now().UTC()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, store, testResource(now, unified.ApprovalAdmin), executor)
	service.Now = func() time.Time { return now }
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteUnderPolicy(context.Background(), "default", plan.ActionID, "pulse_patrol_policy", func(_ context.Context, record unified.ActionAuditRecord, at time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
		return policyTestLease(record, at), "bounded policy", nil
	})
	if err != nil || completed.State != unified.ActionStateCompleted || executor.calls != 1 {
		t.Fatalf("completed=%#v err=%v calls=%d", completed, err, executor.calls)
	}
	if len(completed.Approvals) != 1 || completed.Approvals[0].Method != unified.MethodPolicy || completed.Approvals[0].PolicyLease == nil {
		t.Fatalf("approvals=%#v", completed.Approvals)
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	seenApproved, seenExecuting := false, false
	for _, event := range events {
		seenApproved = seenApproved || event.State == unified.ActionStateApproved
		seenExecuting = seenExecuting || event.State == unified.ActionStateExecuting
	}
	if !seenApproved || !seenExecuting {
		t.Fatalf("events=%#v", events)
	}
}

func TestPolicyAdmissionCommitsApprovalAndExecutingAtomicallyMemoryStore(t *testing.T) {
	runPolicyAdmissionCommitsAtomically(t, unified.NewMemoryStore())
}
func TestPolicyAdmissionCommitsApprovalAndExecutingAtomicallySQLite(t *testing.T) {
	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runPolicyAdmissionCommitsAtomically(t, store)
}

func runPolicyBarrierRevocations(t *testing.T, storeFactory func(t *testing.T) unified.ResourceStore) {
	cases := []struct {
		name   string
		mutate func(*unified.ActionPolicyAuthorizationLease) error
	}{
		{"policy_row_deleted", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationInvalid
		}},
		{"policy_disabled", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"capability_removed_from_allowlist", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"recurring_window_closed", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"mode_downgraded_to_approval", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"mode_downgraded_to_monitor", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"full_mode_unlock_removed", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"license_removes_effective_mode", func(lease *unified.ActionPolicyAuthorizationLease) error {
			lease.LicenseAllowsAutoFix = false
			lease.Digest = unified.ActionPolicyAuthorizationDigest(*lease)
			return nil
		}},
		{"capability_removed", func(*unified.ActionPolicyAuthorizationLease) error { return unified.ErrActionPlanDrift }},
		{"safety_class_changed_to_never", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"approval_floor_changed_to_dry_run", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"approval_floor_changed_to_mfa", func(*unified.ActionPolicyAuthorizationLease) error {
			return unified.ErrActionPolicyAuthorizationRevoked
		}},
		{"never_auto_remediate_enabled", func(*unified.ActionPolicyAuthorizationLease) error { return unified.ErrResourceRemediationLocked }},
		{"emergency_stop_enabled", func(lease *unified.ActionPolicyAuthorizationLease) error {
			lease.EmergencyStop = true
			lease.Digest = unified.ActionPolicyAuthorizationDigest(*lease)
			return nil
		}},
		{"policy_store_unreadable", func(*unified.ActionPolicyAuthorizationLease) error { return errors.New("policy store unavailable") }},
		{"lease_wrong_org", func(lease *unified.ActionPolicyAuthorizationLease) error {
			lease.OrgID = "other"
			lease.Digest = unified.ActionPolicyAuthorizationDigest(*lease)
			return nil
		}},
		{"lease_expired", func(lease *unified.ActionPolicyAuthorizationLease) error {
			lease.ExpiresAt = lease.IssuedAt
			lease.Digest = unified.ActionPolicyAuthorizationDigest(*lease)
			return nil
		}},
		{"lease_digest_tampered", func(lease *unified.ActionPolicyAuthorizationLease) error {
			lease.Digest = "sha256:tampered"
			return nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := storeFactory(t)
			now := time.Now().UTC()
			executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
			service := serviceForStore(t, store, testResource(now, unified.ApprovalAdmin), executor)
			service.Now = func() time.Time { return now }
			plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
			if err != nil {
				t.Fatal(err)
			}
			failed, err := service.ExecuteUnderPolicy(context.Background(), "default", plan.ActionID, "pulse_patrol_policy", func(_ context.Context, record unified.ActionAuditRecord, at time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
				lease := policyTestLease(record, at)
				if mutationErr := tc.mutate(&lease); mutationErr != nil {
					return unified.ActionPolicyAuthorizationLease{}, "", mutationErr
				}
				return lease, "policy", nil
			})
			if err == nil || failed.State != unified.ActionStateFailed || executor.calls != 0 {
				t.Fatalf("failed=%#v err=%v calls=%d", failed, err, executor.calls)
			}
			if len(failed.Approvals) != 0 {
				t.Fatalf("policy approval persisted: %#v", failed.Approvals)
			}
			events, _ := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 20)
			for _, event := range events {
				if event.State == unified.ActionStateExecuting {
					t.Fatalf("executing event persisted: %#v", events)
				}
			}
		})
	}
}

func TestExecuteUnderPolicyBarrierRevocationsMemoryStore(t *testing.T) {
	runPolicyBarrierRevocations(t, func(*testing.T) unified.ResourceStore { return unified.NewMemoryStore() })
}
func TestExecuteUnderPolicyBarrierRevocationsSQLite(t *testing.T) {
	runPolicyBarrierRevocations(t, func(t *testing.T) unified.ResourceStore {
		store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		return store
	})
}

func runHumanApprovalSurvivesPolicyRevocation(t *testing.T, store unified.ResourceStore) {
	now := time.Now().UTC()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, store, testResource(now, unified.ApprovalAdmin), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, service, "default", plan.ActionID, unified.ActionApprovalRecord{Actor: "operator", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved})); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), ""); err != nil || executor.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, executor.calls)
	}
}
func TestHumanApprovalSurvivesAutomaticPolicyRevocationMemoryStore(t *testing.T) {
	runHumanApprovalSurvivesPolicyRevocation(t, unified.NewMemoryStore())
}
func TestHumanApprovalSurvivesAutomaticPolicyRevocationSQLite(t *testing.T) {
	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runHumanApprovalSurvivesPolicyRevocation(t, store)
}

func runEmergencyStopBlocksHumanAndPolicy(t *testing.T, store unified.ResourceStore) {
	now := time.Now().UTC()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, store, testResource(now, unified.ApprovalAdmin), executor)
	service.EmergencyStop = func(string) (bool, error) { return true, nil }
	human := restartRequest()
	human.RequestID = "human-stop"
	plan, err := service.Plan(context.Background(), "default", human, testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, service, "default", plan.ActionID, unified.ActionApprovalRecord{Actor: "operator", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved})); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), ""); !errors.Is(err, unified.ErrActionEmergencyStop) {
		t.Fatalf("human error=%v", err)
	}
	policy := restartRequest()
	policy.RequestID = "policy-stop"
	policyPlan, err := service.Plan(context.Background(), "default", policy, testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ExecuteUnderPolicy(context.Background(), "default", policyPlan.ActionID, "pulse_patrol_policy", func(_ context.Context, record unified.ActionAuditRecord, at time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
		return policyTestLease(record, at), "policy", nil
	}); !errors.Is(err, unified.ErrActionEmergencyStop) {
		t.Fatalf("policy error=%v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls=%d", executor.calls)
	}
}
func TestEmergencyStopBlocksHumanAndPolicyAdmissionMemoryStore(t *testing.T) {
	runEmergencyStopBlocksHumanAndPolicy(t, unified.NewMemoryStore())
}
func TestEmergencyStopBlocksHumanAndPolicyAdmissionSQLite(t *testing.T) {
	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runEmergencyStopBlocksHumanAndPolicy(t, store)
}

func TestLegacyPolicyApprovalWithoutLeaseFailsClosedAfterRestart(t *testing.T) {
	dir := t.TempDir()
	first, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := restartRequest()
	request.Actor = testActionActor("requester", "default")
	request.RequestedBy = request.Actor.SubjectID
	plan, err := (actionplanner.Planner{Now: func() time.Time { return now }}).Plan(request, testResource(now, unified.ApprovalAdmin))
	if err != nil {
		t.Fatal(err)
	}
	legacyApproved := unified.ActionAuditRecord{
		ID: plan.ActionID, CreatedAt: now, UpdatedAt: now, State: unified.ActionStateApproved,
		Request: request, Plan: plan,
		Approvals: []unified.ActionApprovalRecord{{Actor: "pulse_patrol_policy", Method: unified.MethodPolicy, Timestamp: now, Outcome: unified.OutcomeApproved}},
	}
	if err := first.RecordActionAudit(legacyApproved); err != nil {
		t.Fatalf("legacy audit: %v", err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service := serviceForStore(t, second, testResource(now, unified.ApprovalAdmin), executor)
	failed, err := service.ExecuteUnderPolicy(context.Background(), "default", plan.ActionID, "pulse_patrol_policy", func(_ context.Context, record unified.ActionAuditRecord, at time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
		return policyTestLease(record, at), "policy", nil
	})
	if !errors.Is(err, unified.ErrActionPolicyAuthorizationInvalid) || failed.State != unified.ActionStateFailed || executor.calls != 0 {
		t.Fatalf("failed=%#v err=%v calls=%d", failed, err, executor.calls)
	}
}

func TestQueuedPolicyActionRevalidatesAfterSQLiteRestart(t *testing.T) {
	dir := t.TempDir()
	first, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	service := serviceForStore(t, first, testResource(now, unified.ApprovalAdmin), &stubExecutor{})
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}}
	service = serviceForStore(t, second, testResource(now, unified.ApprovalAdmin), executor)
	failed, err := service.ExecuteUnderPolicy(context.Background(), "default", plan.ActionID, "pulse_patrol_policy", func(context.Context, unified.ActionAuditRecord, time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationRevoked
	})
	if !errors.Is(err, unified.ErrActionPolicyAuthorizationRevoked) || failed.State != unified.ActionStateFailed || executor.calls != 0 {
		t.Fatalf("failed=%#v err=%v calls=%d", failed, err, executor.calls)
	}
}

type stubExecutor struct {
	result    *unified.ExecutionResult
	err       error
	calls     int
	received  unified.ActionAuditRecord
	readiness *unified.ResourceActionReadiness
}

type reconcilingExecutor struct {
	executeCalls   int
	reconcileCalls int
	executeErr     error
	found          bool
	result         *unified.ExecutionResult
}
type bindingReconcilingExecutor struct{ reconcilingExecutor }

func (e *bindingReconcilingExecutor) BindActionDispatch(_ context.Context, _ unified.ActionAuditRecord, a unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	return a, nil
}

func (e *reconcilingExecutor) ExecuteAction(_ context.Context, _ unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	e.executeCalls++
	return nil, e.executeErr
}

func (e *reconcilingExecutor) ReconcileActionDispatch(_ context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	e.reconcileCalls++
	if !e.found {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	return e.result, unified.ActionDispatchReceipt{
		AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: time.Now().UTC(),
	}, true, nil
}

func TestLegacyUnboundReceiptPendingAttemptIsInertForBindingOwnedExecutor(t *testing.T) {
	now := time.Now().UTC()
	store := unified.NewMemoryStore()
	executor := &bindingReconcilingExecutor{}
	service := serviceForStore(t, store, testResource(now, unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found {
		t.Fatal(err)
	}
	started, event, err := unified.BeginActionExecution(record, "operator", now)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := unified.NewActionDispatchAttempt(started.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordActionExecutionAdmission(started, event, attempt); err != nil {
		t.Fatal(err)
	}
	claimed, won, err := store.ClaimActionDispatch(started.ID, "legacy-worker", now, time.Minute)
	if err != nil || !won {
		t.Fatalf("claim=%#v won=%v err=%v", claimed, won, err)
	}
	if _, err := store.MarkActionDispatchStarted(claimed.ID, "legacy-worker", now); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.RecoverExecutingActions(context.Background(), "default", "system:recovery", 10)
	if err != nil || len(recovered) != 1 || recovered[0].State != unified.ActionStateExecuting {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
	if executor.executeCalls != 0 || executor.reconcileCalls != 0 {
		t.Fatalf("execute=%d reconcile=%d", executor.executeCalls, executor.reconcileCalls)
	}
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
		Executor:            env.executor,
		DecisionAuthorizer:  DecisionAuthorizerFunc(func(context.Context, string, unified.ActionAuditRecord, unified.ActionDecision) error { return nil }),
		ExecutionAuthorizer: ExecutionAuthorizerFunc(func(context.Context, string, unified.ActionAuditRecord, unified.ActionActor) error { return nil }),
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
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
	if _, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default")); err != nil {
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
	if _, err := env.service.Plan(context.Background(), "default", missing, testActionActor("requester", "default")); !errors.As(err, &notFound) {
		t.Fatalf("unknown resource error = %v, want ResourceNotFoundError", err)
	}

	unknownCap := restartRequest()
	unknownCap.CapabilityName = "detonate"
	_, err := env.service.Plan(context.Background(), "default", unknownCap, testActionActor("requester", "default"))
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
	if _, err := env.service.Plan(context.Background(), "default", empty, testActionActor("requester", "default")); !errors.As(err, &validation) {
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

	_, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	updated, err := env.service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, env.service, "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor:   "operator@example.com",
		Method:  unified.MethodAPI,
		Outcome: unified.OutcomeApproved,
		Reason:  "confirmed outage",
	}))

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
	if _, err := env.service.Decide(context.Background(), "default", "act_missing", testActionDecision(t, env.service, "default", "act_missing", unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	})); !errors.As(err, &notFound) {
		t.Fatalf("unknown action error = %v, want ActionNotFoundError", err)
	}
}

func TestExecuteRunsApprovedActionToTerminalAudit(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := env.service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, env.service, "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	})); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	completed, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator@example.com", "default"), "approved restart")
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator@example.com", "default"), ""); !errors.Is(err, unified.ErrActionNotApproved) {
		t.Fatalf("error = %v, want ErrActionNotApproved", err)
	}
	if env.executor.calls != 0 {
		t.Fatalf("executor must not run for unapproved actions, calls = %d", env.executor.calls)
	}
}

func TestExecuteRefusesRemediationLockedResource(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := env.store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:42",
		NeverAutoRemediate: true,
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	failed, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("agent:test", "default"), "")
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// The capability policy tightens between plan and dispatch: the
	// replanned contract no longer matches the recorded plan hash.
	drifted := testResource(now, unified.ApprovalAdmin)
	env.registry = unified.NewRegistry(env.store)
	env.registry.IngestResources([]unified.Resource{drifted})

	failed, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("agent:test", "default"), "")
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	env.service.Executor = nil
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("agent:test", "default"), ""); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("error = %v, want ErrExecutorUnavailable", err)
	}
}

func TestLifecycleFailsClosedWithoutStore(t *testing.T) {
	now := time.Now().UTC()
	env := newServiceEnv(t, testResource(now, unified.ApprovalNone))
	env.service.Store = func(string) (Store, error) { return nil, errors.New("db offline") }

	if _, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default")); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("Plan error = %v, want ErrStoreUnavailable", err)
	}
	if _, err := env.service.Decide(context.Background(), "default", "act_x", testActionDecision(t, env.service, "default", "act_x", unified.ActionApprovalRecord{Outcome: unified.OutcomeApproved})); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("Decide error = %v, want ErrStoreUnavailable", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", "act_x", testActionActor("agent:test", "default"), ""); !errors.Is(err, ErrStoreUnavailable) {
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
	plan, err := env.service.PlanWithOptions(context.Background(), "default", restartRequest(), PlanOptions{Actor: testActionActor("requester", "default"), Origin: origin})
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
	updated, err := env.service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, env.service, "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	}))

	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if updated.Origin == nil || updated.Origin.ProposalID != "prop-9" {
		t.Fatalf("decision record origin = %#v", updated.Origin)
	}

	// Plain Plan must never stamp an origin.
	plain := restartRequest()
	plain.RequestID = "req-plain"
	plainPlan, err := env.service.Plan(context.Background(), "default", plain, testActionActor("requester", "default"))
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := env.service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, env.service, "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "operator@example.com", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved,
	})); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator@example.com", "default"), ""); err != nil {
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

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := env.store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:42",
		NeverAutoRemediate: true,
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("agent:test", "default"), ""); !errors.Is(err, unified.ErrResourceRemediationLocked) {
		t.Fatalf("error = %v, want ErrResourceRemediationLocked", err)
	}

	want := []unified.ActionState{unified.ActionStatePlanned, unified.ActionStateFailed}
	if len(transitions) != len(want) || transitions[0] != want[0] || transitions[1] != want[1] {
		t.Fatalf("transitions = %v, want %v", transitions, want)
	}
}

func TestActionPolicyProvenanceParticipatesInFreshnessWithoutAuthorizingDispatch(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 0, 0, 0, time.UTC)
	env := newServiceEnv(t, testResource(now, unified.ApprovalAdmin))
	factor := unified.ActionPolicyAuthorityFactor{
		Kind: unified.ActionPolicyAuthorityTenant, SourceID: "patrol-tenant-policy",
		Revision: "tenant-policy:sha256:0123456789abcdef01234567", Status: unified.ActionPolicyAuthorityConsulted,
		ReasonCodes: []unified.ActionPolicyReasonCode{unified.PolicyReasonTenantModeAssisted, unified.PolicyReasonTenantFullLocked},
	}
	plan, err := env.service.PlanWithOptions(context.Background(), "default", restartRequest(), PlanOptions{Actor: testActionActor("requester", "default"), PolicyFactors: []unified.ActionPolicyAuthorityFactor{factor}})
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := env.service.Get("default", plan.ActionID)
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if err := env.service.ValidatePlanFresh("default", record); err != nil {
		t.Fatalf("captured descriptive snapshot should replan deterministically: %v", err)
	}
	record.Plan.PolicyDecision.Authorities[1].Revision = "tenant-policy:sha256:1123456789abcdef01234567"
	record.Plan.PolicyDecision.DecisionID = unified.ActionPolicyDecisionDigest(record.Plan.PolicyDecision)
	if err := env.service.ValidatePlanFresh("default", record); !errors.Is(err, unified.ErrActionPlanDrift) {
		t.Fatalf("provenance drift error=%v", err)
	}
	if _, _, _, err := unified.BeginPolicyActionExecution(record, unified.ActionApprovalRecord{Actor: "pulse_patrol_policy", Method: unified.MethodPolicy, Outcome: unified.OutcomeApproved, Timestamp: now}, unified.ActionPolicyAuthorizationLease{}, now); !errors.Is(err, unified.ErrActionPolicyAuthorizationInvalid) {
		t.Fatalf("descriptive provenance authorized dispatch: %v", err)
	}
}
