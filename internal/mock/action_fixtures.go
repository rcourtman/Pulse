package mock

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// ActionFixture is one graph-owned action audit and its append-only lifecycle.
// Mock API handlers project this shape through the same public response types as
// the durable store without writing demo records into a user's audit database.
type ActionFixture struct {
	Audit  unifiedresources.ActionAuditRecord
	Events []unifiedresources.ActionLifecycleEvent
}

// ActionFixtures returns defensive copies of the canonical mock action set.
func ActionFixtures() []ActionFixture {
	if !IsMockEnabled() {
		return nil
	}
	return CurrentFixtureGraph().ActionFixtures
}

func buildActionFixtures(resources []unifiedresources.Resource, now time.Time) []ActionFixture {
	now = now.UTC()
	hostID := mockActionResourceID(resources, "agent", "system", "physical-host")
	vmID := mockActionResourceID(resources, "vm")
	containerID := mockActionResourceID(resources, "app-container")

	pending := newMockActionFixture(mockActionFixtureSpec{
		ID:             "demo-action-install-updates",
		ResourceID:     hostID,
		CapabilityName: "install_os_updates",
		Reason:         "Patrol found security updates on a production host.",
		RequestedBy:    "pulse_patrol",
		PlannedAt:      now.Add(-12 * time.Minute),
		ExpiresAt:      now.Add(108 * time.Minute),
		ApprovalFloor:  unifiedresources.ApprovalAdmin,
		AutoClass:      unifiedresources.PolicyReasonCapabilityAutoElevated,
		CurrentState:   "8 security updates are available; package manager health is healthy.",
		IntendedChange: "Install the complete approved update set. A reboot, if needed, remains a separate action.",
		SafetyChecks:   []string{"Package manager health is known", "No package or reboot parameters were supplied"},
		Verification:   []string{"Refresh the update inventory", "Record remaining updates and reboot requirement"},
	})

	approved := approveMockAction(newMockActionFixture(mockActionFixtureSpec{
		ID:             "demo-action-restart-checkout",
		ResourceID:     vmID,
		CapabilityName: "restart",
		Reason:         "Recover checkout after three failed health checks.",
		RequestedBy:    "pulse_patrol",
		PlannedAt:      now.Add(-20 * time.Minute),
		ExpiresAt:      now.Add(40 * time.Minute),
		ApprovalFloor:  unifiedresources.ApprovalAdmin,
		AutoClass:      unifiedresources.PolicyReasonCapabilityAutoNever,
		CurrentState:   "The workload is running but its service health check is failing.",
		IntendedChange: "Restart the workload once and verify that service health recovers.",
		SafetyChecks:   []string{"Target identity is current", "Restart is bounded to one workload"},
		Verification:   []string{"Observe workload uptime reset", "Confirm the service health check"},
	}), "alex@example.com", unifiedresources.OutcomeApproved, now.Add(-8*time.Minute))

	executing := approveMockAction(newMockActionFixture(mockActionFixtureSpec{
		ID:             "demo-action-restart-worker",
		ResourceID:     containerID,
		CapabilityName: "restart_container",
		Reason:         "Restart the stuck notifications worker after queue processing stopped.",
		RequestedBy:    "pulse_patrol",
		PlannedAt:      now.Add(-9 * time.Minute),
		ExpiresAt:      now.Add(51 * time.Minute),
		ApprovalFloor:  unifiedresources.ApprovalAdmin,
		AutoClass:      unifiedresources.PolicyReasonCapabilityAutoNever,
		CurrentState:   "The container is running, but queue throughput has been zero for 15 minutes.",
		IntendedChange: "Restart the container and wait for a durable agent receipt.",
		SafetyChecks:   []string{"Container identity is current", "No image or configuration change is included"},
		Verification:   []string{"Confirm a new container start time", "Observe queue throughput"},
	}), "sam@example.com", unifiedresources.OutcomeApproved, now.Add(-5*time.Minute))
	executing.Audit.State = unifiedresources.ActionStateExecuting
	executing.Audit.UpdatedAt = now.Add(-3 * time.Minute)
	executing.Events = append(executing.Events, mockActionTransition(executing.Audit, unifiedresources.ActionStateExecuting, executing.Audit.UpdatedAt, "Pulse recorded dispatch before sending the action."))
	executing = mustNormalizeMockActionFixture(executing)

	completed := newMockActionFixture(mockActionFixtureSpec{
		ID:             "demo-action-clean-cache",
		ResourceID:     hostID,
		CapabilityName: "clean_package_cache",
		Reason:         "Reclaim downloaded package data after the root filesystem crossed its warning threshold.",
		RequestedBy:    "pulse_patrol",
		PlannedAt:      now.Add(-2 * time.Hour),
		ExpiresAt:      now.Add(-90 * time.Minute),
		ApprovalFloor:  unifiedresources.ApprovalNone,
		AutoClass:      unifiedresources.PolicyReasonCapabilityAutoLowRisk,
		CurrentState:   "Downloaded package data is using 100 MB on the pressured filesystem.",
		IntendedChange: "Clear downloaded package data only; installed packages and configuration remain unchanged.",
		SafetyChecks:   []string{"The pressured filesystem is the package-cache filesystem", "The action accepts no path"},
		Verification:   []string{"Measure package-cache bytes again", "Request a fresh storage scan"},
	})
	completedAt := now.Add(-112 * time.Minute)
	completed.Audit.State = unifiedresources.ActionStateCompleted
	completed.Audit.UpdatedAt = completedAt
	completed.Audit.Result = &unifiedresources.ExecutionResult{ActionResultV2: &unifiedresources.ActionResultV2{
		Version:   unifiedresources.ActionResultV2Version,
		Execution: unifiedresources.ActionExecutionTruth{Status: unifiedresources.ActionExecutionSucceeded, Summary: "APT package cache: phase=complete; 104857600 bytes before, 25165824 bytes after, 79691776 bytes reclaimed; rollback available: false; rescan required: true"},
		Verification: unifiedresources.ActionVerificationTruth{
			Status:        unifiedresources.ActionVerificationConfirmed,
			EvidenceClass: unifiedresources.ActionEvidenceAgentAttested,
			Summary:       "The executing agent measured the cache after cleanup.",
			Evidence: []unifiedresources.ActionEvidence{{
				Version: unifiedresources.ActionEvidenceVersion, ID: "demo-evidence-clean-cache", ObserverID: hostID,
				ObserverKind: "agent", ObserverTrustDomain: "agent:" + hostID, ExecutorTrustDomain: "agent:" + hostID,
				Method: "typed_read_after_write", SubjectID: hostID, ObservedAt: completedAt.Add(-10 * time.Second), ReceivedAt: completedAt,
				Summary: "Package-cache usage fell from 100 MB to 24 MB.",
			}},
		},
		Compensation: unifiedresources.ActionCompensationTruth{Support: unifiedresources.ActionCompensationUnavailable, Status: unifiedresources.ActionCompensationNotAvailable, Summary: "Downloaded package data cannot be restored; a fresh scan is required."},
	}}
	completed.Events = append(completed.Events,
		mockActionTransition(completed.Audit, unifiedresources.ActionStateExecuting, completedAt.Add(-35*time.Second), "Policy-authorized action execution started."),
		mockActionTransition(completed.Audit, unifiedresources.ActionStateCompleted, completedAt, "Action execution and verification were recorded."),
	)
	completed = mustNormalizeMockActionFixture(completed)

	rejected := approveMockAction(newMockActionFixture(mockActionFixtureSpec{
		ID:             "demo-action-rejected-restart",
		ResourceID:     vmID,
		CapabilityName: "restart",
		Reason:         "Restart a reporting service after elevated response times.",
		RequestedBy:    "pulse_patrol",
		PlannedAt:      now.Add(-5 * time.Hour),
		ExpiresAt:      now.Add(-4 * time.Hour),
		ApprovalFloor:  unifiedresources.ApprovalAdmin,
		AutoClass:      unifiedresources.PolicyReasonCapabilityAutoNever,
		CurrentState:   "Response time is elevated, but requests are still succeeding.",
		IntendedChange: "Restart the workload once and verify its service health.",
		SafetyChecks:   []string{"No outage is currently confirmed", "Restart affects one workload"},
		Verification:   []string{"Confirm the service returns healthy", "Compare response time after restart"},
	}), "morgan@example.com", unifiedresources.OutcomeRejected, now.Add(-4*time.Hour-48*time.Minute))

	failed := approveMockAction(newMockActionFixture(mockActionFixtureSpec{
		ID:             "demo-action-failed-restart",
		ResourceID:     containerID,
		CapabilityName: "restart_container",
		Reason:         "Recover the inventory worker after repeated liveness failures.",
		RequestedBy:    "pulse_patrol",
		PlannedAt:      now.Add(-26 * time.Hour),
		ExpiresAt:      now.Add(-25 * time.Hour),
		ApprovalFloor:  unifiedresources.ApprovalAdmin,
		AutoClass:      unifiedresources.PolicyReasonCapabilityAutoNever,
		CurrentState:   "The container is running but its liveness check is failing.",
		IntendedChange: "Restart the container once and verify that liveness recovers.",
		SafetyChecks:   []string{"Container identity is current", "Restart is bounded to one container"},
		Verification:   []string{"Confirm a new start time", "Observe the liveness check"},
	}), "alex@example.com", unifiedresources.OutcomeApproved, now.Add(-25*time.Hour-55*time.Minute))
	failedAt := now.Add(-25*time.Hour - 52*time.Minute)
	failed.Audit.State = unifiedresources.ActionStateFailed
	failed.Audit.UpdatedAt = failedAt
	failed.Audit.Result = &unifiedresources.ExecutionResult{ActionResultV2: &unifiedresources.ActionResultV2{
		Version:      unifiedresources.ActionResultV2Version,
		Execution:    unifiedresources.ActionExecutionTruth{Status: unifiedresources.ActionExecutionFailed, ReasonCode: "agent_unreachable", Summary: "The target agent was unreachable before the mutation began."},
		Verification: unifiedresources.ActionVerificationTruth{Status: unifiedresources.ActionVerificationNotAttempted, EvidenceClass: unifiedresources.ActionEvidenceNone},
		Compensation: unifiedresources.ActionCompensationTruth{Support: unifiedresources.ActionCompensationUnavailable, Status: unifiedresources.ActionCompensationNotAvailable, Summary: "No recovery action was needed because the mutation did not start."},
	}}
	failed.Events = append(failed.Events,
		mockActionTransition(failed.Audit, unifiedresources.ActionStateExecuting, failedAt.Add(-20*time.Second), "Pulse recorded dispatch before contacting the agent."),
		mockActionTransition(failed.Audit, unifiedresources.ActionStateFailed, failedAt, "The action did not run because the target agent was unreachable."),
	)
	failed = mustNormalizeMockActionFixture(failed)

	fixtures := []ActionFixture{pending, approved, executing, completed, rejected, failed}
	sort.SliceStable(fixtures, func(i, j int) bool { return fixtures[i].Audit.UpdatedAt.After(fixtures[j].Audit.UpdatedAt) })
	return fixtures
}

type mockActionFixtureSpec struct {
	ID             string
	ResourceID     string
	CapabilityName string
	Reason         string
	RequestedBy    string
	PlannedAt      time.Time
	ExpiresAt      time.Time
	ApprovalFloor  unifiedresources.ActionApprovalLevel
	AutoClass      unifiedresources.ActionPolicyReasonCode
	CurrentState   string
	IntendedChange string
	SafetyChecks   []string
	Verification   []string
}

func newMockActionFixture(spec mockActionFixtureSpec) ActionFixture {
	requiresApproval := spec.ApprovalFloor == unifiedresources.ApprovalAdmin || spec.ApprovalFloor == unifiedresources.ApprovalMultiFactor
	requirement := unifiedresources.ApprovalRequirementForFloor(spec.ApprovalFloor)
	actor := unifiedresources.ActionActor{SubjectID: spec.RequestedBy, Kind: unifiedresources.ActionActorService, CredentialID: "service:mock-patrol", OrgID: "default"}
	scope := unifiedresources.ActionPolicyDecisionScope{OrgID: "default", ResourceID: spec.ResourceID, CapabilityName: spec.CapabilityName}
	authorities := []unifiedresources.ActionPolicyAuthorityFactor{
		{Kind: unifiedresources.ActionPolicyAuthorityCapability, SourceID: "capability-registry:" + spec.CapabilityName, Revision: mockActionRevision("policy:sha256:", spec.ID+":capability"), Status: unifiedresources.ActionPolicyAuthorityConsulted, ApprovalFloor: spec.ApprovalFloor, ReasonCodes: []unifiedresources.ActionPolicyReasonCode{mockActionApprovalReason(spec.ApprovalFloor), spec.AutoClass}},
		{Kind: unifiedresources.ActionPolicyAuthorityTenant, SourceID: "patrol-tenant-policy", Revision: mockActionRevision("tenant-policy:sha256:", spec.ID+":tenant"), Status: unifiedresources.ActionPolicyAuthorityConsulted, ReasonCodes: []unifiedresources.ActionPolicyReasonCode{unifiedresources.PolicyReasonTenantModeAssisted, unifiedresources.PolicyReasonTenantFullLocked}},
		{Kind: unifiedresources.ActionPolicyAuthorityResource, SourceID: "resource-operator-policy:" + spec.ResourceID, Revision: mockActionRevision("resource-policy:sha256:", spec.ID+":resource"), Status: unifiedresources.ActionPolicyAuthorityConsulted, ReasonCodes: []unifiedresources.ActionPolicyReasonCode{unifiedresources.PolicyReasonResourceCapabilityAllow, unifiedresources.PolicyReasonResourceWindowOpen}},
	}
	policy, err := unifiedresources.BuildActionPolicyDecisionProvenance(spec.ID, scope, authorities, requirement, true, requiresApproval)
	if err != nil {
		panic(fmt.Sprintf("build mock action policy %q: %v", spec.ID, err))
	}
	state := unifiedresources.ActionStatePlanned
	if requiresApproval {
		state = unifiedresources.ActionStatePending
	}
	audit := unifiedresources.ActionAuditRecord{
		ID: spec.ID, CreatedAt: spec.PlannedAt, UpdatedAt: spec.PlannedAt, State: state,
		Request: unifiedresources.ActionRequest{RequestID: "request-" + spec.ID, ResourceID: spec.ResourceID, CapabilityName: spec.CapabilityName, Params: map[string]any{}, Reason: spec.Reason, RequestedBy: spec.RequestedBy, Actor: actor},
		Plan: unifiedresources.ActionPlan{
			ActionID: spec.ID, RequestID: "request-" + spec.ID, Allowed: true, RequiresApproval: requiresApproval,
			ApprovalPolicy: spec.ApprovalFloor, ApprovalRequirement: requirement, RollbackAvailable: false,
			Message: spec.IntendedChange, PlannedAt: spec.PlannedAt, ExpiresAt: spec.ExpiresAt,
			ResourceVersion: mockActionRevision("resource:sha256:", spec.ID+":state"), PolicyVersion: mockActionRevision("policy:sha256:", spec.ID+":plan"),
			PolicyDecision: policy, PlanHash: mockActionRevision("sha256:", spec.ID+":"+spec.ResourceID),
			Preflight: &unifiedresources.ActionPreflight{Target: spec.ResourceID, CurrentState: spec.CurrentState, IntendedChange: spec.IntendedChange, DryRunAvailable: false, DryRunSummary: "This capability has no provider dry run.", SafetyChecks: spec.SafetyChecks, VerificationSteps: spec.Verification, GeneratedAt: spec.PlannedAt},
		},
		Origin: &unifiedresources.ActionOrigin{Surface: "patrol", FindingID: "finding-" + spec.ID, InvestigationID: "investigation-demo-actions"},
	}
	events := []unifiedresources.ActionLifecycleEvent{mockActionTransition(audit, unifiedresources.ActionStatePlanned, spec.PlannedAt, "Action plan created from a Patrol finding.")}
	if requiresApproval {
		events = append(events, mockActionTransition(audit, unifiedresources.ActionStatePending, spec.PlannedAt, "Action is waiting for operator approval."))
	}
	return mustNormalizeMockActionFixture(ActionFixture{Audit: audit, Events: events})
}

func approveMockAction(fixture ActionFixture, subject string, outcome unifiedresources.ApprovalOutcome, decidedAt time.Time) ActionFixture {
	actor := unifiedresources.ActionActor{SubjectID: subject, Kind: unifiedresources.ActionActorUser, CredentialID: "session:mock-demo", OrgID: "default"}
	evidence := unifiedresources.ApprovalEvidence{Version: 1, Method: unifiedresources.MethodSession, Actor: actor, OrgID: "default", ActionID: fixture.Audit.ID, PlanHash: fixture.Audit.Plan.PlanHash, Outcome: outcome, IssuedAt: decidedAt, ExpiresAt: fixture.Audit.Plan.ExpiresAt}
	approval := unifiedresources.ActionApprovalRecord{Actor: subject, ActorBinding: actor, Method: unifiedresources.MethodSession, Timestamp: decidedAt, Outcome: outcome, Reason: mockActionDecisionReason(outcome), Evidence: &evidence}
	audit, event, err := unifiedresources.ApplyActionDecision(fixture.Audit, approval, decidedAt)
	if err != nil {
		panic(fmt.Sprintf("apply mock action decision %q: %v", fixture.Audit.ID, err))
	}
	fixture.Audit = audit
	fixture.Events = append(fixture.Events, event)
	return mustNormalizeMockActionFixture(fixture)
}

func mockActionTransition(audit unifiedresources.ActionAuditRecord, state unifiedresources.ActionState, at time.Time, message string) unifiedresources.ActionLifecycleEvent {
	return unifiedresources.ActionLifecycleEvent{ActionID: audit.ID, Timestamp: at, State: state, Kind: unifiedresources.ActionLifecycleEventTransition, Actor: audit.Request.RequestedBy, Message: message}
}

func mustNormalizeMockActionFixture(fixture ActionFixture) ActionFixture {
	audit, err := unifiedresources.NormalizeActionAuditRecord(fixture.Audit)
	if err != nil {
		panic(fmt.Sprintf("normalize mock action %q: %v", fixture.Audit.ID, err))
	}
	if fixture.Audit.Request.Params != nil {
		audit.Request.Params = make(map[string]any, len(fixture.Audit.Request.Params))
		for key, value := range fixture.Audit.Request.Params {
			audit.Request.Params[key] = value
		}
	}
	events := make([]unifiedresources.ActionLifecycleEvent, len(fixture.Events))
	for index, event := range fixture.Events {
		events[index], err = unifiedresources.NormalizeActionLifecycleEvent(event)
		if err != nil {
			panic(fmt.Sprintf("normalize mock action event %q: %v", fixture.Audit.ID, err))
		}
	}
	return ActionFixture{Audit: audit, Events: events}
}

func cloneActionFixtures(fixtures []ActionFixture) []ActionFixture {
	if fixtures == nil {
		return nil
	}
	cloned := make([]ActionFixture, len(fixtures))
	for index, fixture := range fixtures {
		cloned[index] = mustNormalizeMockActionFixture(fixture)
	}
	return cloned
}

func mockActionResourceID(resources []unifiedresources.Resource, resourceTypes ...string) string {
	for _, resourceType := range resourceTypes {
		for _, resource := range resources {
			if string(resource.Type) == resourceType && strings.TrimSpace(resource.ID) != "" {
				return resource.ID
			}
		}
	}
	return "resource:demo-unavailable"
}

func mockActionApprovalReason(floor unifiedresources.ActionApprovalLevel) unifiedresources.ActionPolicyReasonCode {
	switch floor {
	case unifiedresources.ApprovalNone:
		return unifiedresources.PolicyReasonCapabilityApprovalNone
	case unifiedresources.ApprovalMultiFactor:
		return unifiedresources.PolicyReasonCapabilityApprovalMFA
	case unifiedresources.ApprovalDryRun:
		return unifiedresources.PolicyReasonCapabilityDryRun
	default:
		return unifiedresources.PolicyReasonCapabilityApprovalAdmin
	}
}

func mockActionDecisionReason(outcome unifiedresources.ApprovalOutcome) string {
	if outcome == unifiedresources.OutcomeRejected {
		return "The service is degraded but still available; investigate before restarting."
	}
	return "Approved after reviewing the target, scope, and verification plan."
}

func mockActionRevision(prefix, seed string) string {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))
	if prefix == "sha256:" {
		return prefix + digest
	}
	return prefix + digest[:24]
}
