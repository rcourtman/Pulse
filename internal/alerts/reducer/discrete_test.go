package reducer

import (
	"testing"
	"time"
)

func discreteSignalAt(matched bool, severity Severity, offset time.Duration) DiscreteSignal {
	return DiscreteSignal{
		ResourceID: "node-1",
		Key:        "connectivity",
		Matched:    matched,
		Severity:   severity,
		ObservedAt: t0.Add(offset),
	}
}

func TestDiscreteFiresAfterConfirmations(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 3}

	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 0), rule)
	if len(events) != 1 || events[0].Type != EventPending {
		t.Fatalf("events = %+v, want pending on first match", events)
	}
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, time.Minute), rule); len(events) != 0 {
		t.Fatalf("second match emitted %+v", events)
	}

	events = state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventFired || events[0].Severity != SeverityCritical {
		t.Fatalf("events = %+v, want critical fired on third match", events)
	}

	incident, ok := state.Incident("node-1", "connectivity")
	if !ok || incident.State != StateFiring {
		t.Fatalf("incident = %+v, want firing", incident)
	}
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want first matched observation %v", incident.StartedAt, t0)
	}
	if incident.Confirmations != 3 {
		t.Fatalf("Confirmations = %d, want clamped at 3", incident.Confirmations)
	}
}

func TestDiscreteNonMatchResetsPendingCount(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 3}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), rule)

	events := state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventPendingCleared {
		t.Fatalf("events = %+v, want pending cleared", events)
	}

	// The count starts over: two more matches must not fire.
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 3*time.Minute), rule)
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute), rule); len(events) != 0 {
		t.Fatalf("post-reset second match emitted %+v", events)
	}
	events = state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 5*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired on third consecutive match", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0.Add(3 * time.Minute)) {
		t.Fatalf("StartedAt = %v, want restart of the pending run", incident.StartedAt)
	}
}

func TestDiscreteSingleConfirmationFiresImmediately(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}

	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want immediate fire", events)
	}
	events = state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved on single recovery", events)
	}
}

func TestDiscreteSeverityFollowsRuleWhileFiring(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventSeverityChanged || events[0].Severity != SeverityCritical {
		t.Fatalf("events = %+v, want severity change to critical", events)
	}
	if events := state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 2*time.Minute), rule); len(events) != 0 {
		t.Fatalf("unchanged severity emitted %+v", events)
	}
}

func TestDiscreteDisabledClearsPendingAndFiring(t *testing.T) {
	pending := NewState()
	rule := DiscreteRule{Confirmations: 3}
	pending.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	events := pending.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), DiscreteRule{Confirmations: 3, Disabled: true})
	if len(events) != 1 || events[0].Type != EventPendingCleared {
		t.Fatalf("events = %+v, want pending cleared on disable", events)
	}

	firing := NewState()
	firing.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), DiscreteRule{Confirmations: 1})
	events = firing.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), DiscreteRule{Confirmations: 1, Disabled: true})
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved on disable", events)
	}
}

func TestDiscreteRefireRequiresFullConfirmations(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 2}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), rule)
	state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule)

	// After recovery the confirmation run restarts from zero.
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 3*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventPending {
		t.Fatalf("events = %+v, want pending after recovery", events)
	}
	events = state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventRefired {
		t.Fatalf("events = %+v, want refired after full confirmations within retention", events)
	}
	// Within RefireRetention the original occurrence's start is restored.
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want restored original start %v", incident.StartedAt, t0)
	}
}

func TestDiscreteRecoveryGateHoldsUntilConsecutiveConfirmations(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1, RecoveryConfirmations: 3}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityCritical, 0), rule)

	// Two healthy observations: still firing.
	if events := state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule); len(events) != 0 {
		t.Fatalf("first healthy emitted %+v", events)
	}
	if events := state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule); len(events) != 0 {
		t.Fatalf("second healthy emitted %+v", events)
	}
	incident, ok := state.Incident("node-1", "connectivity")
	if !ok || incident.State != StateFiring || incident.RecoveryCount != 2 {
		t.Fatalf("incident = %+v, want firing with recovery count 2", incident)
	}

	events := state.ApplyDiscrete(discreteSignalAt(false, "", 3*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved on third consecutive healthy", events)
	}
	if _, ok := state.Incident("node-1", "connectivity"); ok {
		t.Fatal("incident should be gone after gated resolve")
	}
}

func TestDiscreteMatchedObservationResetsRecoveryRun(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1, RecoveryConfirmations: 3}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule)
	state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule)

	// Back offline: recovery run resets.
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 3*time.Minute), rule)
	incident, _ := state.Incident("node-1", "connectivity")
	if incident.RecoveryCount != 0 {
		t.Fatalf("RecoveryCount = %d, want reset to 0", incident.RecoveryCount)
	}

	// Two healthy observations are again not enough.
	state.ApplyDiscrete(discreteSignalAt(false, "", 4*time.Minute), rule)
	if events := state.ApplyDiscrete(discreteSignalAt(false, "", 5*time.Minute), rule); len(events) != 0 {
		t.Fatalf("post-reset second healthy emitted %+v", events)
	}
	events := state.ApplyDiscrete(discreteSignalAt(false, "", 6*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved after full recovery run", events)
	}
}

func TestDiscretePendingClearsImmediatelyDespiteRecoveryGate(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 3, RecoveryConfirmations: 3}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	events := state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventPendingCleared {
		t.Fatalf("events = %+v, want pending cleared on single healthy observation", events)
	}
}

func TestDiscreteDisableBypassesRecoveryGate(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1, RecoveryConfirmations: 3}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), DiscreteRule{Confirmations: 1, RecoveryConfirmations: 3, Disabled: true})
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want immediate resolve on disable", events)
	}
}

func TestDiscreteRefireOutsideRetentionStartsFresh(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule)

	// Beyond RefireRetention the resolved occurrence is no longer
	// consumable: a re-fire is a new occurrence with a fresh start.
	refireAt := time.Minute + RefireRetention + time.Second
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, refireAt), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want plain fired outside retention", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0.Add(refireAt)) {
		t.Fatalf("StartedAt = %v, want fresh start %v", incident.StartedAt, t0.Add(refireAt))
	}
}

func TestDiscreteRefireRecordConsumedOnce(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.ApplyDiscrete(discreteSignalAt(false, "", time.Minute), rule)

	// First re-fire restores; its own later resolve records a NEW
	// occurrence whose start is the restored (original) start.
	events := state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventRefired {
		t.Fatalf("events = %+v, want refired", events)
	}
	state.ApplyDiscrete(discreteSignalAt(false, "", 3*time.Minute), rule)

	// Second re-fire within retention restores again — from the second
	// occurrence's record, which carries the same original start.
	events = state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 4*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventRefired {
		t.Fatalf("events = %+v, want refired from the newer record", events)
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want original start carried through %v", incident.StartedAt, t0)
	}
}
