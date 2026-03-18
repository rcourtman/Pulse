package chat

import (
	"encoding/json"
	"fmt"
)

func formatToolInputForFrontend(toolName string, input map[string]interface{}, emptyIfNil bool) (inputStr string, rawInput string) {
	if input == nil {
		if emptyIfNil {
			return "", ""
		}
		return "{}", ""
	}

	if inputBytes, err := json.Marshal(input); err == nil {
		rawInput = string(inputBytes)
	}

	// Special handling for command execution tools to avoid showing raw JSON.
	if toolName == "pulse_control" || toolName == "pulse_run_command" || toolName == "control" {
		if cmd, ok := input["command"].(string); ok {
			return fmt.Sprintf("Running: %s", cmd), rawInput
		}
		if action, ok := input["action"].(string); ok {
			target := ""
			if t, ok := input["guest_id"].(string); ok {
				target = t
			} else if t, ok := input["container"].(string); ok {
				target = t
			}
			return fmt.Sprintf("%s %s", action, target), rawInput
		}
	}

	if inputBytes, err := json.Marshal(input); err == nil {
		return string(inputBytes), rawInput
	}
	return "{}", rawInput
}
