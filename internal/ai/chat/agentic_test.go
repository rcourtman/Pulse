package chat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockProvider struct {
	mock.Mock
}

type mockMetricsHistory struct{}

func (m *mockMetricsHistory) GetResourceMetrics(resourceID string, period time.Duration) ([]tools.MetricPoint, error) {
	return nil, nil
}

func (m *mockMetricsHistory) GetAllMetricsSummary(period time.Duration) (map[string]tools.ResourceMetricsSummary, error) {
	return map[string]tools.ResourceMetricsSummary{}, nil
}

func (m *MockProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providers.ChatResponse), args.Error(1)
}

func (m *MockProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	args := m.Called(ctx, req, callback)
	return args.Error(0)
}

func (m *MockProvider) TestConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockProvider) SupportsThinking(model string) bool {
	args := m.Called(model)
	return args.Bool(0)
}

func (m *MockProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]providers.ModelInfo), args.Error(1)
}

func TestAgenticLoop(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	t.Run("Execute Simple Chat", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		ctx := context.Background()
		sessionID := "test-session"
		messages := []Message{{Role: "user", Content: "Hello"}}

		mockProvider.On("ChatStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Hi there!"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		})

		var events []StreamEvent
		callback := func(event StreamEvent) {
			events = append(events, event)
		}

		results, err := loop.Execute(ctx, sessionID, messages, callback)
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Hi there!", results[0].Content)
		assert.Len(t, events, 1) // Only "content" is forwarded by the loop closure switch

		var eventContent ContentData
		json.Unmarshal(events[0].Data, &eventContent)
		assert.Equal(t, "Hi there!", eventContent.Text)
	})

	t.Run("Execute with Tool Call", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		ctx := context.Background()
		sessionID := "tool-session"
		messages := []Message{{Role: "user", Content: "List nodes"}}

		// 1. Initial tool call response
		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			// The original instruction had a syntactically incorrect return here.
			// To make the file syntactically correct, I'm commenting out the problematic line
			// and restoring the original correct condition, as the instruction "Comment out MockProvider usage"
			// was general and the specific code edit was flawed.
			// 	return agentexec.PolicyDecision{Allowed: true}
			return len(req.Messages) == 1
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "tool_start",
				Data: providers.ToolStartEvent{ID: "call_123", Name: "list_nodes"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{
						{ID: "call_123", Name: "list_nodes", Input: map[string]interface{}{}},
					},
				},
			})
		}).Once()

		// 2. Response after tool result
		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 3 // user msg + assistant tool call + user tool result
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "I found 2 nodes."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		}).Once()

		results, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {})
		assert.NoError(t, err)
		require.Len(t, results, 3) // assistant tool call, tool result, final response
		assert.Equal(t, "I found 2 nodes.", results[2].Content)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Abort Session", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		loop.Abort("abort-me")
		// In a real execution, loop would check aborted map
		assert.True(t, loop.aborted["abort-me"])
	})
}

func TestAgenticLoop_UpdateTools(t *testing.T) {
	mockProvider := &MockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(mockProvider, executor, "test")

	hasMetrics := false
	for _, tool := range loop.tools {
		if tool.Name == "pulse_get_metrics" {
			hasMetrics = true
			break
		}
	}
	assert.False(t, hasMetrics)

	executor.SetMetricsHistory(&mockMetricsHistory{})
	loop.UpdateTools()

	hasMetrics = false
	for _, tool := range loop.tools {
		if tool.Name == "pulse_get_metrics" {
			hasMetrics = true
			break
		}
	}
	assert.True(t, hasMetrics)
}

func TestAgenticLoop_AnswerQuestion(t *testing.T) {
	mockProvider := &MockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(mockProvider, executor, "test")

	t.Run("NoPendingQuestion", func(t *testing.T) {
		err := loop.AnswerQuestion("missing", nil)
		require.Error(t, err)
	})

	t.Run("AnswersDelivered", func(t *testing.T) {
		ch := make(chan []QuestionAnswer, 1)
		loop.pendingQs["q1"] = ch

		answers := []QuestionAnswer{{ID: "q", Value: "a"}}
		err := loop.AnswerQuestion("q1", answers)
		require.NoError(t, err)

		select {
		case got := <-ch:
			assert.Equal(t, answers, got)
		default:
			t.Fatal("expected answers to be delivered")
		}
	})

	t.Run("AlreadyAnswered", func(t *testing.T) {
		ch := make(chan []QuestionAnswer)
		loop.pendingQs["q2"] = ch

		err := loop.AnswerQuestion("q2", []QuestionAnswer{{ID: "q", Value: "a"}})
		require.Error(t, err)
	})
}

func TestWaitForApprovalDecision(t *testing.T) {
	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	require.NoError(t, err)

	t.Run("ContextCancelled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := waitForApprovalDecision(ctx, store, "missing")
		require.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := waitForApprovalDecision(ctx, store, "missing")
		require.Error(t, err)
	})

	t.Run("Approved", func(t *testing.T) {
		req := &approval.ApprovalRequest{
			ID:      "app-1",
			Command: "ls",
		}
		require.NoError(t, store.CreateApproval(req))

		go func() {
			time.Sleep(10 * time.Millisecond)
			_, _ = store.Approve("app-1", "tester")
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		decision, err := waitForApprovalDecision(ctx, store, "app-1")
		require.NoError(t, err)
		assert.Equal(t, approval.StatusApproved, decision.Status)
	})
}
