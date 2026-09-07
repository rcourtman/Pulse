package chat

import "github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"

// ToolKind classifies tool calls for canonical invocation permissions.
type ToolKind = agentcapabilities.ToolCallKind

const (
	// ToolKindResolve - discovery/query tools that find resources
	ToolKindResolve = agentcapabilities.ToolCallKindResolve

	// ToolKindRead - read-only tools (logs, metrics, status, config)
	ToolKindRead = agentcapabilities.ToolCallKindRead

	// ToolKindWrite - mutating tools (restart, stop, start, delete, file write)
	ToolKindWrite = agentcapabilities.ToolCallKindWrite

	// ToolKindUserInput - interactive tools that request user input
	ToolKindUserInput = agentcapabilities.ToolCallKindUserInput
)

// ClassifyToolCall classifies a tool call using the shared invocation contract.
func ClassifyToolCall(toolName string, args map[string]interface{}) ToolKind {
	return agentcapabilities.ClassifyToolCall(toolName, args)
}
