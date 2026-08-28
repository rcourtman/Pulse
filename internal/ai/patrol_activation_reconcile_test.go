package ai

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestReconcileActionableFindingBacklogIsBoundedAndPrioritizesCriticalFindings(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	service := NewService(persistence, nil)
	service.cfg = &config.AIConfig{Enabled: true, PatrolAutonomyLevel: config.PatrolAutonomyApproval}
	patrol := NewPatrolService(service, nil)
	orchestrator := newIntegrationOrchestrator(true)
	patrol.SetInvestigationOrchestrator(orchestrator)

	now := time.Now().UTC()
	for _, finding := range []*Finding{
		{ID: "warning-old", Severity: FindingSeverityWarning, ResourceID: "vm-1", DetectedAt: now.Add(-5 * time.Hour)},
		{ID: "critical-new", Severity: FindingSeverityCritical, ResourceID: "vm-2", DetectedAt: now.Add(-time.Hour)},
		{ID: "warning-new", Severity: FindingSeverityWarning, ResourceID: "vm-3", DetectedAt: now.Add(-30 * time.Minute)},
		{ID: "critical-old", Severity: FindingSeverityCritical, ResourceID: "vm-4", DetectedAt: now.Add(-4 * time.Hour)},
		{ID: "warning-middle", Severity: FindingSeverityWarning, ResourceID: "vm-5", DetectedAt: now.Add(-2 * time.Hour)},
		{ID: "info", Severity: FindingSeverityInfo, ResourceID: "vm-6", DetectedAt: now.Add(-6 * time.Hour)},
	} {
		patrol.findings.Add(finding)
	}

	if got := patrol.ReconcileActionableFindingBacklog(); got != maxPatrolActivationBacklogInvestigations {
		t.Fatalf("ReconcileActionableFindingBacklog() = %d, want %d", got, maxPatrolActivationBacklogInvestigations)
	}
	patrol.investigationWg.Wait()

	started := map[string]bool{}
	for len(orchestrator.investigateCh) > 0 {
		started[<-orchestrator.investigateCh] = true
	}
	for _, id := range []string{"critical-old", "critical-new", "warning-old"} {
		if !started[id] {
			t.Errorf("expected prioritized finding %q to start, got %v", id, started)
		}
	}
	for _, id := range []string{"warning-middle", "warning-new", "info"} {
		if started[id] {
			t.Errorf("unexpected backlog finding %q started, got %v", id, started)
		}
	}
}

func TestReconcileActionableFindingBacklogDoesNothingInWatchOnly(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	service := NewService(persistence, nil)
	service.cfg = &config.AIConfig{Enabled: true, PatrolAutonomyLevel: config.PatrolAutonomyMonitor}
	patrol := NewPatrolService(service, nil)
	orchestrator := newIntegrationOrchestrator(true)
	patrol.SetInvestigationOrchestrator(orchestrator)
	patrol.findings.Add(&Finding{
		ID: "warning", Severity: FindingSeverityWarning, ResourceID: "vm-1", DetectedAt: time.Now().UTC(),
	})

	if got := patrol.ReconcileActionableFindingBacklog(); got != 0 {
		t.Fatalf("ReconcileActionableFindingBacklog() = %d, want 0", got)
	}
	select {
	case id := <-orchestrator.investigateCh:
		t.Fatalf("Watch Only started investigation %q", id)
	default:
	}
}

func TestReconcileActionableFindingBacklogUsesRemainingAdmissionCapacity(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	service := NewService(persistence, nil)
	service.cfg = &config.AIConfig{Enabled: true, PatrolAutonomyLevel: config.PatrolAutonomyApproval}
	patrol := NewPatrolService(service, nil)
	orchestrator := newIntegrationOrchestrator(true)
	atomic.StoreInt32(&orchestrator.runningCount, maxPatrolActivationBacklogInvestigations-1)
	patrol.SetInvestigationOrchestrator(orchestrator)

	for _, id := range []string{"warning-1", "warning-2", "warning-3"} {
		patrol.findings.Add(&Finding{
			ID: id, Severity: FindingSeverityWarning, ResourceID: "vm-1", DetectedAt: time.Now().UTC(),
		})
	}

	if got := patrol.ReconcileActionableFindingBacklog(); got != 1 {
		t.Fatalf("ReconcileActionableFindingBacklog() = %d, want 1 remaining slot", got)
	}
	patrol.investigationWg.Wait()
	if got := len(orchestrator.investigateCh); got != 1 {
		t.Fatalf("started investigations = %d, want 1", got)
	}
}
