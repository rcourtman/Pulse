package reducer

import (
	"testing"
	"time"
)

func TestIntentOperatorSuppressionHoldsActivation(t *testing.T) {
	state := NewState()
	suppressed := DiscreteRule{Confirmations: 1, Intent: &DiscreteIntent{
		OperatorSuppressed: true, OperatorReason: "operator_expected_offline",
	}}

	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 0), suppressed)
	if len(events) != 1 || events[0].Type != EventPending {
		t.Fatalf("events = %+v, want pending while operator-suppressed", events)
	}
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, time.Minute), suppressed); len(events) != 0 {
		t.Fatalf("held observation emitted %+v", events)
	}

	// Suppression lifts: next observation activates with the run's first
	// active observation as the start.
	released := DiscreteRule{Confirmations: 1, Intent: &DiscreteIntent{}}
	events = state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 2*time.Minute), released)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired after suppression lifts", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want first active observation %v", incident.StartedAt, t0)
	}
}

func TestIntentGraceHoldsUntilElapsed(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1, Intent: &DiscreteIntent{Explicit: true, GraceSeconds: 120}}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), rule); len(events) != 0 {
		t.Fatalf("inside grace emitted %+v", events)
	}
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired at grace boundary", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want grace-run start %v", incident.StartedAt, t0)
	}
}

func TestIntentDipDuringGraceResetsRun(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1, Intent: &DiscreteIntent{Explicit: true, GraceSeconds: 120}}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	events := state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventPendingCleared {
		t.Fatalf("events = %+v, want pending cleared on dip", events)
	}

	// Grace restarts from the re-entry.
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 2*time.Minute), rule)
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 3*time.Minute), rule); len(events) != 0 {
		t.Fatalf("restarted grace emitted %+v", events)
	}
	events = state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired after restarted grace", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("StartedAt = %v, want restarted run start", incident.StartedAt)
	}
}

func TestIntentDoesNotSuppressFiringIncident(t *testing.T) {
	state := NewState()
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), DiscreteRule{Confirmations: 1})

	// Maintenance starting after activation does not clear or hold the
	// already-firing incident.
	held := DiscreteRule{Confirmations: 1, Intent: &DiscreteIntent{
		OperatorSuppressed: true, OperatorReason: "operator_maintenance",
	}}
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), held); len(events) != 0 {
		t.Fatalf("firing incident emitted %+v under maintenance", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if incident.State != StateFiring {
		t.Fatalf("incident = %+v, want still firing", incident)
	}
}

func TestIntentGraceComposesWithConfirmations(t *testing.T) {
	state := NewState()
	// Three confirmations AND a 5-minute grace: activation needs both.
	rule := DiscreteRule{Confirmations: 3, Intent: &DiscreteIntent{Explicit: true, GraceSeconds: 300}}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), rule)
	// Third confirmation reached at 2m, but grace holds until 5m.
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 2*time.Minute), rule); len(events) != 0 {
		t.Fatalf("confirmed-but-in-grace emitted %+v", events)
	}
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 5*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired once grace also elapses", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want run start %v", incident.StartedAt, t0)
	}
}

func backupIntent(grace, post, cap int, backupActive bool) *DiscreteIntent {
	return &DiscreteIntent{
		Explicit:                 true,
		GraceSeconds:             grace,
		BackupEnabled:            true,
		BackupActive:             backupActive,
		BackupPostGraceSeconds:   post,
		BackupMaxDeferralSeconds: cap,
	}
}

func TestBackupDeferralHoldsWhileBackupRuns(t *testing.T) {
	state := NewState()
	rule := func(active bool) DiscreteRule {
		return DiscreteRule{Confirmations: 1, Intent: backupIntent(60, 120, 600, active)}
	}

	// Offline with a backup running: held well past the plain grace.
	state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 0), rule(true))
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 3*time.Minute), rule(true)); len(events) != 0 {
		t.Fatalf("backup-deferred observation emitted %+v", events)
	}

	// Backup ends at 4m: eligibility extends to 4m + 120s post-grace.
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 4*time.Minute), rule(false)); len(events) != 0 {
		t.Fatalf("post-backup grace emitted %+v", events)
	}
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 5*time.Minute), rule(false)); len(events) != 0 {
		t.Fatalf("still inside post-grace emitted %+v", events)
	}
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 6*time.Minute), rule(false))
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired at backup end + post-grace", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want run start %v", incident.StartedAt, t0)
	}
}

func TestBackupDeferralCapBoundsTheHold(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1, Intent: backupIntent(60, 120, 300, true)}

	// Backup never ends: activation at the max-deferral cap (5m).
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute), rule); len(events) != 0 {
		t.Fatalf("under cap emitted %+v", events)
	}
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 5*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired at deferral cap", events)
	}
}

func TestBackupPostGraceCappedByMaxDeferral(t *testing.T) {
	state := NewState()
	// Backup ends at 4m; post-grace would extend to 4m+180s=7m, but the cap
	// is 5m — activation at 5m.
	rule := func(active bool) DiscreteRule {
		return DiscreteRule{Confirmations: 1, Intent: backupIntent(60, 180, 300, active)}
	}
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule(true))
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute), rule(false))
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute+30*time.Second), rule(false)); len(events) != 0 {
		t.Fatalf("under cap emitted %+v", events)
	}
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 5*time.Minute), rule(false))
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired at cap despite longer post-grace", events)
	}
}
