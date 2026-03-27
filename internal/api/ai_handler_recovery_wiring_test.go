package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
)

type capturingAIService struct {
	running bool
}

func (s *capturingAIService) Start(ctx context.Context) error { s.running = true; return nil }
func (s *capturingAIService) Stop(ctx context.Context) error  { s.running = false; return nil }
func (s *capturingAIService) Restart(ctx context.Context, newCfg *config.AIConfig) error {
	s.running = true
	return nil
}
func (s *capturingAIService) IsRunning() bool { return s.running }
func (s *capturingAIService) Execute(ctx context.Context, req chat.ExecuteRequest) (map[string]interface{}, error) {
	return nil, nil
}
func (s *capturingAIService) ExecuteStream(ctx context.Context, req chat.ExecuteRequest, callback chat.StreamCallback) error {
	return nil
}
func (s *capturingAIService) ListSessions(ctx context.Context) ([]chat.Session, error) {
	return nil, nil
}
func (s *capturingAIService) CreateSession(ctx context.Context) (*chat.Session, error) {
	return &chat.Session{ID: "test"}, nil
}
func (s *capturingAIService) DeleteSession(ctx context.Context, sessionID string) error { return nil }
func (s *capturingAIService) GetMessages(ctx context.Context, sessionID string) ([]chat.Message, error) {
	return nil, nil
}
func (s *capturingAIService) AbortSession(ctx context.Context, sessionID string) error { return nil }
func (s *capturingAIService) SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *capturingAIService) GetSessionDiff(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *capturingAIService) ForkSession(ctx context.Context, sessionID string) (*chat.Session, error) {
	return &chat.Session{ID: "fork"}, nil
}
func (s *capturingAIService) RevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *capturingAIService) UnrevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *capturingAIService) AnswerQuestion(ctx context.Context, questionID string, answers []chat.QuestionAnswer) error {
	return nil
}
func (s *capturingAIService) SetAlertProvider(provider chat.MCPAlertProvider)                     {}
func (s *capturingAIService) SetFindingsProvider(provider chat.MCPFindingsProvider)               {}
func (s *capturingAIService) SetBaselineProvider(provider chat.MCPBaselineProvider)               {}
func (s *capturingAIService) SetPatternProvider(provider chat.MCPPatternProvider)                 {}
func (s *capturingAIService) SetMetricsHistory(provider chat.MCPMetricsHistoryProvider)           {}
func (s *capturingAIService) SetAgentProfileManager(manager chat.AgentProfileManager)             {}
func (s *capturingAIService) SetGuestConfigProvider(provider chat.MCPGuestConfigProvider)         {}
func (s *capturingAIService) SetBackupProvider(provider chat.MCPBackupProvider)                   {}
func (s *capturingAIService) SetDiskHealthProvider(provider chat.MCPDiskHealthProvider)           {}
func (s *capturingAIService) SetUpdatesProvider(provider chat.MCPUpdatesProvider)                 {}
func (s *capturingAIService) SetFindingsManager(manager chat.FindingsManager)                     {}
func (s *capturingAIService) SetMetadataUpdater(updater chat.MetadataUpdater)                     {}
func (s *capturingAIService) SetKnowledgeStoreProvider(provider chat.KnowledgeStoreProvider)      {}
func (s *capturingAIService) SetIncidentRecorderProvider(provider chat.IncidentRecorderProvider)  {}
func (s *capturingAIService) SetEventCorrelatorProvider(provider chat.EventCorrelatorProvider)    {}
func (s *capturingAIService) SetDiscoveryProvider(provider chat.MCPDiscoveryProvider)             {}
func (s *capturingAIService) SetUnifiedResourceProvider(provider chat.MCPUnifiedResourceProvider) {}
func (s *capturingAIService) UpdateControlSettings(cfg *config.AIConfig)                          {}
func (s *capturingAIService) GetBaseURL() string                                                  { return "" }

func TestAIHandlerStart_WiresRecoveryPointsProviderForDefaultChatService(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	var capturedCfg chat.Config
	newChatService = func(cfg chat.Config) AIService {
		capturedCfg = cfg
		return &capturingAIService{}
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
		ID:          "default-point-bad-json",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtr(time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtr(time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: "default",
			Name:      "data",
		},
		Details: map[string]any{"foo": "bar"},
	})
	corruptRecoveryRowJSON(t, dbPath, "default-point-bad-json", true, false, true)

	handler := NewAIHandler(mtp, nil, nil)
	handler.SetRecoveryManager(manager)

	if err := handler.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start(): %v", err)
	}

	if capturedCfg.OrgID != "default" {
		t.Fatalf("capturedCfg.OrgID = %q, want default", capturedCfg.OrgID)
	}
	if capturedCfg.RecoveryPointsProvider == nil {
		t.Fatal("expected recovery points provider to be wired into default chat service")
	}

	points, total, err := capturedCfg.RecoveryPointsProvider.ListPoints(context.Background(), recovery.ListPointsOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("RecoveryPointsProvider.ListPoints(default): %v", err)
	}
	if total != 1 || len(points) != 1 {
		t.Fatalf("expected one default recovery point, got total=%d len=%d", total, len(points))
	}
	if points[0].SubjectRef != nil || points[0].Details != nil {
		t.Fatalf("expected malformed metadata to degrade gracefully, got subjectRef=%#v details=%#v", points[0].SubjectRef, points[0].Details)
	}
	if points[0].Display == nil || points[0].Display.SubjectLabel != "default/data" {
		t.Fatalf("expected canonical display fallback, got %#v", points[0].Display)
	}
}

func TestAIHandlerGetService_WiresRecoveryPointsProviderForTenantChatService(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	capturedCfgs := make(map[string]chat.Config)
	newChatService = func(cfg chat.Config) AIService {
		capturedCfgs[cfg.OrgID] = cfg
		return &capturingAIService{}
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
		ID:          "tenant-point-bad-json",
		Provider:    recovery.ProviderTrueNAS,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtr(time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtr(time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type: "truenas-dataset",
			Name: "tank/apps",
			ID:   "tank/apps",
		},
	})
	corruptRecoveryRowJSON(t, dbPath, "tenant-point-bad-json", true, false, false)

	handler := NewAIHandler(mtp, nil, nil)
	handler.SetRecoveryManager(manager)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-a")
	svc := handler.GetService(ctx)
	if svc == nil {
		t.Fatal("expected tenant chat service to be created")
	}

	capturedCfg, ok := capturedCfgs["tenant-a"]
	if !ok {
		t.Fatal("expected tenant chat config to be captured")
	}
	if capturedCfg.RecoveryPointsProvider == nil {
		t.Fatal("expected recovery points provider to be wired into tenant chat service")
	}

	points, total, err := capturedCfg.RecoveryPointsProvider.ListPoints(context.Background(), recovery.ListPointsOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("RecoveryPointsProvider.ListPoints(tenant-a): %v", err)
	}
	if total != 1 || len(points) != 1 {
		t.Fatalf("expected one tenant recovery point, got total=%d len=%d", total, len(points))
	}
	if points[0].SubjectRef != nil {
		t.Fatalf("expected malformed subject ref to degrade gracefully, got %#v", points[0].SubjectRef)
	}
	if points[0].Display == nil || points[0].Display.SubjectLabel != "tank/apps" {
		t.Fatalf("expected canonical tenant display fallback, got %#v", points[0].Display)
	}
}

func seedRecoveryPointForAIHandlerWiringTest(t *testing.T, mtp *config.MultiTenantPersistence, manager *recoverymanager.Manager, orgID string, point recovery.RecoveryPoint) string {
	t.Helper()

	store, err := manager.StoreForOrg(orgID)
	if err != nil {
		t.Fatalf("StoreForOrg(%s): %v", orgID, err)
	}
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints(%s): %v", orgID, err)
	}

	persistence, err := mtp.GetPersistence(orgID)
	if err != nil {
		t.Fatalf("GetPersistence(%s): %v", orgID, err)
	}
	return filepath.Join(persistence.DataDir(), "recovery", "recovery.db")
}
