package unifiedresources

import "time"

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
	PlannedAt       time.Time `json:"plannedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
	ResourceVersion string    `json:"resourceVersion"` // Hash of the resource state at planning time
	PolicyVersion   string    `json:"policyVersion"`   // Version of the capability/policy when planned
	GraphVersion    string    `json:"graphVersion"`    // Enforces blast-radius hasn't drifted
	PlanHash        string    `json:"planHash"`        // Hash verifying params and relationships haven't drifted
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
