package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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
	handler := &AISettingsHandler{
		investigationStores: make(map[string]*investigation.Store),
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
		ResourceType: "host",
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
		ResourceType: "host",
		Title:        "title",
		Description:  "desc",
	})
	findings.Add(&ai.Finding{
		ID:           "finding-2",
		Severity:     ai.FindingSeverityCritical,
		Category:     ai.FindingCategorySecurity,
		ResourceID:   "res-2",
		ResourceName: "res-2",
		ResourceType: "host",
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

func TestHandleListApprovals(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("failed to create approval store: %v", err)
	}
	approval.SetStore(store)

	if err := store.CreateApproval(&approval.ApprovalRequest{Command: "echo ok"}); err != nil {
		t.Fatalf("failed to create approval: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/approvals", nil)
	rec := httptest.NewRecorder()
	handler.HandleListApprovals(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Approvals []approval.ApprovalRequest `json:"approvals"`
		Stats     map[string]int             `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(resp.Approvals))
	}
	if resp.Stats["pending"] != 1 {
		t.Fatalf("expected pending approvals to be 1, got %d", resp.Stats["pending"])
	}
}

func TestHandlePatrolAutonomyGetAndUpdate(t *testing.T) {
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

	update := PatrolAutonomySettings{
		AutonomyLevel:           config.PatrolAutonomyFull,
		FullModeUnlocked:        func() *bool { v := true; return &v }(),
		InvestigationBudget:     3,
		InvestigationTimeoutSec: 10,
	}
	body, _ := json.Marshal(update)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(string(body)))
	updateRec := httptest.NewRecorder()
	handler.HandleUpdatePatrolAutonomy(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	var updateResp map[string]interface{}
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	settings := updateResp["settings"].(map[string]interface{})
	if settings["autonomy_level"] != config.PatrolAutonomyFull {
		t.Fatalf("unexpected autonomy level %v", settings["autonomy_level"])
	}
	if settings["full_mode_unlocked"] != true {
		t.Fatalf("expected full_mode_unlocked true, got %v", settings["full_mode_unlocked"])
	}
	if settings["investigation_budget"].(float64) != 5 {
		t.Fatalf("expected clamped budget to 5, got %v", settings["investigation_budget"])
	}
	if settings["investigation_timeout_sec"].(float64) != 60 {
		t.Fatalf("expected clamped timeout to 60, got %v", settings["investigation_timeout_sec"])
	}

	loaded, err := persistence.LoadAIConfig()
	if err != nil {
		t.Fatalf("LoadAIConfig: %v", err)
	}
	if loaded.PatrolAutonomyLevel != config.PatrolAutonomyFull || !loaded.PatrolFullModeUnlocked || loaded.PatrolInvestigationBudget != 5 || loaded.PatrolInvestigationTimeoutSec != 60 {
		t.Fatalf("unexpected persisted settings: %+v", loaded)
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

func TestHandleReapproveInvestigationFix(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("failed to create approval store: %v", err)
	}
	approval.SetStore(store)

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
		ProposedFix: &ai.InvestigationFix{
			ID:          "fix-1",
			Description: "Restart service",
			Commands:    []string{"systemctl restart foo"},
		},
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/finding-1/reapprove", nil)
	rec := httptest.NewRecorder()
	handler.HandleReapproveInvestigationFix(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	approvalID := resp["approval_id"]
	if approvalID == "" {
		t.Fatalf("expected approval_id in response")
	}
	if _, ok := store.GetApproval(approvalID); !ok {
		t.Fatalf("expected approval %s to exist", approvalID)
	}
}

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

func TestHandleReinvestigateFinding(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.PatrolAutonomyLevel = config.PatrolAutonomyApproval
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	svc.SetStateProvider(&MockStateProvider{})

	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}

	callCh := make(chan reinvestigateCall, 1)
	orchestrator := &stubInvestigationOrchestrator{reinvestigateCh: callCh}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/finding-1/reinvestigate", nil)
	rec := httptest.NewRecorder()
	handler.HandleReinvestigateFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	select {
	case call := <-callCh:
		if call.findingID != "finding-1" || call.autonomy != config.PatrolAutonomyApproval {
			t.Fatalf("unexpected reinvestigation call: %+v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected reinvestigation to be triggered")
	}
}

func TestExecuteInvestigationFix_MCPTool(t *testing.T) {
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

	findingID := "finding-1"
	findings := patrol.GetFindings()
	findings.Add(&ai.Finding{
		ID:           findingID,
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "host",
		Title:        "title",
		Description:  "desc",
	})

	store := investigation.NewStore("")
	session := store.Create(findingID, "session-1")
	session.ProposedFix = &investigation.Fix{
		ID:          "fix-1",
		Description: "Get capabilities",
		Commands:    []string{"pulse_get_capabilities()"},
	}
	if !store.Update(session) {
		t.Fatalf("failed to update investigation session")
	}
	handler.investigationStores = map[string]*investigation.Store{"default": store}

	chatSvc := chat.NewService(chat.Config{AIConfig: config.NewDefaultAIConfig()})
	handler.chatHandler = &AIHandler{legacyService: chatSvc}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/approvals/exec", nil)
	rec := httptest.NewRecorder()
	handler.executeInvestigationFix(rec, req, &approval.ApprovalRequest{
		ID:       "approval-1",
		ToolID:   "investigation_fix",
		Command:  "pulse_get_capabilities()",
		TargetID: findingID,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The tool pulse_get_capabilities doesn't exist in the registry, so execution
	// fails gracefully. The handler still returns 200 OK with success=false and
	// records the outcome as fix_failed.
	if resp["success"] != false {
		t.Fatalf("expected success=false for unknown tool, got %v", resp["success"])
	}

	updatedFinding := findings.Get(findingID)
	if updatedFinding == nil || updatedFinding.InvestigationOutcome != string(investigation.OutcomeFixFailed) {
		t.Fatalf("unexpected finding outcome: %+v", updatedFinding)
	}

	updatedSession := store.Get(session.ID)
	if updatedSession == nil || updatedSession.Outcome != investigation.OutcomeFixFailed {
		t.Fatalf("unexpected investigation outcome: %+v", updatedSession)
	}
}

func TestExecuteInvestigationFix_TargetDriftBlocked(t *testing.T) {
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

	findingID := "finding-drift"
	findings := patrol.GetFindings()
	findings.Add(&ai.Finding{
		ID:           findingID,
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "res-1",
		ResourceType: "host",
		Title:        "title",
		Description:  "desc",
	})

	store := investigation.NewStore("")
	session := store.Create(findingID, "session-1")
	session.ProposedFix = &investigation.Fix{
		ID:          "fix-1",
		Description: "Restart service",
		Commands:    []string{"echo ok"},
		TargetHost:  "node-b",
	}
	if !store.Update(session) {
		t.Fatalf("failed to update investigation session")
	}
	handler.investigationStores = map[string]*investigation.Store{"default": store}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/approvals/exec", nil)
	rec := httptest.NewRecorder()
	handler.executeInvestigationFix(rec, req, &approval.ApprovalRequest{
		ID:         "approval-1",
		ToolID:     "investigation_fix",
		Command:    "echo ok",
		TargetID:   findingID,
		TargetName: "node-a",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status conflict, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["code"] != "target_mismatch" {
		t.Fatalf("expected target_mismatch code, got %v", resp["code"])
	}
}

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

func TestFindAgentForTarget(t *testing.T) {
	server := agentexec.NewServer(func(string, string) bool { return true })
	ts := newIPv4HTTPServer(t, http.HandlerFunc(server.HandleWebSocket))
	defer ts.Close()

	conn1 := registerAgent(t, ts.URL, "agent-1", "host-a")
	defer conn1.Close()
	conn2 := registerAgent(t, ts.URL, "agent-2", "host-b")
	defer conn2.Close()

	handler := &AISettingsHandler{agentServer: server}

	if got := handler.findAgentForTarget("host-a"); got != "agent-1" {
		t.Fatalf("expected agent-1, got %q", got)
	}
	if got := handler.findAgentForTarget("agent-2"); got != "agent-2" {
		t.Fatalf("expected agent-2, got %q", got)
	}
	if got := handler.findAgentForTarget(""); got != "" {
		t.Fatalf("expected empty agent when multiple connected, got %q", got)
	}
}
