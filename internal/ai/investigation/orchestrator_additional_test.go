package investigation

import (
	"context"
	"encoding/json"
	"testing"
)

type stubChatService struct {
	sessionID string
	execute   func(StreamCallback) error
}

func (s *stubChatService) CreateSession(ctx context.Context) (*Session, error) {
	if s.sessionID == "" {
		s.sessionID = "session-1"
	}
	return &Session{ID: s.sessionID}, nil
}

func (s *stubChatService) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	if s.execute != nil {
		return s.execute(callback)
	}
	return nil
}

func (s *stubChatService) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	return nil, nil
}

func (s *stubChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (s *stubChatService) ListAvailableTools(ctx context.Context, prompt string) []string {
	return nil
}

func (s *stubChatService) SetAutonomousMode(enabled bool) {}

type stubCommandExecutor struct {
	output string
	code   int
	err    error
}

func (s *stubCommandExecutor) ExecuteCommand(ctx context.Context, command, targetHost string) (string, int, error) {
	return s.output, s.code, s.err
}

type stubApprovalStore struct {
	called bool
	err    error
}

func (s *stubApprovalStore) Create(appr *Approval) error {
	s.called = true
	return s.err
}

type stubFindingsStore struct {
	finding *Finding
	updated bool
}

func (s *stubFindingsStore) Get(id string) *Finding {
	if s.finding != nil && s.finding.ID == id {
		return s.finding
	}
	return nil
}

func (s *stubFindingsStore) Update(f *Finding) bool {
	s.updated = true
	s.finding = f
	return true
}

func TestOrchestrator_ConfigAndLimits(t *testing.T) {
	store := NewStore("")
	orchestrator := NewOrchestrator(&stubChatService{}, store, nil, nil, DefaultConfig())

	cfg := orchestrator.GetConfig()
	cfg.MaxConcurrent = 1
	orchestrator.SetConfig(cfg)

	if !orchestrator.CanStartInvestigation() {
		t.Fatalf("expected investigation to be allowed")
	}

	orchestrator.runningCount = 1
	if orchestrator.CanStartInvestigation() {
		t.Fatalf("expected max concurrent to block")
	}
}

func TestOrchestrator_ExecuteWithLimits_Success(t *testing.T) {
	store := NewStore("")
	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{"text": "analysis"})
			cb(StreamEvent{Type: "content", Data: payload})
			cb(StreamEvent{Type: "tool_end"})
			return nil
		},
	}
	orchestrator := NewOrchestrator(chatService, store, nil, nil, DefaultConfig())
	investigation := store.Create("finding-1", "session-1")

	if err := orchestrator.executeWithLimits(context.Background(), investigation, "prompt", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if investigation.Summary == "" {
		t.Fatalf("expected summary to be set")
	}
}

func TestOrchestrator_ExecuteWithLimits_StreamError(t *testing.T) {
	store := NewStore("")
	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{"message": "boom"})
			cb(StreamEvent{Type: "error", Data: payload})
			return nil
		},
	}
	orchestrator := NewOrchestrator(chatService, store, nil, nil, DefaultConfig())
	investigation := store.Create("finding-1", "session-1")

	if err := orchestrator.executeWithLimits(context.Background(), investigation, "prompt", false); err == nil {
		t.Fatalf("expected stream error")
	}
}

func TestOrchestrator_ProcessResult_ApprovalFlow(t *testing.T) {
	store := NewStore("")
	approval := &stubApprovalStore{}
	findings := &stubFindingsStore{finding: &Finding{ID: "finding-1", Severity: "critical"}}
	orchestrator := NewOrchestrator(&stubChatService{}, store, findings, approval, DefaultConfig())

	investigation := store.Create("finding-1", "session-1")
	investigation.Summary = "PROPOSED_FIX: systemctl restart app\nTARGET_HOST: node-1"

	if err := orchestrator.processResult(context.Background(), investigation, findings.finding, "controlled"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := store.Get(investigation.ID)
	if updated.Outcome != OutcomeFixQueued {
		t.Fatalf("expected fix queued outcome")
	}
	if !approval.called {
		t.Fatalf("expected approval store to be called")
	}
}

func TestOrchestrator_ProcessResult_AutonomySuccess(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "finding-1", Severity: "warning"}}
	orchestrator := NewOrchestrator(&stubChatService{}, store, findings, nil, DefaultConfig())
	orchestrator.SetCommandExecutor(&stubCommandExecutor{output: "ok", code: 0})

	investigation := store.Create("finding-1", "session-1")
	investigation.Summary = "PROPOSED_FIX: echo ok\nTARGET_HOST: local"

	if err := orchestrator.processResult(context.Background(), investigation, findings.finding, "full"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := store.Get(investigation.ID)
	if updated.Outcome != OutcomeFixExecuted {
		t.Fatalf("expected fix executed outcome")
	}
}

func TestOrchestrator_ProcessResult_AutonomyFailure(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "finding-1", Severity: "warning"}}
	orchestrator := NewOrchestrator(&stubChatService{}, store, findings, nil, DefaultConfig())
	orchestrator.SetCommandExecutor(&stubCommandExecutor{output: "fail", code: 1})

	investigation := store.Create("finding-1", "session-1")
	investigation.Summary = "PROPOSED_FIX: echo fail\nTARGET_HOST: local"

	if err := orchestrator.processResult(context.Background(), investigation, findings.finding, "full"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := store.Get(investigation.ID)
	if updated.Outcome != OutcomeFixFailed {
		t.Fatalf("expected fix failed outcome")
	}
}

func TestOrchestrator_ProcessResult_NoExecutor(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "finding-1", Severity: "warning"}}
	orchestrator := NewOrchestrator(&stubChatService{}, store, findings, nil, DefaultConfig())

	investigation := store.Create("finding-1", "session-1")
	investigation.Summary = "PROPOSED_FIX: echo ok\nTARGET_HOST: local"

	if err := orchestrator.processResult(context.Background(), investigation, findings.finding, "full"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := store.Get(investigation.ID)
	if updated.Outcome != OutcomeFixQueued {
		t.Fatalf("expected fix queued outcome")
	}
}

func TestOrchestrator_ProcessResult_NoFix(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "finding-1", Severity: "warning"}}
	orchestrator := NewOrchestrator(&stubChatService{}, store, findings, nil, DefaultConfig())

	investigation := store.Create("finding-1", "session-1")
	investigation.Summary = "CANNOT_FIX: too complex"

	if err := orchestrator.processResult(context.Background(), investigation, findings.finding, "controlled"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := store.Get(investigation.ID)
	if updated.Outcome != OutcomeCannotFix {
		t.Fatalf("expected cannot fix outcome")
	}
}

func TestOrchestrator_ParseInvestigationSummary(t *testing.T) {
	orchestrator := NewOrchestrator(&stubChatService{}, NewStore(""), nil, nil, DefaultConfig())

	fix, outcome := orchestrator.parseInvestigationSummary("PROPOSED_FIX: echo ok\nTARGET_HOST: node-1")
	if fix == nil || outcome != OutcomeFixQueued {
		t.Fatalf("expected fix proposal")
	}

	if _, outcome = orchestrator.parseInvestigationSummary("CANNOT_FIX: no"); outcome != OutcomeCannotFix {
		t.Fatalf("expected cannot fix outcome")
	}
	if _, outcome = orchestrator.parseInvestigationSummary("NEEDS_ATTENTION: help"); outcome != OutcomeNeedsAttention {
		t.Fatalf("expected needs attention outcome")
	}
	if _, outcome = orchestrator.parseInvestigationSummary("unknown"); outcome != OutcomeNeedsAttention {
		t.Fatalf("expected default needs attention outcome")
	}
}

func TestOrchestrator_ReinvestigateFinding(t *testing.T) {
	store := NewStore("")
	findings := &stubFindingsStore{finding: &Finding{ID: "finding-1"}}
	chatService := &stubChatService{
		execute: func(cb StreamCallback) error {
			payload, _ := json.Marshal(map[string]string{"text": "CANNOT_FIX: ok"})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}
	orchestrator := NewOrchestrator(chatService, store, findings, nil, DefaultConfig())

	if err := orchestrator.ReinvestigateFinding(context.Background(), "finding-1", "controlled"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOrchestrator_ReinvestigateFinding_Errors(t *testing.T) {
	orchestrator := NewOrchestrator(&stubChatService{}, NewStore(""), nil, nil, DefaultConfig())
	if err := orchestrator.ReinvestigateFinding(context.Background(), "missing", "controlled"); err == nil {
		t.Fatalf("expected error for missing store")
	}

	findings := &stubFindingsStore{}
	orchestrator = NewOrchestrator(&stubChatService{}, NewStore(""), findings, nil, DefaultConfig())
	if err := orchestrator.ReinvestigateFinding(context.Background(), "missing", "controlled"); err == nil {
		t.Fatalf("expected error for missing finding")
	}
}

func TestBuildInvestigationPromptAndTrim(t *testing.T) {
	orchestrator := NewOrchestrator(&stubChatService{}, NewStore(""), nil, nil, DefaultConfig())
	finding := &Finding{
		ID:             "finding-1",
		Title:          "CPU High",
		Severity:       "warning",
		Category:       "performance",
		ResourceID:     "vm-1",
		ResourceName:   "web",
		ResourceType:   "vm",
		Description:    "desc",
		Evidence:       "evidence",
		Recommendation: "reco",
	}
	prompt := orchestrator.buildInvestigationPrompt(finding)
	if prompt == "" {
		t.Fatalf("expected prompt")
	}
	if trim("  spaced  ") != "spaced" {
		t.Fatalf("expected trim to remove whitespace")
	}
}

func TestFormatOptional(t *testing.T) {
	if formatOptional("Label", "") != "" {
		t.Fatalf("expected empty optional formatting")
	}
	if formatOptional("Label", "value") == "" {
		t.Fatalf("expected formatted optional")
	}
}

func TestOrchestrator_Getters(t *testing.T) {
	store := NewStore("")
	orchestrator := NewOrchestrator(&stubChatService{}, store, nil, nil, DefaultConfig())

	session := store.Create("finding-1", "session-1")
	store.UpdateStatus(session.ID, StatusRunning)

	if orchestrator.GetInvestigation(session.ID) == nil {
		t.Fatalf("expected investigation")
	}
	if orchestrator.GetInvestigationByFinding("finding-1") == nil {
		t.Fatalf("expected investigation by finding")
	}
	if len(orchestrator.GetRunningInvestigations()) != 1 {
		t.Fatalf("expected running investigations")
	}
	if orchestrator.GetRunningCount() != 1 {
		t.Fatalf("expected running count")
	}
}
