package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// agentCheckingActionExecutor counts the agent checks (FeasibilityChecker
// calls) the lifecycle asks for, standing in for the apt-get -s upgrade or
// container inspect a real executor sends to the agent.
type agentCheckingActionExecutor struct {
	stubActionExecutor
	agentChecks int
	refusal     *unified.ResourceActionReadiness
}

func (e *agentCheckingActionExecutor) CheckActionFeasible(_ context.Context, _ string, _ unified.ActionRequest, _ unified.Resource) unified.ResourceActionReadiness {
	e.agentChecks++
	if e.refusal == nil {
		return unified.ResourceActionReadiness{}
	}
	return *e.refusal
}

func newLiveCheckActionHandlers(t *testing.T, grants actionCapabilityAuthorizer, executor *agentCheckingActionExecutor) *ResourceHandlers {
	t.Helper()
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	authority := actionAuthority{authorizer: grants, orgChecker: NewAuthorizationChecker(nil)}
	h.SetActionAuthorizers(authority, authority, authority)
	h.SetActionExecutor(executor)
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{{
			ID: "vm:42", Type: unified.ResourceTypeVM, Name: "web-42", Status: unified.StatusWarning,
			LastSeen: now, UpdatedAt: now, Sources: []unified.DataSource{unified.SourceProxmox},
			Capabilities: []unified.ResourceCapability{{
				Name: "restart", Type: unified.CapabilityTypeCommon, Description: "Restart the VM",
				MinimumApprovalLevel: unified.ApprovalAdmin, InternalHandler: "proxmox.vm.restart",
				Params: []unified.CapabilityParam{{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}}},
			}},
		}},
	})
	return h
}

func planLiveCheckAction(h *ResourceHandlers, requestID string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"`+requestID+`","resourceId":"vm:42","capabilityName":"restart",
		"params":{"mode":"graceful"},"reason":"Recover after confirmed outage"
	}`))
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, "operator@example.com"))
	return rec
}

// Creating a plan must not make the agent do work unless the caller could
// execute the action. A plan-only RBAC grant gets a plan without the agent
// check; dispatch checks it again under execute.
func TestPlanActionAgentCheckFollowsRBACExecuteGrant(t *testing.T) {
	plan := auth.ActionPlan + "/" + auth.ResourceActions
	execute := auth.ActionExecute + "/" + auth.ResourceActions
	refusal := &unified.ResourceActionReadiness{Name: "restart", Available: false, ReasonCode: "package_inventory_changed", Reason: "The host changed since planning."}

	t.Run("plan only", func(t *testing.T) {
		executor := &agentCheckingActionExecutor{refusal: refusal}
		h := newLiveCheckActionHandlers(t, actionCapabilityAuthorizer{plan: true}, executor)
		rec := planLiveCheckAction(h, "plan-only")
		if rec.Code != http.StatusOK {
			t.Fatalf("plan-only grant: status=%d body=%s", rec.Code, rec.Body.String())
		}
		if executor.agentChecks != 0 {
			t.Fatalf("plan-only grant made the agent check the plan %d times", executor.agentChecks)
		}
	})

	t.Run("plan and execute", func(t *testing.T) {
		executor := &agentCheckingActionExecutor{refusal: refusal}
		h := newLiveCheckActionHandlers(t, actionCapabilityAuthorizer{plan: true, execute: true}, executor)
		rec := planLiveCheckAction(h, "plan-execute")
		if rec.Code == http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("package_inventory_changed")) {
			t.Fatalf("execute grant: status=%d body=%s, want the agent refusal", rec.Code, rec.Body.String())
		}
		if executor.agentChecks != 1 {
			t.Fatalf("execute grant: agent checks = %d, want 1", executor.agentChecks)
		}
	})
}

// Reviewing an action (GET /api/actions/{id} computes readiness) follows the
// same rule: a grant without execute sees Pulse's checks, not the agent's.
func TestActionDetailReadinessAgentCheckFollowsRBACExecuteGrant(t *testing.T) {
	plan := auth.ActionPlan + "/" + auth.ResourceActions
	approve := auth.ActionApprove + "/" + auth.ResourceActions
	execute := auth.ActionExecute + "/" + auth.ResourceActions
	for _, tc := range []struct {
		name       string
		grants     actionCapabilityAuthorizer
		wantChecks int
	}{
		{name: "plan and approve without execute", grants: actionCapabilityAuthorizer{plan: true, approve: true}, wantChecks: 0},
		{name: "plan, approve and execute", grants: actionCapabilityAuthorizer{plan: true, approve: true, execute: true}, wantChecks: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &agentCheckingActionExecutor{}
			h := newLiveCheckActionHandlers(t, tc.grants, executor)
			rec := planLiveCheckAction(h, "detail-"+tc.name)
			if rec.Code != http.StatusOK {
				t.Fatalf("plan status=%d body=%s", rec.Code, rec.Body.String())
			}
			var planned unified.ActionPlan
			if err := json.Unmarshal(rec.Body.Bytes(), &planned); err != nil {
				t.Fatal(err)
			}
			detailRec := httptest.NewRecorder()
			detailReq := httptest.NewRequest(http.MethodGet, "/api/actions/"+planned.ActionID, nil)
			detailReq.SetPathValue("id", planned.ActionID)
			h.HandleGetAction(detailRec, actionHandlerTestRequest(detailReq, "operator@example.com"))
			if detailRec.Code != http.StatusOK {
				t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
			}
			var detail actionDetailResponse
			if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if !detail.Readiness.Ready || executor.agentChecks != tc.wantChecks {
				t.Fatalf("readiness=%#v agent checks=%d, want %d", detail.Readiness, executor.agentChecks, tc.wantChecks)
			}
		})
	}
}

func TestActionAuthorityLiveCheckNeedsExecute(t *testing.T) {
	plan := auth.ActionPlan + "/" + auth.ResourceActions
	execute := auth.ActionExecute + "/" + auth.ResourceActions
	ctx := auth.WithUser(context.Background(), "operator@example.com")
	user := unified.ActionActor{SubjectID: "operator@example.com", Kind: unified.ActionActorUser, CredentialID: "session:test", OrgID: "default"}
	service := unified.ActionActor{SubjectID: "patrol", Kind: unified.ActionActorService, CredentialID: "service:patrol", OrgID: "default"}

	planOnly := actionAuthority{authorizer: actionCapabilityAuthorizer{plan: true}, orgChecker: NewAuthorizationChecker(nil)}
	if err := planOnly.AuthorizeLiveCheck(ctx, "default", user); err == nil {
		t.Fatal("plan-only grant admitted the agent check")
	}
	withExecute := actionAuthority{authorizer: actionCapabilityAuthorizer{plan: true, execute: true}, orgChecker: NewAuthorizationChecker(nil)}
	if err := withExecute.AuthorizeLiveCheck(ctx, "default", user); err != nil {
		t.Fatalf("execute grant refused: %v", err)
	}
	if err := planOnly.AuthorizeLiveCheck(context.Background(), "default", service); err != nil {
		t.Fatalf("server broker identity refused: %v", err)
	}
	if err := withExecute.AuthorizeLiveCheck(context.Background(), "default", user); err == nil {
		t.Fatal("user actor without a matching authenticated request admitted the agent check")
	}
}

// The Assistant plans under its own service identity; the chat caller's
// action-execute authority decides whether the agent checks the plan.
func TestAssistantPlanAgentCheckFollowsCallerExecuteAuthority(t *testing.T) {
	refusal := &unified.ResourceActionReadiness{Name: "restart", Available: false, ReasonCode: "package_inventory_changed"}
	for _, tc := range []struct {
		name       string
		ctx        context.Context
		wantChecks int
	}{
		{name: "caller without execute", ctx: chat.WithHostCommandAuthority(context.Background(), false), wantChecks: 0},
		{name: "unbound caller", ctx: context.Background(), wantChecks: 0},
		{name: "caller with execute", ctx: chat.WithHostCommandAuthority(context.Background(), true), wantChecks: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &agentCheckingActionExecutor{refusal: refusal}
			h := newLiveCheckActionHandlers(t, actionCapabilityAuthorizer{}, executor)
			_, err := assistantTypedActionPlanner{resources: h}.PlanTypedAction(tc.ctx, "default", unified.ActionRequest{
				RequestID: "assistant-" + tc.name, ResourceID: "vm:42", CapabilityName: "restart",
				Params: map[string]any{"mode": "graceful"}, Reason: "Assistant proposed restart",
			})
			if executor.agentChecks != tc.wantChecks {
				t.Fatalf("agent checks = %d, want %d (err=%v)", executor.agentChecks, tc.wantChecks, err)
			}
			if (err == nil) != (tc.wantChecks == 0) {
				t.Fatalf("plan error = %v", err)
			}
		})
	}
}

// Approving follows the same rule. Without execute the approval is recorded
// on Pulse's checks alone (dispatch still asks the agent); with execute the
// agent's refusal blocks the approval.
func TestDecideActionAgentCheckFollowsRBACExecuteGrant(t *testing.T) {
	plan := auth.ActionPlan + "/" + auth.ResourceActions
	approve := auth.ActionApprove + "/" + auth.ResourceActions
	execute := auth.ActionExecute + "/" + auth.ResourceActions
	refusal := &unified.ResourceActionReadiness{Name: "restart", Available: false, ReasonCode: "package_inventory_changed"}
	for _, tc := range []struct {
		name        string
		grants      actionCapabilityAuthorizer
		wantChecks  int
		wantApprove bool
	}{
		{name: "plan and approve without execute", grants: actionCapabilityAuthorizer{plan: true, approve: true}, wantChecks: 0, wantApprove: true},
		{name: "approve and execute", grants: actionCapabilityAuthorizer{plan: true, approve: true, execute: true}, wantChecks: 1, wantApprove: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &agentCheckingActionExecutor{}
			h := newLiveCheckActionHandlers(t, actionCapabilityAuthorizer{plan: true}, executor)
			rec := planLiveCheckAction(h, "decide-"+tc.name)
			if rec.Code != http.StatusOK {
				t.Fatalf("plan status=%d body=%s", rec.Code, rec.Body.String())
			}
			var planned unified.ActionPlan
			if err := json.Unmarshal(rec.Body.Bytes(), &planned); err != nil {
				t.Fatal(err)
			}
			authority := actionAuthority{authorizer: tc.grants, orgChecker: NewAuthorizationChecker(nil)}
			h.SetActionAuthorizers(authority, authority, authority)
			executor.refusal = refusal

			decisionRec := httptest.NewRecorder()
			decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+planned.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"reviewed"}`))
			decisionReq.SetPathValue("id", planned.ActionID)
			h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, "operator@example.com"))
			wantStatus, wantState := http.StatusOK, unified.ActionStateApproved
			if !tc.wantApprove {
				wantStatus, wantState = http.StatusConflict, unified.ActionStatePending
			}
			if decisionRec.Code != wantStatus || (!tc.wantApprove && !bytes.Contains(decisionRec.Body.Bytes(), []byte("package_inventory_changed"))) {
				t.Fatalf("decision status=%d body=%s, want %d", decisionRec.Code, decisionRec.Body.String(), wantStatus)
			}
			store, err := h.getStore("default")
			if err != nil {
				t.Fatal(err)
			}
			audit, found, err := store.GetActionAudit(planned.ActionID)
			if err != nil || !found || audit.State != wantState {
				t.Fatalf("persisted audit state=%q found=%v err=%v, want %q", audit.State, found, err, wantState)
			}
			if executor.agentChecks != tc.wantChecks {
				t.Fatalf("agent checks = %d, want %d", executor.agentChecks, tc.wantChecks)
			}
		})
	}
}

// An owner-bound token needs both an execute scope and the owner's execute
// grant before the agent checks anything for it.
func TestActionAuthorityLiveCheckForTokens(t *testing.T) {
	execute := auth.ActionExecute + "/" + auth.ResourceActions
	allowExecute := actionAuthority{authorizer: actionCapabilityAuthorizer{execute: true}, orgChecker: NewAuthorizationChecker(nil)}
	planScoped := ownerBoundActionToken("live-check-plan", "alice", config.ScopeActionsPlan)
	if err := allowExecute.AuthorizeLiveCheck(actionAuthorityContext(planScoped), "default", tokenActionActor(planScoped)); err == nil {
		t.Fatal("actions:plan token admitted the agent check")
	}
	executeScoped := ownerBoundActionToken("live-check-execute", "alice", config.ScopeActionsPlan, config.ScopeActionsExecute)
	if err := allowExecute.AuthorizeLiveCheck(actionAuthorityContext(executeScoped), "default", tokenActionActor(executeScoped)); err != nil {
		t.Fatalf("actions:execute token with an execute grant refused: %v", err)
	}
	planOnlyRole := actionAuthority{authorizer: actionCapabilityAuthorizer{auth.ActionPlan + "/" + auth.ResourceActions: true}, orgChecker: NewAuthorizationChecker(nil)}
	if err := planOnlyRole.AuthorizeLiveCheck(actionAuthorityContext(executeScoped), "default", tokenActionActor(executeScoped)); err == nil {
		t.Fatal("actions:execute token whose owner lacks the execute grant admitted the agent check")
	}
}
