package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestMetricWindowPointsUsesInMemoryMetricAlias(t *testing.T) {
	now := time.Now().UTC()
	history := NewMetricsHistory(32, time.Hour)
	history.AddGuestMetric("vm-1", "netin", 12, now.Add(-4*time.Minute))
	history.AddGuestMetric("vm-1", "netin", 18, now.Add(-2*time.Minute))
	monitor := &Monitor{metricsHistory: history}

	points, err := monitor.metricWindowPoints(alerts.MetricWindowRequest{
		ResourceID:   "vm-1",
		ResourceType: "vm",
		Metric:       "networkIn",
		Start:        now.Add(-5 * time.Minute),
		End:          now,
	})
	if err != nil {
		t.Fatalf("metricWindowPoints returned error: %v", err)
	}
	if len(points) != 2 || points[0].Value != 12 || points[1].Value != 18 {
		t.Fatalf("metricWindowPoints = %+v, want canonical netin history", points)
	}
}

func TestMetricWindowMergePrefersFreshInMemoryDuplicate(t *testing.T) {
	now := time.Now().UTC()
	stored := []MetricPoint{
		{Timestamp: now.Add(-4 * time.Minute), Value: 40},
		{Timestamp: now.Add(-2 * time.Minute), Value: 99},
	}
	inMemory := []MetricPoint{
		{Timestamp: now.Add(-2 * time.Minute), Value: 55},
		{Timestamp: now.Add(-time.Minute), Value: 60},
	}

	got := mergeMetricWindowPoints(stored, inMemory, now.Add(-5*time.Minute), now)
	if len(got) != 3 {
		t.Fatalf("mergeMetricWindowPoints returned %d points, want 3: %+v", len(got), got)
	}
	if got[1].Value != 55 {
		t.Fatalf("duplicate timestamp value = %.1f, want fresh in-memory value 55", got[1].Value)
	}
}

func TestMetricsHistoryResetClearsMetricWindowCache(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	history.metricWindowCache = map[string]metricWindowCacheEntry{
		"vm\x00vm-1\x00cpu\x00300": {
			points:    []alerts.MetricWindowPoint{{Timestamp: time.Now().UTC(), Value: 42}},
			expiresAt: time.Now().Add(time.Minute),
		},
	}

	history.Reset()
	if history.metricWindowCache != nil {
		t.Fatalf("metric window cache survived reset: %+v", history.metricWindowCache)
	}
}
