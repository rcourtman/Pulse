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

func TestConvertToProviderMessages_RepairsOrphanToolCallsInLoadedSession(t *testing.T) {
	// Real production trigger: the patrol-main session accumulated
	// "assistant: [tool_calls A, B, C, D] / tool: A / tool: B / tool: C /
	// tool: D" turns interleaved with user prompts. When a prior run was
	// interrupted (the malformed-history bug for Patrol, or a chat session
	// ending mid-tool-call for Assistant), some result messages were never
	// captured. The next request that loads this history is rejected by
	// the provider as a structural violation. Without the repair pass,
	// that's the exact "must be followed by tool messages responding to
	// each tool_call_id" error that flapped Patrol for 33 days.
	//
	// The repair pass must inject synthetic error tool results for the
	// missing call ids so the conversation is structurally valid before
	// it reaches the provider. The synthetic result must be marked as an
	// error and tell the model the call was interrupted so it can retry
	// or proceed without that data.
	messages := []Message{
		{Role: "user", Content: "investigate infrastructure"},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "call-A", Name: "pulse_storage"},
				{ID: "call-B", Name: "pulse_backups"},
				{ID: "call-C", Name: "pulse_alerts"},
			},
		},
		// Only call-A has a tool result. B and C are orphans.
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "call-A", Content: "ok"}},
		{Role: "user", Content: "next question"},
	}

	converted := convertToProviderMessages(messages)

	// The synthetic results must appear immediately after the assistant
	// message so the structural invariant holds: every tool_call_id has
	// a matching downstream tool message before the next user turn.
	if len(converted) != 6 {
		t.Fatalf("expected 6 messages after repair (original 4 + 2 synthetic), got %d:\n%+v", len(converted), converted)
	}

	assistantIdx := -1
	for i, m := range converted {
		if len(m.ToolCalls) > 0 {
			assistantIdx = i
			break
		}
	}
	if assistantIdx == -1 {
		t.Fatalf("expected to find assistant message with tool_calls")
	}

	// Collect tool_call_ids present in tool result messages AFTER the
	// assistant message, in order.
	var fulfilledAfter []string
	for _, m := range converted[assistantIdx+1:] {
		if m.ToolResult != nil {
			fulfilledAfter = append(fulfilledAfter, m.ToolResult.ToolUseID)
		}
	}
	wantInOrder := []string{"call-A", "call-B", "call-C"}
	// Real tool result + 2 synthetic. The real one (call-A) is already
	// in the conversation at its original position; call-B and call-C
	// are the injected ones, immediately following the assistant turn.
	// Just check that all three IDs are present.
	have := map[string]bool{}
	for _, id := range fulfilledAfter {
		have[id] = true
	}
	for _, want := range wantInOrder {
		if !have[want] {
			t.Fatalf("expected tool_call_id %q to have a downstream tool result, got %+v", want, fulfilledAfter)
		}
	}

	// The synthetic results must be marked as errors so the model
	// knows the data is missing rather than treating empty content as
	// a successful empty response.
	syntheticCount := 0
	for _, m := range converted {
		if m.ToolResult == nil {
			continue
		}
		if m.ToolResult.ToolUseID == "call-A" {
			continue // real result, not synthetic
		}
		if !m.ToolResult.IsError {
			t.Fatalf("expected synthetic tool result %q to be marked is_error=true, got %+v", m.ToolResult.ToolUseID, m.ToolResult)
		}
		if !strings.Contains(m.ToolResult.Content, "interrupted") {
			t.Fatalf("expected synthetic content to explain the interruption, got %q", m.ToolResult.Content)
		}
		syntheticCount++
	}
	if syntheticCount != 2 {
		t.Fatalf("expected exactly 2 synthetic results, got %d", syntheticCount)
	}
}

func TestConvertToProviderMessages_NoOpWhenAllToolCallsHaveResults(t *testing.T) {
	// When every tool_call_id is already matched by a downstream tool
	// message, the repair pass must be a no-op — no synthetic messages
	// injected, no reordering, no leaked is_error tags.
	messages := []Message{
		{Role: "user", Content: "ok"},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "x", Name: "pulse_storage"},
				{ID: "y", Name: "pulse_alerts"},
			},
		},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "x", Content: "ok"}},
		{Role: "user", ToolResult: &ToolResult{ToolUseID: "y", Content: "ok"}},
	}

	converted := convertToProviderMessages(messages)
	if len(converted) != len(messages) {
		t.Fatalf("expected no synthetic messages, got %d vs %d", len(converted), len(messages))
	}
	for _, m := range converted {
		if m.ToolResult != nil && m.ToolResult.IsError {
			t.Fatalf("expected no is_error results, got %+v", m.ToolResult)
		}
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
