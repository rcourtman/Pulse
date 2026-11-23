package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type harnessRelayClient struct {
	mu            sync.Mutex
	notifications []relay.PushNotificationPayload
}

func (c *harnessRelayClient) Status() relay.ClientStatus { return relay.ClientStatus{} }
func (c *harnessRelayClient) Close()                     {}
func (c *harnessRelayClient) SendPushNotification(n relay.PushNotificationPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, n)
	return nil
}

func (c *harnessRelayClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.notifications)
}

func (c *harnessRelayClient) list() []relay.PushNotificationPayload {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]relay.PushNotificationPayload, len(c.notifications))
	copy(out, c.notifications)
	return out
}

func harnessSeedResource(now time.Time) unified.Resource {
	return unified.Resource{
		ID:        "vm:42",
		Type:      unified.ResourceTypeVM,
		Name:      "web-42",
		Status:    unified.StatusWarning,
		LastSeen:  now,
		UpdatedAt: now,
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Restart this VM",
				MinimumApprovalLevel: unified.ApprovalAdmin,
			},
		},
	}
}

func harnessAPITokenRequest(req *http.Request, token *config.APITokenRecord) *http.Request {
	attachAPITokenRecord(req, token)
	ctx := auth.WithUser(req.Context(), apiTokenAuthenticatedUser(token))
	ctx = auth.WithAPIToken(ctx, token)
	return req.WithContext(ctx)
}

func TestActionActorForRequestCarriesTokenCredentialLabel(t *testing.T) {
	token := &config.APITokenRecord{
		ID:     "tok-label-1",
		Name:   "  Claude Code CLI  ",
		Hash:   "sha256:never-expose-hash",
		Prefix: "pls_secret",
		OrgID:  "default",
		Scopes: []string{config.ScopeActionsPlan},
	}
	setAPITokenOwnerUserID(token, "alice")
	req := harnessAPITokenRequest(httptest.NewRequest(http.MethodPost, "/api/actions/plan", nil), token)

	actor, err := actionActorForRequest(&config.Config{}, req, "default")
	if err != nil {
		t.Fatalf("actionActorForRequest error = %v", err)
	}
	if actor.Kind != unified.ActionActorAPIToken {
		t.Fatalf("kind = %q, want %q", actor.Kind, unified.ActionActorAPIToken)
	}
	if actor.SubjectID != "alice" {
		t.Fatalf("subjectId = %q, want alice", actor.SubjectID)
	}
	if actor.CredentialID != "api-token:tok-label-1" {
		t.Fatalf("credentialId = %q, want api-token:tok-label-1", actor.CredentialID)
	}
	if actor.CredentialLabel != "Claude Code CLI" {
		t.Fatalf("credentialLabel = %q, want %q", actor.CredentialLabel, "Claude Code CLI")
	}
}

func TestHandlePlanActionExternalAgentDefaultsAndSeparation(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{DataPath: t.TempDir()}
	h := newActionTestResourceHandlers(t, cfg)
	h.SetActionExecutor(&stubActionExecutor{result: &unified.ExecutionResult{Success: true}})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{harnessSeedResource(now)},
	})

	token := &config.APITokenRecord{
		ID:     "tok-ext-1",
		Name:   "Claude Code",
		OrgID:  "default",
		Scopes: []string{config.ScopeActionsPlan, config.ScopeActionsApprove},
	}
	setAPITokenOwnerUserID(token, "alice")

	surfaceCases := []struct {
		name        string
		requestID   string
		headerValue string
		wantSurface string
	}{
		{name: "mcp surface", requestID: "req-mcp", headerValue: agentcapabilities.AgentSurfacePulseMCP, wantSurface: "pulse_mcp"},
		{name: "cli surface", requestID: "req-cli", headerValue: agentcapabilities.AgentSurfacePulseCLI, wantSurface: "pulse_cli"},
		{name: "default api token surface", requestID: "req-default", headerValue: "", wantSurface: "api_token"},
		{name: "unknown header falls back to api_token", requestID: "req-unknown", headerValue: "custom-bot", wantSurface: "api_token"},
	}

	var mcpPlan unified.ActionPlan
	for _, tc := range surfaceCases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"requestId":"` + tc.requestID + `","resourceId":"vm:42","capabilityName":"restart","reason":"Recover stuck worker"}`
			req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(body))
			if tc.headerValue != "" {
				req.Header.Set(agentcapabilities.AgentSurfaceHeader, tc.headerValue)
			}
			req = harnessAPITokenRequest(req, token)
			rec := httptest.NewRecorder()
			h.HandlePlanAction(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}

			var plan unified.ActionPlan
			if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
				t.Fatalf("decode plan: %v", err)
			}
			if !plan.ApprovalRequirement.DisallowRequester {
				t.Fatal("expected DisallowRequester=true for API token plan")
			}
			if ttl := plan.ExpiresAt.Sub(plan.PlannedAt); ttl != actionplanner.DefaultExternalPlanTTL {
				t.Fatalf("TTL = %v, want %v", ttl, actionplanner.DefaultExternalPlanTTL)
			}

			record, found, err := h.ActionLifecycle().Get("default", plan.ActionID)
			if err != nil || !found {
				t.Fatalf("Get(%q): found=%v err=%v", plan.ActionID, found, err)
			}
			if record.Origin == nil || record.Origin.Surface != tc.wantSurface {
				t.Fatalf("origin = %#v, want surface %q", record.Origin, tc.wantSurface)
			}
			if record.Request.Actor.CredentialLabel != "Claude Code" {
				t.Fatalf("credentialLabel = %q, want Claude Code", record.Request.Actor.CredentialLabel)
			}
			if err := h.ActionLifecycle().ValidatePlanFresh("default", record); err != nil {
				t.Fatalf("ValidatePlanFresh failed on external-agent plan: %v", err)
			}
			if tc.requestID == "req-mcp" {
				mcpPlan = plan
			}
		})
	}

	// Browser-session UI plan must preserve Origin: nil, 5m TTL, and DisallowRequester: false.
	uiReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{"requestId":"req-ui","resourceId":"vm:42","capabilityName":"restart","reason":"Manual restart"}`))
	uiRec := httptest.NewRecorder()
	h.HandlePlanAction(uiRec, actionHandlerTestRequest(uiReq, "alice"))
	if uiRec.Code != http.StatusOK {
		t.Fatalf("UI plan status = %d, body = %s", uiRec.Code, uiRec.Body.String())
	}
	var uiPlan unified.ActionPlan
	if err := json.Unmarshal(uiRec.Body.Bytes(), &uiPlan); err != nil {
		t.Fatalf("decode ui plan: %v", err)
	}
	if uiPlan.ApprovalRequirement.DisallowRequester {
		t.Fatal("browser-session UI plan must keep DisallowRequester=false")
	}
	if ttl := uiPlan.ExpiresAt.Sub(uiPlan.PlannedAt); ttl != actionplanner.DefaultPlanTTL {
		t.Fatalf("UI plan TTL = %v, want %v", ttl, actionplanner.DefaultPlanTTL)
	}
	uiRecord, found, err := h.ActionLifecycle().Get("default", uiPlan.ActionID)
	if err != nil || !found {
		t.Fatalf("Get UI plan: found=%v err=%v", found, err)
	}
	if uiRecord.Origin != nil {
		t.Fatalf("UI plan origin = %#v, want nil", uiRecord.Origin)
	}

	// Self-approval by the requesting actor must be rejected with 403 action_separation_required.
	selfDecideReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+mcpPlan.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"self approve","planHash":"`+mcpPlan.PlanHash+`"}`))
	selfDecideReq.SetPathValue("id", mcpPlan.ActionID)
	selfDecideReq = harnessAPITokenRequest(selfDecideReq, token)
	selfDecideRec := httptest.NewRecorder()
	h.HandleDecideAction(selfDecideRec, selfDecideReq)
	if selfDecideRec.Code != http.StatusForbidden {
		t.Fatalf("self-approval status = %d, want 403, body = %s", selfDecideRec.Code, selfDecideRec.Body.String())
	}
	if !strings.Contains(selfDecideRec.Body.String(), agentcapabilities.AgentErrCodeActionSeparationRequired) {
		t.Fatalf("self-approval error code mismatch: %s", selfDecideRec.Body.String())
	}

	// Approval by a different human operator must succeed.
	otherDecideReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+mcpPlan.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"second operator reviewed","planHash":"`+mcpPlan.PlanHash+`"}`))
	otherDecideReq.SetPathValue("id", mcpPlan.ActionID)
	otherDecideReq = actionHandlerTestRequest(otherDecideReq, "bob")
	otherDecideRec := httptest.NewRecorder()
	h.HandleDecideAction(otherDecideRec, otherDecideReq)
	if otherDecideRec.Code != http.StatusOK {
		t.Fatalf("second-operator approval status = %d, body = %s", otherDecideRec.Code, otherDecideRec.Body.String())
	}
	var decided actionDecisionResponse
	if err := json.Unmarshal(otherDecideRec.Body.Bytes(), &decided); err != nil {
		t.Fatalf("decode decision response: %v", err)
	}
	if decided.State != unified.ActionStateApproved {
		t.Fatalf("state = %q, want %q", decided.State, unified.ActionStateApproved)
	}
}

func TestActionsPlanTokenReadOwnPlansOnly(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	rawTokenA := "pls_plan_only_token_a_1234567890"
	rawTokenB := "pls_plan_only_token_b_1234567890"
	rawApproveToken := "pls_approve_token_c_1234567890"

	tokenA := config.APITokenRecord{
		ID:     "tok-plan-a",
		Name:   "Agent Alpha",
		Hash:   auth.HashAPIToken(rawTokenA),
		Prefix: rawTokenA[:8],
		OrgID:  "default",
		Scopes: []string{config.ScopeActionsPlan, config.ScopeMonitoringRead},
	}
	setAPITokenOwnerUserID(&tokenA, "alice")

	tokenB := config.APITokenRecord{
		ID:     "tok-plan-b",
		Name:   "Agent Beta",
		Hash:   auth.HashAPIToken(rawTokenB),
		Prefix: rawTokenB[:8],
		OrgID:  "default",
		Scopes: []string{config.ScopeActionsPlan, config.ScopeMonitoringRead},
	}
	setAPITokenOwnerUserID(&tokenB, "alice")

	approveToken := config.APITokenRecord{
		ID:     "tok-approve",
		Name:   "Approver Bot",
		Hash:   auth.HashAPIToken(rawApproveToken),
		Prefix: rawApproveToken[:8],
		OrgID:  "default",
		Scopes: []string{config.ScopeActionsApprove},
	}
	setAPITokenOwnerUserID(&approveToken, "bob")

	cfg := &config.Config{
		DataPath:  t.TempDir(),
		AuthUser:  "admin",
		APITokens: []config.APITokenRecord{tokenA, tokenB, approveToken},
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "6.0.0-test")
	t.Cleanup(func() {
		router.ShutdownBackgroundWorkers()
		router.ShutdownResourceStores()
	})
	router.resourceHandlers.SetActionExecutor(&stubActionExecutor{result: &unified.ExecutionResult{Success: true}})
	router.resourceHandlers.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{harnessSeedResource(now)},
	})

	planWithToken := func(rawToken, reqID string) unified.ActionPlan {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{"requestId":"`+reqID+`","resourceId":"vm:42","capabilityName":"restart","reason":"Test own plan visibility"}`))
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("plan (%s) status = %d, body = %s", reqID, rec.Code, rec.Body.String())
		}
		var plan unified.ActionPlan
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatalf("decode plan: %v", err)
		}
		return plan
	}

	planA := planWithToken(rawTokenA, "req-plan-a")
	planB := planWithToken(rawTokenB, "req-plan-b")

	// Plan-only token A lists actions: receives ONLY planA with readOnly: true.
	listReqA := httptest.NewRequest(http.MethodGet, "/api/actions?view=pending", nil)
	listReqA.Header.Set("X-API-Token", rawTokenA)
	listRecA := httptest.NewRecorder()
	router.Handler().ServeHTTP(listRecA, listReqA)
	if listRecA.Code != http.StatusOK {
		t.Fatalf("list A status = %d, body = %s", listRecA.Code, listRecA.Body.String())
	}
	var listA actionInboxResponse
	if err := json.Unmarshal(listRecA.Body.Bytes(), &listA); err != nil {
		t.Fatalf("decode list A: %v", err)
	}
	if !listA.ReadOnly {
		t.Fatal("expected readOnly=true for actions:plan-only list response")
	}
	if listA.Count != 1 || len(listA.Actions) != 1 || listA.Actions[0].ID != planA.ActionID {
		t.Fatalf("token A saw actions = %+v, want only %q", listA.Actions, planA.ActionID)
	}

	// Plan-only token A reads its own plan detail: 200 OK with readOnly: true.
	getOwnReq := httptest.NewRequest(http.MethodGet, "/api/actions/"+planA.ActionID, nil)
	getOwnReq.Header.Set("X-API-Token", rawTokenA)
	getOwnRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(getOwnRec, getOwnReq)
	if getOwnRec.Code != http.StatusOK {
		t.Fatalf("get own plan status = %d, body = %s", getOwnRec.Code, getOwnRec.Body.String())
	}
	var ownDetail actionDetailResponse
	if err := json.Unmarshal(getOwnRec.Body.Bytes(), &ownDetail); err != nil {
		t.Fatalf("decode own detail: %v", err)
	}
	if !ownDetail.ReadOnly {
		t.Fatal("expected readOnly=true on own plan detail for actions:plan-only token")
	}
	if ownDetail.Audit.ID != planA.ActionID {
		t.Fatalf("detail id = %q, want %q", ownDetail.Audit.ID, planA.ActionID)
	}

	// Plan-only token A tries to read token B's plan: 403 forbidden.
	getForeignReq := httptest.NewRequest(http.MethodGet, "/api/actions/"+planB.ActionID, nil)
	getForeignReq.Header.Set("X-API-Token", rawTokenA)
	getForeignRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(getForeignRec, getForeignReq)
	if getForeignRec.Code != http.StatusForbidden {
		t.Fatalf("get foreign plan status = %d, want 403, body = %s", getForeignRec.Code, getForeignRec.Body.String())
	}
	var errResp agentcapabilities.ErrorEnvelope
	if err := json.Unmarshal(getForeignRec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode 403 error: %v", err)
	}
	if errResp.Error != agentcapabilities.AgentErrCodeActionReadForbidden {
		t.Fatalf("403 error code = %q, want %q", errResp.Error, agentcapabilities.AgentErrCodeActionReadForbidden)
	}
	if strings.Contains(errResp.Message, "\u2014") || strings.Contains(errResp.Message, ";") {
		t.Fatalf("error message must not contain em-dash or semicolon: %q", errResp.Message)
	}

	// Plan-only token A cannot access the approver-only pending queue.
	pendingReq := httptest.NewRequest(http.MethodGet, "/api/actions/pending", nil)
	pendingReq.Header.Set("X-API-Token", rawTokenA)
	pendingRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(pendingRec, pendingReq)
	if pendingRec.Code != http.StatusForbidden {
		t.Fatalf("pending queue status for plan-only token = %d, want 403", pendingRec.Code)
	}

	// Approver token sees both plans with readOnly: false.
	listApproveReq := httptest.NewRequest(http.MethodGet, "/api/actions?view=pending", nil)
	listApproveReq.Header.Set("X-API-Token", rawApproveToken)
	listApproveRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(listApproveRec, listApproveReq)
	if listApproveRec.Code != http.StatusOK {
		t.Fatalf("approver list status = %d, body = %s", listApproveRec.Code, listApproveRec.Body.String())
	}
	var approveList actionInboxResponse
	if err := json.Unmarshal(listApproveRec.Body.Bytes(), &approveList); err != nil {
		t.Fatalf("decode approver list: %v", err)
	}
	if approveList.ReadOnly {
		t.Fatal("approver list must have readOnly=false")
	}
	if approveList.Count != 2 {
		t.Fatalf("approver count = %d, want 2", approveList.Count)
	}

	getApproveReq := httptest.NewRequest(http.MethodGet, "/api/actions/"+planA.ActionID, nil)
	getApproveReq.Header.Set("X-API-Token", rawApproveToken)
	getApproveRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(getApproveRec, getApproveReq)
	if getApproveRec.Code != http.StatusOK {
		t.Fatalf("approver get status = %d, body = %s", getApproveRec.Code, getApproveRec.Body.String())
	}
	var approveDetail actionDetailResponse
	if err := json.Unmarshal(getApproveRec.Body.Bytes(), &approveDetail); err != nil {
		t.Fatalf("decode approver detail: %v", err)
	}
	if approveDetail.ReadOnly {
		t.Fatal("approver detail must have readOnly=false")
	}
}

func TestRouterActionTransitionPublishesOrgScopedSSEAndOperatorNotifications(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	cfg := &config.Config{DataPath: t.TempDir()}
	router := NewRouter(cfg, nil, nil, nil, nil, "6.0.0-test")
	t.Cleanup(func() {
		router.ShutdownBackgroundWorkers()
		router.ShutdownResourceStores()
	})
	router.resourceHandlers.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{harnessSeedResource(now)},
	})

	relayClient := &harnessRelayClient{}
	router.relayClient = relayClient

	var alertMu sync.Mutex
	var capturedAlerts []struct {
		orgID string
		alert *alerts.Alert
	}
	router.externalAgentAlertNotifier = func(orgID string, alert *alerts.Alert) {
		alertMu.Lock()
		defer alertMu.Unlock()
		capturedAlerts = append(capturedAlerts, struct {
			orgID string
			alert *alerts.Alert
		}{orgID: orgID, alert: alert})
	}

	events, unsub := router.agentEventBroadcaster.Subscribe()
	defer unsub()

	extPending := unified.ActionAuditRecord{
		ID:        "act_ext_notify_1",
		CreatedAt: now,
		UpdatedAt: now,
		State:     unified.ActionStatePending,
		Origin:    &unified.ActionOrigin{Surface: "pulse_mcp"},
		Request: unified.ActionRequest{
			RequestID:      "req-ext-notify-1",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "Recover stuck worker",
			RequestedBy:    "alice",
			Actor: unified.ActionActor{
				SubjectID:       "alice",
				Kind:            unified.ActionActorAPIToken,
				CredentialID:    "api-token:tok-1",
				CredentialLabel: "Claude Code",
				OrgID:           "default",
			},
		},
		Plan: unified.ActionPlan{
			ActionID:       "act_ext_notify_1",
			ApprovalPolicy: unified.ApprovalAdmin,
			PlannedAt:      now,
			ExpiresAt:      now.Add(actionplanner.DefaultExternalPlanTTL),
		},
	}

	// First transition publishes SSE, mobile push, and operator alert.
	router.handleActionTransition("default", extPending)
	// Repeated transition for the same action ID must be deduped for notifications.
	router.handleActionTransition("default", extPending)

	select {
	case event := <-events:
		if event.Kind != AgentEventApprovalPending {
			t.Fatalf("kind = %q, want %q", event.Kind, AgentEventApprovalPending)
		}
		if !event.visibleToOrg("default") || event.visibleToOrg("org-beta") {
			t.Fatal("default org approval.pending must not leak to org-beta")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for approval.pending SSE event")
	}
	// Drain second SSE event from repeated transition.
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("expected second SSE event")
	}

	if got := relayClient.count(); got != 1 {
		t.Fatalf("relay push count = %d, want 1 (deduped per action ID)", got)
	}
	push := relayClient.list()[0]
	if push.ActionID != "act_ext_notify_1" || push.ActionType != relay.PushActionDecideAction {
		t.Fatalf("unexpected relay push payload: %+v", push)
	}
	if strings.Contains(push.Title, "\u2014") || strings.Contains(push.Title, ";") || strings.Contains(push.Body, "\u2014") || strings.Contains(push.Body, ";") {
		t.Fatalf("relay push copy must not contain em-dash or semicolon: %+v", push)
	}

	alertMu.Lock()
	alertCount := len(capturedAlerts)
	firstAlert := capturedAlerts[0]
	alertMu.Unlock()
	if alertCount != 1 {
		t.Fatalf("operator alert count = %d, want 1 (deduped per action ID)", alertCount)
	}
	if firstAlert.orgID != "default" || firstAlert.alert == nil {
		t.Fatalf("unexpected first alert: %+v", firstAlert)
	}
	msg := firstAlert.alert.Message
	for _, want := range []string{"Claude Code", "restart", "web-42", "/actions?action=act_ext_notify_1"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("operator alert message %q missing %q", msg, want)
		}
	}
	if strings.Contains(msg, "\u2014") || strings.Contains(msg, ";") {
		t.Fatalf("operator alert message must not contain em-dash or semicolon: %q", msg)
	}

	// Non-default org external-agent plan: org-scoped SSE + org-beta alert, NO default-org relay push.
	tenantPending := extPending
	tenantPending.ID = "act_ext_tenant_1"
	tenantPending.Request.Actor.OrgID = "org-beta"
	router.handleActionTransition("org-beta", tenantPending)

	select {
	case event := <-events:
		if event.Kind != AgentEventApprovalPending {
			t.Fatalf("tenant kind = %q, want %q", event.Kind, AgentEventApprovalPending)
		}
		if !event.visibleToOrg("org-beta") || event.visibleToOrg("default") {
			t.Fatal("org-beta approval.pending must stay isolated to org-beta")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for org-beta approval.pending")
	}
	if got := relayClient.count(); got != 1 {
		t.Fatalf("relay push count after non-default org transition = %d, want 1", got)
	}
	alertMu.Lock()
	if len(capturedAlerts) != 2 || capturedAlerts[1].orgID != "org-beta" {
		t.Fatalf("expected second alert for org-beta, got %+v", capturedAlerts)
	}
	alertMu.Unlock()

	// Non-external-agent plans (UI nil origin and Patrol origin) must NOT trigger external-agent notifications.
	uiPending := extPending
	uiPending.ID = "act_ui_1"
	uiPending.Origin = nil
	router.handleActionTransition("default", uiPending)
	<-events

	patrolPending := extPending
	patrolPending.ID = "act_patrol_1"
	patrolPending.Origin = &unified.ActionOrigin{Surface: "patrol"}
	router.handleActionTransition("default", patrolPending)
	<-events

	if got := relayClient.count(); got != 1 {
		t.Fatalf("non-external plans triggered relay push: count = %d, want 1", got)
	}
	alertMu.Lock()
	if len(capturedAlerts) != 2 {
		t.Fatalf("non-external plans triggered operator alert: count = %d, want 2", len(capturedAlerts))
	}
	alertMu.Unlock()

	// Approved and rejected transitions publish org-scoped action.approved and action.rejected SSE events.
	approved := extPending
	approved.State = unified.ActionStateApproved
	approved.Approvals = []unified.ActionApprovalRecord{{Actor: "bob", Outcome: unified.OutcomeApproved, Timestamp: now.Add(time.Minute)}}
	router.handleActionTransition("default", approved)
	select {
	case event := <-events:
		if event.Kind != AgentEventActionApproved || !event.visibleToOrg("default") || event.visibleToOrg("org-beta") {
			t.Fatalf("unexpected approved SSE event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action.approved")
	}

	rejected := tenantPending
	rejected.State = unified.ActionStateRejected
	rejected.Approvals = []unified.ActionApprovalRecord{{Actor: "carol", Outcome: unified.OutcomeRejected, Reason: "no", Timestamp: now.Add(2 * time.Minute)}}
	router.handleActionTransition("org-beta", rejected)
	select {
	case event := <-events:
		if event.Kind != AgentEventActionRejected || !event.visibleToOrg("org-beta") || event.visibleToOrg("default") {
			t.Fatalf("unexpected rejected SSE event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action.rejected")
	}

	// Safe with nil relayClient and nil NotificationManager.
	nilRouter := &Router{agentEventBroadcaster: NewAgentEventBroadcaster()}
	nilRouter.handleActionTransition("default", extPending)
}
