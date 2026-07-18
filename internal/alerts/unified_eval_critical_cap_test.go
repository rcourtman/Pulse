package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A percentage metric with trigger at or above the 99 critical cap must still
// produce a valid, evaluable spec (warning-only, no escalation) instead of
// being skipped as invalid (#1593 log spam / silent monitoring gap).
func TestBuildCanonicalMetricSpec_HighPercentageTriggerStaysEvaluable(t *testing.T) {
	for _, trigger := range []float64{99, 100} {
		spec, err := buildCanonicalMetricSpec("agent-1", "nas", unifiedresources.ResourceTypeAgent, "memory", &HysteresisThreshold{Trigger: trigger, Clear: trigger - 5})
		if err != nil {
			t.Fatalf("trigger %v: expected valid spec, got error %v", trigger, err)
		}
		if spec.MetricThreshold.Critical != nil {
			t.Fatalf("trigger %v: expected escalation omitted, got critical %v", trigger, *spec.MetricThreshold.Critical)
		}
	}

	spec, err := buildCanonicalMetricSpec("agent-1", "nas", unifiedresources.ResourceTypeAgent, "memory", &HysteresisThreshold{Trigger: 95, Clear: 90})
	if err != nil {
		t.Fatalf("trigger 95: unexpected error %v", err)
	}
	if spec.MetricThreshold.Critical == nil || *spec.MetricThreshold.Critical != 99 {
		t.Fatalf("trigger 95: expected critical capped at 99, got %+v", spec.MetricThreshold.Critical)
	}
}
