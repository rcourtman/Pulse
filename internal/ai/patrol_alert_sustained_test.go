package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAlertExaminedSince(t *testing.T) {
	ps := NewPatrolService(NewService(config.NewConfigPersistence(t.TempDir()), nil), &patrolTestStateProvider{})
	episodeStart := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

	if ps.AlertExaminedSince("disk-alert", episodeStart) {
		t.Fatal("no run has happened yet")
	}

	// None of these count: an earlier episode, another alert, a failed run and
	// the check that verified an earlier clear.
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "earlier", AlertIdentifier: "disk-alert", StartedAt: episodeStart.Add(-time.Hour), Status: "healthy", TriggerReason: string(TriggerReasonAlertFired)})
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "other", AlertIdentifier: "cpu-alert", StartedAt: episodeStart.Add(time.Minute), Status: "healthy", TriggerReason: string(TriggerReasonAlertFired)})
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "failed", AlertIdentifier: "disk-alert", StartedAt: episodeStart.Add(time.Minute), Status: "error", TriggerReason: string(TriggerReasonAlertFired)})
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "cleared", AlertIdentifier: "disk-alert", StartedAt: episodeStart.Add(time.Minute), Status: "healthy", TriggerReason: string(TriggerReasonAlertCleared)})
	if ps.AlertExaminedSince("disk-alert", episodeStart) {
		t.Fatal("no run has looked at this episode yet")
	}

	ps.runHistoryStore.Add(PatrolRunRecord{ID: "looked", AlertIdentifier: "disk-alert", StartedAt: episodeStart.Add(12 * time.Minute), Status: "issues_found", TriggerReason: string(TriggerReasonAlertFired)})
	if !ps.AlertExaminedSince("disk-alert", episodeStart) {
		t.Fatal("a completed run named this alert after the episode began")
	}
	if ps.AlertExaminedSince("", episodeStart) {
		t.Fatal("an empty identifier matches nothing")
	}
}

// A run for an alert that has stayed active is asked for what the alert does
// not say, and told not to repeat it. A run for an alert that just fired keeps
// its existing instruction.
func TestSeedFindingsAndContext_SustainedAlertAsksForWhatTheAlertLeavesOut(t *testing.T) {
	ps := NewPatrolService(NewService(config.NewConfigPersistence(t.TempDir()), nil), &patrolTestStateProvider{})
	state := newPatrolRuntimeState(models.StateSnapshot{})
	alertContext := &PatrolAlertContext{ResourceName: "minipc (/data)", AlertType: "disk", Level: "warning", Value: 93, Threshold: 90}

	stayed, _ := ps.seedFindingsAndContextState(&PatrolScope{Reason: TriggerReasonAlertFired, AlertContext: alertContext, AlertSustained: true}, state)
	for _, want := range []string{"which has stayed active", "Do not report a finding that only repeats the alert", "what is using the space"} {
		if !strings.Contains(stayed, want) {
			t.Fatalf("sustained note is missing %q:\n%s", want, stayed)
		}
	}
	if strings.Contains(stayed, "Reporting a finding for this breach is expected") {
		t.Fatalf("sustained run was told a finding is expected:\n%s", stayed)
	}

	fresh, _ := ps.seedFindingsAndContextState(&PatrolScope{Reason: TriggerReasonAlertFired, AlertContext: alertContext}, state)
	if !strings.Contains(fresh, "Reporting a finding for this breach is expected") || strings.Contains(fresh, "which has stayed active") {
		t.Fatalf("a freshly fired alert must keep its instruction:\n%s", fresh)
	}
}
