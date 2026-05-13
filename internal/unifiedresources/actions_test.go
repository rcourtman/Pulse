package unifiedresources

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeActionAuditRecordPopulatesGovernedPlanPreflight(t *testing.T) {
	now := time.Date(2026, 4, 25, 22, 30, 0, 0, time.UTC)
	record, err := NormalizeActionAuditRecord(ActionAuditRecord{
		ID:        " action-1 ",
		CreatedAt: now,
		State:     ActionStateExecuting,
		Request: ActionRequest{
			ResourceID:     " vm:42 ",
			CapabilityName: " pulse_control ",
			Reason:         "Restart workload",
			RequestedBy:    " pulse_assistant ",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeActionAuditRecord() error = %v", err)
	}

	if record.ID != "action-1" || record.Plan.ActionID != "action-1" {
		t.Fatalf("action id normalization failed: %#v", record)
	}
	if record.Request.RequestID != "action-1" || record.Plan.RequestID != "action-1" {
		t.Fatalf("request id normalization failed: request=%q plan=%q", record.Request.RequestID, record.Plan.RequestID)
	}
	if record.Request.ResourceID != "vm:42" || record.Request.CapabilityName != "pulse_control" || record.Request.RequestedBy != "pulse_assistant" {
		t.Fatalf("request normalization failed: %#v", record.Request)
	}
	if record.Plan.ApprovalPolicy != ApprovalNone {
		t.Fatalf("approval policy = %q, want %q", record.Plan.ApprovalPolicy, ApprovalNone)
	}
	if record.Plan.Preflight == nil {
		t.Fatal("expected preflight to be populated")
	}
	if record.Plan.Preflight.Target != "vm:42" {
		t.Fatalf("preflight target = %q, want vm:42", record.Plan.Preflight.Target)
	}
	if record.Plan.Preflight.DryRunAvailable {
		t.Fatal("default preflight must explicitly mark dry-run unavailable")
	}
	if !strings.Contains(record.Plan.Preflight.DryRunSummary, "No provider-supported dry run") {
		t.Fatalf("unexpected dry-run summary: %q", record.Plan.Preflight.DryRunSummary)
	}
	if len(record.Plan.Preflight.SafetyChecks) == 0 || len(record.Plan.Preflight.VerificationSteps) == 0 {
		t.Fatalf("preflight should carry safety and verification checks: %#v", record.Plan.Preflight)
	}

	withLegacyVerification := record
	withLegacyVerification.Result = &ExecutionResult{
		Success: true,
		Verification: &ActionVerificationResult{
			Ran:     true,
			Command: " systemctl is-active 'nginx' ",
			Output:  " active\n",
			Success: true,
			RanAt:   now.Add(time.Minute),
		},
	}
	withLegacyVerification, err = NormalizeActionAuditRecord(withLegacyVerification)
	if err != nil {
		t.Fatalf("NormalizeActionAuditRecord(legacy verification) error = %v", err)
	}
	if withLegacyVerification.Verification == nil || withLegacyVerification.Verification.Command != "systemctl is-active 'nginx'" || withLegacyVerification.Verification.Output != "active" {
		t.Fatalf("canonical verification not populated from result verification: %#v", withLegacyVerification.Verification)
	}
	if withLegacyVerification.Result.Verification == nil || withLegacyVerification.Result.Verification.Command != withLegacyVerification.Verification.Command {
		t.Fatalf("legacy verification not kept aligned: result=%#v canonical=%#v", withLegacyVerification.Result.Verification, withLegacyVerification.Verification)
	}

	withUnrunVerification := record
	withUnrunVerification.Verification = &ActionVerificationResult{
		Ran:     false,
		Command: "should not persist",
		Output:  "sensitive",
		Success: true,
		RanAt:   now.Add(time.Minute),
		Note:    "details",
	}
	withUnrunVerification, err = NormalizeActionAuditRecord(withUnrunVerification)
	if err != nil {
		t.Fatalf("NormalizeActionAuditRecord(unrun verification) error = %v", err)
	}
	if withUnrunVerification.Verification == nil || withUnrunVerification.Verification.Ran {
		t.Fatalf("expected canonical ran=false verification, got %#v", withUnrunVerification.Verification)
	}
	if withUnrunVerification.Verification.Command != "" || withUnrunVerification.Verification.Output != "" || withUnrunVerification.Verification.Note != "" || !withUnrunVerification.Verification.RanAt.IsZero() {
		t.Fatalf("ran=false verification must not retain details: %#v", withUnrunVerification.Verification)
	}
}

func TestNormalizeActionAuditRecordRejectsUngovernedRecords(t *testing.T) {
	_, err := NormalizeActionAuditRecord(ActionAuditRecord{
		ID:    "action-1",
		State: ActionStateCompleted,
		Request: ActionRequest{
			ResourceID:  "vm:42",
			RequestedBy: "pulse_assistant",
		},
	})
	if err == nil {
		t.Fatal("expected missing capability to be rejected")
	}

	_, err = NormalizeActionAuditRecord(ActionAuditRecord{
		ID:    "action-1",
		State: ActionState("unknown"),
		Request: ActionRequest{
			ResourceID:     "vm:42",
			CapabilityName: "pulse_control",
			RequestedBy:    "pulse_assistant",
		},
	})
	if err == nil {
		t.Fatal("expected invalid state to be rejected")
	}
}

func TestNormalizeActionLifecycleEventRejectsInvalidEvents(t *testing.T) {
	if _, err := NormalizeActionLifecycleEvent(ActionLifecycleEvent{State: ActionStatePlanned}); err == nil {
		t.Fatal("expected missing action id to be rejected")
	}
	if _, err := NormalizeActionLifecycleEvent(ActionLifecycleEvent{ActionID: "action-1", State: ActionState("paused")}); err == nil {
		t.Fatal("expected invalid lifecycle state to be rejected")
	}
}

func TestApplyActionDecisionApprovesPendingActionWithoutExecution(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_test",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		State:     ActionStatePending,
		Request: ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover service",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:         "act_test",
			RequestID:        "req-1",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
	}

	updated, event, err := ApplyActionDecision(record, ActionApprovalRecord{
		Actor:   "operator@example.com",
		Outcome: OutcomeApproved,
		Reason:  "inside maintenance window",
	}, now)
	if err != nil {
		t.Fatalf("ApplyActionDecision: %v", err)
	}
	if updated.State != ActionStateApproved || updated.Result != nil {
		t.Fatalf("updated action = %#v, want approved without execution result", updated)
	}
	if len(updated.Approvals) != 1 || updated.Approvals[0].Method != MethodAPI || updated.Approvals[0].Actor != "operator@example.com" {
		t.Fatalf("approval record = %#v", updated.Approvals)
	}
	if event.ActionID != "act_test" || event.State != ActionStateApproved || !strings.Contains(event.Message, "Execution remains pending") {
		t.Fatalf("lifecycle event = %#v", event)
	}
}

func TestApplyActionDecisionRejectsUnsafeTransitions(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	base := ActionAuditRecord{
		ID:        "act_test",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		State:     ActionStatePending,
		Request: ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover service",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:        "act_test",
			RequestID:       "req-1",
			ExpiresAt:       now.Add(time.Minute),
			ResourceVersion: "resource:sha256:test",
			PolicyVersion:   "policy:sha256:test",
			PlanHash:        "sha256:test",
		},
	}
	if _, _, err := ApplyActionDecision(base, ActionApprovalRecord{Outcome: ApprovalOutcome("maybe")}, now); !errors.Is(err, ErrInvalidApprovalOutcome) {
		t.Fatalf("invalid outcome error = %v", err)
	}
	notPending := base
	notPending.State = ActionStateApproved
	if _, _, err := ApplyActionDecision(notPending, ActionApprovalRecord{Outcome: OutcomeApproved}, now); !errors.Is(err, ErrActionNotPending) {
		t.Fatalf("not pending error = %v", err)
	}
	expired := base
	expired.Plan.ExpiresAt = now.Add(-time.Second)
	if _, _, err := ApplyActionDecision(expired, ActionApprovalRecord{Outcome: OutcomeApproved}, now); !errors.Is(err, ErrActionPlanExpired) {
		t.Fatalf("expired error = %v", err)
	}
}

func TestBeginActionExecutionStartsApprovedAction(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 30, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_execute",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
		State:     ActionStateApproved,
		Request: ActionRequest{
			RequestID:      "req-execute",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover service",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:         "act_execute",
			RequestID:        "req-execute",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
		Approvals: []ActionApprovalRecord{
			{Actor: "operator@example.com", Outcome: OutcomeApproved, Timestamp: now.Add(-30 * time.Second)},
		},
	}

	updated, event, err := BeginActionExecution(record, "operator@example.com", now)
	if err != nil {
		t.Fatalf("BeginActionExecution: %v", err)
	}
	if updated.State != ActionStateExecuting || updated.Result != nil || !updated.UpdatedAt.Equal(now) {
		t.Fatalf("updated action = %#v, want executing without result", updated)
	}
	if event.ActionID != "act_execute" || event.State != ActionStateExecuting || event.Actor != "operator@example.com" {
		t.Fatalf("lifecycle event = %#v", event)
	}
}

func TestBeginActionExecutionRejectsUnsafeStates(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 30, 0, 0, time.UTC)
	base := ActionAuditRecord{
		ID:        "act_execute",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		State:     ActionStatePending,
		Request: ActionRequest{
			RequestID:      "req-execute",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover service",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:         "act_execute",
			RequestID:        "req-execute",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
	}
	if _, _, err := BeginActionExecution(base, "operator@example.com", now); !errors.Is(err, ErrActionNotApproved) {
		t.Fatalf("pending error = %v, want %v", err, ErrActionNotApproved)
	}
	executing := base
	executing.State = ActionStateExecuting
	if _, _, err := BeginActionExecution(executing, "operator@example.com", now); !errors.Is(err, ErrActionAlreadyExecuting) {
		t.Fatalf("executing error = %v, want %v", err, ErrActionAlreadyExecuting)
	}
	completed := base
	completed.State = ActionStateCompleted
	if _, _, err := BeginActionExecution(completed, "operator@example.com", now); !errors.Is(err, ErrActionExecutionFinal) {
		t.Fatalf("completed error = %v, want %v", err, ErrActionExecutionFinal)
	}
	failedExpired := base
	failedExpired.State = ActionStateFailed
	failedExpired.Plan.ExpiresAt = now.Add(-time.Second)
	if _, _, err := BeginActionExecution(failedExpired, "operator@example.com", now); !errors.Is(err, ErrActionExecutionFinal) {
		t.Fatalf("failed expired error = %v, want %v", err, ErrActionExecutionFinal)
	}
	expired := base
	expired.State = ActionStateApproved
	expired.Plan.ExpiresAt = now.Add(-time.Second)
	if _, _, err := BeginActionExecution(expired, "operator@example.com", now); !errors.Is(err, ErrActionPlanExpired) {
		t.Fatalf("expired error = %v, want %v", err, ErrActionPlanExpired)
	}
	dryRunOnly := base
	dryRunOnly.State = ActionStatePlanned
	dryRunOnly.Plan.RequiresApproval = false
	dryRunOnly.Plan.ApprovalPolicy = ApprovalDryRun
	if _, _, err := BeginActionExecution(dryRunOnly, "operator@example.com", now); !errors.Is(err, ErrActionDryRunOnly) {
		t.Fatalf("dry-run-only error = %v, want %v", err, ErrActionDryRunOnly)
	}
}

func TestRefuseActionExecutionRecordsPermanentRefusal(t *testing.T) {
	now := time.Date(2026, 5, 4, 13, 0, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_refused",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
		State:     ActionStateApproved,
		Request: ActionRequest{
			RequestID:      "req-refused",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover service",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:         "act_refused",
			RequestID:        "req-refused",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
	}

	for _, tc := range []struct {
		name       string
		reason     error
		wantPrefix string
	}{
		{name: "plan drift", reason: ErrActionPlanDrift, wantPrefix: "plan_drift:"},
		{name: "expired", reason: ErrActionPlanExpired, wantPrefix: "action_plan_expired:"},
		{name: "dry run only", reason: ErrActionDryRunOnly, wantPrefix: "action_dry_run_only:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated, event, err := RefuseActionExecution(record, tc.reason, " operator@example.com ", now)
			if err != nil {
				t.Fatalf("RefuseActionExecution: %v", err)
			}
			if updated.State != ActionStateFailed || updated.Result == nil || updated.Result.Success {
				t.Fatalf("updated action = %#v, want failed result", updated)
			}
			if !strings.HasPrefix(updated.Result.ErrorMessage, tc.wantPrefix) {
				t.Fatalf("ErrorMessage = %q, want prefix %q", updated.Result.ErrorMessage, tc.wantPrefix)
			}
			if event.ActionID != updated.ID || event.State != ActionStateFailed || event.Actor != "operator@example.com" || event.Message != updated.Result.ErrorMessage {
				t.Fatalf("lifecycle event = %#v, updated result = %#v", event, updated.Result)
			}
		})
	}

	if _, _, err := RefuseActionExecution(record, errors.New("transient executor unavailable"), "", now); !errors.Is(err, ErrActionExecutionRefusal) {
		t.Fatalf("unexpected refusal error = %v, want %v", err, ErrActionExecutionRefusal)
	}
}

func TestCompleteActionExecutionRecordsResult(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 45, 0, 0, time.UTC)
	record := ActionAuditRecord{
		ID:        "act_execute",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-30 * time.Second),
		State:     ActionStateExecuting,
		Request: ActionRequest{
			RequestID:      "req-execute",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "recover service",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: ActionPlan{
			ActionID:         "act_execute",
			RequestID:        "req-execute",
			Allowed:          true,
			RequiresApproval: true,
			ApprovalPolicy:   ApprovalAdmin,
			PlannedAt:        now.Add(-time.Minute),
			ExpiresAt:        now.Add(time.Minute),
			ResourceVersion:  "resource:sha256:test",
			PolicyVersion:    "policy:sha256:test",
			PlanHash:         "sha256:test",
		},
	}

	updated, event, err := CompleteActionExecution(record, &ExecutionResult{
		Success: true,
		Output:  "done",
		Verification: &ActionVerificationResult{
			Ran:     true,
			Command: "systemctl is-active 'nginx'",
			Output:  "active",
			Success: true,
			RanAt:   now,
		},
	}, "operator@example.com", now)
	if err != nil {
		t.Fatalf("CompleteActionExecution: %v", err)
	}
	if updated.State != ActionStateCompleted || updated.Result == nil || updated.Result.Output != "done" {
		t.Fatalf("completed action = %#v", updated)
	}
	if updated.Verification == nil || !updated.Verification.Ran || updated.Verification.Command != "systemctl is-active 'nginx'" {
		t.Fatalf("completed action verification = %#v", updated.Verification)
	}
	if event.State != ActionStateCompleted || event.Message != "Action execution completed." {
		t.Fatalf("completed event = %#v", event)
	}

	failed, failedEvent, err := CompleteActionExecution(record, &ExecutionResult{Success: false, ErrorMessage: "provider rejected restart"}, "operator@example.com", now)
	if err != nil {
		t.Fatalf("CompleteActionExecution failed result: %v", err)
	}
	if failed.State != ActionStateFailed || failed.Result == nil || failed.Result.ErrorMessage != "provider rejected restart" {
		t.Fatalf("failed action = %#v", failed)
	}
	if failedEvent.State != ActionStateFailed || failedEvent.Message != "provider rejected restart" {
		t.Fatalf("failed event = %#v", failedEvent)
	}

	notExecuting := record
	notExecuting.State = ActionStateApproved
	if _, _, err := CompleteActionExecution(notExecuting, nil, "operator@example.com", now); !errors.Is(err, ErrActionNotExecuting) {
		t.Fatalf("not executing error = %v, want %v", err, ErrActionNotExecuting)
	}
}
