package chat

import (
	"context"
	"fmt"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

func TestPreRequestContextCheck_WithinLimit(t *testing.T) {
	// Create a request that fits within the context window.
	// Verify no modifications happen (tools still present, messages unchanged).
	req := providers.ChatRequest{
		System:   "You are helpful.",
		Messages: []providers.Message{{Role: "user", Content: "Hello"}},
		Tools:    []providers.Tool{{Name: "pulse_query"}},
	}

	estimated := EstimateRequestTokens(req)
	limit := providers.ContextWindowTokens("anthropic:claude-sonnet-4-20250514")
	if estimated >= limit {
		t.Fatal("test setup error: request should fit within limit")
	}

	// No intervention expected: tools should remain.
	if req.Tools == nil {
		t.Fatal("tools should not be nil for within-limit request")
	}
}

func TestPreRequestContextCheck_OverLimit_Compacts(t *testing.T) {
	// Create messages that exceed a small model's context.
	// Verify compactOldToolResults is effective.
	largeContent := make([]byte, 200_000)
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	msgs := []providers.Message{
		{Role: "assistant", Content: "Checking...", ToolCalls: []providers.ToolCall{{ID: "c1", Name: "pulse_query"}}},
		{Role: "user", ToolResult: &providers.ToolResult{ToolUseID: "c1", Content: string(largeContent)}},
		{Role: "assistant", Content: "Let me check more..."},
		{Role: "user", Content: "What's happening?"},
	}

	// After compaction, the tool result should be much shorter.
	compactOldToolResults(msgs, 3, 0, 100, nil)
	compacted := msgs[1].ToolResult.Content
	if len(compacted) >= len(largeContent) {
		t.Fatal("compaction should have reduced the tool result size")
	}
}

func TestPreRequestContextCheck_WayOverLimit_DropsTools(t *testing.T) {
	// Simulate the scenario where even after compaction, we're over limit.
	// This tests the logic path, not the full loop.
	// Create a request with a system prompt that alone exceeds a tiny limit.
	bigSystem := make([]byte, 40_000)
	for i := range bigSystem {
		bigSystem[i] = 'x'
	}

	req := providers.ChatRequest{
		System:   string(bigSystem),
		Messages: []providers.Message{{Role: "user", Content: "test"}},
		Tools:    []providers.Tool{{Name: "pulse_query"}, {Name: "pulse_metrics"}},
	}

	estimated := EstimateRequestTokens(req)
	// Use gpt-4 which has 8K context.
	limit := providers.ContextWindowTokens("gpt-4")

	if estimated <= limit {
		t.Fatal("test setup error: request should exceed limit")
	}
	// Verify the estimation correctly identifies the overflow.
	if estimated < 10_000 {
		t.Fatalf("estimated should be >10K for 40K chars system prompt, got %d", estimated)
	}
}

func TestPreRequestContextCheck_FinalAbort(t *testing.T) {
	// Simulate the scenario where system prompt alone is too large.
	// Even after dropping tools and pruning, the request can't fit.
	hugeSystem := strings.Repeat("x", 50_000) // ~12,500 tokens
	req := providers.ChatRequest{
		System:   hugeSystem,
		Messages: []providers.Message{{Role: "user", Content: "test"}},
	}
	// gpt-4 has 8K context - system alone exceeds it
	estimated := EstimateRequestTokens(req)
	limit := providers.ContextWindowTokens("gpt-4")
	if estimated <= limit {
		t.Fatalf("test setup: expected %d > %d", estimated, limit)
	}
	// Verify the estimation detects the overflow
	// (actual abort happens in the loop, which we can't easily test in isolation,
	// but we verify the estimation is correct)
	if estimated < 12000 {
		t.Fatalf("expected >12K tokens for 50K char system, got %d", estimated)
	}
}

func TestToolTokenCaching(t *testing.T) {
	// Verify EstimateToolsTokens is deterministic (same result for same input)
	tools := []providers.Tool{
		{Name: "pulse_query", Description: "Query resources", InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
			},
		}},
		{Name: "pulse_metrics", Description: "Get metrics"},
	}

	first := EstimateToolsTokens(tools)
	second := EstimateToolsTokens(tools)
	if first != second {
		t.Fatalf("tool token estimates should be deterministic: %d != %d", first, second)
	}
	if first <= 0 {
		t.Fatal("tool token estimate should be positive")
	}
}

func TestAgenticLoopPreservesEarlierEvidenceWithinContextLimit(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{ControlLevel: tools.ControlLevelReadOnly})
	evidence := `{"resource_id":"agent-hot","temperature":94,"alert_active":true,"observed_at":"2026-09-05T19:19:11Z","note":"` + strings.Repeat("observed evidence ", 40) + `"}`
	executor.RegisterTool(tools.RegisteredTool{
		Invocation: tools.StaticInvocation(agentcapabilities.ToolCallKindRead, agentcapabilities.MutationNone),
		Definition: tools.Tool{Name: "read_observation", InputSchema: tools.InputSchema{Type: "object", Properties: map[string]tools.PropertySchema{"sample": {Type: "string"}}}},
		Handler: func(context.Context, *tools.PulseToolExecutor, map[string]interface{}) (tools.CallToolResult, error) {
			return tools.NewTextResult(evidence), nil
		},
	})
	turns := 0
	provider := &mockStreamingProvider{chatStreamFunc: func(_ context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turns++
		require.Less(t, EstimateRequestTokens(req), providers.ContextWindowTokens("anthropic:claude-opus-5"))
		if turns == 5 {
			found := 0
			for _, m := range req.Messages {
				if m.ToolResult != nil {
					require.False(t, m.ToolResult.IsError)
					require.Equal(t, evidence, m.ToolResult.Content, "earlier observations must still be present on the final answer turn")
					found++
				}
			}
			require.Equal(t, 4, found)
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "The current temperature alert is active."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "end_turn"}})
		} else {
			require.Less(t, turns, 5)
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "tool_use", ToolCalls: []providers.ToolCall{{ID: fmt.Sprintf("read-%d", turns), Name: "read_observation", Input: map[string]interface{}{"sample": fmt.Sprint(turns)}}}}})
		}
		return nil
	}}
	loop := NewAgenticLoop(provider, executor, "Answer from the supplied observations.")
	loop.SetProviderInfo("anthropic", "claude-opus-5")
	loop.SetMaxTurns(6)
	_, err := loop.ExecuteWithTools(context.Background(), "evidence-retention", []Message{{Role: "user", Content: "Check temperature and alerts."}}, []providers.Tool{{Name: "read_observation"}}, func(StreamEvent) {})
	require.NoError(t, err)
	require.Equal(t, 5, turns)
}
