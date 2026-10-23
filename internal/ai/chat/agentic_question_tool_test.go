package chat

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// hasToolResult reports whether the request carries a tool result for the
// given provider tool-call ID.
func hasToolResult(req providers.ChatRequest, toolUseID string) bool {
	for _, m := range req.Messages {
		if m.ToolResult != nil && m.ToolResult.ToolUseID == toolUseID {
			return true
		}
	}
	return false
}

// No Pulse client can answer pulse_question since the web chat was retired:
// Pulse Mobile has no question UI. An interactive turn must therefore pair a
// question call with a refusal that sends the model back to plain reply text,
// and finish, instead of waiting for an answer that cannot arrive.
func TestAgenticLoop_RefusesPulseQuestionInInteractiveSessions(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockProvider := &MockProvider{}
	loop := NewAgenticLoop(mockProvider, executor, "You are a helper")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	questionInput := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"id":       "q1",
				"type":     "select",
				"question": "Pick one",
				"options": []interface{}{
					map[string]interface{}{"label": "A", "value": "a"},
					map[string]interface{}{"label": "B", "value": "b"},
				},
			},
		},
	}

	queryInput := map[string]interface{}{"action": "topology"}

	// Turn 1: the model calls pulse_question alongside a read tool.
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		return !hasToolResult(req, "t1")
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{
			Type: "tool_start",
			Data: providers.ToolStartEvent{ID: "t1", Name: pulseQuestionToolName, Input: questionInput},
		})
		cb(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{ID: "t1", Name: pulseQuestionToolName, Input: questionInput},
					{ID: "t2", Name: "pulse_query", Input: queryInput},
				},
			},
		})
	}).Once()

	// Turn 2: the model reads the refusal and the sibling result, then asks
	// in its reply instead.
	var refusal string
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		if !hasToolResult(req, "t2") {
			return false
		}
		for _, m := range req.Messages {
			if m.ToolResult != nil && m.ToolResult.ToolUseID == "t1" {
				refusal = m.ToolResult.Content
				return true
			}
		}
		return false
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{
			Type: "content",
			Data: providers.ContentEvent{Text: "Which one do you want, A or B?"},
		})
		cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
	}).Once()

	var (
		mu     sync.Mutex
		events []StreamEvent
	)
	callback := func(event StreamEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	messages := []Message{{Role: "user", Content: "Do something but ask me first"}}
	results, err := loop.Execute(ctx, "question-session", messages, callback)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "Which one do you want, A or B?", results[len(results)-1].Content)
	assert.Contains(t, refusal, "pulse_question is unavailable")
	assert.Contains(t, refusal, "Ask the user in your reply text")

	// The durable transcript pairs the recorded question call with its
	// refusal, so a reloaded session never holds an unanswered call.
	paired := false
	for _, msg := range results {
		if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "t1" {
			paired = true
			assert.True(t, msg.ToolResult.IsError)
		}
	}
	assert.True(t, paired, "refusal must be persisted with the question call")

	mu.Lock()
	defer mu.Unlock()
	for _, ev := range events {
		assert.NotEqual(t, "question", ev.Type, "no client can render a question card")
		if ev.Type == "tool_start" {
			var ts ToolStartData
			_ = json.Unmarshal(ev.Data, &ts)
			assert.NotEqual(t, pulseQuestionToolName, ts.Name)
		}
	}

	mockProvider.AssertExpectations(t)
}
