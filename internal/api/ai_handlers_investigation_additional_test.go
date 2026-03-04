package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// testInvestigationStore is a minimal in-memory store for tests that need
// Create/Get/Update/Complete on investigation sessions. It satisfies the
// aicontracts.InvestigationStore interface.
type testInvestigationStore struct {
	mu        sync.RWMutex
	sessions  map[string]*aicontracts.InvestigationSession
	byFinding map[string][]string
	counter   int
}

func newTestInvestigationStore() *testInvestigationStore {
	return &testInvestigationStore{
		sessions:  make(map[string]*aicontracts.InvestigationSession),
		byFinding: make(map[string][]string),
	}
}

func (s *testInvestigationStore) LoadFromDisk() error { return nil }
func (s *testInvestigationStore) ForceSave() error    { return nil }

func (s *testInvestigationStore) Create(findingID, chatSessionID string) *aicontracts.InvestigationSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	session := &aicontracts.InvestigationSession{
		ID:        fmt.Sprintf("inv-%d", s.counter),
		FindingID: findingID,
		SessionID: chatSessionID,
		Status:    aicontracts.InvestigationStatusPending,
		StartedAt: time.Now(),
	}
	s.sessions[session.ID] = session
	s.byFinding[findingID] = append(s.byFinding[findingID], session.ID)
	return session
}

func (s *testInvestigationStore) Get(id string) *aicontracts.InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.sessions[id]; ok {
		cp := *sess
		if sess.ProposedFix != nil {
			fc := *sess.ProposedFix
			cp.ProposedFix = &fc
		}
		return &cp
	}
	return nil
}

func (s *testInvestigationStore) GetByFinding(findingID string) []*aicontracts.InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*aicontracts.InvestigationSession
	for _, id := range s.byFinding[findingID] {
		if sess, ok := s.sessions[id]; ok {
			cp := *sess
			if sess.ProposedFix != nil {
				fc := *sess.ProposedFix
				cp.ProposedFix = &fc
			}
			result = append(result, &cp)
		}
	}
	return result
}

func (s *testInvestigationStore) GetLatestByFinding(findingID string) *aicontracts.InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byFinding[findingID]
	if len(ids) == 0 {
		return nil
	}
	var latest *aicontracts.InvestigationSession
	for _, id := range ids {
		if sess, ok := s.sessions[id]; ok {
			if latest == nil || sess.StartedAt.After(latest.StartedAt) {
				latest = sess
			}
		}
	}
	if latest != nil {
		cp := *latest
		if latest.ProposedFix != nil {
			fc := *latest.ProposedFix
			cp.ProposedFix = &fc
		}
		return &cp
	}
	return nil
}

func (s *testInvestigationStore) GetRunning() []*aicontracts.InvestigationSession { return nil }
func (s *testInvestigationStore) CountRunning() int                               { return 0 }

func (s *testInvestigationStore) Update(session *aicontracts.InvestigationSession) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[session.ID]; !ok {
		return false
	}
	cp := *session
	if session.ProposedFix != nil {
		fc := *session.ProposedFix
		cp.ProposedFix = &fc
	}
	s.sessions[session.ID] = &cp
	return true
}

func (s *testInvestigationStore) UpdateStatus(id string, status aicontracts.InvestigationStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Status = status
		return true
	}
	return false
}

func (s *testInvestigationStore) Complete(id string, outcome aicontracts.InvestigationOutcome, summary string, proposedFix *aicontracts.Fix) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Status = aicontracts.InvestigationStatusCompleted
		sess.Outcome = outcome
		sess.Summary = summary
		if proposedFix != nil {
			fc := *proposedFix
			sess.ProposedFix = &fc
		}
		now := time.Now()
		sess.CompletedAt = &now
		return true
	}
	return false
}

func (s *testInvestigationStore) Fail(id string, errorMsg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Status = aicontracts.InvestigationStatusFailed
		sess.Error = errorMsg
		now := time.Now()
		sess.CompletedAt = &now
		return true
	}
	return false
}

func (s *testInvestigationStore) SetOutcome(id string, outcome aicontracts.InvestigationOutcome) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.Outcome = outcome
		return true
	}
	return false
}

func (s *testInvestigationStore) IncrementTurnCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.TurnCount++
		return sess.TurnCount
	}
	return 0
}

func (s *testInvestigationStore) SetApprovalID(id, approvalID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		sess.ApprovalID = approvalID
		return true
	}
	return false
}

func (s *testInvestigationStore) GetAll() []*aicontracts.InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*aicontracts.InvestigationSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		cp := *sess
		if sess.ProposedFix != nil {
			fc := *sess.ProposedFix
			cp.ProposedFix = &fc
		}
		result = append(result, &cp)
	}
	return result
}

func (s *testInvestigationStore) CountFixed() int             { return 0 }
func (s *testInvestigationStore) Cleanup(_ time.Duration) int { return 0 }
func (s *testInvestigationStore) EnforceSizeLimit(_ int) int  { return 0 }

type stubInvestigationOrchestrator struct {
	session           *ai.InvestigationSession
	reinvestigateCh   chan reinvestigateCall
	lastAutonomy      string
	lastReinvestigate string
}

type reinvestigateCall struct {
	findingID string
	autonomy  string
}

func (s *stubInvestigationOrchestrator) InvestigateFinding(ctx context.Context, finding *ai.InvestigationFinding, autonomyLevel string) error {
	return nil
}

func (s *stubInvestigationOrchestrator) GetInvestigationByFinding(findingID string) *ai.InvestigationSession {
	if s.session == nil || s.session.FindingID != findingID {
		return nil
	}
	return s.session
}

func (s *stubInvestigationOrchestrator) GetRunningCount() int {
	return 0
}

func (s *stubInvestigationOrchestrator) GetFixedCount() int {
	return 0
}

func (s *stubInvestigationOrchestrator) CanStartInvestigation() bool {
	return true
}

func (s *stubInvestigationOrchestrator) ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error {
	s.lastReinvestigate = findingID
	s.lastAutonomy = autonomyLevel
	if s.reinvestigateCh != nil {
		s.reinvestigateCh <- reinvestigateCall{findingID: findingID, autonomy: autonomyLevel}
	}
	return nil
}

func (s *stubInvestigationOrchestrator) Shutdown(ctx context.Context) error {
	return nil
}

type stubChatService struct {
	messages []ai.ChatMessage
}

func (s *stubChatService) CreateSession(ctx context.Context) (*ai.ChatSession, error) {
	return &ai.ChatSession{ID: "session-1"}, nil
}

func (s *stubChatService) ExecuteStream(ctx context.Context, req ai.ChatExecuteRequest, callback ai.ChatStreamCallback) error {
	return nil
}

func (s *stubChatService) GetMessages(ctx context.Context, sessionID string) ([]ai.ChatMessage, error) {
	return s.messages, nil
}

func (s *stubChatService) ExecutePatrolStream(ctx context.Context, req ai.PatrolExecuteRequest, callback ai.ChatStreamCallback) (*ai.PatrolStreamResponse, error) {
	return &ai.PatrolStreamResponse{}, nil
}

func (s *stubChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (s *stubChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

func TestSetupInvestigationOrchestrator_WiresTenantBudgetChecker(t *testing.T) {
	// Register factories so setupInvestigationOrchestrator can create a store and orchestrator.
	prevStoreFactory := getCreateInvestigationStore()
	SetCreateInvestigationStore(func(dataDir string) aicontracts.InvestigationStore {
		return newTestInvestigationStore()
	})
	t.Cleanup(func() { SetCreateInvestigationStore(prevStoreFactory) })

	prevOrchestratorFactory := getCreateInvestigationOrchestrator()
	SetCreateInvestigationOrchestrator(func(deps aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator {
		return &stubInvestigationOrchestrator{}
	})
	t.Cleanup(func() { SetCreateInvestigationOrchestrator(prevOrchestratorFactory) })

	handler := &AISettingsHandler{
		investigationStores: make(map[string]aicontracts.InvestigationStore),
	}

	tenantSvc := ai.NewService(nil, nil)
	tenantSvc.SetStateProvider(&MockStateProvider{})
	if tenantSvc.GetPatrolService() == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	tenantChatSvc := chat.NewService(chat.Config{AIConfig: config.NewDefaultAIConfig()})
	handler.chatHandler = &AIHandler{
		services: map[string]AIService{
			"tenant-1": tenantChatSvc,
		},
	}

	handler.setupInvestigationOrchestrator("tenant-1", tenantSvc)

	if tenantSvc.GetChatService() == nil {
		t.Fatalf("expected tenant AI service chat adapter to be wired")
	}
	if _, ok := tenantSvc.GetChatService().(*chatServiceAdapter); !ok {
		t.Fatalf("expected chat service adapter wiring, got %T", tenantSvc.GetChatService())
	}

	budgetChecker := reflect.ValueOf(tenantChatSvc).Elem().FieldByName("budgetChecker")
	if !budgetChecker.IsValid() || budgetChecker.IsNil() {
		t.Fatalf("expected tenant chat service budget checker to be wired")
	}
}

func TestFindingsStoreWrapper_GetAndUpdate(t *testing.T) {
	store := ai.NewFindingsStore()
	store.Add(&ai.Finding{
		ID:           "finding-1",
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "agent",
		Title:        "title",
		Description:  "desc",
	})

	wrapper := &findingsStoreWrapper{store: store}
	found := wrapper.Get("finding-1")
	if found == nil || found.GetID() != "finding-1" {
		t.Fatalf("expected finding to be returned")
	}
	if wrapper.Get("missing") != nil {
		t.Fatalf("expected missing finding to return nil")
	}

	updated := wrapper.UpdateInvestigation("finding-1", "session-1", "running", "outcome", nil, 2)
	if !updated {
		t.Fatalf("expected UpdateInvestigation to return true")
	}
	got := store.Get("finding-1")
	if got.InvestigationOutcome != "outcome" || got.InvestigationStatus != "running" || got.InvestigationAttempts != 2 {
		t.Fatalf("unexpected investigation update: %+v", got)
	}

	nilWrapper := &findingsStoreWrapper{store: nil}
	if nilWrapper.Get("finding-1") != nil {
		t.Fatalf("expected nil store to return nil")
	}
	if nilWrapper.UpdateInvestigation("finding-1", "session-1", "running", "outcome", nil, 1) {
		t.Fatalf("expected nil store update to return false")
	}
}

func TestHandleClearAllFindings(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	findings := patrol.GetFindings()
	findings.Add(&ai.Finding{
		ID:           "finding-1",
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "agent",
		Title:        "title",
		Description:  "desc",
	})
	findings.Add(&ai.Finding{
		ID:           "finding-2",
		Severity:     ai.FindingSeverityCritical,
		Category:     ai.FindingCategorySecurity,
		ResourceID:   "res-2",
		ResourceName: "res-2",
		ResourceType: "agent",
		Title:        "title",
		Description:  "desc",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/ai/patrol/findings?confirm=true", nil)
	rec := httptest.NewRecorder()
	handler.HandleClearAllFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["cleared"].(float64) != 2 {
		t.Fatalf("expected 2 findings cleared, got %v", resp["cleared"])
	}
	if got := patrol.GetFindings().GetAll(nil); len(got) != 0 {
		t.Fatalf("expected findings store to be empty")
	}
}

// TestHandleListApprovals, TestHandleListApprovals_IsolatesByOrg,
// TestHandlePatrolAutonomyGetAndUpdate have been moved to enterprise.
// The GET handler test (HandleGetPatrolAutonomy) remains below.

func TestHandlePatrolAutonomyGet(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.PatrolAutonomyLevel = config.PatrolAutonomyApproval
	aiCfg.PatrolInvestigationBudget = 8
	aiCfg.PatrolInvestigationTimeoutSec = 120
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	getReq := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/autonomy", nil)
	getRec := httptest.NewRecorder()
	handler.HandleGetPatrolAutonomy(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var getResp PatrolAutonomySettings
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if getResp.AutonomyLevel != config.PatrolAutonomyApproval || getResp.InvestigationBudget != 8 || getResp.InvestigationTimeoutSec != 120 {
		t.Fatalf("unexpected autonomy settings: %+v", getResp)
	}
}

func TestHandleGetInvestigation(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}

	session := &ai.InvestigationSession{
		ID:        "inv-1",
		FindingID: "finding-1",
		SessionID: "session-1",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/finding-1/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ai.InvestigationSession
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != "inv-1" || resp.FindingID != "finding-1" {
		t.Fatalf("unexpected investigation response: %+v", resp)
	}
}

// TestHandleReapproveInvestigationFix and TestHandleReapproveInvestigationFix_SetsOrgIDFromContext
// have been moved to enterprise.

func TestHandleGetInvestigationMessages(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	svc.SetChatService(&stubChatService{
		messages: []ai.ChatMessage{
			{ID: "msg-1", Role: "assistant", Content: "hello"},
		},
	})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}

	session := &ai.InvestigationSession{
		ID:        "inv-1",
		FindingID: "finding-1",
		SessionID: "session-1",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/finding-1/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["session_id"] != "session-1" {
		t.Fatalf("unexpected session_id %v", resp["session_id"])
	}
	msgs := resp["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

// TestHandleReinvestigateFinding, TestExecuteInvestigationFix_MCPTool, and
// TestExecuteInvestigationFix_TargetDriftBlocked have been moved to enterprise.

func wsURLForHTTP(url string) string {
	if strings.HasPrefix(url, "https://") {
		return "wss://" + strings.TrimPrefix(url, "https://")
	}
	return "ws://" + strings.TrimPrefix(url, "http://")
}

func registerAgent(t *testing.T, url, agentID, hostname string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(url), nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

	msg, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  agentID,
		Hostname: hostname,
		Version:  "1.0.0",
		Platform: "linux",
		Token:    "ok",
	})
	if err != nil {
		conn.Close()
		t.Fatalf("failed to build registration message: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		conn.Close()
		t.Fatalf("failed to write registration message: %v", err)
	}

	_, raw, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		t.Fatalf("failed to read registration response: %v", err)
	}

	var resp agentexec.Message
	if err := json.Unmarshal(raw, &resp); err != nil {
		conn.Close()
		t.Fatalf("failed to decode registration response: %v", err)
	}
	var reg agentexec.RegisteredPayload
	if err := resp.DecodePayload(&reg); err != nil {
		conn.Close()
		t.Fatalf("failed to decode registration payload: %v", err)
	}
	if !reg.Success {
		conn.Close()
		t.Fatalf("registration failed: %s", reg.Message)
	}

	return conn
}

func TestAgentCommandAdapter_FindAgentForTarget(t *testing.T) {
	server := agentexec.NewServer(func(string, string) bool { return true })
	ts := newIPv4HTTPServer(t, http.HandlerFunc(server.HandleWebSocket))
	defer ts.Close()

	conn1 := registerAgent(t, ts.URL, "agent-1", "host-a")
	defer conn1.Close()
	conn2 := registerAgent(t, ts.URL, "agent-2", "host-b")
	defer conn2.Close()

	adapter := &agentCommandAdapter{handler: &AISettingsHandler{agentServer: server}}

	if got := adapter.FindAgentForTarget("host-a"); got != "agent-1" {
		t.Fatalf("expected agent-1, got %q", got)
	}
	if got := adapter.FindAgentForTarget("agent-2"); got != "agent-2" {
		t.Fatalf("expected agent-2, got %q", got)
	}
	if got := adapter.FindAgentForTarget(""); got != "" {
		t.Fatalf("expected empty agent when multiple connected, got %q", got)
	}
}
