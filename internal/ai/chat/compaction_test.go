package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeToolResultMessage creates a provider message with a tool result.
func makeToolResultMessage(toolUseID, content string, isError bool) providers.Message {
	return providers.Message{
		Role: "user",
		ToolResult: &providers.ToolResult{
			ToolUseID: toolUseID,
			Content:   content,
			IsError:   isError,
		},
	}
}

// makeAssistantMessageWithToolCalls creates an assistant message with tool calls.
func makeAssistantMessageWithToolCalls(content string, toolCalls ...providers.ToolCall) providers.Message {
	return providers.Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// bigContent returns a string of the given length for testing size thresholds.
func bigContent(n int) string {
	return strings.Repeat("x", n)
}

func TestCompactOldToolResults_NoCompactionOnFirstTurn(t *testing.T) {
	msgs := []providers.Message{
		{Role: "user", Content: "check my infra"},
	}
	original := msgs[0].Content
	compactOldToolResults(msgs, len(msgs), 3, 500)
	assert.Equal(t, original, msgs[0].Content, "user message should not be modified")
}

func TestCompactOldToolResults_DoesNotCompactCurrentTurn(t *testing.T) {
	// Simulate: turn 0 produced an assistant + tool result, turn 1 is about to start.
	// currentTurnStartIndex points to after the last message from turn 0.
	bigResult := bigContent(2000)
	msgs := []providers.Message{
		{Role: "user", Content: "check storage"},
		makeAssistantMessageWithToolCalls("", providers.ToolCall{ID: "tc1", Name: "pulse_storage", Input: map[string]interface{}{"type": "pools"}}),
		makeToolResultMessage("tc1", bigResult, false),
	}

	// currentTurnStartIndex = 3 means all messages are from the current turn
	compactOldToolResults(msgs, 3, 3, 500)
	assert.Equal(t, bigResult, msgs[2].ToolResult.Content, "current turn results should not be compacted")
}

func TestCompactOldToolResults_CompactsOldTurns(t *testing.T) {
	bigResult1 := bigContent(5000)
	bigResult2 := bigContent(3000)
	smallResult := bigContent(100) // Below threshold

	msgs := []providers.Message{
		// Turn 0: user message + assistant + tool results
		{Role: "user", Content: "scan everything"},
		makeAssistantMessageWithToolCalls("scanning...",
			providers.ToolCall{ID: "tc1", Name: "pulse_query", Input: map[string]interface{}{"type": "topology"}},
			providers.ToolCall{ID: "tc2", Name: "pulse_storage", Input: map[string]interface{}{"type": "pools"}},
			providers.ToolCall{ID: "tc3", Name: "pulse_metrics", Input: map[string]interface{}{"type": "performance"}},
		),
		makeToolResultMessage("tc1", bigResult1, false),
		makeToolResultMessage("tc2", bigResult2, false),
		makeToolResultMessage("tc3", smallResult, false), // Too small to compact

		// Turn 1: assistant analyzed results, made more calls
		makeAssistantMessageWithToolCalls("found issues, investigating...",
			providers.ToolCall{ID: "tc4", Name: "pulse_read", Input: map[string]interface{}{"action": "logs"}},
		),
		makeToolResultMessage("tc4", bigContent(4000), false),

		// Turn 2: assistant summary
		makeAssistantMessageWithToolCalls("checking one more thing...",
			providers.ToolCall{ID: "tc5", Name: "pulse_metrics", Input: map[string]interface{}{"type": "baselines"}},
		),
		makeToolResultMessage("tc5", bigContent(6000), false),

		// Turn 3: assistant with more tool calls (current turn)
		makeAssistantMessageWithToolCalls("final check...",
			providers.ToolCall{ID: "tc6", Name: "pulse_query", Input: map[string]interface{}{"type": "alerts"}},
		),
		makeToolResultMessage("tc6", bigContent(2000), false),
	}

	// currentTurnStartIndex = index of the turn 3 assistant message
	currentTurnStart := 9 // msgs[9] is the turn 3 assistant
	keepTurns := 2
	minChars := 500

	compactOldToolResults(msgs, currentTurnStart, keepTurns, minChars)

	// With keepTurns=2, we walk back from currentTurnStart=9:
	//   index 7 (turn 2 assistant): turnsFound=1
	//   index 5 (turn 1 assistant): turnsFound=2 >= keepTurns -> compactBefore=5
	// So indices 0-4 are eligible for compaction.

	// Turn 0 results (indices 2, 3, 4) — compacted (except small one)
	assert.Contains(t, msgs[2].ToolResult.Content, "[Tool result compacted:", "big result from turn 0 should be compacted")
	assert.Contains(t, msgs[2].ToolResult.Content, "pulse_query", "compacted summary should include tool name")
	assert.Contains(t, msgs[3].ToolResult.Content, "[Tool result compacted:", "big result from turn 0 should be compacted")
	assert.Equal(t, smallResult, msgs[4].ToolResult.Content, "small result should NOT be compacted (under minChars)")

	// Turn 1 result (index 6): within keepTurns boundary (>= compactBefore=5), kept in full
	assert.NotContains(t, msgs[6].ToolResult.Content, "[Tool result compacted:", "turn 1 result should be kept (within keepTurns)")

	// Turn 2 result (index 8): within keepTurns, should be kept in full
	assert.NotContains(t, msgs[8].ToolResult.Content, "[Tool result compacted:", "turn 2 result should be kept (within keepTurns)")

	// Turn 3 results (index 10): current turn, should be kept
	assert.NotContains(t, msgs[10].ToolResult.Content, "[Tool result compacted:", "current turn results should be kept")

	// Assistant messages should never be touched
	assert.Equal(t, "scanning...", msgs[1].Content)
	assert.Equal(t, "found issues, investigating...", msgs[5].Content)
	assert.Equal(t, "checking one more thing...", msgs[7].Content)
}

func TestCompactOldToolResults_DoesNotCompactErrors(t *testing.T) {
	errorContent := bigContent(2000)
	msgs := []providers.Message{
		{Role: "user", Content: "do something"},
		makeAssistantMessageWithToolCalls("trying...",
			providers.ToolCall{ID: "tc1", Name: "pulse_control", Input: map[string]interface{}{"action": "restart"}},
		),
		makeToolResultMessage("tc1", errorContent, true), // Error result
		// Next turn
		makeAssistantMessageWithToolCalls("retrying...",
			providers.ToolCall{ID: "tc2", Name: "pulse_query", Input: map[string]interface{}{"type": "status"}},
		),
		makeToolResultMessage("tc2", bigContent(1000), false),
	}

	compactOldToolResults(msgs, 3, 0, 500) // keepTurns=0 means compact everything before currentTurnStart

	assert.Equal(t, errorContent, msgs[2].ToolResult.Content, "error results should never be compacted")
}

func TestCompactOldToolResults_KeepTurnsZero(t *testing.T) {
	// keepTurns=0: compact everything before currentTurnStartIndex
	bigResult := bigContent(2000)
	msgs := []providers.Message{
		{Role: "user", Content: "query"},
		makeAssistantMessageWithToolCalls("",
			providers.ToolCall{ID: "tc1", Name: "pulse_query", Input: map[string]interface{}{"type": "list"}},
		),
		makeToolResultMessage("tc1", bigResult, false),
		// Current turn starts here
		makeAssistantMessageWithToolCalls("",
			providers.ToolCall{ID: "tc2", Name: "pulse_read", Input: map[string]interface{}{}},
		),
		makeToolResultMessage("tc2", bigContent(3000), false),
	}

	compactOldToolResults(msgs, 3, 0, 500)
	assert.Contains(t, msgs[2].ToolResult.Content, "[Tool result compacted:", "should compact with keepTurns=0")
}

func TestCompactOldToolResults_EmptyMessages(t *testing.T) {
	// Edge case: empty or nil-ish slices should not panic
	compactOldToolResults(nil, 0, 3, 500)
	compactOldToolResults([]providers.Message{}, 0, 3, 500)
	compactOldToolResults([]providers.Message{{Role: "user", Content: "hi"}}, 0, 3, 500)
}

func TestCompactOldToolResults_ContextSavings(t *testing.T) {
	// Simulate a patrol-like scenario: 10 tool calls, each returning ~4000 chars.
	// After compaction with keepTurns=2, old results should be dramatically smaller.
	var msgs []providers.Message
	msgs = append(msgs, providers.Message{Role: "user", Content: "run patrol"})

	for i := 0; i < 10; i++ {
		tcID := fmt.Sprintf("tc%d", i)
		msgs = append(msgs, makeAssistantMessageWithToolCalls(
			fmt.Sprintf("step %d", i),
			providers.ToolCall{ID: tcID, Name: "pulse_query", Input: map[string]interface{}{"type": "topology"}},
		))
		msgs = append(msgs, makeToolResultMessage(tcID, bigContent(4000), false))
	}

	// Current turn starts after all 10 tool rounds
	currentTurnStart := len(msgs)

	// Measure total tool result chars before compaction
	charsBefore := 0
	for _, m := range msgs {
		if m.ToolResult != nil {
			charsBefore += len(m.ToolResult.Content)
		}
	}

	compactOldToolResults(msgs, currentTurnStart, 3, 500)

	// Measure after
	charsAfter := 0
	compactedCount := 0
	for _, m := range msgs {
		if m.ToolResult != nil {
			charsAfter += len(m.ToolResult.Content)
			if strings.Contains(m.ToolResult.Content, "[Tool result compacted:") {
				compactedCount++
			}
		}
	}

	// Should have compacted 7 results (10 - 3 keepTurns)
	assert.Equal(t, 7, compactedCount, "should compact 7 of 10 results (keeping last 3 turns)")

	// Context savings should be substantial (at least 50%)
	savings := float64(charsBefore-charsAfter) / float64(charsBefore) * 100
	assert.Greater(t, savings, 50.0, "should save at least 50%% of tool result chars; saved %.1f%%", savings)

	t.Logf("Context savings: %d -> %d chars (%.1f%% reduction, %d results compacted)",
		charsBefore, charsAfter, savings, compactedCount)
}

func TestBuildCompactSummary(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		toolInput map[string]interface{}
		content   string
		wantParts []string
	}{
		{
			name:      "with params",
			toolName:  "pulse_storage",
			toolInput: map[string]interface{}{"type": "pools"},
			content:   bigContent(5000),
			wantParts: []string{"pulse_storage", "type=pools", "5000 chars"},
		},
		{
			name:      "no params",
			toolName:  "pulse_query",
			toolInput: map[string]interface{}{},
			content:   bigContent(1234),
			wantParts: []string{"pulse_query", "1234 chars"},
		},
		{
			name:      "multiple priority params",
			toolName:  "pulse_metrics",
			toolInput: map[string]interface{}{"type": "performance", "resource_id": "vm101", "period": "7d"},
			content:   bigContent(8000),
			wantParts: []string{"pulse_metrics", "type=performance", "resource_id=vm101", "period=7d"},
		},
		{
			name:      "nil input",
			toolName:  "pulse_discovery",
			toolInput: nil,
			content:   bigContent(600),
			wantParts: []string{"pulse_discovery", "600 chars"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCompactSummary(tt.toolName, tt.toolInput, tt.content)
			for _, part := range tt.wantParts {
				assert.Contains(t, result, part)
			}
			assert.Contains(t, result, "[Tool result compacted:")
			assert.Contains(t, result, "already been processed")
		})
	}
}

func TestFormatKeyParams(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  string
	}{
		{
			name:  "nil input",
			input: nil,
			want:  "",
		},
		{
			name:  "empty input",
			input: map[string]interface{}{},
			want:  "",
		},
		{
			name:  "single priority key",
			input: map[string]interface{}{"type": "pools"},
			want:  "type=pools",
		},
		{
			name:  "multiple priority keys",
			input: map[string]interface{}{"type": "performance", "resource_id": "101"},
			want:  "type=performance, resource_id=101",
		},
		{
			name:  "priority key with empty value is skipped",
			input: map[string]interface{}{"type": "", "resource_id": "101"},
			want:  "resource_id=101",
		},
		{
			name:  "non-string values are ignored",
			input: map[string]interface{}{"type": "pools", "limit": 100},
			want:  "type=pools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKeyParams(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCompactOldToolResults_PreservesToolCallInfo(t *testing.T) {
	// Verify that the compacted summary includes the correct tool name
	// even when the tool call is in a different message than the result.
	msgs := []providers.Message{
		{Role: "user", Content: "check"},
		makeAssistantMessageWithToolCalls("",
			providers.ToolCall{ID: "tc1", Name: "pulse_storage", Input: map[string]interface{}{"type": "pools"}},
			providers.ToolCall{ID: "tc2", Name: "pulse_metrics", Input: map[string]interface{}{"type": "performance", "period": "7d"}},
		),
		makeToolResultMessage("tc1", bigContent(2000), false),
		makeToolResultMessage("tc2", bigContent(3000), false),
		// Next turn (current)
		makeAssistantMessageWithToolCalls("done",
			providers.ToolCall{ID: "tc3", Name: "pulse_read", Input: map[string]interface{}{}},
		),
		makeToolResultMessage("tc3", bigContent(1000), false),
	}

	compactOldToolResults(msgs, 4, 0, 500) // compact everything before index 4

	require.Contains(t, msgs[2].ToolResult.Content, "pulse_storage")
	require.Contains(t, msgs[2].ToolResult.Content, "type=pools")
	require.Contains(t, msgs[3].ToolResult.Content, "pulse_metrics")
	require.Contains(t, msgs[3].ToolResult.Content, "type=performance")
	require.Contains(t, msgs[3].ToolResult.Content, "period=7d")
}

func TestTruncateToolResultForModel_ReducedLimit(t *testing.T) {
	// Verify the limit is now 16000
	assert.Equal(t, 16000, MaxToolResultCharsLimit, "MaxToolResultCharsLimit should be 16000")

	// Under limit: no truncation
	short := bigContent(15000)
	assert.Equal(t, short, truncateToolResultForModel(short))

	// Over limit: truncated with message
	long := bigContent(20000)
	result := truncateToolResultForModel(long)
	assert.Less(t, len(result), 20000)
	assert.Contains(t, result, "[TRUNCATED:")
	assert.Contains(t, result, "4000 characters cut")
}
