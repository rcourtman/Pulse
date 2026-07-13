package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func configureActionHandlerTestAuthority(h *ResourceHandlers) {
	h.SetActionAuthorizers(
		actionlifecycle.DecisionAuthorizerFunc(func(context.Context, string, unified.ActionAuditRecord, unified.ActionDecision) error { return nil }),
		actionlifecycle.ExecutionAuthorizerFunc(func(context.Context, string, unified.ActionAuditRecord, unified.ActionActor) error { return nil }),
	)
}

func newActionTestResourceHandlers(t *testing.T, cfg *config.Config) *ResourceHandlers {
	t.Helper()
	h := NewResourceHandlers(cfg)
	configureActionHandlerTestAuthority(h)
	return h
}

func actionHandlerTestRequest(req *http.Request, subject string) *http.Request {
	if strings.TrimSpace(subject) == "" {
		subject = strings.TrimSpace(auth.GetUser(req.Context()))
	}
	if subject == "" {
		subject = "operator@example.com"
	}
	actor := unified.ActionActor{SubjectID: subject, Kind: unified.ActionActorUser, CredentialID: "session:test", OrgID: "default"}
	ctx := auth.WithUser(req.Context(), subject)
	ctx = withTrustedActionActor(ctx, actor)
	return req.WithContext(ctx)
}

func boundActionTestRequest(requestID, resourceID, capability, reason, subject string) unified.ActionRequest {
	actor := unified.ActionActor{SubjectID: subject, Kind: unified.ActionActorService, CredentialID: "service:test-requester", OrgID: "default"}
	return unified.ActionRequest{RequestID: requestID, ResourceID: resourceID, CapabilityName: capability, Reason: reason, RequestedBy: subject, Actor: actor}
}

func boundActionTestApproval(actionID, planHash, subject string, at time.Time) unified.ActionApprovalRecord {
	return boundActionTestDecisionApproval(actionID, planHash, subject, unified.OutcomeApproved, at)
}

func boundActionTestDecisionApproval(actionID, planHash, subject string, outcome unified.ApprovalOutcome, at time.Time) unified.ActionApprovalRecord {
	actor := unified.ActionActor{SubjectID: subject, Kind: unified.ActionActorUser, CredentialID: "session:test", OrgID: "default"}
	evidence := unified.ApprovalEvidence{Version: 1, Method: unified.MethodSession, Actor: actor, OrgID: "default", ActionID: actionID, PlanHash: planHash, Outcome: outcome, IssuedAt: at}
	return unified.ActionApprovalRecord{Actor: subject, ActorBinding: actor, Method: unified.MethodSession, Timestamp: at, Outcome: outcome, Evidence: &evidence}
}

type stubActionExecutor struct {
	result   *unified.ExecutionResult
	err      error
	received unified.ActionAuditRecord
	calls    int
}

func (s *stubActionExecutor) ExecuteAction(_ context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	s.calls++
	s.received = record
	return s.result, s.err
}

func TestHandlePlanActionBindsActorAndPlanHashToAuthenticatedOrg(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
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
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
				},
				Relationships: []unified.ResourceRelationship{
					{
						SourceID: "vm:42",
						TargetID: "node-1",
						Type:     unified.RelRunsOn,
						Active:   true,
					},
				},
			},
		},
	})
	body := bytes.NewBufferString(`{
		"requestId":"agent-run-123",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body)
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "InternalHandler") || strings.Contains(rec.Body.String(), "proxmox.vm.restart") {
		t.Fatalf("response leaked internal execution handler: %s", rec.Body.String())
	}

	var plan unified.ActionPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !plan.Allowed {
		t.Fatalf("Allowed = false, want true")
	}
	if !plan.RequiresApproval {
		t.Fatalf("RequiresApproval = false, want true")
	}
	if plan.ApprovalPolicy != unified.ApprovalAdmin {
		t.Fatalf("ApprovalPolicy = %q, want %q", plan.ApprovalPolicy, unified.ApprovalAdmin)
	}
	if plan.ActionID == "" || !strings.HasPrefix(plan.PlanHash, "sha256:") {
		t.Fatalf("missing action identity/hash: actionID=%q planHash=%q", plan.ActionID, plan.PlanHash)
	}
	if plan.ApprovalRequirement.Version != unified.ActionApprovalRequirementVersion {
		t.Fatalf("ApprovalRequirement = %#v, want canonical version", plan.ApprovalRequirement)
	}
	if plan.PolicyDecision.Version != unified.ActionPolicyDecisionVersion || len(plan.PolicyDecision.Authorities) != 1 || plan.PolicyDecision.Authorities[0].Kind != unified.ActionPolicyAuthorityCapability {
		t.Fatalf("public/manual plan provenance = %#v, want capability authority only", plan.PolicyDecision)
	}
	if plan.Preflight == nil || plan.Preflight.Target != "vm:42" {
		t.Fatalf("Preflight = %#v, want target vm:42", plan.Preflight)
	}
	if len(plan.PredictedBlastRadius) != 2 || plan.PredictedBlastRadius[0] != "vm:42" || plan.PredictedBlastRadius[1] != "node-1" {
		t.Fatalf("PredictedBlastRadius = %#v", plan.PredictedBlastRadius)
	}
}

func TestHandlePlanActionRejectsClientSuppliedPolicyProvenance(t *testing.T) {
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	body := bytes.NewBufferString(`{"requestId":"req-1","resourceId":"vm:42","capabilityName":"restart","reason":"recover","policyDecision":{"version":1}}`)
	rec := httptest.NewRecorder()
	h.HandlePlanAction(rec, actionHandlerTestRequest(httptest.NewRequest(http.MethodPost, "/api/actions/plan", body), ""))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_action_request") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestApprovalPlanInfoRejectsMalformedCanonicalPolicyDecision(t *testing.T) {
	for name, payload := range map[string]json.RawMessage{
		"unknown_field":   []byte(`{"version":1,"unknown":true}`),
		"trailing_value":  []byte(`{"version":0,"status":"legacy_unknown"}{}`),
		"partial_current": []byte(`{"version":1,"status":"resolved"}`),
		"null":            []byte(`null`),
		"empty_object":    []byte(`{}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := approvalPlanInfoToRequest(&aicontracts.ActionPlanInfo{PolicyDecision: payload}); err == nil {
				t.Fatal("malformed provenance downgraded to legacy truth")
			}
		})
	}
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	provenance, err := unified.BuildActionPolicyDecisionProvenance("act-1", unified.ActionPolicyDecisionScope{OrgID: "default", ResourceID: "vm:42", CapabilityName: "restart"}, []unified.ActionPolicyAuthorityFactor{{Kind: unified.ActionPolicyAuthorityCapability, SourceID: "capability-registry:restart", Revision: "policy:sha256:0123456789abcdef01234567", Status: unified.ActionPolicyAuthorityConsulted, ApprovalFloor: unified.ApprovalAdmin, ReasonCodes: []unified.ActionPolicyReasonCode{unified.PolicyReasonCapabilityApprovalAdmin, unified.PolicyReasonCapabilityAutoNever}}}, requirement, true, true)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(provenance)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := approvalPlanInfoToRequest(&aicontracts.ActionPlanInfo{ActionID: "act-other", Allowed: true, RequiresApproval: true, ApprovalPolicy: string(unified.ApprovalAdmin), PolicyDecision: payload}); err == nil {
		t.Fatal("cross-plan canonical provenance was accepted")
	}
	canonicalPlan := &unified.ActionPlan{ActionID: "act-1", Allowed: true, RequiresApproval: true, ApprovalPolicy: unified.ApprovalAdmin, ApprovalRequirement: requirement, PolicyDecision: provenance}
	converted, err := approvalPlanInfoToRequest(approvalPlanRequestToInfo(canonicalPlan))
	if err != nil || converted == nil || !reflect.DeepEqual(converted.PolicyDecision, provenance) || converted.ApprovalRequirement != requirement {
		t.Fatalf("canonical provenance relay round trip: plan=%#v err=%v", converted, err)
	}
}

func TestHandlePlanActionRejectsOrIgnoresPublicRequestedByAndStampsAuthenticatedActor(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
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
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
				},
			},
		},
	})
	body := func() *bytes.Buffer {
		return bytes.NewBufferString(`{
			"requestId":"agent-run-123",
			"resourceId":"vm:42",
			"capabilityName":"restart",
			"params":{"mode":"graceful"},
			"reason":"Recover after confirmed outage",
			"requestedBy":"agent:oncall-helper"
		}`)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body())
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("audits len = %d, want 1: %#v", len(audits), audits)
	}
	audit := audits[0]
	if audit.ID != plan.ActionID || audit.State != unified.ActionStatePending {
		t.Fatalf("audit identity/state = %q/%q, want %q/%q", audit.ID, audit.State, plan.ActionID, unified.ActionStatePending)
	}
	if audit.Request.RequestID != "agent-run-123" || audit.Request.RequestedBy != "operator@example.com" || audit.Request.Actor.SubjectID != "operator@example.com" || audit.Request.Actor.OrgID != "default" {
		t.Fatalf("audit request was not preserved: %#v", audit.Request)
	}
	if audit.Plan.PlanHash != plan.PlanHash || audit.Plan.Preflight == nil {
		t.Fatalf("audit plan did not preserve plan/preflight: %#v", audit.Plan)
	}

	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seenStates := map[unified.ActionState]bool{}
	for _, event := range events {
		seenStates[event.State] = true
		if event.Actor != "operator@example.com" {
			t.Fatalf("event actor = %q, want requester", event.Actor)
		}
	}
	if len(events) != 2 || !seenStates[unified.ActionStatePlanned] || !seenStates[unified.ActionStatePending] {
		t.Fatalf("events = %#v, want planned and pending_approval", events)
	}

	retryRec := httptest.NewRecorder()
	retryReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body())
	h.HandlePlanAction(retryRec, actionHandlerTestRequest(retryReq, ""))
	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body=%s", retryRec.Code, retryRec.Body.String())
	}
	events, err = store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents after retry: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("retry duplicated lifecycle events: %#v", events)
	}

	listRec := httptest.NewRecorder()
	h.HandleListActions(listRec, httptest.NewRequest(http.MethodGet, "/api/actions?view=pending", nil))
	var inbox actionInboxResponse
	if listRec.Code != http.StatusOK || json.Unmarshal(listRec.Body.Bytes(), &inbox) != nil || len(inbox.Actions) != 1 || !reflect.DeepEqual(inbox.Actions[0].Plan.PolicyDecision, plan.PolicyDecision) {
		t.Fatalf("list provenance mismatch: status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	detailRec := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/api/actions/"+plan.ActionID, nil)
	detailReq.SetPathValue("id", plan.ActionID)
	h.HandleGetAction(detailRec, detailReq)
	var detail actionlifecycle.ActionDetail
	if detailRec.Code != http.StatusOK || json.Unmarshal(detailRec.Body.Bytes(), &detail) != nil || !reflect.DeepEqual(detail.Audit.Plan.PolicyDecision, plan.PolicyDecision) {
		t.Fatalf("detail provenance mismatch: status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
}

func TestHandleListPendingActionsReturnsOnlyCanonicalDecisionQueue(t *testing.T) {
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	now := time.Now().UTC()
	for _, record := range []unified.ActionAuditRecord{
		{
			ID: "act-pending", CreatedAt: now, UpdatedAt: now, State: unified.ActionStatePending,
			Request: unified.ActionRequest{RequestID: "proposal-1", ResourceID: "vm:42", CapabilityName: "restart", Reason: "Recover workload", RequestedBy: "pulse_patrol"},
			Plan:    unified.ActionPlan{ActionID: "act-pending", RequestID: "proposal-1", Allowed: true, RequiresApproval: true, ApprovalPolicy: unified.ApprovalAdmin},
			Origin:  &unified.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: "investigation-1", ProposalID: "proposal-1"},
		},
		{
			ID: "act-completed", CreatedAt: now, UpdatedAt: now, State: unified.ActionStateCompleted,
			Request: unified.ActionRequest{RequestID: "proposal-2", ResourceID: "vm:43", CapabilityName: "restart", Reason: "Recovered", RequestedBy: "pulse_patrol"},
			Plan:    unified.ActionPlan{ActionID: "act-completed", RequestID: "proposal-2", Allowed: true},
		},
	} {
		if err := store.RecordActionAudit(record); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/actions/pending", nil)
	h.HandleListPendingActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response pendingActionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Count != 1 || len(response.Actions) != 1 || response.Actions[0].ID != "act-pending" {
		t.Fatalf("pending response = %#v", response)
	}
	if response.Actions[0].Origin == nil || response.Actions[0].Origin.InvestigationID != "investigation-1" {
		t.Fatalf("pending action lost Patrol origin: %#v", response.Actions[0].Origin)
	}
}

func TestHandleListAndDetailActionsProjectCanonicalResourcePresentation(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{{
			ID:       "vm:42",
			Type:     unified.ResourceTypeVM,
			Name:     "Checkout API",
			Status:   unified.StatusWarning,
			LastSeen: now,
			Sources:  []unified.DataSource{unified.SourceProxmox},
		}},
	})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	record := unified.ActionAuditRecord{
		ID: "act-resource-presentation", CreatedAt: now, UpdatedAt: now, State: unified.ActionStatePending,
		Request: unified.ActionRequest{RequestID: "req-resource-presentation", ResourceID: "vm:42", CapabilityName: "restart", Reason: "Recover checkout", RequestedBy: "pulse_patrol"},
		Plan: unified.ActionPlan{
			ActionID: "act-resource-presentation", RequestID: "req-resource-presentation",
			Allowed: true, RequiresApproval: true, ApprovalPolicy: unified.ApprovalAdmin,
			PlannedAt: now, ExpiresAt: now.Add(4 * time.Hour), PlanHash: "sha256:resource-presentation",
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatal(err)
	}

	listRec := httptest.NewRecorder()
	h.HandleListActions(listRec, httptest.NewRequest(http.MethodGet, "/api/actions?view=pending", nil))
	var inbox actionInboxResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &inbox); listRec.Code != http.StatusOK || err != nil {
		t.Fatalf("list status=%d body=%s err=%v", listRec.Code, listRec.Body.String(), err)
	}
	if len(inbox.Actions) != 1 || inbox.Actions[0].Resource == nil || inbox.Actions[0].Resource.Name != "Checkout API" || inbox.Actions[0].Resource.ID != "vm:42" || inbox.Actions[0].Resource.Type != unified.ResourceTypeVM {
		t.Fatalf("list resource projection=%#v", inbox.Actions)
	}

	detailRec := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/api/actions/act-resource-presentation", nil)
	detailReq.SetPathValue("id", record.ID)
	h.HandleGetAction(detailRec, detailReq)
	var detail actionDetailResponse
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); detailRec.Code != http.StatusOK || err != nil {
		t.Fatalf("detail status=%d body=%s err=%v", detailRec.Code, detailRec.Body.String(), err)
	}
	if detail.Audit.Resource == nil || detail.Audit.Resource.Name != "Checkout API" || detail.Audit.Request.ResourceID != "vm:42" {
		t.Fatalf("detail resource projection=%#v", detail.Audit)
	}
}

func TestHandleGetActionAndInboxAreTenantScoped(t *testing.T) {
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	now := time.Now().UTC()
	store, err := h.getStore("org-a")
	if err != nil {
		t.Fatal(err)
	}
	record := unified.ActionAuditRecord{
		ID: "act-org-a", CreatedAt: now, UpdatedAt: now, State: unified.ActionStatePending,
		Request: unified.ActionRequest{RequestID: "req-org-a", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "operator"},
		Plan:    unified.ActionPlan{ActionID: "act-org-a", RequestID: "req-org-a", Allowed: true, RequiresApproval: true, ApprovalPolicy: unified.ApprovalAdmin, PlannedAt: now, ExpiresAt: now.Add(time.Hour), ResourceVersion: "v1", PolicyVersion: "p1", PlanHash: "sha256:org-a"},
	}
	if _, _, err := store.CreateActionAudit(record, []unified.ActionLifecycleEvent{{ActionID: record.ID, Timestamp: now, State: unified.ActionStatePending, Actor: "operator", Message: "Pending approval."}}); err != nil {
		t.Fatal(err)
	}

	for _, orgID := range []string{"org-b", "default"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/actions/act-org-a", nil)
		req.SetPathValue("id", "act-org-a")
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
		h.HandleGetAction(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("cross-org detail status for %s=%d body=%s", orgID, rec.Code, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/actions/act-org-a", nil)
	req.SetPathValue("id", "act-org-a")
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	h.HandleGetAction(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	var detail actionlifecycle.ActionDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || detail.Audit.ID != record.ID {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/actions?view=pending", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-b"))
	h.HandleListActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cross-org list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var inbox actionInboxResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &inbox); err != nil || inbox.Count != 0 {
		t.Fatalf("cross-org inbox=%#v err=%v", inbox, err)
	}
}

func TestHandleListActionsRejectsUnknownViewAndUnsafeLimit(t *testing.T) {
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	for _, target := range []string{"/api/actions?view=unknown", "/api/actions?limit=501", "/api/actions?limit=-1"} {
		rec := httptest.NewRecorder()
		h.HandleListActions(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("target=%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleDecideActionApprovesPendingPlanWithoutExecution(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
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
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
				},
			},
		},
	})

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"agent-run-approve",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`))
	h.HandlePlanAction(planRec, actionHandlerTestRequest(planReq, ""))
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status = %d, body=%s", planRec.Code, planRec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan response: %v", err)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{
		"outcome":"approved",
		"reason":"inside maintenance window"
	}`))
	decisionReq.SetPathValue("id", plan.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, ""))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d, body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	var decision actionDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision response: %v", err)
	}
	if decision.ActionID != plan.ActionID || decision.State != unified.ActionStateApproved {
		t.Fatalf("decision identity/state = %q/%q, want %q/%q", decision.ActionID, decision.State, plan.ActionID, unified.ActionStateApproved)
	}
	if decision.Approval.Actor != "operator@example.com" || decision.Approval.Method != unified.MethodSession || decision.Approval.Outcome != unified.OutcomeApproved {
		t.Fatalf("decision approval = %#v", decision.Approval)
	}
	if decision.Audit.Result != nil {
		t.Fatalf("approval must not execute the action, got result %#v", decision.Audit.Result)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	audit, ok, err := store.GetActionAudit(plan.ActionID)
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unified.ActionStateApproved || len(audit.Approvals) != 1 || audit.Result != nil {
		t.Fatalf("persisted audit = %#v, ok=%v", audit, ok)
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unified.ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
		if event.State == unified.ActionStateExecuting || event.State == unified.ActionStateCompleted {
			t.Fatalf("approval must not create execution event: %#v", event)
		}
	}
	if len(events) != 4 || !seen[unified.ActionStatePlanned] || !seen[unified.ActionStatePending] || !seen[unified.ActionStateApproved] {
		t.Fatalf("events = %#v, want planned, pending_approval, approved", events)
	}

	exactRetryRec := httptest.NewRecorder()
	exactRetryReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{
		"outcome":"approved",
		"reason":"inside maintenance window"
	}`))
	exactRetryReq.SetPathValue("id", plan.ActionID)
	exactRetryReq = exactRetryReq.WithContext(auth.WithUser(exactRetryReq.Context(), "operator@example.com"))
	h.HandleDecideAction(exactRetryRec, actionHandlerTestRequest(exactRetryReq, ""))
	if exactRetryRec.Code != http.StatusOK {
		t.Fatalf("exact retry status = %d, body=%s", exactRetryRec.Code, exactRetryRec.Body.String())
	}
	audit, _, _ = store.GetActionAudit(plan.ActionID)
	events, _ = store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if len(audit.Approvals) != 1 || len(events) != 4 {
		t.Fatalf("exact retry duplicated state: approvals=%d events=%d", len(audit.Approvals), len(events))
	}

	retryRec := httptest.NewRecorder()
	retryReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{
		"outcome":"rejected",
		"reason":"late conflicting decision"
	}`))
	retryReq.SetPathValue("id", plan.ActionID)
	retryReq = retryReq.WithContext(auth.WithUser(retryReq.Context(), "second-operator@example.com"))
	h.HandleDecideAction(retryRec, actionHandlerTestRequest(retryReq, ""))
	if retryRec.Code != http.StatusConflict {
		t.Fatalf("retry decision status = %d, body=%s", retryRec.Code, retryRec.Body.String())
	}
	if !strings.Contains(retryRec.Body.String(), `"error":"action_not_pending"`) {
		t.Fatalf("retry decision body = %s", retryRec.Body.String())
	}
}

func TestHandleExecuteActionRunsApprovedPlanThroughExecutor(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
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
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
				},
			},
		},
	})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "restart dispatched"}}
	h.SetActionExecutor(executor)
	published := make(chan unified.ActionAuditRecord, 1)
	h.SetActionCompletedPublisher(func(record unified.ActionAuditRecord) {
		published <- record
	})

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"agent-run-execute",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`))
	h.HandlePlanAction(planRec, actionHandlerTestRequest(planReq, ""))
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status = %d, body=%s", planRec.Code, planRec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan response: %v", err)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{
		"outcome":"approved",
		"reason":"inside maintenance window"
	}`))
	decisionReq.SetPathValue("id", plan.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, ""))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d, body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/execute", bytes.NewBufferString(`{
		"reason":"execute during approved maintenance window"
	}`))
	executeReq.SetPathValue("id", plan.ActionID)
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, actionHandlerTestRequest(executeReq, ""))
	if executeRec.Code != http.StatusOK {
		t.Fatalf("execute status = %d, body=%s", executeRec.Code, executeRec.Body.String())
	}

	var execution actionExecutionResponse
	if err := json.Unmarshal(executeRec.Body.Bytes(), &execution); err != nil {
		t.Fatalf("decode execution response: %v", err)
	}
	if execution.ActionID != plan.ActionID || execution.State != unified.ActionStateCompleted {
		t.Fatalf("execution identity/state = %q/%q, want %q/%q", execution.ActionID, execution.State, plan.ActionID, unified.ActionStateCompleted)
	}
	if execution.Result == nil || !execution.Result.Success || execution.Result.Output != "restart dispatched" {
		t.Fatalf("execution result = %#v", execution.Result)
	}
	if executor.calls != 1 || executor.received.State != unified.ActionStateExecuting || executor.received.Result != nil {
		t.Fatalf("executor received = %#v after %d calls", executor.received, executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	audit, ok, err := store.GetActionAudit(plan.ActionID)
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unified.ActionStateCompleted || audit.Result == nil || audit.Result.Output != "restart dispatched" {
		t.Fatalf("persisted execution audit = %#v, ok=%v", audit, ok)
	}
	select {
	case eventRecord := <-published:
		if eventRecord.ID != plan.ActionID || eventRecord.State != unified.ActionStateCompleted {
			t.Fatalf("published action completion = %#v, want completed %q", eventRecord, plan.ActionID)
		}
	default:
		t.Fatal("expected API action execution to publish a terminal action completion event")
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unified.ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
	}
	if len(events) != 6 ||
		!seen[unified.ActionStatePlanned] ||
		!seen[unified.ActionStatePending] ||
		!seen[unified.ActionStateApproved] ||
		!seen[unified.ActionStateExecuting] ||
		!seen[unified.ActionStateCompleted] {
		t.Fatalf("events = %#v, want full planned-to-completed lifecycle", events)
	}
}

func TestHandleExecuteActionRejectsStalePlanBeforeExecutor(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	resource := unified.Resource{
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
				MinimumApprovalLevel: unified.ApprovalAdmin,
				InternalHandler:      "proxmox.vm.restart",
				Params: []unified.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			},
		},
	}
	provider := &mutableResourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
		freshness: now,
	}
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(provider)
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "should not run"}}
	h.SetActionExecutor(executor)
	published := make(chan unified.ActionAuditRecord, 1)
	h.SetActionCompletedPublisher(func(record unified.ActionAuditRecord) {
		published <- record
	})

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"agent-run-stale-plan",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`))
	h.HandlePlanAction(planRec, actionHandlerTestRequest(planReq, ""))
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status = %d, body=%s", planRec.Code, planRec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan response: %v", err)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{
		"outcome":"approved",
		"reason":"approved before resource changed"
	}`))
	decisionReq.SetPathValue("id", plan.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, ""))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d, body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	resource.Status = unified.StatusOnline
	resource.UpdatedAt = now.Add(time.Second)
	resource.Capabilities[0].MinimumApprovalLevel = unified.ApprovalMultiFactor
	provider = &mutableResourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now.Add(time.Second)},
		resources: []unified.Resource{resource},
		freshness: now.Add(time.Second),
	}
	h.SetStateProvider(provider)
	h.invalidateCache("default")

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	approvedAudit, ok, err := store.GetActionAudit(plan.ActionID)
	if err != nil {
		t.Fatalf("GetActionAudit before execute: %v", err)
	}
	if !ok {
		t.Fatal("expected approved action audit before execute")
	}
	if err := h.ActionLifecycle().ValidatePlanFresh("default", approvedAudit); !errors.Is(err, unified.ErrActionPlanDrift) {
		t.Fatalf("expected current resource contract to drift before execute, got %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", plan.ActionID)
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, actionHandlerTestRequest(executeReq, ""))
	if executeRec.Code != http.StatusConflict {
		t.Fatalf("execute status = %d, body=%s", executeRec.Code, executeRec.Body.String())
	}
	if !strings.Contains(executeRec.Body.String(), `"error":"action_plan_drift"`) {
		t.Fatalf("execute body = %s", executeRec.Body.String())
	}
	if executor.calls != 0 {
		t.Fatalf("stale plan should not call executor, calls=%d received=%#v", executor.calls, executor.received)
	}

	audit, ok, err := store.GetActionAudit(plan.ActionID)
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unified.ActionStateFailed || audit.Result == nil || !strings.HasPrefix(audit.Result.ErrorMessage, "plan_drift:") {
		t.Fatalf("stale-plan audit = %#v, ok=%v", audit, ok)
	}
	select {
	case eventRecord := <-published:
		if eventRecord.ID != plan.ActionID || eventRecord.State != unified.ActionStateFailed {
			t.Fatalf("published stale-plan completion = %#v, want failed %q", eventRecord, plan.ActionID)
		}
	default:
		t.Fatal("expected stale-plan refusal to publish a terminal action completion event")
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seenFailed := false
	for _, event := range events {
		if event.State == unified.ActionStateFailed && strings.HasPrefix(event.Message, "plan_drift:") {
			seenFailed = true
			break
		}
	}
	if !seenFailed {
		t.Fatalf("expected failed plan_drift lifecycle event, got %#v", events)
	}
}

func TestHandleExecuteActionWithoutExecutorLeavesApprovedAuditUnchanged(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	record := unified.ActionAuditRecord{
		ID:        "act_no_executor",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateApproved,
		Request:   boundActionTestRequest("req-no-executor", "vm:42", "restart", "Recover after confirmed outage", "agent:oncall-helper"),
		Plan: unified.ActionPlan{
			ActionID:            "act_no_executor",
			RequestID:           "req-no-executor",
			Allowed:             true,
			RequiresApproval:    true,
			ApprovalPolicy:      unified.ApprovalAdmin,
			ApprovalRequirement: unified.ApprovalRequirementForFloor(unified.ApprovalAdmin),
			PlannedAt:           now.Add(-time.Minute),
			ExpiresAt:           now.Add(5 * time.Minute),
			ResourceVersion:     "resource:sha256:test",
			PolicyVersion:       "policy:sha256:test",
			PlanHash:            "sha256:test",
		},
		Approvals: []unified.ActionApprovalRecord{boundActionTestApproval("act_no_executor", "sha256:test", "operator@example.com", now)},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/act_no_executor/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", "act_no_executor")
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, actionHandlerTestRequest(executeReq, ""))
	if executeRec.Code != http.StatusNotImplemented {
		t.Fatalf("execute status = %d, body=%s", executeRec.Code, executeRec.Body.String())
	}
	if !strings.Contains(executeRec.Body.String(), `"error":"action_executor_unavailable"`) {
		t.Fatalf("execute body = %s", executeRec.Body.String())
	}

	got, ok, err := store.GetActionAudit("act_no_executor")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || got.State != unified.ActionStateApproved || got.Result != nil {
		t.Fatalf("audit changed despite missing executor = %#v, ok=%v", got, ok)
	}
	events, err := store.GetActionLifecycleEvents("act_no_executor", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("missing executor must not append lifecycle events: %#v", events)
	}
}

func TestHandleExecuteActionRejectsDryRunOnlyPlan(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "should not run"}}
	h.SetActionExecutor(executor)
	published := make(chan unified.ActionAuditRecord, 1)
	h.SetActionCompletedPublisher(func(record unified.ActionAuditRecord) {
		published <- record
	})

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	record := unified.ActionAuditRecord{
		ID:        "act_dry_run_only",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStatePlanned,
		Request:   boundActionTestRequest("req-dry-run-only", "vm:42", "restart", "Validate restart path without execution", "agent:oncall-helper"),
		Plan: unified.ActionPlan{
			ActionID:            "act_dry_run_only",
			RequestID:           "req-dry-run-only",
			Allowed:             true,
			ApprovalPolicy:      unified.ApprovalDryRun,
			ApprovalRequirement: unified.ApprovalRequirementForFloor(unified.ApprovalDryRun),
			PlannedAt:           now.Add(-time.Minute),
			ExpiresAt:           now.Add(5 * time.Minute),
			ResourceVersion:     "resource:sha256:test",
			PolicyVersion:       "policy:sha256:test",
			PlanHash:            "sha256:test",
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/act_dry_run_only/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", "act_dry_run_only")
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, actionHandlerTestRequest(executeReq, ""))
	if executeRec.Code != http.StatusConflict {
		t.Fatalf("execute status = %d, body=%s", executeRec.Code, executeRec.Body.String())
	}
	if !strings.Contains(executeRec.Body.String(), `"error":"action_dry_run_only"`) {
		t.Fatalf("execute body = %s", executeRec.Body.String())
	}
	if executor.calls != 0 {
		t.Fatalf("dry-run-only plan should not call executor, calls=%d received=%#v", executor.calls, executor.received)
	}

	got, ok, err := store.GetActionAudit("act_dry_run_only")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || got.State != unified.ActionStateFailed || got.Result == nil || got.Result.Success || !strings.HasPrefix(got.Result.ErrorMessage, "action_dry_run_only:") {
		t.Fatalf("dry-run-only audit = %#v, ok=%v", got, ok)
	}
	select {
	case eventRecord := <-published:
		if eventRecord.ID != "act_dry_run_only" || eventRecord.State != unified.ActionStateFailed {
			t.Fatalf("published dry-run-only completion = %#v", eventRecord)
		}
	default:
		t.Fatal("expected dry-run-only refusal to publish a terminal action completion event")
	}
	events, err := store.GetActionLifecycleEvents("act_dry_run_only", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 1 || events[0].State != unified.ActionStateFailed || !strings.HasPrefix(events[0].Message, "action_dry_run_only:") {
		t.Fatalf("expected one failed dry-run-only lifecycle event, got %#v", events)
	}
}

func TestHandleExecuteActionMaterializesExplicitExpiredState(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "should not run"}}
	h.SetActionExecutor(executor)
	published := make(chan unified.ActionAuditRecord, 1)
	h.SetActionCompletedPublisher(func(record unified.ActionAuditRecord) {
		published <- record
	})

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	record := unified.ActionAuditRecord{
		ID:        "act_expired",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-6 * time.Minute),
		State:     unified.ActionStateApproved,
		Request:   boundActionTestRequest("req-expired", "vm:42", "restart", "Recover after confirmed outage", "agent:oncall-helper"),
		Plan: unified.ActionPlan{
			ActionID:            "act_expired",
			RequestID:           "req-expired",
			Allowed:             true,
			RequiresApproval:    true,
			ApprovalPolicy:      unified.ApprovalAdmin,
			ApprovalRequirement: unified.ApprovalRequirementForFloor(unified.ApprovalAdmin),
			PlannedAt:           now.Add(-10 * time.Minute),
			ExpiresAt:           now.Add(-5 * time.Minute),
			ResourceVersion:     "resource:sha256:test",
			PolicyVersion:       "policy:sha256:test",
			PlanHash:            "sha256:test",
		},
		Approvals: []unified.ActionApprovalRecord{boundActionTestApproval("act_expired", "sha256:test", "operator@example.com", now.Add(-6*time.Minute))},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/act_expired/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", "act_expired")
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, actionHandlerTestRequest(executeReq, ""))
	if executeRec.Code != http.StatusConflict {
		t.Fatalf("execute status = %d, body=%s", executeRec.Code, executeRec.Body.String())
	}
	if !strings.Contains(executeRec.Body.String(), `"error":"action_plan_expired"`) {
		t.Fatalf("execute body = %s", executeRec.Body.String())
	}
	if executor.calls != 0 {
		t.Fatalf("expired plan should not call executor, calls=%d received=%#v", executor.calls, executor.received)
	}

	got, ok, err := store.GetActionAudit("act_expired")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || got.State != unified.ActionStateExpired || got.Result != nil {
		t.Fatalf("expired-plan audit = %#v, ok=%v", got, ok)
	}
	select {
	case eventRecord := <-published:
		t.Fatalf("expiry must not publish fabricated completion truth: %#v", eventRecord)
	default:
	}
	events, err := store.GetActionLifecycleEvents("act_expired", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 1 || events[0].State != unified.ActionStateExpired || events[0].Message != "Action plan expired before dispatch." {
		t.Fatalf("expected one explicit expiry lifecycle event, got %#v", events)
	}
}

func TestPersistActionPlanAuditRejectsOrphanLifecycleState(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	store := unified.NewMemoryStore()
	req := unified.ActionRequest{
		RequestID:      "agent-run-123",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:oncall-helper",
	}
	plan := unified.ActionPlan{
		ActionID:         "act_partial",
		RequestID:        "agent-run-123",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unified.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		ResourceVersion:  "resource:sha256:test",
		PolicyVersion:    "policy:sha256:test",
		PlanHash:         "sha256:test",
	}
	if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
		ActionID:  plan.ActionID,
		Timestamp: now,
		State:     unified.ActionStatePlanned,
		Actor:     req.RequestedBy,
		Message:   "Action plan created.",
	}); err != nil {
		t.Fatalf("seed lifecycle event: %v", err)
	}

	if err := actionlifecycle.PersistPlanAudit(store, req, plan); err == nil {
		t.Fatal("orphan lifecycle state must make atomic plan creation fail")
	}
	if _, found, err := store.GetActionAudit(plan.ActionID); err != nil || found {
		t.Fatalf("atomic creation left audit behind: found=%v err=%v", found, err)
	}
}

func TestHandlePlanActionRejectsMissingCapability(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{ID: "vm:42", Type: unified.ResourceTypeVM, Name: "web-42", Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now},
		},
	})
	body := bytes.NewBufferString(`{
		"requestId":"agent-run-123",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body)
	h.HandlePlanAction(rec, actionHandlerTestRequest(req, ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"capability_not_found"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
