package aicontracts

import (
	"context"
	"encoding/json"
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

// ActionCapabilityCatalog is the read-only projection of the capabilities a
// proposal for a resource can use now, so an investigation can select a
// capability and fill its declared parameters instead of guessing planner
// input. A capability the resource declares but planning would refuse is not
// in Capabilities: it is in Unavailable with the refusal.
type ActionCapabilityCatalog struct {
	ResourceID   string                 `json:"resource_id"`
	Capabilities []ActionCapabilityInfo `json:"capabilities"`
	// Unavailable lists declared capabilities that cannot run now, each with
	// the executor's own reason.
	Unavailable []ActionCapabilityRefusal `json:"unavailable,omitempty"`
	// CommandsDisabled is set when the Pulse agent that reads from and acts
	// on this resource's host reports command execution off. Pulse then
	// cannot read logs or files, or run commands or actions, for the
	// resource. It is a setting the owner chose at install, not a fault.
	CommandsDisabled *CommandsDisabledHost `json:"commands_disabled,omitempty"`
	// GuestActionsUnavailable is set when this resource is a Proxmox guest
	// and Pulse refuses an action it declares because of how its node is set
	// up: no action runner for stop, shutdown or reboot, or no Pulse agent
	// that can start a guest. It is how this install is set up, not a fault.
	// A node whose agent has commands off is CommandsDisabled instead.
	GuestActionsUnavailable *GuestActionsUnavailable `json:"guest_actions_unavailable,omitempty"`
}

// ActionCapabilityRefusal is one declared capability planning would refuse.
type ActionCapabilityRefusal struct {
	Name       string `json:"name"`
	ReasonCode string `json:"reason_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// CommandsDisabledHost names a host whose Pulse agent was installed without
// command access, with the one sentence and the one step Pulse shows for it.
// Pulse writes both so no model has to guess why a read or action was refused.
type CommandsDisabledHost struct {
	HostResourceID string `json:"host_resource_id,omitempty"`
	HostName       string `json:"host_name"`
	// Reason is the fact, for example "Commands are not enabled on the Pulse
	// agent on docker01."
	Reason string `json:"reason"`
	// Step is how the owner turns commands on for that host.
	Step string `json:"step"`
}

// GuestActionsUnavailable names a Proxmox guest and the actions Pulse refuses
// for it because of how its node is set up. Pulse writes the sentence so no
// model has to explain the refusal. The sentence carries the step when there
// is one, such as updating the node's Pulse agent.
type GuestActionsUnavailable struct {
	GuestName string `json:"guest_name,omitempty"`
	NodeName  string `json:"node_name"`
	// Actions are the declared guest actions refused for this reason, for
	// example "start" for a stopped guest.
	Actions []string `json:"actions"`
	// Reason is the fact and what to do now, for example "Pulse cannot
	// start guests on pve1: ... Use Proxmox for this." When a start is among
	// the refused actions it is the start's sentence.
	Reason string `json:"reason"`
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
	ActionID           string          `json:"action_id"`
	State              string          `json:"state"`
	ActionResultV2     json.RawMessage `json:"action_result_v2,omitempty"`
	VerificationStatus string          `json:"verification_status,omitempty"`
	Plan               ActionPlanInfo  `json:"plan"`
}

// ActionReference links an investigation to its canonical action lifecycle
// record. It replaces the command-shaped Fix as the executable artifact:
// the investigation stores only this reference, while proposal parameters
// and lifecycle state live in the canonical action audit.
type ActionReference struct {
	// CausalResourceID is attributed by the model, not established by action acceptance.
	CausalResourceID   string          `json:"causal_resource_id,omitempty"`
	ActionResultV2     json.RawMessage `json:"action_result_v2,omitempty"`
	VerificationStatus string          `json:"verification_status,omitempty"`
	ActionID           string          `json:"action_id"`
	ProposalID         string          `json:"proposal_id,omitempty"`
	ResourceID         string          `json:"resource_id"`
	CapabilityName     string          `json:"capability_name"`
	State              string          `json:"state"`
	Plan               ActionPlanInfo  `json:"plan"`
}

// CloneActionReference returns an immutable deep copy suitable for crossing
// investigation-store and product-read-model boundaries.
func CloneActionReference(reference *ActionReference) *ActionReference {
	if reference == nil {
		return nil
	}
	clone := *reference
	clone.Plan.ApprovalRequirement = append(json.RawMessage(nil), reference.Plan.ApprovalRequirement...)
	clone.ActionResultV2 = append(json.RawMessage(nil), reference.ActionResultV2...)
	clone.Plan.PolicyDecision = append(json.RawMessage(nil), reference.Plan.PolicyDecision...)
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
