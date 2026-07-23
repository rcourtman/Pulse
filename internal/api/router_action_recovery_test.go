package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// callbackLossDockerExecutor simulates a server that dies (or loses the agent
// connection) after the typed operation reaches the transport: the dispatch
// is committed and bound, but the callback never lands. Binding and
// reconciliation delegate to the real docker executor so recovery exercises
// the production receipt path.
type callbackLossDockerExecutor struct {
	inner      dockerContainerActionExecutor
	executions int
}

func (e *callbackLossDockerExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	e.executions++
	return nil, errors.New("simulated callback loss after transport send")
}

func (e *callbackLossDockerExecutor) BindActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	return e.inner.BindActionDispatch(ctx, record, attempt)
}

func (e *callbackLossDockerExecutor) ReconcileActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	return e.inner.ReconcileActionDispatch(ctx, record, attempt)
}

func (e *callbackLossDockerExecutor) ActionHandlerNames() []string {
	return e.inner.ActionHandlerNames()
}

func (e *callbackLossDockerExecutor) ActionDispatchOperationKinds() []string {
	return e.inner.ActionDispatchOperationKinds()
}

func (e *callbackLossDockerExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	return e.inner.CheckActionAvailable(ctx, req, resource)
}

// TestRouterRecoverExecutingActionsReconcilesDurableReceipt pins the
// production recovery pass shared by the startup lifecycle worker and the
// agentexec agent-registered notifier: an action stuck in the executing state
// with a receipt-pending dispatch attempt is completed query-only from the
// agent's durable operation receipt, without a second dispatch.
func TestRouterRecoverExecutingActionsReconcilesDurableReceipt(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerActionResource("app-container:api", "docker", "running", now)
	startedAt := now.Add(-time.Minute)
	resource.Docker.StartedAt = &startedAt
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeDockerActionAgentCommander{}
	executor := &callbackLossDockerExecutor{inner: newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)}
	h.SetActionExecutor(executor)

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"req-restart-recovery",
		"resourceId":"app-container:api",
		"capabilityName":"restart",
		"reason":"exercise restart recovery"
	}`))
	h.HandlePlanAction(planRec, actionHandlerTestRequest(planReq, "operator@example.com"))
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status=%d body=%s", planRec.Code, planRec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil || plan.ActionID == "" {
		t.Fatalf("plan decode err=%v plan=%#v", err, plan)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"controlled recovery proof"}`))
	decisionReq.SetPathValue("id", plan.ActionID)
	h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, "operator@example.com"))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/execute", bytes.NewBufferString(`{"reason":"controlled callback loss"}`))
	executionReq.SetPathValue("id", plan.ActionID)
	h.HandleExecuteAction(httptest.NewRecorder(), actionHandlerTestRequest(executionReq, "operator@example.com"))

	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	attempt, found, err := store.GetActionDispatchAttempt(plan.ActionID)
	if err != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("pending attempt found=%v err=%v attempt=%#v", found, err, attempt)
	}
	if executor.executions != 1 {
		t.Fatalf("executions before recovery = %d, want 1", executor.executions)
	}

	facts := agentexec.DockerContainerLifecycleResultPayload{
		RequestID: attempt.ID, ActionID: attempt.ActionID, Operation: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, ContainerID: resource.Docker.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhaseComplete, MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		Before: agentexec.DockerContainerLifecycleSnapshot{ContainerID: resource.Docker.ContainerID, State: "running", Running: true, StartedAt: startedAt, ObservedAt: now.Add(-2 * time.Second)},
		After:  agentexec.DockerContainerLifecycleSnapshot{ContainerID: resource.Docker.ContainerID, State: "running", Running: true, StartedAt: now.Add(-time.Second), ObservedAt: now.Add(-time.Second)},
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
	agents.queryResult = operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundTerminal, Record: &operationreceipt.Record{Identity: identity, State: operationreceipt.StateTerminal, AcceptedAt: now.Add(-4 * time.Second), StartedAt: now.Add(-3 * time.Second), TerminalAt: now, ResultKind: agentexec.DockerContainerLifecycleReceiptKind, ResultVersion: agentexec.DockerContainerLifecycleReceiptVersion, Result: raw}}

	router := &Router{resourceHandlers: h}
	router.recoverExecutingActions("system:restart-recovery")

	audit, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || audit.State != unified.ActionStateCompleted {
		t.Fatalf("recovered audit found=%v err=%v audit=%#v", found, err, audit)
	}
	if audit.Result == nil || audit.Result.ActionResultV2 == nil || audit.Result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded {
		t.Fatalf("recovered action truth=%#v", audit.Result)
	}
	if executor.executions != 1 {
		t.Fatalf("executions after recovery = %d, want 1 (recovery must not redispatch)", executor.executions)
	}
	if len(agents.queries) != 1 {
		t.Fatalf("receipt queries = %d, want exactly 1", len(agents.queries))
	}
}

// A preflight refusal can make the update capability disappear before the
// server consumes the terminal receipt (for example when the live image digest
// no longer matches the planned digest). Recovery must route from the committed
// operation binding instead of mutable current inventory, or the action remains
// receipt-pending forever even though the agent has terminal truth.
func TestRouterRecoverExecutingDockerUpdateAfterCapabilityDisappears(t *testing.T) {
	now := time.Now().UTC()
	resource := dockerContainerUpdateActionResource("app-container:api", "docker", now)
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{resource},
	})
	agents := &fakeDockerActionAgentCommander{}
	callbackLoss := &callbackLossDockerExecutor{inner: newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)}
	h.SetActionExecutor(newRoutedActionExecutor(h, callbackLoss))

	planRec := httptest.NewRecorder()
	planReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", bytes.NewBufferString(`{
		"requestId":"req-update-preflight-recovery",
		"resourceId":"app-container:api",
		"capabilityName":"update",
		"reason":"apply the reviewed container update"
	}`))
	h.HandlePlanAction(planRec, actionHandlerTestRequest(planReq, "operator@example.com"))
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status=%d body=%s", planRec.Code, planRec.Body.String())
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil || plan.ActionID == "" {
		t.Fatalf("plan decode err=%v plan=%#v", err, plan)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"controlled recovery proof"}`))
	decisionReq.SetPathValue("id", plan.ActionID)
	h.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, "operator@example.com"))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+plan.ActionID+"/execute", bytes.NewBufferString(`{"reason":"exercise terminal preflight recovery"}`))
	executionReq.SetPathValue("id", plan.ActionID)
	h.HandleExecuteAction(httptest.NewRecorder(), actionHandlerTestRequest(executionReq, "operator@example.com"))

	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	attempt, found, err := store.GetActionDispatchAttempt(plan.ActionID)
	if err != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("pending attempt found=%v err=%v attempt=%#v", found, err, attempt)
	}
	if attempt.OperationKind != agentexec.DockerContainerOperationUpdate || callbackLoss.executions != 1 {
		t.Fatalf("dispatch binding=%#v executions=%d", attempt, callbackLoss.executions)
	}

	facts := agentexec.DockerContainerUpdateResultPayload{
		RequestID: attempt.ID, ActionID: attempt.ActionID,
		Operation: attempt.OperationKind, OperationVersion: attempt.OperationVersion,
		RequestDigest: attempt.RequestDigest, ContainerID: resource.Docker.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhasePreflight,
		Error:          "container image digest no longer matches the planned update",
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	identity := operationreceipt.Identity{
		AttemptID: attempt.ID, ActionID: attempt.ActionID,
		OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion,
		RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID,
	}
	agents.queryResult = operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record: &operationreceipt.Record{
			Identity: identity, State: operationreceipt.StateTerminal,
			AcceptedAt: now.Add(-4 * time.Second), StartedAt: now.Add(-3 * time.Second),
			TerminalAt: now, ResultKind: agentexec.DockerContainerUpdateReceiptKind,
			ResultVersion: agentexec.DockerContainerUpdateReceiptVersion, Result: raw,
		},
	}

	// The drift that caused preflight refusal also removes update from current
	// inventory. This is the state that previously made routed recovery fail.
	drifted := resource
	drifted.Docker.UpdateStatus = nil
	drifted.Capabilities = drifted.Capabilities[:1]
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now.Add(time.Second)},
		resources: []unified.Resource{drifted},
	})

	router := &Router{resourceHandlers: h}
	router.recoverExecutingActions("system:periodic-recovery")

	audit, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || audit.State != unified.ActionStateFailed {
		t.Fatalf("recovered audit found=%v err=%v audit=%#v", found, err, audit)
	}
	if audit.Result == nil || audit.Result.ActionResultV2 == nil {
		t.Fatalf("recovered action result=%#v", audit.Result)
	}
	truth := audit.Result.ActionResultV2
	if truth.Execution.Status != unified.ActionExecutionNotRun ||
		truth.Execution.ReasonCode != "preflight_refused" ||
		truth.Verification.Status != unified.ActionVerificationInconclusive {
		t.Fatalf("recovered preflight truth=%#v", truth)
	}
	if callbackLoss.executions != 1 {
		t.Fatalf("executions after recovery=%d, want one (query-only recovery)", callbackLoss.executions)
	}
	if len(agents.queries) != 1 {
		t.Fatalf("receipt queries=%d, want one", len(agents.queries))
	}
}
