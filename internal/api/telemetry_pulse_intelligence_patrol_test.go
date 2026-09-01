package api

import (
	"testing"
	"time"
)

func TestRouterPulseIntelligencePatrolAutonomyLevelDefaultsToMonitor(t *testing.T) {
	var nilRouter *Router
	if got := nilRouter.pulseIntelligencePatrolAutonomyLevel(); got != "monitor" {
		t.Fatalf("nil router autonomy level = %q, want monitor", got)
	}
	if got := (&Router{}).pulseIntelligencePatrolAutonomyLevel(); got != "monitor" {
		t.Fatalf("router without AI settings handler autonomy level = %q, want monitor", got)
	}
	if got := (&Router{}).GetPulseIntelligenceActionTelemetry(time.Time{}).PatrolAutonomyLevel; got != "monitor" {
		t.Fatalf("action snapshot autonomy level = %q, want monitor", got)
	}
}
