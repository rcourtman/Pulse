package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// patrolActionBrokerActor is the fixed requestedBy identity for every
// Patrol-proposed action. Enterprise code cannot influence it.
const patrolActionBrokerActor = "pulse_patrol"

// patrolActionOriginSurface tags Patrol-proposed action audits so later
// decisions and terminal outcomes can be reconciled onto the finding.
const patrolActionOriginSurface = "patrol"

const patrolActionPolicyActor = "pulse_patrol_policy"

// PatrolActionPolicySnapshot is core-owned authorization posture. Enterprise
// code and model output cannot populate it.
type PatrolActionPolicySnapshot struct {
	EffectiveAutonomyLevel string
	FullModeUnlocked       bool
	EmergencyStop          bool
	PolicyVersion          string
}

type PatrolActionPolicyProvider func(ctx context.Context, orgID string) (PatrolActionPolicySnapshot, error)

type patrolActionPolicyInputs struct {
	snapshot      PatrolActionPolicySnapshot
	tenantLoaded  bool
	tenantErr     error
	capability    *unified.ResourceCapability
	capabilityErr error
	resourceState unified.ResourceOperatorState
	resourceFound bool
	resourceErr   error
}

type patrolActionPolicyEvaluation struct {
	factors        []unified.ActionPolicyAuthorityFactor
	autoAuthorized bool
	reason         string
	err            error
}

// patrolActionBroker is the org-bound core adapter behind
// aicontracts.OrchestratorActionBroker. It routes every Patrol proposal
// through the shared action lifecycle service: identical registry lookup,
// capability validation, availability checks, plan hashing, and audit
// persistence as the REST plan endpoint. Core may authorize and execute an
// eligible proposal under stored tenant/resource policy; the enterprise
// investigation side stays a proposer, never a dispatcher.
type patrolActionBroker struct {
	orgID string
	// lifecycle resolves the shared service per call so late-bound
	// executor and publisher wiring on ResourceHandlers stays current.
	lifecycle func() *actionlifecycle.Service
	policy    PatrolActionPolicyProvider
	now       func() time.Time
}

// NewPatrolActionBroker builds the tenant-bound Patrol proposal broker over
// the shared action lifecycle service.
func NewPatrolActionBroker(orgID string, resources *ResourceHandlers, policy ...PatrolActionPolicyProvider) aicontracts.OrchestratorActionBroker {
	broker := &patrolActionBroker{
		orgID:     strings.TrimSpace(orgID),
		lifecycle: resources.ActionLifecycle,
	}
	if len(policy) > 0 {
		broker.policy = policy[0]
	}
	return broker
}

func (b *patrolActionBroker) Capabilities(ctx context.Context, resourceID string) (aicontracts.ActionCapabilityCatalog, error) {
	resourceID = unified.CanonicalResourceID(resourceID)
	capabilities, err := b.lifecycle().Capabilities(ctx, b.orgID, resourceID)
	if err != nil {
		return aicontracts.ActionCapabilityCatalog{}, err
	}
	catalog := aicontracts.ActionCapabilityCatalog{
		ResourceID:   resourceID,
		Capabilities: make([]aicontracts.ActionCapabilityInfo, 0, len(capabilities)),
	}
	for _, capability := range capabilities {
		info := aicontracts.ActionCapabilityInfo{
			Name:                 capability.Name,
			Description:          capability.Description,
			MinimumApprovalLevel: string(capability.MinimumApprovalLevel),
			AutoAuthorization:    string(unified.NormalizeActionAutoAuthorizationClass(capability.AutoAuthorization)),
			Platform:             capability.Platform,
			Params:               make([]aicontracts.ActionCapabilityParamInfo, 0, len(capability.Params)),
		}
		for _, param := range capability.Params {
			info.Params = append(info.Params, aicontracts.ActionCapabilityParamInfo{
				Name:        param.Name,
				Type:        param.Type,
				Required:    param.Required,
				Enum:        append([]string(nil), param.Enum...),
				Pattern:     param.Pattern,
				Description: param.Description,
				Sensitive:   param.IsSensitive,
			})
		}
		catalog.Capabilities = append(catalog.Capabilities, info)
	}
	return catalog, nil
}

func (b *patrolActionBroker) Submit(ctx context.Context, proposal aicontracts.ActionProposal) (aicontracts.ActionDisposition, error) {
	proposal.ProposalID = strings.TrimSpace(proposal.ProposalID)
	proposal.FindingID = strings.TrimSpace(proposal.FindingID)
	proposal.InvestigationID = strings.TrimSpace(proposal.InvestigationID)
	proposal.ResourceID = unified.CanonicalResourceID(proposal.ResourceID)
	proposal.CapabilityName = strings.TrimSpace(proposal.CapabilityName)
	proposal.Reason = strings.TrimSpace(proposal.Reason)
	if proposal.ResourceID == "" {
		return aicontracts.ActionDisposition{}, fmt.Errorf("action proposal requires a resource id")
	}
	if proposal.CapabilityName == "" {
		return aicontracts.ActionDisposition{}, fmt.Errorf("action proposal requires a capability name")
	}
	if proposal.Reason == "" {
		return aicontracts.ActionDisposition{}, fmt.Errorf("action proposal requires a reason")
	}
	// Finding and investigation identity are required before persistence:
	// without them the planned action cannot be deterministically
	// reconciled back onto its Patrol finding at decision or terminal
	// time, which would orphan a valid governed action.
	if proposal.FindingID == "" {
		return aicontracts.ActionDisposition{}, fmt.Errorf("action proposal requires a finding id")
	}
	if proposal.InvestigationID == "" {
		return aicontracts.ActionDisposition{}, fmt.Errorf("action proposal requires an investigation id")
	}

	if err := b.rejectSensitiveParams(ctx, proposal); err != nil {
		return aicontracts.ActionDisposition{}, err
	}
	policyFactors, planningAutoAuthorized := b.planPolicyFactors(ctx, proposal, b.currentTime())

	plan, err := b.lifecycle().PlanWithOptions(ctx, b.orgID, unified.ActionRequest{
		RequestID:      proposal.ProposalID,
		ResourceID:     proposal.ResourceID,
		CapabilityName: proposal.CapabilityName,
		Params:         proposal.Params,
		Reason:         proposal.Reason,
		RequestedBy:    patrolActionBrokerActor,
	}, actionlifecycle.PlanOptions{
		Actor: unified.ActionActor{
			SubjectID:    patrolActionBrokerActor,
			Kind:         unified.ActionActorService,
			CredentialID: "service:patrol-action-broker",
			OrgID:        b.orgID,
		},
		Origin: &unified.ActionOrigin{
			Surface:         patrolActionOriginSurface,
			FindingID:       proposal.FindingID,
			InvestigationID: proposal.InvestigationID,
			ProposalID:      proposal.ProposalID,
		},
		PolicyFactors: policyFactors,
	})
	if err != nil {
		return aicontracts.ActionDisposition{}, err
	}

	record, found, err := b.lifecycle().Get(b.orgID, plan.ActionID)
	if err != nil {
		return aicontracts.ActionDisposition{}, err
	}
	if !found {
		return aicontracts.ActionDisposition{}, fmt.Errorf("planned action %q was not persisted", plan.ActionID)
	}
	if record.State == unified.ActionStateCompleted || record.State == unified.ActionStateFailed || record.State == unified.ActionStateRejected {
		return dispositionFromRecord(record), nil
	}

	if planningAutoAuthorized {
		record, err = b.lifecycle().ExecuteUnderPolicy(ctx, b.orgID, plan.ActionID, patrolActionPolicyActor, func(ctx context.Context, current unified.ActionAuditRecord, now time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
			return b.policyAuthorizationLease(ctx, proposal, current, now)
		})
		if err != nil {
			if record.State == unified.ActionStateFailed {
				return dispositionFromRecord(record), nil
			}
			return dispositionFromRecord(record), err
		}
	}

	return dispositionFromRecord(record), nil
}

func (b *patrolActionBroker) planPolicyFactors(ctx context.Context, proposal aicontracts.ActionProposal, now time.Time) ([]unified.ActionPolicyAuthorityFactor, bool) {
	evaluation := evaluatePatrolActionPolicy(b.loadPatrolActionPolicyInputs(ctx, proposal), b.orgID, proposal, now)
	return evaluation.factors, evaluation.autoAuthorized
}

func (b *patrolActionBroker) loadPatrolActionPolicyInputs(ctx context.Context, proposal aicontracts.ActionProposal) patrolActionPolicyInputs {
	inputs := patrolActionPolicyInputs{}
	if b.policy != nil {
		inputs.snapshot, inputs.tenantErr = b.policy(ctx, b.orgID)
		inputs.tenantLoaded = inputs.tenantErr == nil
	}
	capabilities, err := b.lifecycle().Capabilities(ctx, b.orgID, proposal.ResourceID)
	if err != nil {
		inputs.capabilityErr = err
	} else {
		for i := range capabilities {
			if strings.TrimSpace(capabilities[i].Name) == proposal.CapabilityName {
				capability := capabilities[i]
				inputs.capability = &capability
				break
			}
		}
	}
	inputs.resourceState, inputs.resourceFound, inputs.resourceErr = b.lifecycle().ResourceOperatorState(b.orgID, proposal.ResourceID)
	if inputs.resourceFound {
		inputs.resourceState = unified.NormalizeResourceOperatorState(inputs.resourceState)
	}
	return inputs
}

func evaluatePatrolActionPolicy(inputs patrolActionPolicyInputs, orgID string, proposal aicontracts.ActionProposal, now time.Time) patrolActionPolicyEvaluation {
	scope := unified.ActionPolicyDecisionScope{OrgID: orgID, ResourceID: proposal.ResourceID, CapabilityName: proposal.CapabilityName}
	tenant := unified.ActionPolicyAuthorityFactor{
		Kind: unified.ActionPolicyAuthorityTenant, SourceID: "patrol-tenant-policy", Scope: scope,
		Status: unified.ActionPolicyAuthorityUnavailable, ReasonCodes: []unified.ActionPolicyReasonCode{unified.PolicyReasonTenantUnavailable},
	}
	if inputs.tenantLoaded {
		tenant.Status = unified.ActionPolicyAuthorityConsulted
		tenant.Revision = policySnapshotVersion(inputs.snapshot)
		tenant.ReasonCodes = tenantPolicyReasonCodes(inputs.snapshot)
	}
	resource := unified.ActionPolicyAuthorityFactor{
		Kind: unified.ActionPolicyAuthorityResource, SourceID: "resource-operator-policy:" + proposal.ResourceID, Scope: scope,
		Status: unified.ActionPolicyAuthorityUnavailable, ReasonCodes: []unified.ActionPolicyReasonCode{unified.PolicyReasonResourceUnavailable},
	}
	if inputs.resourceErr == nil {
		if !inputs.resourceFound {
			resource.Status = unified.ActionPolicyAuthorityNotFound
			resource.ReasonCodes = []unified.ActionPolicyReasonCode{unified.PolicyReasonResourceMissing}
		} else {
			resource.Status = unified.ActionPolicyAuthorityConsulted
			resource.Revision = resourcePolicyVersion(inputs.resourceState)
			resource.ReasonCodes = resourcePolicyReasonCodes(inputs.resourceState, proposal.CapabilityName, now)
		}
	}
	evaluation := patrolActionPolicyEvaluation{factors: []unified.ActionPolicyAuthorityFactor{tenant, resource}}
	if inputs.tenantErr != nil {
		evaluation.err = inputs.tenantErr
		return evaluation
	}
	if inputs.capabilityErr != nil {
		evaluation.err = inputs.capabilityErr
		return evaluation
	}
	if inputs.resourceErr != nil {
		evaluation.err = inputs.resourceErr
		return evaluation
	}
	if !inputs.tenantLoaded || inputs.capability == nil || !inputs.resourceFound || inputs.snapshot.EmergencyStop {
		return evaluation
	}
	capability := *inputs.capability
	class := unified.NormalizeActionAutoAuthorizationClass(capability.AutoAuthorization)
	eligible := class != unified.AutoAuthorizeNever && capability.MinimumApprovalLevel != unified.ApprovalDryRun && capability.MinimumApprovalLevel != unified.ApprovalMultiFactor
	switch strings.TrimSpace(inputs.snapshot.EffectiveAutonomyLevel) {
	case "assisted":
		eligible = eligible && class == unified.AutoAuthorizeLowRisk
	case "full":
		eligible = eligible && inputs.snapshot.FullModeUnlocked
	default:
		eligible = false
	}
	evaluation.autoAuthorized = eligible && inputs.resourceState.AllowsAutoRemediationAt(proposal.CapabilityName, now)
	if evaluation.autoAuthorized {
		evaluation.reason = fmt.Sprintf("Patrol %s policy authorized %s capability %s for resource %s", inputs.snapshot.EffectiveAutonomyLevel, class, proposal.CapabilityName, proposal.ResourceID)
	}
	return evaluation
}

func tenantPolicyReasonCodes(snapshot PatrolActionPolicySnapshot) []unified.ActionPolicyReasonCode {
	reasons := make([]unified.ActionPolicyReasonCode, 0, 3)
	if snapshot.EmergencyStop {
		reasons = append(reasons, unified.PolicyReasonTenantEmergencyStop)
	}
	switch strings.TrimSpace(snapshot.EffectiveAutonomyLevel) {
	case "monitor":
		reasons = append(reasons, unified.PolicyReasonTenantModeMonitor)
	case "assisted":
		reasons = append(reasons, unified.PolicyReasonTenantModeAssisted)
	case "full":
		reasons = append(reasons, unified.PolicyReasonTenantModeFull)
	default:
		reasons = append(reasons, unified.PolicyReasonTenantModeUnknown)
	}
	if snapshot.FullModeUnlocked {
		reasons = append(reasons, unified.PolicyReasonTenantFullUnlocked)
	} else {
		reasons = append(reasons, unified.PolicyReasonTenantFullLocked)
	}
	return reasons
}

func resourcePolicyReasonCodes(state unified.ResourceOperatorState, capabilityName string, now time.Time) []unified.ActionPolicyReasonCode {
	reasons := make([]unified.ActionPolicyReasonCode, 0, 3)
	if state.NeverAutoRemediate {
		reasons = append(reasons, unified.PolicyReasonResourceNeverAuto)
	}
	policy := unified.NormalizeAutoRemediationPolicy(state.AutoRemediationPolicy)
	allowed := false
	for _, name := range policy.CapabilityNames {
		if name == strings.TrimSpace(capabilityName) {
			allowed = policy.Enabled
			break
		}
	}
	if allowed {
		reasons = append(reasons, unified.PolicyReasonResourceCapabilityAllow)
	} else {
		reasons = append(reasons, unified.PolicyReasonResourceCapabilityDeny)
	}
	if policy.Window != nil && allowed {
		windowState := state
		windowState.NeverAutoRemediate = false
		if allowed && windowState.AllowsAutoRemediationAt(capabilityName, now) {
			reasons = append(reasons, unified.PolicyReasonResourceWindowOpen)
		} else {
			reasons = append(reasons, unified.PolicyReasonResourceWindowClosed)
		}
	}
	return reasons
}

func resourcePolicyVersion(state unified.ResourceOperatorState) string {
	payload, _ := json.Marshal(unified.NormalizeResourceOperatorState(state))
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("resource-policy:sha256:%x", sum[:12])
}

func (b *patrolActionBroker) policyAuthorizationLease(ctx context.Context, proposal aicontracts.ActionProposal, record unified.ActionAuditRecord, now time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
	inputs := b.loadPatrolActionPolicyInputs(ctx, proposal)
	evaluation := evaluatePatrolActionPolicy(inputs, b.orgID, proposal, now)
	if evaluation.err != nil {
		return unified.ActionPolicyAuthorizationLease{}, "", evaluation.err
	}
	if !evaluation.autoAuthorized {
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationRevoked
	}
	if inputs.capability == nil {
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationRevoked
	}
	if !inputs.resourceFound {
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationInvalid
	}
	capability := *inputs.capability
	state := inputs.resourceState
	tenantVersion := policySnapshotVersion(inputs.snapshot)
	expiresAt := record.Plan.ExpiresAt
	if expiresAt.IsZero() || expiresAt.After(now.Add(time.Minute)) {
		expiresAt = now.Add(time.Minute)
	}
	lease := unified.ActionPolicyAuthorizationLease{
		Version: 1, OrgID: b.orgID, ActionID: record.ID, ResourceID: proposal.ResourceID, CapabilityName: proposal.CapabilityName,
		PlanHash: record.Plan.PlanHash, CapabilityPolicyVersion: record.Plan.PolicyVersion,
		AutoAuthorization: unified.NormalizeActionAutoAuthorizationClass(capability.AutoAuthorization), ApprovalPolicy: capability.MinimumApprovalLevel,
		TenantPolicyVersion: tenantVersion, EffectiveAutonomyLevel: strings.TrimSpace(inputs.snapshot.EffectiveAutonomyLevel), LicenseAllowsAutoFix: true,
		FullModeUnlocked: inputs.snapshot.FullModeUnlocked, EmergencyStop: inputs.snapshot.EmergencyStop,
		ResourcePolicyVersion: resourcePolicyVersion(state), CapabilityNames: append([]string(nil), state.AutoRemediationPolicy.CapabilityNames...),
		Window: state.AutoRemediationPolicy.Window, NeverAutoRemediate: state.NeverAutoRemediate, IssuedAt: now.UTC(), ExpiresAt: expiresAt.UTC(),
	}
	lease.Digest = unified.ActionPolicyAuthorizationDigest(lease)
	return lease, evaluation.reason, nil
}

func policySnapshotVersion(snapshot PatrolActionPolicySnapshot) string {
	payload, _ := json.Marshal(snapshot)
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("tenant-policy:sha256:%x", sum[:12])
}

func dispositionFromRecord(record unified.ActionAuditRecord) aicontracts.ActionDisposition {
	canonical := unified.CanonicalActionResultV2(record)
	resultJSON, _ := json.Marshal(canonical)
	_, legacyVerification, _ := unified.ApplyActionResultV2(nil, canonical)
	return aicontracts.ActionDisposition{
		ActionID:           record.ID,
		State:              string(record.State),
		ActionResultV2:     resultJSON,
		VerificationStatus: string(legacyVerification.Status),
		Plan:               *approvalPlanRequestToInfo(&record.Plan),
	}
}

func (b *patrolActionBroker) autoAuthorizationDecision(ctx context.Context, proposal aicontracts.ActionProposal) (bool, string, error) {
	return b.autoAuthorizationDecisionAt(ctx, proposal, b.currentTime())
}

func (b *patrolActionBroker) autoAuthorizationDecisionAt(ctx context.Context, proposal aicontracts.ActionProposal, now time.Time) (bool, string, error) {
	evaluation := evaluatePatrolActionPolicy(b.loadPatrolActionPolicyInputs(ctx, proposal), b.orgID, proposal, now)
	return evaluation.autoAuthorized, evaluation.reason, evaluation.err
}

func (b *patrolActionBroker) currentTime() time.Time {
	if b.now != nil {
		return b.now().UTC()
	}
	return time.Now().UTC()
}

// rejectSensitiveParams fails a proposal that populates any parameter the
// capability declares sensitive. Secrets must come from an operator on the
// canonical surfaces, never from model output that would persist in
// investigation stores and action audit records.
func (b *patrolActionBroker) rejectSensitiveParams(ctx context.Context, proposal aicontracts.ActionProposal) error {
	if len(proposal.Params) == 0 {
		return nil
	}
	capabilities, err := b.lifecycle().Capabilities(ctx, b.orgID, proposal.ResourceID)
	if err != nil {
		return err
	}
	for _, capability := range capabilities {
		if !strings.EqualFold(strings.TrimSpace(capability.Name), proposal.CapabilityName) {
			continue
		}
		for _, param := range capability.Params {
			if !param.IsSensitive {
				continue
			}
			if value, ok := proposal.Params[param.Name]; ok && value != nil {
				return fmt.Errorf("%w: parameter %q on capability %q", aicontracts.ErrSensitiveParamsRequireOperator, param.Name, proposal.CapabilityName)
			}
		}
		return nil
	}
	// Unknown capability names fall through to the planner, which owns
	// the canonical capability-not-found refusal.
	return nil
}
