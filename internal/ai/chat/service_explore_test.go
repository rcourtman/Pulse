package chat

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestService_FilterToolsForExplorePrompt_ForcesReadOnly(t *testing.T) {
	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})

	filtered := service.filterToolsForExplorePrompt(context.Background(), "restart vm 101 and edit config")

	if hasProviderTool(filtered, "pulse_control") {
		t.Fatal("expected pulse_control to be excluded for explore pre-pass")
	}
	if hasProviderTool(filtered, "pulse_docker") {
		t.Fatal("expected pulse_docker to be excluded for explore pre-pass")
	}
	if hasProviderTool(filtered, "pulse_file_edit") {
		t.Fatal("expected pulse_file_edit to be excluded for explore pre-pass")
	}
	if hasProviderTool(filtered, pulseQuestionToolName) {
		t.Fatal("expected pulse_question to be excluded for explore pre-pass")
	}
	if !hasProviderTool(filtered, "pulse_query") {
		t.Fatal("expected pulse_query to remain available for explore pre-pass")
	}
}

func TestService_ExecuteStream_ExplorePrepassEnabled(t *testing.T) {
	t.Setenv(exploreEnabledEnvVar, "true")

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	exploreCalls := 0
	exploreProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			exploreCalls++
			for _, tool := range req.Tools {
				if isWriteTool(tool.Name) {
					return fmt.Errorf("explore received write tool: %s", tool.Name)
				}
				if tool.Name == pulseQuestionToolName {
					return fmt.Errorf("explore received interactive tool: %s", tool.Name)
				}
			}
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Scope\n- vm 101\n\nFindings\n- service is reachable"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	mainCalls := 0
	mainSawExploreSummary := false
	mainProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			mainCalls++
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if msg.Role != "user" || msg.ToolResult != nil {
					continue
				}
				if strings.Contains(msg.Content, "Explore findings (read-only pre-pass):") {
					mainSawExploreSummary = true
				}
				break
			}
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Main response"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	svc := NewService(Config{
		AIConfig: &config.AIConfig{
			ControlLevel:   config.ControlLevelControlled,
			ChatModel:      "openai:chat-main",
			DiscoveryModel: "openai:explore-fast",
			OpenAIAPIKey:   "sk-test",
		},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	svc.sessions = store
	svc.provider = mainProvider
	svc.started = true
	svc.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		if modelStr == "openai:explore-fast" {
			return exploreProvider, nil
		}
		return nil, fmt.Errorf("unexpected model for providerFactory: %s", modelStr)
	}

	doneCount := 0
	req := ExecuteRequest{
		SessionID: "explore-enabled-session",
		Prompt:    "restart vm 101",
	}
	err = svc.ExecuteStream(context.Background(), req, func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if exploreCalls != 1 {
		t.Fatalf("expected explore pre-pass to run once, got %d", exploreCalls)
	}
	if mainCalls != 1 {
		t.Fatalf("expected main loop to run once, got %d", mainCalls)
	}
	if !mainSawExploreSummary {
		t.Fatal("expected main loop request to include injected explore summary")
	}
	if doneCount != 1 {
		t.Fatalf("expected done callback once, got %d", doneCount)
	}
}

func TestService_ExecuteStream_ExplorePrepassDisabledByEnv(t *testing.T) {
	t.Setenv(exploreEnabledEnvVar, "false")

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	exploreCalls := 0
	mainCalls := 0
	mainSawExploreSummary := false
	mainProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			mainCalls++
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if msg.Role != "user" || msg.ToolResult != nil {
					continue
				}
				if strings.Contains(msg.Content, "Explore findings (read-only pre-pass):") {
					mainSawExploreSummary = true
				}
				break
			}
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Main response"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	svc := NewService(Config{
		AIConfig: &config.AIConfig{
			ControlLevel:   config.ControlLevelControlled,
			ChatModel:      "openai:chat-main",
			DiscoveryModel: "openai:explore-fast",
			OpenAIAPIKey:   "sk-test",
		},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	svc.sessions = store
	svc.provider = mainProvider
	svc.started = true
	svc.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		exploreCalls++
		return nil, fmt.Errorf("providerFactory should not be called when explore is disabled: %s", modelStr)
	}

	err = svc.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "explore-disabled-session",
		Prompt:    "check vm 101",
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if exploreCalls != 0 {
		t.Fatalf("expected no explore provider calls, got %d", exploreCalls)
	}
	if mainCalls != 1 {
		t.Fatalf("expected main loop to run once, got %d", mainCalls)
	}
	if mainSawExploreSummary {
		t.Fatal("did not expect explore summary injection when explore is disabled")
	}
}
