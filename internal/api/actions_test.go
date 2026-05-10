package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

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

func TestHandleDecideActionApprovesPendingPlanWithoutExecution(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
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

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"agent-run-approve",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`))
	h.HandlePlanAction(planRec, planReq)
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
	h.HandleDecideAction(decisionRec, decisionReq)
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
	if decision.Approval.Actor != "operator@example.com" || decision.Approval.Method != unified.MethodAPI || decision.Approval.Outcome != unified.OutcomeApproved {
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
	if len(events) != 3 || !seen[unified.ActionStatePlanned] || !seen[unified.ActionStatePending] || !seen[unified.ActionStateApproved] {
		t.Fatalf("events = %#v, want planned, pending_approval, approved", events)
	}

	retryRec := httptest.NewRecorder()
	retryReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{
		"outcome":"rejected",
		"reason":"late conflicting decision"
	}`))
	retryReq.SetPathValue("id", plan.ActionID)
	retryReq = retryReq.WithContext(auth.WithUser(retryReq.Context(), "second-operator@example.com"))
	h.HandleDecideAction(retryRec, retryReq)
	if retryRec.Code != http.StatusConflict {
		t.Fatalf("retry decision status = %d, body=%s", retryRec.Code, retryRec.Body.String())
	}
	if !strings.Contains(retryRec.Body.String(), `"error":"action_not_pending"`) {
		t.Fatalf("retry decision body = %s", retryRec.Body.String())
	}
}

func TestHandleExecuteActionRunsApprovedPlanThroughExecutor(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
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
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "restart dispatched"}}
	h.SetActionExecutor(executor)

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"agent-run-execute",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`))
	h.HandlePlanAction(planRec, planReq)
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
	h.HandleDecideAction(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d, body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/execute", bytes.NewBufferString(`{
		"reason":"execute during approved maintenance window"
	}`))
	executeReq.SetPathValue("id", plan.ActionID)
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, executeReq)
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
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unified.ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
	}
	if len(events) != 5 ||
		!seen[unified.ActionStatePlanned] ||
		!seen[unified.ActionStatePending] ||
		!seen[unified.ActionStateApproved] ||
		!seen[unified.ActionStateExecuting] ||
		!seen[unified.ActionStateCompleted] {
		t.Fatalf("events = %#v, want full planned-to-completed lifecycle", events)
	}
}

func TestHandleExecuteActionWithoutExecutorLeavesApprovedAuditUnchanged(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	record := unified.ActionAuditRecord{
		ID:        "act_no_executor",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateApproved,
		Request: unified.ActionRequest{
			RequestID:      "req-no-executor",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "Recover after confirmed outage",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: unified.ActionPlan{
			ActionID:         "act_no_executor",
			RequestID:        "req-no-executor",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   unified.ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(5 * time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
		Approvals: []unified.ActionApprovalRecord{
			{
				Actor:     "operator@example.com",
				Method:    unified.MethodAPI,
				Timestamp: now,
				Outcome:   unified.OutcomeApproved,
				Reason:    "approved for proof",
			},
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/act_no_executor/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", "act_no_executor")
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, executeReq)
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
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true, Output: "should not run"}}
	h.SetActionExecutor(executor)

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	record := unified.ActionAuditRecord{
		ID:        "act_dry_run_only",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStatePlanned,
		Request: unified.ActionRequest{
			RequestID:      "req-dry-run-only",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "Validate restart path without execution",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: unified.ActionPlan{
			ActionID:        "act_dry_run_only",
			RequestID:       "req-dry-run-only",
			Allowed:         true,
			ApprovalPolicy:  unified.ApprovalDryRun,
			PlannedAt:       now.Add(-time.Minute),
			ExpiresAt:       now.Add(5 * time.Minute),
			ResourceVersion: "resource:sha256:test",
			PolicyVersion:   "policy:sha256:test",
			PlanHash:        "sha256:test",
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	executeRec := httptest.NewRecorder()
	executeReq := httptest.NewRequest(http.MethodPost, "/api/actions/act_dry_run_only/execute", bytes.NewBufferString(`{}`))
	executeReq.SetPathValue("id", "act_dry_run_only")
	executeReq = executeReq.WithContext(auth.WithUser(executeReq.Context(), "operator@example.com"))
	h.HandleExecuteAction(executeRec, executeReq)
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
	if !ok || got.State != unified.ActionStatePlanned || got.Result != nil {
		t.Fatalf("dry-run-only audit changed = %#v, ok=%v", got, ok)
	}
	events, err := store.GetActionLifecycleEvents("act_dry_run_only", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("dry-run-only execution must not append lifecycle events: %#v", events)
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
	if !strings.Contains(rec.Body.String(), `"error":"capability_not_found"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
