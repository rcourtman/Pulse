package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubServiceProvider struct {
	streamFn func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
}

func (s *stubServiceProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (s *stubServiceProvider) TestConnection(ctx context.Context) error { return nil }

func (s *stubServiceProvider) Name() string { return "stub" }

func (s *stubServiceProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (s *stubServiceProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	if s.streamFn != nil {
		return s.streamFn(ctx, req, callback)
	}
	callback(providers.StreamEvent{
		Type: "content",
		Data: providers.ContentEvent{Text: "hello"},
	})
	callback(providers.StreamEvent{
		Type: "done",
		Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
	})
	return nil
}

func (s *stubServiceProvider) SupportsThinking(model string) bool { return false }

func TestService_ExecuteStream_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: loop,
		provider:    provider, // Required: per-request loops need a provider to create new instances
		started:     true,
	}

	var doneCount int
	callback := func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
	}

	req := ExecuteRequest{
		SessionID: "sess-1",
		Prompt:    "hello",
	}
	if err := svc.ExecuteStream(context.Background(), req, callback); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if doneCount != 1 {
		t.Fatalf("expected done callback once, got %d", doneCount)
	}

	messages, err := store.GetMessages("sess-1")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
}

func TestService_ExecuteStream_HandoffContextIsModelOnly(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	var capturedMessages []providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages = append([]providers.Message(nil), req.Messages...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: loop,
		provider:    provider,
		started:     true,
	}

	req := ExecuteRequest{
		SessionID:      "sess-handoff",
		Prompt:         "What happened?",
		HandoffContext: "[Finding Context]\nID: finding-123\nConclusion: CPU saturated after backup.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-handoff")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected stored messages")
	}
	if stored[0].Content != "What happened?" {
		t.Fatalf("stored user message = %q, want clean prompt", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "[Finding Context]") {
		t.Fatalf("stored user message should not include handoff context: %q", stored[0].Content)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	if !strings.Contains(modelUserContent, "[Finding Context]") {
		t.Fatalf("model user content missing handoff context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "User message: What happened?") {
		t.Fatalf("model user content missing clean user message: %q", modelUserContent)
	}
}

func TestService_ExecuteStream_ReusesModelHandoffContextAcrossFollowUps(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	var capturedRequests [][]providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedRequests = append(capturedRequests, append([]providers.Message(nil), req.Messages...))
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: loop,
		provider:    provider,
		started:     true,
	}

	handoffContext := "[Finding Context]\nID: finding-123\nConclusion: CPU saturated after backup."
	firstReq := ExecuteRequest{
		SessionID:      "sess-handoff-followup",
		Prompt:         "What happened?",
		HandoffContext: handoffContext,
	}
	if err := svc.ExecuteStream(context.Background(), firstReq, func(StreamEvent) {}); err != nil {
		t.Fatalf("first ExecuteStream failed: %v", err)
	}

	secondReq := ExecuteRequest{
		SessionID: "sess-handoff-followup",
		Prompt:    "What should I do next?",
	}
	if err := svc.ExecuteStream(context.Background(), secondReq, func(StreamEvent) {}); err != nil {
		t.Fatalf("second ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-handoff-followup")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	var storedUserMessages []string
	for _, msg := range stored {
		if msg.Role == "user" {
			storedUserMessages = append(storedUserMessages, msg.Content)
		}
	}
	if len(storedUserMessages) != 2 {
		t.Fatalf("stored user messages = %d, want 2", len(storedUserMessages))
	}
	for _, content := range storedUserMessages {
		if strings.Contains(content, "[Finding Context]") {
			t.Fatalf("stored user message should not include handoff context: %q", content)
		}
	}
	if storedUserMessages[0] != "What happened?" || storedUserMessages[1] != "What should I do next?" {
		t.Fatalf("stored user messages = %#v, want clean prompts", storedUserMessages)
	}

	if len(capturedRequests) != 2 {
		t.Fatalf("provider request count = %d, want 2", len(capturedRequests))
	}
	firstModelUserContent := latestProviderUserContent(t, capturedRequests[0])
	if !strings.Contains(firstModelUserContent, "[Finding Context]") {
		t.Fatalf("first provider turn missing handoff context: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "User message: What happened?") {
		t.Fatalf("first provider turn missing clean user message: %q", firstModelUserContent)
	}
	secondModelUserContent := latestProviderUserContent(t, capturedRequests[1])
	if !strings.Contains(secondModelUserContent, "[Finding Context]") {
		t.Fatalf("follow-up provider turn missing stored handoff context: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "User message: What should I do next?") {
		t.Fatalf("follow-up provider turn missing clean user message: %q", secondModelUserContent)
	}
}

func TestService_ExecuteStream_HandoffResourceHydratesResolvedContext(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm:node1:101", VMID: 101, Name: "web-server", Node: "node1", Status: "running"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state)
	vmResources := registry.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one canonical VM resource, got %d", len(vmResources))
	}
	vmResource := vmResources[0]
	unifiedProvider := unifiedresources.NewUnifiedAIAdapter(registry)

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openai:test"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		unifiedResourceProvider: unifiedProvider,
		started:                 true,
	}

	req := ExecuteRequest{
		SessionID: "sess-handoff-resource",
		Prompt:    "What should I do next?",
		HandoffResources: []HandoffResource{{
			ID:   vmResource.ID,
			Name: "web-server",
			Type: "vm",
			Node: "node1",
		}},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	resolved := store.GetResolvedContext("sess-handoff-resource")
	info, found := resolved.GetResolvedResourceByAlias("web-server")
	if !found {
		t.Fatalf("expected handoff resource to be registered by alias")
	}
	if !resolved.WasRecentlyAccessed(info.GetResourceID(), time.Minute) {
		t.Fatalf("expected handoff resource to be marked as explicitly accessed")
	}
	if _, err := resolved.ValidateResourceForAction(info.GetResourceID(), "restart"); err != nil {
		t.Fatalf("expected handoff VM to allow governed restart action: %v", err)
	}
}

func latestProviderUserContent(t *testing.T, messages []providers.Message) string {
	t.Helper()

	for idx := len(messages) - 1; idx >= 0; idx-- {
		if messages[idx].Role == "user" {
			return messages[idx].Content
		}
	}
	t.Fatal("expected provider request to include a user message")
	return ""
}

func TestService_ExecuteStream_PrefetchMentionsAndOverrideModel(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm:node1:101", VMID: 101, Name: "vm1", Node: "node1", Status: "running"},
		},
	}
	readState := unifiedresources.NewRegistry(nil)
	readState.IngestSnapshot(state)
	vmResources := readState.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one canonical VM resource, got %d", len(vmResources))
	}
	vmResourceID := vmResources[0].ID

	var capturedModel string
	svc := &Service{
		cfg:               &config.AIConfig{ChatModel: "openai:primary"},
		sessions:          store,
		executor:          executor,
		agenticLoop:       loop,
		contextPrefetcher: NewContextPrefetcher(readState, nil),
		provider:          provider,
		started:           true,
		providerFactory: func(modelStr string) (providers.StreamingProvider, error) {
			capturedModel = modelStr
			return provider, nil
		},
	}

	autonomous := true
	req := ExecuteRequest{
		SessionID:      "sess-2",
		Prompt:         "check @vm1",
		Model:          "openai:override",
		Mentions:       []StructuredMention{{ID: "vm:node1:101", Name: "vm1", Type: "vm", Node: "node1"}},
		MaxTurns:       1,
		AutonomousMode: &autonomous,
	}

	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if capturedModel != "openai:override" {
		t.Fatalf("expected override model to be used, got %q", capturedModel)
	}

	resolved := store.GetResolvedContext("sess-2")
	if !resolved.WasRecentlyAccessed(vmResourceID, time.Minute) {
		t.Fatal("expected explicit access to be recorded for structured mention")
	}
}

func TestService_ExecuteStream_PrefetchMentionsMarksVMwareUnifiedResourceAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	now := time.Now().UTC()
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestRecords(unifiedresources.SourceVMware, []unifiedresources.IngestRecord{{
		SourceID: "vc-1:vm:vm-201",
		Resource: unifiedresources.Resource{
			ID:         "vmware-vm-1",
			Type:       unifiedresources.ResourceTypeVM,
			Name:       "app-01",
			Status:     unifiedresources.StatusOnline,
			LastSeen:   now,
			UpdatedAt:  now,
			ParentName: "esxi-01.lab.local",
			VMware: &unifiedresources.VMwareData{
				ConnectionID:    "vc-1",
				ConnectionName:  "Lab VC",
				ManagedObjectID: "vm-201",
				EntityType:      "vm",
				RuntimeHostName: "esxi-01.lab.local",
			},
		},
		Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"app-01"}},
	}})
	vmResources := rr.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one VMware VM resource, got %d", len(vmResources))
	}
	vmResourceID := vmResources[0].ID

	svc := &Service{
		cfg:               &config.AIConfig{ChatModel: "openai:primary"},
		sessions:          store,
		executor:          executor,
		agenticLoop:       loop,
		contextPrefetcher: NewContextPrefetcher(rr, nil),
		provider:          provider,
		started:           true,
		providerFactory: func(modelStr string) (providers.StreamingProvider, error) {
			return provider, nil
		},
	}

	autonomous := true
	req := ExecuteRequest{
		SessionID:      "sess-vmware-mentions",
		Prompt:         "check @app-01",
		Mentions:       []StructuredMention{{ID: vmResourceID, Name: "app-01", Type: "vm"}},
		MaxTurns:       1,
		AutonomousMode: &autonomous,
	}

	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	resolved := store.GetResolvedContext("sess-vmware-mentions")
	if !resolved.WasRecentlyAccessed(vmResourceID, time.Minute) {
		t.Fatal("expected explicit access to be recorded for VMware structured mention")
	}
}

func TestService_ExecuteStream_AgenticPulseStorageSnapshotsToleratesMalformedRecoveryMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
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
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
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
							ID:   "call-snapshots",
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
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "call-snapshots" {
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
					Data: providers.ContentEvent{Text: "Recovered snapshot inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						InputTokens:  10,
						OutputTokens: 12,
					},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: NewAgenticLoop(provider, executor, "system"),
		provider:    provider,
		started:     true,
	}

	req := ExecuteRequest{
		SessionID: "sess-storage-snapshots",
		Prompt:    "List recovery snapshots for guest 100 on pve1.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	messages, err := store.GetMessages(req.SessionID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "\"snapshot_name\":\"before-upgrade\"") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected stored tool result with canonical snapshot fallback, got %#v", messages)
	}
}

func TestService_ExecuteStream_AgenticPulseStorageBackupTasksToleratesMalformedRecoveryMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
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
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
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
							ID:   "call-backup-tasks",
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
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "call-backup-tasks" {
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
					Data: providers.ContentEvent{Text: "Recovered backup task inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						InputTokens:  10,
						OutputTokens: 12,
					},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: NewAgenticLoop(provider, executor, "system"),
		provider:    provider,
		started:     true,
	}

	req := ExecuteRequest{
		SessionID: "sess-storage-backup-tasks",
		Prompt:    "List recovery backup tasks for guest 101 on pve1.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	messages, err := store.GetMessages(req.SessionID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "\"status\":\"OK\"") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected stored tool result with canonical backup-task fallback, got %#v", messages)
	}
}
