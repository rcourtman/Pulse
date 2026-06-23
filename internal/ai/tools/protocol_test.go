package tools

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestAssistantToolResultWrapsSharedEnvelope(t *testing.T) {
	var content agentcapabilities.ToolContent = NewTextContent("ok")
	if content.Type != "text" || content.Text != "ok" {
		t.Fatalf("NewTextContent returned unexpected shared content: %+v", content)
	}

	result := NewJSONResultWithIsError(map[string]string{"error": "policy_blocked"}, true)
	var shared agentcapabilities.ToolResult = result
	if !shared.IsError {
		t.Fatal("Assistant JSON tool result must preserve shared isError=true")
	}
	if len(shared.Content) != 1 || !strings.Contains(shared.Content[0].Text, `"error":"policy_blocked"`) {
		t.Fatalf("Assistant JSON tool result did not use shared content envelope: %+v", shared)
	}
}

func TestAssistantCallToolParamsUseSharedAgentCapabilityTypes(t *testing.T) {
	var params agentcapabilities.ToolCallParams = CallToolParams{
		Name:      "pulse_read",
		Arguments: map[string]any{"resource_id": "vm:101"},
	}
	if params.Name != "pulse_read" || params.Arguments["resource_id"] != "vm:101" {
		t.Fatalf("CallToolParams must alias shared call params: %+v", params)
	}
}

func TestAssistantToolResponseUsesSharedAgentCapabilityEnvelope(t *testing.T) {
	var shared agentcapabilities.ToolResponse = ToolResponse{
		OK: false,
		Error: &ToolError{
			Code:    ErrCodeRoutingMismatch,
			Message: "choose the child resource",
			Blocked: true,
		},
	}
	if shared.Error == nil || shared.Error.Code != agentcapabilities.ErrCodeRoutingMismatch {
		t.Fatalf("ToolResponse must alias shared agent capability envelope: %+v", shared)
	}
	if ErrCodeFSMBlocked != agentcapabilities.ErrCodeFSMBlocked {
		t.Fatalf("FSM-blocked error code = %q, want shared %q", ErrCodeFSMBlocked, agentcapabilities.ErrCodeFSMBlocked)
	}
	if ErrCodeExecutionContextUnavailable != agentcapabilities.ErrCodeExecutionContextUnavailable {
		t.Fatalf("execution-context error code = %q, want shared %q", ErrCodeExecutionContextUnavailable, agentcapabilities.ErrCodeExecutionContextUnavailable)
	}

	result := NewToolResponseResult(NewToolBlockedError(ErrCodeActionNotAllowed, "not allowed", nil))
	var toolResult agentcapabilities.ToolResult = result
	if !toolResult.IsError {
		t.Fatal("blocked shared ToolResponse must preserve isError=true")
	}
	if len(toolResult.Content) != 1 || !strings.Contains(toolResult.Content[0].Text, `"code":"ACTION_NOT_ALLOWED"`) {
		t.Fatalf("blocked ToolResponse was not encoded through shared result: %+v", toolResult)
	}
}

func TestAssistantToolSchemaUsesSharedAgentCapabilityTypes(t *testing.T) {
	var shared agentcapabilities.Tool = Tool{
		Name: "pulse_read",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"resourceId": {Type: "string"},
			},
			Required: []string{"resourceId"},
		},
	}

	if shared.InputSchema.Properties["resourceId"].Type != "string" {
		t.Fatalf("Assistant Tool must alias shared agent capability schema: %+v", shared)
	}
}
