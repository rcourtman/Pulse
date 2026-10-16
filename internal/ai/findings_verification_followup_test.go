package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestUnknownVerificationAllowsFollowupWithoutAnotherCooldown(t *testing.T) {
	completed := time.Now().Add(-15 * time.Minute)
	f := &Finding{ID: "uncertain", Severity: FindingSeverityWarning,
		InvestigationStatus:  string(InvestigationStatusCompleted),
		InvestigationOutcome: string(InvestigationOutcomeFixVerificationUnknown),
		LastInvestigatedAt:   &completed, InvestigationAttempts: 1}
	if !f.ShouldInvestigate("full") || !f.CanRetryInvestigation() {
		t.Fatal("an inconclusive action permanently suppressed follow-up")
	}
	recent := time.Now().Add(-5 * time.Minute)
	f.LastInvestigatedAt = &recent
	if !f.ShouldInvestigate("full") || !f.CanRetryInvestigation() {
		t.Fatal("uncertain verification delayed a fresh model assessment")
	}
	f.LastInvestigatedAt = &completed
	f.InvestigationStatus = string(InvestigationStatusRunning)
	if f.ShouldInvestigate("full") || f.CanRetryInvestigation() {
		t.Fatal("follow-up duplicated a running investigation")
	}
}

func TestUnknownVerificationFollowupSurvivesRestartAndWaitsForWatchHandoff(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	store := NewFindingsStore()
	if err := store.SetPersistence(NewFindingsPersistenceAdapter(persistence)); err != nil {
		t.Fatal(err)
	}
	completed := time.Now().Add(-time.Minute)
	store.Add(&Finding{ID: "uncertain", ResourceID: "app-container:service", ResourceType: "app-container",
		Title: "Service unavailable", Severity: FindingSeverityWarning, Category: FindingCategoryReliability,
		InvestigationSessionID: "original", InvestigationStatus: string(InvestigationStatusCompleted),
		InvestigationOutcome: string(InvestigationOutcomeFixVerificationUnknown), LastInvestigatedAt: &completed, InvestigationAttempts: 1})
	if err := store.ForceSave(); err != nil {
		t.Fatal(err)
	}
	restored := NewFindingsStore()
	if err := restored.SetPersistence(NewFindingsPersistenceAdapter(persistence)); err != nil {
		t.Fatal(err)
	}
	service := NewService(persistence, nil)
	service.cfg = &config.AIConfig{Enabled: true, PatrolAutonomyLevel: config.PatrolAutonomyApproval}
	patrol := NewPatrolService(service, nil)
	patrol.findings = restored
	orchestrator := newIntegrationOrchestrator(true)
	patrol.SetInvestigationOrchestrator(orchestrator)
	adapter := &patrolFindingCreatorAdapter{patrol: patrol}
	if err := adapter.AssessFinding(tools.PatrolFindingAssessmentInput{FindingID: "uncertain", Verdict: "present", Evidence: "Current HTTP health remains unavailable after the uncertain restart"}); err != nil {
		t.Fatal(err)
	}
	patrol.investigationWg.Wait()
	if len(orchestrator.investigateCh) != 0 {
		t.Fatal("investigation started while the Watch adapter still owned the executor")
	}
	current := restored.Get("uncertain")
	if current.InvestigationSessionID != "original" || current.InvestigationAttempts != 1 || current.ResolvedAt != nil {
		t.Fatalf("assessment discarded prior uncertainty: %#v", current)
	}
	patrol.dispatchPatrolInvestigations(&AIAnalysisResult{Findings: adapter.getCollectedFindings()})
	patrol.investigationWg.Wait()
	select {
	case id := <-orchestrator.investigateCh:
		if id != "uncertain" {
			t.Fatalf("follow-up finding=%s", id)
		}
	default:
		t.Fatal("fresh model assessment did not hand the unresolved finding to investigation")
	}
}
