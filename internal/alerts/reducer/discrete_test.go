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
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want re-fire after full confirmations", events)
	}
}
