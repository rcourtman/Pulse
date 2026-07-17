package ai

import (
	"context"
	"fmt"
	"strings"
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
	lastReq  providers.ChatRequest
	calls    int
}

func (m *mockPatrolProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	m.lastReq = req
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &providers.ChatResponse{Content: m.response}, nil
}

func (m *mockPatrolProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	return nil
}

func (m *mockPatrolProvider) SupportsThinking(model string) bool       { return false }
func (m *mockPatrolProvider) TestConnection(ctx context.Context) error { return nil }
func (m *mockPatrolProvider) Name() string                             { return "mock" }
func (m *mockPatrolProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

type mockPatrolStateProvider struct {
	state models.StateSnapshot
}

func (m mockPatrolStateProvider) ReadSnapshot() models.StateSnapshot { return m.state }

func TestPatrolService_ReviewAlertBatch(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	provider := &mockPatrolProvider{response: "ALERT 1: RESOLVE: cpu back to normal\nALERT 2: KEEP: still high"}
	aiSvc := &Service{
		provider: provider,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	alertsToReview := []AlertInfo{
		{
			ID:           "a1",
			Type:         "cpu",
			ResourceName: "node1",
			ResourceType: "node",
			Message:      "high cpu",
			Value:        90,
			Threshold:    80,
			Duration:     "5m",
		},
		{
			ID:           "a-usage",
			Type:         "usage",
			ResourceName: "local",
			ResourceType: "usage",
			ResourceID:   "storage-1",
			Message:      "high usage",
			Value:        92,
			Threshold:    90,
			Duration:     "20m",
		},
	}
	state := models.StateSnapshot{
		Nodes:   []models.Node{{ID: "node1", Name: "node1", CPU: 10}},
		Storage: []models.Storage{{ID: "storage-1", Name: "local", Usage: 91, Status: "active"}},
	}

	verdicts := ps.reviewAlertsWithModelState(context.Background(), alertsToReview, patrolRuntimeStateForTest(ps, state), aiSvc, "patrol-run-123")
	if provider.calls != 1 {
		t.Fatalf("expected one batched model call for two alerts, got %d", provider.calls)
	}
	if len(verdicts) != 2 {
		t.Fatalf("expected one verdict per alert, got %d", len(verdicts))
	}
	if !verdicts[0].resolve {
		t.Fatalf("expected first alert to resolve")
	}
	if !strings.Contains(verdicts[0].reason, "cpu back to normal") {
		t.Fatalf("expected resolution reason, got %q", verdicts[0].reason)
	}
	if verdicts[1].resolve {
		t.Fatalf("expected second alert to stay unresolved")
	}
	prompt := provider.lastReq.Messages[1].Content
	if !strings.Contains(prompt, "Resource: node1 (node)") {
		t.Fatalf("expected prompt to include canonical node resource type, got %q", prompt)
	}
	if !strings.Contains(prompt, "Resource: local (storage)") {
		t.Fatalf("expected prompt to normalize usage alerts to storage, got %q", prompt)
	}
	if provider.lastReq.ExecutionID != "patrol-run-123" {
		t.Fatalf("execution_id=%q want patrol-run-123", provider.lastReq.ExecutionID)
	}
}

func TestPatrolService_ReviewAlertBatch_SingleAlertBareResolveFallback(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	provider := &mockPatrolProvider{response: "RESOLVE: looks good"}
	aiSvc := &Service{
		provider: provider,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	alert := AlertInfo{
		ID:           "a1",
		Type:         "cpu",
		ResourceName: "node1",
		ResourceType: "node",
		Value:        90,
		Threshold:    80,
	}
	state := models.StateSnapshot{Nodes: []models.Node{{ID: "node1", Name: "node1", CPU: 10}}}

	verdicts := ps.reviewAlertsWithModelState(context.Background(), []AlertInfo{alert}, patrolRuntimeStateForTest(ps, state), aiSvc, "patrol-run-bare")
	if len(verdicts) != 1 || !verdicts[0].resolve {
		t.Fatalf("expected bare RESOLVE response to resolve the single alert, got %+v", verdicts)
	}
	if !strings.Contains(verdicts[0].reason, "looks good") {
		t.Fatalf("expected resolution reason, got %q", verdicts[0].reason)
	}
}

func TestPatrolService_ReviewAlertBatch_UnparseableKeepsAll(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	provider := &mockPatrolProvider{response: "Everything appears to be operating within normal parameters."}
	aiSvc := &Service{
		provider: provider,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	alertsToReview := []AlertInfo{
		{ID: "a1", Type: "cpu", ResourceName: "node1", ResourceType: "node"},
		{ID: "a2", Type: "memory", ResourceName: "node1", ResourceType: "node"},
	}
	state := models.StateSnapshot{Nodes: []models.Node{{ID: "node1", Name: "node1"}}}

	verdicts := ps.reviewAlertsWithModelState(context.Background(), alertsToReview, patrolRuntimeStateForTest(ps, state), aiSvc, "")
	for i, v := range verdicts {
		if v.resolve {
			t.Fatalf("expected unparseable response to keep alert %d, got %+v", i, v)
		}
	}
}

func TestPatrolService_ReviewAlertBatch_ChunksLargeBacklogs(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	provider := &mockPatrolProvider{response: "ALERT 1: KEEP: still investigating"}
	aiSvc := &Service{
		provider: provider,
		cfg:      &config.AIConfig{PatrolModel: "mock:model"},
	}

	alertsToReview := make([]AlertInfo, patrolAlertReviewBatchSize+1)
	for i := range alertsToReview {
		alertsToReview[i] = AlertInfo{ID: fmt.Sprintf("a%d", i), Type: "cpu", ResourceName: "node1", ResourceType: "node"}
	}
	state := models.StateSnapshot{Nodes: []models.Node{{ID: "node1", Name: "node1"}}}

	verdicts := ps.reviewAlertsWithModelState(context.Background(), alertsToReview, patrolRuntimeStateForTest(ps, state), aiSvc, "")
	if provider.calls != 2 {
		t.Fatalf("expected %d alerts to review in 2 chunked calls, got %d calls", len(alertsToReview), provider.calls)
	}
	if len(verdicts) != len(alertsToReview) {
		t.Fatalf("expected %d verdicts, got %d", len(alertsToReview), len(verdicts))
	}
}

func TestApplyAlertReviewResponse_FirstLinePerAlertWins(t *testing.T) {
	verdicts := []patrolAlertReviewVerdict{{alert: AlertInfo{ID: "a1"}}}
	matched := applyAlertReviewResponse(verdicts, "ALERT 1: RESOLVE: fixed\nALERT 1: KEEP: on second thought\nALERT 9: RESOLVE: out of range")
	if !matched {
		t.Fatalf("expected verdict lines to match")
	}
	if !verdicts[0].resolve {
		t.Fatalf("expected first verdict line to win, got %+v", verdicts[0])
	}
}

func TestPatrolService_GetResourceCurrentState(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Storage: []models.Storage{{ID: "s1", Name: "store1", Usage: 75.5, Status: "ok"}},
	}
	alert := AlertInfo{ResourceType: "storage", ResourceID: "s1", ResourceName: "store1"}
	if got := ps.getResourceCurrentStateState(alert, patrolRuntimeStateForTest(ps, state)); got == "" {
		t.Fatalf("expected storage state")
	}

	alert = AlertInfo{ResourceType: "unknown"}
	if got := ps.getResourceCurrentStateState(alert, patrolRuntimeStateForTest(ps, state)); got == "" {
		t.Fatalf("expected fallback for unknown resource")
	}
}

func TestPatrolService_TriggerPatrolForAlert(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.adHocTrigger = make(chan *alerts.Alert, 1)

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

func TestPatrolService_TriggerPatrolForAlert_RespectsAlertTriggerConfig(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.SetEventTriggerConfig(PatrolEventTriggerConfig{
		AlertTriggersEnabled:   false,
		AnomalyTriggersEnabled: true,
	})

	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 1})
	ps.SetTriggerManager(tm)

	ps.TriggerPatrolForAlert(&alerts.Alert{ID: "a2", Type: "cpu", ResourceID: "node1"})
	if tm.GetPendingCount() != 0 {
		t.Fatalf("expected alert-driven patrol to be skipped when alert trigger source is disabled")
	}
}

func TestPatrolService_TriggerPatrolForAlert_RespectsRuntimeBlock(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.SetEventTriggerConfig(PatrolEventTriggerConfig{
		AlertTriggersEnabled:   true,
		AnomalyTriggersEnabled: true,
	})
	ps.SetEventTriggerBlock(BackgroundAutomationEventTriggerBlock())

	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 1})
	ps.SetTriggerManager(tm)

	ps.TriggerPatrolForAlert(&alerts.Alert{ID: "a3", Type: "cpu", ResourceID: "node1"})
	if tm.GetPendingCount() != 0 {
		t.Fatalf("expected alert-driven patrol to be skipped when event triggers are blocked by runtime policy")
	}

	status := tm.GetStatus()
	if !status.AlertTriggersEnabled || !status.AnomalyTriggersEnabled {
		t.Fatal("expected runtime block not to rewrite configured trigger preferences")
	}
	if !status.EventTriggersBlocked {
		t.Fatal("expected trigger manager status to expose event trigger runtime block")
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
	if status.BlockedReason != patrolProviderNotConfiguredReason {
		t.Fatalf("blocked reason = %q, want %q", status.BlockedReason, patrolProviderNotConfiguredReason)
	}
}
