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

// mockChatService implements ChatServiceProvider and chatServiceExecutorAccessor for testing
type mockChatService struct {
	executor                *tools.PulseToolExecutor
	executePatrolStreamFunc func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error)
}

func (m *mockChatService) CreateSession(ctx context.Context) (*ChatSession, error) {
	return &ChatSession{ID: "mock-session"}, nil
}

func (m *mockChatService) ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error {
	return nil
}

func (m *mockChatService) ExecutePatrolStream(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
	if m.executePatrolStreamFunc != nil {
		return m.executePatrolStreamFunc(ctx, req, callback)
	}
	return &PatrolStreamResponse{}, nil
}

func (m *mockChatService) GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error) {
	return nil, nil
}

func (m *mockChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

func (m *mockChatService) GetExecutor() *tools.PulseToolExecutor {
	return m.executor
}

// TestPatrolService_RunPatrol_FullCoverage tests the main patrol loop including findings generation
func TestPatrolService_RunPatrol_FullCoverage(t *testing.T) {
	// Setup dependencies
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}

	// Create real executor with minimal config
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	svc.provider = &mockProvider{} // Satisfy IsEnabled()

	// Setup mock chat service
	mockCS := &mockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			// Simulate tool use
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}

			// Create a finding via the creator adapter directly
			// This simulates what the tool handler would do
			input := tools.PatrolFindingInput{
				Severity:     "warning",
				Category:     "performance",
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Title:        "High CPU",
				Description:  "CPU usage is high",
				Evidence:     "CPU > 90%",
			}

			_, _, err := creator.CreateFinding(input)
			if err != nil {
				return nil, err
			}

			return &PatrolStreamResponse{
				Content: "Analysis complete. Found 1 issue.",
			}, nil
		},
	}
	svc.SetChatService(mockCS)

	// Mock state provider
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1", Status: "running", CPU: 0.95},
			},
			Nodes: []models.Node{
				{ID: "node-1", Name: "pve-1", Status: "online"},
			},
		},
	}

	// Create patrol service with mocked dependencies
	ps := NewPatrolService(svc, stateProvider)

	// Configure patrol to ensure it runs
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      100 * time.Millisecond,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})

	// Set proactive mode false to ensure exact threshold matching if any
	ps.SetProactiveMode(false)

	// Run patrol manually
	ctx := context.Background()
	ps.runPatrol(ctx)

	// Check findings
	findings := ps.GetFindings().GetActive(FindingSeverityInfo)
	if len(findings) == 0 {
		t.Error("Expected findings from patrol run, got 0")
	} else {
		found := false
		for _, f := range findings {
			if f.Title == "High CPU" {
				found = true
				if f.ResourceID != "vm-100" {
					t.Errorf("Expected resource ID vm-100, got %s", f.ResourceID)
				}
			}
		}
		if !found {
			t.Error("Expected finding title 'High CPU' not found")
		}
	}

	// Check status
	status := ps.GetStatus()
	if status.FindingsCount != 1 {
		t.Errorf("Expected 1 finding in status, got %d", status.FindingsCount)
	}
}

// TestPatrolService_StartStop verifies the startup and shutdown sequence covering patrolLoop
func TestPatrolService_StartStop(t *testing.T) {
	svc := NewService(nil, nil)
	stateProvider := &mockStateProvider{}
	ps := NewPatrolService(svc, stateProvider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	ps.Start(ctx) // Idempotency check
	ps.Stop()
	ps.Stop() // Idempotency check
}

// TestPatrolService_Setters_Coverage tests setter methods not covered in existing tests
func TestPatrolService_Setters_Coverage(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Test Start/Stop with config change
	ps.Start(context.Background())
	time.Sleep(10 * time.Millisecond)

	newCfg := PatrolConfig{
		Enabled:  true,
		Interval: 5 * time.Minute,
	}
	ps.SetConfig(newCfg)
	time.Sleep(10 * time.Millisecond)
	ps.Stop()

	// Verify SetIncidentStore getter/setter
	ps.SetIncidentStore(nil)
	if ps.GetIncidentStore() != nil {
		t.Error("Expected nil incident store")
	}
}

// TestPatrol_RunPatrol_AIRequired verifies specific behaviors when AI is/isn't available
func TestPatrol_RunPatrol_AIRequired(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node-high-cpu", Name: "node1", Status: "online", CPU: 95.0, Memory: models.Memory{Usage: 80.0}},
			},
		},
	}

	ps := NewPatrolService(nil, stateProvider)

	// Create service without ChatService
	svc := NewService(nil, nil)
	svc.cfg = &config.AIConfig{Enabled: true}
	// No ChatService set means GetChatService() returns nil
	svc.provider = &mockProvider{} // Satisfy IsEnabled()

	ps.aiService = svc

	// Run patrol
	ctx := context.Background()
	ps.runPatrol(ctx) // Should log error and return early

	// Check status - should record error
	status := ps.GetStatus()
	if status.ErrorCount == 0 && status.Healthy {
		t.Error("Expected error count > 0 or unhealthy status when chat service unavailable")
	}
}
