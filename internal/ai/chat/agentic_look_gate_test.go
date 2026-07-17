package chat

// Tests for the look-before-asking gate: pulse_question issued before any
// tool attempt in a run is refused with a steer back to read-only tools,
// sibling tool calls in the same provider turn still execute, and the gate
// fails open after maxLookGateBlocks refusals so an unanswerable prompt
// cannot livelock. The user-visible promise (a natural first question gets
// an answer, never a clarification card) is pinned at the stream boundary
// in interaction_scenario_corpus_test.go.

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookGateQuestionInput() map[string]interface{} {
	return map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"id": "q1", "type": "select",
				"question": "Which resource do you want to check?",
				"options":  []interface{}{map[string]interface{}{"label": "Fleet", "value": "fleet"}},
			},
		},
	}
}

// TestAgenticLoop_LookGateRefusesFirstActionQuestion drives the observed
// small-model failure: the model's first action is an elicitation whose
// answer is derivable from enumeration. The gate must refuse it (no
// question card), and the run must continue to a tool-backed answer.
func TestAgenticLoop_LookGateRefusesFirstActionQuestion(t *testing.T) {
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		switch turn {
		case 1:
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{ID: "q-1", Name: pulseQuestionToolName, Input: lookGateQuestionInput()},
				},
			}})
		case 2:
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{ID: "r-1", Name: "pulse_query", Input: map[string]interface{}{"action": "health"}},
				},
			}})
		default:
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "No active alerts."}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		}
		return nil
	}

	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	loop := NewAgenticLoop(provider, exec, "base prompt")

	var questionEvents int
	messages, err := loop.ExecuteWithTools(
		context.Background(),
		"look-gate-session",
		[]Message{{Role: "user", Content: "are there any alerts I should look at?"}},
		nil,
		func(event StreamEvent) {
			if event.Type == "question" {
				questionEvents++
			}
		},
	)
	require.NoError(t, err)
	require.Zero(t, questionEvents, "a refused elicitation must not emit a question card")

	var blockedSeen, querySeen, finalSeen bool
	for _, msg := range messages {
		if msg.ToolResult != nil {
			switch msg.ToolResult.ToolUseID {
			case "q-1":
				blockedSeen = true
				require.True(t, msg.ToolResult.IsError, "gate refusal must be an error result")
				require.Contains(t, msg.ToolResult.Content, "BLOCKED")
				require.Contains(t, msg.ToolResult.Content, "pulse_summarize")
			case "r-1":
				querySeen = true
				assert.NotContains(t, msg.ToolResult.Content, "SKIPPED")
			}
		}
		if msg.Role == "assistant" && strings.Contains(msg.Content, "No active alerts.") {
			finalSeen = true
		}
	}
	require.True(t, blockedSeen, "blocked question must persist a paired error result")
	require.True(t, querySeen, "the follow-up tool attempt must execute")
	require.True(t, finalSeen, "the run must end in an answer")
}

// TestAgenticLoop_LookGateKeepsSiblingToolCalls pins that a refused
// question does not trip the interactive-set path that skips sibling
// tools: a first turn pairing a question with a read call still executes
// the read.
func TestAgenticLoop_LookGateKeepsSiblingToolCalls(t *testing.T) {
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		switch turn {
		case 1:
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{ID: "q-1", Name: pulseQuestionToolName, Input: lookGateQuestionInput()},
					{ID: "r-1", Name: "pulse_query", Input: map[string]interface{}{"action": "health"}},
				},
			}})
		default:
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "done"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		}
		return nil
	}

	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	loop := NewAgenticLoop(provider, exec, "base prompt")

	messages, err := loop.ExecuteWithTools(
		context.Background(),
		"look-gate-sibling-session",
		[]Message{{Role: "user", Content: "how is my machine doing?"}},
		nil,
		func(StreamEvent) {},
	)
	require.NoError(t, err)

	var blockedSeen, queryExecuted bool
	for _, msg := range messages {
		if msg.ToolResult == nil {
			continue
		}
		switch msg.ToolResult.ToolUseID {
		case "q-1":
			blockedSeen = true
			require.Contains(t, msg.ToolResult.Content, "BLOCKED")
		case "r-1":
			queryExecuted = true
			assert.NotContains(t, msg.ToolResult.Content, "SKIPPED",
				"sibling read call must execute, not be skipped for a refused question")
		}
	}
	require.True(t, blockedSeen)
	require.True(t, queryExecuted)
}

// TestAgenticLoop_LookGateFailsOpenAfterMaxBlocks pins the livelock
// escape hatch: after maxLookGateBlocks refusals with still no tool
// attempt, the next pulse_question goes through to the user.
func TestAgenticLoop_LookGateFailsOpenAfterMaxBlocks(t *testing.T) {
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		if turn <= maxLookGateBlocks+1 {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{ID: "q-" + string(rune('0'+turn)), Name: pulseQuestionToolName, Input: lookGateQuestionInput()},
				},
			}})
			return nil
		}
		callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Understood."}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		return nil
	}

	exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	loop := NewAgenticLoop(provider, exec, "base prompt")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		mu          sync.Mutex
		questionEvt *QuestionData
	)
	doneCh := make(chan struct{})
	var execErr error
	go func() {
		defer close(doneCh)
		_, execErr = loop.ExecuteWithTools(
			ctx,
			"look-gate-failopen-session",
			[]Message{{Role: "user", Content: "help me decide"}},
			nil,
			func(event StreamEvent) {
				if event.Type != "question" {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if questionEvt == nil {
					var qd QuestionData
					_ = json.Unmarshal(event.Data, &qd)
					questionEvt = &qd
				}
			},
		)
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return questionEvt != nil && questionEvt.QuestionID != ""
	}, 3*time.Second, 10*time.Millisecond,
		"after %d refusals the question must reach the user", maxLookGateBlocks)

	mu.Lock()
	qID := questionEvt.QuestionID
	mu.Unlock()
	require.NoError(t, loop.AnswerQuestion(qID, []QuestionAnswer{{ID: "q1", Value: "fleet"}}))

	select {
	case <-doneCh:
	case <-ctx.Done():
		t.Fatalf("loop did not complete: %v", ctx.Err())
	}
	require.NoError(t, execErr)
}
