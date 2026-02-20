package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type mockPatrolProvider struct {
	response string
	err      error
}

func (m mockPatrolProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &providers.ChatResponse{Content: m.response}, nil
}

func (m mockPatrolProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	return nil
}

func (m mockPatrolProvider) SupportsThinking(model string) bool       { return false }
func (m mockPatrolProvider) TestConnection(ctx context.Context) error { return nil }
func (m mockPatrolProvider) Name() string                             { return "mock" }
func (m mockPatrolProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

type mockPatrolStateProvider struct {
	state models.StateSnapshot
}

func (m mockPatrolStateProvider) GetState() models.StateSnapshot { return m.state }

func TestPatrolService_AskAIAboutAlert(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	aiSvc := &Service{
		provider: mockPatrolProvider{response: "RESOLVE: looks good"},
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	alert := AlertInfo{
		ID:           "a1",
		Type:         "cpu",
		ResourceName: "node1",
		ResourceType: "node",
		Message:      "high cpu",
		Value:        90,
		Threshold:    80,
		Duration:     "5m",
	}
	state := models.StateSnapshot{Nodes: []models.Node{{ID: "node1", Name: "node1", CPU: 10}}}

	resolved, reason := ps.askAIAboutAlert(context.Background(), alert, state, aiSvc)
	if !resolved {
		t.Fatalf("expected alert to resolve")
	}
	if reason == "" {
		t.Fatalf("expected resolution reason")
	}
}

func TestPatrolService_GetResourceCurrentState(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "s1", Name: "store1", Usage: 75.5, Status: "ok"}},
	}
	alert := AlertInfo{ResourceType: "Storage", ResourceID: "s1", ResourceName: "store1"}
	if got := ps.getResourceCurrentState(alert, state); got == "" {
		t.Fatalf("expected storage state")
	}

	alert = AlertInfo{ResourceType: "unknown"}
	if got := ps.getResourceCurrentState(alert, state); got == "" {
		t.Fatalf("expected fallback for unknown resource")
	}
}

func TestPatrolService_TriggerPatrolForAlert(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.adHocTrigger = make(chan *alerts.Alert, 1)
	ps.running = true // simulate started patrol loop

	ps.TriggerPatrolForAlert(nil) // should no-op

	alert := &alerts.Alert{ID: "a1", Type: "cpu", ResourceID: "node1"}
	ps.TriggerPatrolForAlert(alert)
	select {
	case got := <-ps.adHocTrigger:
		if got.ID != alert.ID {
			t.Fatalf("expected alert to be queued")
		}
	default:
		t.Fatalf("expected alert to be queued")
	}

	// Trigger manager path
	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 1})
	ps.SetTriggerManager(tm)
	ps.TriggerPatrolForAlert(alert)
	if tm.GetPendingCount() != 1 {
		t.Fatalf("expected trigger manager to queue alert")
	}
}

func TestPatrolService_RunTargetedPatrol_Disabled(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config.Enabled = false

	alert := &alerts.Alert{ID: "a2", Type: "cpu", ResourceID: "node1"}
	ps.runTargetedPatrol(context.Background(), alert)
}

func TestPatrolService_RunScopedPatrol_EarlyPaths(t *testing.T) {
	ps := NewPatrolService(nil, mockPatrolStateProvider{state: models.StateSnapshot{}})
	ps.config.Enabled = false
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node1"}})

	// Re-queue when run already in progress
	ps.config.Enabled = true
	ps.runInProgress = true
	ps.runStartedAt = time.Now()
	ps.SetTriggerManager(NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 2}))
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node1"}})
	if ps.triggerManager.GetPendingCount() != 1 {
		t.Fatalf("expected scoped patrol to be re-queued")
	}

	// Drop permanently when retries exhausted
	ps.runInProgress = true
	ps.runStartedAt = time.Now()
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node1"}, RetryCount: scopedPatrolMaxRetries})

	// No resources matched
	ps.runInProgress = false
	ps.stateProvider = mockPatrolStateProvider{state: models.StateSnapshot{}}
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node1"}})

	// AI unavailable path
	ps.runInProgress = false
	ps.stateProvider = mockPatrolStateProvider{state: models.StateSnapshot{Nodes: []models.Node{{ID: "node1", Name: "node1"}}}}
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node1"}, ResourceTypes: []string{"node"}})
	status := ps.GetStatus()
	if status.BlockedReason == "" {
		t.Fatalf("expected blocked reason when AI unavailable")
	}
}
