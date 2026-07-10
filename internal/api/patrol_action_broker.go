package api

import (
	"context"
	"fmt"
	"strings"

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

// patrolActionBroker is the org-bound core adapter behind
// aicontracts.OrchestratorActionBroker. It routes every Patrol proposal
// through the shared action lifecycle service: identical registry lookup,
// capability validation, availability checks, plan hashing, and audit
// persistence as the REST plan endpoint. Submit is plan-only; approval and
// execution remain on the canonical decision/execute lifecycle, so the
// enterprise investigation side stays a proposer, never a dispatcher.
type patrolActionBroker struct {
	orgID string
	// lifecycle resolves the shared service per call so late-bound
	// executor and publisher wiring on ResourceHandlers stays current.
	lifecycle func() *actionlifecycle.Service
}

// NewPatrolActionBroker builds the tenant-bound Patrol proposal broker over
// the shared action lifecycle service.
func NewPatrolActionBroker(orgID string, resources *ResourceHandlers) aicontracts.OrchestratorActionBroker {
	return &patrolActionBroker{
		orgID:     strings.TrimSpace(orgID),
		lifecycle: resources.ActionLifecycle,
	}
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

	// Plan-only by contract: even an ApprovalNone capability is returned
	// as a planned disposition and never auto-executed on submission.
	return aicontracts.ActionDisposition{
		ActionID: plan.ActionID,
		State:    string(actionlifecycle.PlannedActionState(plan)),
		Plan:     *approvalPlanRequestToInfo(&plan),
	}, nil
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
