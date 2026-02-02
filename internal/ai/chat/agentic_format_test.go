package chat

import (
	"strings"
	"testing"
)

func TestFormatKeyParams_PriorityAndFallback(t *testing.T) {
	input := map[string]interface{}{"action": "restart", "resource_id": "vm-1", "other": "x"}
	got := formatKeyParams(input)
	if !strings.Contains(got, "action=restart") || !strings.Contains(got, "resource_id=vm-1") {
		t.Fatalf("expected priority params, got %q", got)
	}

	fallback := map[string]interface{}{"foo": "bar", "count": 2}
	got = formatKeyParams(fallback)
	if got == "" || !strings.Contains(got, "foo=bar") {
		t.Fatalf("expected fallback params, got %q", got)
	}

	if formatKeyParams(nil) != "" {
		t.Fatalf("expected empty result for nil input")
	}
}

func TestConvertToProviderMessages_TruncatesToolResult(t *testing.T) {
	longContent := strings.Repeat("a", MaxToolResultCharsLimit+1)

	messages := []Message{{
		Role:    "assistant",
		Content: "ok",
		ToolCalls: []ToolCall{{
			ID:   "tool-1",
			Name: "pulse_query",
			Input: map[string]interface{}{
				"action": "health",
			},
		}},
		ToolResult: &ToolResult{ToolUseID: "tool-1", Content: longContent, IsError: false},
	}}

	converted := convertToProviderMessages(messages)
	if len(converted) != 1 {
		t.Fatalf("expected 1 message, got %d", len(converted))
	}
	if len(converted[0].ToolCalls) != 1 || converted[0].ToolCalls[0].Name != "pulse_query" {
		t.Fatalf("expected tool call to be preserved")
	}
	if converted[0].ToolResult == nil || !strings.Contains(converted[0].ToolResult.Content, "TRUNCATED") {
		t.Fatalf("expected truncated tool result, got %+v", converted[0].ToolResult)
	}
	if converted[0].Role != "assistant" {
		t.Fatalf("expected role to be preserved")
	}
}
