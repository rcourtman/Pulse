package reducer

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

func signalAt(value float64, offset time.Duration) MetricSignal {
	return MetricSignal{
		ResourceID: "vm-100",
		Metric:     "cpu",
		Value:      value,
		ObservedAt: t0.Add(offset),
	}
}

func eventTypes(events []Event) []EventType {
	types := make([]EventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func TestFireHoldResolveWithHysteresis(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75}

	events := state.ApplyMetric(signalAt(85, 0), rule)
	if len(events) != 1 || events[0].Type != EventFired || events[0].Severity != SeverityWarning {
		t.Fatalf("events = %+v, want one warning fired", events)
	}

	// Between clear and trigger: hold, no events, no state refresh.
	if events := state.ApplyMetric(signalAt(78, time.Minute), rule); len(events) != 0 {
		t.Fatalf("hysteresis hold emitted %+v", events)
	}
	incident, ok := state.Incident("vm-100", "cpu")
	if !ok || incident.State != StateFiring || incident.LastValue != 85 {
		t.Fatalf("incident = %+v, want firing with unrefreshed value 85", incident)
	}

	events = state.ApplyMetric(signalAt(74, 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved", events)
	}
	if _, ok := state.Incident("vm-100", "cpu"); ok {
		t.Fatal("incident should be gone after resolve")
	}
}

func TestSeverityEscalatesAndDemotesWhileFiring(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75}

	state.ApplyMetric(signalAt(85, 0), rule)

	events := state.ApplyMetric(signalAt(95, time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventSeverityChanged || events[0].Severity != SeverityCritical {
		t.Fatalf("events = %+v, want escalation to critical", events)
	}

	events = state.ApplyMetric(signalAt(85, 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventSeverityChanged || events[0].Severity != SeverityWarning {
		t.Fatalf("events = %+v, want demotion to warning", events)
	}
}

func TestFiresCriticalImmediatelyAboveCriticalThreshold(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75}

	events := state.ApplyMetric(signalAt(95, 0), rule)
	if len(events) != 1 || events[0].Type != EventFired || events[0].Severity != SeverityCritical {
		t.Fatalf("events = %+v, want critical fired", events)
	}
}

func TestPercentageCriticalCapAt99(t *testing.T) {
	state := NewState()
	// Trigger 95 → critical would be 105, capped at 99 for cpu.
	rule := MetricRule{Trigger: 95, Clear: 90}

	events := state.ApplyMetric(signalAt(99, 0), rule)
	if len(events) != 1 || events[0].Severity != SeverityCritical {
		t.Fatalf("events = %+v, want critical at capped threshold", events)
	}

	// Non-percentage metric keeps trigger+10.
	other := NewState()
	diskWrite := MetricSignal{ResourceID: "vm-100", Metric: "diskWrite", Value: 99, ObservedAt: t0}
	events = other.ApplyMetric(diskWrite, MetricRule{Trigger: 95})
	if len(events) != 1 || events[0].Severity != SeverityWarning {
		t.Fatalf("events = %+v, want warning below 105 for non-percentage metric", events)
	}
}

func TestDelayFiresWithPendingStartAndDipResets(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75, DelaySeconds: 120}

	events := state.ApplyMetric(signalAt(90, 0), rule)
	if len(events) != 1 || events[0].Type != EventPending {
		t.Fatalf("events = %+v, want pending", events)
	}

	// Still inside the delay window: nothing fires.
	if events := state.ApplyMetric(signalAt(91, time.Minute), rule); len(events) != 0 {
		t.Fatalf("early observation emitted %+v", events)
	}

	// Dip below trigger resets the delay entirely.
	events = state.ApplyMetric(signalAt(70, 90*time.Second), rule)
	if len(events) != 1 || events[0].Type != EventPendingCleared {
		t.Fatalf("events = %+v, want pending cleared", events)
	}

	// Re-enter pending; fire only after the full delay from re-entry.
	state.ApplyMetric(signalAt(92, 100*time.Second), rule)
	if events := state.ApplyMetric(signalAt(92, 100*time.Second+119*time.Second), rule); len(events) != 0 {
		t.Fatalf("fired before delay elapsed: %+v", events)
	}
	events = state.ApplyMetric(signalAt(92, 100*time.Second+120*time.Second), rule)
	if len(events) != 1 || events[0].Type != EventFired {
		t.Fatalf("events = %+v, want fired after delay", events)
	}
	incident, _ := state.Incident("vm-100", "cpu")
	if !incident.StartedAt.Equal(t0.Add(100 * time.Second)) {
		t.Fatalf("StartedAt = %v, want pending entry time %v", incident.StartedAt, t0.Add(100*time.Second))
	}
}

func TestCriticalCanBypassWarningStabilityDelay(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75, DelaySeconds: 300, CriticalBypassesDelay: true}

	events := state.ApplyMetric(signalAt(95, 0), rule)
	if len(events) != 1 || events[0].Type != EventFired || events[0].Severity != SeverityCritical {
		t.Fatalf("events = %+v, want immediate critical fire", events)
	}
}

func TestCriticalDoesNotBypassExplicitIntent(t *testing.T) {
	state := NewState()
	rule := MetricRule{
		Trigger: 80, Clear: 75, CriticalBypassesDelay: true,
		Intent: &DiscreteIntent{Explicit: true, GraceSeconds: 300},
	}

	events := state.ApplyMetric(signalAt(95, 0), rule)
	if len(events) != 1 || events[0].Type != EventPending {
		t.Fatalf("events = %+v, want explicit intent to keep critical pending", events)
	}
}

func TestMetricRecoveryRequiresContinuousHealthyWindow(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75, RecoveryDelaySeconds: 120}
	state.ApplyMetric(signalAt(90, 0), rule)

	if events := state.ApplyMetric(signalAt(70, time.Minute), rule); len(events) != 0 {
		t.Fatalf("first healthy observation emitted %+v", events)
	}
	if events := state.ApplyMetric(signalAt(78, 90*time.Second), rule); len(events) != 0 {
		t.Fatalf("hysteresis-band interruption emitted %+v", events)
	}
	if events := state.ApplyMetric(signalAt(70, 2*time.Minute), rule); len(events) != 0 {
		t.Fatalf("restarted healthy run emitted %+v", events)
	}
	if events := state.ApplyMetric(signalAt(70, 3*time.Minute+59*time.Second), rule); len(events) != 0 {
		t.Fatalf("recovery fired before continuous window elapsed: %+v", events)
	}
	events := state.ApplyMetric(signalAt(70, 4*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want recovery after continuous healthy window", events)
	}
}

func TestClearFallsBackToTriggerWhenUnset(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 0}

	state.ApplyMetric(signalAt(85, 0), rule)

	// At exactly the trigger the fire branch wins: still firing, no events.
	if events := state.ApplyMetric(signalAt(80, time.Minute), rule); len(events) != 0 {
		t.Fatalf("events at trigger equality = %+v, want none", events)
	}
	if incident, ok := state.Incident("vm-100", "cpu"); !ok || incident.State != StateFiring {
		t.Fatalf("incident = %+v, want still firing at trigger equality", incident)
	}

	// With Clear unset, resolve requires value <= trigger.
	events := state.ApplyMetric(signalAt(79, 2*time.Minute), rule)
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved at value <= trigger", events)
	}
}

func TestDisabledRuleClearsPendingAndFiring(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75, DelaySeconds: 60}

	state.ApplyMetric(signalAt(90, 0), rule)
	events := state.ApplyMetric(signalAt(90, time.Second), MetricRule{Trigger: 0})
	if len(events) != 1 || events[0].Type != EventPendingCleared {
		t.Fatalf("events = %+v, want pending cleared on disable", events)
	}

	firing := NewState()
	firing.ApplyMetric(signalAt(90, 0), MetricRule{Trigger: 80, Clear: 75})
	events = firing.ApplyMetric(signalAt(90, time.Second), MetricRule{Trigger: 0})
	if len(events) != 1 || events[0].Type != EventResolved {
		t.Fatalf("events = %+v, want resolved on disable", events)
	}
}

func TestDeterminism(t *testing.T) {
	run := func() []EventType {
		state := NewState()
		rule := MetricRule{Trigger: 80, Clear: 75, DelaySeconds: 60}
		var all []Event
		steps := []struct {
			value  float64
			offset time.Duration
		}{
			{85, 0}, {86, 30 * time.Second}, {85, 61 * time.Second},
			{95, 2 * time.Minute}, {78, 3 * time.Minute}, {70, 4 * time.Minute},
		}
		for _, step := range steps {
			all = append(all, state.ApplyMetric(signalAt(step.value, step.offset), rule)...)
		}
		return eventTypes(all)
	}

	first := run()
	second := run()
	if len(first) != len(second) {
		t.Fatalf("non-deterministic event counts: %v vs %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("non-deterministic events: %v vs %v", first, second)
		}
	}
	want := []EventType{EventPending, EventFired, EventSeverityChanged, EventResolved}
	if len(first) != len(want) {
		t.Fatalf("events = %v, want %v", first, want)
	}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("events = %v, want %v", first, want)
		}
	}
}
