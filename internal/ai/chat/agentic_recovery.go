package chat

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

type autoRecoveryPlan struct {
	ErrorCode string
	ToolName  string
	Input     map[string]interface{}
}

type autoRecoveryDetails struct {
	AutoRecoverable   bool                   `json:"auto_recoverable"`
	SuggestedRewrite  string                 `json:"suggested_rewrite"`
	SuggestedTool     string                 `json:"suggested_tool"`
	SuggestedArgument map[string]interface{} `json:"suggested_arguments"`
}

// tryAutoRecovery checks if a tool result is auto-recoverable and returns a retry plan.
// Returns (plan, alreadyAttempted) where:
// - plan is nil when no auto-recovery should be attempted
// - alreadyAttempted is true if auto-recovery was already attempted for this call
func tryAutoRecovery(result tools.CallToolResult, tc providers.ToolCall, executor *tools.PulseToolExecutor, ctx context.Context) (*autoRecoveryPlan, bool) {
	// Check if this is already a recovery attempt
	if _, ok := tc.Input["_auto_recovery_attempt"]; ok {
		return nil, true // Already attempted, don't retry again
	}

	// Parse the result to check for auto_recoverable flag
	resultStr := FormatToolResult(result)

	// Look for the structured error response pattern
	// The result should contain JSON with auto_recoverable and suggested_rewrite
	if !strings.Contains(resultStr, `"auto_recoverable"`) {
		return nil, false
	}

	// Extract the JSON portion from the result
	// Results are formatted as "Error: {json}" or just "{json}"
	jsonStart := strings.Index(resultStr, "{")
	if jsonStart == -1 {
		return nil, false
	}

	var parsed struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				AutoRecoverable   bool                   `json:"auto_recoverable"`
				SuggestedRewrite  string                 `json:"suggested_rewrite"`
				SuggestedTool     string                 `json:"suggested_tool"`
				SuggestedArgument map[string]interface{} `json:"suggested_arguments"`
				Category          string                 `json:"category"`
			} `json:"details"`
		} `json:"error"`
		AutoRecoverable   bool                   `json:"auto_recoverable"`
		SuggestedRewrite  string                 `json:"suggested_rewrite"`
		SuggestedTool     string                 `json:"suggested_tool"`
		SuggestedArgument map[string]interface{} `json:"suggested_arguments"`
	}

	if err := json.Unmarshal([]byte(resultStr[jsonStart:]), &parsed); err != nil {
		// Try alternative format where details are at top level
		var altParsed struct {
			ErrorCode          string                 `json:"error_code"`
			AutoRecoverable    bool                   `json:"auto_recoverable"`
			SuggestedRewrite   string                 `json:"suggested_rewrite"`
			SuggestedTool      string                 `json:"suggested_tool"`
			SuggestedArguments map[string]interface{} `json:"suggested_arguments"`
		}
		if err2 := json.Unmarshal([]byte(resultStr[jsonStart:]), &altParsed); err2 != nil {
			return nil, false
		}
		return buildAutoRecoveryPlan(tc, altParsed.ErrorCode, autoRecoveryDetails{
			AutoRecoverable:   altParsed.AutoRecoverable,
			SuggestedRewrite:  altParsed.SuggestedRewrite,
			SuggestedTool:     altParsed.SuggestedTool,
			SuggestedArgument: altParsed.SuggestedArguments,
		}), false
	}

	if plan := buildAutoRecoveryPlan(tc, parsed.Error.Code, autoRecoveryDetails{
		AutoRecoverable:   parsed.Error.Details.AutoRecoverable,
		SuggestedRewrite:  parsed.Error.Details.SuggestedRewrite,
		SuggestedTool:     parsed.Error.Details.SuggestedTool,
		SuggestedArgument: parsed.Error.Details.SuggestedArgument,
	}); plan != nil {
		return plan, false
	}

	return buildAutoRecoveryPlan(tc, parsed.Error.Code, autoRecoveryDetails{
		AutoRecoverable:   parsed.AutoRecoverable,
		SuggestedRewrite:  parsed.SuggestedRewrite,
		SuggestedTool:     parsed.SuggestedTool,
		SuggestedArgument: parsed.SuggestedArgument,
	}), false
}

func buildAutoRecoveryPlan(tc providers.ToolCall, errorCode string, details autoRecoveryDetails) *autoRecoveryPlan {
	if !details.AutoRecoverable {
		return nil
	}

	if suggestedTool := strings.TrimSpace(details.SuggestedTool); suggestedTool != "" {
		input := cloneAutoRecoveryInput(details.SuggestedArgument)
		input["_auto_recovery_attempt"] = true
		return &autoRecoveryPlan{
			ErrorCode: firstNonEmptyAutoRecoveryCode(errorCode),
			ToolName:  suggestedTool,
			Input:     input,
		}
	}

	if suggestedRewrite := strings.TrimSpace(details.SuggestedRewrite); suggestedRewrite != "" {
		input := cloneAutoRecoveryInput(tc.Input)
		input["command"] = suggestedRewrite
		input["_auto_recovery_attempt"] = true
		return &autoRecoveryPlan{
			ErrorCode: firstNonEmptyAutoRecoveryCode(errorCode),
			ToolName:  tc.Name,
			Input:     input,
		}
	}

	return nil
}

func cloneAutoRecoveryInput(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(input)+1)
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func firstNonEmptyAutoRecoveryCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "AUTO_RECOVERY"
	}
	return code
}
