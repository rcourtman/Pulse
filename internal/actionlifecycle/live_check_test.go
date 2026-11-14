package actionlifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// executeGrant admits the agent check only for subjects who hold the execute
// capability, the way the API authority does under an RBAC grant.
func executeGrant(subjects ...string) LiveCheckAuthorizer {
	return LiveCheckAuthorizerFunc(func(_ context.Context, _ string, actor unified.ActionActor) error {
		for _, subject := range subjects {
			if strings.EqualFold(actor.SubjectID, subject) {
				return nil
			}
		}
		return ErrActionAuthorizationDenied
	})
}

func refusingFeasibility() *unified.ResourceActionReadiness {
	return &unified.ResourceActionReadiness{
		Name: "restart", Available: false, ReasonCode: "target_state_changed",
		Reason: "The target changed before approval.",
	}
}

func TestPlanWithoutExecuteCapabilityDoesNotAskTheAgent(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.service.LiveCheckAuthorizer = executeGrant("runner")
	env.executor.feasibility = refusingFeasibility()

	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("planner", "default"))
	if err != nil {
		t.Fatalf("plan-only caller: Plan error = %v", err)
	}
	if env.executor.feasibilityCalls != 0 {
		t.Fatalf("plan-only caller reached the agent %d times", env.executor.feasibilityCalls)
	}
	if record := mustActionRecord(t, env.service, plan.ActionID); record.State != unified.ActionStatePending {
		t.Fatalf("state = %q, want pending", record.State)
	}

	req := restartRequest()
	req.RequestID = "req-runner"
	_, err = env.service.Plan(context.Background(), "default", req, testActionActor("runner", "default"))
	var refused *AvailabilityRefusedError
	if !errors.As(err, &refused) || refused.Readiness.ReasonCode != "target_state_changed" {
		t.Fatalf("execute grant: error=%v, want the agent refusal", err)
	}
	if env.executor.feasibilityCalls != 1 {
		t.Fatalf("execute grant: agent checks = %d, want 1", env.executor.feasibilityCalls)
	}
}

func TestPlanStillAppliesPulseAvailabilityWithoutExecuteCapability(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.service.LiveCheckAuthorizer = executeGrant()
	env.executor.readiness = &unified.ResourceActionReadiness{Name: "restart", Available: false, ReasonCode: "agent_disconnected"}

	_, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("planner", "default"))
	var refused *AvailabilityRefusedError
	if !errors.As(err, &refused) || refused.Readiness.ReasonCode != "agent_disconnected" {
		t.Fatalf("error=%v, want the Pulse-side availability refusal", err)
	}
}

func TestLiveCheckFailsClosedWithoutAuthorizer(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.service.LiveCheckAuthorizer = nil
	env.executor.feasibility = refusingFeasibility()

	if _, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("planner", "default")); err != nil {
		t.Fatalf("Plan error = %v", err)
	}
	if env.executor.feasibilityCalls != 0 {
		t.Fatalf("agent checks without an authorizer = %d", env.executor.feasibilityCalls)
	}
}

func TestBoundLiveCheckAuthorityWithholdsAgentCheckForBrokers(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.executor.feasibility = refusingFeasibility()
	broker := unified.ActionActor{SubjectID: "pulse_assistant", Kind: unified.ActionActorService, CredentialID: "service:assistant", OrgID: "default"}

	ctx := WithLiveCheckAuthority(context.Background(), false)
	if _, err := env.service.Plan(ctx, "default", restartRequest(), broker); err != nil {
		t.Fatalf("caller without execute: Plan error = %v", err)
	}
	if env.executor.feasibilityCalls != 0 {
		t.Fatalf("caller without execute reached the agent %d times", env.executor.feasibilityCalls)
	}

	req := restartRequest()
	req.RequestID = "req-executor"
	ctx = WithLiveCheckAuthority(context.Background(), true)
	var refused *AvailabilityRefusedError
	if _, err := env.service.Plan(ctx, "default", req, broker); !errors.As(err, &refused) {
		t.Fatalf("caller with execute: error=%v, want the agent refusal", err)
	}
}

func TestApprovalWithoutExecuteCapabilityLeavesTheAgentCheckToDispatch(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.service.LiveCheckAuthorizer = executeGrant("runner")
	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("planner", "default"))
	if err != nil {
		t.Fatal(err)
	}
	env.executor.feasibility = refusingFeasibility()

	readiness := env.service.AssessCurrentReadiness(context.Background(), "default", mustActionRecord(t, env.service, plan.ActionID), testActionActor("approver", "default"))
	if !readiness.Ready || readiness.Message != "Action passed Pulse's checks. The agent checks it again before it runs." {
		t.Fatalf("approve-only readiness = %#v", readiness)
	}
	if env.executor.feasibilityCalls != 0 {
		t.Fatalf("approve-only review reached the agent %d times", env.executor.feasibilityCalls)
	}
	readiness = env.service.AssessCurrentReadiness(context.Background(), "default", mustActionRecord(t, env.service, plan.ActionID), testActionActor("runner", "default"))
	if readiness.Ready || readiness.Code != "target_state_changed" {
		t.Fatalf("execute-grant readiness = %#v", readiness)
	}
	checksBeforeDecision := env.executor.feasibilityCalls

	approved, err := env.service.Decide(context.Background(), "default", plan.ActionID, testActionDecision(t, env.service, "default", plan.ActionID, unified.ActionApprovalRecord{
		Actor: "approver", Outcome: unified.OutcomeApproved,
	}))
	if err != nil || approved.State != unified.ActionStateApproved {
		t.Fatalf("approve-only approval: state=%q err=%v", approved.State, err)
	}
	if env.executor.feasibilityCalls != checksBeforeDecision {
		t.Fatalf("approve-only approval reached the agent")
	}

	// Dispatch always asks the agent, so the infeasible action still cannot run.
	if _, err := env.service.Execute(context.Background(), "default", plan.ActionID, testActionActor("runner", "default"), "run"); !errors.Is(err, unified.ErrActionExecutionUnavailable) {
		t.Fatalf("dispatch error = %v, want the agent refusal", err)
	}
	if env.executor.calls != 0 || env.executor.feasibilityCalls != checksBeforeDecision+1 {
		t.Fatalf("executor calls=%d agent checks=%d", env.executor.calls, env.executor.feasibilityCalls)
	}
}

func TestApprovalWithExecuteCapabilityRechecksTheAgent(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.service.LiveCheckAuthorizer = executeGrant("runner")
	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), testActionActor("planner", "default"))
	if err != nil {
		t.Fatal(err)
	}
	env.executor.feasibility = refusingFeasibility()
	decision := testActionDecision(t, env.service, "default", plan.ActionID, unified.ActionApprovalRecord{Actor: "runner", Outcome: unified.OutcomeApproved})
	if _, err := env.service.Decide(context.Background(), "default", plan.ActionID, decision); !errors.Is(err, unified.ErrActionExecutionUnavailable) {
		t.Fatalf("approval error = %v, want the agent refusal", err)
	}
	if env.executor.feasibilityCalls != 1 {
		t.Fatalf("agent checks = %d, want 1", env.executor.feasibilityCalls)
	}
}

func TestRefreshUsesTheRefreshingCallersAuthorityForBrokerPlans(t *testing.T) {
	env := newServiceEnv(t, testResource(time.Now().UTC(), unified.ApprovalAdmin))
	env.service.LiveCheckAuthorizer = LiveCheckAuthorizerFunc(func(_ context.Context, _ string, actor unified.ActionActor) error {
		if actor.Kind == unified.ActionActorService || actor.SubjectID == "runner" {
			return nil
		}
		return ErrActionAuthorizationDenied
	})
	broker := unified.ActionActor{SubjectID: "patrol", Kind: unified.ActionActorService, CredentialID: "service:patrol", OrgID: "default"}
	env.service.RefreshPlanner = func(_ context.Context, _ string, previous unified.ActionAuditRecord, _ unified.ActionActor, requestID string) (unified.ActionRequest, PlanOptions, error) {
		req := previous.Request
		req.RequestID = requestID
		return req, PlanOptions{Actor: broker, Origin: previous.Origin}, nil
	}
	plan, err := env.service.Plan(context.Background(), "default", restartRequest(), broker)
	if err != nil {
		t.Fatal(err)
	}
	expired := plan.ExpiresAt.Add(time.Second)
	env.service.Now = func() time.Time { return expired }
	env.executor.feasibility = refusingFeasibility()
	checksBefore := env.executor.feasibilityCalls

	replacement, err := env.service.Refresh(context.Background(), "default", plan.ActionID, testActionActor("planner", "default"))
	if err != nil {
		t.Fatalf("plan-only refresh error = %v", err)
	}
	if replacement.ID == plan.ActionID || env.executor.feasibilityCalls != checksBefore {
		t.Fatalf("plan-only refresh: replacement=%q agent checks=%d->%d", replacement.ID, checksBefore, env.executor.feasibilityCalls)
	}
}
