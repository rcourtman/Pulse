package chat

import (
	"encoding/json"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func formatToolInputForFrontend(toolName string, input map[string]interface{}, emptyIfNil bool) (inputStr string, rawInput string) {
	// Canonical exposure projection: every user-visible or durable view
	// of tool-call arguments routes through here, so restricted values
	// (proposal params) never leave the transient provider/validation
	// path.
	input = agentcapabilities.RedactToolCallArgumentsForExposure(toolName, input)
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
	if toolName == agentcapabilities.PulseControlToolName || toolName == agentcapabilities.PulseRunCommandToolName || toolName == "control" {
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
