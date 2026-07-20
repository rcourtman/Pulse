package mock

import (
	"math"
	"testing"
	"time"
)

func TestSampleMetricSeriesMatchesCanonicalPointSampler(t *testing.T) {
	start := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 96)
	for i := 0; i < 96; i++ {
		timestamps = append(timestamps, start.Add(time.Duration(i)*15*time.Minute))
	}

	for _, tc := range []struct {
		resourceClass string
		resourceID    string
		metric        string
	}{
		{resourceClass: "node", resourceID: "pve1", metric: "cpu"},
		{resourceClass: "vm", resourceID: "database-primary", metric: "memory"},
		{resourceClass: "storage", resourceID: "backup-pool", metric: "usage"},
		{resourceClass: "disk", resourceID: "NVME-SERIAL-1", metric: "smart_temp"},
		{resourceClass: "dockerContainer", resourceID: "api-service", metric: "netin"},
	} {
		t.Run(tc.resourceClass+"/"+tc.metric, func(t *testing.T) {
			got := SampleMetricSeries(tc.resourceClass, tc.resourceID, tc.metric, timestamps)
			if len(got) != len(timestamps) {
				t.Fatalf("series length = %d, want %d", len(got), len(timestamps))
			}
			for i, at := range timestamps {
				want := SampleMetric(tc.resourceClass, tc.resourceID, tc.metric, at)
				if math.Abs(got[i]-want) > 1e-12 {
					t.Fatalf("point %d at %v = %v, want %v", i, at, got[i], want)
				}
			}
		})
	}
}

func TestSampleMetricSeriesEmptyInput(t *testing.T) {
	if got := SampleMetricSeries("vm", "vm-1", "cpu", nil); got != nil {
		t.Fatalf("empty series = %#v, want nil", got)
	}
}
