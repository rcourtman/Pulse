package chat

// getSystemPrompt builds the full system prompt including the current mode context.
// This is called at request time so the prompt reflects the current mode.
func (a *AgenticLoop) getSystemPrompt() string {
	a.mu.Lock()
	isAutonomous := a.autonomousMode
	a.mu.Unlock()

	var modeContext string
	if isAutonomous {
		modeContext = `
EXECUTION MODE: Autonomous
Commands execute immediately without user approval. Follow the Discover → Investigate → Act
workflow. Gather information before taking action. Use the tools freely to explore logs, check
status, and understand the situation before attempting fixes.`
	} else {
		modeContext = `
EXECUTION MODE: Controlled
Commands require user approval before execution. The system handles this automatically via a
confirmation prompt - you don't need to ask "Would you like me to...?" Just execute what's
needed and the system will prompt the user to approve if required.`
	}

	prompt := a.baseSystemPrompt + modeContext

	// Append accumulated knowledge facts to system prompt
	if ka := a.knowledgeAccumulator; ka != nil && ka.Len() > 0 {
		prompt += "\n\n" + ka.Render()
	}

	return prompt
}
