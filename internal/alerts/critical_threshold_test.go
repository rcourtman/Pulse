package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestComputeCriticalThreshold_PercentageMetric(t *testing.T) {
	tests := []struct {
		name       string
		trigger    float64
		metricType string
		want       float64
	}{
		{name: "cpu low trigger", trigger: 80, metricType: "cpu", want: 90},
		{name: "cpu mid trigger", trigger: 85, metricType: "cpu", want: 95},
		{name: "cpu high trigger capped", trigger: 95, metricType: "cpu", want: 99},
		{name: "cpu 90 capped at boundary", trigger: 90, metricType: "cpu", want: 99},
		{name: "memory high trigger capped", trigger: 93, metricType: "memory", want: 99},
		{name: "disk high trigger capped", trigger: 91, metricType: "disk", want: 99},
		{name: "usage high trigger capped", trigger: 97, metricType: "usage", want: 99},
		{name: "temperature not capped", trigger: 95, metricType: "temperature", want: 105},
		{name: "diskRead not capped", trigger: 100, metricType: "diskRead", want: 110},
		{name: "networkIn not capped", trigger: 50, metricType: "networkIn", want: 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeCriticalThreshold(tt.trigger, tt.metricType)
			if got != tt.want {
				t.Errorf("computeCriticalThreshold(%.1f, %q) = %.1f, want %.1f", tt.trigger, tt.metricType, got, tt.want)
			}
		})
	}
}

func TestBuildCanonicalMetricSpec_CriticalCappedForHighTrigger(t *testing.T) {
	threshold := &config.HysteresisThreshold{Trigger: 95, Clear: 90}

	spec, err := buildCanonicalMetricSpec("res-1", "CPU", unifiedresources.ResourceTypeAgent, "cpu", threshold)
	if err != nil {
		t.Fatalf("buildCanonicalMetricSpec: %v", err)
	}
	if spec.MetricThreshold.Critical == nil {
		t.Fatal("expected critical threshold to be set")
	}
	if *spec.MetricThreshold.Critical > 99 {
		t.Errorf("critical threshold = %.1f, should be capped at 99 for percentage metric", *spec.MetricThreshold.Critical)
	}
	if *spec.MetricThreshold.Critical != 99 {
		t.Errorf("critical threshold = %.1f, want 99", *spec.MetricThreshold.Critical)
	}
}
