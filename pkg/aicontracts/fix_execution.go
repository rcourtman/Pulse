package aicontracts

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// Fix execution contract types
// ---------------------------------------------------------------------------
// These types cross the OSS↔enterprise boundary for investigation fix
// approval, execution, and patrol autonomy management.

// ApprovalInfo is the contract type for approval requests crossing the
// OSS/enterprise boundary. It mirrors the core approval.ApprovalRequest
// fields needed by enterprise handlers and the frontend UI.
type ApprovalInfo struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"orgId,omitempty"`
	ExecutionID string     `json:"executionId"`
	ToolID      string     `json:"toolId"`
	Command     string     `json:"command"`
	TargetType  string     `json:"targetType"`
	TargetID    string     `json:"targetId"`
	TargetName  string     `json:"targetName"`
	Context     string     `json:"context"`
	RiskLevel   string     `json:"riskLevel"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requestedAt"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	DecidedAt   *time.Time `json:"decidedAt,omitempty"`
	DecidedBy   string     `json:"decidedBy,omitempty"`
	DenyReason  string     `json:"denyReason,omitempty"`
	CommandHash string     `json:"commandHash,omitempty"`
	Consumed    bool       `json:"consumed,omitempty"`
}

// ApprovalStoreAccessor provides approval operations for enterprise handlers.
type ApprovalStoreAccessor interface {
	// GetApproval returns an approval request by ID.
	GetApproval(id string) (*ApprovalInfo, bool)
	// Approve marks an approval request as approved.
	Approve(id, username string) (*ApprovalInfo, error)
	// CreateApproval creates a new approval request.
	CreateApproval(req *ApprovalInfo) error
	// GetPendingForOrg returns pending approvals for an org and per-org stats.
	GetPendingForOrg(orgID string) ([]*ApprovalInfo, map[string]int)
	// BelongsToOrg checks whether an approval belongs to the given org.
	BelongsToOrg(info *ApprovalInfo, orgID string) bool
	// AssessRiskLevel determines the risk level of a command.
	AssessRiskLevel(command, targetType string) string
}

// MCPToolExecutor executes MCP tool calls via the chat service.
type MCPToolExecutor interface {
	ExecuteMCPTool(ctx context.Context, command, approvalID string) (output string, exitCode int, err error)
}

// AgentCommandExecutor executes shell commands via connected agents.
type AgentCommandExecutor interface {
	ExecuteCommand(ctx context.Context, agentID, command string) (stdout, stderr string, exitCode int, err error)
	FindAgentForTarget(targetHost string) string
}

// FindingOutcomeUpdater updates investigation outcomes on findings.
type FindingOutcomeUpdater interface {
	UpdateFindingOutcome(ctx context.Context, orgID, findingID, outcome string)
}

// FixVerificationLauncher launches background fix verification after a fix
// has been executed. It sleeps briefly and then re-checks whether the issue
// persists.
type FixVerificationLauncher interface {
	LaunchVerification(ctx context.Context, orgID, findingID, sessionID string, proposedFix *Fix, store InvestigationStore)
}

// PatrolConfigAccessor provides read access to patrol configuration for
// enterprise handlers.
type PatrolConfigAccessor interface {
	GetEffectiveAutonomyLevel() string
	HasLicenseFeature(feature string) bool
	GetPatrolInvestigationBudget() int
	GetPatrolInvestigationTimeout() time.Duration
	GetPatrolFullModeUnlocked() bool
	GetPatrolAutonomyLevel() string
	IsValidPatrolAutonomyLevel(level string) bool
}

// PatrolConfigUpdater provides config mutation for autonomy settings updates.
type PatrolConfigUpdater interface {
	SaveAutonomySettings(ctx context.Context, level string, unlocked bool, budget, timeoutSec int) error
	ReloadConfig(ctx context.Context) error
}
