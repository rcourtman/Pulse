package config

import "testing"

func TestNormalizeMetricEvaluationWindowsSeedsCPUAndPreservesCurrentValue(t *testing.T) {
	if got := NormalizeMetricEvaluationWindows(nil)["all"]["cpu"]; got != DefaultCPUEvaluationWindowSeconds {
		t.Fatalf("default CPU window = %d, want %d", got, DefaultCPUEvaluationWindowSeconds)
	}

	got := NormalizeMetricEvaluationWindows(map[string]map[string]int{
		"all":   {"CPU": 0, "memory": 900},
		"VM":    {"diskRead": 900},
		"agent": {"networkOut": MaxMetricEvaluationWindowSeconds + 1},
	})
	if got["all"]["cpu"] != 0 {
		t.Fatalf("explicit current-value mode was not preserved: %+v", got)
	}
	if _, exists := got["all"]["memory"]; exists {
		t.Fatalf("safety-sensitive memory metric should not be windowed: %+v", got)
	}
	if got["vm"]["diskread"] != 900 {
		t.Fatalf("resource override was not canonicalized: %+v", got)
	}
	if got["agent"]["networkout"] != MaxMetricEvaluationWindowSeconds {
		t.Fatalf("window was not bounded: %+v", got)
	}
}
