package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
)

type recoveryHTTPStreamingProvider struct {
	streamFn func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
}

func (p *recoveryHTTPStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (p *recoveryHTTPStreamingProvider) TestConnection(ctx context.Context) error { return nil }

func (p *recoveryHTTPStreamingProvider) Name() string { return "stub" }

func (p *recoveryHTTPStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (p *recoveryHTTPStreamingProvider) SupportsThinking(model string) bool { return false }

func (p *recoveryHTTPStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	if p.streamFn != nil {
		return p.streamFn(ctx, req, callback)
	}
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "hello"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "end_turn"}})
	return nil
}

func hasProviderToolForAPI(toolsList []providers.Tool, name string) bool {
	for _, tool := range toolsList {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func TestHandleChat_DefaultRecoverySnapshotsTolerateMalformedMetadata(t *testing.T) {
	t.Setenv("PULSE_EXPLORE_ENABLED", "false")

	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	turn := 0
	provider := &recoveryHTTPStreamingProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			turn++
			switch turn {
			case 1:
				if !hasProviderToolForAPI(req.Tools, "pulse_storage") {
					t.Fatalf("expected pulse_storage tool to be available, got %#v", req.Tools)
				}
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						StopReason: "tool_use",
						ToolCalls: []providers.ToolCall{{
							ID:   "http-call-snapshots",
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
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "http-call-snapshots" {
						toolResult = msg.ToolResult.Content
						break
					}
				}
				if toolResult == "" {
					t.Fatalf("expected pulse_storage tool result in second turn, got %#v", req.Messages)
				}
				if !strings.Contains(toolResult, "\"snapshot_name\":\"before-upgrade\"") {
					t.Fatalf("expected canonical snapshot fallback in tool result, got %s", toolResult)
				}
				if !strings.Contains(toolResult, "\"instance\":\"pve1\"") || !strings.Contains(toolResult, "\"node\":\"node1\"") {
					t.Fatalf("expected canonical placement labels in tool result, got %s", toolResult)
				}
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "HTTP recovered snapshot inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{InputTokens: 8, OutputTokens: 11},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	newChatService = func(cfg chat.Config) AIService {
		service := chat.NewService(cfg)
		setUnexportedField(t, service, "providerFactory", func(string) (providers.StreamingProvider, error) {
			return provider, nil
		})
		return service
	}

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	manager := recoverymanager.New(mtp)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "stub:model"
	aiCfg.ChatModel = "stub:model"
	aiCfg.PatrolModel = "stub:model"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig(default): %v", err)
	}

	dbPath := seedRecoveryPointForAIHandlerWiringTest(t, mtp, manager, "default", recovery.RecoveryPoint{
		ID:          "pve-snapshot:before-upgrade",
		Provider:    recovery.ProviderProxmoxPVE,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeLocal,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtr(time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)),
		CompletedAt: timePtr(time.Date(2026, 2, 24, 10, 30, 0, 0, time.UTC)),
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
		Details: map[string]any{"summary": "before-upgrade"},
	})
	corruptRecoveryRowJSON(t, dbPath, "pve-snapshot:before-upgrade", true, false, true)

	handler := NewAIHandler(mtp, nil, nil)
	handler.SetRecoveryManager(manager)
	if err := handler.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start(): %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{"prompt":"List recovery snapshots for guest 100 on pve1.","session_id":"http-default-snapshots"}`))
	rec := httptest.NewRecorder()
	handler.HandleChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleChat() status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if strings.Contains(body, `"type":"error"`) {
		t.Fatalf("expected no error SSE event, got %s", body)
	}
	if !strings.Contains(body, "HTTP recovered snapshot inventory") {
		t.Fatalf("expected recovery content event, got %s", body)
	}
}

func TestHandleChat_TenantRecoveryBackupTasksTolerateMalformedMetadata(t *testing.T) {
	t.Setenv("PULSE_EXPLORE_ENABLED", "false")

	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	turn := 0
	provider := &recoveryHTTPStreamingProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			turn++
			switch turn {
			case 1:
				if !hasProviderToolForAPI(req.Tools, "pulse_storage") {
					t.Fatalf("expected pulse_storage tool to be available, got %#v", req.Tools)
				}
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						StopReason: "tool_use",
						ToolCalls: []providers.ToolCall{{
							ID:   "http-call-backup-tasks",
							Name: "pulse_storage",
							Input: map[string]interface{}{
								"type":     "backup_tasks",
								"guest_id": "101",
								"instance": "tenant-pve",
								"status":   "OK",
							},
						}},
					},
				})
				return nil
			case 2:
				var toolResult string
				for _, msg := range req.Messages {
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "http-call-backup-tasks" {
						toolResult = msg.ToolResult.Content
						break
					}
				}
				if toolResult == "" {
					t.Fatalf("expected pulse_storage tool result in second turn, got %#v", req.Messages)
				}
				if !strings.Contains(toolResult, "\"status\":\"OK\"") || !strings.Contains(toolResult, "\"type\":\"vm\"") {
					t.Fatalf("expected canonical backup-task fallback in tool result, got %s", toolResult)
				}
				if !strings.Contains(toolResult, "\"instance\":\"tenant-pve\"") || !strings.Contains(toolResult, "\"node\":\"node9\"") {
					t.Fatalf("expected canonical placement labels in tool result, got %s", toolResult)
				}
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "HTTP recovered backup task inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{InputTokens: 8, OutputTokens: 11},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	newChatService = func(cfg chat.Config) AIService {
		service := chat.NewService(cfg)
		setUnexportedField(t, service, "providerFactory", func(string) (providers.StreamingProvider, error) {
			return provider, nil
		})
		return service
	}

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	manager := recoverymanager.New(mtp)
	persistence, err := mtp.GetPersistence("tenant-a")
	if err != nil {
		t.Fatalf("GetPersistence(tenant-a): %v", err)
	}

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "stub:model"
	aiCfg.ChatModel = "stub:model"
	aiCfg.PatrolModel = "stub:model"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig(tenant-a): %v", err)
	}

	dbPath := seedRecoveryPointForAIHandlerWiringTest(t, mtp, manager, "tenant-a", recovery.RecoveryPoint{
		ID:          "pve-task:task-101-backup",
		Provider:    recovery.ProviderProxmoxPVE,
		Kind:        recovery.KindBackup,
		Mode:        recovery.ModeLocal,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtr(time.Date(2026, 2, 24, 11, 0, 0, 0, time.UTC)),
		CompletedAt: timePtr(time.Date(2026, 2, 24, 11, 15, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type:      "vm",
			Name:      "101",
			ID:        "101",
			Namespace: "tenant-pve",
			Class:     "node9",
		},
		Display: &recovery.RecoveryPointDisplay{
			SubjectLabel:  "101",
			ItemType:      "vm",
			ClusterLabel:  "tenant-pve",
			NodeHostLabel: "node9",
			EntityIDLabel: "101",
		},
	})
	corruptRecoveryRowJSON(t, dbPath, "pve-task:task-101-backup", true, false, false)

	handler := NewAIHandler(mtp, nil, nil)
	handler.SetRecoveryManager(manager)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", strings.NewReader(`{"prompt":"List recovery backup tasks for guest 101 on tenant-pve.","session_id":"http-tenant-backup-tasks"}`))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	rec := httptest.NewRecorder()
	handler.HandleChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HandleChat() status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if strings.Contains(body, `"type":"error"`) {
		t.Fatalf("expected no error SSE event, got %s", body)
	}
	if !strings.Contains(body, "HTTP recovered backup task inventory") {
		t.Fatalf("expected recovery content event, got %s", body)
	}
}
