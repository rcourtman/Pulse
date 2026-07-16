package chat

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

const (
	// Watch is an automatic structured decision loop, not an open-ended
	// investigation. Keep enough room for complete finding arguments while
	// preventing a concise check from inheriting a provider's unbounded default.
	patrolDetectionRunOutputAllowance  = 7_000
	patrolDetectionTurnOutputAllowance = 2_048
	patrolDetectionSummaryAllowance    = 1_024
	patrolDetectionMinimumAllowance    = 512
)

func applyExecutionInferenceAllowance(req *providers.ChatRequest, profile tools.ExecutionProfile, summaryOnly bool, outputTokensUsed int) {
	if req == nil || profile != tools.ProfilePatrolDetection {
		return
	}

	allowance := patrolDetectionTurnOutputAllowance
	if summaryOnly {
		allowance = patrolDetectionSummaryAllowance
	}
	if remaining := patrolDetectionRunOutputAllowance - outputTokensUsed; remaining < allowance {
		allowance = remaining
	}
	if allowance < patrolDetectionMinimumAllowance {
		allowance = patrolDetectionMinimumAllowance
	}

	req.MaxTokens = allowance
	req.ReasoningEffort = providers.ReasoningEffortLow
}

// Abort aborts an ongoing session.
func (a *AgenticLoop) Abort(sessionID string) {
	a.mu.Lock()
	a.aborted[sessionID] = true
	a.mu.Unlock()
}

// SetAutonomousMode sets whether the loop is in autonomous mode (for investigations).
// When enabled, approval requests don't block waiting for user input.
func (a *AgenticLoop) SetAutonomousMode(enabled bool) {
	a.mu.Lock()
	a.autonomousMode = enabled
	a.mu.Unlock()
}

// SetExecutionProfile applies the core-owned execution profile for this
// loop. The profile owns non-interactive behavior (question handling,
// tool-only-turn wrap-up) and the prompt's execution-mode description;
// it is deliberately separate from autonomous mode, which only affects
// approval waiting and grants no mutation authority.
func (a *AgenticLoop) SetExecutionProfile(profile tools.ExecutionProfile) {
	if !profile.Valid() {
		panic("unknown execution profile: profiles are a closed vocabulary and cannot default to interactive permissions")
	}
	a.mu.Lock()
	a.executionProfile = profile
	if profile == tools.ProfilePatrolDetection || profile == tools.ProfilePatrolInvestigation {
		a.streamIdleTimeout = patrolProviderStreamIdleTimeout
	} else {
		a.streamIdleTimeout = 0
	}
	a.mu.Unlock()
}

func (a *AgenticLoop) currentExecutionProfile() tools.ExecutionProfile {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.executionProfile
}

// SetSessionFSM sets the workflow FSM for the current session.
// This must be called before ExecuteWithTools to enable structural guarantees.
func (a *AgenticLoop) SetSessionFSM(fsm *SessionFSM) {
	a.mu.Lock()
	a.sessionFSM = fsm
	a.mu.Unlock()
}

// SetKnowledgeAccumulator sets the knowledge accumulator for fact extraction.
// This must be called before Execute to enable knowledge accumulation.
func (a *AgenticLoop) SetKnowledgeAccumulator(ka *KnowledgeAccumulator) {
	a.mu.Lock()
	a.knowledgeAccumulator = ka
	a.mu.Unlock()
}

// SetMaxTurns overrides the maximum number of agentic turns for this loop.
func (a *AgenticLoop) SetMaxTurns(n int) {
	a.mu.Lock()
	a.maxTurns = n
	a.mu.Unlock()
}

// SetMaxEvidenceCalls bounds model-selected evidence gathering for a Patrol
// investigation. The terminal patrol_propose_action call is not evidence and
// remains available after this budget is exhausted.
func (a *AgenticLoop) SetMaxEvidenceCalls(n int) {
	a.mu.Lock()
	a.maxEvidenceCalls = n
	a.mu.Unlock()
}

// SetProviderInfo sets the provider/model info for telemetry.
func (a *AgenticLoop) SetProviderInfo(provider, model string) {
	a.mu.Lock()
	a.providerName = provider
	a.modelName = model
	a.mu.Unlock()
}

// SetExecutionID binds a stable higher-level execution identifier to all
// provider calls made by this loop so Patrol traces one logical run across
// agentic turns.
func (a *AgenticLoop) SetExecutionID(executionID string) {
	a.mu.Lock()
	a.executionID = executionID
	a.mu.Unlock()
}

// SetBudgetChecker sets a function called after each agentic turn to enforce
// token spending limits. If the checker returns an error, the loop stops.
func (a *AgenticLoop) SetBudgetChecker(fn func() error) {
	a.budgetChecker = fn
}

// GetTotalInputTokens returns the accumulated input tokens across all turns.
func (a *AgenticLoop) GetTotalInputTokens() int {
	return a.totalInputTokens
}

// GetTotalOutputTokens returns the accumulated output tokens across all turns.
func (a *AgenticLoop) GetTotalOutputTokens() int {
	return a.totalOutputTokens
}

// GetTotalToolCalls returns the accepted model-selected tool call count across all turns.
func (a *AgenticLoop) GetTotalToolCalls() int {
	return a.totalToolCalls
}

// GetTotalModelTurns returns completed provider responses for this loop.
func (a *AgenticLoop) GetTotalModelTurns() int {
	return a.totalModelTurns
}

// GetTotalEvidenceCalls returns model-selected Patrol investigation calls
// other than the terminal typed action proposal.
func (a *AgenticLoop) GetTotalEvidenceCalls() int {
	return a.totalEvidenceCalls
}

// ResetTokenCounts resets the accumulated token counts (for reuse across executions).
func (a *AgenticLoop) ResetTokenCounts() {
	a.totalInputTokens = 0
	a.totalOutputTokens = 0
	a.totalToolCalls = 0
}
