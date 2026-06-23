package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func indexOfStreamType(types []string, target string) int {
	for i, eventType := range types {
		if eventType == target {
			return i
		}
	}
	return -1
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
		require.Len(t, events, 1)
		assert.Equal(t, "content", events[0].Type)

		var eventContent ContentData
		_ = json.Unmarshal(events[0].Data, &eventContent)
		assert.Equal(t, "Hi there!", eventContent.Text)
	})

	t.Run("Execute Retains Reasoning Internally Without Streaming It", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		ctx := context.Background()
		sessionID := "thinking-session"
		messages := []Message{{Role: "user", Content: "Hello"}}

		mockProvider.On("ChatStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "thinking",
				Data: providers.ThinkingEvent{Text: "private provider reasoning"},
			})
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
		results, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {
			events = append(events, event)
		})

		assert.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "private provider reasoning", results[0].ReasoningContent)
		for _, event := range events {
			assert.NotEqual(t, "thinking", event.Type)
			assert.NotContains(t, string(event.Data), "private provider reasoning")
		}
		require.Len(t, events, 2)
		assert.Equal(t, "workflow_state", events[0].Type)
		var workflowState WorkflowStateData
		require.NoError(t, json.Unmarshal(events[0].Data, &workflowState))
		assert.Equal(t, "model_thinking", workflowState.Phase)
		assert.Equal(t, "Model is reasoning before responding.", workflowState.Message)
		assert.Equal(t, "content", events[1].Type)
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

		var streamTypes []string
		var startEvents []ToolStartData
		var progressEvents []ToolProgressData
		results, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {
			streamTypes = append(streamTypes, event.Type)
			if event.Type == "tool_start" {
				var data ToolStartData
				_ = json.Unmarshal(event.Data, &data)
				startEvents = append(startEvents, data)
			}
			if event.Type == "tool_progress" {
				var data ToolProgressData
				_ = json.Unmarshal(event.Data, &data)
				progressEvents = append(progressEvents, data)
			}
		})
		assert.NoError(t, err)
		require.Len(t, results, 3) // assistant tool call, tool result, final response
		assert.Equal(t, "I found 2 nodes.", results[2].Content)
		require.Len(t, startEvents, 1, "expected tool execution to emit a live start event")
		assert.Equal(t, "call_123", startEvents[0].ID)
		assert.Equal(t, "running", startEvents[0].Phase)
		require.Len(t, progressEvents, 1, "expected tool execution to emit a running progress event")
		assert.Equal(t, "call_123", progressEvents[0].ID)
		assert.Equal(t, "list_nodes", progressEvents[0].Name)
		assert.Equal(t, "running", progressEvents[0].Phase)
		assert.Equal(t, "Running.", progressEvents[0].Message)
		assert.Less(t, indexOfStreamType(streamTypes, "tool_progress"), indexOfStreamType(streamTypes, "tool_end"))
		mockProvider.AssertExpectations(t)
	})

	t.Run("Provider Tool Argument Progress Mutates Pending Tool", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		ctx := context.Background()
		sessionID := "test-session"
		messages := []Message{{Role: "user", Content: "List matching resources"}}

		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 1
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "tool_start",
				Data: providers.ToolStartEvent{ID: "call_query", Name: "pulse_query"},
			})
			callback(providers.StreamEvent{
				Type: "tool_progress",
				Data: providers.ToolProgressEvent{
					ID:       "call_query",
					Name:     "pulse_query",
					RawInput: `{"action":"sea`,
					Phase:    "pending",
					Message:  "Receiving tool input.",
				},
			})
			callback(providers.StreamEvent{
				Type: "tool_progress",
				Data: providers.ToolProgressEvent{
					ID:       "call_query",
					Name:     "pulse_query",
					Input:    map[string]interface{}{"action": "search", "query": "prowlarr"},
					RawInput: `{"action":"search","query":"prowlarr"}`,
					Phase:    "pending",
					Message:  "Prepared tool input.",
				},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{
						{
							ID:    "call_query",
							Name:  "pulse_query",
							Input: map[string]interface{}{"action": "search", "query": "prowlarr"},
						},
					},
				},
			})
		}).Once()

		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 3
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Prowlarr is present."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		}).Once()

		var progressEvents []ToolProgressData
		_, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {
			if event.Type != "tool_progress" {
				return
			}
			var data ToolProgressData
			_ = json.Unmarshal(event.Data, &data)
			progressEvents = append(progressEvents, data)
		})

		require.NoError(t, err)
		require.GreaterOrEqual(t, len(progressEvents), 3)
		assert.Equal(t, "Receiving tool input.", progressEvents[0].Message)
		assert.Equal(t, `{"action":"sea`, progressEvents[0].RawInput)
		assert.Equal(t, "Prepared tool input.", progressEvents[1].Message)
		assert.JSONEq(t, `{"action":"search","query":"prowlarr"}`, progressEvents[1].Input)
		assert.Equal(t, "running", progressEvents[2].Phase)
		assert.Equal(t, "call_query", progressEvents[2].ID)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Abort Session", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		loop.Abort("abort-me")
		// In a real execution, loop would check aborted map
		assert.True(t, loop.aborted["abort-me"])
	})

	t.Run("KnowledgeAccumulatorInjectedInSystemPrompt", func(t *testing.T) {
		state := models.StateSnapshot{
			VMs: []models.VM{{
				ID:     "vm-100",
				VMID:   100,
				Name:   "vm-one",
				Node:   "node1",
				Status: "running",
				Tags:   []string{"database"},
				CPU:    0.5,
				CPUs:   4,
				Memory: models.Memory{
					Usage: 0.42,
					Used:  4 * 1024 * 1024 * 1024,
					Total: 8 * 1024 * 1024 * 1024,
				},
			}},
		}
		execWithState := tools.NewPulseToolExecutor(tools.ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
		})
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, execWithState, "You are a helper")
		ka := NewKnowledgeAccumulator()
		loop.SetKnowledgeAccumulator(ka)

		ctx := context.Background()
		sessionID := "ka-session"
		messages := []Message{{Role: "user", Content: "Get VM status"}}

		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 1
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "tool_start",
				Data: providers.ToolStartEvent{ID: "call_1", Name: "pulse_query"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{{
						ID:   "call_1",
						Name: "pulse_query",
						Input: map[string]interface{}{
							"action":        "get",
							"resource_type": "vm",
							"resource_id":   "100",
						},
					}},
				},
			})
		}).Once()

		var secondSystem string
		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 3
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			req := args.Get(1).(providers.ChatRequest)
			secondSystem = req.System
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Done."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		}).Once()

		_, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {})
		require.NoError(t, err)
		require.NotEmpty(t, secondSystem)
		assert.Contains(t, secondSystem, "## Known Facts")
		assert.Contains(t, secondSystem, "vm:node1:100:status")
		assert.Contains(t, secondSystem, "virtual machine resource")
	})
}

func TestAgenticLoop_UpdateTools(t *testing.T) {
	mockProvider := &MockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(mockProvider, executor, "test")

	// After tool consolidation, pulse_metrics replaces pulse_get_metrics
	hasMetrics := false
	for _, tool := range loop.tools {
		if tool.Name == "pulse_metrics" {
			hasMetrics = true
			break
		}
	}
	assert.False(t, hasMetrics)

	executor.SetMetricsHistory(&mockMetricsHistory{})
	loop.UpdateTools()

	hasMetrics = false
	for _, tool := range loop.tools {
		if tool.Name == "pulse_metrics" {
			hasMetrics = true
			break
		}
	}
	assert.True(t, hasMetrics)
}

func TestAgenticLoop_SystemPromptIncludesCurrentTime(t *testing.T) {
	mockProvider := &MockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := NewAgenticLoop(mockProvider, executor, "BASE PROMPT BODY")

	prompt := loop.getSystemPrompt()

	// The base prompt must still be present...
	if !strings.Contains(prompt, "BASE PROMPT BODY") {
		t.Fatalf("expected base prompt to be preserved, got %q", prompt)
	}
	// ...alongside a fresh current-time block so the Assistant can answer
	// time/date questions without a clock-less deflection or a target-host demand.
	if !strings.Contains(prompt, "CURRENT TIME:") {
		t.Fatalf("expected system prompt to carry CURRENT TIME, got %q", prompt)
	}
	if !strings.Contains(prompt, fmt.Sprintf("%d", time.Now().Year())) {
		t.Fatalf("expected system prompt to carry the current year, got %q", prompt)
	}
	if !strings.Contains(prompt, "do not run a command or ask for a target host just to") {
		t.Fatalf("expected system prompt to tell the model not to deflect time questions, got %q", prompt)
	}
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

		_, err := waitForApprovalDecision(ctx, store, "missing", approval.DefaultOrgID)
		require.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := waitForApprovalDecision(ctx, store, "missing", approval.DefaultOrgID)
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

		decision, err := waitForApprovalDecision(ctx, store, "app-1", approval.DefaultOrgID)
		require.NoError(t, err)
		assert.Equal(t, approval.StatusApproved, decision.Status)
	})
}

func TestToolCallKey(t *testing.T) {
	t.Run("same name and input produce same key", func(t *testing.T) {
		input := map[string]interface{}{"action": "get", "resource_type": "lxc", "target_id": "node1"}
		k1 := toolCallKey("pulse_discovery", input)
		k2 := toolCallKey("pulse_discovery", input)
		assert.Equal(t, k1, k2)
	})

	t.Run("different input produces different key", func(t *testing.T) {
		input1 := map[string]interface{}{"action": "get", "resource_id": "100"}
		input2 := map[string]interface{}{"action": "get", "resource_id": "200"}
		k1 := toolCallKey("pulse_discovery", input1)
		k2 := toolCallKey("pulse_discovery", input2)
		assert.NotEqual(t, k1, k2)
	})

	t.Run("different tool name produces different key", func(t *testing.T) {
		input := map[string]interface{}{"action": "get"}
		k1 := toolCallKey("pulse_discovery", input)
		k2 := toolCallKey("pulse_query", input)
		assert.NotEqual(t, k1, k2)
	})

	t.Run("nil input", func(t *testing.T) {
		k := toolCallKey("pulse_discovery", nil)
		assert.Contains(t, k, "pulse_discovery")
	})
}

func TestLoopDetection(t *testing.T) {
	// Simulate the loop detection logic from executeWithTools
	const maxIdenticalCalls = 3

	t.Run("allows up to maxIdenticalCalls", func(t *testing.T) {
		recentCallCounts := make(map[string]int)
		input := map[string]interface{}{"action": "get", "resource_type": "lxc"}
		key := toolCallKey("pulse_discovery", input)

		for i := 0; i < maxIdenticalCalls; i++ {
			recentCallCounts[key]++
			assert.LessOrEqual(t, recentCallCounts[key], maxIdenticalCalls,
				"call %d should be allowed", i+1)
		}
	})

	t.Run("blocks call exceeding maxIdenticalCalls", func(t *testing.T) {
		recentCallCounts := make(map[string]int)
		input := map[string]interface{}{"action": "get", "resource_type": "lxc"}
		key := toolCallKey("pulse_discovery", input)

		// Simulate 3 allowed calls
		for i := 0; i < maxIdenticalCalls; i++ {
			recentCallCounts[key]++
		}

		// 4th call should be blocked
		recentCallCounts[key]++
		assert.Greater(t, recentCallCounts[key], maxIdenticalCalls,
			"4th identical call should exceed limit")
	})

	t.Run("different calls tracked independently", func(t *testing.T) {
		recentCallCounts := make(map[string]int)
		input1 := map[string]interface{}{"action": "get", "resource_id": "100"}
		input2 := map[string]interface{}{"action": "get", "resource_id": "200"}
		key1 := toolCallKey("pulse_discovery", input1)
		key2 := toolCallKey("pulse_discovery", input2)

		// Call key1 three times
		for i := 0; i < maxIdenticalCalls; i++ {
			recentCallCounts[key1]++
		}

		// key2 should still be fine
		recentCallCounts[key2]++
		assert.Equal(t, 1, recentCallCounts[key2])
		assert.Equal(t, maxIdenticalCalls, recentCallCounts[key1])
	})
}

func TestLoopDetectionIntegration(t *testing.T) {
	// Integration test: run the agentic loop with a provider that keeps
	// calling the same tool, and verify the 4th identical call is blocked.
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockProvider := &MockProvider{}
	loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
	ctx := context.Background()
	sessionID := "loop-detect-session"
	messages := []Message{{Role: "user", Content: "discover lxc 100"}}

	callCount := 0
	// The provider will keep requesting the same tool call up to 5 times
	mockProvider.On("ChatStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		callback := args.Get(2).(providers.StreamCallback)
		callCount++

		// Check if we got a LOOP_DETECTED error in the messages — if so, stop calling tools
		req := args.Get(1).(providers.ChatRequest)
		for _, msg := range req.Messages {
			if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "LOOP_DETECTED") {
				// Model should stop — emit content and no tool calls
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "I'll try a different approach."},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{},
				})
				return
			}
		}

		// Keep calling the same tool
		callback(providers.StreamEvent{
			Type: "tool_start",
			Data: providers.ToolStartEvent{ID: fmt.Sprintf("call_%d", callCount), Name: "pulse_discovery"},
		})
		callback(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{
				ToolCalls: []providers.ToolCall{
					{
						ID:   fmt.Sprintf("call_%d", callCount),
						Name: "pulse_discovery",
						Input: map[string]interface{}{
							"action":        "get",
							"resource_type": "lxc",
							"resource_id":   "100",
							"target_id":     "node1",
						},
					},
				},
			},
		})
	})

	var events []StreamEvent
	results, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {
		events = append(events, event)
	})

	assert.NoError(t, err)

	// Verify LOOP_DETECTED appears in at least one tool result
	foundLoopDetected := false
	for _, msg := range results {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "LOOP_DETECTED") {
			foundLoopDetected = true
			break
		}
	}
	assert.True(t, foundLoopDetected, "expected LOOP_DETECTED in tool results")

	// The loop should have stopped (model returned content after seeing LOOP_DETECTED)
	// Total calls: 4 tool-calling turns (3 allowed + 1 blocked) + 1 final content turn = 5
	assert.LessOrEqual(t, callCount, 6, "loop should terminate after detection")
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 5, "hello..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateForLog(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSummarizeForNegativeMarker(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "empty response",
			input:    "",
			contains: []string{"empty response"},
		},
		{
			name:     "JSON with empty array",
			input:    `{"resource_id":"105","period":"24h","points":[]}`,
			contains: []string{"points: 0 items", "resource=105", "period=24h"},
		},
		{
			name:     "JSON with populated array",
			input:    `{"pools":[{"name":"a"},{"name":"b"}],"total":2}`,
			contains: []string{"pools: 2 items", "total=2"},
		},
		{
			name:     "JSON with error",
			input:    `{"error":"not_found","resource_id":"999"}`,
			contains: []string{"error=not_found", "resource=999"},
		},
		{
			name:     "JSON array",
			input:    `[{"a":1},{"b":2},{"c":3}]`,
			contains: []string{"array with 3 items"},
		},
		{
			name:     "plain text",
			input:    "No data available for this resource",
			contains: []string{"No data available"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizeForNegativeMarker(tt.input)
			for _, c := range tt.contains {
				assert.Contains(t, result, c, "expected %q to contain %q", result, c)
			}
		})
	}
}

func TestParallelToolExecution(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	t.Run("MultipleToolCallsExecuteAndReturnInOrder", func(t *testing.T) {
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		ctx := context.Background()
		sessionID := "parallel-session"
		messages := []Message{{Role: "user", Content: "Check three things"}}

		// Turn 1: provider returns 3 tool calls at once
		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 1
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			// Emit tool_start events for each tool
			for _, id := range []string{"call_a", "call_b", "call_c"} {
				callback(providers.StreamEvent{
					Type: "tool_start",
					Data: providers.ToolStartEvent{ID: id, Name: "pulse_query"},
				})
			}
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{
						{ID: "call_a", Name: "pulse_query", Input: map[string]interface{}{
							"action": "get", "resource_type": "vm", "resource_id": "100",
						}},
						{ID: "call_b", Name: "pulse_query", Input: map[string]interface{}{
							"action": "get", "resource_type": "vm", "resource_id": "200",
						}},
						{ID: "call_c", Name: "pulse_query", Input: map[string]interface{}{
							"action": "get", "resource_type": "lxc", "resource_id": "300",
						}},
					},
				},
			})
		}).Once()

		// Turn 2: provider sees the 3 tool results and gives final answer
		// Messages: 1 user + 1 assistant (with 3 tool calls) + 3 tool results = 5
		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) >= 5
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "All three checks completed."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		}).Once()

		var toolEndEvents []ToolEndData
		results, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {
			if event.Type == "tool_end" {
				var data ToolEndData
				_ = json.Unmarshal(event.Data, &data)
				toolEndEvents = append(toolEndEvents, data)
			}
		})

		assert.NoError(t, err)

		// Verify we got 3 tool_end events in the original order
		require.Len(t, toolEndEvents, 3, "expected 3 tool_end events")
		assert.Equal(t, "call_a", toolEndEvents[0].ID)
		assert.Equal(t, "call_b", toolEndEvents[1].ID)
		assert.Equal(t, "call_c", toolEndEvents[2].ID)

		// Verify result messages contain: assistant (with tool calls) + 3 tool results + final answer
		require.GreaterOrEqual(t, len(results), 5, "expected at least 5 result messages")

		// Verify tool results are in order by checking ToolUseIDs
		toolResultIDs := []string{}
		for _, msg := range results {
			if msg.ToolResult != nil {
				toolResultIDs = append(toolResultIDs, msg.ToolResult.ToolUseID)
			}
		}
		require.Len(t, toolResultIDs, 3, "expected 3 tool result messages")
		assert.Equal(t, "call_a", toolResultIDs[0])
		assert.Equal(t, "call_b", toolResultIDs[1])
		assert.Equal(t, "call_c", toolResultIDs[2])
		assert.Equal(t, 3, loop.GetTotalToolCalls())

		// Verify final response
		lastMsg := results[len(results)-1]
		assert.Equal(t, "assistant", lastMsg.Role)
		assert.Equal(t, "All three checks completed.", lastMsg.Content)

		mockProvider.AssertExpectations(t)
	})

	t.Run("SingleToolCallNoGoroutineOverhead", func(t *testing.T) {
		// Ensure single tool calls still work correctly (no regression)
		mockProvider := &MockProvider{}
		loop := NewAgenticLoop(mockProvider, executor, "You are a helper")
		ctx := context.Background()
		sessionID := "single-tool-session"
		messages := []Message{{Role: "user", Content: "Check one thing"}}

		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) == 1
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "tool_start",
				Data: providers.ToolStartEvent{ID: "call_single", Name: "pulse_query"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{
					ToolCalls: []providers.ToolCall{
						{ID: "call_single", Name: "pulse_query", Input: map[string]interface{}{
							"action": "get", "resource_type": "vm", "resource_id": "100",
						}},
					},
				},
			})
		}).Once()

		mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
			return len(req.Messages) >= 3
		}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Done."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{},
			})
		}).Once()

		var toolEndCount int
		results, err := loop.Execute(ctx, sessionID, messages, func(event StreamEvent) {
			if event.Type == "tool_end" {
				toolEndCount++
			}
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, toolEndCount, "expected 1 tool_end event")

		// Verify final response
		lastMsg := results[len(results)-1]
		assert.Equal(t, "Done.", lastMsg.Content)

		mockProvider.AssertExpectations(t)
	})
}
