package mock

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestMetricSamplerRemainsBoundToFixtureGraph(t *testing.T) {
	previousRegistry := currentMetricRoleRegistry()
	t.Cleanup(func() {
		setMetricRoleRegistry(previousRegistry)
	})

	graph := FixtureGraph{
		State: models.StateSnapshot{
			Containers: []models.Container{{
				ID:     "neutral-155",
				Name:   "backup-orchestrator",
				Status: "running",
			}},
		},
	}
	sampler := NewMetricSampler(graph)
	if got := sampler.role("container", "neutral-155"); got != metricRoleBackup {
		t.Fatalf("sampler role = %q, want %q", got, metricRoleBackup)
	}

	at := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	want := sampler.SampleMetric("container", "neutral-155", "memory", at)

	setMetricRoleRegistry(map[string]string{
		metricRoleRegistryKey("container", "neutral-155"): metricRoleDatabase,
	})
	if got := sampler.SampleMetric("container", "neutral-155", "memory", at); math.Abs(got-want) > 1e-12 {
		t.Fatalf("sampler changed after global registry update: got %v, want %v", got, want)
	}
	if global := SampleMetric("container", "neutral-155", "memory", at); math.Abs(global-want) < 1e-6 {
		t.Fatalf("global database sample unexpectedly matched graph-bound backup sample: %v", global)
	}

	timestamps := []time.Time{at.Add(-time.Minute), at, at.Add(time.Minute)}
	series := sampler.SampleMetricSeries("container", "neutral-155", "memory", timestamps)
	for i, timestamp := range timestamps {
		point := sampler.SampleMetric("container", "neutral-155", "memory", timestamp)
		if math.Abs(series[i]-point) > 1e-12 {
			t.Fatalf("series point %d = %v, want %v", i, series[i], point)
		}
	}
}
