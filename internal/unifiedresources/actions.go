package unifiedresources

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ActionState tracks the lifecycle of bounded capability execution.
type ActionState string

const (
	ActionStatePlanned   ActionState = "planned"
	ActionStatePending   ActionState = "pending_approval"
	ActionStateApproved  ActionState = "approved"
	ActionStateRejected  ActionState = "rejected"
	ActionStateExecuting ActionState = "executing"
	ActionStateCompleted ActionState = "completed"
	ActionStateFailed    ActionState = "failed"
)

// ActionRequest is the payload from an agent or human requesting a capability execution.
type ActionRequest struct {
	RequestID      string         `json:"requestId"` // Caller idempotency key / external correlation
	ResourceID     string         `json:"resourceId"`
	CapabilityName string         `json:"capabilityName"`
	Params         map[string]any `json:"params,omitempty"`
	Reason         string         `json:"reason"`
	RequestedBy    string         `json:"requestedBy"` // e.g., "agent:oncall-helper"
}

// ApprovalOutcome represents the decision on a requested capability.
type ApprovalOutcome string

const (
	OutcomeApproved ApprovalOutcome = "approved"
	OutcomeRejected ApprovalOutcome = "rejected"
)

// ApprovalMethod tracks how the decision was collected.
type ApprovalMethod string

const (
	MethodUI           ApprovalMethod = "ui"
	MethodAPI          ApprovalMethod = "api"
	MethodMFAChallenge ApprovalMethod = "mfa_challenge"
)

// ActionApprovalRecord captures a specific approval or rejection event.
type ActionApprovalRecord struct {
	Actor     string          `json:"actor"`     // Who approved/rejected it
	Method    ApprovalMethod  `json:"method"`    // e.g. "ui", "api", "mfa_challenge"
	Timestamp time.Time       `json:"timestamp"` // When the decision was made
	Outcome   ApprovalOutcome `json:"outcome"`   // "approved" or "rejected"
	Reason    string          `json:"reason,omitempty"`
}

// ActionPreflight is the deterministic pre-execution readout shown before an
// action is approved or executed. It is intentionally explicit when no provider
// dry-run exists, so action audits can distinguish "not available" from
// "not recorded".
type ActionPreflight struct {
	Target            string    `json:"target,omitempty"`
	CurrentState      string    `json:"currentState,omitempty"`
	IntendedChange    string    `json:"intendedChange,omitempty"`
	DryRunAvailable   bool      `json:"dryRunAvailable"`
	DryRunSummary     string    `json:"dryRunSummary,omitempty"`
	SafetyChecks      []string  `json:"safetyChecks,omitempty"`
	VerificationSteps []string  `json:"verificationSteps,omitempty"`
	GeneratedAt       time.Time `json:"generatedAt,omitempty"`
}

// ActionPlan is the deterministic response Pulse gives back before execution.
type ActionPlan struct {
	ActionID             string              `json:"actionId"` // Internal durable identity
	RequestID            string              `json:"requestId"`
	Allowed              bool                `json:"allowed"`
	RequiresApproval     bool                `json:"requiresApproval"`
	ApprovalPolicy       ActionApprovalLevel `json:"approvalPolicy"`
	PredictedBlastRadius []string            `json:"predictedBlastRadius,omitempty"` // Correlated related resources
	RollbackAvailable    bool                `json:"rollbackAvailable"`
	Message              string              `json:"message,omitempty"`

	// Stale-plan protection
	PlannedAt       time.Time        `json:"plannedAt"`
	ExpiresAt       time.Time        `json:"expiresAt"`
	ResourceVersion string           `json:"resourceVersion"` // Hash of the resource state at planning time
	PolicyVersion   string           `json:"policyVersion"`   // Version of the capability/policy when planned
	PlanHash        string           `json:"planHash"`        // Hash verifying params and resource state haven't drifted
	Preflight       *ActionPreflight `json:"preflight,omitempty"`
}

// ExecutionResult captures the output of the native capability driver.
type ExecutionResult struct {
	Success      bool   `json:"success"`
	Output       string `json:"output,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ActionAuditRecord tracks the full end-to-end lifecycle of a tool invocation.
type ActionAuditRecord struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
	State     ActionState            `json:"state"`
	Request   ActionRequest          `json:"request"`
	Plan      ActionPlan             `json:"plan"`
	Approvals []ActionApprovalRecord `json:"approvals,omitempty"`
	Result    *ExecutionResult       `json:"result,omitempty"`
}

// ActionLifecycleEvent represents an append-only state transition in an action's life.
type ActionLifecycleEvent struct {
	ActionID  string      `json:"actionId"`
	Timestamp time.Time   `json:"timestamp"`
	State     ActionState `json:"state"`
	Actor     string      `json:"actor,omitempty"`
	Message   string      `json:"message,omitempty"`
}

// ActionEngine defines the enforced runtime loop for capabilities.
type ActionEngine interface {
	PlanAction(req ActionRequest) (*ActionPlan, error)
	ApproveAction(actionID string, approval ActionApprovalRecord) error
	ExecuteAction(actionID string) (*ExecutionResult, error)
}

var (
	ErrActionNotPending       = errors.New("action is not pending approval")
	ErrActionPlanExpired      = errors.New("action plan expired")
	ErrInvalidApprovalOutcome = errors.New("invalid approval outcome")
)

// ApplyActionDecision records an explicit approval or rejection against a
// pending governed action without starting execution. Execution remains a
// separate contract so approvals cannot become an implicit control bypass.
func ApplyActionDecision(record ActionAuditRecord, approval ActionApprovalRecord, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	approval.Actor = strings.TrimSpace(approval.Actor)
	approval.Reason = strings.TrimSpace(approval.Reason)
	if approval.Method == "" {
		approval.Method = MethodAPI
	}
	if approval.Timestamp.IsZero() {
		approval.Timestamp = now
	} else {
		approval.Timestamp = approval.Timestamp.UTC()
	}

	var nextState ActionState
	var message string
	switch approval.Outcome {
	case OutcomeApproved:
		nextState = ActionStateApproved
		message = "Action approved. Execution remains pending a separate execution contract."
	case OutcomeRejected:
		nextState = ActionStateRejected
		message = "Action rejected before execution."
	default:
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrInvalidApprovalOutcome
	}
	if record.State != ActionStatePending {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionNotPending
	}
	if !record.Plan.ExpiresAt.IsZero() && !now.Before(record.Plan.ExpiresAt) {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionPlanExpired
	}

	record.State = nextState
	record.UpdatedAt = approval.Timestamp
	record.Approvals = append(record.Approvals, approval)
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event := ActionLifecycleEvent{
		ActionID:  normalized.ID,
		Timestamp: approval.Timestamp,
		State:     nextState,
		Actor:     approval.Actor,
		Message:   message,
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	return normalized, normalizedEvent, nil
}

// NormalizeActionAuditRecord applies the canonical action-governance floor
// before a record is persisted. It keeps older callers usable by filling safe
// deterministic defaults, but rejects records that cannot identify the action,
// state, resource, capability, or requester.
func NormalizeActionAuditRecord(record ActionAuditRecord) (ActionAuditRecord, error) {
	record.ID = strings.TrimSpace(record.ID)
	record.Plan.ActionID = strings.TrimSpace(record.Plan.ActionID)
	if record.ID == "" {
		record.ID = record.Plan.ActionID
	}
	if record.ID == "" {
		return ActionAuditRecord{}, fmt.Errorf("action audit id required")
	}
	if record.Plan.ActionID == "" {
		record.Plan.ActionID = record.ID
	}
	if record.Plan.ActionID != record.ID {
		return ActionAuditRecord{}, fmt.Errorf("action audit id %q does not match plan action id %q", record.ID, record.Plan.ActionID)
	}
	if !isValidActionState(record.State) {
		return ActionAuditRecord{}, fmt.Errorf("unsupported action state %q", record.State)
	}

	record.Request.RequestID = strings.TrimSpace(record.Request.RequestID)
	record.Plan.RequestID = strings.TrimSpace(record.Plan.RequestID)
	if record.Request.RequestID == "" {
		record.Request.RequestID = record.Plan.RequestID
	}
	if record.Request.RequestID == "" {
		record.Request.RequestID = record.ID
	}
	if record.Plan.RequestID == "" {
		record.Plan.RequestID = record.Request.RequestID
	}
	if record.Plan.RequestID != record.Request.RequestID {
		return ActionAuditRecord{}, fmt.Errorf("action request id %q does not match plan request id %q", record.Request.RequestID, record.Plan.RequestID)
	}

	record.Request.ResourceID = CanonicalResourceID(record.Request.ResourceID)
	record.Request.CapabilityName = strings.TrimSpace(record.Request.CapabilityName)
	record.Request.Reason = strings.TrimSpace(record.Request.Reason)
	record.Request.RequestedBy = strings.TrimSpace(record.Request.RequestedBy)
	if record.Request.ResourceID == "" {
		return ActionAuditRecord{}, fmt.Errorf("action request resource id required")
	}
	if record.Request.CapabilityName == "" {
		return ActionAuditRecord{}, fmt.Errorf("action request capability name required")
	}
	if record.Request.RequestedBy == "" {
		return ActionAuditRecord{}, fmt.Errorf("action request requestedBy required")
	}

	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	} else {
		record.UpdatedAt = record.UpdatedAt.UTC()
	}

	if record.Plan.PlannedAt.IsZero() {
		record.Plan.PlannedAt = record.CreatedAt
	} else {
		record.Plan.PlannedAt = record.Plan.PlannedAt.UTC()
	}
	if record.Plan.ExpiresAt.IsZero() {
		record.Plan.ExpiresAt = record.Plan.PlannedAt.Add(5 * time.Minute)
	} else {
		record.Plan.ExpiresAt = record.Plan.ExpiresAt.UTC()
	}
	if record.Plan.ApprovalPolicy == "" {
		if record.Plan.RequiresApproval {
			record.Plan.ApprovalPolicy = ApprovalAdmin
		} else {
			record.Plan.ApprovalPolicy = ApprovalNone
		}
	}
	record.Plan.Preflight = NormalizeActionPreflight(record.Plan.Preflight, record.Request, record.Plan)

	for i := range record.Approvals {
		record.Approvals[i].Actor = strings.TrimSpace(record.Approvals[i].Actor)
		record.Approvals[i].Reason = strings.TrimSpace(record.Approvals[i].Reason)
		if record.Approvals[i].Method == "" {
			record.Approvals[i].Method = MethodAPI
		}
		if record.Approvals[i].Outcome == "" {
			record.Approvals[i].Outcome = OutcomeApproved
		}
		if record.Approvals[i].Timestamp.IsZero() {
			record.Approvals[i].Timestamp = record.UpdatedAt
		} else {
			record.Approvals[i].Timestamp = record.Approvals[i].Timestamp.UTC()
		}
	}

	return record, nil
}

// NormalizeActionLifecycleEvent applies the same action-governance identity and
// state checks to append-only lifecycle events.
func NormalizeActionLifecycleEvent(event ActionLifecycleEvent) (ActionLifecycleEvent, error) {
	event.ActionID = strings.TrimSpace(event.ActionID)
	if event.ActionID == "" {
		return ActionLifecycleEvent{}, fmt.Errorf("action lifecycle event action id required")
	}
	if !isValidActionState(event.State) {
		return ActionLifecycleEvent{}, fmt.Errorf("unsupported action lifecycle state %q", event.State)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.Actor = strings.TrimSpace(event.Actor)
	event.Message = strings.TrimSpace(event.Message)
	return event, nil
}

// NormalizeActionPreflight ensures persisted action plans always state whether
// a dry-run was available and what post-execution verification should inspect.
func NormalizeActionPreflight(preflight *ActionPreflight, request ActionRequest, plan ActionPlan) *ActionPreflight {
	if preflight == nil {
		preflight = &ActionPreflight{}
	}
	preflight.Target = strings.TrimSpace(preflight.Target)
	if preflight.Target == "" {
		preflight.Target = request.ResourceID
	}
	preflight.CurrentState = strings.TrimSpace(preflight.CurrentState)
	preflight.IntendedChange = strings.TrimSpace(preflight.IntendedChange)
	if preflight.IntendedChange == "" {
		preflight.IntendedChange = strings.TrimSpace(plan.Message)
	}
	if preflight.IntendedChange == "" {
		preflight.IntendedChange = strings.TrimSpace(request.Reason)
	}
	preflight.DryRunSummary = strings.TrimSpace(preflight.DryRunSummary)
	if preflight.DryRunSummary == "" {
		if preflight.DryRunAvailable {
			preflight.DryRunSummary = "Provider-supported dry run is available for this action."
		} else {
			preflight.DryRunSummary = "No provider-supported dry run is available for this action."
		}
	}
	if len(preflight.SafetyChecks) == 0 {
		preflight.SafetyChecks = []string{"Action is recorded in the unified action audit."}
	}
	if len(preflight.VerificationSteps) == 0 {
		preflight.VerificationSteps = []string{"Review the action result and lifecycle events after execution."}
	}
	if preflight.GeneratedAt.IsZero() {
		preflight.GeneratedAt = plan.PlannedAt
	} else {
		preflight.GeneratedAt = preflight.GeneratedAt.UTC()
	}
	return preflight
}

func isValidActionState(state ActionState) bool {
	switch state {
	case ActionStatePlanned, ActionStatePending, ActionStateApproved, ActionStateRejected, ActionStateExecuting, ActionStateCompleted, ActionStateFailed:
		return true
	default:
		return false
	}
}
