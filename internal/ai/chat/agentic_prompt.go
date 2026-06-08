package chat

import (
	"fmt"
	"time"
)

// getSystemPrompt builds the full system prompt including the current mode context.
// This is called at request time so the prompt reflects the current mode and the
// current wall-clock time. The base prompt is frozen when the loop is created at
// service start, so anything that must stay fresh per turn (mode, current time)
// is appended here rather than baked into baseSystemPrompt.
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

	// Give the model the current time directly. Without this the Assistant has no
	// clock in context and deflects time/date questions ("I don't have access to a
	// real-time clock", "tell me a target host and I'll run `date`") even in
	// autonomous mode. The wall-clock value is the Pulse server clock and carries no
	// PII, so it is safe to share with cloud-routed models. Formatted per turn so a
	// long-lived session stays current rather than freezing at service-start time.
	currentTime := fmt.Sprintf(`
CURRENT TIME: %s (Pulse server clock).
Treat this as the current date and time. Answer "what time is it" / "what's the date" style
questions directly from this value — do not run a command or ask for a target host just to
report the current time.`, time.Now().Format("Mon, 02 Jan 2006 15:04:05 MST"))

	prompt := a.baseSystemPrompt + modeContext + currentTime

	// Append accumulated knowledge facts to system prompt
	if ka := a.knowledgeAccumulator; ka != nil && ka.Len() > 0 {
		prompt += "\n\n" + ka.Render()
	}

	return prompt
}
