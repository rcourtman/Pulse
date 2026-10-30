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
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

func TestPatrolActionReconciliationCannotRegressFromOutOfOrderCallback(t *testing.T) {
	investigations := newTestInvestigationStore()
	investigation := investigations.Create("finding-1", "session-1")
	audits := unifiedresources.NewMemoryStore()
	now := time.Now().UTC()
	pending := unifiedresources.ActionAuditRecord{
		ID: "act-1", CreatedAt: now, UpdatedAt: now, State: unifiedresources.ActionStatePending,
		Request: unifiedresources.ActionRequest{RequestID: "proposal-1", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
		Plan:    unifiedresources.ActionPlan{ActionID: "act-1", RequestID: "proposal-1", Allowed: true, RequiresApproval: true},
		Origin:  &unifiedresources.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: investigation.ID, ProposalID: "proposal-1"},
	}
	completed := pending
	completed.State = unifiedresources.ActionStateCompleted
	completed.UpdatedAt = now.Add(time.Second)
	completed.Result = &unifiedresources.ExecutionResult{Success: true}
	completed.VerificationOutcome = unifiedresources.VerificationOutcome{Status: unifiedresources.VerificationVerified}
	if _, _, err := audits.CreateActionAudit(completed, nil); err != nil {
		t.Fatalf("CreateActionAudit(completed): %v", err)
	}

	handler := &AISettingsHandler{
		investigationStores:   map[string]aicontracts.InvestigationStore{"default": investigations},
		resourceStoreProvider: func(string) (unifiedresources.ResourceStore, error) { return audits, nil },
	}
	// A stale callback payload must not regress the already-completed audit.
	handler.ReconcilePatrolActionTransition("default", pending)
	got := investigations.Get(investigation.ID)
	if got.Action == nil || got.Action.ActionID != "act-1" || got.Action.State != string(unifiedresources.ActionStateCompleted) {
		t.Fatalf("reconciled action = %#v, want authoritative completed action", got.Action)
	}
	if got.Outcome != aicontracts.OutcomeFixVerificationUnknown {
		t.Fatalf("outcome = %q, want %q", got.Outcome, aicontracts.OutcomeFixVerificationUnknown)
	}
}

func TestPatrolActionHydrationRepairsDurableFindingRecordWithoutRewritingEvidence(t *testing.T) {
	investigations := newTestInvestigationStore()
	investigation := investigations.Create("finding-1", "session-1")
	investigation.Status = aicontracts.InvestigationStatusCompleted
	investigation.Outcome = aicontracts.OutcomeFixQueued
	investigation.Summary = "Cause uncertain. Restart proposed for review."
	investigation.EvidenceIDs = []string{"observed-health"}
	investigations.Update(investigation)
	svc := ai.NewService(nil, nil)
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	findings := patrol.GetFindings()
	findings.Add(&ai.Finding{ID: "finding-1", ResourceID: "vm:42", Title: "Unhealthy service", Severity: ai.FindingSeverityWarning,
		InvestigationStatus: string(investigation.Status), InvestigationOutcome: string(aicontracts.OutcomeFixRejected)})
	// Reproduce the persisted mismatch after an outcome already reconciled.
	record := ai.BuildFindingInvestigationRecord(findings.Get("finding-1"), investigation)
	record.Rollback = []string{"Retained rollback evidence"}
	record.Impact = "Original service impact absent from the later finding projection."
	findings.UpdateInvestigationRecord("finding-1", record)
	audits := unifiedresources.NewMemoryStore()
	audit := unifiedresources.ActionAuditRecord{
		ID: "act-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), State: unifiedresources.ActionStateRejected,
		Request: unifiedresources.ActionRequest{RequestID: "proposal-1", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
		Plan:    unifiedresources.ActionPlan{ActionID: "act-1", RequestID: "proposal-1", Allowed: true},
		Origin:  &unifiedresources.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: investigation.ID, ProposalID: "proposal-1"},
	}
	if _, _, err := audits.CreateActionAudit(audit, nil); err != nil {
		t.Fatal(err)
	}
	handler := &AISettingsHandler{defaultAIService: svc,
		investigationStores:   map[string]aicontracts.InvestigationStore{"default": investigations},
		resourceStoreProvider: func(string) (unifiedresources.ResourceStore, error) { return audits, nil },
	}
	var published []*ai.Finding
	patrol.SetUnifiedFindingCallback(func(f *ai.Finding) bool { published = append(published, f); return true })
	published = nil // Ignore the initial synchronization when registering the callback.
	handler.hydratePatrolInvestigationAction("default", investigation)
	got := findings.Get("finding-1").InvestigationRecord
	if got.Outcome != aicontracts.OutcomeFixRejected || got.Action == nil || got.Action.State != "rejected" {
		t.Fatalf("durable record did not reconcile: %#v", got)
	}
	if got.Conclusion != investigation.Summary || got.Impact != record.Impact || got.Confidence != record.Confidence || !reflect.DeepEqual(got.Rollback, record.Rollback) || !reflect.DeepEqual(got.Evidence, record.Evidence) {
		t.Fatalf("retained investigation evidence changed: %#v", got)
	}
	if len(got.Verification) != 1 || len(published) != 1 || published[0].InvestigationRecord.Outcome != aicontracts.OutcomeFixRejected {
		t.Fatalf("reconciled record was not published: record=%#v published=%#v", got, published)
	}
	handler.hydratePatrolInvestigationAction("default", investigation)
	if len(published) != 1 {
		t.Fatal("duplicate hydration republished unchanged record")
	}
}

func TestPatrolActionReconciliationHydratesTerminalAuditAfterRestart(t *testing.T) {
	investigations := newTestInvestigationStore()
	investigation := investigations.Create("finding-1", "session-1")
	dir := t.TempDir()
	audits, err := unifiedresources.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	record := unifiedresources.ActionAuditRecord{
		ID: "act-missed", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), State: unifiedresources.ActionStateRejected,
		Request: unifiedresources.ActionRequest{RequestID: "proposal-1", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
		Plan:    unifiedresources.ActionPlan{ActionID: "act-missed", RequestID: "proposal-1", Allowed: true, RequiresApproval: true},
		Origin:  &unifiedresources.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: investigation.ID, ProposalID: "proposal-1"},
	}
	if _, _, err := audits.CreateActionAudit(record, nil); err != nil {
		t.Fatalf("CreateActionAudit: %v", err)
	}
	if err := audits.Close(); err != nil {
		t.Fatal(err)
	}
	audits, err = unifiedresources.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer audits.Close()
	handler := &AISettingsHandler{
		investigationStores:   map[string]aicontracts.InvestigationStore{"default": investigations},
		resourceStoreProvider: func(string) (unifiedresources.ResourceStore, error) { return audits, nil },
	}

	hydrated := handler.hydratePatrolInvestigationAction("default", investigation)
	if hydrated.Action == nil || hydrated.Action.ActionID != "act-missed" || hydrated.Action.State != string(unifiedresources.ActionStateRejected) {
		t.Fatalf("hydrated action = %#v", hydrated.Action)
	}
	if hydrated.Outcome != aicontracts.OutcomeFixRejected {
		t.Fatalf("hydrated outcome = %q, want rejected", hydrated.Outcome)
	}
	persisted := investigations.Get(investigation.ID)
	if persisted.Action == nil || persisted.Action.ActionID != "act-missed" || persisted.Outcome != aicontracts.OutcomeFixRejected {
		t.Fatalf("persisted repair = %#v", persisted)
	}
}

func TestPatrolOutcomeForActionAuditPreservesInconclusiveVerification(t *testing.T) {
	for _, status := range []unifiedresources.VerificationStatus{unifiedresources.VerificationUnknown, unifiedresources.VerificationUnverified} {
		audit := unifiedresources.ActionAuditRecord{
			State:               unifiedresources.ActionStateCompleted,
			VerificationOutcome: unifiedresources.VerificationOutcome{Status: status},
		}
		if got := patrolOutcomeForActionAudit(audit); got != aicontracts.OutcomeFixVerificationUnknown {
			t.Fatalf("status %q mapped to %q, want verification_unknown", status, got)
		}
	}
}

type stubInvestigationOrchestrator struct {
	session           *ai.InvestigationSession
	investigateCh     chan string
	reinvestigateCh   chan reinvestigateCall
	lastAutonomy      string
	lastReinvestigate string
}

type reinvestigateCall struct {
	findingID string
	autonomy  string
}

func (s *stubInvestigationOrchestrator) InvestigateFinding(ctx context.Context, finding *ai.InvestigationFinding, autonomyLevel string) error {
	if s.investigateCh != nil {
		s.investigateCh <- finding.ID
	}
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
		mtPersistence:       config.NewMultiTenantPersistence(t.TempDir()),
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
	if nilWrapper.StartInvestigation("finding-1", "session-1", time.Time{}, "running", "", nil, 1) {
		t.Fatalf("expected nil store start to return false")
	}
	if nilWrapper.CompleteInvestigation("finding-1", "session-1", time.Time{}, "completed", "outcome", nil, 1) {
		t.Fatalf("expected nil store completion to return false")
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

	req := newLoopbackRequest(http.MethodDelete, "/api/ai/patrol/findings?confirm=true", nil)
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

	getReq := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/autonomy", nil)
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
		ProposedFix: &ai.InvestigationFix{
			ID:          "fix-1",
			Description: "Restart service",
		},
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/findings/finding-1/investigation", nil)
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
	if resp.ToolsAvailable == nil || resp.ToolsUsed == nil || resp.EvidenceIDs == nil {
		t.Fatalf("expected investigation arrays to normalize, got %+v", resp)
	}
	if resp.ProposedFix == nil || resp.ProposedFix.Commands == nil {
		t.Fatalf("expected proposed fix commands to normalize, got %+v", resp.ProposedFix)
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

	req := newLoopbackRequest(http.MethodGet, "/api/ai/findings/finding-1/investigation/messages", nil)
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
	first, ok := msgs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected message object, got %#v", msgs[0])
	}
	toolCalls, ok := first["tool_calls"].([]interface{})
	if !ok {
		t.Fatalf("expected tool_calls array, got %#v", first["tool_calls"])
	}
	if len(toolCalls) != 0 {
		t.Fatalf("expected empty tool_calls array, got %#v", toolCalls)
	}
}

// TestHandleReinvestigateFinding, TestExecuteInvestigationFix_AssistantTool, and
// TestExecuteInvestigationFix_TargetDriftBlocked have been moved to enterprise.

func wsURLForHTTP(url string) string {
	if strings.HasPrefix(url, "https://") {
		return "wss://" + strings.TrimPrefix(url, "https://")
	}
	return "ws://" + strings.TrimPrefix(url, "http://")
}

func wsHeadersForHTTP(t *testing.T, url string) http.Header {
	t.Helper()

	origin, err := securityutil.HTTPOriginForWebSocketBaseURL(url)
	if err != nil {
		t.Fatalf("failed to derive websocket origin: %v", err)
	}

	headers := http.Header{}
	headers.Set("Origin", origin)
	return headers
}

func registerAgent(t *testing.T, url, agentID, hostname string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(url), wsHeadersForHTTP(t, url))
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
	server := agentexec.NewServer(func(string, string, string) bool { return true })
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

func TestPatrolExpiredActionHydrationRemovesQueuedOutcomeWithoutRewritingEvidence(t *testing.T) {
	investigations := newTestInvestigationStore()
	investigation := investigations.Create("finding-1", "session-1")
	investigation.Status = aicontracts.InvestigationStatusCompleted
	investigation.Outcome = aicontracts.OutcomeFixQueued
	investigation.Summary = "Cause uncertain. Restart proposed for review."
	investigation.EvidenceIDs = []string{"observed-health"}
	investigations.Update(investigation)
	svc := ai.NewService(nil, nil)
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	findings := patrol.GetFindings()
	findings.Add(&ai.Finding{ID: "finding-1", ResourceID: "vm:42", Title: "Unhealthy service", Severity: ai.FindingSeverityWarning,
		InvestigationStatus: string(investigation.Status), InvestigationOutcome: string(aicontracts.OutcomeNeedsAttention)})
	// Reproduce the persisted mismatch after an outcome already reconciled.
	record := ai.BuildFindingInvestigationRecord(findings.Get("finding-1"), investigation)
	record.Rollback = []string{"Retained rollback evidence"}
	record.Impact = "Original service impact absent from the later finding projection."
	findings.UpdateInvestigationRecord("finding-1", record)
	audits := unifiedresources.NewMemoryStore()
	audit := unifiedresources.ActionAuditRecord{
		ID: "act-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), State: unifiedresources.ActionStateExpired,
		Request: unifiedresources.ActionRequest{RequestID: "proposal-1", ResourceID: "vm:42", CapabilityName: "restart", RequestedBy: "pulse_patrol"},
		Plan:    unifiedresources.ActionPlan{ActionID: "act-1", RequestID: "proposal-1", Allowed: true},
		Origin:  &unifiedresources.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: "finding-1", InvestigationID: investigation.ID, ProposalID: "proposal-1"},
	}
	if _, _, err := audits.CreateActionAudit(audit, nil); err != nil {
		t.Fatal(err)
	}
	handler := &AISettingsHandler{defaultAIService: svc,
		investigationStores:   map[string]aicontracts.InvestigationStore{"default": investigations},
		resourceStoreProvider: func(string) (unifiedresources.ResourceStore, error) { return audits, nil },
	}
	var published []*ai.Finding
	patrol.SetUnifiedFindingCallback(func(f *ai.Finding) bool { published = append(published, f); return true })
	published = nil // Ignore the initial synchronization when registering the callback.
	handler.hydratePatrolInvestigationAction("default", investigation)
	got := findings.Get("finding-1").InvestigationRecord
	if got.Outcome != aicontracts.OutcomeNeedsAttention || got.Action == nil || got.Action.State != "expired" {
		t.Fatalf("durable record did not reconcile: %#v", got)
	}
	if got.Conclusion != investigation.Summary || got.Impact != record.Impact || got.Confidence != record.Confidence || !reflect.DeepEqual(got.Rollback, record.Rollback) || !reflect.DeepEqual(got.Evidence, record.Evidence) {
		t.Fatalf("retained investigation evidence changed: %#v", got)
	}
	if len(got.Verification) != 0 {
		t.Fatalf("unexecuted expired action invented verification: %#v", got.Verification)
	}
	if len(published) != 1 || published[0].InvestigationRecord.Outcome != aicontracts.OutcomeNeedsAttention {
		t.Fatalf("reconciled record was not published: record=%#v published=%#v", got, published)
	}
	handler.hydratePatrolInvestigationAction("default", investigation)
	if len(published) != 1 {
		t.Fatal("duplicate hydration republished unchanged record")
	}
}

func TestSetupInvestigationOrchestratorUsesSubscriptionAgentLimits(t *testing.T) {
	prevStoreFactory := getCreateInvestigationStore()
	SetCreateInvestigationStore(func(dataDir string) aicontracts.InvestigationStore {
		return newTestInvestigationStore()
	})
	t.Cleanup(func() { SetCreateInvestigationStore(prevStoreFactory) })

	var created []aicontracts.InvestigationConfig
	prevOrchestratorFactory := getCreateInvestigationOrchestrator()
	SetCreateInvestigationOrchestrator(func(deps aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator {
		created = append(created, deps.Config)
		return &stubInvestigationOrchestrator{}
	})
	t.Cleanup(func() { SetCreateInvestigationOrchestrator(prevOrchestratorFactory) })

	persistence := config.NewConfigPersistence(t.TempDir())
	saved := config.NewDefaultAIConfig()
	saved.PatrolModel = config.AIProviderCodexSubscription + ":gpt-5.6-luna"
	if err := persistence.SaveAIConfig(*saved); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	tenantSvc := ai.NewService(persistence, nil)
	if err := tenantSvc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	tenantSvc.SetStateProvider(&MockStateProvider{})

	handler := &AISettingsHandler{
		mtPersistence:       config.NewMultiTenantPersistence(t.TempDir()),
		investigationStores: make(map[string]aicontracts.InvestigationStore),
	}
	handler.chatHandler = &AIHandler{
		services: map[string]AIService{
			"tenant-1": chat.NewService(chat.Config{AIConfig: config.NewDefaultAIConfig()}),
		},
	}
	handler.setupInvestigationOrchestrator("tenant-1", tenantSvc)

	if len(created) != 1 {
		t.Fatalf("orchestrator factory calls = %d, want 1", len(created))
	}
	if created[0].MaxConcurrent != 1 {
		t.Fatalf("MaxConcurrent = %d, want 1 for a subscription agent that serves one call at a time", created[0].MaxConcurrent)
	}
	if want := time.Duration(config.DefaultPatrolSubscriptionInvestigationTimeoutSec) * time.Second; created[0].Timeout != want {
		t.Fatalf("Timeout = %s, want %s", created[0].Timeout, want)
	}
}

// orchestratorFindingsFor wires a tenant's investigation orchestrator and
// returns the findings and session stores it was built with, plus the
// tenant's findings.
func orchestratorFindingsFor(t *testing.T) (aicontracts.OrchestratorFindingsStore, aicontracts.InvestigationStore, *ai.FindingsStore) {
	t.Helper()
	findings, sessions, patrol := orchestratorPatrolFor(t, nil)
	return findings, sessions, patrol.GetFindings()
}

// orchestratorPatrolFor is orchestratorFindingsFor with the tenant's Patrol
// service instead of only its findings, and audits as the action audit store
// when it is not nil.
func orchestratorPatrolFor(t *testing.T, audits unifiedresources.ResourceStore) (aicontracts.OrchestratorFindingsStore, aicontracts.InvestigationStore, *ai.PatrolService) {
	t.Helper()
	prevStoreFactory := getCreateInvestigationStore()
	SetCreateInvestigationStore(func(dataDir string) aicontracts.InvestigationStore {
		return newTestInvestigationStore()
	})
	t.Cleanup(func() { SetCreateInvestigationStore(prevStoreFactory) })

	var deps aicontracts.OrchestratorDeps
	prevOrchestratorFactory := getCreateInvestigationOrchestrator()
	SetCreateInvestigationOrchestrator(func(d aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator {
		deps = d
		return &stubInvestigationOrchestrator{}
	})
	t.Cleanup(func() { SetCreateInvestigationOrchestrator(prevOrchestratorFactory) })

	tenantSvc := ai.NewService(nil, nil)
	tenantSvc.SetOrgID("tenant-1")
	tenantSvc.SetStateProvider(&MockStateProvider{})
	// Audit reconciliation resolves the run's service through GetAIService.
	// Register the service owning these findings, rather than letting the
	// persistence manager create a second, unrelated service for the tenant.
	handler := &AISettingsHandler{
		mtPersistence:       config.NewMultiTenantPersistence(t.TempDir()),
		aiServices:          map[string]*ai.Service{"tenant-1": tenantSvc},
		investigationStores: make(map[string]aicontracts.InvestigationStore),
	}
	if audits != nil {
		handler.resourceStoreProvider = func(string) (unifiedresources.ResourceStore, error) { return audits, nil }
	}
	handler.chatHandler = &AIHandler{
		services: map[string]AIService{
			"tenant-1": chat.NewService(chat.Config{AIConfig: config.NewDefaultAIConfig()}),
		},
	}
	handler.setupInvestigationOrchestrator("tenant-1", tenantSvc)
	if deps.FindingsStore == nil || deps.Store == nil {
		t.Fatal("investigation orchestrator was not wired")
	}
	return deps.FindingsStore, deps.Store, tenantSvc.GetPatrolService()
}

func investigatedFinding(id string) *ai.Finding {
	return &ai.Finding{
		ID:           id,
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		ResourceID:   "vm-" + id,
		ResourceName: "vm-" + id,
		ResourceType: "vm",
		Title:        "Memory pressure on vm-" + id,
		Description:  "Memory usage is above 90%",
	}
}

// startOrchestratorRun does what the orchestrator does when it starts a run:
// it records the run's session, then marks the finding as running under it.
func startOrchestratorRun(t *testing.T, findings aicontracts.OrchestratorFindingsStore, sessions aicontracts.InvestigationStore, findingID, chatSessionID string) aicontracts.Finding {
	t.Helper()
	run := findings.Get(findingID)
	if run == nil {
		t.Fatalf("finding %s not found", findingID)
	}
	sessions.Create(findingID, chatSessionID)
	run.InvestigationSessionID = chatSessionID
	run.InvestigationStatus = string(aicontracts.InvestigationStatusRunning)
	run.InvestigationOutcome = ""
	run.InvestigationAttempts++
	if !findings.Update(run) {
		t.Fatalf("starting run %s was not recorded", chatSessionID)
	}
	return *run
}

// finishOrchestratorRun is the orchestrator's finding write when a run ends.
func finishOrchestratorRun(findings aicontracts.OrchestratorFindingsStore, run aicontracts.Finding, status aicontracts.InvestigationStatus, outcome aicontracts.InvestigationOutcome) bool {
	now := time.Now()
	run.InvestigationStatus = string(status)
	run.InvestigationOutcome = string(outcome)
	run.LastInvestigatedAt = &now
	return findings.Update(&run)
}

func assertOrchestratorRunState(t *testing.T, findings *ai.FindingsStore, id, sessionID string, status aicontracts.InvestigationStatus, outcome aicontracts.InvestigationOutcome, attempts int) {
	t.Helper()
	got := findings.Get(id)
	if got == nil {
		t.Fatalf("finding %s not found", id)
	}
	if got.InvestigationSessionID != sessionID || got.InvestigationStatus != string(status) ||
		got.InvestigationOutcome != string(outcome) || got.InvestigationAttempts != attempts {
		t.Fatalf("%s investigation = session %q %s/%s, %d attempts; want session %q %s/%s, %d attempts",
			id, got.InvestigationSessionID, got.InvestigationStatus, got.InvestigationOutcome, got.InvestigationAttempts,
			sessionID, status, outcome, attempts)
	}
}

func TestOrchestratorRunResultSkipsALaterOccurrence(t *testing.T) {
	orchestratorFindings, sessions, findings := orchestratorFindingsFor(t)
	findings.Add(investigatedFinding("finding-1"))
	runA := startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-a")

	// While A is in flight the finding resolves and is detected again.
	findings.Resolve("finding-1", true)
	findings.Add(investigatedFinding("finding-1"))

	// A concludes for the occurrence it investigated, not the current one.
	applied := finishOrchestratorRun(orchestratorFindings, runA, aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeResolved)
	assertOrchestratorRunState(t, findings, "finding-1", "", "", "", 0)
	if applied {
		t.Fatal("a run of the earlier occurrence reported closing the current one")
	}
	if got := findings.Get("finding-1"); got.LastInvestigatedAt != nil || !got.ShouldInvestigate("approval") {
		t.Fatalf("the earlier occurrence's result kept the current one from being investigated: %+v", got)
	}

	// Nor does it land once a run of the current occurrence has started.
	runB := startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-b")
	if finishOrchestratorRun(orchestratorFindings, runA, aicontracts.InvestigationStatusFailed, aicontracts.OutcomeTimedOut) {
		t.Fatal("a run of the earlier occurrence overwrote the current run")
	}
	assertOrchestratorRunState(t, findings, "finding-1", "chat-b", aicontracts.InvestigationStatusRunning, "", 1)

	if !finishOrchestratorRun(orchestratorFindings, runB, aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeNeedsAttention) {
		t.Fatal("the current run's result was refused")
	}
	assertOrchestratorRunState(t, findings, "finding-1", "chat-b", aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeNeedsAttention, 1)
}

func TestOrchestratorRunFromAnEarlierOccurrenceIsNotInstalled(t *testing.T) {
	orchestratorFindings, sessions, findings := orchestratorFindingsFor(t)
	findings.Add(investigatedFinding("finding-1"))

	// A records its session for the first occurrence, with that occurrence's
	// attempt count; the finding resolves and is detected again before A marks
	// it as running.
	runA := orchestratorFindings.Get("finding-1")
	sessions.Create("finding-1", "chat-a")
	findings.Resolve("finding-1", true)
	findings.Add(investigatedFinding("finding-1"))
	runA.InvestigationSessionID = "chat-a"
	runA.InvestigationStatus = string(aicontracts.InvestigationStatusRunning)
	runA.InvestigationAttempts = 3
	if orchestratorFindings.Update(runA) {
		t.Fatal("a run started for the earlier occurrence was installed on the current one")
	}
	assertOrchestratorRunState(t, findings, "finding-1", "", "", "", 0)

	// Nor does A's admission replace a run of the current occurrence.
	runB := startOrchestratorRun(t, orchestratorFindings, sessions, "finding-1", "chat-b")
	if orchestratorFindings.Update(runA) {
		t.Fatal("a run started for the earlier occurrence replaced the current run")
	}
	assertOrchestratorRunState(t, findings, "finding-1", "chat-b", aicontracts.InvestigationStatusRunning, "", 1)
	if finishOrchestratorRun(orchestratorFindings, *runA, aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeFixVerified) {
		t.Fatal("a run of the earlier occurrence recorded its result on the current one")
	}

	if !finishOrchestratorRun(orchestratorFindings, runB, aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeCannotFix) {
		t.Fatal("the current run's result was refused")
	}
	assertOrchestratorRunState(t, findings, "finding-1", "chat-b", aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeCannotFix, 1)
	if findings.Get("finding-1").ResolvedAt != nil {
		t.Fatal("the earlier occurrence's verified fix resolved the current one")
	}
}

func TestOrchestratorRunResultSkipsANewerRun(t *testing.T) {
	orchestratorFindings, sessions, findings := orchestratorFindingsFor(t)
	findings.Add(investigatedFinding("retried"))
	findings.Add(investigatedFinding("late"))
	retriedA := startOrchestratorRun(t, orchestratorFindings, sessions, "retried", "chat-retried-a")
	lateA := startOrchestratorRun(t, orchestratorFindings, sessions, "late", "chat-late-a")

	// The stuck-run sweep gave both runs up as timed out, and the retry
	// started another run for one of them.
	now := time.Now()
	findings.RecoverStuckInvestigation("retried", "chat-retried-a", time.Time{}, ai.InvestigationStatusFailed, ai.InvestigationOutcomeTimedOut, now)
	findings.RecoverStuckInvestigation("late", "chat-late-a", time.Time{}, ai.InvestigationStatusFailed, ai.InvestigationOutcomeTimedOut, now)
	retriedB := startOrchestratorRun(t, orchestratorFindings, sessions, "retried", "chat-retried-b")

	// The first runs report back after all.
	if finishOrchestratorRun(orchestratorFindings, retriedA, aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeResolved) {
		t.Fatal("an earlier run overwrote the run that replaced it")
	}
	assertOrchestratorRunState(t, findings, "retried", "chat-retried-b", aicontracts.InvestigationStatusRunning, "", 2)
	// With no newer run, the run the finding still names has the real result.
	if !finishOrchestratorRun(orchestratorFindings, lateA, aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeCannotFix) {
		t.Fatal("a late result from the finding's own run was refused")
	}
	assertOrchestratorRunState(t, findings, "late", "chat-late-a", aicontracts.InvestigationStatusCompleted, aicontracts.OutcomeCannotFix, 1)

	if !finishOrchestratorRun(orchestratorFindings, retriedB, aicontracts.InvestigationStatusFailed, aicontracts.OutcomeTimedOut) {
		t.Fatal("the newer run's result was refused")
	}
	assertOrchestratorRunState(t, findings, "retried", "chat-retried-b", aicontracts.InvestigationStatusFailed, aicontracts.OutcomeTimedOut, 2)
}

// actionRecordingStore records an investigation's action on its own, as the
// enterprise store does, and counts how often a whole session is replaced.
type actionRecordingStore struct {
	*testInvestigationStore
	replaced int
}

func (s *actionRecordingStore) Update(session *aicontracts.InvestigationSession) bool {
	s.replaced++
	return s.testInvestigationStore.Update(session)
}

func (s *actionRecordingStore) SetAction(id string, action *aicontracts.ActionReference) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return false
	}
	session.Action = aicontracts.CloneActionReference(action)
	return true
}

// patrolProjectionFixture is a running Patrol investigation, the action audit
// store its plans live in, and the handler that projects them onto it.
type patrolProjectionFixture struct {
	handler       *AISettingsHandler
	audits        *unifiedresources.MemoryStore
	sessions      *actionRecordingStore
	investigation *aicontracts.InvestigationSession
}

func newPatrolProjectionFixture(t *testing.T) patrolProjectionFixture {
	t.Helper()
	svc := ai.NewService(nil, nil)
	svc.SetStateProvider(&MockStateProvider{})
	svc.GetPatrolService().GetFindings().Add(&ai.Finding{ID: "finding-1", ResourceID: "vm:42", Title: "Unhealthy service",
		Severity: ai.FindingSeverityWarning, InvestigationSessionID: "session-1",
		InvestigationStatus: string(aicontracts.InvestigationStatusRunning)})
	sessions := &actionRecordingStore{testInvestigationStore: newTestInvestigationStore()}
	investigation := sessions.Create("finding-1", "session-1")
	audits := unifiedresources.NewMemoryStore()
	handler := &AISettingsHandler{defaultAIService: svc,
		investigationStores:   map[string]aicontracts.InvestigationStore{"default": sessions},
		resourceStoreProvider: func(string) (unifiedresources.ResourceStore, error) { return audits, nil },
	}
	return patrolProjectionFixture{handler: handler, audits: audits, sessions: sessions, investigation: investigation}
}

func (f patrolProjectionFixture) recordPlan(t *testing.T, audit unifiedresources.ActionAuditRecord) unifiedresources.ActionAuditRecord {
	t.Helper()
	if err := f.audits.RecordActionAudit(audit); err != nil {
		t.Fatalf("RecordActionAudit(%s): %v", audit.ID, err)
	}
	recorded, found, err := f.audits.GetActionAudit(audit.ID)
	if err != nil || !found {
		t.Fatalf("GetActionAudit(%s): found=%v err=%v", audit.ID, found, err)
	}
	return recorded
}

// A projection records the action and the outcome on their own when the store
// can, so it never replaces the session and cannot undo progress the
// orchestrator saved after the projection read it.
func TestPatrolActionProjectionRecordsOnlyTheActionAndOutcome(t *testing.T) {
	f := newPatrolProjectionFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	plan := f.recordPlan(t, patrolPlanAudit("act-1", f.investigation.ID, now, now.Add(time.Hour)))

	f.handler.ReconcilePatrolActionTransition("default", plan)

	got := f.sessions.Get(f.investigation.ID)
	if got.Action == nil || got.Action.ActionID != "act-1" || got.Outcome != aicontracts.OutcomeFixQueued {
		t.Fatalf("session = %+v, want the planned action and its queued fix", got)
	}
	if f.sessions.replaced != 0 {
		t.Fatalf("the projection replaced the whole session %d times", f.sessions.replaced)
	}
}

// A projection waits for any other projection of the same investigation, so one
// that read an older state of the action cannot land after one that read a
// newer state. Other investigations do not wait for it.
func TestPatrolActionProjectionWaitsForTheInvestigationsLock(t *testing.T) {
	f := newPatrolProjectionFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	plan := f.recordPlan(t, patrolPlanAudit("act-1", f.investigation.ID, now, now.Add(time.Hour)))

	release := lockPatrolInvestigation("default", f.investigation.ID)
	done := make(chan struct{})
	go func() {
		f.handler.ReconcilePatrolActionTransition("default", plan)
		close(done)
	}()
	select {
	case <-done:
		release()
		t.Fatal("a projection ran while another held the investigation's lock")
	case <-time.After(100 * time.Millisecond):
	}
	lockPatrolInvestigation("default", "another-investigation")()
	release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the projection did not run once the lock was free")
	}
	if got := f.sessions.Get(f.investigation.ID); got.Action == nil || got.Action.ActionID != "act-1" {
		t.Fatalf("session = %+v, want the action projected once the lock was free", got)
	}
}

// Hydration returns the action and outcome the investigation's store holds after
// the repair, not a projection of the caller's copy. After a refresh replaced
// the plan, a copy taken before the refresh must not bring back the replaced
// plan, here expired.
func TestPatrolActionHydrationUsesTheInvestigationsCurrentAction(t *testing.T) {
	f := newPatrolProjectionFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	f.recordPlan(t, patrolPlanAudit("act-1", f.investigation.ID, now.Add(-10*time.Minute), now.Add(-5*time.Minute)))
	replacement := patrolPlanAudit("act-2", f.investigation.ID, now.Add(-time.Minute), now.Add(time.Hour))
	replacement.Request.RequestID = "refresh:act-1"
	replacement.Plan.RequestID = "refresh:act-1"
	f.recordPlan(t, replacement)
	if expired, err := f.audits.ExpireActionAudits(now, 10); err != nil || len(expired) != 1 || expired[0].ID != "act-1" {
		t.Fatalf("expired = %+v err=%v, want the replaced plan", expired, err)
	}
	f.sessions.SetAction(f.investigation.ID, &aicontracts.ActionReference{ActionID: "act-2", State: string(unifiedresources.ActionStatePending)})
	f.sessions.SetOutcome(f.investigation.ID, aicontracts.OutcomeFixQueued)
	stale := *f.sessions.Get(f.investigation.ID)
	stale.Action = &aicontracts.ActionReference{ActionID: "act-1", State: string(unifiedresources.ActionStatePending)}

	got := f.handler.hydratePatrolInvestigationAction("default", &stale)

	if got.Action == nil || got.Action.ActionID != "act-2" || got.Outcome != aicontracts.OutcomeFixQueued {
		t.Fatalf("hydrated = action %+v outcome %q, want the replacement still queued", got.Action, got.Outcome)
	}
}

// A run's close finalizes, under the investigation's lock, the session the
// orchestrator closed: a copy the stuck-run sweep marked failed gives way to the
// run's own close once that has landed.
func TestPatrolRunCloseFinalizesTheClosedSessionUnderTheLock(t *testing.T) {
	f := newPatrolProjectionFixture(t)
	f.sessions.Complete(f.investigation.ID, aicontracts.OutcomeResolved, "The service recovered on its own.", nil)
	abandoned := *f.sessions.Get(f.investigation.ID)
	abandoned.Status = aicontracts.InvestigationStatusFailed
	abandoned.Error = "Patrol stopped waiting for this investigation after it ran past its time limit."

	var finalized *aicontracts.InvestigationSession
	f.handler.closePatrolInvestigationRun("default", &abandoned, func(closed *aicontracts.InvestigationSession) {
		finalized = closed
		acquired := make(chan struct{})
		go func() {
			lockPatrolInvestigation("default", f.investigation.ID)()
			close(acquired)
		}()
		select {
		case <-acquired:
			t.Error("a projection of the investigation ran while the run was being finalized")
		case <-time.After(50 * time.Millisecond):
		}
	})

	if finalized == nil || finalized.Status != aicontracts.InvestigationStatusCompleted || finalized.Outcome != aicontracts.OutcomeResolved || finalized.Error != "" {
		t.Fatalf("finalized %+v, want the run's own completed close", finalized)
	}
}

// A run the orchestrator still shows as running, which the stuck-run sweep gave
// up on, is finalized as the sweep marked it.
func TestPatrolRunCloseFinalizesAnAbandonedRunAsGivenUp(t *testing.T) {
	f := newPatrolProjectionFixture(t)
	f.sessions.UpdateStatus(f.investigation.ID, aicontracts.InvestigationStatusRunning)
	abandoned := *f.sessions.Get(f.investigation.ID)
	abandoned.Status = aicontracts.InvestigationStatusFailed
	abandoned.Error = "Patrol stopped waiting for this investigation after it ran past its time limit."

	var finalized *aicontracts.InvestigationSession
	f.handler.closePatrolInvestigationRun("default", &abandoned, func(closed *aicontracts.InvestigationSession) {
		finalized = closed
	})

	if finalized == nil || finalized.Status != aicontracts.InvestigationStatusFailed || finalized.Error != abandoned.Error {
		t.Fatalf("finalized %+v, want the run as the sweep gave it up", finalized)
	}
}

// The investigation keeps the action's canonical result and verification, as
// the broker reports them, and the model's causal attribution, which the
// action audit does not hold.
func TestPatrolActionProjectionKeepsTheFullActionReference(t *testing.T) {
	f := newPatrolProjectionFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	plan := f.recordPlan(t, patrolPlanAudit("act-1", f.investigation.ID, now, now.Add(time.Hour)))
	f.sessions.SetAction(f.investigation.ID, &aicontracts.ActionReference{ActionID: "act-1", State: string(unifiedresources.ActionStatePending), CausalResourceID: "storage:nfs-1"})

	f.handler.ReconcilePatrolActionTransition("default", plan)

	got := f.sessions.Get(f.investigation.ID).Action
	want := dispositionFromRecord(plan)
	if got == nil || got.CausalResourceID != "storage:nfs-1" || len(got.ActionResultV2) == 0 ||
		string(got.ActionResultV2) != string(want.ActionResultV2) || got.VerificationStatus != want.VerificationStatus {
		t.Fatalf("action = %+v, want the broker's result and verification with the causal attribution kept", got)
	}
}

func TestAutoFixOrchestratorSetupUsesRequestedOrg(t *testing.T) {
	for _, tc := range []struct{ input, org string }{
		{"tenant-a", "tenant-a"}, {"tenant-b", "tenant-b"},
		{"default", "default"}, {"", "default"}, {" tenant-a ", "tenant-a"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			f := newHistoryReadFixture(t)
			before := historyFixtureFiles(t, f.root)
			peers := make(map[string]aicontracts.InvestigationOrchestrator)
			for org, svc := range f.services {
				peers[org] = &stubInvestigationOrchestrator{}
				svc.GetPatrolService().SetInvestigationOrchestrator(peers[org])
			}
			f.services[tc.org].GetPatrolService().SetInvestigationOrchestrator(nil)
			factory := getCreateInvestigationOrchestrator()
			SetCreateInvestigationOrchestrator(func(d aicontracts.OrchestratorDeps) aicontracts.InvestigationOrchestrator {
				store := d.Store.(*historyReadDiskStore)
				if store.org != tc.org {
					t.Errorf("store belongs to %q, want %q", store.org, tc.org)
				}
				expectedChat := f.handler.chatHandler.defaultService
				if tc.org != "default" {
					expectedChat = f.handler.chatHandler.services[tc.org]
				}
				if got := d.ChatService.(*orchestratorChatAdapter).svc; got != expectedChat {
					t.Error("factory received another organization's chat service")
				}
				finding := d.FindingsStore.Get("shared-history")
				if finding == nil || finding.InvestigationSessionID != f.records[tc.org].SessionID {
					t.Error("factory received another organization's findings store")
				}
				return factory(d)
			})
			deps := newAIAutoFixHandlerDeps(&Router{aiSettingsHandler: f.handler})
			deps.SetupOrchestrator(tc.input)
			for org, svc := range f.services {
				got := svc.GetPatrolService().GetInvestigationOrchestrator()
				if org == tc.org {
					if got == nil || got == peers[org] {
						t.Fatal("requested organization did not receive its orchestrator")
					}
					req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/shared-history/reinvestigate", nil)
					req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, org))
					if deps.GetOrchestrator(req) != got || deps.GetInvestigationStore(org) == nil {
						t.Error("extension cannot retrieve the exact scoped runtime it just installed")
					}
				} else if got != peers[org] {
					t.Errorf("setup for %q replaced %q's orchestrator", tc.org, org)
				}
			}
			for _, suffix := range []string{"", "/messages"} {
				rec := f.read(tc.org, suffix)
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), f.records[tc.org].SessionID) {
					t.Fatalf("scoped history read: %d %s", rec.Code, rec.Body.String())
				}
				for other, record := range f.records {
					if other != tc.org && strings.Contains(rec.Body.String(), record.SessionID) {
						t.Error("scoped read contains another organization's session")
					}
				}
			}
			if f.factoryCalls.Load() != 1 || len(f.handler.investigationStores) != 1 {
				t.Error("lazy setup or consuming read created an extra runtime")
			}
			if !reflect.DeepEqual(before, historyFixtureFiles(t, f.root)) {
				t.Fatal("setup rewrote retained history, chat or encrypted credentials")
			}
		})
	}
}

func TestAutoFixOrchestratorSetupDoesNotCreateOrBorrowAService(t *testing.T) {
	for _, mode := range []string{"missing", "../tenant-a", "misbound", "single-tenant", "no-default"} {
		t.Run(mode, func(t *testing.T) {
			f := newHistoryReadFixture(t)
			before := historyFixtureFiles(t, f.root)
			requested := mode
			if mode == "misbound" {
				f.handler.aiServices[mode] = f.services["default"]
			}
			if mode == "single-tenant" {
				f.handler.mtPersistence = nil
				requested = "tenant-a"
			}
			if mode == "no-default" {
				f.handler.defaultAIService = nil
				requested = "default"
			}
			servicesBefore := len(f.handler.aiServices)
			peers := make(map[string]aicontracts.InvestigationOrchestrator)
			for org, svc := range f.services {
				peers[org] = &stubInvestigationOrchestrator{}
				svc.GetPatrolService().SetInvestigationOrchestrator(peers[org])
			}
			deps := newAIAutoFixHandlerDeps(&Router{aiSettingsHandler: f.handler})
			deps.SetupOrchestrator(requested)
			if f.storeCalls.Load() != 0 || f.factoryCalls.Load() != 0 || len(f.handler.aiServices) != servicesBefore {
				t.Error("unresolved org initialized a store, orchestrator or AI service")
			}
			for org, svc := range f.services {
				if svc.GetPatrolService().GetInvestigationOrchestrator() != peers[org] {
					t.Errorf("unresolved org changed %q's runtime", org)
				}
			}
			if !reflect.DeepEqual(before, historyFixtureFiles(t, f.root)) {
				t.Fatal("unresolved org changed retained state")
			}
		})
	}
}

func TestAutoFixOrchestratorSetupConcurrentOrganizations(t *testing.T) {
	f := newHistoryReadFixture(t)
	before := historyFixtureFiles(t, f.root)
	deps := newAIAutoFixHandlerDeps(&Router{aiSettingsHandler: f.handler})
	var wg sync.WaitGroup
	for org := range f.services {
		wg.Add(1)
		go func() { defer wg.Done(); deps.SetupOrchestrator(org) }()
	}
	wg.Wait()
	for org, svc := range f.services {
		runtime, ok := svc.GetPatrolService().GetInvestigationOrchestrator().(*historyReadOrchestrator)
		if !ok || runtime.store.(*historyReadDiskStore).org != org {
			t.Errorf("%q's concurrent setup did not bind its own retained store", org)
		}
		for _, suffix := range []string{"", "/messages"} {
			rec := f.read(org, suffix)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), f.records[org].SessionID) {
				t.Errorf("concurrent setup lost %q's original thread: %d %s", org, rec.Code, rec.Body.String())
			}
		}
	}
	if f.factoryCalls.Load() != 3 || len(f.handler.investigationStores) != 3 {
		t.Error("independent setup did not install exactly one runtime per organization")
	}
	if !reflect.DeepEqual(before, historyFixtureFiles(t, f.root)) {
		t.Fatal("concurrent setup changed retained history or credentials")
	}
}
