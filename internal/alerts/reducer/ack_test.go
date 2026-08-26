package reducer

import (
	"testing"
	"time"
)

func TestAcknowledgeRequiresFiringIncident(t *testing.T) {
	state := NewState()
	if state.Acknowledge("node-1", "connectivity", "richard", t0) {
		t.Fatal("ack of a missing incident should fail")
	}

	rule := DiscreteRule{Confirmations: 2}
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	if state.Acknowledge("node-1", "connectivity", "richard", t0) {
		t.Fatal("ack of a pending incident should fail")
	}

	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, time.Minute), rule)
	if !state.Acknowledge("node-1", "connectivity", "richard", t0.Add(2*time.Minute)) {
		t.Fatal("ack of a firing incident should succeed")
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.Acknowledged || incident.AckUser != "richard" {
		t.Fatalf("incident = %+v, want acknowledged by richard", incident)
	}
}

func TestUnacknowledgeClearsFlagAndRecord(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.Acknowledge("node-1", "connectivity", "richard", t0.Add(time.Minute))

	if !state.Unacknowledge("node-1", "connectivity") {
		t.Fatal("unack of a firing incident should succeed")
	}
	incident, _ := state.Incident("node-1", "connectivity")
	if incident.Acknowledged {
		t.Fatal("incident should no longer be acknowledged")
	}

	// After unack, a resolve/re-fire cycle must NOT restore the ack.
	state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule)
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 3*time.Minute), rule)
	incident, _ = state.Incident("node-1", "connectivity")
	if incident.Acknowledged {
		t.Fatal("unacknowledged record must not be restored on re-fire")
	}
}

func TestAckSurvivesResolveAndRefireWithinRetention(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.Acknowledge("node-1", "connectivity", "richard", t0.Add(time.Minute))

	state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule)
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 3*time.Minute), rule)

	incident, _ := state.Incident("node-1", "connectivity")
	if !incident.Acknowledged || incident.AckUser != "richard" {
		t.Fatalf("incident = %+v, want ack restored on re-fire", incident)
	}
	if !incident.AckAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("AckAt = %v, want original ack time preserved", incident.AckAt)
	}
}

func TestAckExpiresAfterRetention(t *testing.T) {
	state := NewState()
	rule := DiscreteRule{Confirmations: 1}
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, 0), rule)
	state.Acknowledge("node-1", "connectivity", "richard", t0.Add(time.Minute))
	state.ApplyDiscrete(discreteSignalAt(false, "", 2*time.Minute), rule)

	// Re-fire more than AckRetention after the resolve: ack must not return.
	refireAt := 2*time.Minute + AckRetention + time.Minute
	state.ApplyDiscrete(discreteSignalAt(true, SeverityWarning, refireAt), rule)
	incident, _ := state.Incident("node-1", "connectivity")
	if incident.Acknowledged {
		t.Fatal("expired ack record must not be restored")
	}
}

func TestAckAppliesToMetricFamilyToo(t *testing.T) {
	state := NewState()
	rule := MetricRule{Trigger: 80, Clear: 75}
	state.ApplyMetric(MetricSignal{ResourceID: "vm-1", Metric: "cpu", Value: 85, ObservedAt: t0}, rule)
	if !state.Acknowledge("vm-1", "cpu", "richard", t0.Add(time.Minute)) {
		t.Fatal("ack of a firing metric incident should succeed")
	}

	state.ApplyMetric(MetricSignal{ResourceID: "vm-1", Metric: "cpu", Value: 70, ObservedAt: t0.Add(2 * time.Minute)}, rule)
	state.ApplyMetric(MetricSignal{ResourceID: "vm-1", Metric: "cpu", Value: 85, ObservedAt: t0.Add(3 * time.Minute)}, rule)
	incident, _ := state.Incident("vm-1", "cpu")
	if !incident.Acknowledged {
		t.Fatal("metric re-fire within retention should restore the ack")
	}
}
