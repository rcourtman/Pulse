package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAIService struct {
	mock.Mock
}

func (m *MockAIService) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAIService) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAIService) Restart(ctx context.Context, newCfg *config.AIConfig) error {
	args := m.Called(ctx, newCfg)
	return args.Error(0)
}

func (m *MockAIService) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockAIService) Execute(ctx context.Context, req chat.ExecuteRequest) (map[string]interface{}, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockAIService) ExecuteStream(ctx context.Context, req chat.ExecuteRequest, callback chat.StreamCallback) error {
	args := m.Called(ctx, req, callback)
	return args.Error(0)
}

func (m *MockAIService) ListSessions(ctx context.Context) ([]chat.Session, error) {
	args := m.Called(ctx)
	return args.Get(0).([]chat.Session), args.Error(1)
}

func (m *MockAIService) CreateSession(ctx context.Context) (*chat.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*chat.Session), args.Error(1)
}

func (m *MockAIService) DeleteSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAIService) GetMessages(ctx context.Context, sessionID string) ([]chat.Message, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).([]chat.Message), args.Error(1)
}

func (m *MockAIService) GetModelHandoffFindingID(ctx context.Context, sessionID string) (string, error) {
	for _, call := range m.ExpectedCalls {
		if call.Method == "GetModelHandoffFindingID" {
			args := m.Called(ctx, sessionID)
			return args.String(0), args.Error(1)
		}
	}
	return "", nil
}

func (m *MockAIService) ClearModelHandoffContext(ctx context.Context, sessionID string) error {
	for _, call := range m.ExpectedCalls {
		if call.Method == "ClearModelHandoffContext" {
			args := m.Called(ctx, sessionID)
			return args.Error(0)
		}
	}
	return nil
}

func (m *MockAIService) AbortSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAIService) SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockAIService) GetSessionDiff(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockAIService) ForkSession(ctx context.Context, sessionID string) (*chat.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*chat.Session), args.Error(1)
}

func (m *MockAIService) RevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockAIService) UnrevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockAIService) AnswerQuestion(ctx context.Context, questionID string, answers []chat.QuestionAnswer) error {
	args := m.Called(ctx, questionID, answers)
	return args.Error(0)
}

func (m *MockAIService) SetAlertProvider(provider chat.MCPAlertProvider)       { m.Called(provider) }
func (m *MockAIService) SetFindingsProvider(provider chat.MCPFindingsProvider) { m.Called(provider) }
func (m *MockAIService) SetBaselineProvider(provider chat.MCPBaselineProvider) { m.Called(provider) }
func (m *MockAIService) SetPatternProvider(provider chat.MCPPatternProvider)   { m.Called(provider) }
func (m *MockAIService) SetMetricsHistory(provider chat.MCPMetricsHistoryProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetBackupProvider(provider chat.MCPBackupProvider) { m.Called(provider) }
func (m *MockAIService) SetGuestConfigProvider(provider chat.MCPGuestConfigProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetAppContainerConfigProvider(provider chat.MCPAppContainerConfigProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetDiskHealthProvider(provider chat.MCPDiskHealthProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetUpdatesProvider(provider chat.MCPUpdatesProvider) { m.Called(provider) }
func (m *MockAIService) SetAgentProfileManager(manager chat.AgentProfileManager) {
	m.Called(manager)
}
func (m *MockAIService) SetFindingsManager(manager chat.FindingsManager) { m.Called(manager) }
func (m *MockAIService) SetMetadataUpdater(updater chat.MetadataUpdater) { m.Called(updater) }
func (m *MockAIService) SetKnowledgeStoreProvider(provider chat.KnowledgeStoreProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetIncidentRecorderProvider(provider chat.IncidentRecorderProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetEventCorrelatorProvider(provider chat.EventCorrelatorProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetDiscoveryProvider(provider chat.MCPDiscoveryProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetUnifiedResourceProvider(provider chat.MCPUnifiedResourceProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetAppContainerActionProvider(provider chat.MCPAppContainerActionProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetAppContainerReadProvider(provider chat.MCPAppContainerReadProvider) {
	m.Called(provider)
}

func (m *MockAIService) UpdateControlSettings(cfg *config.AIConfig) { m.Called(cfg) }
func (m *MockAIService) GetBaseURL() string {
	args := m.Called()
	return args.String(0)
}

type MockAIPersistence struct {
	mock.Mock
	dataDir string
}

func (m *MockAIPersistence) LoadAIConfig() (*config.AIConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*config.AIConfig), args.Error(1)
}

func (m *MockAIPersistence) DataDir() string {
	return m.dataDir
}

func newTestAIHandler(cfg *config.Config, persistence AIPersistence, _ *agentexec.Server) *AIHandler {
	handler := NewAIHandler(nil, nil, nil)
	handler.defaultConfig = cfg
	handler.defaultPersistence = persistence
	return handler
}

func TestStart(t *testing.T) {
	// Mock newChatService
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	mockSvc := new(MockAIService)
	newChatService = func(cfg chat.Config) AIService {
		return mockSvc
	}

	mockPersist := new(MockAIPersistence)
	h := newTestAIHandler(&config.Config{}, mockPersist, nil)

	// AI disabled in config
	mockPersist.On("LoadAIConfig").Return(&config.AIConfig{Enabled: false}, nil).Once()
	err := h.Start(context.Background(), nil)
	assert.NoError(t, err)
	assert.Nil(t, h.defaultService)

	// AI enabled
	aiCfg := &config.AIConfig{Enabled: true, Model: "test"}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil).Once()
	mockSvc.On("Start", mock.Anything).Return(nil).Once()

	err = h.Start(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, mockSvc, h.defaultService)
}

func TestStop(t *testing.T) {
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, nil, nil)
	h.defaultService = mockSvc

	mockSvc.On("Stop", mock.Anything).Return(nil)
	err := h.Stop(context.Background())
	assert.NoError(t, err)

	// Nil service
	h.defaultService = nil
	err = h.Stop(context.Background())
	assert.NoError(t, err)
}

func TestStart_Error(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	mockSvc := new(MockAIService)
	newChatService = func(cfg chat.Config) AIService {
		return mockSvc
	}

	mockPersist := new(MockAIPersistence)
	h := newTestAIHandler(&config.Config{}, mockPersist, nil)

	aiCfg := &config.AIConfig{Enabled: true, Model: "test"}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil)
	mockSvc.On("Start", mock.Anything).Return(assert.AnError)

	err := h.Start(context.Background(), nil)
	assert.Error(t, err)
}

func TestRestart(t *testing.T) {
	mockPersist := new(MockAIPersistence)
	mockPersist.dataDir = t.TempDir()
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, mockPersist, nil)
	h.defaultService = mockSvc
	prevStore := approval.GetStore()
	t.Cleanup(func() {
		h.clearApprovalStore()
		approval.SetStore(prevStore)
	})

	aiCfg := &config.AIConfig{Enabled: true}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil)
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("Restart", mock.Anything, aiCfg).Return(nil)
	err := h.Restart(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, approval.GetStore())
}

func TestRestart_DisabledClearsApprovalStore(t *testing.T) {
	mockPersist := new(MockAIPersistence)
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, mockPersist, nil)
	h.defaultService = mockSvc

	prevStore := approval.GetStore()
	dataDir := t.TempDir()
	seedStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:        dataDir,
		DefaultTimeout: time.Minute,
		MaxApprovals:   10,
	})
	if err != nil {
		t.Fatalf("create seed approval store: %v", err)
	}
	approval.SetStore(seedStore)
	h.approvalStore = seedStore
	h.approvalStoreDir = dataDir
	t.Cleanup(func() {
		h.clearApprovalStore()
		approval.SetStore(prevStore)
	})

	mockPersist.On("LoadAIConfig").Return(&config.AIConfig{Enabled: false}, nil)
	mockSvc.On("Restart", mock.Anything, mock.Anything).Return(nil)

	err = h.Restart(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, approval.GetStore())
}

func TestGetService(t *testing.T) {
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, nil, nil)
	h.defaultService = mockSvc
	assert.Equal(t, mockSvc, h.GetService(context.Background()))
}

func TestGetAIConfig(t *testing.T) {
	mockPersist := new(MockAIPersistence)
	h := newTestAIHandler(nil, mockPersist, nil)

	aiCfg := &config.AIConfig{Model: "test"}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil)

	result := h.GetAIConfig(context.Background())
	assert.Equal(t, aiCfg, result)
}

func TestLoadAIConfig_Error(t *testing.T) {
	mockPersist := new(MockAIPersistence)
	h := newTestAIHandler(nil, mockPersist, nil)

	mockPersist.On("LoadAIConfig").Return((*config.AIConfig)(nil), assert.AnError)

	result := h.loadAIConfig(context.Background())
	assert.Nil(t, result)
}

func TestHandleStatus(t *testing.T) {
	cfg := &config.Config{
		APIToken: "test-token",
	}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)

	req := httptest.NewRequest("GET", "/api/ai/status", nil)

	w := httptest.NewRecorder()

	h.HandleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.True(t, resp["running"].(bool))
	assert.Equal(t, "direct", resp["engine"])
}

func TestHandleSessions(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	sessions := []chat.Session{{ID: "s1"}, {ID: "s2"}}
	mockSvc.On("ListSessions", mock.Anything).Return(sessions, nil)

	req := httptest.NewRequest("GET", "/api/ai/sessions", nil)
	w := httptest.NewRecorder()

	h.HandleSessions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleCreateSession(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	session := &chat.Session{ID: "new-session"}
	mockSvc.On("CreateSession", mock.Anything).Return(session, nil)

	req := httptest.NewRequest("POST", "/api/ai/sessions", nil)
	w := httptest.NewRecorder()

	h.HandleCreateSession(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleDeleteSession(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("DeleteSession", mock.Anything, "s1").Return(nil)

	req := httptest.NewRequest("DELETE", "/api/ai/sessions/s1", nil)
	w := httptest.NewRecorder()

	h.HandleDeleteSession(w, req, "s1")

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandleMessages(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	messages := []chat.Message{{Role: "user", Content: "hello"}}
	mockSvc.On("GetMessages", mock.Anything, "s1").Return(messages, nil)

	req := httptest.NewRequest("GET", "/api/ai/sessions/s1/messages", nil)
	w := httptest.NewRecorder()

	h.HandleMessages(w, req, "s1")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"tool_calls":[]`)
}

func TestHandleChat_NotRunning(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(false)

	req := httptest.NewRequest("POST", "/api/ai/chat", nil)
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleChat_InvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)

	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader("invalid"))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleChat_Success(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)

	// Mock ExecuteStream to just return nil
	mockSvc.On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		callback := args.Get(2).(chat.StreamCallback)
		data, _ := json.Marshal("hello")
		callback(chat.StreamEvent{Type: "content", Data: data})
	})

	body := `{"prompt": "hi"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
}

func TestHandleChat_PreservesCanonicalMentionTypes(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			if len(reqArg.Mentions) != 3 {
				t.Fatalf("mentions len = %d, want 3 (%+v)", len(reqArg.Mentions), reqArg.Mentions)
			}
			assert.Equal(t, "system-container", reqArg.Mentions[0].Type)
			assert.Equal(t, "app-container", reqArg.Mentions[1].Type)
			assert.Equal(t, "agent", reqArg.Mentions[2].Type)
		})

	body := `{"prompt":"hi","mentions":[{"id":"system-container:pve1:200","name":"ct200","type":"system-container","node":"pve1"},{"id":"docker:agent-1:nginx","name":"nginx","type":"app-container"},{"id":"agent:node-1","name":"node-1","type":"agent"}]}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleChat_PassesAutonomousModeOverride(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			if assert.NotNil(t, reqArg.AutonomousMode) {
				assert.False(t, *reqArg.AutonomousMode)
			}
		})

	body := `{"prompt":"summarize dashboard","autonomous_mode":false}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleChat_IncludesInvestigationRecordContext(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	detectedAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	lastSeenAt := detectedAt.Add(2 * time.Minute)
	completedAt := detectedAt.Add(3 * time.Minute)
	lastInvestigatedAt := detectedAt.Add(4 * time.Minute)
	aiEnhancedAt := detectedAt.Add(5 * time.Minute)
	lastRegressionAt := detectedAt.Add(6 * time.Minute)
	store := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	store.AddFromAI(&unified.UnifiedFinding{
		ID:                    "finding-123",
		Source:                unified.SourceAIPatrol,
		Severity:              unified.SeverityCritical,
		Category:              unified.CategoryPerformance,
		ResourceID:            "vm-100",
		ResourceName:          "web-server",
		ResourceType:          "vm",
		Node:                  "pve-1",
		Title:                 "High CPU usage",
		Description:           "CPU stayed above 95%.",
		Recommendation:        "Review the backup job.",
		Evidence:              "cpu=96%",
		DetectedAt:            detectedAt,
		LastSeenAt:            lastSeenAt,
		AIContext:             "The spike overlaps the nightly backup job.",
		RootCauseID:           "finding-root",
		CorrelatedIDs:         []string{"finding-storage"},
		RemediationID:         "remediation-123",
		AIConfidence:          0.87,
		AIEnhancedAt:          &aiEnhancedAt,
		InvestigationStatus:   "completed",
		InvestigationOutcome:  "fix_queued",
		LastInvestigatedAt:    &lastInvestigatedAt,
		InvestigationAttempts: 1,
		LoopState:             "awaiting_approval",
		Lifecycle: []unified.UnifiedFindingLifecycleEvent{{
			At:      detectedAt,
			Type:    "created",
			Message: "Patrol opened the finding",
		}, {
			At:      completedAt,
			Type:    "investigation_completed",
			Message: "Fix queued for approval",
			From:    "investigating",
			To:      "fix_queued",
		}},
		RegressionCount:  2,
		LastRegressionAt: &lastRegressionAt,
		TimesRaised:      3,
		InvestigationRecord: &aicontracts.InvestigationRecord{
			ID:        "investigation-123",
			FindingID: "finding-123",
			SessionID: "session-123",
			Subject: aicontracts.InvestigationRecordSubject{
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Node:         "pve-1",
			},
			Trigger: aicontracts.InvestigationRecordTrigger{
				FindingKey:  "cpu-high",
				Source:      "ai-patrol",
				Severity:    "critical",
				Category:    "performance",
				Title:       "High CPU usage",
				DetectedAt:  detectedAt,
				Description: "CPU stayed above 95%.",
			},
			Status:            aicontracts.InvestigationStatusCompleted,
			Outcome:           aicontracts.OutcomeFixQueued,
			Confidence:        aicontracts.InvestigationRecordConfidenceHigh,
			Evidence:          []aicontracts.InvestigationRecordEvidence{{Kind: "metrics", Summary: "CPU stayed above 95% for 10 minutes"}},
			Conclusion:        "Backup job saturated CPU.",
			RecommendedAction: "Approve a controlled service restart after backup completion.",
			ProposedFix: &aicontracts.InvestigationRecordFix{
				ID:          "fix-123",
				Description: "Restart the workload service",
				Commands:    []string{"systemctl restart workload.service"},
				RiskLevel:   "medium",
				TargetHost:  "pve-1",
				Rationale:   "The process is wedged after backup IO pressure.",
				Destructive: true,
			},
			Verification: []string{"CPU returned below 50%"},
			ToolsUsed:    []string{"metrics.history", "ssh.exec"},
			StartedAt:    detectedAt,
			CompletedAt:  &completedAt,
			ApprovalID:   "approval-123",
		},
	})
	store.AddFromAI(&unified.UnifiedFinding{
		ID:                   "finding-root",
		Source:               unified.SourceAIPatrol,
		Severity:             unified.SeverityWarning,
		Category:             unified.CategoryCapacity,
		ResourceID:           "storage-100",
		ResourceName:         "backup-store",
		ResourceType:         "storage",
		Node:                 "pve-1",
		Title:                "Backup storage pressure",
		Description:          "Backup storage latency increased before the CPU spike.",
		DetectedAt:           detectedAt.Add(-5 * time.Minute),
		LastSeenAt:           lastSeenAt,
		AIContext:            "Backup storage pressure preceded the workload CPU saturation.",
		AIConfidence:         0.74,
		InvestigationStatus:  "completed",
		InvestigationOutcome: "root_cause",
		Lifecycle: []unified.UnifiedFindingLifecycleEvent{{
			At:      detectedAt.Add(-2 * time.Minute),
			Type:    "correlated",
			Message: "Backup latency identified",
			From:    "detected",
			To:      "root_cause",
		}},
	})
	store.AddFromAI(&unified.UnifiedFinding{
		ID:                   "finding-storage",
		Source:               unified.SourceAIPatrol,
		Severity:             unified.SeverityWarning,
		Category:             unified.CategoryCapacity,
		ResourceID:           "storage-200",
		ResourceName:         "vm-datastore",
		ResourceType:         "storage",
		Node:                 "pve-2",
		Title:                "Datastore latency",
		Description:          "Datastore writes were elevated during the same window.",
		DetectedAt:           detectedAt.Add(-4 * time.Minute),
		LastSeenAt:           lastSeenAt,
		InvestigationStatus:  "completed",
		InvestigationOutcome: "correlated",
		InvestigationRecord: &aicontracts.InvestigationRecord{
			ID: "investigation-storage",
			Subject: aicontracts.InvestigationRecordSubject{
				ResourceID:   "storage-200",
				ResourceName: "vm-datastore",
				ResourceType: "storage",
				Node:         "pve-2",
			},
			Status:     aicontracts.InvestigationStatusCompleted,
			Outcome:    aicontracts.OutcomeResolved,
			Confidence: aicontracts.InvestigationRecordConfidenceMedium,
			Conclusion: "Datastore latency recovered after backup completion.",
		},
	})
	h.SetUnifiedStore(store)

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			assert.Equal(t, "finding-123", reqArg.FindingID)
			assert.Equal(t, "What happened?", reqArg.Prompt)
			assert.Contains(t, reqArg.HandoffContext, "[Operator Briefing]")
			assert.Contains(t, reqArg.HandoffContext, "Briefing Source: Pulse Patrol structured finding")
			assert.Contains(t, reqArg.HandoffContext, "Finding: High CPU usage (critical, performance, active)")
			assert.Contains(t, reqArg.HandoffContext, "Resource: web-server (vm) [vm-100] on pve-1")
			assert.Contains(t, reqArg.HandoffContext, "Priority: critical performance; status active; loop awaiting_approval; raised 3 times; regressed 2 times")
			assert.Contains(t, reqArg.HandoffContext, "Attention Reason: active critical finding; regressed 2 times; last regression 2026-05-06T12:06:00Z; loop awaiting_approval; approval approval-123; destructive proposed fix; fix queued for governed review")
			assert.Contains(t, reqArg.HandoffContext, "Recency: detected 2026-05-06T12:00:00Z; last seen 2026-05-06T12:02:00Z; raised 3 times; regressed 2 times; last regression 2026-05-06T12:06:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Investigation: completed; outcome fix_queued; confidence high; attempts 1")
			assert.Contains(t, reqArg.HandoffContext, "Evidence Snapshot: metrics: CPU stayed above 95% for 10 minutes")
			assert.Contains(t, reqArg.HandoffContext, "Verification: CPU returned below 50%")
			assert.Contains(t, reqArg.HandoffContext, "Latest Lifecycle Event: 2026-05-06T12:03:00Z | investigation_completed | Fix queued for approval | investigating -> fix_queued")
			assert.Contains(t, reqArg.HandoffContext, "Current Conclusion: Backup job saturated CPU.")
			assert.Contains(t, reqArg.HandoffContext, "Recommended Next Step: Approve a controlled service restart after backup completion.")
			assert.Contains(t, reqArg.HandoffContext, "Operator Decision: review governed approval approval-123 before execution; proposed fix fix-123; risk medium; destructive true")
			assert.Contains(t, reqArg.HandoffContext, "Action Posture: approval approval-123; proposed fix fix-123; risk medium; destructive true; remediation remediation-123")
			assert.Contains(t, reqArg.HandoffContext, "Operator Boundary: Treat Patrol data as product context for explanation and review")
			assert.Contains(t, reqArg.HandoffContext, "[Finding Context]")
			assert.Contains(t, reqArg.HandoffContext, "[Investigation Record]")
			assert.Contains(t, reqArg.HandoffContext, "Finding Status: active")
			assert.Contains(t, reqArg.HandoffContext, "Source: ai-patrol")
			assert.Contains(t, reqArg.HandoffContext, "Finding Detected At: 2026-05-06T12:00:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Finding Last Seen At: 2026-05-06T12:02:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Finding Times Raised: 3")
			assert.Contains(t, reqArg.HandoffContext, "AI Context: The spike overlaps the nightly backup job.")
			assert.Contains(t, reqArg.HandoffContext, "AI Confidence: 0.87")
			assert.Contains(t, reqArg.HandoffContext, "Root Cause ID: finding-root")
			assert.Contains(t, reqArg.HandoffContext, "Correlated Finding 1: finding-storage")
			assert.Contains(t, reqArg.HandoffContext, "[Related Finding Context]")
			assert.Contains(t, reqArg.HandoffContext, "Root Cause Finding: finding-root | Backup storage pressure (warning, capacity, active) | resource backup-store (storage) [storage-100] on pve-1 | recency detected 2026-05-06T11:55:00Z; last seen 2026-05-06T12:02:00Z; latest lifecycle 2026-05-06T11:58:00Z | correlated | Backup latency identified | detected -> root_cause | investigation completed; outcome root_cause; ai confidence 0.74 | conclusion Backup storage pressure preceded the workload CPU saturation.")
			assert.Contains(t, reqArg.HandoffContext, "Correlated Finding 1: finding-storage | Datastore latency (warning, capacity, active) | resource vm-datastore (storage) [storage-200] on pve-2 | recency detected 2026-05-06T11:56:00Z; last seen 2026-05-06T12:02:00Z | investigation completed; outcome resolved; confidence medium | conclusion Datastore latency recovered after backup completion.")
			assert.Contains(t, reqArg.HandoffContext, "Related Finding Boundary: Related findings are current unified finding context for explanation only")
			assert.Contains(t, reqArg.HandoffContext, "Remediation ID: remediation-123")
			assert.Contains(t, reqArg.HandoffContext, "Last Investigated At: 2026-05-06T12:04:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Investigation Attempts: 1")
			assert.Contains(t, reqArg.HandoffContext, "Loop State: awaiting_approval")
			assert.Contains(t, reqArg.HandoffContext, "Regression Count: 2")
			assert.Contains(t, reqArg.HandoffContext, "Last Regression At: 2026-05-06T12:06:00Z")
			assert.Contains(t, reqArg.HandoffContext, "[Finding Lifecycle Context]")
			assert.Contains(t, reqArg.HandoffContext, "Lifecycle Event 2: 2026-05-06T12:03:00Z | investigation_completed | Fix queued for approval | investigating -> fix_queued")
			assert.Contains(t, reqArg.HandoffContext, "Lifecycle Boundary: Finding lifecycle events are current Patrol review context only")
			assert.Contains(t, reqArg.HandoffContext, "Resource ID: vm-100")
			assert.Contains(t, reqArg.HandoffContext, "Subject Resource ID: vm-100")
			assert.Contains(t, reqArg.HandoffContext, "Conclusion: Backup job saturated CPU.")
			assert.Contains(t, reqArg.HandoffContext, "Recommended Action: Approve a controlled service restart after backup completion.")
			assert.Contains(t, reqArg.HandoffContext, "Evidence 1: metrics: CPU stayed above 95% for 10 minutes")
			assert.Contains(t, reqArg.HandoffContext, "Proposed Fix: Restart the workload service")
			assert.Contains(t, reqArg.HandoffContext, "Proposed Fix Commands: 1 command recorded for approval context")
			assert.NotContains(t, reqArg.HandoffContext, "User message: What happened?")
			assert.NotContains(t, reqArg.HandoffContext, "systemctl restart workload.service")
			assert.Equal(t, []chat.HandoffResource{{
				ID:   "vm-100",
				Name: "web-server",
				Type: "vm",
				Node: "pve-1",
			}, {
				ID:   "storage-100",
				Name: "backup-store",
				Type: "storage",
				Node: "pve-1",
			}, {
				ID:   "storage-200",
				Name: "vm-datastore",
				Type: "storage",
				Node: "pve-2",
			}}, reqArg.HandoffResources)
			assert.Equal(t, []chat.HandoffAction{{
				FindingID:          "finding-123",
				RecordID:           "investigation-123",
				ApprovalID:         "approval-123",
				FixID:              "fix-123",
				Description:        "Restart the workload service",
				RiskLevel:          "medium",
				Destructive:        true,
				TargetHost:         "pve-1",
				TargetResourceID:   "vm-100",
				TargetResourceName: "web-server",
				TargetResourceType: "vm",
				TargetNode:         "pve-1",
			}}, reqArg.HandoffActions)
			assert.NotContains(t, fmt.Sprintf("%#v", reqArg.HandoffActions), "systemctl restart workload.service")
		})

	body := `{"prompt":"What happened?","finding_id":"finding-123"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleChat_RecoversLivePatrolApprovalForFindingHandoffAction(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	prevStore := approval.GetStore()
	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:        t.TempDir(),
		DefaultTimeout: 10 * time.Minute,
		MaxApprovals:   10,
	})
	assert.NoError(t, err)
	approval.SetStore(approvalStore)
	t.Cleanup(func() {
		approval.SetStore(prevStore)
	})

	assert.NoError(t, approvalStore.CreateApproval(&approval.ApprovalRequest{
		ID:         "approval-live",
		ToolID:     "investigation_fix",
		Command:    "systemctl restart workload.service",
		TargetType: "vm",
		TargetID:   "finding-123",
		TargetName: "web-server",
		Context:    "Restart the workload service after backup saturation clears.",
		RiskLevel:  approval.RiskHigh,
	}))

	detectedAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	store := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	store.AddFromAI(&unified.UnifiedFinding{
		ID:                   "finding-123",
		Source:               unified.SourceAIPatrol,
		Severity:             unified.SeverityCritical,
		Category:             unified.CategoryPerformance,
		ResourceID:           "vm-100",
		ResourceName:         "web-server",
		ResourceType:         "vm",
		Node:                 "pve-1",
		Title:                "High CPU usage",
		Description:          "CPU stayed above 95%.",
		InvestigationStatus:  "completed",
		InvestigationOutcome: "fix_queued",
		LoopState:            "awaiting_approval",
		InvestigationRecord: &aicontracts.InvestigationRecord{
			ID:        "investigation-123",
			FindingID: "finding-123",
			Subject: aicontracts.InvestigationRecordSubject{
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Node:         "pve-1",
			},
			Trigger: aicontracts.InvestigationRecordTrigger{
				Title:      "High CPU usage",
				DetectedAt: detectedAt,
			},
			Status:     aicontracts.InvestigationStatusCompleted,
			Outcome:    aicontracts.OutcomeFixQueued,
			Confidence: aicontracts.InvestigationRecordConfidenceHigh,
			Conclusion: "Backup job saturated CPU.",
			ProposedFix: &aicontracts.InvestigationRecordFix{
				ID:          "fix-123",
				Description: "Restart the workload service",
				Commands:    []string{"systemctl restart workload.service"},
				RiskLevel:   "medium",
				TargetHost:  "pve-1",
				Destructive: true,
			},
			StartedAt: detectedAt,
		},
	})
	h.SetUnifiedStore(store)

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			assert.Equal(t, []chat.HandoffAction{{
				FindingID:          "finding-123",
				RecordID:           "investigation-123",
				ApprovalID:         "approval-live",
				FixID:              "fix-123",
				Description:        "Restart the workload service",
				RiskLevel:          "high",
				Destructive:        true,
				TargetHost:         "pve-1",
				TargetResourceID:   "vm-100",
				TargetResourceName: "web-server",
				TargetResourceType: "vm",
				TargetNode:         "pve-1",
			}}, reqArg.HandoffActions)
			assert.Contains(t, reqArg.HandoffContext, "Operator Decision: review governed approval approval-live before execution; proposed fix fix-123; risk high; destructive true")
			assert.Contains(t, reqArg.HandoffContext, "Action Posture: approval approval-live; proposed fix fix-123; risk high; destructive true")
			assert.NotContains(t, reqArg.HandoffContext, "Action Posture: proposed fix fix-123; risk medium")
			assert.NotContains(t, reqArg.HandoffContext, "systemctl restart workload.service")
			assert.NotContains(t, fmt.Sprintf("%#v", reqArg.HandoffActions), "systemctl restart workload.service")
		})

	body := `{"prompt":"What approval is waiting?","finding_id":"finding-123"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleChat_RefreshesStoredFindingContextForFollowUp(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	resolvedAt := time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)
	store := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	store.AddFromAI(&unified.UnifiedFinding{
		ID:                   "finding-123",
		Source:               unified.SourceAIPatrol,
		Severity:             unified.SeverityWarning,
		Category:             unified.CategoryReliability,
		ResourceID:           "vm-100",
		ResourceName:         "web-server",
		ResourceType:         "vm",
		Node:                 "pve-1",
		Title:                "Backup pressure resolved",
		Description:          "CPU pressure returned to baseline.",
		Recommendation:       "Keep monitoring the next backup window.",
		InvestigationStatus:  "completed",
		InvestigationOutcome: "resolved",
		UserNote:             "Operator confirmed the maintenance window completed.",
		ResolvedAt:           &resolvedAt,
		DetectedAt:           resolvedAt.Add(-30 * time.Minute),
		LastSeenAt:           resolvedAt,
	})
	h.SetUnifiedStore(store)

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("GetModelHandoffFindingID", mock.Anything, "session-123").
		Return("finding-123", nil)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			assert.Equal(t, "session-123", reqArg.SessionID)
			assert.Equal(t, "finding-123", reqArg.FindingID)
			assert.Equal(t, "What changed?", reqArg.Prompt)
			assert.Contains(t, reqArg.HandoffContext, "[Finding Context]")
			assert.Contains(t, reqArg.HandoffContext, "Finding Status: resolved")
			assert.Contains(t, reqArg.HandoffContext, "Recency: detected 2026-05-06T12:30:00Z; last seen 2026-05-06T13:00:00Z; resolved 2026-05-06T13:00:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Finding Detected At: 2026-05-06T12:30:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Finding Last Seen At: 2026-05-06T13:00:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Finding Resolved At: 2026-05-06T13:00:00Z")
			assert.Contains(t, reqArg.HandoffContext, "Title: Backup pressure resolved")
			assert.Contains(t, reqArg.HandoffContext, "Investigation Outcome: resolved")
			assert.Contains(t, reqArg.HandoffContext, "User Note: Operator confirmed the maintenance window completed.")
			assert.NotContains(t, reqArg.HandoffContext, "User message: What changed?")
		})

	body := `{"prompt":"What changed?","session_id":"session-123"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestHandleChat_ClearsStoredFindingContextWhenFollowUpFindingMissing(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	h.SetUnifiedStore(unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig()))

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("GetModelHandoffFindingID", mock.Anything, "session-123").
		Return("finding-missing", nil)
	mockSvc.
		On("ClearModelHandoffContext", mock.Anything, "session-123").
		Return(nil)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			assert.Equal(t, "session-123", reqArg.SessionID)
			assert.Equal(t, "", reqArg.FindingID)
			assert.Equal(t, "What changed?", reqArg.Prompt)
			assert.Equal(t, "", reqArg.HandoffContext)
			assert.Empty(t, reqArg.HandoffResources)
			assert.Empty(t, reqArg.HandoffActions)
		})

	body := `{"prompt":"What changed?","session_id":"session-123"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestUnifiedFindingChatStatusLifecycleStates(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	resolvedAt := now.Add(-time.Minute)
	snoozedUntil := now.Add(time.Hour)

	assert.Equal(t, "active", unifiedFindingChatStatus(&unified.UnifiedFinding{}, now))
	assert.Equal(t, "resolved", unifiedFindingChatStatus(&unified.UnifiedFinding{ResolvedAt: &resolvedAt}, now))
	assert.Equal(t, "snoozed", unifiedFindingChatStatus(&unified.UnifiedFinding{SnoozedUntil: &snoozedUntil}, now))
	assert.Equal(t, "dismissed", unifiedFindingChatStatus(&unified.UnifiedFinding{DismissedReason: "noise"}, now))
	assert.Equal(t, "suppressed", unifiedFindingChatStatus(&unified.UnifiedFinding{Suppressed: true}, now))
	assert.Equal(t, "suppressed", unifiedFindingChatStatus(&unified.UnifiedFinding{DismissedReason: "noise", Suppressed: true}, now))
}

func TestHandleChat_DropsLegacyMentionTypes(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	mockSvc.
		On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			reqArg := args.Get(1).(chat.ExecuteRequest)
			if len(reqArg.Mentions) != 1 {
				t.Fatalf("mentions len = %d, want 1 (%+v)", len(reqArg.Mentions), reqArg.Mentions)
			}
			assert.Equal(t, "agent", reqArg.Mentions[0].Type)
		})

	body := `{"prompt":"hi","mentions":[{"id":"host:node-1","name":"node-1","type":"host"},{"id":"system-container:pve1:200","name":"ct200","type":"system_container","node":"pve1"},{"id":"docker:agent-1:nginx","name":"nginx","type":"docker_container"},{"id":"ct:pve1:201","name":"ct201","type":"container","node":"pve1"},{"id":"ct:pve1:202","name":"ct202","type":"lxc","node":"pve1"},{"id":"docker:agent-1:db","name":"db","type":"docker-container"},{"id":"k8s:cluster-1","name":"cluster-1","type":"k8s"},{"id":"agent:node-2","name":"node-2","type":"agent"}]}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleChat(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCanonicalizeChatMentionType_RejectsRemovedAliases(t *testing.T) {
	assert.Equal(t, "agent", canonicalizeChatMentionType("truenas"))
	assert.Equal(t, "", canonicalizeChatMentionType("host"))
	assert.Equal(t, "", canonicalizeChatMentionType("container"))
	assert.Equal(t, "", canonicalizeChatMentionType("lxc"))
	assert.Equal(t, "", canonicalizeChatMentionType("docker"))
	assert.Equal(t, "", canonicalizeChatMentionType("docker-container"))
	assert.Equal(t, "", canonicalizeChatMentionType("k8s"))
	assert.Equal(t, "", canonicalizeChatMentionType("system_container"))
	assert.Equal(t, "", canonicalizeChatMentionType("docker_container"))
	assert.Equal(t, "", canonicalizeChatMentionType("app_container"))
}

func TestHandleAnswerQuestion(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("AnswerQuestion", mock.Anything, "q1", mock.Anything).Return(nil)

	body := `{"answers": [{"id": "a1", "value": "v1"}]}`
	req := httptest.NewRequest("POST", "/api/ai/question/q1/answer", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleAnswerQuestion(w, req, "q1")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSessions_NotRunning(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(false)

	req := httptest.NewRequest("GET", "/api/ai/sessions", nil)
	w := httptest.NewRecorder()
	h.HandleSessions(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleSessions_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("ListSessions", mock.Anything).Return(([]chat.Session)(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/api/ai/sessions", nil)
	w := httptest.NewRecorder()
	h.HandleSessions(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleCreateSession_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("CreateSession", mock.Anything).Return((*chat.Session)(nil), assert.AnError)

	req := httptest.NewRequest("POST", "/api/ai/sessions", nil)
	w := httptest.NewRecorder()
	h.HandleCreateSession(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleDeleteSession_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("DeleteSession", mock.Anything, "s1").Return(assert.AnError)

	req := httptest.NewRequest("DELETE", "/api/ai/sessions/s1", nil)
	w := httptest.NewRecorder()
	h.HandleDeleteSession(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleMessages_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("GetMessages", mock.Anything, "s1").Return(([]chat.Message)(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/api/ai/sessions/s1/messages", nil)
	w := httptest.NewRecorder()
	h.HandleMessages(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleAbort_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("AbortSession", mock.Anything, "s1").Return(assert.AnError)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/abort", nil)
	w := httptest.NewRecorder()
	h.HandleAbort(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleSummarize_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("SummarizeSession", mock.Anything, "s1").Return((map[string]interface{})(nil), assert.AnError)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/summarize", nil)
	w := httptest.NewRecorder()
	h.HandleSummarize(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleAnswerQuestion_InvalidJSON(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)

	req := httptest.NewRequest("POST", "/api/ai/question/q1/answer", strings.NewReader("invalid"))
	w := httptest.NewRecorder()
	h.HandleAnswerQuestion(w, req, "q1")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAnswerQuestion_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("AnswerQuestion", mock.Anything, "q1", mock.Anything).Return(assert.AnError)

	body := `{"answers": []}`
	req := httptest.NewRequest("POST", "/api/ai/question/q1/answer", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleAnswerQuestion(w, req, "q1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleChat_Options(t *testing.T) {
	h := newTestAIHandler(&config.Config{AllowedOrigins: "*"}, nil, nil)
	req := httptest.NewRequest("OPTIONS", "/api/ai/chat", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	h.HandleChat(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestHandleChat_Options_DisallowedOrigin(t *testing.T) {
	h := newTestAIHandler(&config.Config{AllowedOrigins: "https://allowed.com"}, nil, nil)
	req := httptest.NewRequest("OPTIONS", "/api/ai/chat", nil)
	req.Header.Set("Origin", "https://not-allowed.com")
	w := httptest.NewRecorder()
	h.HandleChat(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
}

func TestHandleChat_MethodNotAllowed(t *testing.T) {
	h := newTestAIHandler(nil, nil, nil)
	req := httptest.NewRequest("GET", "/api/ai/chat", nil)
	w := httptest.NewRecorder()
	h.HandleChat(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleChat_Error(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)

	body := `{"prompt": "hi"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleChat(w, req)
	// ExecuteStream error happens after headers are sent, so w.Code might be 200
	// but the error is returned.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleChat_BindsExecutionToRequestContext(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	reqCtx, cancelReq := context.WithCancel(context.Background())
	defer cancelReq()

	executeDone := make(chan struct{})
	handlerDone := make(chan struct{})

	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("ExecuteStream", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		cancelReq()
		<-ctx.Done()
		if ctx.Err() != context.Canceled {
			t.Fatalf("expected request cancellation, got %v", ctx.Err())
		}
		close(executeDone)
	})

	body := `{"prompt":"hi"}`
	req := httptest.NewRequest("POST", "/api/ai/chat", strings.NewReader(body)).WithContext(reqCtx)
	w := httptest.NewRecorder()

	go func() {
		defer close(handlerDone)
		h.HandleChat(w, req)
	}()

	select {
	case <-executeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected ExecuteStream context to be canceled with the request")
	}

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected handler to return after request cancellation")
	}
}

func TestHandleDiff_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("GetSessionDiff", mock.Anything, "s1").Return((map[string]interface{})(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/api/ai/sessions/s1/diff", nil)
	w := httptest.NewRecorder()
	h.HandleDiff(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleFork_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("ForkSession", mock.Anything, "s1").Return((*chat.Session)(nil), assert.AnError)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/fork", nil)
	w := httptest.NewRecorder()
	h.HandleFork(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleRevert_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("RevertSession", mock.Anything, "s1").Return((map[string]interface{})(nil), assert.AnError)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/revert", nil)
	w := httptest.NewRecorder()
	h.HandleRevert(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleUnrevert_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("UnrevertSession", mock.Anything, "s1").Return((map[string]interface{})(nil), assert.AnError)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/unrevert", nil)
	w := httptest.NewRecorder()
	h.HandleUnrevert(w, req, "s1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleStatus_NotRunning(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(false)

	req := httptest.NewRequest("GET", "/api/ai/status", nil)
	w := httptest.NewRecorder()
	h.HandleStatus(w, req)
	assert.Equal(t, http.StatusOK, w.Code) // HandleStatus returns 200 even if not running
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.False(t, resp["running"].(bool))
}

func TestMockUnimplemented(t *testing.T) {
	mockSvc := new(MockAIService)
	mockSvc.On("SetFindingsManager", mock.Anything).Return()
	mockSvc.On("SetMetadataUpdater", mock.Anything).Return()
	mockSvc.On("UpdateControlSettings", mock.Anything).Return()

	h := newTestAIHandler(nil, nil, nil)
	h.defaultService = mockSvc

	h.SetFindingsManager(nil)
	h.SetMetadataUpdater(nil)
	h.UpdateControlSettings(nil)

	mockSvc.AssertExpectations(t)
}

func TestProviders(t *testing.T) {
	h := newTestAIHandler(nil, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("SetAlertProvider", mock.Anything).Return()
	mockSvc.On("SetFindingsProvider", mock.Anything).Return()
	mockSvc.On("SetBaselineProvider", mock.Anything).Return()
	mockSvc.On("SetPatternProvider", mock.Anything).Return()
	mockSvc.On("SetMetricsHistory", mock.Anything).Return()
	mockSvc.On("SetAgentProfileManager", mock.Anything).Return()
	mockSvc.On("SetBackupProvider", mock.Anything).Return()
	mockSvc.On("SetDiskHealthProvider", mock.Anything).Return()
	mockSvc.On("SetUpdatesProvider", mock.Anything).Return()

	h.SetAlertProvider(nil)
	h.SetFindingsProvider(nil)
	h.SetBaselineProvider(nil)
	h.SetPatternProvider(nil)
	h.SetMetricsHistory(nil)
	h.SetAgentProfileManager(nil)
	h.SetBackupProvider(nil)
	h.SetDiskHealthProvider(nil)
	h.SetUpdatesProvider(nil)

	mockSvc.AssertExpectations(t)
}

func TestHandleAbort_Success(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("AbortSession", mock.Anything, "s1").Return(nil)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/abort", nil)
	w := httptest.NewRecorder()
	h.HandleAbort(w, req, "s1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleSummarize_Success(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("SummarizeSession", mock.Anything, "s1").Return(map[string]interface{}{"summary": "ok"}, nil)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/summarize", nil)
	w := httptest.NewRecorder()
	h.HandleSummarize(w, req, "s1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleDiff_Success(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("GetSessionDiff", mock.Anything, "s1").Return(map[string]interface{}{"diff": "test"}, nil)

	req := httptest.NewRequest("GET", "/api/ai/sessions/s1/diff", nil)
	w := httptest.NewRecorder()
	h.HandleDiff(w, req, "s1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleFork_Success(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("ForkSession", mock.Anything, "s1").Return(&chat.Session{ID: "s2"}, nil)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/fork", nil)
	w := httptest.NewRecorder()
	h.HandleFork(w, req, "s1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRevert_Success(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("RevertSession", mock.Anything, "s1").Return(map[string]interface{}{"reverted": true}, nil)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/revert", nil)
	w := httptest.NewRecorder()
	h.HandleRevert(w, req, "s1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleUnrevert_Success(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("UnrevertSession", mock.Anything, "s1").Return(map[string]interface{}{"unreverted": true}, nil)

	req := httptest.NewRequest("POST", "/api/ai/sessions/s1/unrevert", nil)
	w := httptest.NewRecorder()
	h.HandleUnrevert(w, req, "s1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleStatus_NoService(t *testing.T) {
	// HandleStatus with no service initialized should still return 200 with running=false
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)

	req := httptest.NewRequest("GET", "/api/ai/status", nil)
	w := httptest.NewRecorder()

	h.HandleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.False(t, resp["running"].(bool))
}

func TestGetService_MultiTenantInitAndCache(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	persistence, err := mtp.GetPersistence("acme")
	if err != nil {
		t.Fatalf("GetPersistence(acme): %v", err)
	}
	saveEnabledTestAIConfig(t, persistence)
	h := NewAIHandler(mtp, nil, nil)

	mockSvc := new(MockAIService)
	mockSvc.On("Start", mock.Anything).Return(nil).Once()

	var gotCfg chat.Config
	newChatService = func(cfg chat.Config) AIService {
		gotCfg = cfg
		return mockSvc
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	svc := h.GetService(ctx)
	assert.Same(t, mockSvc, svc)

	expectedDir := filepath.Join(tempDir, "orgs", "acme")
	assert.Equal(t, expectedDir, gotCfg.DataDir)
	assert.NotNil(t, gotCfg.AIConfig)

	// Second call should return cached service without re-starting
	svc = h.GetService(ctx)
	assert.Same(t, mockSvc, svc)

	mockSvc.AssertExpectations(t)
}

func TestGetService_MultiTenantUsesTenantReadState(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	persistence, err := mtp.GetPersistence("acme")
	if err != nil {
		t.Fatalf("GetPersistence(acme): %v", err)
	}
	saveEnabledTestAIConfig(t, persistence)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"acme": tenantMonitor})

	h := NewAIHandler(mtp, mtm, nil)
	globalReadState := unifiedresources.NewRegistry(nil)
	h.SetReadState(globalReadState)

	mockSvc := new(MockAIService)
	mockSvc.On("Start", mock.Anything).Return(nil).Once()

	var gotCfg chat.Config
	newChatService = func(cfg chat.Config) AIService {
		gotCfg = cfg
		return mockSvc
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	svc := h.GetService(ctx)
	assert.Same(t, mockSvc, svc)

	if gotCfg.ReadState != tenantAdapter {
		t.Fatalf("expected tenant read state adapter, got %#v", gotCfg.ReadState)
	}
	if gotCfg.ReadState == globalReadState {
		t.Fatalf("expected tenant read state to override global read state")
	}

	mockSvc.AssertExpectations(t)
}

func TestGetService_HostedTenantDoesNotAutoBootstrapQuickstartService(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	_, err := mtp.GetPersistence("t-tenant")
	if err != nil {
		t.Fatalf("GetPersistence(t-tenant): %v", err)
	}
	seedHostedAIBillingState(t, mtp, "default")

	h := NewAIHandler(mtp, nil, nil)
	h.hostedMode = true

	newChatService = func(cfg chat.Config) AIService {
		t.Fatalf("newChatService must not be called without explicit BYOK/local AI config: %#v", cfg.AIConfig)
		return nil
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "t-tenant")
	svc := h.GetService(ctx)
	assert.Nil(t, svc)

	persistence, err := mtp.GetPersistence("t-tenant")
	if err != nil {
		t.Fatalf("GetPersistence(t-tenant): %v", err)
	}
	assert.False(t, persistence.HasAIConfig())
}

func TestGetService_MultiTenantStartFailureDoesNotCacheDeadService(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	persistence, err := mtp.GetPersistence("acme")
	if err != nil {
		t.Fatalf("GetPersistence(acme): %v", err)
	}
	saveEnabledTestAIConfig(t, persistence)

	h := NewAIHandler(mtp, nil, nil)

	failedSvc := new(MockAIService)
	failedSvc.On("Start", mock.Anything).Return(assert.AnError).Once()

	runningSvc := new(MockAIService)
	runningSvc.On("Start", mock.Anything).Return(nil).Once()

	created := 0
	newChatService = func(cfg chat.Config) AIService {
		created++
		if created == 1 {
			return failedSvc
		}
		return runningSvc
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	svc := h.GetService(ctx)
	assert.Nil(t, svc)
	_, cached := h.services["acme"]
	assert.False(t, cached, "failed tenant start should not be cached")

	svc = h.GetService(ctx)
	assert.Same(t, runningSvc, svc)
	assert.Equal(t, 2, created)

	failedSvc.AssertExpectations(t)
	runningSvc.AssertExpectations(t)
}

func TestSetServiceInitializer_AppliesToExistingServices(t *testing.T) {
	h := NewAIHandler(nil, nil, nil)
	legacySvc := new(MockAIService)
	tenantSvc := new(MockAIService)
	h.defaultService = legacySvc
	h.services["acme"] = tenantSvc

	calls := map[string]int{}
	h.SetServiceInitializer(func(ctx context.Context, svc AIService) {
		calls[GetOrgID(ctx)]++
	})

	if calls["default"] != 1 {
		t.Fatalf("expected initializer for default service, got %d", calls["default"])
	}
	if calls["acme"] != 1 {
		t.Fatalf("expected initializer for tenant service, got %d", calls["acme"])
	}
}

func TestGetService_DefaultAppliesServiceInitializer(t *testing.T) {
	h := NewAIHandler(nil, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	calls := 0
	lastOrg := ""
	h.SetServiceInitializer(func(ctx context.Context, svc AIService) {
		calls++
		lastOrg = GetOrgID(ctx)
	})

	svc := h.GetService(context.Background())
	assert.Same(t, mockSvc, svc)
	if calls == 0 {
		t.Fatal("expected service initializer to be called")
	}
	if lastOrg != "default" {
		t.Fatalf("expected initializer org default, got %q", lastOrg)
	}
}

func TestGetService_MultiTenantAppliesServiceInitializerOnCreate(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)
	tenantPersistence, err := mtp.GetPersistence("acme")
	if err != nil {
		t.Fatalf("tenant persistence: %v", err)
	}
	if err := tenantPersistence.SaveAIConfig(config.AIConfig{Enabled: true}); err != nil {
		t.Fatalf("save tenant AI config: %v", err)
	}

	tenantMonitor := &monitoring.Monitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"acme": tenantMonitor})

	mockSvc := new(MockAIService)
	mockSvc.On("Start", mock.Anything).Return(nil).Once()
	newChatService = func(cfg chat.Config) AIService {
		return mockSvc
	}

	h := NewAIHandler(mtp, mtm, nil)
	calls := 0
	var seenOrg string
	h.SetServiceInitializer(func(ctx context.Context, svc AIService) {
		calls++
		seenOrg = GetOrgID(ctx)
		if svc != mockSvc {
			t.Fatalf("expected initializer to receive tenant service")
		}
	})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	svc := h.GetService(ctx)
	assert.True(t, svc == mockSvc, "expected tenant initializer to receive the created tenant service")
	if calls != 1 {
		t.Fatalf("expected initializer called once for tenant service, got %d", calls)
	}
	if seenOrg != "acme" {
		t.Fatalf("expected initializer org acme, got %q", seenOrg)
	}

	mockSvc.AssertExpectations(t)
}

func TestRemoveTenantService(t *testing.T) {
	h := NewAIHandler(nil, nil, nil)
	mockSvc := new(MockAIService)
	mockSvc.On("Stop", mock.Anything).Return(assert.AnError).Once()
	h.services["acme"] = mockSvc

	err := h.RemoveTenantService(context.Background(), "acme")
	assert.NoError(t, err)
	_, exists := h.services["acme"]
	assert.False(t, exists)

	mockSvc.AssertExpectations(t)
}

func TestRemoveTenantService_DefaultNoop(t *testing.T) {
	h := NewAIHandler(nil, nil, nil)
	mockSvc := new(MockAIService)
	h.services["default"] = mockSvc

	err := h.RemoveTenantService(context.Background(), "default")
	assert.NoError(t, err)
	_, exists := h.services["default"]
	assert.True(t, exists)
}

func TestGetConfig_NonDefaultFallsBackWhenMultiTenantUnavailable(t *testing.T) {
	cfg := &config.Config{APIToken: "token"}
	h := newTestAIHandler(cfg, nil, nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")

	result := h.getConfig(ctx)
	assert.Same(t, cfg, result)
}

func TestGetPersistence_NonDefaultFallsBackWhenMultiTenantUnavailable(t *testing.T) {
	mockPersist := new(MockAIPersistence)
	h := newTestAIHandler(nil, mockPersist, nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")

	result := h.getPersistence(ctx)
	assert.Same(t, mockPersist, result)
}

func TestGetConfig_NonDefaultInvalidOrgFailsClosedWhenMultiTenantAvailable(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	defer mtm.Stop()

	h := NewAIHandler(mtp, mtm, nil)
	h.defaultConfig = &config.Config{APIToken: "token"}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")

	result := h.getConfig(ctx)
	assert.Nil(t, result)
}

func TestGetPersistence_NonDefaultInvalidOrgFailsClosedWhenMultiTenantAvailable(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	defer mtm.Stop()

	h := NewAIHandler(mtp, mtm, nil)
	h.defaultPersistence = config.NewConfigPersistence(t.TempDir())
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")

	result := h.getPersistence(ctx)
	assert.Nil(t, result)
}

func TestReadStateForOrg_NonDefaultMissingTenantReadStateFailsClosed(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	defer mtm.Stop()

	h := NewAIHandler(mtp, mtm, nil)
	h.SetReadState(unifiedresources.NewRegistry(nil))

	result := h.readStateForOrg("tenant-1")
	assert.Nil(t, result)
}

func TestGetDataDirDefault(t *testing.T) {
	h := newTestAIHandler(nil, nil, nil)
	assert.Equal(t, "data", h.getDataDir(nil, ""))
	assert.Equal(t, "custom", h.getDataDir(nil, "custom"))
}

func TestSetMultiTenantPointers(t *testing.T) {
	h := NewAIHandler(nil, nil, nil)
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := &monitoring.MultiTenantMonitor{}

	h.SetMultiTenantPersistence(mtp)
	h.SetMultiTenantMonitor(mtm)

	assert.Same(t, mtp, h.mtPersistence)
	assert.Same(t, mtm, h.mtMonitor)
}
