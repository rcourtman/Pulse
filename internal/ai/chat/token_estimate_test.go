package chat

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

func TestEstimateTokens(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		if got := EstimateTokens(""); got != 0 {
			t.Fatalf("EstimateTokens(\"\") = %d, want 0", got)
		}
	})

	t.Run("known text", func(t *testing.T) {
		text := "hello world"
		want := 3 // 11 chars ~= ceil(11/4)
		if got := EstimateTokens(text); got != want {
			t.Fatalf("EstimateTokens(%q) = %d, want %d", text, got, want)
		}
	})
}

func TestEstimateMessagesTokens(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		if got := EstimateMessagesTokens(nil); got != 0 {
			t.Fatalf("EstimateMessagesTokens(nil) = %d, want 0", got)
		}
		if got := EstimateMessagesTokens([]providers.Message{}); got != 0 {
			t.Fatalf("EstimateMessagesTokens(empty) = %d, want 0", got)
		}
	})

	t.Run("message content", func(t *testing.T) {
		msgs := []providers.Message{
			{Role: "user", Content: "hello world"},
		}

		want := messageTokenOverhead + EstimateTokens("hello world")
		if got := EstimateMessagesTokens(msgs); got != want {
			t.Fatalf("EstimateMessagesTokens(content) = %d, want %d", got, want)
		}
	})

	t.Run("tool calls and tool results", func(t *testing.T) {
		callInput := map[string]interface{}{
			"query": "status",
			"limit": 3,
		}
		callInputJSON, err := json.Marshal(callInput)
		if err != nil {
			t.Fatalf("marshal call input: %v", err)
		}

		msgs := []providers.Message{
			{
				Role:             "assistant",
				Content:          "Investigating now.",
				ReasoningContent: "Need current VM status.",
				ToolCalls: []providers.ToolCall{
					{
						ID:    "call-1",
						Name:  "pulse_query",
						Input: callInput,
					},
				},
			},
			{
				Role: "user",
				ToolResult: &providers.ToolResult{
					ToolUseID: "call-1",
					Content:   "vm-101 is running",
				},
			},
		}

		want := 0
		want += messageTokenOverhead + EstimateTokens("Investigating now.") + EstimateTokens("Need current VM status.")
		want += EstimateTokens("pulse_query")
		want += EstimateTokens(string(callInputJSON))
		want += messageTokenOverhead + EstimateTokens("vm-101 is running")

		if got := EstimateMessagesTokens(msgs); got != want {
			t.Fatalf("EstimateMessagesTokens(tooling) = %d, want %d", got, want)
		}
	})
}

func TestEstimateToolsTokens(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		if got := EstimateToolsTokens(nil); got != 0 {
			t.Fatalf("EstimateToolsTokens(nil) = %d, want 0", got)
		}
		if got := EstimateToolsTokens([]providers.Tool{}); got != 0 {
			t.Fatalf("EstimateToolsTokens(empty) = %d, want 0", got)
		}
	})

	t.Run("tools with schemas", func(t *testing.T) {
		tools := []providers.Tool{
			{
				Name:        "pulse_query",
				Description: "Query resources",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"query"},
				},
			},
		}

		toolJSON, err := json.Marshal(tools[0])
		if err != nil {
			t.Fatalf("marshal tool: %v", err)
		}
		want := EstimateTokens(string(toolJSON))

		if got := EstimateToolsTokens(tools); got != want {
			t.Fatalf("EstimateToolsTokens(tools) = %d, want %d", got, want)
		}
	})
}

func TestEstimateRequestTokens(t *testing.T) {
	callInput := map[string]interface{}{"target": "vm-101"}
	msgs := []providers.Message{
		{Role: "user", Content: "Check VM status"},
		{
			Role: "assistant",
			ToolCalls: []providers.ToolCall{
				{ID: "call-1", Name: "pulse_query", Input: callInput},
			},
		},
	}
	tools := []providers.Tool{
		{
			Name: "pulse_query",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
	}
	req := providers.ChatRequest{
		System:   "You are a helpful assistant.",
		Messages: msgs,
		Tools:    tools,
	}

	want := EstimateTokens(req.System) + EstimateMessagesTokens(msgs) + EstimateToolsTokens(tools)
	if got := EstimateRequestTokens(req); got != want {
		t.Fatalf("EstimateRequestTokens(req) = %d, want %d", got, want)
	}
}
