package investigation

import (
	"context"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type mockAIFinding struct {
	id        string
	severity  string
	category  string
	resource  string
	name      string
	resType   string
	title     string
	desc      string
	reco      string
	evidence  string
	sessionID string
	status    string
	outcome   string
	lastAt    *time.Time
	attempts  int
}

func (m *mockAIFinding) GetID() string                     { return m.id }
func (m *mockAIFinding) GetSeverity() string               { return m.severity }
func (m *mockAIFinding) GetCategory() string               { return m.category }
func (m *mockAIFinding) GetResourceID() string             { return m.resource }
func (m *mockAIFinding) GetResourceName() string           { return m.name }
func (m *mockAIFinding) GetResourceType() string           { return m.resType }
func (m *mockAIFinding) GetTitle() string                  { return m.title }
func (m *mockAIFinding) GetDescription() string            { return m.desc }
func (m *mockAIFinding) GetRecommendation() string         { return m.reco }
func (m *mockAIFinding) GetEvidence() string               { return m.evidence }
func (m *mockAIFinding) GetInvestigationSessionID() string { return m.sessionID }
func (m *mockAIFinding) GetInvestigationStatus() string    { return m.status }
func (m *mockAIFinding) GetInvestigationOutcome() string   { return m.outcome }
func (m *mockAIFinding) GetLastInvestigatedAt() *time.Time { return m.lastAt }
func (m *mockAIFinding) GetInvestigationAttempts() int     { return m.attempts }
func (m *mockAIFinding) SetInvestigationSessionID(string)  {}
func (m *mockAIFinding) SetInvestigationStatus(string)     {}
func (m *mockAIFinding) SetInvestigationOutcome(string)    {}
func (m *mockAIFinding) SetLastInvestigatedAt(*time.Time)  {}
func (m *mockAIFinding) SetInvestigationAttempts(int)      {}

type mockAIFindingsStore struct {
	finding *mockAIFinding
	updated bool
}

func (m *mockAIFindingsStore) Get(id string) AIFinding {
	if m.finding != nil && m.finding.id == id {
		return m.finding
	}
	return nil
}

func (m *mockAIFindingsStore) UpdateInvestigation(id, sessionID, status, outcome string, lastInvestigatedAt *time.Time, attempts int) bool {
	m.updated = true
	return true
}

func TestApprovalAdapter(t *testing.T) {
	adapter := NewApprovalAdapter(nil)
	if err := adapter.Create(&Approval{ID: "a1", RiskLevel: "low"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("unexpected store error: %v", err)
	}

	adapter = NewApprovalAdapter(store)
	if err := adapter.Create(&Approval{
		ID:          "a1",
		RiskLevel:   "critical",
		Command:     "echo ok",
		FindingID:   "f1",
		Description: "desc",
	}); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	req, ok := store.GetApproval("a1")
	if !ok {
		t.Fatalf("expected approval stored")
	}
	if req.RiskLevel != approval.RiskHigh {
		t.Fatalf("expected risk high, got %s", req.RiskLevel)
	}
}

func TestFindingsStoreAdapter(t *testing.T) {
	adapter := NewFindingsStoreAdapter(nil)
	if adapter.Get("missing") != nil {
		t.Fatalf("expected nil for missing store")
	}
	if adapter.Update(nil) {
		t.Fatalf("expected update to fail with nil store")
	}

	finding := &mockAIFinding{
		id:       "f1",
		severity: "critical",
		category: "performance",
		resource: "res-1",
		name:     "node-1",
		resType:  "node",
		title:    "title",
		desc:     "desc",
		reco:     "reco",
		evidence: "evidence",
	}
	store := &mockAIFindingsStore{finding: finding}
	adapter = NewFindingsStoreAdapter(store)

	got := adapter.Get("f1")
	if got == nil || got.ID != "f1" {
		t.Fatalf("expected finding")
	}
	if !adapter.Update(got) {
		t.Fatalf("expected update to succeed")
	}
	if !store.updated {
		t.Fatalf("expected update to be forwarded")
	}
}

type stubStreamingProvider struct{}

func (s *stubStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{}, nil
}

func (s *stubStreamingProvider) TestConnection(ctx context.Context) error {
	return nil
}

func (s *stubStreamingProvider) Name() string {
	return "stub"
}

func (s *stubStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (s *stubStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "ok"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{}})
	return nil
}

func (s *stubStreamingProvider) SupportsThinking(model string) bool {
	return false
}

func setServiceField(t *testing.T, svc *chat.Service, fieldName string, value interface{}) {
	t.Helper()
	val := reflect.ValueOf(svc).Elem().FieldByName(fieldName)
	if !val.IsValid() {
		t.Fatalf("field %s not found", fieldName)
	}
	reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func newTestChatService(t *testing.T) *chat.Service {
	t.Helper()
	dataDir := t.TempDir()
	svc := chat.NewService(chat.Config{
		AIConfig: &config.AIConfig{Enabled: true},
		DataDir:  dataDir,
	})

	sessions, err := chat.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubStreamingProvider{}
	agentic := chat.NewAgenticLoop(provider, executor, "system")

	setServiceField(t, svc, "sessions", sessions)
	setServiceField(t, svc, "agenticLoop", agentic)
	setServiceField(t, svc, "executor", executor)
	setServiceField(t, svc, "started", true)

	return svc
}

func TestChatServiceAdapter_BasicFlow(t *testing.T) {
	adapter := NewChatServiceAdapter(nil)
	if adapter.IsRunning() {
		t.Fatalf("expected nil service to be not running")
	}
	adapter.SetAutonomousMode(true)
	if _, _, err := adapter.ExecuteCommand(context.Background(), "echo ok", ""); err == nil {
		t.Fatalf("expected error for nil service")
	}
	if err := adapter.ExecuteStream(context.Background(), ExecuteRequest{Prompt: "hi"}, func(StreamEvent) {}); err == nil {
		t.Fatalf("expected error for nil service")
	}

	service := newTestChatService(t)
	adapter = NewChatServiceAdapter(service)

	session, err := adapter.CreateSession(context.Background())
	if err != nil || session == nil {
		t.Fatalf("expected session creation")
	}

	gotContent := false
	err = adapter.ExecuteStream(context.Background(), ExecuteRequest{Prompt: "hello", SessionID: session.ID}, func(event StreamEvent) {
		if event.Type == "content" {
			gotContent = true
		}
	})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if !gotContent {
		t.Fatalf("expected content event")
	}

	messages, err := adapter.GetMessages(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("unexpected message error: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected messages")
	}

	if err := adapter.DeleteSession(context.Background(), session.ID); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	setServiceField(t, service, "started", false)
	if err := adapter.ExecuteStream(context.Background(), ExecuteRequest{Prompt: "hi"}, func(StreamEvent) {}); err == nil {
		t.Fatalf("expected error when service not running")
	}
}
