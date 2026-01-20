package chat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockProvider struct {
	mock.Mock
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
