package chat

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// tryAutoRecovery checks if a tool result is auto-recoverable and returns the suggested rewrite.
// Returns (suggestedRewrite, alreadyAttempted) where:
// - suggestedRewrite is the command to retry with (empty if not recoverable)
// - alreadyAttempted is true if auto-recovery was already attempted for this call
func tryAutoRecovery(result tools.CallToolResult, tc providers.ToolCall, executor *tools.PulseToolExecutor, ctx context.Context) (string, bool) {
	// Check if this is already a recovery attempt
	if _, ok := tc.Input["_auto_recovery_attempt"]; ok {
		return "", true // Already attempted, don't retry again
	}

	// Parse the result to check for auto_recoverable flag
	resultStr := FormatToolResult(result)

	// Look for the structured error response pattern
	// The result should contain JSON with auto_recoverable and suggested_rewrite
	if !strings.Contains(resultStr, `"auto_recoverable"`) {
		return "", false
	}

	// Extract the JSON portion from the result
	// Results are formatted as "Error: {json}" or just "{json}"
	jsonStart := strings.Index(resultStr, "{")
	if jsonStart == -1 {
		return "", false
	}

	var parsed struct {
		Error struct {
			Details struct {
				AutoRecoverable  bool   `json:"auto_recoverable"`
				SuggestedRewrite string `json:"suggested_rewrite"`
				Category         string `json:"category"`
			} `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(resultStr[jsonStart:]), &parsed); err != nil {
		// Try alternative format where details are at top level
		var altParsed struct {
			AutoRecoverable  bool   `json:"auto_recoverable"`
			SuggestedRewrite string `json:"suggested_rewrite"`
		}
		if err2 := json.Unmarshal([]byte(resultStr[jsonStart:]), &altParsed); err2 != nil {
			return "", false
		}
		if altParsed.AutoRecoverable && altParsed.SuggestedRewrite != "" {
			return altParsed.SuggestedRewrite, false
		}
		return "", false
	}

	if parsed.Error.Details.AutoRecoverable && parsed.Error.Details.SuggestedRewrite != "" {
		return parsed.Error.Details.SuggestedRewrite, false
	}

	return "", false
}
