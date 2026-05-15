package aicontracts

import "time"

// ---------------------------------------------------------------------------
// Plan category
// ---------------------------------------------------------------------------

// PlanCategory categorizes remediation plans by safety level.
type PlanCategory string

const (
	CategoryInformational PlanCategory = "informational"
	CategoryGuided        PlanCategory = "guided"
	CategoryOneClick      PlanCategory = "one_click"
	CategoryAutonomous    PlanCategory = "autonomous"
)

// ---------------------------------------------------------------------------
// Risk level
// ---------------------------------------------------------------------------

// RiskLevel indicates the risk of a remediation action.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// ---------------------------------------------------------------------------
// Execution status
// ---------------------------------------------------------------------------

// ExecutionStatus tracks the status of a remediation execution.
type ExecutionStatus string

const (
	ExecStatusPending    ExecutionStatus = "pending"
	ExecStatusApproved   ExecutionStatus = "approved"
	ExecStatusRunning    ExecutionStatus = "running"
	ExecStatusCompleted  ExecutionStatus = "completed"
	ExecStatusFailed     ExecutionStatus = "failed"
	ExecStatusRolledBack ExecutionStatus = "rolled_back"
)

// ---------------------------------------------------------------------------
// Remediation types
// ---------------------------------------------------------------------------

// RemediationStep represents a single step in a remediation plan.
type RemediationStep struct {
	Order       int                    `json:"order"`
	Description string                 `json:"description"`
	Command     string                 `json:"command,omitempty"`
	Target      string                 `json:"target,omitempty"`
	Rollback    string                 `json:"rollback,omitempty"`
	WaitAfter   time.Duration          `json:"wait_after,omitempty"`
	Condition   string                 `json:"condition,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RemediationPlan represents a complete plan to fix an issue.
type RemediationPlan struct {
	ID            string            `json:"id"`
	FindingID     string            `json:"finding_id"`
	ResourceID    string            `json:"resource_id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Category      PlanCategory      `json:"category"`
	RiskLevel     RiskLevel         `json:"risk_level"`
	Steps         []RemediationStep `json:"steps"`
	Rationale     string            `json:"rationale,omitempty"`
	Prerequisites []string          `json:"prerequisites,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`

	// ProposedActionPlan is an optional governed action proposal projection.
	// It is present only when the remediation owner has already produced a
	// concrete action artifact; Patrol finding creation must not populate it
	// from category/title heuristics. Always nil for plans that have no
	// attached action proposal.
	ProposedActionPlan *ProposedActionPlan `json:"proposed_action_plan,omitempty"`
}

// ProposedActionPlan is the wire-side projection of a governed action proposal
// attached to a RemediationPlan. RequiresApproval is invariant - every
// proposed plan must carry an explicit approval gate. Allowed=false signals
// "no write capability is wired yet; this is a preflight-only proposal".
type ProposedActionPlan struct {
	ActionID         string                   `json:"actionId"`
	CapabilityName   string                   `json:"capabilityName,omitempty"`
	Allowed          bool                     `json:"allowed"`
	RequiresApproval bool                     `json:"requiresApproval"`
	ApprovalPolicy   string                   `json:"approvalPolicy,omitempty"`
	Message          string                   `json:"message,omitempty"`
	Source           string                   `json:"source,omitempty"`
	ProjectedMetric  *ProposedMetricSummary   `json:"projectedMetric,omitempty"`
	Preflight        *ProposedActionPreflight `json:"preflight,omitempty"`
	PlannedAt        time.Time                `json:"plannedAt,omitempty"`
	ExpiresAt        time.Time                `json:"expiresAt,omitempty"`
}

// ProposedActionPreflight mirrors unifiedresources.ActionPreflight on the
// wire so the frontend can render the proposed change without depending on
// internal types.
type ProposedActionPreflight struct {
	Target            string   `json:"target,omitempty"`
	CurrentState      string   `json:"currentState,omitempty"`
	IntendedChange    string   `json:"intendedChange,omitempty"`
	DryRunAvailable   bool     `json:"dryRunAvailable"`
	DryRunSummary     string   `json:"dryRunSummary,omitempty"`
	SafetyChecks      []string `json:"safetyChecks,omitempty"`
	VerificationSteps []string `json:"verificationSteps,omitempty"`
}

// ProposedMetricSummary is the operator-facing "why this proposal exists"
// snapshot - current metric value, projected value at horizon, threshold,
// and (optional) time-to-threshold-breach in seconds. The frontend renders
// this in the capacity-forecast approval card so the operator can decide
// without digging into Patrol metrics.
type ProposedMetricSummary struct {
	Metric                 string  `json:"metric"`
	CurrentValue           float64 `json:"currentValue"`
	PredictedValue         float64 `json:"predictedValue,omitempty"`
	ThresholdValue         float64 `json:"thresholdValue,omitempty"`
	TimeToThresholdSeconds *int64  `json:"timeToThresholdSeconds,omitempty"`
}

// RemediationExecution tracks the execution of a remediation plan.
type RemediationExecution struct {
	ID               string          `json:"id"`
	PlanID           string          `json:"plan_id"`
	Status           ExecutionStatus `json:"status"`
	ApprovedBy       string          `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time      `json:"approved_at,omitempty"`
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	CurrentStep      int             `json:"current_step"`
	StepResults      []StepResult    `json:"step_results,omitempty"`
	Error            string          `json:"error,omitempty"`
	RollbackError    string          `json:"rollback_error,omitempty"`
	Verified         *bool           `json:"verified,omitempty"`
	VerificationNote string          `json:"verification_note,omitempty"`
}

// StepResult records the result of executing a step.
type StepResult struct {
	Step     int           `json:"step"`
	Success  bool          `json:"success"`
	Output   string        `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration_ms"`
	RunAt    time.Time     `json:"run_at"`
}

// ApprovalRule defines pre-approved actions for autonomous execution.
type ApprovalRule struct {
	ID           string       `json:"id"`
	Description  string       `json:"description"`
	Category     PlanCategory `json:"category"`
	ResourceType string       `json:"resource_type,omitempty"`
	ActionType   string       `json:"action_type"`
	MaxRiskLevel RiskLevel    `json:"max_risk_level"`
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	CreatedBy    string       `json:"created_by,omitempty"`
}

// EngineConfig configures the remediation engine.
type EngineConfig struct {
	DataDir          string
	MaxExecutions    int
	PlanExpiry       time.Duration
	ExecutionTimeout time.Duration
}

// DefaultEngineConfig returns sensible defaults for the remediation engine.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxExecutions:    100,
		PlanExpiry:       24 * time.Hour,
		ExecutionTimeout: 5 * time.Minute,
	}
}
