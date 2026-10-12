package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// TestPatrolActionBrokerNeverExecutesWhateverTheCapabilityEligibility pins
// the retirement of Patrol's automatic fix modes: a proposal is recorded as
// a plan and waits for an operator, whatever auto-authorization class the
// capability declares, whatever approval level it requires, and however
// often the proposal is replayed.
func TestPatrolActionBrokerNeverExecutesWhateverTheCapabilityEligibility(t *testing.T) {
	tests := []struct {
		name        string
		approval    unified.ActionApprovalLevel
		eligibility unified.ActionAutoAuthorizationClass
		wantState   unified.ActionState
	}{
		{name: "low risk admin approval", approval: unified.ApprovalAdmin, eligibility: unified.AutoAuthorizeLowRisk, wantState: unified.ActionStatePending},
		{name: "elevated admin approval", approval: unified.ApprovalAdmin, eligibility: unified.AutoAuthorizeElevated, wantState: unified.ActionStatePending},
		{name: "low risk no approval", approval: unified.ApprovalNone, eligibility: unified.AutoAuthorizeLowRisk, wantState: unified.ActionStatePlanned},
		{name: "elevated no approval", approval: unified.ApprovalNone, eligibility: unified.AutoAuthorizeElevated, wantState: unified.ActionStatePlanned},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, executor := newPatrolBrokerTestHandlersWithEligibility(t, tc.approval, tc.eligibility)
			broker := NewPatrolActionBroker("default", h)
			first, err := broker.Submit(context.Background(), patrolTestProposal())
			if err != nil {
				t.Fatalf("Submit: %v", err)
			}
			replayed, err := broker.Submit(context.Background(), patrolTestProposal())
			if err != nil || replayed.ActionID != first.ActionID {
				t.Fatalf("replay disposition=%#v err=%v", replayed, err)
			}
			if first.State != string(tc.wantState) || replayed.State != string(tc.wantState) || executor.calls != 0 {
				t.Fatalf("states=%q/%q calls=%d, want %q and no execution", first.State, replayed.State, executor.calls, tc.wantState)
			}
			record, found, err := h.ActionLifecycle().Get("default", first.ActionID)
			if err != nil || !found || len(record.Approvals) != 0 || record.Result != nil {
				t.Fatalf("proposal carried execution authority: found=%v err=%v record=%#v", found, err, record)
			}
		})
	}
}

// TestPatrolActionBrokerRecordsCapabilityOnlyPolicyProvenance pins that a
// Patrol plan no longer snapshots tenant autonomy or the per-resource
// automatic-action allowlist: neither can authorize anything, so the plan
// names the capability as its only authority even when the resource carries
// operator state.
func TestPatrolActionBrokerRecordsCapabilityOnlyPolicyProvenance(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	state := unified.ResourceOperatorState{CanonicalID: "vm:42", NeverAutoRemediate: true, SetAt: time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)}
	if err := store.SetResourceOperatorState(state); err != nil {
		t.Fatal(err)
	}
	disposition, err := NewPatrolActionBroker("default", h).Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("record: found=%v err=%v", found, err)
	}
	provenance := record.Plan.PolicyDecision
	if provenance.Version != unified.ActionPolicyDecisionVersion || len(provenance.Authorities) != 1 || provenance.Authorities[0].Kind != unified.ActionPolicyAuthorityCapability {
		t.Fatalf("provenance=%#v", provenance)
	}
	if executor.calls != 0 {
		t.Fatalf("plan dispatched: calls=%d", executor.calls)
	}
}

func TestPatrolApprovalRefusesLostReadinessBeforeRecordingDecision(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	disposition, err := NewPatrolActionBroker("default", h).Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	executor.readiness = &unified.ResourceActionReadiness{
		Name:       "restart",
		Available:  false,
		ReasonCode: "command_agent_disconnected",
		Reason:     "Connect the command agent for web-42.",
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved"}`))
	request.SetPathValue("id", disposition.ActionID)
	h.HandleDecideAction(recorder, actionHandlerTestRequest(request, "operator@example.com"))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Error   string            `json:"error"`
		Details map[string]string `json:"details"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error != "action_execution_unavailable" || envelope.Details["reasonCode"] != "command_agent_disconnected" || envelope.Details["reason"] != "Connect the command agent for web-42." {
		t.Fatalf("envelope=%#v", envelope)
	}
	record, found, err := h.ActionLifecycle().Get("default", disposition.ActionID)
	if err != nil || !found || record.State != unified.ActionStatePending || len(record.Approvals) != 0 || executor.calls != 0 {
		t.Fatalf("record=%#v found=%v err=%v calls=%d", record, found, err, executor.calls)
	}
}

func TestPatrolRefreshRebindsPlanAndTrustedOrigin(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	disposition, err := NewPatrolActionBroker("default", h).Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	previous, found, err := h.ActionLifecycle().Get("default", disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("previous found=%v err=%v", found, err)
	}
	registry, err := h.buildRegistry("default")
	if err != nil {
		t.Fatal(err)
	}
	resource, found := registry.Get(previous.Request.ResourceID)
	if !found || resource == nil {
		t.Fatal("planned resource missing")
	}
	changed := *resource
	changed.UpdatedAt = changed.UpdatedAt.Add(time.Second)
	changed.Name = changed.Name + " refreshed"
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: changed.UpdatedAt},
		resources: []unified.Resource{changed},
	})
	h.invalidateCache("default")
	h.SetActionRefreshPlanner(NewActionRefreshPlanner())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/actions/"+previous.ID+"/refresh", bytes.NewBufferString(`{"planHash":"`+previous.Plan.PlanHash+`"}`))
	request.SetPathValue("id", previous.ID)
	h.HandleRefreshAction(recorder, actionHandlerTestRequest(request, "operator@example.com"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var detail actionDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	replacement := detail.Audit.ActionAuditRecord
	if replacement.ID == previous.ID || replacement.Request.RequestID != "refresh:"+previous.ID || replacement.Request.Actor.SubjectID != patrolActionBrokerActor {
		t.Fatalf("replacement=%#v", replacement)
	}
	if replacement.Origin == nil || replacement.Origin.FindingID != previous.Origin.FindingID || replacement.Origin.InvestigationID != previous.Origin.InvestigationID {
		t.Fatalf("origin=%#v", replacement.Origin)
	}
	if !detail.Readiness.Ready || len(replacement.Plan.PolicyDecision.Authorities) != 1 || replacement.Plan.PolicyDecision.Authorities[0].Kind != unified.ActionPolicyAuthorityCapability {
		t.Fatalf("readiness=%#v policy=%#v", detail.Readiness, replacement.Plan.PolicyDecision)
	}
}

func TestPatrolActionBrokerConflictingOriginReplayFailsWithoutDispatch(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)
	if _, err := broker.Submit(context.Background(), patrolTestProposal()); err != nil {
		t.Fatal(err)
	}
	conflict := patrolTestProposal()
	conflict.FindingID = "finding-2"
	conflict.InvestigationID = "inv-2"
	if _, err := broker.Submit(context.Background(), conflict); !errors.Is(err, unified.ErrActionIdentityConflict) {
		t.Fatalf("error=%v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls=%d", executor.calls)
	}
}

func newPatrolBrokerTestHandlers(t *testing.T, minimumApproval unified.ActionApprovalLevel) (*ResourceHandlers, *stubActionExecutor) {
	return newPatrolBrokerTestHandlersWithEligibility(t, minimumApproval, unified.AutoAuthorizeLowRisk)
}

func newPatrolBrokerTestHandlersWithEligibility(t *testing.T, minimumApproval unified.ActionApprovalLevel, eligibility unified.ActionAutoAuthorizationClass) (*ResourceHandlers, *stubActionExecutor) {
	t.Helper()
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{
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
						AutoAuthorization:    eligibility,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
					{
						Name:                 "join_cluster",
						Type:                 unified.CapabilityTypeCommon,
						Description:          "Join an authenticated cluster",
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.join",
						Params: []unified.CapabilityParam{
							{Name: "join_token", Type: "string", Required: true, IsSensitive: true},
						},
					},
				},
			},
		},
	})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true}}
	h.SetActionExecutor(executor)
	return h, executor
}

func patrolTestProposal() aicontracts.ActionProposal {
	return aicontracts.ActionProposal{
		ProposalID:      "prop-1",
		FindingID:       "finding-1",
		InvestigationID: "inv-1",
		ResourceID:      "vm:42",
		CapabilityName:  "restart",
		Params:          map[string]any{"mode": "graceful"},
		Reason:          "Recover after confirmed outage",
	}
}

func TestPatrolActionBrokerSubmitPlansThroughCanonicalLifecycle(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	proposal := patrolTestProposal()
	proposal.EvidenceIDs = []string{" evidence-2 ", "evidence-1", "evidence-1", ""}
	disposition, err := broker.Submit(context.Background(), proposal)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.ActionID == "" {
		t.Fatal("disposition has no action id")
	}
	if disposition.State != string(unified.ActionStatePending) {
		t.Fatalf("state = %q, want pending_approval", disposition.State)
	}
	if !disposition.Plan.RequiresApproval || disposition.Plan.ApprovalPolicy != string(unified.ApprovalAdmin) {
		t.Fatalf("plan projection = %#v", disposition.Plan)
	}
	if executor.calls != 0 {
		t.Fatalf("submit without core policy must not execute, executor calls = %d", executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	record, ok, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if record.Request.RequestedBy != patrolActionBrokerActor {
		t.Fatalf("requestedBy = %q, want %q", record.Request.RequestedBy, patrolActionBrokerActor)
	}
	if record.Origin == nil ||
		record.Origin.Surface != patrolActionOriginSurface ||
		record.Origin.FindingID != "finding-1" ||
		record.Origin.InvestigationID != "inv-1" ||
		record.Origin.ProposalID != "prop-1" ||
		fmt.Sprint(record.Origin.EvidenceIDs) != "[evidence-1 evidence-2]" {
		t.Fatalf("audit origin = %#v", record.Origin)
	}
}

func TestPatrolActionBrokerApprovalNoneDoesNotExecuteWithoutCorePolicy(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalNone)
	broker := NewPatrolActionBroker("default", h)

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStatePlanned) {
		t.Fatalf("state = %q, want planned", disposition.State)
	}
	if disposition.Plan.RequiresApproval {
		t.Fatal("ApprovalNone capability should not require approval")
	}
	if executor.calls != 0 {
		t.Fatalf("submit without core policy must not execute, executor calls = %d", executor.calls)
	}
}

func TestPatrolActionBrokerRejectsSensitiveParams(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	proposal := patrolTestProposal()
	proposal.CapabilityName = "join_cluster"
	proposal.Params = map[string]any{"join_token": "sekret-token-value"}
	_, err := broker.Submit(context.Background(), proposal)
	if !errors.Is(err, aicontracts.ErrSensitiveParamsRequireOperator) {
		t.Fatalf("error = %v, want ErrSensitiveParamsRequireOperator", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("sensitive-param proposals must not persist audits, got %d", len(audits))
	}
}

func TestPatrolActionBrokerCapabilitiesCatalog(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	catalog, err := broker.Capabilities(context.Background(), "vm:42")
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if catalog.ResourceID != "vm:42" || len(catalog.Capabilities) != 2 {
		t.Fatalf("catalog = %#v", catalog)
	}
	byName := map[string]aicontracts.ActionCapabilityInfo{}
	for _, capability := range catalog.Capabilities {
		byName[capability.Name] = capability
	}
	restart := byName["restart"]
	if restart.MinimumApprovalLevel != string(unified.ApprovalAdmin) || restart.AutoAuthorization != string(unified.AutoAuthorizeLowRisk) || len(restart.Params) != 1 || restart.Params[0].Sensitive {
		t.Fatalf("restart capability = %#v", restart)
	}
	join := byName["join_cluster"]
	if len(join.Params) != 1 || !join.Params[0].Sensitive {
		t.Fatalf("join_cluster capability must mark its token sensitive: %#v", join)
	}

	if _, err := broker.Capabilities(context.Background(), "vm:404"); err == nil {
		t.Fatal("unknown resource must error")
	}
}

func TestPatrolActionBrokerRequiresCorrelationIdentity(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	missingFinding := patrolTestProposal()
	missingFinding.FindingID = " "
	if _, err := broker.Submit(context.Background(), missingFinding); err == nil {
		t.Fatal("proposal without finding id must be refused")
	}

	missingInvestigation := patrolTestProposal()
	missingInvestigation.InvestigationID = ""
	if _, err := broker.Submit(context.Background(), missingInvestigation); err == nil {
		t.Fatal("proposal without investigation id must be refused")
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("identity-less proposals must not persist audits, got %d", len(audits))
	}
}

func TestPatrolActionBrokerSubmitPublishesOrgScopedTransition(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	var gotOrg string
	var gotState unified.ActionState
	var gotOrigin *unified.ActionOrigin
	h.SetActionTransitionPublisher(func(orgID string, record unified.ActionAuditRecord) {
		gotOrg = orgID
		gotState = record.State
		gotOrigin = record.Origin
	})
	broker := NewPatrolActionBroker("default", h)

	if _, err := broker.Submit(context.Background(), patrolTestProposal()); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotOrg != "default" {
		t.Fatalf("transition org = %q, want default", gotOrg)
	}
	if gotState != unified.ActionStatePending {
		t.Fatalf("transition state = %q, want pending_approval", gotState)
	}
	if gotOrigin == nil || gotOrigin.FindingID != "finding-1" {
		t.Fatalf("transition origin = %#v", gotOrigin)
	}
}

func TestPatrolTypedActionJourneyDetectPlanApproveExecuteVerifyAndReconcile(t *testing.T) {
	resources, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	executor.result = &unified.ExecutionResult{
		Success: true,
		Output:  "restart dispatched",
		Verification: &unified.ActionVerificationResult{
			Ran: true, Success: true, RanAt: time.Now().UTC(), Note: "workload health confirmed",
		},
	}
	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	pushes := make(chan relay.PushNotificationPayload, 1)
	patrol.SetPushNotifyCallback(func(payload relay.PushNotificationPayload) { pushes <- payload })
	finding := addPatrolFindingForResource(t, patrol, "finding-1", time.Now().UTC(), "vm:42", "Unhealthy workload")
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "session-1")
	if investigation.ID != "inv-1" {
		t.Fatalf("investigation id = %q, journey proposal expects inv-1", investigation.ID)
	}
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)

	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	planned := investigations.Get(investigation.ID)
	if planned.Action == nil || planned.Action.ActionID != disposition.ActionID || planned.Action.State != string(unified.ActionStatePending) {
		t.Fatalf("planned investigation action = %#v", planned.Action)
	}
	if planned.Outcome != aicontracts.OutcomeFixQueued {
		t.Fatalf("planned outcome = %q", planned.Outcome)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"maintenance window"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	resources.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, ""))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	executionRec := httptest.NewRecorder()
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"approved maintenance window"}`))
	executionReq.SetPathValue("id", disposition.ActionID)
	executionReq = executionReq.WithContext(auth.WithUser(executionReq.Context(), "operator@example.com"))
	resources.HandleExecuteAction(executionRec, actionHandlerTestRequest(executionReq, ""))
	if executionRec.Code != http.StatusOK {
		t.Fatalf("execution status = %d body=%s", executionRec.Code, executionRec.Body.String())
	}

	completed := investigations.Get(investigation.ID)
	if completed.Action == nil || completed.Action.State != string(unified.ActionStateCompleted) {
		t.Fatalf("completed investigation action = %#v", completed.Action)
	}
	if completed.Outcome != aicontracts.OutcomeFixVerificationUnknown {
		t.Fatalf("completed outcome = %q, want verification unknown", completed.Outcome)
	}
	updatedFinding := patrol.GetFindings().Get(finding.ID)
	if updatedFinding == nil || updatedFinding.InvestigationOutcome != string(aicontracts.OutcomeFixVerificationUnknown) || updatedFinding.ResolvedAt != nil {
		t.Fatalf("reconciled finding = %#v", updatedFinding)
	}
	store, err := resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found || audit.VerificationOutcome.Status != unified.VerificationVerified {
		t.Fatalf("terminal audit: found=%v err=%v audit=%#v", found, err, audit)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want exactly one", executor.calls)
	}
	select {
	case push := <-pushes:
		if push.ActionType != relay.PushActionViewFixResult || push.ActionID != finding.ID || push.Body != "Action completed; verification was inconclusive" {
			t.Fatalf("terminal push = %#v", push)
		}
	default:
		t.Fatal("agent-attested lifecycle did not publish an honest terminal mobile notification")
	}
}

func TestPatrolActionBrokerPlanningReplaySurvivesLostRegistry(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h).(*patrolActionBroker)
	lifecycle := broker.lifecycle()
	broker.lifecycle = func() *actionlifecycle.Service { return lifecycle }
	first, err := broker.Plan(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	lifecycle.Registry = func(string) (*unified.ResourceRegistry, error) { return nil, errors.New("registry unavailable") }
	again, err := broker.Plan(context.Background(), patrolTestProposal())
	if err != nil || again.ActionID != first.ActionID || again.Plan.PlanHash != first.Plan.PlanHash {
		t.Fatalf("replay lost existing plan: %+v %v", again, err)
	}
	if executor.calls != 0 {
		t.Fatal("planning replay executed infrastructure")
	}
}
