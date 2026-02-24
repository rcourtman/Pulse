package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
func (m *MockAIService) SetTopologyProvider(provider chat.TopologyProvider) { m.Called(provider) }
func (m *MockAIService) SetDiscoveryProvider(provider chat.MCPDiscoveryProvider) {
	m.Called(provider)
}
func (m *MockAIService) SetUnifiedResourceProvider(provider chat.MCPUnifiedResourceProvider) {
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

type MockAIStateProvider struct {
	mock.Mock
}

func (m *MockAIStateProvider) GetState() models.StateSnapshot {
	args := m.Called()
	return args.Get(0).(models.StateSnapshot)
}

func newTestAIHandler(cfg *config.Config, persistence AIPersistence, _ *agentexec.Server) *AIHandler {
	handler := NewAIHandler(nil, nil, nil)
	handler.legacyConfig = cfg
	handler.legacyPersistence = persistence
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
	assert.Nil(t, h.legacyService)

	// AI enabled
	aiCfg := &config.AIConfig{Enabled: true, Model: "test"}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil).Once()
	mockSvc.On("Start", mock.Anything).Return(nil).Once()

	err = h.Start(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, mockSvc, h.legacyService)
}

func TestStop(t *testing.T) {
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, nil, nil)
	h.legacyService = mockSvc

	mockSvc.On("Stop", mock.Anything).Return(nil)
	err := h.Stop(context.Background())
	assert.NoError(t, err)

	// Nil service
	h.legacyService = nil
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
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, mockPersist, nil)
	h.legacyService = mockSvc

	aiCfg := &config.AIConfig{}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil)
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("Restart", mock.Anything, aiCfg).Return(nil)
	err := h.Restart(context.Background())
	assert.NoError(t, err)

	// Service nil
	h.legacyService = nil
	err = h.Restart(context.Background())
	assert.NoError(t, err)
}

func TestGetService(t *testing.T) {
	mockSvc := new(MockAIService)
	h := newTestAIHandler(nil, nil, nil)
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	messages := []chat.Message{{Role: "user", Content: "hello"}}
	mockSvc.On("GetMessages", mock.Anything, "s1").Return(messages, nil)

	req := httptest.NewRequest("GET", "/api/ai/sessions/s1/messages", nil)
	w := httptest.NewRecorder()

	h.HandleMessages(w, req, "s1")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleChat_NotRunning(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc

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

func TestHandleAnswerQuestion(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc
	mockSvc.On("IsRunning").Return(false)

	req := httptest.NewRequest("GET", "/api/ai/sessions", nil)
	w := httptest.NewRecorder()
	h.HandleSessions(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleSessions_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
	mockSvc.On("IsRunning").Return(true)

	req := httptest.NewRequest("POST", "/api/ai/question/q1/answer", strings.NewReader("invalid"))
	w := httptest.NewRecorder()
	h.HandleAnswerQuestion(w, req, "q1")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAnswerQuestion_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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

func TestHandleDiff_Error(t *testing.T) {
	h := newTestAIHandler(&config.Config{}, nil, nil)
	mockSvc := new(MockAIService)
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc

	h.SetFindingsManager(nil)
	h.SetMetadataUpdater(nil)
	h.UpdateControlSettings(nil)

	mockSvc.AssertExpectations(t)
}

func TestProviders(t *testing.T) {
	h := newTestAIHandler(nil, nil, nil)
	mockSvc := new(MockAIService)
	h.legacyService = mockSvc

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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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
	h.legacyService = mockSvc
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

func TestSetServiceInitializer_AppliesToExistingServices(t *testing.T) {
	h := NewAIHandler(nil, nil, nil)
	legacySvc := new(MockAIService)
	tenantSvc := new(MockAIService)
	h.legacyService = legacySvc
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
	h.legacyService = mockSvc

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
	assert.Same(t, mockSvc, svc)
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
	h.stateProviders["acme"] = &MockAIStateProvider{}

	err := h.RemoveTenantService(context.Background(), "acme")
	assert.NoError(t, err)
	_, exists := h.services["acme"]
	assert.False(t, exists)
	_, exists = h.stateProviders["acme"]
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
	h.legacyConfig = &config.Config{APIToken: "token"}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")

	result := h.getConfig(ctx)
	assert.Nil(t, result)
}

func TestGetPersistence_NonDefaultInvalidOrgFailsClosedWhenMultiTenantAvailable(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	defer mtm.Stop()

	h := NewAIHandler(mtp, mtm, nil)
	h.legacyPersistence = config.NewConfigPersistence(t.TempDir())
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
