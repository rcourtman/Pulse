package ai

import "testing"

func TestPatrolMetrics_Recorders(t *testing.T) {
	m := GetPatrolMetrics()
	m.RecordCircuitBlock()
	m.RecordInvestigationOutcome("resolved")
	m.RecordFixVerification("success")
	m.RecordScopedDroppedFinal()
	m.RecordRun("manual", "scoped")
}
