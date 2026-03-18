package chat

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

// maybeInjectWrapUpNudge appends a system hint to the last non-error tool result in
// providerMessages when totalCalls exceeds the threshold. This nudges the model to
// start wrapping up without forcing text-only mode.
// Returns true if a nudge was injected.
func maybeInjectWrapUpNudge(messages []providers.Message, totalCalls, maxTurns, currentTurn, threshold int) bool {
	if totalCalls < threshold {
		return false
	}

	turnsRemaining := maxTurns - currentTurn - 1
	nudge := fmt.Sprintf("\n\n[System: You have made %d tool calls (%d turns remaining). You likely have enough data to answer. Start forming your response. You may make 1-2 more targeted calls if critical information is missing, but avoid exploratory calls.]",
		totalCalls, turnsRemaining)

	// Find the last non-error tool result in messages and append the nudge
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].ToolResult != nil && !messages[i].ToolResult.IsError {
			messages[i].ToolResult.Content += nudge
			log.Info().
				Int("total_calls", totalCalls).
				Int("turns_remaining", turnsRemaining).
				Int("message_index", i).
				Msg("[WrapUpNudge] Injected wrap-up nudge into tool result")
			return true
		}
	}
	return false
}

// maybeInjectWrapUpEscalation appends a strong wrap-up directive to the last non-error
// tool result. Called once when the model ignores the initial nudge and reaches 18+ calls.
// Returns true if an escalation was injected.
func maybeInjectWrapUpEscalation(messages []providers.Message, totalCalls int) bool {
	escalation := fmt.Sprintf("\n\n[System: WRAP UP NOW. You have made %d tool calls — well past the recommended limit. You MUST respond with your findings on this turn. Do NOT make any more tool calls.]",
		totalCalls)

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].ToolResult != nil && !messages[i].ToolResult.IsError {
			messages[i].ToolResult.Content += escalation
			log.Info().
				Int("total_calls", totalCalls).
				Int("message_index", i).
				Msg("[WrapUpEscalation] Injected wrap-up escalation into tool result")
			return true
		}
	}
	return false
}
