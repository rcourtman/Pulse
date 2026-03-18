package chat

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

// SetProviderInfo sets the provider/model info for telemetry.
func (a *AgenticLoop) SetProviderInfo(provider, model string) {
	a.mu.Lock()
	a.providerName = provider
	a.modelName = model
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

// ResetTokenCounts resets the accumulated token counts (for reuse across executions).
func (a *AgenticLoop) ResetTokenCounts() {
	a.totalInputTokens = 0
	a.totalOutputTokens = 0
}
