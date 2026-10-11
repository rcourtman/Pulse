package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

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

// seedPendingApprovalAudit persists the planned and pending audit state a
// governed approval request carries before an operator decides it.
func seedPendingApprovalAudit(t *testing.T, store unifiedresources.ResourceStore, req *approval.ApprovalRequest) {
	t.Helper()
	actor := approval.RequesterForRequest(req)
	record := actionAuditRecordFromApproval(req, unifiedresources.ActionStatePending, actor)
	events := []unifiedresources.ActionLifecycleEvent{
		{ActionID: req.Plan.ActionID, State: unifiedresources.ActionStatePlanned, Timestamp: record.CreatedAt, Actor: actor, Message: strings.TrimSpace(req.Context)},
		{ActionID: req.Plan.ActionID, State: unifiedresources.ActionStatePending, Timestamp: record.CreatedAt, Actor: actor, Message: "waiting for approval"},
	}
	if _, _, err := store.CreateActionAudit(record, events); err != nil {
		t.Fatalf("CreateActionAudit: %v", err)
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
	seedPendingApprovalAudit(t, actionStore, req)
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
	seedPendingApprovalAudit(t, actionStore, req)
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
