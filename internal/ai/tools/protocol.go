package tools

import "github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"

// Tool describes an available Assistant registry tool. The shape aliases the
// shared Pulse Intelligence tool schema so provider projection and external
// agent adapters cannot drift.
type Tool = agentcapabilities.Tool

// InputSchema describes the expected input for a tool.
type InputSchema = agentcapabilities.InputSchema

// PropertySchema describes a property in the input schema.
type PropertySchema = agentcapabilities.PropertySchema

// CallToolParams are the shared params for registry tool calls.
type CallToolParams = agentcapabilities.ToolCallParams

// CallToolResult is the shared result envelope returned by Assistant registry
// tools.
type CallToolResult = agentcapabilities.ToolResult

// Content represents content in a shared tool result.
type Content = agentcapabilities.ToolContent

// ToolResponse is the shared structured tool-result envelope used for
// machine-readable blocked/failed Assistant registry tool outcomes.
type ToolResponse = agentcapabilities.ToolResponse

// ToolError is the shared structured failure shape for ToolResponse.
type ToolError = agentcapabilities.ToolError

// Common error codes
const (
	ErrCodeFSMBlocked                  = agentcapabilities.ErrCodeFSMBlocked
	ErrCodeStrictResolution            = agentcapabilities.ErrCodeStrictResolution
	ErrCodeRoutingMismatch             = agentcapabilities.ErrCodeRoutingMismatch
	ErrCodeExecutionContextUnavailable = agentcapabilities.ErrCodeExecutionContextUnavailable
	ErrCodeNotFound                    = agentcapabilities.ErrCodeNotFound
	ErrCodeActionNotAllowed            = agentcapabilities.ErrCodeActionNotAllowed
	ErrCodePolicyBlocked               = agentcapabilities.ErrCodePolicyBlocked
	ErrCodeApprovalRequired            = agentcapabilities.ErrCodeApprovalRequired
	ErrCodeInvalidInput                = agentcapabilities.ErrCodeInvalidInput
	ErrCodeExecutionFailed             = agentcapabilities.ErrCodeExecutionFailed
	ErrCodeNoAgent                     = agentcapabilities.ErrCodeNoAgent
)

// NewToolBlockedError creates a policy/validation blocked error
func NewToolBlockedError(code, message string, details map[string]interface{}) ToolResponse {
	return agentcapabilities.NewToolBlockedError(code, message, details)
}

// Helper functions

// NewTextContent creates a text content object
func NewTextContent(text string) Content {
	return agentcapabilities.NewToolTextContent(text)
}

// NewErrorResult creates an error tool result
func NewErrorResult(err error) CallToolResult {
	return agentcapabilities.NewToolErrorResult(err)
}

// NewTextResult creates a successful text tool result
func NewTextResult(text string) CallToolResult {
	return agentcapabilities.NewToolTextResult(text)
}

// NewJSONResult creates a successful JSON tool result
// The data is marshaled to JSON and returned as text content
func NewJSONResult(data interface{}) CallToolResult {
	return agentcapabilities.NewToolJSONResult(data)
}

// NewJSONResultWithIsError creates a JSON tool result with explicit error state.
// Use this when you want to return structured JSON to the model/UI while still
// signalling a failure to the agentic loop (IsError=true).
func NewJSONResultWithIsError(data interface{}, isError bool) CallToolResult {
	return agentcapabilities.NewToolJSONResultWithIsError(data, isError)
}

// NewToolResponseResult creates a CallToolResult from a ToolResponse
// This provides the consistent shared tool-result envelope.
func NewToolResponseResult(resp ToolResponse) CallToolResult {
	return agentcapabilities.NewToolResponseResult(resp)
}
