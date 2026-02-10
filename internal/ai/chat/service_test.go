package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock StateProvider
type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

// Mock CommandPolicy
type mockCommandPolicy struct{}

func (m *mockCommandPolicy) Evaluate(command string) agentexec.PolicyDecision {
	return agentexec.PolicyAllow
}

// Mock AgentServer
type mockAgentServer struct{}

func (m *mockAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	return []agentexec.ConnectedAgent{}
}

func (m *mockAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	return &agentexec.CommandResultPayload{}, nil
}

func hasTool(toolsList []tools.Tool, name string) bool {
	for _, tool := range toolsList {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func hasProviderTool(toolsList []providers.Tool, name string) bool {
	for _, tool := range toolsList {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func TestNewService(t *testing.T) {
	cfg := Config{
		AIConfig: &config.AIConfig{
			Enabled:      true,
			OpenAIAPIKey: "sk-test",
			ChatModel:    "openai:gpt-4",
		},
		StateProvider: &mockStateProvider{},
		Policy:        &mockCommandPolicy{},
		AgentServer:   &mockAgentServer{},
	}

	service := NewService(cfg)
	assert.NotNil(t, service)
	assert.NotNil(t, service.executor)
	assert.False(t, service.IsRunning())
}

func TestService_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		AIConfig: &config.AIConfig{
			Enabled:      true,
			OpenAIAPIKey: "sk-test",
			ChatModel:    "openai:gpt-4",
		},
		StateProvider: &mockStateProvider{},
		DataDir:       tmpDir,
	}

	service := NewService(cfg)
	ctx := context.Background()

	// Start
	err := service.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, service.IsRunning())
	assert.NotNil(t, service.sessions)
	assert.NotNil(t, service.agenticLoop)

	// Idempotent Start
	err = service.Start(ctx)
	assert.NoError(t, err)

	// Stop
	err = service.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, service.IsRunning())
}

func TestService_Restart(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		AIConfig: &config.AIConfig{
			Enabled:      true,
			OpenAIAPIKey: "sk-test",
			ChatModel:    "openai:gpt-4",
		},
		StateProvider: &mockStateProvider{},
		DataDir:       tmpDir,
	}

	service := NewService(cfg)
	ctx := context.Background()
	_ = service.Start(ctx)

	newCfg := &config.AIConfig{
		Enabled:         true,
		AnthropicAPIKey: "chk-test",
		ChatModel:       "anthropic:claude-3",
	}

	err := service.Restart(ctx, newCfg)
	assert.NoError(t, err)
	assert.True(t, service.IsRunning())
	assert.Equal(t, "anthropic:claude-3", service.cfg.GetChatModel())
}

func TestService_CreateProvider(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.AIConfig
		expectErr bool
	}{
		{
			name: "OpenAI",
			config: &config.AIConfig{
				OpenAIAPIKey: "sk-test",
				ChatModel:    "openai:gpt-4",
			},
			expectErr: false,
		},
		{
			name: "Anthropic",
			config: &config.AIConfig{
				AnthropicAPIKey: "chk-test",
				ChatModel:       "anthropic:claude-3",
			},
			expectErr: false,
		},
		{
			name: "Ollama",
			config: &config.AIConfig{
				OllamaBaseURL: "http://localhost:11434",
				ChatModel:     "ollama:llama3",
			},
			expectErr: false,
		},
		{
			name: "Missing Model",
			config: &config.AIConfig{
				ChatModel: "",
			},
			expectErr: true,
		},
		{
			name: "Invalid Model Format",
			config: &config.AIConfig{
				ChatModel: "gpt-4",
			},
			expectErr: true,
		},
		{
			name: "Unsupported Provider",
			config: &config.AIConfig{
				ChatModel: "unknown:model",
			},
			expectErr: true,
		},
		{
			name: "Missing API Key",
			config: &config.AIConfig{
				OpenAIAPIKey: "",
				ChatModel:    "openai:gpt-4",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{cfg: tt.config}
			p, err := s.createProvider()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, p)
			}
		})
	}
}

func TestService_ExecuteStream_Failures(t *testing.T) {
	service := &Service{}
	err := service.ExecuteStream(context.Background(), ExecuteRequest{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service not started")
}

func TestService_Start_Failures(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		s := &Service{cfg: nil}
		err := s.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Pulse Assistant config is nil")
	})

	t.Run("InvalidProvider", func(t *testing.T) {
		cfg := Config{
			AIConfig: &config.AIConfig{
				ChatModel: "unknown:model",
			},
		}
		s := NewService(cfg)
		err := s.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create provider")
	})
}

func TestService_SessionWrappers(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		AIConfig: &config.AIConfig{
			Enabled:      true,
			OpenAIAPIKey: "test-key",
			ChatModel:    "openai:gpt-4",
		},
		StateProvider: &mockStateProvider{},
		DataDir:       tmpDir,
	}

	service := NewService(cfg)
	ctx := context.Background()

	// Wrappers fail if service not started
	_, err := service.CreateSession(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service not started")

	// Start service
	err = service.Start(ctx)
	require.NoError(t, err)

	// Create Session
	session, err := service.CreateSession(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)

	// Note: Service does not expose GetSession or AddMessage directly in its public API
	// so we cannot test them as wrappers here. They are used internally by ExecuteStream.

	// Get Messages
	msgs, err := service.GetMessages(ctx, session.ID)
	require.NoError(t, err)
	assert.Empty(t, msgs) // Initially empty

	// List Sessions
	list, err := service.ListSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete Session
	err = service.DeleteSession(ctx, session.ID)
	require.NoError(t, err)

	// Verify deletion via List
	list, err = service.ListSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestService_Adapters(t *testing.T) {
	// Test StateProviderAdapter
	mockState := &mockStateProvider{
		state: models.StateSnapshot{
			Hosts: []models.Host{{Hostname: "host1"}},
		},
	}
	spAdapter := &stateProviderAdapter{sp: mockState}
	state := spAdapter.GetState()
	assert.Len(t, state.Hosts, 1)
	assert.Equal(t, "host1", state.Hosts[0].Hostname)

	// Test CommandPolicyAdapter
	mockPolicy := &mockCommandPolicy{}
	cpAdapter := &commandPolicyAdapter{p: mockPolicy}
	decision := cpAdapter.Evaluate("echo hello")
	assert.Equal(t, agentexec.PolicyAllow, decision)

	// Test AgentServerAdapter
	mockAgent := &mockAgentServer{}
	asAdapter := &agentServerAdapter{s: mockAgent}

	// GetConnectedAgents
	agents := asAdapter.GetConnectedAgents()
	assert.Empty(t, agents)

	// ExecuteCommand
	res, err := asAdapter.ExecuteCommand(context.Background(), "agent1", agentexec.ExecuteCommandPayload{})
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestService_ExtendedMethods(t *testing.T) {
	cfg := Config{
		AIConfig:      &config.AIConfig{Enabled: true},
		StateProvider: &mockStateProvider{},
	}
	service := NewService(cfg)
	ctx := context.Background()

	// 1. AnswerQuestion - fails if service not started
	err := service.AnswerQuestion(ctx, "q1", nil)
	assert.Error(t, err)

	// 2. Unimplemented methods return safely
	res, err := service.SummarizeSession(ctx, "s1")
	assert.NoError(t, err)
	assert.Equal(t, "not_implemented", res["status"])

	res, err = service.GetSessionDiff(ctx, "s1")
	assert.NoError(t, err)
	assert.Equal(t, "not_implemented", res["status"])

	res, err = service.RevertSession(ctx, "s1")
	assert.NoError(t, err)
	assert.Equal(t, "not_implemented", res["status"])

	res, err = service.UnrevertSession(ctx, "s1")
	assert.NoError(t, err)
	assert.Equal(t, "not_implemented", res["status"])

	// 3. ForkSession returns error
	_, err = service.ForkSession(ctx, "s1")
	assert.Error(t, err)

	// 4. GetBaseURL
	url := service.GetBaseURL()
	assert.Equal(t, "", url)

	// 5. AbortSession
	err = service.AbortSession(ctx, "s1")
	assert.Error(t, err) // Service not started
}
func TestService_SettersAndUpdateControlSettings(t *testing.T) {
	service := NewService(Config{
		AIConfig: &config.AIConfig{ControlLevel: config.ControlLevelReadOnly},
		// Agent server makes pulse_control eligible when control is enabled.
		AgentServer:   &mockAgentServer{},
		StateProvider: &mockStateProvider{},
	})

	require.NotNil(t, service.executor)
	// In read-only mode, pulse_control is not available
	assert.False(t, hasTool(service.executor.ListTools(), "pulse_control"))

	service.SetAlertProvider(nil)
	service.SetFindingsProvider(nil)
	service.SetBaselineProvider(nil)
	service.SetPatternProvider(nil)
	service.SetMetricsHistory(nil)
	service.SetBackupProvider(nil)
	service.SetDiskHealthProvider(nil)
	service.SetUpdatesProvider(nil)
	service.SetAgentProfileManager(nil)
	service.SetFindingsManager(nil)
	service.SetMetadataUpdater(nil)

	service.UpdateControlSettings(nil)
	service.UpdateControlSettings(&config.AIConfig{
		ControlLevel:    config.ControlLevelControlled,
		ProtectedGuests: []string{"101"},
	})

	// After updating to controlled mode, pulse_control should be available
	assert.True(t, hasTool(service.executor.ListTools(), "pulse_control"))
}

func TestService_FilterToolsForPrompt_ReadOnlyFiltersWriteTools(t *testing.T) {
	// Read-only prompts should not include write/control tools.
	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	// After tool consolidation: pulse_control handles commands and guest control,
	// pulse_docker handles Docker operations
	require.True(t, hasTool(service.executor.ListTools(), "pulse_control"))
	require.True(t, hasTool(service.executor.ListTools(), "pulse_docker"))
	require.True(t, hasTool(service.executor.ListTools(), "pulse_query"))

	// Read-only prompts should exclude write tools
	filtered := service.filterToolsForPrompt(context.Background(), "run uptime", false)
	assert.False(t, hasProviderTool(filtered, "pulse_control"))
	assert.False(t, hasProviderTool(filtered, "pulse_docker"))
	assert.True(t, hasProviderTool(filtered, "pulse_query"))
}

func TestService_FilterToolsForPrompt_WriteIntentIncludesWriteTools(t *testing.T) {
	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	require.True(t, hasTool(service.executor.ListTools(), "pulse_control"))
	require.True(t, hasTool(service.executor.ListTools(), "pulse_docker"))
	require.True(t, hasTool(service.executor.ListTools(), "pulse_query"))

	filtered := service.filterToolsForPrompt(context.Background(), "restart vm 101", false)
	assert.True(t, hasProviderTool(filtered, "pulse_control"))
	assert.True(t, hasProviderTool(filtered, "pulse_docker"))
	assert.True(t, hasProviderTool(filtered, "pulse_query"))
}

func TestService_Execute_NonStreaming(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	require.NoError(t, err)

	mockProvider := &MockProvider{}
	mockProvider.On("ChatStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			callback := args.Get(2).(providers.StreamCallback)
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Hello"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
		}).
		Once()

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	service := &Service{
		executor:    executor,
		sessions:    store,
		agenticLoop: NewAgenticLoop(mockProvider, executor, "prompt"),
		provider:    mockProvider,
		started:     true,
	}

	resp, err := service.Execute(context.Background(), ExecuteRequest{Prompt: "Hi"})
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp["content"])
	mockProvider.AssertExpectations(t)
}
