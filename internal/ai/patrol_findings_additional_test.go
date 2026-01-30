package ai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type patrolTestStateProvider struct {
	state models.StateSnapshot
}

func (p *patrolTestStateProvider) GetState() models.StateSnapshot {
	return p.state
}

// patrolMockChatService implements ChatServiceProvider and chatServiceExecutorAccessor for testing.
type patrolMockChatService struct {
	executor                *tools.PulseToolExecutor
	executePatrolStreamFunc func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error)
}

func (m *patrolMockChatService) CreateSession(ctx context.Context) (*ChatSession, error) {
	return &ChatSession{ID: "mock-session"}, nil
}

func (m *patrolMockChatService) ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error {
	return nil
}

func (m *patrolMockChatService) ExecutePatrolStream(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
	if m.executePatrolStreamFunc != nil {
		return m.executePatrolStreamFunc(ctx, req, callback)
	}
	return &PatrolStreamResponse{}, nil
}

func (m *patrolMockChatService) GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error) {
	return nil, nil
}

func (m *patrolMockChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *patrolMockChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

func (m *patrolMockChatService) GetExecutor() *tools.PulseToolExecutor {
	return m.executor
}

func TestPatrolService_DismissFinding(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	f := &Finding{
		ID:           "f1",
		Key:          "cpu-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		ResourceType: "node",
		Title:        "High CPU",
	}
	ps.findings.Add(f)

	if err := ps.DismissFinding("f1", "not_an_issue", "expected during maintenance"); err != nil {
		t.Fatalf("dismiss finding: %v", err)
	}
	stored := ps.findings.Get("f1")
	if stored == nil || stored.DismissedReason != "not_an_issue" {
		t.Fatalf("expected finding to be dismissed, got %+v", stored)
	}
	if stored.UserNote != "expected during maintenance" {
		t.Fatalf("expected dismissal note to be preserved, got %q", stored.UserNote)
	}

	if err := ps.DismissFinding("f1", "bad_reason", ""); err == nil {
		t.Fatal("expected error for invalid dismissal reason")
	}
}

func TestPatrolFindingCreatorAdapter_IsActionable(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	lowState := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-low",
				Name:   "node-low",
				Status: "online",
				CPU:    0.20,
				Memory: models.Memory{Usage: 30.0},
			},
		},
	}
	lowAdapter := newPatrolFindingCreatorAdapter(ps, lowState)
	_, _, err := lowAdapter.CreateFinding(tools.PatrolFindingInput{
		Key:          "cpu-high",
		Severity:     "warning",
		Category:     "performance",
		ResourceID:   "node-low",
		ResourceName: "node-low",
		ResourceType: "node",
		Title:        "High CPU",
		Description:  "CPU usage is high",
		Evidence:     "CPU > 90%",
	})
	if err == nil {
		t.Fatal("expected low CPU warning finding to be rejected")
	}

	highState := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-high",
				Name:   "node-high",
				Status: "online",
				CPU:    0.90,
				Memory: models.Memory{Usage: 80.0},
			},
		},
	}
	highAdapter := newPatrolFindingCreatorAdapter(ps, highState)
	_, _, err = highAdapter.CreateFinding(tools.PatrolFindingInput{
		Key:          "cpu-high",
		Severity:     "warning",
		Category:     "performance",
		ResourceID:   "node-high",
		ResourceName: "node-high",
		ResourceType: "node",
		Title:        "High CPU",
		Description:  "CPU usage is high",
		Evidence:     "CPU > 90%",
	})
	if err != nil {
		t.Fatalf("expected high CPU warning finding to be accepted, got %v", err)
	}
}

func TestPatrolService_ForcePatrol_RecordsRun(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockCS := &patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}
			_, _, _ = creator.CreateFinding(tools.PatrolFindingInput{
				Severity:     "warning",
				Category:     "performance",
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Title:        "High CPU",
				Description:  "CPU usage is high",
				Evidence:     "CPU > 90%",
			})
			return &PatrolStreamResponse{Content: "Analysis complete"}, nil
		},
	}
	svc.SetChatService(mockCS)

	stateProvider := &patrolTestStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1", Status: "running", CPU: 0.95},
			},
			Nodes: []models.Node{
				{ID: "node-1", Name: "pve-1", Status: "online"},
			},
		},
	}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})

	before := ps.runHistoryStore.Count()
	ps.ForcePatrol(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected ForcePatrol to record a run (count stayed at %d)", before)
}
