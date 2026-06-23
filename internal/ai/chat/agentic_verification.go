package chat

import (
	"encoding/json"
)

// truncateForLog truncates a string for logging, adding "..." if truncated.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// toolCallKey returns a string key for a tool call (name + serialized input)
// used to detect repeated identical calls in the agentic loop.
func toolCallKey(name string, input map[string]interface{}) string {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return name
	}
	return name + ":" + string(inputBytes)
}

// getCommandFromInput extracts the command from tool input for logging.
func getCommandFromInput(input map[string]interface{}) string {
	if cmd, ok := input["command"].(string); ok {
		return cmd
	}
	return "<unknown>"
}
