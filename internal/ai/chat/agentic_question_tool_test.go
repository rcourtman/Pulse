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

func TestAgenticLoop_PulseQuestionToolInteractiveFlow(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockProvider := &MockProvider{}
	loop := NewAgenticLoop(mockProvider, executor, "You are a helper")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessionID := "question-session"
	messages := []Message{{Role: "user", Content: "Do something but ask me first"}}

	// Turn 1: model requests pulse_question
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		return len(req.Messages) == 1
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{
			Type: "tool_start",
			Data: providers.ToolStartEvent{
				ID:   "t1",
				Name: pulseQuestionToolName,
				Input: map[string]interface{}{
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
				},
			},
		})
		cb(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{
						ID:   "t1",
						Name: pulseQuestionToolName,
						Input: map[string]interface{}{
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
						},
					},
				},
			},
		})
	}).Once()

	// Turn 2: model sees tool result and replies.
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		// Expect tool result message for t1 to be present
		for _, m := range req.Messages {
			if m.ToolResult != nil && m.ToolResult.ToolUseID == "t1" {
				var payload struct {
					QuestionID string           `json:"question_id"`
					Answers    []QuestionAnswer `json:"answers"`
				}
				if err := json.Unmarshal([]byte(m.ToolResult.Content), &payload); err != nil {
					return false
				}
				if payload.QuestionID == "" || len(payload.Answers) != 1 {
					return false
				}
				if payload.Answers[0].ID != "q1" || payload.Answers[0].Value != "b" {
					return false
				}
				return true
			}
		}
		return false
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{
			Type: "content",
			Data: providers.ContentEvent{Text: "Thanks, you picked B."},
		})
		cb(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{},
		})
	}).Once()

	var (
		mu          sync.Mutex
		events      []StreamEvent
		questionEvt *QuestionData
	)

	callback := func(event StreamEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
		if event.Type == "question" && questionEvt == nil {
			var qd QuestionData
			_ = json.Unmarshal(event.Data, &qd)
			questionEvt = &qd
		}
	}

	// Run loop in background; answer as soon as we see the question event.
	var (
		results []Message
		err     error
		doneCh  = make(chan struct{})
	)
	go func() {
		defer close(doneCh)
		results, err = loop.Execute(ctx, sessionID, messages, callback)
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return questionEvt != nil && questionEvt.QuestionID != ""
	}, 2*time.Second, 10*time.Millisecond, "expected question event")

	mu.Lock()
	qID := questionEvt.QuestionID
	mu.Unlock()

	require.NoError(t, loop.AnswerQuestion(qID, []QuestionAnswer{{ID: "q1", Value: "b"}}))

	select {
	case <-doneCh:
	case <-ctx.Done():
		t.Fatalf("agentic loop did not complete: %v", ctx.Err())
	}

	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "Thanks, you picked B.", results[len(results)-1].Content)

	// Ensure pulse_question tool_start is suppressed (UI uses question card).
	mu.Lock()
	defer mu.Unlock()
	for _, ev := range events {
		if ev.Type == "tool_start" {
			var ts ToolStartData
			_ = json.Unmarshal(ev.Data, &ts)
			assert.NotEqual(t, pulseQuestionToolName, ts.Name)
		}
	}

	mockProvider.AssertExpectations(t)
}
