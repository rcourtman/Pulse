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
Commands may execute without per-command approval when policy allows. Decide whether current
context is enough, whether read-only evidence is needed, or whether a state-changing tool is
appropriate. Prefer current evidence before changing state.`
	} else {
		modeContext = `
EXECUTION MODE: Controlled
State-changing tools require governed approval when policy says approval is required. If the
user asks you to perform an action, choose the appropriate tool and Pulse will handle any
required approval prompt.`
	}

	prompt := a.baseSystemPrompt + modeContext

	// Append accumulated knowledge facts to system prompt
	if ka := a.knowledgeAccumulator; ka != nil && ka.Len() > 0 {
		prompt += "\n\n" + ka.Render()
	}

	return prompt
}
