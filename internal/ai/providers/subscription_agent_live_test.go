package providers

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSubscriptionAgentLive is an opt-in local qualification probe. It proves
// that the installed subscription CLI can authenticate and return a governed
// Pulse tool selection without executing that tool. It is excluded from normal
// CI because it depends on an interactive user's local plan and model access.
func TestSubscriptionAgentLive(t *testing.T) {
	if os.Getenv("PULSE_TEST_SUBSCRIPTION_AGENTS") != "1" {
		t.Skip("set PULSE_TEST_SUBSCRIPTION_AGENTS=1 to exercise local subscription logins")
	}
	tests := []struct {
		name  string
		agent SubscriptionAgent
		model string
	}{
		{name: "codex", agent: SubscriptionAgentCodex, model: envOrDefault("PULSE_TEST_CODEX_MODEL", "gpt-5.6-luna")},
		{name: "claude", agent: SubscriptionAgentClaude, model: envOrDefault("PULSE_TEST_CLAUDE_MODEL", "sonnet")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			client := NewSubscriptionAgentClient(tt.agent, tt.model, 2*time.Minute)
			if err := client.TestConnection(ctx); err != nil {
				t.Fatalf("authentication readiness failed: %v", err)
			}
			var toolStarts int
			var done DoneEvent
			err := client.ChatStream(ctx, ChatRequest{
				Model:    string(tt.agent) + ":" + tt.model,
				System:   "Select the supplied observation tool exactly once. Do not claim that it ran.",
				Messages: []Message{{Role: "user", Content: "Inspect node tower using the supplied Pulse tool."}},
				Tools: []Tool{{Name: "get_node_status", Description: "Read the current status of one node", InputSchema: map[string]interface{}{
					"type": "object", "additionalProperties": false, "required": []string{"node"}, "properties": map[string]interface{}{"node": map[string]interface{}{"type": "string"}},
				}}},
				ToolChoice: &ToolChoice{Type: ToolChoiceRequired},
			}, func(event StreamEvent) {
				switch event.Type {
				case "tool_start":
					toolStarts++
				case "done":
					done, _ = event.Data.(DoneEvent)
				}
			})
			if err != nil {
				t.Fatalf("structured streaming tool-selection turn failed: %v", err)
			}
			if toolStarts != 1 || len(done.ToolCalls) != 1 || done.ToolCalls[0].Name != "get_node_status" {
				t.Fatalf("unexpected structured stream: starts=%d done=%#v", toolStarts, done)
			}
		})
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
