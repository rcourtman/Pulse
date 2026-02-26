// Package remediation provides AI-guided fix plans with safe execution capabilities.
//
// Type aliases: All shared types are re-exported from pkg/aicontracts so that
// both internal code and the enterprise binary use the same type definitions.
// New code should import pkg/aicontracts directly.
package remediation

import "github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"

// ---------------------------------------------------------------------------
// Type aliases — re-export from pkg/aicontracts
// ---------------------------------------------------------------------------

type PlanCategory = aicontracts.PlanCategory
type RiskLevel = aicontracts.RiskLevel
type ExecutionStatus = aicontracts.ExecutionStatus
type RemediationStep = aicontracts.RemediationStep
type RemediationPlan = aicontracts.RemediationPlan
type RemediationExecution = aicontracts.RemediationExecution
type StepResult = aicontracts.StepResult
type ApprovalRule = aicontracts.ApprovalRule
type EngineConfig = aicontracts.EngineConfig
type CommandExecutor = aicontracts.CommandExecutor

// PlanCategory constants.
const (
	CategoryInformational = aicontracts.CategoryInformational
	CategoryGuided        = aicontracts.CategoryGuided
	CategoryOneClick      = aicontracts.CategoryOneClick
	CategoryAutonomous    = aicontracts.CategoryAutonomous
)

// RiskLevel constants.
const (
	RiskLow      = aicontracts.RiskLow
	RiskMedium   = aicontracts.RiskMedium
	RiskHigh     = aicontracts.RiskHigh
	RiskCritical = aicontracts.RiskCritical
)

// ExecutionStatus constants.
const (
	StatusPending    = aicontracts.ExecStatusPending
	StatusApproved   = aicontracts.ExecStatusApproved
	StatusRunning    = aicontracts.ExecStatusRunning
	StatusCompleted  = aicontracts.ExecStatusCompleted
	StatusFailed     = aicontracts.ExecStatusFailed
	StatusRolledBack = aicontracts.ExecStatusRolledBack
)

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return aicontracts.DefaultEngineConfig()
}
