package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type barrierPatrolExecutor struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (e *barrierPatrolExecutor) ExecuteAction(_ context.Context, _ unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	if e.calls.Add(1) == 1 {
		close(e.entered)
	}
	<-e.release
	return &unified.ExecutionResult{Success: true}, nil
}

func configurePatrolAutoAuthorization(t *testing.T, h *ResourceHandlers) {
	t.Helper()
	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{CanonicalID: "vm:42", AutoRemediationPolicy: unified.AutoRemediationPolicy{Enabled: true, CapabilityNames: []string{"restart"}}, SetAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
}

func TestPatrolActionBrokerBarrierReplayAdmitsExecutorExactlyOnce(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	executor := &barrierPatrolExecutor{entered: make(chan struct{}), release: make(chan struct{})}
	h.SetActionExecutor(executor)
	configurePatrolAutoAuthorization(t, h)
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "assisted"}, nil
	})
	first := make(chan error, 1)
	go func() { _, err := broker.Submit(context.Background(), patrolTestProposal()); first <- err }()
	<-executor.entered
	secondDisposition, secondErr := broker.Submit(context.Background(), patrolTestProposal())
	if secondErr != nil {
		t.Fatalf("second Submit: %v", secondErr)
	}
	if secondDisposition.State != string(unified.ActionStateExecuting) {
		t.Fatalf("second state=%q", secondDisposition.State)
	}
	close(executor.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if executor.calls.Load() != 1 {
		t.Fatalf("executor calls=%d, want 1", executor.calls.Load())
	}
}

func TestPatrolActionBrokerReplayDuringExecutionReturnsCurrentDisposition(t *testing.T) {
	TestPatrolActionBrokerBarrierReplayAdmitsExecutorExactlyOnce(t)
}

func TestPatrolActionBrokerTerminalReplayPreservesAuditAndEvents(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	configurePatrolAutoAuthorization(t, h)
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "assisted"}, nil
	})
	first, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	store, _ := h.getStore("default")
	before, err := store.GetActionLifecycleEvents(first.ActionID, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.GetActionLifecycleEvents(first.ActionID, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if second.State != string(unified.ActionStateCompleted) || len(after) != len(before) || executor.calls != 1 {
		t.Fatalf("second=%#v events=%d/%d calls=%d", second, len(before), len(after), executor.calls)
	}
}

func TestPatrolActionBrokerSnapshotsTenantResourceAndCapabilityPolicyAtPlanTime(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	store, err := h.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	state := unified.ResourceOperatorState{CanonicalID: "vm:42", NeverAutoRemediate: true, AutoRemediationPolicy: unified.AutoRemediationPolicy{Enabled: true, CapabilityNames: []string{"restart"}}, SetAt: time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)}
	if err := store.SetResourceOperatorState(state); err != nil {
		t.Fatal(err)
	}
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "monitor", EmergencyStop: true}, nil
	})
	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("record: found=%v err=%v", found, err)
	}
	provenance := record.Plan.PolicyDecision
	if provenance.Version != unified.ActionPolicyDecisionVersion || len(provenance.Authorities) != 3 {
		t.Fatalf("provenance=%#v", provenance)
	}
	if provenance.Authorities[0].Kind != unified.ActionPolicyAuthorityCapability || provenance.Authorities[1].Kind != unified.ActionPolicyAuthorityTenant || provenance.Authorities[2].Kind != unified.ActionPolicyAuthorityResource {
		t.Fatalf("authority order=%#v", provenance.Authorities)
	}
	if !containsPolicyReason(provenance.Authorities[1].ReasonCodes, unified.PolicyReasonTenantEmergencyStop) || !containsPolicyReason(provenance.Authorities[1].ReasonCodes, unified.PolicyReasonTenantModeMonitor) || !containsPolicyReason(provenance.Authorities[2].ReasonCodes, unified.PolicyReasonResourceNeverAuto) {
		t.Fatalf("policy reasons=%#v", provenance.Authorities)
	}
	if executor.calls != 0 {
		t.Fatalf("descriptive provenance authorized dispatch: calls=%d", executor.calls)
	}
}

func containsPolicyReason(reasons []unified.ActionPolicyReasonCode, target unified.ActionPolicyReasonCode) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}

func TestPatrolActionBrokerConflictingOriginReplayFailsWithoutDispatch(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)
	if _, err := broker.Submit(context.Background(), patrolTestProposal()); err != nil {
		t.Fatal(err)
	}
	conflict := patrolTestProposal()
	conflict.FindingID = "finding-2"
	conflict.InvestigationID = "inv-2"
	if _, err := broker.Submit(context.Background(), conflict); !errors.Is(err, unified.ErrActionIdentityConflict) {
		t.Fatalf("error=%v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls=%d", executor.calls)
	}
}

func newPatrolBrokerTestHandlers(t *testing.T, minimumApproval unified.ActionApprovalLevel) (*ResourceHandlers, *stubActionExecutor) {
	return newPatrolBrokerTestHandlersWithEligibility(t, minimumApproval, unified.AutoAuthorizeLowRisk)
}

func newPatrolBrokerTestHandlersWithEligibility(t *testing.T, minimumApproval unified.ActionApprovalLevel, eligibility unified.ActionAutoAuthorizationClass) (*ResourceHandlers, *stubActionExecutor) {
	t.Helper()
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
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
						MinimumApprovalLevel: minimumApproval,
						AutoAuthorization:    eligibility,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
					{
						Name:                 "join_cluster",
						Type:                 unified.CapabilityTypeCommon,
						Description:          "Join an authenticated cluster",
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.join",
						Params: []unified.CapabilityParam{
							{Name: "join_token", Type: "string", Required: true, IsSensitive: true},
						},
					},
				},
			},
		},
	})
	executor := &stubActionExecutor{result: &unified.ExecutionResult{Success: true}}
	h.SetActionExecutor(executor)
	return h, executor
}

func patrolTestProposal() aicontracts.ActionProposal {
	return aicontracts.ActionProposal{
		ProposalID:      "prop-1",
		FindingID:       "finding-1",
		InvestigationID: "inv-1",
		ResourceID:      "vm:42",
		CapabilityName:  "restart",
		Params:          map[string]any{"mode": "graceful"},
		Reason:          "Recover after confirmed outage",
	}
}

func TestPatrolActionBrokerSubmitPlansThroughCanonicalLifecycle(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	proposal := patrolTestProposal()
	proposal.EvidenceIDs = []string{" evidence-2 ", "evidence-1", "evidence-1", ""}
	disposition, err := broker.Submit(context.Background(), proposal)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.ActionID == "" {
		t.Fatal("disposition has no action id")
	}
	if disposition.State != string(unified.ActionStatePending) {
		t.Fatalf("state = %q, want pending_approval", disposition.State)
	}
	if !disposition.Plan.RequiresApproval || disposition.Plan.ApprovalPolicy != string(unified.ApprovalAdmin) {
		t.Fatalf("plan projection = %#v", disposition.Plan)
	}
	if executor.calls != 0 {
		t.Fatalf("submit without core policy must not execute, executor calls = %d", executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	record, ok, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !ok {
		t.Fatalf("GetActionAudit: ok=%v err=%v", ok, err)
	}
	if record.Request.RequestedBy != patrolActionBrokerActor {
		t.Fatalf("requestedBy = %q, want %q", record.Request.RequestedBy, patrolActionBrokerActor)
	}
	if record.Origin == nil ||
		record.Origin.Surface != patrolActionOriginSurface ||
		record.Origin.FindingID != "finding-1" ||
		record.Origin.InvestigationID != "inv-1" ||
		record.Origin.ProposalID != "prop-1" ||
		fmt.Sprint(record.Origin.EvidenceIDs) != "[evidence-1 evidence-2]" {
		t.Fatalf("audit origin = %#v", record.Origin)
	}
}

func TestPatrolActionBrokerApprovalNoneDoesNotExecuteWithoutCorePolicy(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalNone)
	broker := NewPatrolActionBroker("default", h)

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStatePlanned) {
		t.Fatalf("state = %q, want planned", disposition.State)
	}
	if disposition.Plan.RequiresApproval {
		t.Fatal("ApprovalNone capability should not require approval")
	}
	if executor.calls != 0 {
		t.Fatalf("submit without core policy must not execute, executor calls = %d", executor.calls)
	}
}

func TestPatrolActionBrokerRejectsSensitiveParams(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	proposal := patrolTestProposal()
	proposal.CapabilityName = "join_cluster"
	proposal.Params = map[string]any{"join_token": "sekret-token-value"}
	_, err := broker.Submit(context.Background(), proposal)
	if !errors.Is(err, aicontracts.ErrSensitiveParamsRequireOperator) {
		t.Fatalf("error = %v, want ErrSensitiveParamsRequireOperator", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.calls)
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("sensitive-param proposals must not persist audits, got %d", len(audits))
	}
}

func TestPatrolActionBrokerCapabilitiesCatalog(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	catalog, err := broker.Capabilities(context.Background(), "vm:42")
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if catalog.ResourceID != "vm:42" || len(catalog.Capabilities) != 2 {
		t.Fatalf("catalog = %#v", catalog)
	}
	byName := map[string]aicontracts.ActionCapabilityInfo{}
	for _, capability := range catalog.Capabilities {
		byName[capability.Name] = capability
	}
	restart := byName["restart"]
	if restart.MinimumApprovalLevel != string(unified.ApprovalAdmin) || restart.AutoAuthorization != string(unified.AutoAuthorizeLowRisk) || len(restart.Params) != 1 || restart.Params[0].Sensitive {
		t.Fatalf("restart capability = %#v", restart)
	}
	join := byName["join_cluster"]
	if len(join.Params) != 1 || !join.Params[0].Sensitive {
		t.Fatalf("join_cluster capability must mark its token sensitive: %#v", join)
	}

	if _, err := broker.Capabilities(context.Background(), "vm:404"); err == nil {
		t.Fatal("unknown resource must error")
	}
}

func TestPatrolActionBrokerAutoAuthorizesOnlyExplicitScopedEligibleCapability(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "vm:42",
		AutoRemediationPolicy: unified.AutoRemediationPolicy{
			Enabled:         true,
			CapabilityNames: []string{"restart"},
		},
		SetAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "assisted"}, nil
	})

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStateCompleted) {
		t.Fatalf("state = %q, want completed", disposition.State)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.calls)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("GetActionAudit: found=%v err=%v", found, err)
	}
	if len(audit.Approvals) != 1 || audit.Approvals[0].Actor != patrolActionPolicyActor || audit.Approvals[0].Method != unified.MethodPolicy {
		t.Fatalf("policy approval = %#v", audit.Approvals)
	}

	// An idempotent proposal replay returns the terminal audit and cannot
	// rewind or execute the action a second time.
	replayed, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil || replayed.State != string(unified.ActionStateCompleted) {
		t.Fatalf("replay disposition=%#v err=%v", replayed, err)
	}
	if executor.calls != 1 {
		t.Fatalf("replay executor calls = %d, want 1", executor.calls)
	}
}

func TestPatrolActionBrokerPolicyScopeAndWindowFailClosed(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "vm:42",
		AutoRemediationPolicy: unified.AutoRemediationPolicy{
			Enabled:         true,
			CapabilityNames: []string{"restart"},
			Window: &unified.AutoRemediationWindow{
				Timezone: "UTC", StartMinute: 60, EndMinute: 120,
			},
		},
		SetAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "assisted"}, nil
	}).(*patrolActionBroker)
	broker.now = func() time.Time { return time.Date(2026, 7, 10, 3, 0, 0, 0, time.UTC) }

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStatePending) || executor.calls != 0 {
		t.Fatalf("outside-window disposition=%#v calls=%d", disposition, executor.calls)
	}
}

func TestPatrolActionBrokerAutonomyModeIsAnUpperBound(t *testing.T) {
	tests := []struct {
		name        string
		eligibility unified.ActionAutoAuthorizationClass
		snapshot    PatrolActionPolicySnapshot
		wantCalls   int
	}{
		{name: "monitor denies low risk", eligibility: unified.AutoAuthorizeLowRisk, snapshot: PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "monitor"}},
		{name: "approval denies low risk", eligibility: unified.AutoAuthorizeLowRisk, snapshot: PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "approval"}},
		{name: "assisted denies elevated", eligibility: unified.AutoAuthorizeElevated, snapshot: PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "assisted"}},
		{name: "locked full denies elevated", eligibility: unified.AutoAuthorizeElevated, snapshot: PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "full"}},
		{name: "unlocked full admits elevated", eligibility: unified.AutoAuthorizeElevated, snapshot: PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "full", FullModeUnlocked: true}, wantCalls: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, executor := newPatrolBrokerTestHandlersWithEligibility(t, unified.ApprovalAdmin, tc.eligibility)
			store, err := h.getStore("default")
			if err != nil {
				t.Fatalf("getStore: %v", err)
			}
			if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
				CanonicalID: "vm:42",
				AutoRemediationPolicy: unified.AutoRemediationPolicy{
					Enabled:         true,
					CapabilityNames: []string{"restart"},
				},
				SetAt: time.Now().UTC(),
			}); err != nil {
				t.Fatalf("SetResourceOperatorState: %v", err)
			}
			broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
				return tc.snapshot, nil
			})
			disposition, err := broker.Submit(context.Background(), patrolTestProposal())
			if err != nil {
				t.Fatalf("Submit: %v", err)
			}
			if executor.calls != tc.wantCalls {
				t.Fatalf("executor calls = %d, want %d (disposition=%#v)", executor.calls, tc.wantCalls, disposition)
			}
			if tc.wantCalls == 0 && disposition.State != string(unified.ActionStatePending) {
				t.Fatalf("denied state = %q, want pending_approval", disposition.State)
			}
			if tc.wantCalls == 1 && disposition.State != string(unified.ActionStateCompleted) {
				t.Fatalf("authorized state = %q, want completed", disposition.State)
			}
		})
	}
}

func TestPatrolActionBrokerDispatchTimePolicyRevocation(t *testing.T) {
	h, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "vm:42",
		AutoRemediationPolicy: unified.AutoRemediationPolicy{
			Enabled:         true,
			CapabilityNames: []string{"restart"},
		},
		SetAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}
	policyReads := 0
	broker := NewPatrolActionBroker("default", h, func(context.Context, string) (PatrolActionPolicySnapshot, error) {
		policyReads++
		if policyReads == 1 {
			return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "assisted"}, nil
		}
		return PatrolActionPolicySnapshot{EffectiveAutonomyLevel: "approval"}, nil
	})

	disposition, err := broker.Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if disposition.State != string(unified.ActionStateFailed) || executor.calls != 0 {
		t.Fatalf("revoked disposition=%#v calls=%d", disposition, executor.calls)
	}
	record, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found {
		t.Fatalf("GetActionAudit: found=%v err=%v", found, err)
	}
	if len(record.Approvals) != 0 || record.Result == nil || !strings.HasPrefix(record.Result.ErrorMessage, "policy_authorization_revoked:") {
		t.Fatalf("revoked policy left reusable authority: approvals=%#v result=%#v", record.Approvals, record.Result)
	}
}

func TestPatrolActionBrokerRequiresCorrelationIdentity(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	broker := NewPatrolActionBroker("default", h)

	missingFinding := patrolTestProposal()
	missingFinding.FindingID = " "
	if _, err := broker.Submit(context.Background(), missingFinding); err == nil {
		t.Fatal("proposal without finding id must be refused")
	}

	missingInvestigation := patrolTestProposal()
	missingInvestigation.InvestigationID = ""
	if _, err := broker.Submit(context.Background(), missingInvestigation); err == nil {
		t.Fatal("proposal without investigation id must be refused")
	}

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("identity-less proposals must not persist audits, got %d", len(audits))
	}
}

func TestPatrolActionBrokerSubmitPublishesOrgScopedTransition(t *testing.T) {
	h, _ := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	var gotOrg string
	var gotState unified.ActionState
	var gotOrigin *unified.ActionOrigin
	h.SetActionTransitionPublisher(func(orgID string, record unified.ActionAuditRecord) {
		gotOrg = orgID
		gotState = record.State
		gotOrigin = record.Origin
	})
	broker := NewPatrolActionBroker("default", h)

	if _, err := broker.Submit(context.Background(), patrolTestProposal()); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotOrg != "default" {
		t.Fatalf("transition org = %q, want default", gotOrg)
	}
	if gotState != unified.ActionStatePending {
		t.Fatalf("transition state = %q, want pending_approval", gotState)
	}
	if gotOrigin == nil || gotOrigin.FindingID != "finding-1" {
		t.Fatalf("transition origin = %#v", gotOrigin)
	}
}

func TestPatrolTypedActionJourneyDetectPlanApproveExecuteVerifyAndReconcile(t *testing.T) {
	resources, executor := newPatrolBrokerTestHandlers(t, unified.ApprovalAdmin)
	executor.result = &unified.ExecutionResult{
		Success: true,
		Output:  "restart dispatched",
		Verification: &unified.ActionVerificationResult{
			Ran: true, Success: true, RanAt: time.Now().UTC(), Note: "workload health confirmed",
		},
	}
	aiHandler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	pushes := make(chan relay.PushNotificationPayload, 1)
	patrol.SetPushNotifyCallback(func(payload relay.PushNotificationPayload) { pushes <- payload })
	finding := addPatrolFindingForResource(t, patrol, "finding-1", time.Now().UTC(), "vm:42", "Unhealthy workload")
	investigations := newTestInvestigationStore()
	investigation := investigations.Create(finding.ID, "session-1")
	if investigation.ID != "inv-1" {
		t.Fatalf("investigation id = %q, journey proposal expects inv-1", investigation.ID)
	}
	aiHandler.investigationStores = map[string]aicontracts.InvestigationStore{"default": investigations}
	aiHandler.SetResourceStoreProvider(resources.getStore)
	resources.SetActionTransitionPublisher(aiHandler.ReconcilePatrolActionTransition)

	disposition, err := NewPatrolActionBroker("default", resources).Submit(context.Background(), patrolTestProposal())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	planned := investigations.Get(investigation.ID)
	if planned.Action == nil || planned.Action.ActionID != disposition.ActionID || planned.Action.State != string(unified.ActionStatePending) {
		t.Fatalf("planned investigation action = %#v", planned.Action)
	}
	if planned.Outcome != aicontracts.OutcomeFixQueued {
		t.Fatalf("planned outcome = %q", planned.Outcome)
	}

	decisionRec := httptest.NewRecorder()
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/decision", bytes.NewBufferString(`{"outcome":"approved","reason":"maintenance window"}`))
	decisionReq.SetPathValue("id", disposition.ActionID)
	decisionReq = decisionReq.WithContext(auth.WithUser(decisionReq.Context(), "operator@example.com"))
	resources.HandleDecideAction(decisionRec, actionHandlerTestRequest(decisionReq, ""))
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d body=%s", decisionRec.Code, decisionRec.Body.String())
	}

	executionRec := httptest.NewRecorder()
	executionReq := httptest.NewRequest(http.MethodPost, "/api/actions/"+disposition.ActionID+"/execute", bytes.NewBufferString(`{"reason":"approved maintenance window"}`))
	executionReq.SetPathValue("id", disposition.ActionID)
	executionReq = executionReq.WithContext(auth.WithUser(executionReq.Context(), "operator@example.com"))
	resources.HandleExecuteAction(executionRec, actionHandlerTestRequest(executionReq, ""))
	if executionRec.Code != http.StatusOK {
		t.Fatalf("execution status = %d body=%s", executionRec.Code, executionRec.Body.String())
	}

	completed := investigations.Get(investigation.ID)
	if completed.Action == nil || completed.Action.State != string(unified.ActionStateCompleted) {
		t.Fatalf("completed investigation action = %#v", completed.Action)
	}
	if completed.Outcome != aicontracts.OutcomeFixVerificationUnknown {
		t.Fatalf("completed outcome = %q, want verification unknown", completed.Outcome)
	}
	updatedFinding := patrol.GetFindings().Get(finding.ID)
	if updatedFinding == nil || updatedFinding.InvestigationOutcome != string(aicontracts.OutcomeFixVerificationUnknown) || updatedFinding.ResolvedAt != nil {
		t.Fatalf("reconciled finding = %#v", updatedFinding)
	}
	store, err := resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	audit, found, err := store.GetActionAudit(disposition.ActionID)
	if err != nil || !found || audit.VerificationOutcome.Status != unified.VerificationVerified {
		t.Fatalf("terminal audit: found=%v err=%v audit=%#v", found, err, audit)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want exactly one", executor.calls)
	}
	select {
	case push := <-pushes:
		if push.ActionType != relay.PushActionViewFixResult || push.ActionID != finding.ID || push.Body != "Action completed; verification was inconclusive" {
			t.Fatalf("terminal push = %#v", push)
		}
	default:
		t.Fatal("agent-attested lifecycle did not publish an honest terminal mobile notification")
	}
}
