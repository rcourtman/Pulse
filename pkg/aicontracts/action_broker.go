package aicontracts

import (
	"context"
	"errors"
)

// ---------------------------------------------------------------------------
// Typed action proposals
//
// OrchestratorActionBroker is the ONLY sanctioned route from an enterprise
// Patrol investigation to a Pulse infrastructure mutation. The enterprise
// side is a proposer, never a dispatcher: the contract deliberately has no
// org ID (the core adapter is tenant-bound at construction), no requestedBy
// (the adapter always stamps the fixed Patrol actor), no autonomy, risk,
// approval-policy, or destructive fields (authorization derives from the
// capability's declared policy, never from model or enterprise input), no
// Decide/Execute methods, and no command or target-host fields. Submit hands
// the proposal to core; only core may progress it under capability-owned
// eligibility plus operator policy.
// ---------------------------------------------------------------------------

// ActionProposal is a typed, no-authority remediation proposal produced by
// an investigation. Params must reference the capability's declared
// parameter schema; free-form command text is not representable.
type ActionProposal struct {
	ProposalID      string         `json:"proposal_id"`
	FindingID       string         `json:"finding_id"`
	InvestigationID string         `json:"investigation_id"`
	ResourceID      string         `json:"resource_id"`
	CapabilityName  string         `json:"capability_name"`
	Params          map[string]any `json:"params,omitempty"`
	Reason          string         `json:"reason"`
	EvidenceIDs     []string       `json:"evidence_ids,omitempty"`
}

// ActionCapabilityCatalog is the read-only projection of a resource's
// advertised capabilities, so an investigation can select a capability and
// fill its declared parameters instead of guessing planner input.
type ActionCapabilityCatalog struct {
	ResourceID   string                 `json:"resource_id"`
	Capabilities []ActionCapabilityInfo `json:"capabilities"`
}

// ActionCapabilityInfo describes one advertised capability.
type ActionCapabilityInfo struct {
	Name                 string                      `json:"name"`
	Description          string                      `json:"description"`
	MinimumApprovalLevel string                      `json:"minimum_approval_level"`
	AutoAuthorization    string                      `json:"auto_authorization"`
	Platform             string                      `json:"platform,omitempty"`
	Params               []ActionCapabilityParamInfo `json:"params,omitempty"`
}

// ActionCapabilityParamInfo describes one declared capability parameter.
// Sensitive parameters must never be populated by a model-driven proposal;
// the broker rejects such proposals so secrets stay out of model output,
// investigation persistence, and action audit records.
type ActionCapabilityParamInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Description string   `json:"description,omitempty"`
	Sensitive   bool     `json:"sensitive,omitempty"`
}

// ActionDisposition reports what the canonical planner did with a proposal.
// State reflects the persisted audit state, including a terminal state when
// core policy authorized and executed the proposal. Plan reuses
// the existing safe ActionPlanInfo projection shared with approval
// surfaces (see fix_execution.go).
type ActionDisposition struct {
	ActionID           string         `json:"action_id"`
	State              string         `json:"state"`
	VerificationStatus string         `json:"verification_status,omitempty"`
	Plan               ActionPlanInfo `json:"plan"`
}

// ActionReference links an investigation to its canonical action lifecycle
// record. It replaces the command-shaped Fix as the executable artifact:
// the investigation stores only this reference, while proposal parameters
// and lifecycle state live in the canonical action audit.
type ActionReference struct {
	ActionID       string         `json:"action_id"`
	ProposalID     string         `json:"proposal_id,omitempty"`
	ResourceID     string         `json:"resource_id"`
	CapabilityName string         `json:"capability_name"`
	State          string         `json:"state"`
	Plan           ActionPlanInfo `json:"plan"`
}

// CloneActionReference returns an immutable deep copy suitable for crossing
// investigation-store and product-read-model boundaries.
func CloneActionReference(reference *ActionReference) *ActionReference {
	if reference == nil {
		return nil
	}
	clone := *reference
	clone.Plan.PredictedBlastRadius = append([]string(nil), reference.Plan.PredictedBlastRadius...)
	if reference.Plan.Preflight != nil {
		preflight := *reference.Plan.Preflight
		preflight.SafetyChecks = append([]string(nil), reference.Plan.Preflight.SafetyChecks...)
		preflight.VerificationSteps = append([]string(nil), reference.Plan.Preflight.VerificationSteps...)
		clone.Plan.Preflight = &preflight
	}
	return &clone
}

// ErrSensitiveParamsRequireOperator reports that a proposal populated a
// parameter the capability declares sensitive. Such proposals stop for
// operator input instead of persisting secret material.
var ErrSensitiveParamsRequireOperator = errors.New("proposal populates a sensitive capability parameter; operator input required")

// OrchestratorActionBroker is the proposal seam between the enterprise
// investigation orchestrator and the core action lifecycle.
type OrchestratorActionBroker interface {
	// Capabilities returns the resource's advertised capability catalog.
	Capabilities(ctx context.Context, resourceID string) (ActionCapabilityCatalog, error)
	// Submit plans the proposal through the canonical action lifecycle
	// and returns its persisted disposition. Any policy authorization and
	// execution is core-owned; the enterprise caller has no decide/execute
	// authority.
	Submit(ctx context.Context, proposal ActionProposal) (ActionDisposition, error)
}
