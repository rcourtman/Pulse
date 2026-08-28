package monitoring

import (
	"strings"
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

func TestMetricWindowPointsKeepsHistorySnapshotDuringReplacement(t *testing.T) {
	now := time.Now().UTC()
	monitor := newChartFallbackTestMonitor(t)
	history := monitor.metricsHistory
	cacheKey := strings.Join([]string{"vm", "vm-1", "cpu", "300"}, "\x00")
	history.metricWindowCache = map[string]metricWindowCacheEntry{
		cacheKey: {
			points:    []alerts.MetricWindowPoint{{Timestamp: now.Add(-time.Minute), Value: 42}},
			expiresAt: now.Add(time.Minute),
		},
	}

	history.metricWindowMu.Lock()
	result := make(chan []alerts.MetricWindowPoint, 1)
	go func() {
		points, _ := monitor.metricWindowPoints(alerts.MetricWindowRequest{
			ResourceID:   "vm-1",
			ResourceType: "vm",
			Metric:       "cpu",
			Start:        now.Add(-5 * time.Minute),
			End:          now,
		})
		result <- points
	}()

	// Give the request time to snapshot history and block on its cache mutex,
	// matching the mock seed replacement that exposed the startup panic.
	time.Sleep(20 * time.Millisecond)
	monitor.mu.Lock()
	monitor.metricsHistory = NewMetricsHistory(32, time.Hour)
	monitor.mu.Unlock()
	history.metricWindowMu.Unlock()

	select {
	case points := <-result:
		if len(points) != 1 || points[0].Value != 42 {
			t.Fatalf("metricWindowPoints = %+v, want cached point from the request snapshot", points)
		}
	case <-time.After(time.Second):
		t.Fatal("metricWindowPoints did not finish after history replacement")
	}
}
