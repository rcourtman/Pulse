package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestGetPulseIntelligenceActionTelemetry_SummarizesActionGovernanceOnly(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
	dataDir := t.TempDir()
	mtPersistence := config.NewMultiTenantPersistence(dataDir)
	if err := mtPersistence.SaveOrganization(&models.Organization{ID: "tenant-a", DisplayName: "Tenant A"}); err != nil {
		t.Fatalf("SaveOrganization: %v", err)
	}

	router := &Router{
		multiTenant:      mtPersistence,
		resourceHandlers: NewResourceHandlers(&config.Config{DataPath: dataDir}),
	}

	defaultStore, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("default getStore: %v", err)
	}
	tenantStore, err := router.resourceHandlers.getStore("tenant-a")
	if err != nil {
		t.Fatalf("tenant getStore: %v", err)
	}

	approvedCompleted := pulseTelemetryActionRecord("approved-completed", now.Add(-35*time.Minute), unifiedresources.ActionStateCompleted, true, []unifiedresources.ActionApprovalRecord{
		{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodUI, Timestamp: now.Add(-34 * time.Minute), Actor: "operator"},
	})
	approvedCompleted.Result = &unifiedresources.ExecutionResult{Success: true}
	approvedCompleted.VerificationOutcome = unifiedresources.VerificationOutcome{Status: unifiedresources.VerificationVerified}
	records := map[unifiedresources.ResourceStore][]unifiedresources.ActionAuditRecord{
		defaultStore: {
			pulseTelemetryActionRecord("plan-only", now.Add(-time.Hour), unifiedresources.ActionStatePlanned, false, nil),
			pulseTelemetryActionRecord("pending", now.Add(-50*time.Minute), unifiedresources.ActionStatePending, true, nil),
			pulseTelemetryActionRecord("approved-executing", now.Add(-40*time.Minute), unifiedresources.ActionStateExecuting, true, []unifiedresources.ActionApprovalRecord{
				{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodUI, Timestamp: now.Add(-39 * time.Minute), Actor: "operator"},
			}),
			approvedCompleted,
			pulseTelemetryActionRecord("old-approved", since.Add(-time.Minute), unifiedresources.ActionStateCompleted, true, []unifiedresources.ActionApprovalRecord{
				{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodUI, Timestamp: since.Add(-2 * time.Minute), Actor: "operator"},
			}),
		},
		tenantStore: {
			pulseTelemetryActionRecord("tenant-approved-failed", now.Add(-30*time.Minute), unifiedresources.ActionStateFailed, true, []unifiedresources.ActionApprovalRecord{
				{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodAPI, Timestamp: now.Add(-29 * time.Minute), Actor: "operator"},
			}),
			pulseTelemetryActionRecord("tenant-rejected", now.Add(-20*time.Minute), unifiedresources.ActionStateRejected, true, []unifiedresources.ActionApprovalRecord{
				{Outcome: unifiedresources.OutcomeRejected, Method: unifiedresources.MethodUI, Timestamp: now.Add(-19 * time.Minute), Actor: "operator"},
			}),
		},
	}
	for store, storeRecords := range records {
		for _, record := range storeRecords {
			if err := store.RecordActionAudit(record); err != nil {
				t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
			}
		}
	}

	got := router.GetPulseIntelligenceActionTelemetry(since)

	if got.ActionPlans30d != 6 {
		t.Fatalf("ActionPlans30d = %d, want 6", got.ActionPlans30d)
	}
	if got.ApprovalRequests30d != 5 {
		t.Fatalf("ApprovalRequests30d = %d, want 5", got.ApprovalRequests30d)
	}
	if got.RejectedActionDecisions30d != 1 {
		t.Fatalf("RejectedActionDecisions30d = %d, want 1", got.RejectedActionDecisions30d)
	}
	if got.ApprovedActionDecisions30d != 3 {
		t.Fatalf("ApprovedActionDecisions30d = %d, want 3", got.ApprovedActionDecisions30d)
	}
	if got.ApprovedActionAttempts30d != 3 {
		t.Fatalf("ApprovedActionAttempts30d = %d, want 3", got.ApprovedActionAttempts30d)
	}
	if got.ApprovedActionSuccesses30d != 1 {
		t.Fatalf("ApprovedActionSuccesses30d = %d, want 1", got.ApprovedActionSuccesses30d)
	}
}

func TestGetPulseIntelligenceActionTelemetry_CountsApprovedLifecycleAttemptsInsideWindow(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 30, 0, 0, time.UTC)
	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
	dataDir := t.TempDir()
	router := &Router{
		resourceHandlers: NewResourceHandlers(&config.Config{DataPath: dataDir}),
	}
	store, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}

	currentApproved := pulseTelemetryActionRecord("current-approved-executed", now.Add(-2*time.Hour), unifiedresources.ActionStateApproved, true, []unifiedresources.ActionApprovalRecord{
		{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodUI, Timestamp: now.Add(-110 * time.Minute), Actor: "operator"},
	})
	if err := store.RecordActionAudit(currentApproved); err != nil {
		t.Fatalf("RecordActionAudit(currentApproved): %v", err)
	}
	started, startEvent, err := unifiedresources.BeginActionExecution(currentApproved, "operator", now.Add(-90*time.Minute))
	if err != nil {
		t.Fatalf("BeginActionExecution(currentApproved): %v", err)
	}
	if err := store.RecordActionExecutionStart(started, startEvent); err != nil {
		t.Fatalf("RecordActionExecutionStart(currentApproved): %v", err)
	}
	completed, doneEvent, err := unifiedresources.CompleteActionExecution(started, &unifiedresources.ExecutionResult{Success: true}, "operator", now.Add(-80*time.Minute))
	if err != nil {
		t.Fatalf("CompleteActionExecution(currentApproved): %v", err)
	}
	completed.VerificationOutcome = unifiedresources.VerificationOutcome{Status: unifiedresources.VerificationVerified}
	if err := store.RecordActionExecutionResult(completed, doneEvent); err != nil {
		t.Fatalf("RecordActionExecutionResult(currentApproved): %v", err)
	}

	oldCreatedAt := since.Add(-time.Minute)
	oldApproved := pulseTelemetryActionRecord("old-approved-executed", oldCreatedAt, unifiedresources.ActionStateApproved, true, []unifiedresources.ActionApprovalRecord{
		{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodAPI, Timestamp: oldCreatedAt.Add(time.Minute), Actor: "operator"},
	})
	oldApproved.Plan.ExpiresAt = now.Add(time.Hour)
	if err := store.RecordActionAudit(oldApproved); err != nil {
		t.Fatalf("RecordActionAudit(oldApproved): %v", err)
	}
	started, startEvent, err = unifiedresources.BeginActionExecution(oldApproved, "operator", now.Add(-60*time.Minute))
	if err != nil {
		t.Fatalf("BeginActionExecution(oldApproved): %v", err)
	}
	if err := store.RecordActionExecutionStart(started, startEvent); err != nil {
		t.Fatalf("RecordActionExecutionStart(oldApproved): %v", err)
	}

	oldRefused := pulseTelemetryActionRecord("old-approved-refused", oldCreatedAt, unifiedresources.ActionStateApproved, true, []unifiedresources.ActionApprovalRecord{
		{Outcome: unifiedresources.OutcomeApproved, Method: unifiedresources.MethodAPI, Timestamp: oldCreatedAt.Add(time.Minute), Actor: "operator"},
	})
	oldRefused.Plan.ExpiresAt = now.Add(time.Hour)
	if err := store.RecordActionAudit(oldRefused); err != nil {
		t.Fatalf("RecordActionAudit(oldRefused): %v", err)
	}
	if _, err := actionlifecycle.RecordRefusedExecution(store, oldRefused, "operator", now.Add(-30*time.Minute), unifiedresources.ErrActionPlanDrift); err != nil {
		t.Fatalf("recordRefusedActionExecution(oldRefused): %v", err)
	}

	unapprovedRefused := pulseTelemetryActionRecord("old-unapproved-refused", oldCreatedAt, unifiedresources.ActionStatePlanned, false, nil)
	unapprovedRefused.Plan.ExpiresAt = now.Add(time.Hour)
	if err := store.RecordActionAudit(unapprovedRefused); err != nil {
		t.Fatalf("RecordActionAudit(unapprovedRefused): %v", err)
	}
	if _, err := actionlifecycle.RecordRefusedExecution(store, unapprovedRefused, "operator", now.Add(-15*time.Minute), unifiedresources.ErrActionPlanDrift); err != nil {
		t.Fatalf("recordRefusedActionExecution(unapprovedRefused): %v", err)
	}

	oldRejected := pulseTelemetryActionRecord("old-rejected-decision", oldCreatedAt, unifiedresources.ActionStatePending, true, nil)
	oldRejected.Plan.ExpiresAt = now.Add(time.Hour)
	if err := store.RecordActionAudit(oldRejected); err != nil {
		t.Fatalf("RecordActionAudit(oldRejected): %v", err)
	}
	rejected, rejectedEvent, err := unifiedresources.ApplyActionDecision(oldRejected, unifiedresources.ActionApprovalRecord{
		Outcome: unifiedresources.OutcomeRejected,
		Method:  unifiedresources.MethodAPI,
		Actor:   "operator",
	}, now.Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("ApplyActionDecision(oldRejected): %v", err)
	}
	if err := store.RecordActionDecision(rejected, rejectedEvent); err != nil {
		t.Fatalf("RecordActionDecision(oldRejected): %v", err)
	}

	got := router.GetPulseIntelligenceActionTelemetry(since)

	if got.ActionPlans30d != 1 {
		t.Fatalf("ActionPlans30d = %d, want 1", got.ActionPlans30d)
	}
	if got.ApprovalRequests30d != 1 {
		t.Fatalf("ApprovalRequests30d = %d, want 1", got.ApprovalRequests30d)
	}
	if got.RejectedActionDecisions30d != 1 {
		t.Fatalf("RejectedActionDecisions30d = %d, want 1", got.RejectedActionDecisions30d)
	}
	if got.ApprovedActionDecisions30d != 1 {
		t.Fatalf("ApprovedActionDecisions30d = %d, want 1", got.ApprovedActionDecisions30d)
	}
	if got.ApprovedActionAttempts30d != 3 {
		t.Fatalf("ApprovedActionAttempts30d = %d, want 3", got.ApprovedActionAttempts30d)
	}
	if got.ApprovedActionSuccesses30d != 1 {
		t.Fatalf("ApprovedActionSuccesses30d = %d, want 1", got.ApprovedActionSuccesses30d)
	}
}

func TestGetPulseIntelligenceActionTelemetry_RequiresVerifiedOutcomeForApprovedSuccess(t *testing.T) {
	now := time.Date(2026, 6, 20, 11, 30, 0, 0, time.UTC)
	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
	dataDir := t.TempDir()
	router := &Router{
		resourceHandlers: NewResourceHandlers(&config.Config{DataPath: dataDir}),
	}
	store, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}

	approved := []unifiedresources.ActionApprovalRecord{{
		Outcome:   unifiedresources.OutcomeApproved,
		Method:    unifiedresources.MethodUI,
		Timestamp: now.Add(-50 * time.Minute),
		Actor:     "operator",
	}}
	successOnly := pulseTelemetryActionRecord("success-only", now.Add(-55*time.Minute), unifiedresources.ActionStateCompleted, true, approved)
	successOnly.Result = &unifiedresources.ExecutionResult{Success: true}

	outcomeVerified := pulseTelemetryActionRecord("outcome-verified", now.Add(-45*time.Minute), unifiedresources.ActionStateCompleted, true, approved)
	outcomeVerified.Result = &unifiedresources.ExecutionResult{Success: true}
	outcomeVerified.VerificationOutcome = unifiedresources.VerificationOutcome{Status: unifiedresources.VerificationVerified}

	legacyVerification := pulseTelemetryActionRecord("legacy-verification", now.Add(-35*time.Minute), unifiedresources.ActionStateCompleted, true, approved)
	legacyVerification.Result = &unifiedresources.ExecutionResult{Success: true}
	legacyVerification.Verification = &unifiedresources.ActionVerificationResult{
		Ran:     true,
		Success: true,
		RanAt:   now.Add(-30 * time.Minute),
	}

	for _, record := range []unifiedresources.ActionAuditRecord{successOnly, outcomeVerified, legacyVerification} {
		if err := store.RecordActionAudit(record); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
		}
	}

	got := router.GetPulseIntelligenceActionTelemetry(since)

	if got.ApprovedActionSuccesses30d != 2 {
		t.Fatalf("ApprovedActionSuccesses30d = %d, want 2 verified outcomes; bare successful execution must not count", got.ApprovedActionSuccesses30d)
	}
}

func pulseTelemetryActionRecord(
	id string,
	createdAt time.Time,
	state unifiedresources.ActionState,
	requiresApproval bool,
	approvals []unifiedresources.ActionApprovalRecord,
) unifiedresources.ActionAuditRecord {
	return unifiedresources.ActionAuditRecord{
		ID:        id,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		State:     state,
		Request: unifiedresources.ActionRequest{
			RequestID:      "req-" + id,
			ResourceID:     "vm:101",
			CapabilityName: "pulse_exec",
			Reason:         "test",
			RequestedBy:    "pulse_assistant",
		},
		Plan: unifiedresources.ActionPlan{
			ActionID:         id,
			RequestID:        "req-" + id,
			Allowed:          true,
			RequiresApproval: requiresApproval,
			PlannedAt:        createdAt,
			ExpiresAt:        createdAt.Add(time.Hour),
		},
		Approvals: approvals,
	}
}
