package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestHandlePlanActionReturnsCanonicalPlan(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
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
	h.HandlePlanAction(rec, req)

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
	if plan.Preflight == nil || plan.Preflight.Target != "vm:42" {
		t.Fatalf("Preflight = %#v, want target vm:42", plan.Preflight)
	}
	if len(plan.PredictedBlastRadius) != 2 || plan.PredictedBlastRadius[0] != "vm:42" || plan.PredictedBlastRadius[1] != "node-1" {
		t.Fatalf("PredictedBlastRadius = %#v", plan.PredictedBlastRadius)
	}
}

func TestHandlePlanActionPersistsAuditAndLifecycle(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
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
	h.HandlePlanAction(rec, req)
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
	if audit.Request.RequestID != "agent-run-123" || audit.Request.RequestedBy != "agent:oncall-helper" {
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
		if event.Actor != "agent:oncall-helper" {
			t.Fatalf("event actor = %q, want requester", event.Actor)
		}
	}
	if len(events) != 2 || !seenStates[unified.ActionStatePlanned] || !seenStates[unified.ActionStatePending] {
		t.Fatalf("events = %#v, want planned and pending_approval", events)
	}

	retryRec := httptest.NewRecorder()
	retryReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body())
	h.HandlePlanAction(retryRec, retryReq)
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
}

func TestPersistActionPlanAuditFillsMissingLifecycleState(t *testing.T) {
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

	if err := persistActionPlanAudit(store, req, plan); err != nil {
		t.Fatalf("persistActionPlanAudit: %v", err)
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seenStates := map[unified.ActionState]bool{}
	for _, event := range events {
		seenStates[event.State] = true
	}
	if len(events) != 2 || !seenStates[unified.ActionStatePlanned] || !seenStates[unified.ActionStatePending] {
		t.Fatalf("events = %#v, want one planned and one pending event", events)
	}

	if err := persistActionPlanAudit(store, req, plan); err != nil {
		t.Fatalf("persistActionPlanAudit retry: %v", err)
	}
	events, err = store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents retry: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("retry duplicated lifecycle events: %#v", events)
	}
}

func TestHandlePlanActionRejectsMissingCapability(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
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
	h.HandlePlanAction(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"capability_not_found"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
