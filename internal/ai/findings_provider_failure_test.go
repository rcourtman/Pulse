package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"testing"
	"time"
)

func TestProviderFailureFollowupSurvivesRestart(t *testing.T) {
	for _, outcome := range []string{"", string(InvestigationOutcomeTimedOut)} {
		t.Run("outcome_"+outcome, func(t *testing.T) {
			persistence := config.NewConfigPersistence(t.TempDir())
			store := NewFindingsStore()
			if err := store.SetPersistence(NewFindingsPersistenceAdapter(persistence)); err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			store.Add(&Finding{ID: "failed-provider", ResourceID: "app-container:service", ResourceType: "app-container", Title: "Service unavailable", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, InvestigationSessionID: "failed-run", InvestigationStatus: string(InvestigationStatusFailed), InvestigationOutcome: outcome, LastInvestigatedAt: &now, InvestigationAttempts: 1})
			if err := store.ForceSave(); err != nil {
				t.Fatal(err)
			}
			restored := NewFindingsStore()
			if err := restored.SetPersistence(NewFindingsPersistenceAdapter(persistence)); err != nil {
				t.Fatal(err)
			}
			f := restored.Get("failed-provider")
			if !f.ShouldInvestigate("full") || !f.CanRetryInvestigation() {
				t.Fatal("persisted provider failure delayed the next scheduled model assessment")
			}
			f.InvestigationAttempts = maxInvestigationAttempts
			if f.ShouldInvestigate("full") {
				t.Fatal("bypassed attempt limit")
			}
			f.InvestigationAttempts = 1
			f.InvestigationStatus = string(InvestigationStatusRunning)
			if f.ShouldInvestigate("full") {
				t.Fatal("duplicated running investigation")
			}
			f.InvestigationStatus = string(InvestigationStatusCompleted)
			f.InvestigationOutcome = string(InvestigationOutcomeNeedsAttention)
			if f.ShouldInvestigate("full") {
				t.Fatal("overrode real human-attention conclusion")
			}
		})
	}
}
