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

	plan, err := b.lifecycle().PlanWithOptions(ctx, b.orgID, unified.ActionRequest{
		RequestID:      proposal.ProposalID,
		ResourceID:     proposal.ResourceID,
		CapabilityName: proposal.CapabilityName,
		Params:         proposal.Params,
		Reason:         proposal.Reason,
		RequestedBy:    patrolActionBrokerActor,
	}, actionlifecycle.PlanOptions{
		Origin: &unified.ActionOrigin{
			Surface:         patrolActionOriginSurface,
			FindingID:       proposal.FindingID,
			InvestigationID: proposal.InvestigationID,
			ProposalID:      proposal.ProposalID,
		},
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

	if allowed, _, policyErr := b.autoAuthorizationDecision(ctx, proposal); policyErr == nil && allowed {
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

func (b *patrolActionBroker) policyAuthorizationLease(ctx context.Context, proposal aicontracts.ActionProposal, record unified.ActionAuditRecord, now time.Time) (unified.ActionPolicyAuthorizationLease, string, error) {
	allowed, reason, err := b.autoAuthorizationDecisionAt(ctx, proposal, now)
	if err != nil {
		return unified.ActionPolicyAuthorizationLease{}, "", err
	}
	if !allowed {
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationRevoked
	}
	snapshot, err := b.policy(ctx, b.orgID)
	if err != nil {
		return unified.ActionPolicyAuthorizationLease{}, "", err
	}
	capabilities, err := b.lifecycle().Capabilities(ctx, b.orgID, proposal.ResourceID)
	if err != nil {
		return unified.ActionPolicyAuthorizationLease{}, "", err
	}
	var capability unified.ResourceCapability
	foundCapability := false
	for _, candidate := range capabilities {
		if strings.TrimSpace(candidate.Name) == proposal.CapabilityName {
			capability = candidate
			foundCapability = true
			break
		}
	}
	if !foundCapability {
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationRevoked
	}
	state, found, err := b.lifecycle().ResourceOperatorState(b.orgID, proposal.ResourceID)
	if err != nil || !found {
		if err != nil {
			return unified.ActionPolicyAuthorizationLease{}, "", err
		}
		return unified.ActionPolicyAuthorizationLease{}, "", unified.ErrActionPolicyAuthorizationInvalid
	}
	tenantVersion := strings.TrimSpace(snapshot.PolicyVersion)
	if tenantVersion == "" {
		tenantVersion = policySnapshotVersion(snapshot)
	}
	resourcePayload, _ := json.Marshal(unified.NormalizeResourceOperatorState(state))
	resourceSum := sha256.Sum256(resourcePayload)
	expiresAt := record.Plan.ExpiresAt
	if expiresAt.IsZero() || expiresAt.After(now.Add(time.Minute)) {
		expiresAt = now.Add(time.Minute)
	}
	lease := unified.ActionPolicyAuthorizationLease{
		Version: 1, OrgID: b.orgID, ActionID: record.ID, ResourceID: proposal.ResourceID, CapabilityName: proposal.CapabilityName,
		PlanHash: record.Plan.PlanHash, CapabilityPolicyVersion: record.Plan.PolicyVersion,
		AutoAuthorization: unified.NormalizeActionAutoAuthorizationClass(capability.AutoAuthorization), ApprovalPolicy: capability.MinimumApprovalLevel,
		TenantPolicyVersion: tenantVersion, EffectiveAutonomyLevel: strings.TrimSpace(snapshot.EffectiveAutonomyLevel), LicenseAllowsAutoFix: true,
		FullModeUnlocked: snapshot.FullModeUnlocked, EmergencyStop: snapshot.EmergencyStop,
		ResourcePolicyVersion: fmt.Sprintf("resource-policy:sha256:%x", resourceSum[:12]), CapabilityNames: append([]string(nil), state.AutoRemediationPolicy.CapabilityNames...),
		Window: state.AutoRemediationPolicy.Window, NeverAutoRemediate: state.NeverAutoRemediate, IssuedAt: now.UTC(), ExpiresAt: expiresAt.UTC(),
	}
	lease.Digest = unified.ActionPolicyAuthorizationDigest(lease)
	return lease, reason, nil
}

func policySnapshotVersion(snapshot PatrolActionPolicySnapshot) string {
	snapshot.PolicyVersion = ""
	payload, _ := json.Marshal(snapshot)
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("tenant-policy:sha256:%x", sum[:12])
}

func dispositionFromRecord(record unified.ActionAuditRecord) aicontracts.ActionDisposition {
	return aicontracts.ActionDisposition{
		ActionID:           record.ID,
		State:              string(record.State),
		VerificationStatus: string(record.VerificationOutcome.Status),
		Plan:               *approvalPlanRequestToInfo(&record.Plan),
	}
}

func (b *patrolActionBroker) autoAuthorizationDecision(ctx context.Context, proposal aicontracts.ActionProposal) (bool, string, error) {
	return b.autoAuthorizationDecisionAt(ctx, proposal, b.currentTime())
}

func (b *patrolActionBroker) autoAuthorizationDecisionAt(ctx context.Context, proposal aicontracts.ActionProposal, now time.Time) (bool, string, error) {
	if b.policy == nil {
		return false, "", nil
	}
	snapshot, err := b.policy(ctx, b.orgID)
	if err != nil {
		return false, "", err
	}
	if snapshot.EmergencyStop {
		return false, "", nil
	}
	capabilities, err := b.lifecycle().Capabilities(ctx, b.orgID, proposal.ResourceID)
	if err != nil {
		return false, "", err
	}
	var capability *unified.ResourceCapability
	for i := range capabilities {
		if strings.TrimSpace(capabilities[i].Name) == proposal.CapabilityName {
			capability = &capabilities[i]
			break
		}
	}
	if capability == nil {
		return false, "", nil
	}
	class := unified.NormalizeActionAutoAuthorizationClass(capability.AutoAuthorization)
	if class == unified.AutoAuthorizeNever || capability.MinimumApprovalLevel == unified.ApprovalDryRun || capability.MinimumApprovalLevel == unified.ApprovalMultiFactor {
		return false, "", nil
	}
	switch strings.TrimSpace(snapshot.EffectiveAutonomyLevel) {
	case "assisted":
		if class != unified.AutoAuthorizeLowRisk {
			return false, "", nil
		}
	case "full":
		if !snapshot.FullModeUnlocked {
			return false, "", nil
		}
	default:
		return false, "", nil
	}
	state, found, err := b.lifecycle().ResourceOperatorState(b.orgID, proposal.ResourceID)
	if err != nil {
		return false, "", err
	}
	if !found || !state.AllowsAutoRemediationAt(proposal.CapabilityName, now) {
		return false, "", nil
	}
	return true, fmt.Sprintf("Patrol %s policy authorized %s capability %s for resource %s", snapshot.EffectiveAutonomyLevel, class, proposal.CapabilityName, proposal.ResourceID), nil
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
