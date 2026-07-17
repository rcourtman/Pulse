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
	completed, doneEvent, err := unifiedresources.CompleteActionExecution(started, &unifiedresources.ExecutionResult{Success: true, Verification: &unifiedresources.ActionVerificationResult{Ran: true, Success: true, RanAt: now.Add(-80 * time.Minute)}}, "operator", now.Add(-80*time.Minute))
	if err != nil {
		t.Fatalf("CompleteActionExecution(currentApproved): %v", err)
	}
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
	rejectedAt := now.Add(-10 * time.Minute)
	rejected, rejectedEvent, err := unifiedresources.ApplyActionDecision(oldRejected, boundActionTestDecisionApproval(oldRejected.ID, oldRejected.Plan.PlanHash, "operator", unifiedresources.OutcomeRejected, rejectedAt), rejectedAt)
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

func TestGetPulseIntelligenceActionTelemetry_AttributesApprovedActionFailureCauses(t *testing.T) {
	// Cause classification measures staleness against the wall clock, so this
	// test anchors records to real time instead of a fixed date.
	now := time.Now().UTC()
	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
	dataDir := t.TempDir()
	router := &Router{
		resourceHandlers: NewResourceHandlers(&config.Config{DataPath: dataDir}),
	}
	store, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}

	approvedAt := func(at time.Time) []unifiedresources.ActionApprovalRecord {
		return []unifiedresources.ActionApprovalRecord{{
			Outcome:   unifiedresources.OutcomeApproved,
			Method:    unifiedresources.MethodUI,
			Timestamp: at,
			Actor:     "operator",
		}}
	}

	// Approved, then terminally refused before dispatch (plan drift).
	refused := pulseTelemetryActionRecord("refused-pre-dispatch", now.Add(-3*time.Hour), unifiedresources.ActionStateApproved, true, approvedAt(now.Add(-3*time.Hour)))
	refused.Plan.ExpiresAt = now.Add(time.Hour)
	if err := store.RecordActionAudit(refused); err != nil {
		t.Fatalf("RecordActionAudit(refused): %v", err)
	}
	if _, err := actionlifecycle.RecordRefusedExecution(store, refused, "operator", now.Add(-170*time.Minute), unifiedresources.ErrActionPlanDrift); err != nil {
		t.Fatalf("RecordRefusedExecution(refused): %v", err)
	}

	// Approved, dispatched, execution failed on the agent.
	execFailed := pulseTelemetryActionRecord("exec-failed", now.Add(-2*time.Hour), unifiedresources.ActionStateFailed, true, approvedAt(now.Add(-2*time.Hour)))
	execFailed.Result = &unifiedresources.ExecutionResult{Success: false, ErrorMessage: "image pull failed"}
	if err := store.RecordActionAudit(execFailed); err != nil {
		t.Fatalf("RecordActionAudit(execFailed): %v", err)
	}

	// Approved, execution succeeded, but no verification evidence confirmed it.
	unverified := pulseTelemetryActionRecord("completed-unverified", now.Add(-100*time.Minute), unifiedresources.ActionStateCompleted, true, approvedAt(now.Add(-100*time.Minute)))
	unverified.Result = &unifiedresources.ExecutionResult{Success: true}
	if err := store.RecordActionAudit(unverified); err != nil {
		t.Fatalf("RecordActionAudit(unverified): %v", err)
	}

	// Approved and abandoned in executing state well past the dispatch window.
	stuck := pulseTelemetryActionRecord("stuck-executing", now.Add(-5*time.Hour), unifiedresources.ActionStateExecuting, true, approvedAt(now.Add(-5*time.Hour)))
	if err := store.RecordActionAudit(stuck); err != nil {
		t.Fatalf("RecordActionAudit(stuck): %v", err)
	}

	// Approved and still legitimately in flight: no failure bucket.
	inFlight := pulseTelemetryActionRecord("in-flight", now.Add(-5*time.Minute), unifiedresources.ActionStateExecuting, true, approvedAt(now.Add(-5*time.Minute)))
	if err := store.RecordActionAudit(inFlight); err != nil {
		t.Fatalf("RecordActionAudit(inFlight): %v", err)
	}

	// Approved and verified success: not a failure.
	verified := pulseTelemetryActionRecord("verified-success", now.Add(-90*time.Minute), unifiedresources.ActionStateCompleted, true, approvedAt(now.Add(-90*time.Minute)))
	verified.Result = &unifiedresources.ExecutionResult{Success: true}
	verified.VerificationOutcome = unifiedresources.VerificationOutcome{Status: unifiedresources.VerificationVerified}
	if err := store.RecordActionAudit(verified); err != nil {
		t.Fatalf("RecordActionAudit(verified): %v", err)
	}

	got := router.GetPulseIntelligenceActionTelemetry(since)

	if got.ApprovedActionAttempts30d != 6 {
		t.Fatalf("ApprovedActionAttempts30d = %d, want 6", got.ApprovedActionAttempts30d)
	}
	if got.ApprovedActionSuccesses30d != 1 {
		t.Fatalf("ApprovedActionSuccesses30d = %d, want 1", got.ApprovedActionSuccesses30d)
	}
	if got.ApprovedActionFailuresPreDispatch30d != 1 {
		t.Fatalf("ApprovedActionFailuresPreDispatch30d = %d, want 1", got.ApprovedActionFailuresPreDispatch30d)
	}
	if got.ApprovedActionFailuresExecution30d != 1 {
		t.Fatalf("ApprovedActionFailuresExecution30d = %d, want 1", got.ApprovedActionFailuresExecution30d)
	}
	if got.ApprovedActionFailuresUnverified30d != 1 {
		t.Fatalf("ApprovedActionFailuresUnverified30d = %d, want 1", got.ApprovedActionFailuresUnverified30d)
	}
	if got.ApprovedActionStuckExecuting30d != 1 {
		t.Fatalf("ApprovedActionStuckExecuting30d = %d, want 1", got.ApprovedActionStuckExecuting30d)
	}
	// The completed-unverified record is the most recent failure; the legacy
	// row carries no canonical reason code, so the sanitized fallback applies.
	if got.ApprovedActionLastFailureReason30d != "verification_unconfirmed" {
		t.Fatalf("ApprovedActionLastFailureReason30d = %q, want %q", got.ApprovedActionLastFailureReason30d, "verification_unconfirmed")
	}
}

func TestGetPulseIntelligenceActionTelemetry_LastFailureReasonUsesCanonicalReasonCode(t *testing.T) {
	now := time.Now().UTC()
	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
	dataDir := t.TempDir()
	router := &Router{
		resourceHandlers: NewResourceHandlers(&config.Config{DataPath: dataDir}),
	}
	store, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}

	refused := pulseTelemetryActionRecord("refused-plan-drift", now.Add(-time.Hour), unifiedresources.ActionStateApproved, true, []unifiedresources.ActionApprovalRecord{{
		Outcome:   unifiedresources.OutcomeApproved,
		Method:    unifiedresources.MethodUI,
		Timestamp: now.Add(-time.Hour),
		Actor:     "operator",
	}})
	refused.Plan.ExpiresAt = now.Add(time.Hour)
	if err := store.RecordActionAudit(refused); err != nil {
		t.Fatalf("RecordActionAudit(refused): %v", err)
	}
	if _, err := actionlifecycle.RecordRefusedExecution(store, refused, "operator", now.Add(-30*time.Minute), unifiedresources.ErrActionPlanDrift); err != nil {
		t.Fatalf("RecordRefusedExecution(refused): %v", err)
	}

	got := router.GetPulseIntelligenceActionTelemetry(since)

	if got.ApprovedActionFailuresPreDispatch30d != 1 {
		t.Fatalf("ApprovedActionFailuresPreDispatch30d = %d, want 1", got.ApprovedActionFailuresPreDispatch30d)
	}
	if got.ApprovedActionLastFailureReason30d != "plan_drift" {
		t.Fatalf("ApprovedActionLastFailureReason30d = %q, want %q", got.ApprovedActionLastFailureReason30d, "plan_drift")
	}
}

func pulseTelemetryActionRecord(
	id string,
	createdAt time.Time,
	state unifiedresources.ActionState,
	requiresApproval bool,
	approvals []unifiedresources.ActionApprovalRecord,
) unifiedresources.ActionAuditRecord {
	approvalPolicy := unifiedresources.ApprovalNone
	if requiresApproval {
		approvalPolicy = unifiedresources.ApprovalAdmin
	}
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
			Actor:          unifiedresources.ActionActor{SubjectID: "pulse_assistant", Kind: unifiedresources.ActionActorService, CredentialID: "service:test", OrgID: "default"},
		},
		Plan: unifiedresources.ActionPlan{
			ActionID:            id,
			RequestID:           "req-" + id,
			Allowed:             true,
			RequiresApproval:    requiresApproval,
			ApprovalPolicy:      approvalPolicy,
			ApprovalRequirement: unifiedresources.ApprovalRequirementForFloor(approvalPolicy),
			PlannedAt:           createdAt,
			ExpiresAt:           createdAt.Add(time.Hour),
			PlanHash:            "sha256:" + id,
		},
		Approvals: approvals,
	}
}
