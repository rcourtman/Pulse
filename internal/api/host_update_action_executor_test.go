package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

type fakeHostUpdateAgent struct {
	connected         bool
	result            *agentexec.HostUpdateResultPayload
	err               error
	agentID           string
	requests          []agentexec.HostUpdatePayload
	receiptVersion    int
	receiptVersionSet bool
	queryResult       operationreceipt.QueryResult
	queryErr          error
	queries           []operationreceipt.Identity
}

func (f *fakeHostUpdateAgent) QueryAgentOperation(_ context.Context, _ string, identity operationreceipt.Identity) (operationreceipt.QueryResult, error) {
	f.queries = append(f.queries, identity)
	return f.queryResult, f.queryErr
}

var testHostPackageInventoryHash = "sha256:" + strings.Repeat("a", 64)
var testHostPackageEmptyInventoryHash = "sha256:" + strings.Repeat("b", 64)

func (f *fakeHostUpdateAgent) ExecuteHostUpdate(_ context.Context, agentID string, req agentexec.HostUpdatePayload) (*agentexec.HostUpdateResultPayload, error) {
	f.agentID = agentID
	f.requests = append(f.requests, req)
	if f.result == nil {
		return nil, f.err
	}
	result := *f.result
	result.RequestID = req.RequestID
	result.ActionID = req.ActionID
	return &result, f.err
}

func (f *fakeHostUpdateAgent) IsAgentConnected(string) bool { return f.connected }
func (f *fakeHostUpdateAgent) AgentOperationReceiptVersion(string) int {
	if f.receiptVersionSet {
		return f.receiptVersion
	}
	return operationreceipt.ProtocolVersion
}

func hostUpdateDispatchTestContext(t *testing.T, actionID string) context.Context {
	t.Helper()
	attempt := hostUpdateDispatchAttempt(t, actionID, time.Now())
	return actionlifecycle.ContextWithCommittedDispatchAttempt(context.Background(), attempt)
}

func hostUpdateDispatchAttempt(t *testing.T, actionID string, now time.Time) unified.ActionDispatchAttempt {
	t.Helper()
	attempt, err := unified.NewActionDispatchAttempt(actionID, now)
	if err != nil {
		t.Fatal(err)
	}
	req := agentexec.HostUpdatePayload{RequestID: attempt.ID, ActionID: actionID, Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: testHostPackageInventoryHash}
	if err := agentexec.BindHostUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	attempt, err = unified.BindActionDispatchAttempt(attempt, unified.ActionDispatchBinding{OperationKind: req.Operation, OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, AgentID: "agent-1"})
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func TestHostUpdateActionExecutorDispatchesTypedOperationAndProjectsVerification(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{hostUpdateActionResource(now)},
	})
	agents := &fakeHostUpdateAgent{connected: true, result: &agentexec.HostUpdateResultPayload{
		RequestID: "action-host-update", Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostUpdatePhaseComplete,
		Before:        agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: now.Add(-time.Second)},
		After:         agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, PendingCount: 0, RebootRequired: true, CheckedAt: now},
		HealthChecked: true, PackageManagerHealthy: true, Verification: agentexec.HostUpdateVerificationVerified,
	}}
	executor := newHostUpdateActionExecutor(h, agents)

	result, err := executor.ExecuteAction(hostUpdateDispatchTestContext(t, "action-host-update"), hostUpdateActionRecord("action-host-update"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || !result.Verification.Ran || !result.Verification.Success {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Output, "3 pending before, 0 pending after") || !strings.Contains(result.Output, "reboot required: true") {
		t.Fatalf("output = %q", result.Output)
	}
	if len(agents.requests) != 1 || agents.agentID != "agent-1" {
		t.Fatalf("agent calls = %#v, agentID=%q", agents.requests, agents.agentID)
	}
	request := agents.requests[0]
	if request.ActionID != "action-host-update" || request.RequestID != "action-host-update.dispatch.1" || request.Operation != agentexec.HostUpdateOperationInstall || request.ExpectedInventoryHash != testHostPackageInventoryHash || request.Timeout != 900 {
		t.Fatalf("typed request = %#v", request)
	}
}

func TestHostUpdateActionExecutorReportsInconclusiveVerificationHonestly(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{hostUpdateActionResource(now)}})
	agents := &fakeHostUpdateAgent{connected: true, result: &agentexec.HostUpdateResultPayload{
		RequestID: "action-host-update", Success: true, ExecutionPhase: agentexec.HostUpdatePhaseComplete,
		Before:       agentexec.HostPackageUpdateSnapshot{PendingCount: 2},
		After:        agentexec.HostPackageUpdateSnapshot{PendingCount: 0},
		Verification: agentexec.HostUpdateVerificationInconclusive,
		Error:        "package installation completed but verification was inconclusive",
	}}

	result, err := newHostUpdateActionExecutor(h, agents).ExecuteAction(hostUpdateDispatchTestContext(t, "action-host-update"), hostUpdateActionRecord("action-host-update"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if result == nil || !result.Success || result.Verification == nil || result.Verification.Ran || result.ActionResultV2 == nil || result.ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive || result.ActionResultV2.Verification.ReasonCode != "agent_readback_inconclusive" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHostUpdateReconcileDelayedTerminalReceiptPreservesAgentAttestedEvidenceWithoutResend(t *testing.T) {
	terminalAt := time.Now().UTC().Add(-2 * time.Hour)
	receivedAt := terminalAt.Add(2 * time.Hour)
	attempt := hostUpdateDispatchAttempt(t, "action-host-update", terminalAt)
	payload := agentexec.HostUpdateResultPayload{
		RequestID: attempt.ID, ActionID: attempt.ActionID, Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostUpdatePhaseComplete,
		Before:        agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: terminalAt.Add(-2 * time.Second)},
		After:         agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, PendingCount: 0, RebootRequired: true, CheckedAt: terminalAt.Add(-time.Second)},
		HealthChecked: true, PackageManagerHealthy: true, Verification: agentexec.HostUpdateVerificationVerified,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
	agents := &fakeHostUpdateAgent{queryResult: operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundTerminal, Record: &operationreceipt.Record{
		Identity: identity, State: operationreceipt.StateTerminal, AcceptedAt: terminalAt.Add(-4 * time.Second), StartedAt: terminalAt.Add(-3 * time.Second), TerminalAt: terminalAt,
		ResultKind: agentexec.HostUpdateReceiptKind, ResultVersion: agentexec.HostAPTReceiptVersion, Result: raw,
	}}}
	executor := hostUpdateActionExecutor{agents: agents, now: func() time.Time { return receivedAt }}

	result, receipt, found, err := executor.ReconcileActionDispatch(context.Background(), hostUpdateActionRecord(attempt.ActionID), attempt)
	if err != nil || !found {
		t.Fatalf("ReconcileActionDispatch: found=%v err=%v", found, err)
	}
	if len(agents.requests) != 0 || len(agents.queries) != 1 {
		t.Fatalf("mutation requests=%d queries=%d", len(agents.requests), len(agents.queries))
	}
	if result == nil || result.ActionResultV2 == nil || result.ActionResultV2.Verification.Status != unified.ActionVerificationConfirmed || result.ActionResultV2.Verification.EvidenceClass != unified.ActionEvidenceAgentAttested {
		t.Fatalf("result=%#v", result)
	}
	evidence := result.ActionResultV2.Verification.Evidence
	if len(evidence) != 1 || !evidence[0].ObservedAt.Equal(payload.After.CheckedAt) || !evidence[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("evidence=%#v", evidence)
	}
	if !receipt.ReceivedAt.Equal(receivedAt) || receipt.TransportRequestID != attempt.ID {
		t.Fatalf("receipt=%#v", receipt)
	}
}

func TestHostUpdateReconcileLegacyTerminalReceiptDowngradesUnknownHealthWithoutResend(t *testing.T) {
	terminalAt := time.Now().UTC().Add(-2 * time.Hour)
	receivedAt := terminalAt.Add(2 * time.Hour)
	attempt := hostUpdateDispatchAttempt(t, "action-host-update", terminalAt)
	payload := agentexec.HostUpdateResultPayload{
		RequestID: attempt.ID, ActionID: attempt.ActionID, Success: true, MutationStarted: true, ExecutionPhase: agentexec.HostUpdatePhaseComplete,
		Before:       agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: terminalAt.Add(-2 * time.Second)},
		After:        agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, CheckedAt: terminalAt.Add(-time.Second)},
		Verification: agentexec.HostUpdateVerificationVerified,
	}
	raw, _ := json.Marshal(payload)
	identity := operationreceipt.Identity{AttemptID: attempt.ID, ActionID: attempt.ActionID, OperationKind: attempt.OperationKind, OperationVersion: attempt.OperationVersion, RequestDigest: attempt.RequestDigest, AgentID: attempt.AgentID}
	agents := &fakeHostUpdateAgent{queryResult: terminalAPTQuery(identity, agentexec.HostUpdateReceiptKind, raw, terminalAt)}
	executor := hostUpdateActionExecutor{agents: agents, now: func() time.Time { return receivedAt }}
	result, _, found, err := executor.ReconcileActionDispatch(context.Background(), hostUpdateActionRecord(attempt.ActionID), attempt)
	if err != nil || !found {
		t.Fatalf("ReconcileActionDispatch: found=%v err=%v", found, err)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationInconclusive || truth.EvidenceClass != unified.ActionEvidenceNone || truth.ReasonCode != "package_manager_health_unknown" || len(truth.Evidence) != 0 {
		t.Fatalf("legacy verification truth=%#v", truth)
	}
	if len(agents.requests) != 0 || len(agents.queries) != 1 {
		t.Fatalf("legacy receipt requests=%d queries=%d", len(agents.requests), len(agents.queries))
	}
}

func TestPatrolFullModeRunsHostUpdateThroughCanonicalLifecycle(t *testing.T) {
	now := time.Now().UTC()
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{hostUpdateActionResource(now)}})
	agents := &fakeHostUpdateAgent{connected: true, result: &agentexec.HostUpdateResultPayload{
		RequestID: "filled-by-executor", Success: true,
		Before:        agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: now.Add(-time.Second)},
		After:         agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: testHostPackageEmptyInventoryHash, PendingCount: 0, CheckedAt: now},
		HealthChecked: true, PackageManagerHealthy: true, Verification: agentexec.HostUpdateVerificationVerified,
	}}
	h.SetActionExecutor(newRoutedActionExecutor(h, newHostUpdateActionExecutor(h, agents)))
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "agent:host-1",
		AutoRemediationPolicy: unified.AutoRemediationPolicy{
			Enabled: true, CapabilityNames: []string{hostPackageUpdateCapability},
		},
		SetAt: now, SetBy: "operator@example.com",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "full", FullModeUnlocked: true}, nil
	})
	disposition, err := broker.Submit(context.Background(), aicontracts.ActionProposal{
		ProposalID: "proposal-host-update", FindingID: "finding-host-update", InvestigationID: "investigation-host-update",
		ResourceID: "agent:host-1", CapabilityName: hostPackageUpdateCapability, Reason: "Install three pending OS updates",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStateCompleted) || disposition.VerificationStatus != string(unified.VerificationVerified) {
		t.Fatalf("disposition = %#v", disposition)
	}
	if len(agents.requests) != 1 || agents.requests[0].Operation != agentexec.HostUpdateOperationInstall {
		t.Fatalf("typed agent requests = %#v", agents.requests)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("GetActionAudit: found=%v err=%v", found, err)
	}
	if len(audit.Approvals) != 1 || audit.Approvals[0].Actor != patrolActionPolicyActor || audit.Approvals[0].Method != unified.MethodPolicy || audit.Result == nil || audit.VerificationOutcome.Status != unified.VerificationVerified {
		t.Fatalf("audit = %#v", audit)
	}
}

func TestHostUpdateActionAvailabilityFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name       string
		mutate     func(*unified.Resource)
		connected  bool
		reasonCode string
	}{
		{name: "commands disabled", mutate: func(r *unified.Resource) { r.Agent.CommandsEnabled = false }, connected: true, reasonCode: "host_commands_disabled"},
		{name: "missing receipt protocol", mutate: func(r *unified.Resource) { r.Agent.OperationReceiptVersion = 0 }, connected: true, reasonCode: "operation_receipt_unsupported"},
		{name: "future receipt protocol", mutate: func(r *unified.Resource) { r.Agent.OperationReceiptVersion = operationreceipt.ProtocolVersion + 1 }, connected: true, reasonCode: "operation_receipt_unsupported"},
		{name: "stale inventory", mutate: func(r *unified.Resource) { r.Agent.PackageUpdates.CheckedAt = now.Add(-time.Hour) }, connected: true, reasonCode: "stale_package_inventory"},
		{name: "inventory error", mutate: func(r *unified.Resource) { r.Agent.PackageUpdates.Error = "inspection failed" }, connected: true, reasonCode: "package_inventory_error"},
		{name: "invalid inventory fingerprint", mutate: func(r *unified.Resource) { r.Agent.PackageUpdates.InventoryHash = "sha256:bad" }, connected: true, reasonCode: "invalid_package_inventory"},
		{name: "no pending", mutate: func(r *unified.Resource) { r.Agent.PackageUpdates.PendingCount = 0 }, connected: true, reasonCode: "no_pending_updates"},
		{name: "disconnected", mutate: func(*unified.Resource) {}, connected: false, reasonCode: "command_agent_disconnected"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resource := hostUpdateActionResource(now)
			tc.mutate(&resource)
			executor := hostUpdateActionExecutor{agents: &fakeHostUpdateAgent{connected: tc.connected}, now: func() time.Time { return now }}
			readiness := executor.CheckActionAvailable(context.Background(), unified.ActionRequest{CapabilityName: hostPackageUpdateCapability}, resource)
			if readiness.Available || readiness.Name != hostPackageUpdateCapability || readiness.ReasonCode != tc.reasonCode {
				t.Fatalf("readiness = %#v", readiness)
			}
		})
	}
}

func TestHostUpdateCompatibleProposalRefusesLiveAgentDowngradeBeforeExecuting(t *testing.T) {
	now := time.Now().UTC()
	resource := hostUpdateActionResource(now)
	resource.Capabilities[0].MinimumApprovalLevel = unified.ApprovalNone
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{snapshot: models.StateSnapshot{LastUpdate: now}, resources: []unified.Resource{resource}})
	agents := &fakeHostUpdateAgent{connected: true}
	h.SetActionExecutor(newRoutedActionExecutor(h, newHostUpdateActionExecutor(h, agents)))
	service := h.ActionLifecycle()
	actor := unified.ActionActor{SubjectID: "requester", Kind: unified.ActionActorService, CredentialID: "service:test", OrgID: "default"}
	plan, err := service.Plan(context.Background(), "default", unified.ActionRequest{RequestID: "downgrade", ResourceID: resource.ID, CapabilityName: hostPackageUpdateCapability, Reason: "test", RequestedBy: "requester", Actor: actor}, actor)
	if err != nil {
		t.Fatal(err)
	}
	agents.receiptVersionSet = true
	agents.receiptVersion = 0
	if _, err := service.Execute(context.Background(), "default", plan.ActionID, actor, "downgrade"); err == nil {
		t.Fatal("downgraded agent executed")
	}
	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	audit, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || audit.State == unified.ActionStateExecuting {
		t.Fatalf("audit=%#v found=%v err=%v", audit, found, err)
	}
	if _, found, err := store.GetActionDispatchAttempt(plan.ActionID); err != nil || found {
		t.Fatalf("dispatch found=%v err=%v", found, err)
	}
	if len(agents.requests) != 0 {
		t.Fatalf("requests=%#v", agents.requests)
	}
}

func TestHostUpdateSelfReportedReceiptSupportCannotBypassLiveServerVersion(t *testing.T) {
	now := time.Now().UTC()
	resource := hostUpdateActionResource(now)
	for _, version := range []int{0, operationreceipt.ProtocolVersion + 1} {
		agents := &fakeHostUpdateAgent{connected: true, receiptVersion: version, receiptVersionSet: true}
		readiness := hostUpdateActionExecutor{agents: agents, now: func() time.Time { return now }}.CheckActionAvailable(context.Background(), unified.ActionRequest{CapabilityName: hostPackageUpdateCapability}, resource)
		if readiness.Available || readiness.ReasonCode != "operation_receipt_unsupported" {
			t.Fatalf("version=%d readiness=%#v", version, readiness)
		}
	}
}

func hostUpdateActionResource(now time.Time) unified.Resource {
	return unified.Resource{
		ID: "agent:host-1", Type: unified.ResourceTypeAgent, Technology: "linux", Name: "host-1",
		Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now,
		Sources:      []unified.DataSource{unified.SourceAgent},
		SourceStatus: map[unified.DataSource]unified.SourceStatus{unified.SourceAgent: {Status: "online", LastSeen: now}},
		Agent: &unified.AgentData{
			AgentID: "agent-1", Platform: "linux", CommandsEnabled: true, OperationReceiptVersion: operationreceipt.ProtocolVersion,
			PackageUpdates: &unified.AgentPackageUpdateMeta{Supported: true, Manager: "apt", InventoryHash: testHostPackageInventoryHash, PendingCount: 3, CheckedAt: now, ObservedAt: now},
		},
		Capabilities: []unified.ResourceCapability{{
			Name: hostPackageUpdateCapability, Type: unified.CapabilityTypeCommon,
			Description: "Install standard OS updates", MinimumApprovalLevel: unified.ApprovalAdmin,
			AutoAuthorization: unified.AutoAuthorizeElevated, Platform: "linux", InternalHandler: hostPackageUpdateActionHandler,
		}},
	}
}

func hostUpdateActionRecord(actionID string) unified.ActionAuditRecord {
	now := time.Now().UTC()
	return unified.ActionAuditRecord{
		ID: actionID, CreatedAt: now, UpdatedAt: now, State: unified.ActionStateExecuting,
		Request: unified.ActionRequest{
			RequestID: "request-" + actionID, ResourceID: "agent:host-1", CapabilityName: hostPackageUpdateCapability,
			Reason: "Install pending host updates", RequestedBy: "pulse_patrol", Params: map[string]any{},
		},
		Plan: unified.ActionPlan{
			ActionID: actionID, RequestID: "request-" + actionID, Allowed: true, RequiresApproval: true,
			ApprovalPolicy: unified.ApprovalAdmin, PlannedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
			ResourceVersion: "resource:sha256:test", PolicyVersion: "policy:sha256:test", PlanHash: "sha256:test",
		},
	}
}
