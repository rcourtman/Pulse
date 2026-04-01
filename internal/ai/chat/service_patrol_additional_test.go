package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

type mockKnowledgeStore struct{}

func (m mockKnowledgeStore) SaveNote(resourceID, note, category string) error { return nil }
func (m mockKnowledgeStore) GetKnowledge(resourceID, category string) []tools.KnowledgeEntry {
	return nil
}

type mockStreamingProvider struct {
	chatStreamFunc func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
	lastRequest    providers.ChatRequest
}

func (m *mockStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok"}, nil
}

func (m *mockStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	m.lastRequest = req
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
	if _, err := svc.createProviderForModel("bad"); err == nil {
		t.Fatalf("expected invalid model format error")
	}

	if _, err := svc.createProviderForModel("unknown:model"); err == nil {
		t.Fatalf("expected unsupported provider error")
	}

	if _, err := svc.createProviderForModel("ollama:llama3"); err != nil {
		t.Fatalf("expected ollama provider to be created: %v", err)
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

func TestService_CreateProviderForModel_OllamaUsesConfiguredBasicAuth(t *testing.T) {
	versionHits := 0
	tagsHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "unai" || password != "secret" {
			t.Fatalf("unexpected basic auth: ok=%v user=%q pass=%q", ok, username, password)
		}
		switch r.URL.Path {
		case "/api/version":
			versionHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			tagsHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3:latest"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := &Service{
		cfg: &config.AIConfig{
			OllamaBaseURL:  server.URL,
			OllamaUsername: "unai",
			OllamaPassword: "secret",
		},
	}

	provider, err := svc.createProviderForModel("ollama:llama3")
	if err != nil {
		t.Fatalf("expected ollama provider to be created: %v", err)
	}

	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("expected ollama provider test connection to succeed: %v", err)
	}
	if versionHits != 1 || tagsHits != 1 {
		t.Fatalf("expected version+tags check, got version=%d tags=%d", versionHits, tagsHits)
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

func TestService_ExecutePatrolStream_PulseStorageSnapshotsToleratesMalformedRecoveryMetadata(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	completedAt := time.Date(2026, 2, 24, 10, 30, 0, 0, time.UTC)
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		ReadState:     &fakeCanonicalReadState{},
		RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{{
			ID:       "pve-snapshot:snap-100-before-upgrade",
			Provider: recovery.ProviderProxmoxPVE,
			Kind:     recovery.KindSnapshot,
			Mode:     recovery.ModeLocal,
			Outcome:  recovery.OutcomeSuccess,
			SubjectRef: &recovery.ExternalRef{
				Type:      "vm",
				Name:      "100",
				ID:        "100",
				Namespace: "pve1",
				Class:     "node1",
			},
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:   "100",
				ItemType:       "vm",
				ClusterLabel:   "pve1",
				NodeHostLabel:  "node1",
				EntityIDLabel:  "100",
				DetailsSummary: "before-upgrade",
			},
			CompletedAt: &completedAt,
		}}},
	})

	turn := 0
	provider := &mockStreamingProvider{
		chatStreamFunc: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			turn++
			switch turn {
			case 1:
				if !hasProviderTool(req.Tools, "pulse_storage") {
					t.Fatalf("expected pulse_storage tool to be available, got %#v", req.Tools)
				}
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						StopReason: "tool_use",
						ToolCalls: []providers.ToolCall{{
							ID:   "patrol-call-snapshots",
							Name: "pulse_storage",
							Input: map[string]interface{}{
								"type":     "snapshots",
								"guest_id": "100",
								"instance": "pve1",
							},
						}},
					},
				})
				return nil
			case 2:
				var toolResult string
				for _, msg := range req.Messages {
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "patrol-call-snapshots" {
						toolResult = msg.ToolResult.Content
						break
					}
				}
				if toolResult == "" {
					t.Fatalf("expected pulse_storage tool result in second turn, got %#v", req.Messages)
				}
				if !strings.Contains(toolResult, "\"snapshot_name\":\"before-upgrade\"") {
					t.Fatalf("expected canonical snapshot name in tool result, got %s", toolResult)
				}
				if !strings.Contains(toolResult, "\"instance\":\"pve1\"") || !strings.Contains(toolResult, "\"node\":\"node1\"") {
					t.Fatalf("expected canonical placement labels in tool result, got %s", toolResult)
				}
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "Patrol recovered snapshot inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{InputTokens: 10, OutputTokens: 12},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	service := &Service{
		started:  true,
		sessions: store,
		executor: executor,
		cfg: &config.AIConfig{
			PatrolModel:          "mock:model",
			PatrolAnalyzeStorage: true,
		},
	}
	service.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		return provider, nil
	}

	resp, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{
		Prompt:    "Inspect recovery snapshots for guest 100 on pve1.",
		MaxTurns:  2,
		SessionID: "patrol-storage-snapshots",
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecutePatrolStream failed: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "snapshot inventory") {
		t.Fatalf("expected patrol response content, got %#v", resp)
	}

	messages, err := store.GetMessages("patrol-storage-snapshots")
	if err != nil {
		t.Fatalf("failed to fetch messages: %v", err)
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "\"snapshot_name\":\"before-upgrade\"") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected stored patrol tool result with canonical snapshot fallback, got %#v", messages)
	}
}

func TestService_ExecutePatrolStream_PulseStorageBackupTasksToleratesMalformedRecoveryMetadata(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	startedAt := time.Date(2026, 2, 24, 11, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 2, 24, 11, 15, 0, 0, time.UTC)
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		ReadState:     &fakeCanonicalReadState{},
		RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{{
			ID:       "pve-task:task-101-backup",
			Provider: recovery.ProviderProxmoxPVE,
			Kind:     recovery.KindBackup,
			Mode:     recovery.ModeLocal,
			Outcome:  recovery.OutcomeSuccess,
			SubjectRef: &recovery.ExternalRef{
				Type:      "vm",
				Name:      "101",
				ID:        "101",
				Namespace: "pve1",
				Class:     "node1",
			},
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:   "101",
				ItemType:       "vm",
				ClusterLabel:   "pve1",
				NodeHostLabel:  "node1",
				EntityIDLabel:  "101",
				DetailsSummary: "completed successfully",
			},
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
		}}},
	})

	turn := 0
	provider := &mockStreamingProvider{
		chatStreamFunc: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			turn++
			switch turn {
			case 1:
				if !hasProviderTool(req.Tools, "pulse_storage") {
					t.Fatalf("expected pulse_storage tool to be available, got %#v", req.Tools)
				}
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						StopReason: "tool_use",
						ToolCalls: []providers.ToolCall{{
							ID:   "patrol-call-backup-tasks",
							Name: "pulse_storage",
							Input: map[string]interface{}{
								"type":     "backup_tasks",
								"guest_id": "101",
								"instance": "pve1",
								"status":   "OK",
							},
						}},
					},
				})
				return nil
			case 2:
				var toolResult string
				for _, msg := range req.Messages {
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "patrol-call-backup-tasks" {
						toolResult = msg.ToolResult.Content
						break
					}
				}
				if toolResult == "" {
					t.Fatalf("expected pulse_storage tool result in second turn, got %#v", req.Messages)
				}
				if !strings.Contains(toolResult, "\"status\":\"OK\"") || !strings.Contains(toolResult, "\"type\":\"vm\"") {
					t.Fatalf("expected canonical backup task fields in tool result, got %s", toolResult)
				}
				if !strings.Contains(toolResult, "\"instance\":\"pve1\"") || !strings.Contains(toolResult, "\"node\":\"node1\"") {
					t.Fatalf("expected canonical placement labels in tool result, got %s", toolResult)
				}
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "Patrol recovered backup task inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{InputTokens: 10, OutputTokens: 12},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	service := &Service{
		started:  true,
		sessions: store,
		executor: executor,
		cfg: &config.AIConfig{
			PatrolModel:          "mock:model",
			PatrolAnalyzeStorage: true,
		},
	}
	service.providerFactory = func(modelStr string) (providers.StreamingProvider, error) {
		return provider, nil
	}

	resp, err := service.ExecutePatrolStream(context.Background(), PatrolRequest{
		Prompt:    "Inspect recovery backup tasks for guest 101 on pve1.",
		MaxTurns:  2,
		SessionID: "patrol-storage-backup-tasks",
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecutePatrolStream failed: %v", err)
	}
	if resp == nil || !strings.Contains(resp.Content, "backup task inventory") {
		t.Fatalf("expected patrol response content, got %#v", resp)
	}

	messages, err := store.GetMessages("patrol-storage-backup-tasks")
	if err != nil {
		t.Fatalf("failed to fetch messages: %v", err)
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "\"status\":\"OK\"") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected stored patrol tool result with canonical backup-task fallback, got %#v", messages)
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

	rs := newTestReadState(models.StateSnapshot{})
	service := &Service{executor: executor, stateProvider: stateProvider, readState: rs}
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
