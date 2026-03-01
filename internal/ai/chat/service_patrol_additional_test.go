package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type mockKnowledgeStore struct{}

func (m mockKnowledgeStore) SaveNote(resourceID, note, category string) error { return nil }
func (m mockKnowledgeStore) GetKnowledge(resourceID, category string) []tools.KnowledgeEntry {
	return nil
}

type mockStreamingProvider struct {
	chatStreamFunc func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
	lastRequest    providers.ChatRequest
	requests       []providers.ChatRequest
}

func (m *mockStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok"}, nil
}

func (m *mockStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	m.lastRequest = req
	m.requests = append(m.requests, req)
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, req, callback)
	}
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "hello"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}})
	return nil
}

func (m *mockStreamingProvider) SupportsThinking(model string) bool       { return false }
func (m *mockStreamingProvider) TestConnection(ctx context.Context) error { return nil }
func (m *mockStreamingProvider) Name() string                             { return "mock" }
func (m *mockStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func TestService_CreateProviderForModel(t *testing.T) {
	svc := &Service{}
	if _, err := svc.createProviderForModel("bad"); err == nil {
		t.Fatalf("expected error with nil config")
	}

	svc.cfg = &config.AIConfig{}
	// "bad" auto-detects as ollama, which should succeed (no API key needed)
	if _, err := svc.createProviderForModel("bad"); err != nil {
		t.Fatalf("expected ollama fallback for unrecognized model: %v", err)
	}

	// "unknown:model" has no known provider prefix, so ParseModelString falls through
	// to ollama (default for unrecognized models), which succeeds
	if _, err := svc.createProviderForModel("unknown:model"); err != nil {
		t.Fatalf("expected ollama fallback for unknown prefix: %v", err)
	}

	if _, err := svc.createProviderForModel("ollama:llama3"); err != nil {
		t.Fatalf("expected ollama provider to be created: %v", err)
	}

	// Slash-delimited OpenRouter models should auto-detect as openai
	svc.cfg.OpenAIAPIKey = "sk-test"
	if _, err := svc.createProviderForModel("google/gemini-2.5-flash"); err != nil {
		t.Fatalf("expected openai provider for slash-delimited model: %v", err)
	}

	// OpenRouter models with :free suffix should also route to openai
	if _, err := svc.createProviderForModel("google/gemini-2.0-flash:free"); err != nil {
		t.Fatalf("expected openai provider for slash-delimited model with suffix: %v", err)
	}

	svc.cfg = &config.AIConfig{
		OpenAIAPIKey:    "sk-test",
		AnthropicAPIKey: "ak-test",
		GeminiAPIKey:    "gk-test",
		DeepSeekAPIKey:  "dk-test",
		OllamaBaseURL:   "http://localhost:11434",
		PatrolModel:     "openai:gpt-4",
	}
	if _, err := svc.createProviderForModel("openai:gpt-4"); err != nil {
		t.Fatalf("expected openai provider: %v", err)
	}
	if _, err := svc.createProviderForModel("anthropic:claude-3"); err != nil {
		t.Fatalf("expected anthropic provider: %v", err)
	}
	if _, err := svc.createProviderForModel("gemini:gemini-1.5"); err != nil {
		t.Fatalf("expected gemini provider: %v", err)
	}
	if _, err := svc.createProviderForModel("deepseek:deepseek-chat"); err != nil {
		t.Fatalf("expected deepseek provider: %v", err)
	}
}

func TestService_ExecutePatrolStream_Success(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	service := &Service{
		started:  true,
		sessions: store,
		executor: executor,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	mockProvider := &mockStreamingProvider{
		chatStreamFunc: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "Patrol ok"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 3, OutputTokens: 2}})
			return nil
		},
	}
	var modelUsed string
	service.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		modelUsed = modelStr
		return mockProvider, nil
	}

	doneCount := 0
	resp, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{
		Prompt:    "check status",
		MaxTurns:  1,
		SessionID: "",
	}, func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Content == "" {
		t.Fatalf("expected response content")
	}
	if modelUsed != "mock:model" {
		t.Fatalf("expected provider factory to be called with patrol model")
	}
	if mockProvider.lastRequest.System == "" {
		t.Fatalf("expected system prompt to be set")
	}
	if doneCount != 1 {
		t.Fatalf("expected done event to be emitted")
	}

	msgs, err := store.GetMessages("patrol-main")
	if err != nil {
		t.Fatalf("failed to fetch messages: %v", err)
	}
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages saved, got %d", len(msgs))
	}
}

func TestService_ExecutePatrolStream_ResetsSessionHistoryEachRun(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	service := &Service{
		started:  true,
		sessions: store,
		executor: executor,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	mockProvider := &mockStreamingProvider{
		chatStreamFunc: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "ok"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1}})
			return nil
		},
	}
	service.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		return mockProvider, nil
	}

	for i := 0; i < 2; i++ {
		if _, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{
			Prompt:    "check status",
			MaxTurns:  1,
			SessionID: "patrol-main",
		}, func(StreamEvent) {}); err != nil {
			t.Fatalf("patrol run %d failed: %v", i+1, err)
		}
	}

	if len(mockProvider.requests) != 2 {
		t.Fatalf("expected 2 patrol requests, got %d", len(mockProvider.requests))
	}
	if len(mockProvider.requests[0].Messages) != 1 {
		t.Fatalf("expected first run to include only current prompt message, got %d", len(mockProvider.requests[0].Messages))
	}
	if len(mockProvider.requests[1].Messages) != 1 {
		t.Fatalf("expected second run to reset history and include only current prompt message, got %d", len(mockProvider.requests[1].Messages))
	}
}

func TestService_ExecutePatrolStream_TokenCapStopsGracefully(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	service := &Service{
		started:  true,
		sessions: store,
		executor: executor,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	mockProvider := &mockStreamingProvider{
		chatStreamFunc: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "initial analysis"}})
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{InputTokens: 2, OutputTokens: 1}})
			return nil
		},
	}
	service.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		return mockProvider, nil
	}

	resp, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{
		Prompt:         "check status",
		MaxTurns:       5,
		MaxTotalTokens: 1,
		SessionID:      "patrol-main",
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if resp.StopReason != "token_limit" {
		t.Fatalf("expected token_limit stop reason, got %q", resp.StopReason)
	}
}

func TestService_ExecutePatrolStream_Errors(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	service := &Service{
		started:  false,
		sessions: store,
		executor: executor,
	}

	if _, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{Prompt: "hi"}, func(StreamEvent) {}); err == nil {
		t.Fatalf("expected service not started error")
	}

	service.started = true
	service.cfg = &config.AIConfig{}
	if _, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{Prompt: "hi"}, func(StreamEvent) {}); err == nil {
		t.Fatalf("expected no patrol model configured error")
	}

	service.cfg.PatrolModel = "bad"
	if _, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{Prompt: "hi"}, func(StreamEvent) {}); err == nil {
		t.Fatalf("expected provider creation error")
	}
}

func TestService_ListAvailableToolsAndSetters(t *testing.T) {
	stateProvider := &mockStateProvider{state: models.StateSnapshot{}}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{StateProvider: stateProvider})

	service := &Service{executor: executor, stateProvider: stateProvider}
	service.SetGuestConfigProvider(nil)
	service.SetBudgetChecker(func() error { return nil })

	discoveryProvider := &mockDiscoveryProvider{}
	service.SetDiscoveryProvider(discoveryProvider)
	if service.contextPrefetcher == nil {
		t.Fatalf("expected context prefetcher to be set")
	}

	toolsList := service.ListAvailableTools(context.Background(), "check status")
	if len(toolsList) == 0 {
		t.Fatalf("expected available tools to be listed")
	}

	if service.GetExecutor() == nil {
		t.Fatalf("expected executor to be returned")
	}
}

func TestService_isAutonomousModeEnabled(t *testing.T) {
	service := &Service{cfg: &config.AIConfig{ControlLevel: config.ControlLevelAutonomous}}
	if !service.isAutonomousModeEnabled() {
		t.Fatalf("expected autonomous mode from config")
	}

	service = &Service{}
	if service.isAutonomousModeEnabled() {
		t.Fatalf("expected autonomous mode to be false")
	}
}

func TestService_ExecuteMCPTool(t *testing.T) {
	service := &Service{}
	if _, err := service.ExecuteMCPTool(context.Background(), "pulse_knowledge", nil); err == nil {
		t.Fatalf("expected error when executor missing")
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{KnowledgeStoreProvider: mockKnowledgeStore{}})
	service.executor = executor

	result, err := service.ExecuteMCPTool(context.Background(), "pulse_knowledge", map[string]interface{}{
		"action":      "recall",
		"resource_id": "vm1",
	})
	if err != nil {
		t.Fatalf("expected tool execution to succeed: %v", err)
	}
	if result == "" {
		t.Fatalf("expected result text")
	}

	if _, err := service.ExecuteMCPTool(context.Background(), "pulse_knowledge", map[string]interface{}{
		"action": "recall",
	}); err == nil {
		t.Fatalf("expected error for missing resource_id")
	}
}

func TestIsSpecialtyTool(t *testing.T) {
	if !isSpecialtyTool("pulse_storage") {
		t.Fatalf("expected pulse_storage to be specialty tool")
	}
	if isSpecialtyTool("pulse_query") {
		t.Fatalf("expected pulse_query to be non-specialty tool")
	}
}
