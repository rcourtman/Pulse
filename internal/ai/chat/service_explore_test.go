package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	statusEvents := 0
	statusSawStart := false
	statusSawDone := false
	mainProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			mainCalls++
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if msg.Role != "user" || msg.ToolResult != nil {
					continue
				}
				if strings.Contains(msg.Content, "<explore_context>") {
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
		if event.Type == "explore_status" {
			statusEvents++
			var data ExploreStatusData
			_ = json.Unmarshal(event.Data, &data)
			if data.Phase == "started" {
				statusSawStart = true
			}
			if data.Phase == "completed" {
				statusSawDone = true
			}
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
	if statusEvents == 0 {
		t.Fatal("expected explore pre-pass to emit visible explore status events")
	}
	if !statusSawStart {
		t.Fatal("expected explore status start event")
	}
	if !statusSawDone {
		t.Fatal("expected explore completion status event")
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
				if strings.Contains(msg.Content, "<explore_context>") {
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

func TestService_ExecuteStream_ExplorePrepassSkipsWithoutExplicitModel(t *testing.T) {
	t.Setenv(exploreEnabledEnvVar, "true")

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	exploreCalls := 0
	mainCalls := 0
	mainSawExploreSummary := false
	sawSkipped := false
	mainProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			mainCalls++
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if msg.Role != "user" || msg.ToolResult != nil {
					continue
				}
				if strings.Contains(msg.Content, "<explore_context>") {
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
			ControlLevel: config.ControlLevelControlled,
			OpenAIAPIKey: "sk-test",
			// No explicit Model/ChatModel/DiscoveryModel -> Explore must skip.
		},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	svc.sessions = store
	svc.provider = mainProvider
	svc.started = true
	svc.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		exploreCalls++
		return nil, fmt.Errorf("explore should not attempt provider creation without explicit model: %s", modelStr)
	}

	err = svc.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "explore-no-explicit-model-session",
		Prompt:    "check vm 101",
	}, func(event StreamEvent) {
		if event.Type != "explore_status" {
			return
		}
		var data ExploreStatusData
		_ = json.Unmarshal(event.Data, &data)
		if data.Phase == "skipped" && data.Outcome == exploreOutcomeSkippedModel {
			sawSkipped = true
		}
	})
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
		t.Fatal("did not expect explore summary injection when no explicit explore model is available")
	}
	if !sawSkipped {
		t.Fatal("expected skipped explore status event when no explicit model is configured")
	}
}

func TestResolveExploreProvider_FallsBackToNextExplicitModel(t *testing.T) {
	svc := NewService(Config{
		AIConfig: &config.AIConfig{
			// malformed candidate should be skipped
			DiscoveryModel: "bad-model-format",
			ChatModel:      "openai:chat-main",
			OpenAIAPIKey:   "sk-test",
		},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})

	var picked string
	svc.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		picked = modelStr
		return &stubServiceProvider{}, nil
	}

	provider, model := svc.resolveExploreProvider("", "", nil)
	if provider == nil {
		t.Fatal("expected explore provider resolution to succeed")
	}
	if model != "openai:chat-main" {
		t.Fatalf("expected fallback to explicit chat model, got %q", model)
	}
	if picked != "openai:chat-main" {
		t.Fatalf("expected provider factory to receive fallback model, got %q", picked)
	}
}

func TestService_ExecuteStream_ExplorePrepassFailureStillRunsMain(t *testing.T) {
	t.Setenv(exploreEnabledEnvVar, "true")

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	exploreCalls := 0
	mainCalls := 0
	mainSawExploreSummary := false
	sawStarted := false
	sawFailed := false

	exploreProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			exploreCalls++
			return fmt.Errorf("explore provider unavailable")
		},
	}
	mainProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			mainCalls++
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if msg.Role != "user" || msg.ToolResult != nil {
					continue
				}
				if strings.Contains(msg.Content, "<explore_context>") {
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
	err = svc.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "explore-failure-session",
		Prompt:    "check vm 101",
	}, func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
		if event.Type != "explore_status" {
			return
		}
		var data ExploreStatusData
		_ = json.Unmarshal(event.Data, &data)
		if data.Phase == "started" {
			sawStarted = true
		}
		if data.Phase == "failed" && data.Outcome == exploreOutcomeFailed {
			sawFailed = true
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
	if mainSawExploreSummary {
		t.Fatal("did not expect explore summary injection when explore pre-pass fails")
	}
	if !sawStarted || !sawFailed {
		t.Fatal("expected explore pre-pass start and failed status events")
	}
	if doneCount != 1 {
		t.Fatalf("expected done callback once, got %d", doneCount)
	}
}

func TestService_ExecuteStream_ExplorePrepassParallelLoad(t *testing.T) {
	t.Setenv(exploreEnabledEnvVar, "true")

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var exploreCalls int32
	var mainCalls int32
	exploreProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			atomic.AddInt32(&exploreCalls, 1)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Scope\n- vm 101\n\nFindings\n- ok"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	mainProvider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			atomic.AddInt32(&mainCalls, 1)
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

	const runs = 12
	var wg sync.WaitGroup
	errCh := make(chan error, runs)

	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("explore-load-%d", idx)
			doneCount := 0
			err := svc.ExecuteStream(context.Background(), ExecuteRequest{
				SessionID: sessionID,
				Prompt:    "check vm 101",
			}, func(event StreamEvent) {
				if event.Type == "done" {
					doneCount++
				}
			})
			if err != nil {
				errCh <- fmt.Errorf("run %d failed: %w", idx, err)
				return
			}
			if doneCount != 1 {
				errCh <- fmt.Errorf("run %d done callback count=%d", idx, doneCount)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	if int(exploreCalls) != runs {
		t.Fatalf("expected %d explore calls, got %d", runs, exploreCalls)
	}
	if int(mainCalls) != runs {
		t.Fatalf("expected %d main calls, got %d", runs, mainCalls)
	}
}
