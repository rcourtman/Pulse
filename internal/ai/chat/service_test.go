package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
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
			Enabled:         true,
			OpenAIAPIKey:    "sk-test",
			ChatModel:       "openai:gpt-4",
			OpenCodeDataDir: tmpDir,
		},
		StateProvider: &mockStateProvider{},
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
			Enabled:         true,
			OpenAIAPIKey:    "sk-test",
			ChatModel:       "openai:gpt-4",
			OpenCodeDataDir: tmpDir,
		},
		StateProvider: &mockStateProvider{},
	}

	service := NewService(cfg)
	ctx := context.Background()
	_ = service.Start(ctx)

	newCfg := &config.AIConfig{
		Enabled:         true,
		AnthropicAPIKey: "chk-test",
		ChatModel:       "anthropic:claude-3",
		OpenCodeDataDir: tmpDir,
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
		assert.Contains(t, err.Error(), "AI config is nil")
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
			Enabled:         true,
			OpenAIAPIKey:    "test-key",
			ChatModel:       "openai:gpt-4",
			OpenCodeDataDir: tmpDir,
		},
		StateProvider: &mockStateProvider{},
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
func TestParseRunCommandDecision(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    bool
		wantErr bool
	}{
		{
			name:    "Allow true",
			payload: `{"allow_run_command": true, "reason": "explicit"}`,
			want:    true,
		},
		{
			name:    "Allow false",
			payload: `{"allow_run_command": false}`,
			want:    false,
		},
		{
			name:    "Bare true",
			payload: "true",
			want:    true,
		},
		{
			name:    "Wrapped JSON",
			payload: "Decision: {\"allow_run_command\": true}",
			want:    true,
		},
		{
			name:    "Invalid payload",
			payload: "n/a",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRunCommandDecision(tc.payload)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Allow)
		})
	}
}
