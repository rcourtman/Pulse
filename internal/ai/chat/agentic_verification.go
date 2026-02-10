package chat

import (
	"encoding/json"
	"strings"
)

// truncateForLog truncates a string for logging, adding "..." if truncated.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// toolResultHasVerificationOK checks whether a tool output includes structured verification
// evidence indicating the mutation was confirmed.
//
// Expected shape (JSON text):
//
//	{ "verification": { "ok": true, ... } }
func toolResultHasVerificationOK(resultText string) bool {
	if resultText == "" {
		return false
	}
	jsonStart := strings.Index(resultText, "{")
	if jsonStart == -1 {
		return false
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(resultText[jsonStart:]), &obj); err != nil {
		return false
	}
	vAny, ok := obj["verification"]
	if !ok {
		return false
	}
	vMap, ok := vAny.(map[string]interface{})
	if !ok {
		return false
	}
	okAny, ok := vMap["ok"]
	if !ok {
		return false
	}
	okBool, ok := okAny.(bool)
	return ok && okBool
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
