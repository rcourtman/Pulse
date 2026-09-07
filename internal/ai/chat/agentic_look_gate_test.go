package chat

// Clarification timing belongs to the model. The harness owns delivery and response pairing.

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
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

func TestAgenticLoop_AllowsModelClarificationBeforeAnyRead(t *testing.T) {
	turn := 0
	provider := &stubStreamingProvider{}
	provider.chatStream = func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
		turn++
		if turn == 1 {
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
		"the model-selected question must reach the user on its first attempt")

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
	require.Equal(t, 2, turn)
}
