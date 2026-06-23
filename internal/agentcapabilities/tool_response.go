package agentcapabilities

import (
	"encoding/json"
	"strings"
)

const (
	ErrCodeFSMBlocked                  = "FSM_BLOCKED"
	ErrCodeStrictResolution            = "STRICT_RESOLUTION"
	ErrCodeRoutingMismatch             = "ROUTING_MISMATCH"
	ErrCodeExecutionContextUnavailable = "EXECUTION_CONTEXT_UNAVAILABLE"
	ErrCodeNotFound                    = "NOT_FOUND"
	ErrCodeActionNotAllowed            = "ACTION_NOT_ALLOWED"
	ErrCodePolicyBlocked               = "POLICY_BLOCKED"
	ErrCodeApprovalRequired            = "APPROVAL_REQUIRED"
	ErrCodeInvalidInput                = "INVALID_INPUT"
	ErrCodeExecutionFailed             = "EXECUTION_FAILED"
	ErrCodeNoAgent                     = "NO_AGENT"
)

// ToolResponse is the structured tool-result envelope used when a tool needs
// machine-readable success/failure fields inside the shared text result
// wrapper. It is shared so Assistant tools and agent-facing adapters branch on
// the same blocked/failed/retryable vocabulary.
type ToolResponse struct {
	OK    bool           `json:"ok"`
	Data  any            `json:"data,omitempty"`
	Error *ToolError     `json:"error,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

// ToolError provides the structured failure shape for ToolResponse.
type ToolError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Blocked   bool           `json:"blocked,omitempty"`
	Failed    bool           `json:"failed,omitempty"`
	Retryable bool           `json:"retryable,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// NewToolBlockedError creates a policy or validation blocked tool response.
func NewToolBlockedError(code, message string, details map[string]any) ToolResponse {
	return ToolResponse{
		OK: false,
		Error: &ToolError{
			Code:    code,
			Message: message,
			Blocked: true,
			Details: details,
		},
	}
}

// NewToolResponseResult wraps a structured ToolResponse in the shared tool
// result envelope while preserving failure state through isError.
func NewToolResponseResult(resp ToolResponse) ToolResult {
	return NewToolJSONResultWithIsError(resp, !resp.OK)
}

func toolResultJSONObject(resultText string) (map[string]any, bool) {
	if resultText == "" {
		return nil, false
	}
	jsonStart := strings.Index(resultText, "{")
	if jsonStart == -1 {
		return nil, false
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(resultText[jsonStart:]), &obj); err != nil {
		return nil, false
	}
	return obj, true
}

// ToolResultErrorCode extracts the shared ToolResponse error code from a tool
// result text. It also accepts the legacy top-level error_code shape so old
// persisted transcripts and compatibility paths branch through the same helper.
func ToolResultErrorCode(resultText string) (string, bool) {
	obj, ok := toolResultJSONObject(resultText)
	if !ok {
		return "", false
	}
	if errorAny, ok := obj["error"]; ok {
		if errorMap, ok := errorAny.(map[string]any); ok {
			if code, ok := errorMap["code"].(string); ok && strings.TrimSpace(code) != "" {
				return code, true
			}
		}
	}
	if code, ok := obj["error_code"].(string); ok && strings.TrimSpace(code) != "" {
		return code, true
	}
	return "", false
}

// ToolResultHasErrorCode reports whether a tool result text carries the given
// shared ToolResponse error code.
func ToolResultHasErrorCode(resultText, code string) bool {
	got, ok := ToolResultErrorCode(resultText)
	return ok && got == code
}

// ToolResultHasVerificationOK checks whether a tool result text includes
// structured verification evidence indicating a mutation was confirmed.
//
// Expected shape, possibly after leading human-readable text:
//
//	{ "verification": { "ok": true, ... } }
func ToolResultHasVerificationOK(resultText string) bool {
	obj, ok := toolResultJSONObject(resultText)
	if !ok {
		return false
	}
	vAny, ok := obj["verification"]
	if !ok {
		return false
	}
	vMap, ok := vAny.(map[string]any)
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
