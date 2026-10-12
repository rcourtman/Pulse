package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
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
// persistence as the REST plan endpoint. Patrol never executes a plan
// itself: every proposal waits for an operator decision, and the enterprise
// investigation side stays a proposer, never a dispatcher.
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

// NewActionRefreshPlanner reconstructs trusted planning inputs for both
// operator-originated and broker-originated replacement plans. Patrol plans
// are rebound to the service actor; the authenticated operator only requests
// the refresh and cannot author that identity.
func NewActionRefreshPlanner() actionlifecycle.RefreshPlanner {
	return func(ctx context.Context, orgID string, previous unified.ActionAuditRecord, actor unified.ActionActor, requestID string) (unified.ActionRequest, actionlifecycle.PlanOptions, error) {
		req := previous.Request
		req.RequestID = requestID
		req.Actor = unified.ActionActor{}
		req.RequestedBy = ""
		opts := actionlifecycle.PlanOptions{Actor: actor, Origin: previous.Origin}
		if !isPatrolActionOrigin(previous.Origin) {
			return req, opts, nil
		}
		opts.RequireOperatorSensitiveParams = true
		req.RequestedBy = patrolActionBrokerActor
		opts.Actor = patrolActionBrokerServiceActor(orgID)
		return req, opts, nil
	}
}

func patrolActionBrokerServiceActor(orgID string) unified.ActionActor {
	return unified.ActionActor{
		SubjectID:    patrolActionBrokerActor,
		Kind:         unified.ActionActorService,
		CredentialID: "service:patrol-action-broker",
		OrgID:        orgID,
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

// Plan persists the canonical plan. Every Patrol plan waits for an operator
// decision; Patrol never requests execution.
func (b *patrolActionBroker) Plan(ctx context.Context, proposal aicontracts.ActionProposal) (aicontracts.ActionDisposition, error) {
	return b.submit(ctx, proposal)
}

// Submit is the extension contract's proposal entry point. It is the same
// plan-only operation as Plan.
func (b *patrolActionBroker) Submit(ctx context.Context, proposal aicontracts.ActionProposal) (aicontracts.ActionDisposition, error) {
	return b.submit(ctx, proposal)
}

func (b *patrolActionBroker) submit(ctx context.Context, proposal aicontracts.ActionProposal) (aicontracts.ActionDisposition, error) {
	proposal.ProposalID = strings.TrimSpace(proposal.ProposalID)
	if proposal.ProposalID == "" {
		return aicontracts.ActionDisposition{}, fmt.Errorf("action proposal requires a request identity")
	}
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

	plan, err := b.lifecycle().PlanWithOptions(ctx, b.orgID, unified.ActionRequest{
		RequestID:      proposal.ProposalID,
		ResourceID:     proposal.ResourceID,
		CapabilityName: proposal.CapabilityName,
		Params:         proposal.Params,
		Reason:         proposal.Reason,
		RequestedBy:    patrolActionBrokerActor,
	}, actionlifecycle.PlanOptions{
		RequireOperatorSensitiveParams: true,
		Actor:                          patrolActionBrokerServiceActor(b.orgID),
		Origin: &unified.ActionOrigin{
			Surface:         patrolActionOriginSurface,
			FindingID:       proposal.FindingID,
			InvestigationID: proposal.InvestigationID,
			ProposalID:      proposal.ProposalID,
			EvidenceIDs:     append([]string(nil), proposal.EvidenceIDs...),
		},
	})
	if err != nil {
		return aicontracts.ActionDisposition{}, err
	}

	record, found, err := b.lifecycle().Get(b.orgID, plan.ActionID)
	if err != nil || !found {
		// Planning already returned a durable identity. Do not turn a later read
		// failure into the false claim that no action exists.
		return aicontracts.ActionDisposition{ActionID: plan.ActionID}, fmt.Errorf("planned action %q could not be read", plan.ActionID)
	}
	return dispositionFromRecord(record), nil
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

// investigationPlannerFor exposes only plan persistence to the model tool.
// The concrete core broker remains tenant-bound and owns all policy inputs.
func investigationPlannerFor(broker aicontracts.OrchestratorActionBroker) tools.ProposalPlanner {
	b, ok := broker.(*patrolActionBroker)
	if !ok {
		return nil
	}
	return func(ctx context.Context, captured tools.CapturedProposal) (unified.ActionAuditRecord, error) {
		disposition, err := b.Plan(ctx, aicontracts.ActionProposal{ProposalID: captured.Identity.ProposalID, FindingID: captured.Identity.FindingID, InvestigationID: captured.Identity.InvestigationID, ResourceID: captured.ResourceID, CapabilityName: captured.CapabilityName, Params: captured.Params, Reason: captured.Reason, EvidenceIDs: captured.Identity.EvidenceIDs})
		if disposition.ActionID == "" {
			return unified.ActionAuditRecord{}, err
		}
		record, found, readErr := b.lifecycle().Get(b.orgID, disposition.ActionID)
		if readErr != nil || !found {
			// Preserve the known identity even if the follow-up read fails.
			return unified.ActionAuditRecord{ID: disposition.ActionID}, fmt.Errorf("action %s persisted but its audit is unavailable", disposition.ActionID)
		}
		return record, err
	}
}
