package telemetry

import (
	"encoding/json"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"strings"
	"testing"
)

func TestPatrolInvestigationRunTelemetryPreviewIsAggregateOnly(t *testing.T) {
	var snap Snapshot
	ApplyPatrolInvestigationTelemetry(&snap, config.PatrolInvestigationTelemetrySummary{Available: true, ObservationDays: 2, Runs: 3, Outcomes: map[string]int{"fix_verified": 1, "needs_attention": 2}, Handoffs: map[aicontracts.InvestigationHandoffReason]int{aicontracts.HandoffMissingEvidence: 2}})
	ping := applySnapshot(Ping{}, func() Snapshot { return snap })
	raw, err := json.Marshal(ping)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["pulse_intelligence_patrol_investigation_runs_30d"] != float64(3) || decoded["pulse_intelligence_patrol_investigation_runs_fix_verified_30d"] != float64(1) || decoded["pulse_intelligence_patrol_investigation_handoff_missing_evidence_30d"] != float64(2) || decoded["pulse_intelligence_patrol_investigation_telemetry_available"] != true {
		t.Fatalf("preview omitted counts: %s", raw)
	}
	for name := range decoded {
		if strings.Contains(name, "investigation") && (strings.Contains(name, "session_id") || strings.Contains(name, "summary") || strings.Contains(name, "resource_id")) {
			t.Fatalf("identity/content field: %s", name)
		}
	}
}
