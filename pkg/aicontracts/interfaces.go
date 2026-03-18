package aicontracts

import (
	"context"
	"time"
)

// InvestigationOrchestrator defines the interface for autonomous investigation
// of patrol findings. The OSS binary never creates a concrete implementation;
// enterprise registers one via BusinessHooks.
type InvestigationOrchestrator interface {
	// InvestigateFinding starts an investigation for a finding.
	InvestigateFinding(ctx context.Context, finding *Finding, autonomyLevel string) error
	// GetInvestigationByFinding returns the latest investigation for a finding.
	GetInvestigationByFinding(findingID string) *InvestigationSession
	// GetRunningCount returns the number of running investigations.
	GetRunningCount() int
	// GetFixedCount returns the number of issues auto-fixed by Patrol.
	GetFixedCount() int
	// CanStartInvestigation returns true if a new investigation can be started.
	CanStartInvestigation() bool
	// ReinvestigateFinding triggers a re-investigation of a finding.
	ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error
	// Shutdown signals all running investigations to stop, persists state,
	// and waits for them to finish (up to the context deadline).
	Shutdown(ctx context.Context) error
}

// InvestigationStoreMaintainer is an optional interface for orchestrators that
// expose their investigation store for periodic maintenance.
type InvestigationStoreMaintainer interface {
	CleanupInvestigationStore(maxAge time.Duration, maxSessions int)
}

// RemediationEngine manages remediation plan lifecycle.
type RemediationEngine interface {
	// CreatePlan creates and stores a new remediation plan.
	CreatePlan(plan *RemediationPlan) error
	// GetPlan returns a plan by ID.
	GetPlan(planID string) *RemediationPlan
	// GetPlanForFinding returns the active plan for a finding.
	GetPlanForFinding(findingID string) *RemediationPlan
	// ListPlans returns remediation plans, ordered by creation time (newest first).
	ListPlans(limit int) []*RemediationPlan
	// GetLatestExecutionForPlan returns the most recent execution for a plan.
	GetLatestExecutionForPlan(planID string) *RemediationExecution
	// ApprovePlan approves a plan for execution.
	ApprovePlan(planID, approvedBy string) (*RemediationExecution, error)
	// Execute runs an approved execution.
	Execute(ctx context.Context, executionID string) error
	// Rollback attempts to rollback a failed execution.
	Rollback(ctx context.Context, executionID string) error
	// GetExecution returns an execution by ID.
	GetExecution(executionID string) *RemediationExecution
	// SetExecutionVerification updates the verification status of an execution.
	SetExecutionVerification(executionID string, verified bool, note string)
	// ListExecutions returns recent executions.
	ListExecutions(limit int) []*RemediationExecution
	// AddApprovalRule adds a pre-approval rule.
	AddApprovalRule(rule *ApprovalRule)
	// IsAutoApproved checks if a plan can be auto-executed.
	IsAutoApproved(plan *RemediationPlan) bool
	// FormatPlanForContext formats a plan for AI context.
	FormatPlanForContext(plan *RemediationPlan) string
	// SetCommandExecutor sets the command executor.
	SetCommandExecutor(executor CommandExecutor)
}

// CommandExecutor executes commands on target systems.
type CommandExecutor interface {
	Execute(ctx context.Context, target, command string) (output string, err error)
}

// AlertAnalyzer handles alert-triggered AI analysis.
// The OSS binary never creates a concrete implementation; enterprise registers
// one via BusinessHooks.
type AlertAnalyzer interface {
	OnAlertFired(alert AlertPayload)
	SetEnabled(enabled bool)
	IsEnabled() bool
	Start()
	Stop()
}

// AlertPayload is the minimal alert shape needed by the analyzer.
// This avoids importing internal/alerts in pkg/.
type AlertPayload interface {
	GetID() string
	GetType() string
	GetResourceID() string
	GetResourceName() string
	GetNode() string
	GetInstance() string
	GetMessage() string
	GetValue() float64
	GetThreshold() float64
	GetMetadata() map[string]interface{}
}

// InvestigationStore manages investigation session persistence.
// The concrete implementation lives in the enterprise repo.
type InvestigationStore interface {
	LoadFromDisk() error
	ForceSave() error
	Create(findingID, chatSessionID string) *InvestigationSession
	Get(id string) *InvestigationSession
	GetByFinding(findingID string) []*InvestigationSession
	GetLatestByFinding(findingID string) *InvestigationSession
	GetRunning() []*InvestigationSession
	CountRunning() int
	Update(session *InvestigationSession) bool
	UpdateStatus(id string, status InvestigationStatus) bool
	Complete(id string, outcome InvestigationOutcome, summary string, proposedFix *Fix) bool
	Fail(id string, errorMsg string) bool
	SetOutcome(id string, outcome InvestigationOutcome) bool
	IncrementTurnCount(id string) int
	SetApprovalID(id, approvalID string) bool
	GetAll() []*InvestigationSession
	CountFixed() int
	Cleanup(maxAge time.Duration) int
	EnforceSizeLimit(maxSessions int) int
}
