package chat

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestAgenticLoop_SteerInjectsAtNextTurnBoundary drives a two-turn run where
// turn 1 blocks on a pulse_question. A steer delivered while the run is
// blocked must be injected as a user message before turn 2's provider call,
// announced via steer_applied, and returned in resultMessages marked Steered.
func TestAgenticLoop_SteerInjectsAtNextTurnBoundary(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockProvider := &MockProvider{}
	loop := NewAgenticLoop(mockProvider, executor, "You are a helper")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessionID := "steer-session"
	messages := []Message{{Role: "user", Content: "Do something but ask me first"}}
	const steerPrompt = "actually check pve2 as well"

	// Turn 1: model looks with a read tool (the look-before-asking gate
	// refuses a first-action pulse_question).
	scriptLookTurn(mockProvider, func(req providers.ChatRequest) bool {
		return len(req.Messages) == 1
	})

	// Turn 2: model requests pulse_question (blocks until answered).
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		return hasToolResult(req, "t0") && !hasToolResult(req, "t1")
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		toolInput := map[string]interface{}{
			"questions": []interface{}{
				map[string]interface{}{
					"id":       "q1",
					"type":     "select",
					"question": "Pick one",
					"options": []interface{}{
						map[string]interface{}{"label": "A", "value": "a"},
					},
				},
			},
		}
		cb(providers.StreamEvent{
			Type: "tool_start",
			Data: providers.ToolStartEvent{ID: "t1", Name: pulseQuestionToolName, Input: toolInput},
		})
		cb(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{ID: "t1", Name: pulseQuestionToolName, Input: toolInput}},
			},
		})
	}).Once()

	// Final turn: the request must carry the steer as a user message AFTER
	// the tool result for t1.
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		toolResultIndex := -1
		steerIndex := -1
		for i, m := range req.Messages {
			if m.ToolResult != nil && m.ToolResult.ToolUseID == "t1" {
				toolResultIndex = i
			}
			if m.Role == "user" && m.Content == steerPrompt {
				steerIndex = i
			}
		}
		return toolResultIndex >= 0 && steerIndex > toolResultIndex
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Checking pve2 too."}})
		cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
	}).Once()

	var (
		mu           sync.Mutex
		questionEvt  *QuestionData
		steerApplied *SteerAppliedData
	)
	callback := func(event StreamEvent) {
		mu.Lock()
		defer mu.Unlock()
		if event.Type == "question" && questionEvt == nil {
			var qd QuestionData
			_ = json.Unmarshal(event.Data, &qd)
			questionEvt = &qd
		}
		if event.Type == "steer_applied" && steerApplied == nil {
			var sd SteerAppliedData
			_ = json.Unmarshal(event.Data, &sd)
			steerApplied = &sd
		}
	}

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

	// Steer while the run is blocked, then unblock it.
	require.NoError(t, loop.Steer(sessionID, Message{
		ID: "steer-msg-1", Role: "user", Content: steerPrompt, Steered: true, Timestamp: time.Now(),
	}, "client-row-1"))

	mu.Lock()
	qID := questionEvt.QuestionID
	mu.Unlock()
	require.NoError(t, loop.AnswerQuestion(qID, []QuestionAnswer{{ID: "q1", Value: "a"}}))

	select {
	case <-doneCh:
	case <-ctx.Done():
		t.Fatalf("agentic loop did not complete: %v", ctx.Err())
	}
	require.NoError(t, err)

	mu.Lock()
	require.NotNil(t, steerApplied, "expected steer_applied event")
	assert.Equal(t, "client-row-1", steerApplied.ClientMessageID)
	assert.Equal(t, "steer-msg-1", steerApplied.MessageID)
	assert.Equal(t, steerPrompt, steerApplied.Prompt)
	mu.Unlock()

	steeredCount := 0
	for _, msg := range results {
		if msg.Steered {
			steeredCount++
			assert.Equal(t, "user", msg.Role)
			assert.Equal(t, steerPrompt, msg.Content)
		}
	}
	assert.Equal(t, 1, steeredCount, "expected exactly one steered message in results")
	assert.Equal(t, "Checking pve2 too.", results[len(results)-1].Content)

	mockProvider.AssertExpectations(t)
}

// TestAgenticLoop_UnconsumedSteerIsDiscarded verifies that a steer arriving
// too late for any boundary is dropped when the run ends, so the client's
// queue-drain fallback cannot double-record it.
func TestAgenticLoop_UnconsumedSteerIsDiscarded(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubStreamingProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	sessionID := "late-steer-session"
	_, err := loop.Execute(context.Background(), sessionID, []Message{{Role: "user", Content: "hi"}}, func(StreamEvent) {})
	require.NoError(t, err)

	// The run already ended; the loop's defer must have cleared the inbox,
	// and a fresh run must not see stale steers from a prior run either.
	require.NoError(t, loop.Steer(sessionID, Message{ID: "late", Role: "user", Content: "too late"}, ""))
	loop.discardPendingSteers(sessionID)
	assert.Empty(t, loop.takePendingSteers(sessionID))
}

// TestAgenticLoop_SteerBacklogIsBounded verifies the inbox cap so a chatty
// client cannot grow the running turn's prompt without limit.
func TestAgenticLoop_SteerBacklogIsBounded(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(&stubStreamingProvider{}, executor, "system")

	for i := 0; i < maxPendingSteersPerSession; i++ {
		require.NoError(t, loop.Steer("backlog-session", Message{ID: "m", Role: "user", Content: "steer"}, ""))
	}
	err := loop.Steer("backlog-session", Message{ID: "m", Role: "user", Content: "one too many"}, "")
	require.ErrorIs(t, err, errSteerBacklogFull)
	assert.Len(t, loop.takePendingSteers("backlog-session"), maxPendingSteersPerSession)
}

// TestService_SteerSession_RoutingOutcomes covers the service-level routing
// results that never reach a loop.
func TestService_SteerSession_RoutingOutcomes(t *testing.T) {
	svc := &Service{}

	result, err := svc.SteerSession(context.Background(), "no-run-session", SessionSteerRequest{Prompt: "hello"})
	require.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "no_active_run", result.Reason)

	result, err = svc.SteerSession(context.Background(), "patrol-main", SessionSteerRequest{Prompt: "hello"})
	require.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "system_session", result.Reason)

	result, err = svc.SteerSession(context.Background(), "some-session", SessionSteerRequest{Prompt: "   "})
	require.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "empty_prompt", result.Reason)

	_, err = svc.SteerSession(context.Background(), "../bad", SessionSteerRequest{Prompt: "hello"})
	require.Error(t, err)
}

// TestService_ExecuteStream_SteeredMessagePersists proves the end-to-end
// path: a steer accepted mid-run is injected at the boundary AND survives
// the end-of-run save (which skips ordinary user messages).
func TestService_ExecuteStream_SteeredMessagePersists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	require.NoError(t, err)

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockProvider := &MockProvider{}
	loop := NewAgenticLoop(mockProvider, executor, "system")

	svc := &Service{
		cfg:                &config.AIConfig{ChatModel: "openai:test"},
		sessions:           store,
		executor:           executor,
		agenticLoop:        loop,
		provider:           mockProvider,
		started:            true,
		activeExecutions:   make(map[string]map[*AgenticLoop]struct{}),
		questionExecutions: make(map[string]*AgenticLoop),
	}

	const steerPrompt = "steer: also check the replication lag"
	sessionID := "sess-steer-persist"

	// Turn 1 looks with a read tool (the look-before-asking gate refuses a
	// first-action pulse_question); turn 2 blocks on a question; the test
	// steers through the SERVICE while blocked, then answers.
	toolInput := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"id": "q1", "type": "select", "question": "Proceed?",
				"options": []interface{}{map[string]interface{}{"label": "Yes", "value": "y"}},
			},
		},
	}
	scriptLookTurn(mockProvider, func(req providers.ChatRequest) bool {
		return !hasToolResult(req, "t0")
	})
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		for _, m := range req.Messages {
			if m.Role == "user" && m.Content == steerPrompt {
				return false
			}
		}
		return hasToolResult(req, "t0")
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{
			Type: "tool_start",
			Data: providers.ToolStartEvent{ID: "t1", Name: pulseQuestionToolName, Input: toolInput},
		})
		cb(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{{ID: "t1", Name: pulseQuestionToolName, Input: toolInput}},
			},
		})
	}).Once()
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		for _, m := range req.Messages {
			if m.Role == "user" && m.Content == steerPrompt {
				return true
			}
		}
		return false
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(2).(providers.StreamCallback)
		cb(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "done, checked lag"}})
		cb(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
	}).Once()

	var (
		mu          sync.Mutex
		questionEvt *QuestionData
	)
	callback := func(event StreamEvent) {
		mu.Lock()
		defer mu.Unlock()
		if event.Type == "question" && questionEvt == nil {
			var qd QuestionData
			_ = json.Unmarshal(event.Data, &qd)
			questionEvt = &qd
		}
	}

	var execErr error
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		execErr = svc.ExecuteStream(context.Background(), ExecuteRequest{SessionID: sessionID, Prompt: "check the cluster"}, callback)
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return questionEvt != nil && questionEvt.QuestionID != ""
	}, 3*time.Second, 10*time.Millisecond, "expected question event")

	steerResult, err := svc.SteerSession(context.Background(), sessionID, SessionSteerRequest{
		Prompt:          steerPrompt,
		ClientMessageID: "client-row-9",
	})
	require.NoError(t, err)
	require.True(t, steerResult.Accepted, "expected steer to reach the active loop, got reason %q", steerResult.Reason)

	mu.Lock()
	qID := questionEvt.QuestionID
	mu.Unlock()
	require.NoError(t, svc.AnswerQuestion(context.Background(), qID, []QuestionAnswer{{ID: "q1", Value: "y"}}))

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteStream did not complete")
	}
	require.NoError(t, execErr)

	// The steered message must be in durable history, after the opening
	// prompt and before the final assistant answer.
	saved, err := store.GetMessages(sessionID)
	require.NoError(t, err)
	steerIndex, finalIndex := -1, -1
	for i, msg := range saved {
		if msg.Steered && msg.Content == steerPrompt {
			steerIndex = i
		}
		if msg.Role == "assistant" && msg.Content == "done, checked lag" {
			finalIndex = i
		}
	}
	require.GreaterOrEqual(t, steerIndex, 1, "steered message missing from durable history")
	require.Greater(t, finalIndex, steerIndex, "final answer should follow the steered message")

	mockProvider.AssertExpectations(t)
}
