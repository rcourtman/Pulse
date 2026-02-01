package ai

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// --- Integration test stubs ---

// integrationOrchestrator records investigation triggers without running real LLM calls.
type integrationOrchestrator struct {
	canStart      bool
	investigated  []string // finding IDs passed to InvestigateFinding
	investigateCh chan string
	runningCount  int32
}

func newIntegrationOrchestrator(canStart bool) *integrationOrchestrator {
	return &integrationOrchestrator{
		canStart:      canStart,
		investigateCh: make(chan string, 10),
	}
}

func (o *integrationOrchestrator) InvestigateFinding(_ context.Context, f *InvestigationFinding, _ string) error {
	o.investigated = append(o.investigated, f.ID)
	o.investigateCh <- f.ID
	return nil
}
func (o *integrationOrchestrator) GetInvestigationByFinding(_ string) *InvestigationSession {
	return nil
}
func (o *integrationOrchestrator) GetRunningCount() int {
	return int(atomic.LoadInt32(&o.runningCount))
}
func (o *integrationOrchestrator) GetFixedCount() int { return 0 }
func (o *integrationOrchestrator) CanStartInvestigation() bool {
	return o.canStart
}
func (o *integrationOrchestrator) ReinvestigateFinding(_ context.Context, _, _ string) error {
	return nil
}
func (o *integrationOrchestrator) Shutdown(_ context.Context) error {
	return nil
}

// --- Integration tests ---

// TestIntegration_PatrolRunCreatesFindings wires a real PatrolService with a mock
// chat service that reports a finding via patrol_report_finding. Verifies:
// - Finding is created in the FindingsStore
// - Investigation is triggered on the orchestrator
func TestIntegration_PatrolRunCreatesFindings(t *testing.T) {
	// Setup AI service with mock provider
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{
		Enabled:             true,
		PatrolModel:         "mock:model",
		PatrolAutonomyLevel: "approval",
	}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockCS := &patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}
			// Report a finding for a resource with high CPU
			_, _, err := creator.CreateFinding(tools.PatrolFindingInput{
				Key:          "high-cpu",
				Severity:     "warning",
				Category:     "performance",
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Title:        "High CPU usage on web-server",
				Description:  "CPU is at 92%",
				Evidence:     "Current CPU: 92%",
			})
			if err != nil {
				return nil, fmt.Errorf("create finding: %w", err)
			}
			return &PatrolStreamResponse{Content: "Found high CPU on web-server"}, nil
		},
	}
	svc.SetChatService(mockCS)

	// State with a VM at 92% CPU
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1", Status: "running", CPU: 0.92, Memory: models.Memory{Usage: 50}},
		},
		Nodes: []models.Node{
			{ID: "node/pve-1", Name: "pve-1", Status: "online", CPU: 0.40},
		},
	}
	stateProvider := &patrolTestStateProvider{state: state}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})

	// Set up orchestrator to record investigation triggers
	orch := newIntegrationOrchestrator(true)
	ps.SetInvestigationOrchestrator(orch)

	// Provide a mock config that returns "approval" autonomy level
	mockAICfg := &config.AIConfig{
		Enabled:             true,
		PatrolAutonomyLevel: "approval",
	}
	svc.cfg = mockAICfg

	// Run patrol
	ps.ForcePatrol(context.Background())

	// Wait for the patrol run to complete and investigation goroutine to fire
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Verify finding was created
	allFindings := ps.findings.GetActive(FindingSeverityInfo)
	found := false
	for _, f := range allFindings {
		if f.ResourceID == "vm-100" && f.Key == "high-cpu" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding for vm-100 with key 'high-cpu', got %d active findings", len(allFindings))
	}

	// Verify investigation was triggered (check with timeout due to async goroutine)
	select {
	case findingID := <-orch.investigateCh:
		// Verify it's the right finding
		if findingID == "" {
			t.Fatal("investigation triggered with empty finding ID")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("expected investigation to be triggered within 3 seconds")
	}
}

// TestIntegration_FindingRejectedByThreshold verifies that findings for resources
// with metrics below the actionability threshold are rejected.
func TestIntegration_FindingRejectedByThreshold(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		PatrolModel: "mock:model",
	}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	var rejectedErr error
	mockCS := &patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}
			// Try to report finding for a resource with LOW CPU (30%)
			_, _, err := creator.CreateFinding(tools.PatrolFindingInput{
				Key:          "high-cpu",
				Severity:     "warning",
				Category:     "performance",
				ResourceID:   "vm-200",
				ResourceName: "idle-server",
				ResourceType: "vm",
				Title:        "High CPU usage on idle-server",
				Description:  "CPU is elevated",
				Evidence:     "Current CPU: 30%",
			})
			rejectedErr = err
			return &PatrolStreamResponse{Content: "Analysis complete"}, nil
		},
	}
	svc.SetChatService(mockCS)

	// State with a VM at only 30% CPU
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-200", VMID: 200, Name: "idle-server", Node: "pve-1", Status: "running", CPU: 0.30, Memory: models.Memory{Usage: 30}},
		},
		Nodes: []models.Node{
			{ID: "node/pve-1", Name: "pve-1", Status: "online"},
		},
	}
	stateProvider := &patrolTestStateProvider{state: state}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		AnalyzeGuests: true,
		AnalyzeNodes:  true,
	})

	orch := newIntegrationOrchestrator(true)
	ps.SetInvestigationOrchestrator(orch)

	// Run patrol
	ps.ForcePatrol(context.Background())

	// Wait for completion
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The finding should have been rejected
	if rejectedErr == nil {
		t.Fatal("expected finding to be rejected (CPU at 30% is below 50% threshold)")
	}

	// No findings should be in the store for this resource
	findings := ps.GetFindingsForResource("vm-200")
	if len(findings) > 0 {
		t.Fatalf("expected no findings for vm-200, got %d", len(findings))
	}

	// No investigation should have been triggered
	select {
	case id := <-orch.investigateCh:
		t.Fatalf("expected no investigation to be triggered, but got finding %s", id)
	case <-time.After(500 * time.Millisecond):
		// Good - no investigation triggered
	}
}

// TestIntegration_StaleFindingReconciliation verifies that findings that are
// NOT re-reported by the LLM on a subsequent patrol run get auto-resolved.
func TestIntegration_StaleFindingReconciliation(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		PatrolModel: "mock:model",
	}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockCS := &patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			// Deliberately report NO findings — the LLM "doesn't see" the issue anymore
			// Also check existing findings to trigger the seeded-finding tracking
			creator := executor.GetPatrolFindingCreator()
			if creator != nil {
				_ = creator.GetActiveFindings("", "warning")
			}
			return &PatrolStreamResponse{Content: "All looks healthy"}, nil
		},
	}
	svc.SetChatService(mockCS)

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-300", VMID: 300, Name: "fixed-server", Node: "pve-1", Status: "running", CPU: 0.20, Memory: models.Memory{Usage: 30}},
		},
		Nodes: []models.Node{
			{ID: "node/pve-1", Name: "pve-1", Status: "online"},
		},
	}
	stateProvider := &patrolTestStateProvider{state: state}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		AnalyzeGuests: true,
		AnalyzeNodes:  true,
	})

	// Pre-seed a finding that was detected on a previous run
	preSeedFinding := &Finding{
		ID:           generateFindingID("vm-300", "performance", "high-cpu"),
		Key:          "high-cpu",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-300",
		ResourceName: "fixed-server",
		ResourceType: "vm",
		Title:        "High CPU usage",
		Description:  "Was high before",
		DetectedAt:   time.Now().Add(-2 * time.Hour),
		LastSeenAt:   time.Now().Add(-1 * time.Hour),
		TimesRaised:  3,
	}
	ps.findings.Add(preSeedFinding)

	// Verify finding is active before patrol
	active := ps.findings.GetActive(FindingSeverityWarning)
	if len(active) == 0 {
		t.Fatal("expected pre-seeded finding to be active before patrol")
	}

	// Run patrol (LLM doesn't re-report the finding)
	ps.ForcePatrol(context.Background())

	// Wait for completion
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The finding should have been auto-resolved by reconcileStaleFindings
	stored := ps.findings.Get(preSeedFinding.ID)
	if stored == nil {
		t.Fatal("expected finding to still exist in store (resolved, not deleted)")
	}
	if !stored.IsResolved() {
		t.Fatal("expected pre-seeded finding to be auto-resolved after patrol didn't re-report it")
	}
}

// TestIntegration_GracefulShutdownStopsClealy verifies that Stop() on a
// PatrolService returns cleanly and doesn't panic even with an orchestrator set.
func TestIntegration_GracefulShutdownStopsCleanly(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	orch := newIntegrationOrchestrator(true)
	ps.SetInvestigationOrchestrator(orch)

	// Start and immediately stop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps.SetConfig(PatrolConfig{Enabled: true, Interval: 1 * time.Hour})
	ps.Start(ctx)

	// Give it a moment to enter the patrol loop
	time.Sleep(100 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		ps.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good - shutdown completed
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not complete within 5 seconds")
	}
}
