package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/mock"
)

const (
	testAuditVerificationCommandRedacted = "[redacted-verification-command]"
	testAuditVerificationOutputRedacted  = "[redacted-verification-output]"
	testAuditVerificationNoteRedacted    = "[redacted-verification-note]"
)

func TestExecuteCommandWithAuditUsesExecutionStateMachine(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "uptime" && payload.TargetType == "agent" && payload.TargetID == "agent-1"
	})).Return(&agentexec.CommandResultPayload{
		Success:  true,
		Stdout:   "ok",
		ExitCode: 0,
	}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: store,
	})
	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"node-1",
		"",
		false,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "uptime",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"run uptime",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}
	if result == nil || result.Stdout != "ok" {
		t.Fatalf("command result = %#v", result)
	}

	audits, err := store.GetActionAudits("node-1", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("audits len = %d, want 1", len(audits))
	}
	audit := audits[0]
	if audit.State != unifiedresources.ActionStateCompleted || audit.Result == nil || audit.Result.Output != "ok" {
		t.Fatalf("audit = %#v", audit)
	}

	events, err := store.GetActionLifecycleEvents(audit.ID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unifiedresources.ActionState]string{}
	for _, event := range events {
		seen[event.State] = event.Message
	}
	for _, state := range []unifiedresources.ActionState{
		unifiedresources.ActionStatePlanned,
		unifiedresources.ActionStateExecuting,
		unifiedresources.ActionStateCompleted,
	} {
		if _, ok := seen[state]; !ok {
			t.Fatalf("missing lifecycle state %q in %#v", state, events)
		}
	}
	if seen[unifiedresources.ActionStateExecuting] != "dispatching command to agent agent-1" {
		t.Fatalf("executing event message = %q", seen[unifiedresources.ActionStateExecuting])
	}
	agentServer.AssertExpectations(t)
}

func TestAttachApprovalActionPlanRecordsPendingActionAudit(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	now := time.Date(2026, 5, 7, 9, 30, 0, 0, time.UTC)
	req := &approval.ApprovalRequest{
		ID:         " approval-patrol-queued ",
		OrgID:      "tenant-1",
		ToolID:     "investigation_fix",
		Command:    "systemctl restart nginx",
		TargetType: "investigation",
		TargetID:   "finding-123",
		TargetName: "web-1",
		Context:    "Restart nginx after Patrol investigation",
		RiskLevel:  approval.RiskHigh,
	}

	AttachApprovalActionPlan(req, now)

	if req.ID != "approval-patrol-queued" {
		t.Fatalf("approval id = %q", req.ID)
	}
	if req.Plan == nil {
		t.Fatal("expected governed action plan")
	}
	if req.Plan.ActionID == "" {
		t.Fatalf("expected action id: %#v", req.Plan)
	}
	if req.Plan.RequestID != req.ID {
		t.Fatalf("plan request id = %q, want %q", req.Plan.RequestID, req.ID)
	}
	if !req.Plan.Allowed || !req.Plan.RequiresApproval || req.Plan.ApprovalPolicy != unifiedresources.ApprovalAdmin {
		t.Fatalf("approval plan posture = %#v", req.Plan)
	}
	if !req.Plan.PlannedAt.Equal(now) {
		t.Fatalf("planned at = %s, want %s", req.Plan.PlannedAt, now)
	}
	if req.Preflight == nil || req.ContextConfidence == nil || req.Plan.Preflight != req.Preflight {
		t.Fatalf("expected preflight and context confidence on approval: %#v", req)
	}

	RecordPendingApprovalAction(store, req)

	audit, ok, err := store.GetActionAudit(req.Plan.ActionID)
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok {
		t.Fatal("expected pending approval action audit")
	}
	if audit.State != unifiedresources.ActionStatePending {
		t.Fatalf("audit state = %q, want pending", audit.State)
	}
	if audit.Request.RequestID != req.ID {
		t.Fatalf("audit request id = %q, want %q", audit.Request.RequestID, req.ID)
	}
	if audit.Request.ResourceID != "investigation:finding-123" {
		t.Fatalf("audit resource id = %q", audit.Request.ResourceID)
	}
	if audit.Request.RequestedBy != approval.RequesterPulsePatrol {
		t.Fatalf("audit requested by = %q", audit.Request.RequestedBy)
	}

	events, err := store.GetActionLifecycleEvents(req.Plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	states := map[unifiedresources.ActionState]bool{}
	for _, event := range events {
		states[event.State] = true
		if event.Actor != approval.RequesterPulsePatrol {
			t.Fatalf("lifecycle actor = %q, want %q", event.Actor, approval.RequesterPulsePatrol)
		}
	}
	if !states[unifiedresources.ActionStatePlanned] || !states[unifiedresources.ActionStatePending] {
		t.Fatalf("expected planned and pending lifecycle states, got %#v", events)
	}
}

func TestExecuteNativeActionWithApprovedAuditRecordsDecisionBeforeExecution(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-approved-native",
		RequestID:        "approval-native",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		ResourceVersion:  "resource:sha256:test",
		PolicyVersion:    "policy:sha256:test",
		PlanHash:         "sha256:test",
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-native",
		Command:    "restart app",
		TargetType: "docker",
		TargetID:   "docker-host:app",
		TargetName: "app",
		Context:    "restart app during maintenance",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	executor := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: actionStore})
	executor.recordPendingApprovalAction(req)
	if _, err := approvalStore.Approve("approval-native", "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	result, err := executor.executeNativeActionWithAudit(
		context.Background(),
		"pulse_docker",
		"docker-host:app",
		"approval-native",
		true,
		map[string]any{"action": "restart"},
		"pulse_control",
		"restart app during maintenance",
		func(context.Context) (*unifiedresources.ExecutionResult, error) {
			return &unifiedresources.ExecutionResult{Success: false, ErrorMessage: "provider rejected restart"}, nil
		},
	)
	if err != nil {
		t.Fatalf("executeNativeActionWithAudit: %v", err)
	}
	if result == nil || result.Success || result.ErrorMessage != "provider rejected restart" {
		t.Fatalf("execution result = %#v", result)
	}

	audit, ok, err := actionStore.GetActionAudit("act-approved-native")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unifiedresources.ActionStateFailed || len(audit.Approvals) != 1 || audit.Result == nil {
		t.Fatalf("audit = %#v, ok=%v", audit, ok)
	}
	if audit.Approvals[0].Actor != "operator@example.com" || audit.Approvals[0].Outcome != unifiedresources.OutcomeApproved {
		t.Fatalf("approval audit = %#v", audit.Approvals[0])
	}

	events, err := actionStore.GetActionLifecycleEvents("act-approved-native", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unifiedresources.ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
	}
	for _, state := range []unifiedresources.ActionState{
		unifiedresources.ActionStatePlanned,
		unifiedresources.ActionStatePending,
		unifiedresources.ActionStateApproved,
		unifiedresources.ActionStateExecuting,
		unifiedresources.ActionStateFailed,
	} {
		if !seen[state] {
			t.Fatalf("missing lifecycle state %q in %#v", state, events)
		}
	}
}

func TestRecordApprovalDecisionUsesActionDecisionStoreContract(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-decision-contract",
		RequestID:        "approval-decision",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		ResourceVersion:  "resource:sha256:test",
		PolicyVersion:    "policy:sha256:test",
		PlanHash:         "sha256:test",
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-decision",
		Command:    "restart vm",
		TargetType: "vm",
		TargetID:   "vm:42",
		TargetName: "web-42",
		Context:    "restart vm during maintenance",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	executor := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: actionStore})
	executor.recordPendingApprovalAction(req)
	if _, err := approvalStore.Approve("approval-decision", "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	executor.RecordApprovalDecision("approval-decision", unifiedresources.ActionStateApproved, "operator@example.com", "approval granted")

	audit, ok, err := actionStore.GetActionAudit("act-decision-contract")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unifiedresources.ActionStateApproved || len(audit.Approvals) != 1 || audit.Result != nil {
		t.Fatalf("audit = %#v, ok=%v", audit, ok)
	}
	events, err := actionStore.GetActionLifecycleEvents("act-decision-contract", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unifiedresources.ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
		if event.State == unifiedresources.ActionStateExecuting || event.State == unifiedresources.ActionStateCompleted {
			t.Fatalf("approval decision must not create execution lifecycle event: %#v", event)
		}
	}
	for _, state := range []unifiedresources.ActionState{
		unifiedresources.ActionStatePlanned,
		unifiedresources.ActionStatePending,
		unifiedresources.ActionStateApproved,
	} {
		if !seen[state] {
			t.Fatalf("missing lifecycle state %q in %#v", state, events)
		}
	}
}

func TestRecordApprovalDecisionUsesRejectedActionDecisionStoreContract(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-rejected-decision-contract",
		RequestID:        "approval-rejected-decision",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		ResourceVersion:  "resource:sha256:test",
		PolicyVersion:    "policy:sha256:test",
		PlanHash:         "sha256:test",
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-rejected-decision",
		Command:    "delete pod",
		TargetType: "kubernetes",
		TargetID:   "pod:danger",
		TargetName: "danger",
		Context:    "delete pod outside maintenance",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	executor := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: actionStore})
	executor.recordPendingApprovalAction(req)
	if _, err := approvalStore.Deny("approval-rejected-decision", "operator@example.com", "outside maintenance"); err != nil {
		t.Fatalf("Deny: %v", err)
	}

	RecordApprovalDecision(actionStore, "approval-rejected-decision", unifiedresources.ActionStateRejected, "operator@example.com", "outside maintenance")

	audit, ok, err := actionStore.GetActionAudit("act-rejected-decision-contract")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unifiedresources.ActionStateRejected || len(audit.Approvals) != 1 || audit.Result != nil {
		t.Fatalf("audit = %#v, ok=%v", audit, ok)
	}
	if audit.Approvals[0].Outcome != unifiedresources.OutcomeRejected {
		t.Fatalf("approval outcome = %q, want %q", audit.Approvals[0].Outcome, unifiedresources.OutcomeRejected)
	}
	events, err := actionStore.GetActionLifecycleEvents("act-rejected-decision-contract", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	seen := map[unifiedresources.ActionState]bool{}
	for _, event := range events {
		seen[event.State] = true
		if event.State == unifiedresources.ActionStateExecuting || event.State == unifiedresources.ActionStateCompleted {
			t.Fatalf("rejected approval decision must not create execution lifecycle event: %#v", event)
		}
	}
	for _, state := range []unifiedresources.ActionState{
		unifiedresources.ActionStatePlanned,
		unifiedresources.ActionStatePending,
		unifiedresources.ActionStateRejected,
	} {
		if !seen[state] {
			t.Fatalf("missing lifecycle state %q in %#v", state, events)
		}
	}
}

func TestRecordApprovalDecisionDoesNotRegressExecutingAudit(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-decision-no-regress",
		RequestID:        "approval-decision-no-regress",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		ResourceVersion:  "resource:sha256:test",
		PolicyVersion:    "policy:sha256:test",
		PlanHash:         "sha256:test",
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-decision-no-regress",
		Command:    "restart service",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "agent-1",
		Context:    "restart service during maintenance",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if _, err := approvalStore.Approve("approval-decision-no-regress", "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	executor := NewPulseToolExecutor(ExecutorConfig{ActionAuditStore: actionStore})
	record := actionAuditRecordFromApproval(req, unifiedresources.ActionStateExecuting, "pulse_control")
	record.Approvals = approvalRecordsForID(req.ID, req.Plan)
	if err := actionStore.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	executor.RecordApprovalDecision("approval-decision-no-regress", unifiedresources.ActionStateApproved, "operator@example.com", "approval granted")

	audit, ok, err := actionStore.GetActionAudit("act-decision-no-regress")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unifiedresources.ActionStateExecuting || audit.Result != nil {
		t.Fatalf("audit = %#v, ok=%v", audit, ok)
	}
}

// TestExecuteCommandWithAuditRefusesPayloadDriftAgainstApprovedPlan covers the
// safety guarantee that the operator approved exactly one (command, target,
// reason) combination. If the payload presented at execute time hashes to
// anything different than the approved plan, the broker must refuse rather
// than dispatch a drifted command under a stale approval.
func TestExecuteCommandWithAuditRefusesPayloadDriftAgainstApprovedPlan(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	approvedHash := approvalPlanHash(
		"act-drift",
		"approval-drift",
		"pulse_control",
		"agent-1",
		"systemctl restart workload",
		"agent",
		"agent-1",
		"restart workload service",
	)
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-drift",
		RequestID:        "approval-drift",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		PlanHash:         approvedHash,
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-drift",
		Command:    "systemctl restart workload",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "agent-1",
		Context:    "restart workload service",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if _, err := approvalStore.Approve("approval-drift", "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	agentServer := &mockAgentServer{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	executor.recordPendingApprovalAction(req)

	// Payload deliberately differs from the approved command. The hash check
	// must catch this and refuse.
	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-1",
		"approval-drift",
		true,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "rm -rf /var/log/pulse",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"restart workload service",
	)
	if !errors.Is(err, unifiedresources.ErrActionPlanDrift) {
		t.Fatalf("executeCommandWithAudit error = %v, want ErrActionPlanDrift", err)
	}
	if result != nil {
		t.Fatalf("expected nil result on drift refusal, got %#v", result)
	}
	agentServer.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)

	// Drift refusal must be observable in the audit history, not just in
	// WARN logs. Operators reviewing the action audit trail need to see
	// "Pulse caught this drift attempt" recorded as a Failed action with
	// a plan_drift error message.
	audits, err := actionStore.GetActionAudits("agent:agent-1", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected 1 drift-refused audit record, got %d", len(audits))
	}
	driftAudit := audits[0]
	if driftAudit.State != unifiedresources.ActionStateFailed {
		t.Fatalf("drift audit state = %q, want %q", driftAudit.State, unifiedresources.ActionStateFailed)
	}
	if driftAudit.Result == nil || driftAudit.Result.Success {
		t.Fatalf("expected drift audit Result.Success=false, got %#v", driftAudit.Result)
	}
	if driftAudit.Result == nil || !strings.Contains(driftAudit.Result.ErrorMessage, "plan_drift") {
		t.Fatalf("expected drift audit ErrorMessage to include plan_drift, got %q", driftAudit.Result.ErrorMessage)
	}
}

// TestExecuteCommandWithAuditRunsClassDerivedVerificationAfterDispatch
// covers the read-after-write loop: when a successful dispatch matches a
// known command class, the broker derives the verification command and
// runs it via the same agent path. The result is persisted on the audit
// record's ExecutionResult.Verification so the audit history shows not
// only what the action did but whether the read-back confirmed it.
func TestExecuteCommandWithAuditRunsClassDerivedVerificationAfterDispatch(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()

	agentServer := &mockAgentServer{}
	// Dispatch: systemctl restart nginx — succeeds.
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "systemctl restart nginx"
	})).Return(&agentexec.CommandResultPayload{
		Success:  true,
		Stdout:   "",
		ExitCode: 0,
	}, nil).Once()
	// Verification: systemctl is-active nginx — succeeds with "active".
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "systemctl is-active 'nginx'"
	})).Return(&agentexec.CommandResultPayload{
		Success:  true,
		Stdout:   "active",
		ExitCode: 0,
	}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	_, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"node-1",
		"",
		false,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart nginx",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"restart nginx after Patrol detected stale config",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}

	audits, err := actionStore.GetActionAudits("node-1", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("audits len = %d, want 1", len(audits))
	}
	verification := audits[0].Result.Verification
	if verification == nil {
		t.Fatalf("expected ExecutionResult.Verification to be populated, got nil")
	}
	if !verification.Ran {
		t.Fatalf("expected Verification.Ran=true")
	}
	if verification.Command != testAuditVerificationCommandRedacted {
		t.Fatalf("Verification.Command = %q, want stored audit redaction marker", verification.Command)
	}
	if !verification.Success {
		t.Fatalf("expected Verification.Success=true after exit code 0")
	}
	if verification.Output != testAuditVerificationOutputRedacted {
		t.Fatalf("Verification.Output = %q, want stored audit redaction marker", verification.Output)
	}
	agentServer.AssertExpectations(t)
}

// TestExecuteCommandWithAuditMarksVerificationFailedWhenReadbackDoesNotConfirm
// covers the negative case: dispatch succeeded, but the read-after-write
// check returned a non-zero exit code (e.g. service is "failed" not
// "active"). Verification.Success must reflect the read-back outcome
// rather than the dispatch outcome so the audit row honestly shows that
// Pulse ran the action but couldn't confirm it took.
func TestExecuteCommandWithAuditMarksVerificationFailedWhenReadbackDoesNotConfirm(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()

	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "systemctl restart workload"
	})).Return(&agentexec.CommandResultPayload{
		Success:  true,
		ExitCode: 0,
	}, nil).Once()
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "systemctl is-active 'workload'"
	})).Return(&agentexec.CommandResultPayload{
		Success:  false,
		Stdout:   "failed",
		ExitCode: 3,
	}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	_, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"node-1",
		"",
		false,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart workload",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"restart workload after backup window",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}

	audits, _ := actionStore.GetActionAudits("node-1", time.Time{}, 10)
	verification := audits[0].Result.Verification
	if verification == nil || !verification.Ran {
		t.Fatalf("expected verification ran, got %#v", verification)
	}
	if verification.Success {
		t.Fatalf("expected Verification.Success=false after exit code 3")
	}
	if verification.Output != testAuditVerificationOutputRedacted {
		t.Fatalf("Verification.Output = %q, want stored audit redaction marker", verification.Output)
	}
	if verification.Note != testAuditVerificationNoteRedacted {
		t.Fatalf("Verification.Note = %q, want stored audit redaction marker", verification.Note)
	}
}

// TestExecuteCommandWithAuditSkipsVerificationForUnclassifiedCommands
// covers the no-fabrication boundary: when no verification command is
// derivable for the action class, the broker leaves Verification nil
// rather than recording a fake "verified" entry.
func TestExecuteCommandWithAuditSkipsVerificationForUnclassifiedCommands(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()

	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "echo hello"
	})).Return(&agentexec.CommandResultPayload{
		Success:  true,
		Stdout:   "hello",
		ExitCode: 0,
	}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	_, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"node-1",
		"",
		false,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "echo hello",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"echo greeting",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}

	audits, _ := actionStore.GetActionAudits("node-1", time.Time{}, 10)
	if audits[0].Result.Verification != nil {
		t.Fatalf("expected nil Verification for unclassified command, got %#v", audits[0].Result.Verification)
	}
	// Exactly one ExecuteCommand call expected (no verification dispatch).
	agentServer.AssertExpectations(t)
}

// TestExecuteCommandWithAuditAllowsMatchingPlanHash covers the positive case:
// when the payload at execute time hashes identically to the approved plan,
// dispatch proceeds normally.
func TestExecuteCommandWithAuditAllowsMatchingPlanHash(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	approvedHash := approvalPlanHash(
		"act-match",
		"approval-match",
		"pulse_control",
		"agent-1",
		"uptime",
		"agent",
		"agent-1",
		"check uptime",
	)
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-match",
		RequestID:        "approval-match",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		PlanHash:         approvedHash,
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-match",
		Command:    "uptime",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "agent-1",
		Context:    "check uptime",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if _, err := approvalStore.Approve("approval-match", "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-1", mock.Anything).Return(&agentexec.CommandResultPayload{
		Success:  true,
		Stdout:   "up 4 days",
		ExitCode: 0,
	}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	executor.recordPendingApprovalAction(req)

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-1",
		"approval-match",
		true,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "uptime",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"check uptime",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}
	if result == nil || result.Stdout != "up 4 days" {
		t.Fatalf("command result = %#v", result)
	}
	agentServer.AssertExpectations(t)
}

func TestExecuteCommandWithDeniedApprovalDoesNotDispatch(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	// PlanHash is the approval-equivalent hash of the (command, target,
	// reason) the operator approved. This test covers the denial path, so
	// the hash must match the executing payload to isolate denial as the
	// reason for refusal — otherwise the broker's drift check fires first.
	deniedHash := approvalPlanHash(
		"act-denied-command",
		"approval-denied-command",
		"pulse_control",
		"agent-1",
		"rm -rf /tmp/pulse-test",
		"agent",
		"agent-1",
		"unsafe cleanup command",
	)
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-denied-command",
		RequestID:        "approval-denied-command",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		ResourceVersion:  "resource:sha256:test",
		PolicyVersion:    "policy:sha256:test",
		PlanHash:         deniedHash,
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-denied-command",
		Command:    "rm -rf /tmp/pulse-test",
		TargetType: "agent",
		TargetID:   "agent-1",
		TargetName: "agent-1",
		Context:    "unsafe cleanup command",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if _, err := approvalStore.Deny("approval-denied-command", "operator@example.com", "unsafe command"); err != nil {
		t.Fatalf("Deny: %v", err)
	}

	agentServer := &mockAgentServer{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	executor.recordPendingApprovalAction(req)

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-1",
		"approval-denied-command",
		true,
		"agent-1",
		agentexec.ExecuteCommandPayload{
			Command:    "rm -rf /tmp/pulse-test",
			TargetType: "agent",
			TargetID:   "agent-1",
		},
		"pulse_control",
		"unsafe cleanup command",
	)
	if !errors.Is(err, unifiedresources.ErrActionNotApproved) {
		t.Fatalf("executeCommandWithAudit error = %v, want %v", err, unifiedresources.ErrActionNotApproved)
	}
	if result != nil {
		t.Fatalf("command result = %#v, want nil", result)
	}
	agentServer.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)

	audit, ok, err := actionStore.GetActionAudit("act-denied-command")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || audit.State != unifiedresources.ActionStateRejected || len(audit.Approvals) != 1 || audit.Approvals[0].Outcome != unifiedresources.OutcomeRejected {
		t.Fatalf("audit = %#v, ok=%v", audit, ok)
	}
	events, err := actionStore.GetActionLifecycleEvents("act-denied-command", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	for _, event := range events {
		if event.State == unifiedresources.ActionStateExecuting || event.State == unifiedresources.ActionStateCompleted {
			t.Fatalf("denied approval must not create execution lifecycle event: %#v", event)
		}
	}
}

func TestExecuteCommandWithAuditRefusesApprovedDryRunOnlyAndExpiredPlans(t *testing.T) {
	for _, tc := range []struct {
		name       string
		suffix     string
		configure  func(*approval.ApprovalRequest)
		wantErr    error
		wantPrefix string
	}{
		{
			name:   "dry run only",
			suffix: "dryrun",
			configure: func(req *approval.ApprovalRequest) {
				req.Plan.ApprovalPolicy = unifiedresources.ApprovalDryRun
			},
			wantErr:    unifiedresources.ErrActionDryRunOnly,
			wantPrefix: "action_dry_run_only:",
		},
		{
			name:   "expired",
			suffix: "expired",
			configure: func(req *approval.ApprovalRequest) {
				req.Plan.ExpiresAt = time.Now().UTC().Add(-time.Minute)
			},
			wantErr:    unifiedresources.ErrActionPlanExpired,
			wantPrefix: "action_plan_expired:",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actionStore := unifiedresources.NewMemoryStore()
			approvalStore, err := approval.NewStore(approval.StoreConfig{
				DataDir:            t.TempDir(),
				DisablePersistence: true,
			})
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}
			previousApprovalStore := approval.GetStore()
			approval.SetStore(approvalStore)
			t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

			now := time.Now().UTC()
			actionID := "act-" + tc.suffix + "-command"
			approvalID := "approval-" + tc.suffix + "-command"
			targetID := "agent-" + tc.suffix
			command := "systemctl restart workload"
			reason := "restart workload service"
			approvedHash := approvalPlanHash(
				actionID,
				approvalID,
				"pulse_control",
				targetID,
				command,
				"agent",
				targetID,
				reason,
			)
			plan := unifiedresources.ActionPlan{
				ActionID:         actionID,
				RequestID:        approvalID,
				Allowed:          true,
				RequiresApproval: true,
				ApprovalPolicy:   unifiedresources.ApprovalAdmin,
				PlannedAt:        now,
				ExpiresAt:        now.Add(5 * time.Minute),
				ResourceVersion:  "resource:sha256:test",
				PolicyVersion:    "policy:sha256:test",
				PlanHash:         approvedHash,
			}
			req := &approval.ApprovalRequest{
				ID:         approvalID,
				Command:    command,
				TargetType: "agent",
				TargetID:   targetID,
				TargetName: targetID,
				Context:    reason,
				Plan:       &plan,
			}
			if err := approvalStore.CreateApproval(req); err != nil {
				t.Fatalf("CreateApproval: %v", err)
			}
			approved, err := approvalStore.Approve(approvalID, "operator@example.com")
			if err != nil {
				t.Fatalf("Approve: %v", err)
			}
			tc.configure(approved)

			agentServer := &mockAgentServer{}
			executor := NewPulseToolExecutor(ExecutorConfig{
				AgentServer:      agentServer,
				ActionAuditStore: actionStore,
			})
			executor.recordPendingApprovalAction(approved)

			completed := make(chan unifiedresources.ActionAuditRecord, 1)
			executor.SetOnActionCompleted(func(record unifiedresources.ActionAuditRecord) {
				completed <- record
			})

			result, err := executor.executeCommandWithAudit(
				context.Background(),
				"pulse_control",
				targetID,
				approvalID,
				true,
				targetID,
				agentexec.ExecuteCommandPayload{
					Command:    command,
					TargetType: "agent",
					TargetID:   targetID,
				},
				"pulse_control",
				reason,
			)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("executeCommandWithAudit error = %v, want %v", err, tc.wantErr)
			}
			if result != nil {
				t.Fatalf("expected nil command result on refusal, got %#v", result)
			}
			agentServer.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)

			audit, ok, err := actionStore.GetActionAudit(actionID)
			if err != nil {
				t.Fatalf("GetActionAudit: %v", err)
			}
			if !ok {
				t.Fatalf("expected refused action audit %q", actionID)
			}
			if audit.State != unifiedresources.ActionStateFailed {
				t.Fatalf("audit state = %q, want %q", audit.State, unifiedresources.ActionStateFailed)
			}
			if audit.Result == nil || audit.Result.Success {
				t.Fatalf("expected Result.Success=false, got %#v", audit.Result)
			}
			if !strings.HasPrefix(audit.Result.ErrorMessage, tc.wantPrefix) {
				t.Fatalf("ErrorMessage = %q, want prefix %q", audit.Result.ErrorMessage, tc.wantPrefix)
			}
			if errors.Is(tc.wantErr, unifiedresources.ErrActionDryRunOnly) {
				if len(audit.Approvals) != 0 {
					t.Fatalf("dry-run-only refusal minted executable approval authority: %#v", audit.Approvals)
				}
			} else if len(audit.Approvals) != 1 || audit.Approvals[0].Outcome != unifiedresources.OutcomeApproved {
				t.Fatalf("expected approved audit record to be preserved, got %#v", audit.Approvals)
			}

			events, err := actionStore.GetActionLifecycleEvents(actionID, time.Time{}, 10)
			if err != nil {
				t.Fatalf("GetActionLifecycleEvents: %v", err)
			}
			seenFailed := false
			for _, event := range events {
				if event.State == unifiedresources.ActionStateExecuting || event.State == unifiedresources.ActionStateCompleted {
					t.Fatalf("refusal must not create dispatch lifecycle event: %#v", event)
				}
				if event.State == unifiedresources.ActionStateFailed {
					seenFailed = true
					if !strings.HasPrefix(event.Message, tc.wantPrefix) {
						t.Fatalf("failed lifecycle message = %q, want prefix %q", event.Message, tc.wantPrefix)
					}
				}
			}
			if !seenFailed {
				t.Fatalf("missing failed refusal lifecycle event in %#v", events)
			}

			select {
			case received := <-completed:
				if received.State != unifiedresources.ActionStateFailed {
					t.Fatalf("callback state = %q, want %q", received.State, unifiedresources.ActionStateFailed)
				}
				if received.Result == nil || !strings.HasPrefix(received.Result.ErrorMessage, tc.wantPrefix) {
					t.Fatalf("callback result = %#v, want prefix %q", received.Result, tc.wantPrefix)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("terminal refusal callback did not fire within 2s")
			}
		})
	}
}

// TestExecuteCommandWithAuditRefusesWhenResourceIsRemediationLocked covers
// the operator-set NeverAutoRemediate safety. When the operator has flagged
// the target resource as never-auto-remediate (via the
// /api/resources/{id}/operator-state surface), the broker must refuse the
// dispatch even with a valid approval and matching plan hash. The
// per-resource intent outranks the per-action approval — this is the
// safety mechanism for "do not touch this resource even if you think you
// should." The refusal must be visible on the audit timeline as a Failed
// record with a `resource_remediation_locked:` ErrorMessage prefix.
func TestExecuteCommandWithAuditRefusesWhenResourceIsRemediationLocked(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()

	// Operator has marked the target resource as never-auto-remediate.
	if err := actionStore.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:        "agent-locked",
		NeverAutoRemediate: true,
		Note:               "manual-only — Pulse must not touch this host",
		SetAt:              time.Now().UTC(),
		SetBy:              "operator:richard",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	previousApprovalStore := approval.GetStore()
	approval.SetStore(approvalStore)
	t.Cleanup(func() { approval.SetStore(previousApprovalStore) })

	now := time.Now().UTC()
	approvedHash := approvalPlanHash(
		"act-locked",
		"approval-locked",
		"pulse_control",
		"agent-locked",
		"systemctl restart workload",
		"agent",
		"agent-locked",
		"restart workload service",
	)
	plan := unifiedresources.ActionPlan{
		ActionID:         "act-locked",
		RequestID:        "approval-locked",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		PlanHash:         approvedHash,
	}
	req := &approval.ApprovalRequest{
		ID:         "approval-locked",
		Command:    "systemctl restart workload",
		TargetType: "agent",
		TargetID:   "agent-locked",
		TargetName: "agent-locked",
		Context:    "restart workload service",
		Plan:       &plan,
	}
	if err := approvalStore.CreateApproval(req); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	if _, err := approvalStore.Approve("approval-locked", "operator@example.com"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	agentServer := &mockAgentServer{}
	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})
	executor.recordPendingApprovalAction(req)

	// Payload matches the approval (no drift); only the operator-set lock
	// should refuse this. If the broker dispatches anyway, the operator's
	// per-resource intent has been overridden by the approval, which is
	// the failure we're guarding against.
	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-locked",
		"approval-locked",
		true,
		"agent-locked",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart workload",
			TargetType: "agent",
			TargetID:   "agent-locked",
		},
		"pulse_control",
		"restart workload service",
	)
	if !errors.Is(err, unifiedresources.ErrResourceRemediationLocked) {
		t.Fatalf("executeCommandWithAudit error = %v, want ErrResourceRemediationLocked", err)
	}
	if result != nil {
		t.Fatalf("expected nil result on remediation-locked refusal, got %#v", result)
	}
	agentServer.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)

	// Refusal must be observable in the audit history with the canonical
	// `resource_remediation_locked:` ErrorMessage prefix so audit-UI
	// filters and alert rules can branch on the stable token.
	audits, err := actionStore.GetActionAudits("agent:agent-locked", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected 1 refused audit record, got %d", len(audits))
	}
	refused := audits[0]
	if refused.State != unifiedresources.ActionStateFailed {
		t.Fatalf("audit state = %q, want %q", refused.State, unifiedresources.ActionStateFailed)
	}
	if refused.Result == nil || refused.Result.Success {
		t.Fatalf("expected Result.Success=false, got %#v", refused.Result)
	}
	if !strings.HasPrefix(refused.Result.ErrorMessage, "resource_remediation_locked:") {
		t.Fatalf("expected ErrorMessage to start with resource_remediation_locked:, got %q", refused.Result.ErrorMessage)
	}
}

// TestExecuteCommandWithAuditAllowsDispatchWhenResourceUnlocked covers
// the negative case: a resource that has operator state recorded but
// without NeverAutoRemediate set must NOT block the dispatch. Pin this
// so the refusal branch can't drift into "always refuse when state
// exists".
func TestExecuteCommandWithAuditAllowsDispatchWhenResourceUnlocked(t *testing.T) {
	actionStore := unifiedresources.NewMemoryStore()
	// State recorded but with NeverAutoRemediate=false — should not refuse.
	if err := actionStore.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
		CanonicalID:          "agent-allowed",
		IntentionallyOffline: true,
		NeverAutoRemediate:   false,
		SetAt:                time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-allowed", mock.Anything).
		Return(&agentexec.CommandResultPayload{Stdout: "OK", ExitCode: 0}, nil)

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: actionStore,
	})

	result, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-allowed",
		"",    // no approval (not testing approval flow here)
		false, // requiresApproval=false
		"agent-allowed",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl status workload",
			TargetType: "agent",
			TargetID:   "agent-allowed",
		},
		"pulse_control",
		"check status",
	)
	if err != nil {
		t.Fatalf("executeCommandWithAudit unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected dispatch to proceed; got nil result")
	}
	agentServer.AssertCalled(t, "ExecuteCommand", mock.Anything, "agent-allowed", mock.Anything)
}

func TestSetOnActionCompleted_FiresOnSuccessfulDispatch(t *testing.T) {
	// The post-completion callback is the seam the API layer uses to
	// bridge action audits into the agent SSE stream
	// (action.completed events). Pin the contract: the callback
	// fires once per terminal-state record, sees the persisted
	// audit, and runs without blocking the dispatch caller —
	// fire-and-forget on its own goroutine.
	store := unifiedresources.NewMemoryStore()
	agentServer := &mockAgentServer{}
	agentServer.On("ExecuteCommand", mock.Anything, "agent-cb", mock.Anything).
		Return(&agentexec.CommandResultPayload{Success: true, Stdout: "ok", ExitCode: 0}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: store,
	})

	got := make(chan unifiedresources.ActionAuditRecord, 1)
	executor.SetOnActionCompleted(func(rec unifiedresources.ActionAuditRecord) {
		got <- rec
	})

	if _, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-cb",
		"",
		false,
		"agent-cb",
		agentexec.ExecuteCommandPayload{
			Command:    "uptime",
			TargetType: "agent",
			TargetID:   "agent-cb",
		},
		"pulse_control",
		"run uptime",
	); err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}

	select {
	case received := <-got:
		if received.State != unifiedresources.ActionStateCompleted {
			t.Errorf("callback record state = %q, want %q", received.State, unifiedresources.ActionStateCompleted)
		}
		if received.ID == "" {
			t.Error("callback record must carry the canonical action id")
		}
		if received.Request.ResourceID != "agent-cb" {
			t.Errorf("callback resource id = %q, want %q", received.Request.ResourceID, "agent-cb")
		}
		if received.Result == nil || !received.Result.Success {
			t.Errorf("callback result must surface success: %#v", received.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("post-completion callback did not fire within 2s")
	}
}

func TestSetOnActionCompleted_RecordCarriesVerificationResult(t *testing.T) {
	// Verification is the post-execution read-after-write probe the
	// broker derives for action classes with a known check command
	// (systemctl-style service control). Pin the contract: the
	// callback record's Verification block is populated when the
	// dispatch class supports it, with the derived command and the
	// success/failure outcome of running it. Agents reading the
	// callback (or the SSE event downstream) close the certainty
	// loop on this field.
	store := unifiedresources.NewMemoryStore()
	agentServer := &mockAgentServer{}
	// First call: the action itself.
	agentServer.On("ExecuteCommand", mock.Anything, "agent-vrf",
		mock.MatchedBy(func(p agentexec.ExecuteCommandPayload) bool {
			return p.Command == "systemctl restart nginx"
		})).
		Return(&agentexec.CommandResultPayload{Success: true, Stdout: "", ExitCode: 0}, nil).Once()
	// Second call: the broker-derived verification probe. The
	// expected command is `systemctl is-active 'nginx'`.
	agentServer.On("ExecuteCommand", mock.Anything, "agent-vrf",
		mock.MatchedBy(func(p agentexec.ExecuteCommandPayload) bool {
			return strings.HasPrefix(p.Command, "systemctl is-active")
		})).
		Return(&agentexec.CommandResultPayload{Success: true, Stdout: "active\n", ExitCode: 0}, nil).Once()

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      agentServer,
		ActionAuditStore: store,
	})
	got := make(chan unifiedresources.ActionAuditRecord, 1)
	executor.SetOnActionCompleted(func(rec unifiedresources.ActionAuditRecord) {
		got <- rec
	})

	if _, err := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-vrf",
		"",
		false,
		"agent-vrf",
		agentexec.ExecuteCommandPayload{
			Command:    "systemctl restart nginx",
			TargetType: "agent",
			TargetID:   "agent-vrf",
		},
		"pulse_control",
		"restart nginx",
	); err != nil {
		t.Fatalf("executeCommandWithAudit: %v", err)
	}

	select {
	case received := <-got:
		if received.Result == nil {
			t.Fatal("callback record must carry an ExecutionResult")
		}
		if received.Result.Verification == nil {
			t.Fatal("callback record must carry a Verification block for service-restart actions — drift here breaks the certainty loop on action.completed")
		}
		if received.Verification == nil {
			t.Fatal("callback record must carry the canonical top-level Verification block")
		}
		v := received.Result.Verification
		if !v.Ran {
			t.Error("Verification.Ran must be true after the broker dispatched the probe")
		}
		if received.Verification.Command != v.Command {
			t.Errorf("canonical Verification.Command = %q, want %q", received.Verification.Command, v.Command)
		}
		if !v.Success {
			t.Errorf("Verification.Success: probe returned exit 0, expected Success=true; got %+v", v)
		}
		if !strings.Contains(v.Command, "systemctl is-active") {
			t.Errorf("Verification.Command must surface the derived probe command; got %q", v.Command)
		}
		if v.RanAt.IsZero() {
			t.Error("Verification.RanAt must be populated so agents can reason about probe freshness")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("post-completion callback did not fire within 2s")
	}
	agentServer.AssertExpectations(t)
}

func TestSetOnActionCompleted_FiresOnPlanDriftRefusal(t *testing.T) {
	// Plan-drift refusal lands in the audit store as Failed with the
	// stable `plan_drift:` prefix; the callback must surface that
	// terminal state so agents can branch on the refusal token
	// without polling.
	now := time.Date(2026, 5, 9, 9, 30, 0, 0, time.UTC)
	approvedPlan := unifiedresources.ActionPlan{
		ActionID:         "drift-action",
		RequestID:        "appr-drift",
		Allowed:          true,
		RequiresApproval: true,
		ApprovalPolicy:   unifiedresources.ApprovalAdmin,
		PlannedAt:        now,
		ExpiresAt:        now.Add(5 * time.Minute),
		PlanHash:         "approved-but-drifted",
		Message:          "approved reason",
	}
	approvedReq := &approval.ApprovalRequest{
		ID:          "appr-drift",
		Command:     "approved-cmd",
		TargetType:  "agent",
		TargetID:    "agent-drift",
		TargetName:  "agent-drift",
		Status:      approval.StatusApproved,
		Plan:        &approvedPlan,
		RequestedAt: now,
		ExpiresAt:   now.Add(5 * time.Minute),
	}
	approvalStoreOriginal := approval.GetStore()
	defer approval.SetStore(approvalStoreOriginal)
	memStore := unifiedresources.NewMemoryStore()
	approvalStoreLocal, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DefaultTimeout:     5 * time.Minute,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("approval.NewStore: %v", err)
	}
	if err := approvalStoreLocal.CreateApproval(approvedReq); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	approval.SetStore(approvalStoreLocal)

	executor := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:      &mockAgentServer{},
		ActionAuditStore: memStore,
	})

	got := make(chan unifiedresources.ActionAuditRecord, 1)
	executor.SetOnActionCompleted(func(rec unifiedresources.ActionAuditRecord) {
		got <- rec
	})

	// Submit a payload whose hash will not match the approved plan.
	if _, dispatchErr := executor.executeCommandWithAudit(
		context.Background(),
		"pulse_control",
		"agent-drift",
		"appr-drift",
		true,
		"agent-drift",
		agentexec.ExecuteCommandPayload{
			Command:    "different-cmd",
			TargetType: "agent",
			TargetID:   "agent-drift",
		},
		"pulse_control",
		"different reason",
	); dispatchErr == nil {
		t.Fatal("expected plan-drift refusal; got nil error")
	} else if !strings.Contains(dispatchErr.Error(), "plan") {
		// Defensive: the validator's exact error wording can evolve;
		// just confirm we got an error from the drift path rather
		// than from somewhere else.
		t.Logf("drift error: %v", dispatchErr)
	}

	select {
	case received := <-got:
		if received.State != unifiedresources.ActionStateFailed {
			t.Errorf("refusal state = %q, want %q", received.State, unifiedresources.ActionStateFailed)
		}
		if received.Result == nil || !strings.HasPrefix(received.Result.ErrorMessage, "plan_drift:") {
			t.Errorf("refusal must carry plan_drift: prefix on ErrorMessage; got %#v", received.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("plan-drift callback did not fire within 2s")
	}
}
