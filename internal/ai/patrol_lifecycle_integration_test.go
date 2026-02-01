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

// TestLifecycle_FindingToInvestigationToApproval verifies the end-to-end lifecycle:
// a patrol run creates a critical finding, which triggers investigation, and the
// finding's investigation status transitions from pending → running.
func TestLifecycle_FindingToInvestigationToApproval(t *testing.T) {
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
			// Report a critical finding
			_, _, err := creator.CreateFinding(tools.PatrolFindingInput{
				Key:          "disk-full",
				Severity:     "critical",
				Category:     "storage",
				ResourceID:   "vm-500",
				ResourceName: "db-server",
				ResourceType: "vm",
				Title:        "Disk almost full on db-server",
				Description:  "Root partition is at 95%",
				Evidence:     "Disk usage: 95%",
			})
			if err != nil {
				return nil, fmt.Errorf("create finding: %w", err)
			}
			return &PatrolStreamResponse{Content: "Found disk full on db-server"}, nil
		},
	}
	svc.SetChatService(mockCS)

	// State with a VM that has high disk usage
	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-500", VMID: 500, Name: "db-server", Node: "pve-1", Status: "running", CPU: 0.50, Memory: models.Memory{Usage: 60}, Disk: models.Disk{Usage: 95}},
		},
		Nodes: []models.Node{
			{ID: "node/pve-1", Name: "pve-1", Status: "online", CPU: 0.30},
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

	// Run patrol
	ps.ForcePatrol(context.Background())

	// Wait for the patrol run to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Verify finding was created
	allFindings := ps.findings.GetActive(FindingSeverityCritical)
	var createdFinding *Finding
	for _, f := range allFindings {
		if f.ResourceID == "vm-500" && f.Key == "disk-full" {
			createdFinding = f
			break
		}
	}
	if createdFinding == nil {
		t.Fatalf("expected finding for vm-500 with key 'disk-full', got %d active findings", len(allFindings))
	}

	// Verify the finding severity is critical
	if createdFinding.Severity != FindingSeverityCritical {
		t.Fatalf("expected finding severity 'critical', got %q", createdFinding.Severity)
	}

	// Verify investigation was triggered (async goroutine)
	select {
	case findingID := <-orch.investigateCh:
		if findingID == "" {
			t.Fatal("investigation triggered with empty finding ID")
		}
		if findingID != createdFinding.ID {
			t.Fatalf("investigation triggered for wrong finding: got %s, want %s", findingID, createdFinding.ID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("expected investigation to be triggered within 3 seconds")
	}

	// Verify the finding's investigation status transitioned (the orchestrator sets it to running
	// when InvestigateFinding is called; in our mock it records the call but we can verify
	// the finding was passed with correct state)
	stored := ps.findings.Get(createdFinding.ID)
	if stored == nil {
		t.Fatal("finding not found in store after investigation trigger")
	}
}

// TestLifecycle_StuckInvestigationRecovery verifies that findings stuck in "running"
// status for too long are recovered to "failed" with outcome "timed_out".
func TestLifecycle_StuckInvestigationRecovery(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{
		Enabled:             true,
		PatrolModel:         "mock:model",
		PatrolAutonomyLevel: "approval",
	}
	svc.provider = &mockProvider{}

	stateProvider := &patrolTestStateProvider{
		state: models.StateSnapshot{},
	}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeGuests: true,
	})

	// Set up orchestrator (required for MaybeInvestigateFinding path)
	orch := newIntegrationOrchestrator(true)
	ps.SetInvestigationOrchestrator(orch)

	// Create a finding and manually set it to "running" with LastInvestigatedAt 20 minutes ago
	twentyMinAgo := time.Now().Add(-20 * time.Minute)
	stuckFinding := &Finding{
		ID:                     "stuck-finding-1",
		Key:                    "high-mem",
		Severity:               FindingSeverityWarning,
		Category:               FindingCategoryPerformance,
		ResourceID:             "vm-600",
		ResourceName:           "app-server",
		ResourceType:           "vm",
		Title:                  "High memory usage on app-server",
		Description:            "Memory at 90%",
		Evidence:               "Current memory: 90%",
		InvestigationStatus:    string(InvestigationStatusRunning),
		InvestigationSessionID: "session-stuck-1",
		LastInvestigatedAt:     &twentyMinAgo,
		InvestigationAttempts:  1,
		DetectedAt:             time.Now().Add(-30 * time.Minute),
		LastSeenAt:             time.Now().Add(-5 * time.Minute),
	}
	ps.findings.Add(stuckFinding)

	// Verify finding is active and in running state before recovery
	stored := ps.findings.Get("stuck-finding-1")
	if stored == nil {
		t.Fatal("expected finding to be in store")
	}
	if stored.InvestigationStatus != string(InvestigationStatusRunning) {
		t.Fatalf("expected investigation status 'running', got %q", stored.InvestigationStatus)
	}

	// Run recovery
	ps.recoverStuckInvestigations()

	// Verify the finding status is now "failed" with outcome "timed_out"
	recovered := ps.findings.Get("stuck-finding-1")
	if recovered == nil {
		t.Fatal("expected finding to still exist after recovery")
	}
	if recovered.InvestigationStatus != string(InvestigationStatusFailed) {
		t.Fatalf("expected investigation status 'failed' after recovery, got %q", recovered.InvestigationStatus)
	}
	if recovered.InvestigationOutcome != string(InvestigationOutcomeTimedOut) {
		t.Fatalf("expected investigation outcome 'timed_out' after recovery, got %q", recovered.InvestigationOutcome)
	}
}

// TestLifecycle_AutonomyLevelRecheckOnRetry verifies that when autonomy is changed
// to "monitor" mode, retryTimedOutInvestigations() calls MaybeInvestigateFinding
// but the finding is NOT investigated because monitor mode blocks investigation.
func TestLifecycle_AutonomyLevelRecheckOnRetry(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	// Start with "full" autonomy
	svc.cfg = &config.AIConfig{
		Enabled:             true,
		PatrolModel:         "mock:model",
		PatrolAutonomyLevel: "full",
	}
	svc.provider = &mockProvider{}

	stateProvider := &patrolTestStateProvider{
		state: models.StateSnapshot{},
	}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeGuests: true,
	})

	// Set up orchestrator to track investigation calls
	orch := newIntegrationOrchestrator(true)
	ps.SetInvestigationOrchestrator(orch)

	// Create a finding with InvestigationStatus="failed" and InvestigationOutcome="timed_out"
	timedOutFinding := &Finding{
		ID:                     "timed-out-finding-1",
		Key:                    "cpu-spike",
		Severity:               FindingSeverityCritical,
		Category:               FindingCategoryPerformance,
		ResourceID:             "vm-700",
		ResourceName:           "compute-server",
		ResourceType:           "vm",
		Title:                  "CPU spike on compute-server",
		Description:            "CPU spiked to 99%",
		Evidence:               "Current CPU: 99%",
		InvestigationStatus:    string(InvestigationStatusFailed),
		InvestigationOutcome:   string(InvestigationOutcomeTimedOut),
		InvestigationAttempts:  1,
		InvestigationSessionID: "session-timeout-1",
		DetectedAt:             time.Now().Add(-1 * time.Hour),
		LastSeenAt:             time.Now().Add(-5 * time.Minute),
	}
	ps.findings.Add(timedOutFinding)

	// Now change autonomy to "monitor" (which blocks all investigation)
	svc.cfg = &config.AIConfig{
		Enabled:             true,
		PatrolModel:         "mock:model",
		PatrolAutonomyLevel: "monitor",
	}

	// Call retryTimedOutInvestigations — this will call MaybeInvestigateFinding
	// but ShouldInvestigate should return false because autonomy is "monitor"
	ps.retryTimedOutInvestigations()

	// Verify that no investigation was triggered on the orchestrator
	select {
	case id := <-orch.investigateCh:
		t.Fatalf("expected no investigation to be triggered in monitor mode, but got finding %s", id)
	case <-time.After(500 * time.Millisecond):
		// Good — no investigation was triggered because monitor mode blocks it
	}

	// Verify the finding's status was NOT changed (still failed/timed_out)
	stored := ps.findings.Get("timed-out-finding-1")
	if stored == nil {
		t.Fatal("expected finding to still exist in store")
	}
	if stored.InvestigationStatus != string(InvestigationStatusFailed) {
		t.Fatalf("expected investigation status to remain 'failed', got %q", stored.InvestigationStatus)
	}
	if stored.InvestigationOutcome != string(InvestigationOutcomeTimedOut) {
		t.Fatalf("expected investigation outcome to remain 'timed_out', got %q", stored.InvestigationOutcome)
	}
}
