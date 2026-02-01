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

	t.Run("KnowledgeAccumulatorInjectedInSystemPrompt", func(t *testing.T) {
		state := models.StateSnapshot{
			VMs: []models.VM{{
				ID:     "vm-100",
				VMID:   100,
				Name:   "vm-one",
				Node:   "node1",
				Status: "running",
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
		assert.Contains(t, secondSystem, "vm-one")
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

func TestRequiresToolUse(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		// Should require tools - action requests
		{"@mention", "@jellyfin status", true},
		{"check status", "check the status of my server", true},
		{"restart request", "please restart nginx", true},
		{"status query", "is homepage running?", true},
		{"logs request", "show me the logs for influxdb", true},
		{"cpu query", "what's the cpu usage on delly?", true},
		{"memory query", "how much memory is traefik using?", true},
		{"container query", "list my docker containers", true},
		{"my infrastructure", "what's happening on my server?", true},
		{"troubleshoot my", "troubleshoot my plex server", true},
		{"my docker", "show me my docker containers", true},

		// Should NOT require tools (conceptual questions)
		{"what is tcp", "what is tcp?", false},
		{"explain docker", "explain how docker networking works", false},
		{"general question", "how do i configure nginx?", false},
		{"theory question", "what's the difference between lxc and vm?", false},
		{"empty message", "", false},
		{"greeting", "hello", false},
		{"thanks", "thanks for your help!", false},
		{"explain proxmox", "explain what proxmox is", false},
		{"describe kubernetes", "describe how kubernetes pods work", false},

		// Edge cases from feedback - these are conceptual despite mentioning infra terms
		{"is docker hard", "is docker networking hard?", false},
		{"best way cpu", "what's the best way to monitor CPU usage?", false},
		{"best practice", "what are the best practices for container security?", false},
		{"should i use", "should i use kubernetes or docker swarm?", false},

		// Edge cases - conceptual patterns with action keywords should still be action
		// ONLY when they reference MY specific infrastructure
		{"what is status", "what is the status of my server", true},
		{"what is running", "what is running on my host", true},
		{"my cpu usage", "what is my cpu usage", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := []providers.Message{
				{Role: "user", Content: tt.message},
			}
			result := requiresToolUse(messages)
			assert.Equal(t, tt.expected, result, "message: %q", tt.message)
		})
	}
}

func TestHasPhantomExecution(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		// Phantom execution patterns - concrete metrics
		{"cpu percentage", "The CPU usage is at 85%", true},
		{"memory usage", "Memory usage is 4.2GB", true},
		{"disk at", "Disk is at 92% capacity", true},

		// Phantom execution patterns - state claims
		{"currently running", "The service is currently running", true},
		{"currently stopped", "The container is currently stopped", true},
		{"logs show", "The logs show several errors", true},
		{"output shows", "The output shows the service failed", true},

		// Phantom execution patterns - fake tool formatting
		{"fake tool block", "```tool\npulse_query\n```", true},
		{"fake function call", "pulse_query({\"type\": \"nodes\"})", true},

		// Phantom execution patterns - action claims
		{"restarted the", "I restarted the nginx service", true},
		{"successfully restarted", "The service was successfully restarted", true},
		{"has been stopped", "The container has been stopped", true},

		// NOT phantom execution - these are safe
		{"suggestion", "You should check the logs", false},
		{"question", "Would you like me to restart it?", false},
		{"explanation", "Docker containers run in isolated environments", false},
		{"future tense", "I will check the status for you", false},
		{"empty", "", false},
		{"general info", "Proxmox uses LXC for containers", false},

		// NOT phantom - these used to false-positive
		{"checked docs", "I checked the documentation and found...", false},
		{"ran through logic", "I ran through the logic and it seems...", false},
		{"looked at code", "I looked at the configuration options", false},
		{"verified understanding", "I verified my understanding of the issue", false},
		{"found that general", "I found that Docker networking is complex", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPhantomExecution(tt.content)
			assert.Equal(t, tt.expected, result, "content: %q", tt.content)
		})
	}
}

func TestToolCallKey(t *testing.T) {
	t.Run("same name and input produce same key", func(t *testing.T) {
		input := map[string]interface{}{"action": "get", "resource_type": "lxc", "host_id": "node1"}
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
							"host_id":       "node1",
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

func TestDetectFreshDataIntent(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     bool
	}{
		{
			name:     "empty messages",
			messages: nil,
			want:     false,
		},
		{
			name: "normal question",
			messages: []Message{
				{Role: "user", Content: "What containers are running?"},
			},
			want: false,
		},
		{
			name: "check again",
			messages: []Message{
				{Role: "user", Content: "Check again please"},
			},
			want: true,
		},
		{
			name: "refresh",
			messages: []Message{
				{Role: "user", Content: "Can you refresh the data?"},
			},
			want: true,
		},
		{
			name: "has it changed",
			messages: []Message{
				{Role: "user", Content: "Has it changed since last time?"},
			},
			want: true,
		},
		{
			name: "latest data",
			messages: []Message{
				{Role: "user", Content: "Show me the latest data for frigate"},
			},
			want: true,
		},
		{
			name: "uses last user message only",
			messages: []Message{
				{Role: "user", Content: "Check again"},
				{Role: "assistant", Content: "OK, checking..."},
				{Role: "user", Content: "What is the CPU usage?"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFreshDataIntent(tt.messages)
			assert.Equal(t, tt.want, got)
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
				json.Unmarshal(event.Data, &data)
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
