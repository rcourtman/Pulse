package ai

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func TestFindingInvestigationReconciliationUsesCurrentOccurrence(t *testing.T) {
	boundary := time.Now().Add(-time.Minute)
	for _, tc := range []struct {
		name, currentSession, resultSession string
		startedAt                           time.Time
		accepted                            bool
	}{
		{"old result before new investigation", "", "old", boundary.Add(-time.Minute), false},
		{"missing occurrence time", "", "old", time.Time{}, false},
		{"old result after new investigation", "current", "old", boundary.Add(time.Second), false},
		{"stale persisted session reference", "old", "old", boundary.Add(-time.Minute), false},
		{"current result", "current", "current", boundary.Add(time.Second), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, outcome := range []InvestigationOutcome{InvestigationOutcomeFixQueued, InvestigationOutcomeFixVerified,
				InvestigationOutcomeFixRejected, InvestigationOutcomeNeedsAttention, InvestigationOutcomeFixVerificationUnknown} {
				store := NewFindingsStore()
				store.Add(&Finding{ID: "failure", ResourceID: "resource", Severity: FindingSeverityWarning,
					LastRegressionAt: &boundary, RegressionCount: 1, InvestigationSessionID: tc.currentSession})
				before := store.Get("failure")
				session := &aicontracts.InvestigationSession{ID: "investigation", FindingID: "failure", SessionID: tc.resultSession, StartedAt: tc.startedAt}
				applied, changed := store.ReconcileInvestigationOutcome("failure", session, string(outcome))
				if applied != tc.accepted || changed != tc.accepted {
					t.Fatalf("outcome=%s applied=%v changed=%v, want %v", outcome, applied, changed, tc.accepted)
				}
				if !tc.accepted && !reflect.DeepEqual(before, store.Get("failure")) {
					t.Fatal("historical result changed current finding")
				}
				record := &aicontracts.InvestigationRecord{ID: session.ID, FindingID: session.FindingID,
					SessionID: session.SessionID, StartedAt: session.StartedAt, Outcome: aicontracts.InvestigationOutcome(outcome)}
				if got := store.UpdateInvestigationRecord("failure", record); got != tc.accepted {
					t.Fatalf("record accepted=%v, want %v", got, tc.accepted)
				}
			}
		})
	}
}

func TestFindingRecurrenceAndHistoricalReconciliationAreAtomic(t *testing.T) {
	for attempt := 0; attempt < 100; attempt++ {
		store := NewFindingsStore()
		report := func() {
			store.Add(&Finding{ID: "failure", ResourceID: "resource", Severity: FindingSeverityWarning, Title: "Failure"})
		}
		report()
		session := &aicontracts.InvestigationSession{ID: "old", FindingID: "failure", SessionID: "old", StartedAt: time.Now().Add(-time.Minute)}
		store.UpdateInvestigation("failure", "old", "completed", string(InvestigationOutcomeFixVerified), nil, 1)
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; report() }()
		go func() {
			defer wg.Done()
			<-start
			store.ReconcileInvestigationOutcome("failure", session, string(InvestigationOutcomeFixVerified))
		}()
		close(start)
		wg.Wait()
		if got := store.Get("failure"); got.ResolvedAt != nil || !got.ShouldInvestigate("full") || got.RegressionCount != 1 {
			t.Fatalf("historical result closed concurrent recurrence: %#v", got)
		}
	}
}
