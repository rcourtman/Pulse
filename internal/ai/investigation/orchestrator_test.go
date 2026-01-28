package investigation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mocks

type MockChatService struct {
	CapturedPrompt string
	ExecuteFunc    func(StreamCallback)
}

func (m *MockChatService) CreateSession(ctx context.Context) (*Session, error) {
	return &Session{ID: uuid.New().String()}, nil
}

func (m *MockChatService) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	m.CapturedPrompt = req.Prompt
	if m.ExecuteFunc != nil {
		m.ExecuteFunc(callback)
	}
	return nil
}

func (m *MockChatService) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	return nil, nil
}
func (m *MockChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}
func (m *MockChatService) ListAvailableTools(ctx context.Context, prompt string) []string {
	return nil
}
func (m *MockChatService) SetAutonomousMode(enabled bool) {}

type MockFindingsStore struct {
	Findings map[string]*Finding
}

func (m *MockFindingsStore) Get(id string) *Finding {
	return m.Findings[id]
}
func (m *MockFindingsStore) Update(f *Finding) bool {
	m.Findings[f.ID] = f
	return true
}

// Test Cases

func TestInvestigateFinding_Success(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	mockChat := &MockChatService{
		ExecuteFunc: func(cb StreamCallback) {
			// Simulate AI response
			cb(StreamEvent{Type: "content", Data: []byte(`{"text": "Analysis: The issue is simple.\n"}`)})
			cb(StreamEvent{Type: "content", Data: []byte(`{"text": "PROPOSED_FIX: restart service\nTARGET_HOST: web-01"}`)})
		},
	}

	finding := &Finding{
		ID:           "find-123",
		Title:        "Service Down",
		Severity:     "critical",
		ResourceName: "web-01",
	}
	mockFindings := &MockFindingsStore{
		Findings: map[string]*Finding{"find-123": finding},
	}

	config := DefaultConfig()
	orchestrator := NewOrchestrator(mockChat, store, mockFindings, nil, config)

	// Execute
	err := orchestrator.InvestigateFinding(context.Background(), finding, "controlled")

	// Verify
	require.NoError(t, err)

	// Check finding update
	assert.Equal(t, "completed", finding.InvestigationStatus)
	assert.Equal(t, "fix_queued", finding.InvestigationOutcome) // Queued because no executor + critical

	// Check stored investigation
	inv := orchestrator.GetInvestigationByFinding("find-123")
	require.NotNil(t, inv)
	assert.Equal(t, "completed", string(inv.Status))
	assert.Contains(t, inv.Summary, "Analysis: The issue is simple")

	// Check prompt contained context
	assert.Contains(t, mockChat.CapturedPrompt, "Service Down")
	assert.Contains(t, mockChat.CapturedPrompt, "web-01")
}

func TestInvestigateFinding_CannotFix(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	mockChat := &MockChatService{
		ExecuteFunc: func(cb StreamCallback) {
			cb(StreamEvent{Type: "content", Data: []byte(`{"text": "CANNOT_FIX: Too complex"}`)})
		},
	}

	finding := &Finding{ID: "find-456", Severity: "warning"}
	mockFindings := &MockFindingsStore{Findings: map[string]*Finding{"find-456": finding}}

	orchestrator := NewOrchestrator(mockChat, store, mockFindings, nil, DefaultConfig())

	err := orchestrator.InvestigateFinding(context.Background(), finding, "controlled")
	require.NoError(t, err)

	assert.Equal(t, "cannot_fix", finding.InvestigationOutcome)
}

func TestGetFixedCount(t *testing.T) {
	tempDir := t.TempDir()
	store := NewStore(tempDir)

	// Manually inject some completed investigations
	store.Create("f1", "s1")
	inv1 := store.GetLatestByFinding("f1")
	store.Complete(inv1.ID, "fix_executed", "Done", nil)

	store.Create("f2", "s2")
	inv2 := store.GetLatestByFinding("f2")
	store.Complete(inv2.ID, "cannot_fix", "Sorry", nil)

	orchestrator := NewOrchestrator(nil, store, nil, nil, DefaultConfig())

	count := orchestrator.GetFixedCount()
	assert.Equal(t, 1, count, "Should have 1 fixed investigation")
}
