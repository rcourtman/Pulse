package monitoring

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This resolver blocks only after metricWindowPoints has captured its history.
// It lets the test replace that history at an exact point without scheduling
// sleeps or exposing a production-only hook.
type blockingMetricTargetStore struct {
	entered chan struct{}
	resume  <-chan struct{}
}

func (*blockingMetricTargetStore) ShouldSkipAPIPolling(string) bool { return false }
func (*blockingMetricTargetStore) GetPollingRecommendations() map[string]float64 {
	return nil
}
func (*blockingMetricTargetStore) GetAll() []unifiedresources.Resource       { return nil }
func (*blockingMetricTargetStore) PopulateFromSnapshot(models.StateSnapshot) {}
func (s *blockingMetricTargetStore) MetricsTargetForResource(string) *unifiedresources.MetricsTarget {
	s.entered <- struct{}{}
	<-s.resume
	return nil
}

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

func TestUsedMemoryHistoryCleanupThroughMonitorAndReaders(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)
	for _, resourceType := range []string{"vm", "container"} {
		t.Run(resourceType, func(t *testing.T) {
			now := time.Now()
			history := NewMetricsHistory(32, 4*time.Hour)
			monitor := &Monitor{metricsHistory: history}
			const id = "instance:node:105"
			monitor.recordGuestMetric(resourceType, id, 20, 60, 4096, -1, -1, -1, -1, -1, now.Add(-2*time.Hour))
			monitor.recordGuestMetric(resourceType, id, 25, -1, -1, -1, -1, -1, -1, -1, now.Add(-time.Minute))
			monitor.cleanupMetricsHistory()
			if got := monitor.GetGuestMetrics(id, 4*time.Hour)["memoryused"]; len(got) != 1 || got[0].Value != 4096 {
				t.Fatalf("in-window last-known bytes were lost or refreshed: %+v", got)
			}
			history.retentionTime = time.Hour
			monitor.cleanupMetricsHistory()
			if got := monitor.GetGuestMetrics(id, 4*time.Hour)["memoryused"]; len(got) != 0 {
				t.Fatalf("ongoing CPU retained expired byte samples: %+v", got)
			}
			monitor.recordGuestMetric(resourceType, id, 30, 70, 8192, -1, -1, -1, -1, -1, now)
			monitor.cleanupMetricsHistory()
			chart := monitor.GetGuestMetricsForChart(id, resourceType, id, 4*time.Hour)["memoryused"]
			if len(chart) != 1 || chart[0].Value != 8192 || !chart[0].Timestamp.Equal(now) {
				t.Fatalf("chart did not retain the fresh byte observation: %+v", chart)
			}
			points, err := monitor.metricWindowPoints(alerts.MetricWindowRequest{
				ResourceID: id, ResourceType: resourceType, Metric: "memoryused",
				Start: now.Add(-4 * time.Hour), End: now,
			})
			if err != nil || len(points) != 1 || points[0].Value != 8192 || !points[0].Timestamp.Equal(now) {
				t.Fatalf("metric window diverged from fresh chart bytes: points=%+v err=%v", points, err)
			}
		})
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
	now := time.Now().UTC()
	history.cacheMetricWindow("vm\x00vm-1\x00cpu\x00300", []MetricPoint{{Timestamp: now, Value: 42}}, now)

	history.Reset()
	if history.metricWindowCache != nil || history.metricWindowBytes != 0 || !history.metricWindowSweepAt.IsZero() {
		t.Fatalf("metric window cache survived reset: %+v", history.metricWindowCache)
	}
}

func TestMetricWindowPointsKeepsHistorySnapshotDuringReplacement(t *testing.T) {
	now := time.Now().UTC()
	monitor := newChartFallbackTestMonitor(t)
	history := monitor.metricsHistory
	cacheKey := strings.Join([]string{"vm", "vm-1", "cpu", "300"}, "\x00")
	history.cacheMetricWindow(cacheKey, []MetricPoint{{Timestamp: now.Add(-time.Minute), Value: 42}}, now)

	resume := make(chan struct{})
	var resumeOnce sync.Once
	resumeRequest := func() { resumeOnce.Do(func() { close(resume) }) }
	defer resumeRequest()
	entered := make(chan struct{}, 1)
	monitor.resourceStore = &blockingMetricTargetStore{entered: entered, resume: resume}
	type requestResult struct {
		points []alerts.MetricWindowPoint
		err    error
	}
	result := make(chan requestResult, 1)
	go func() {
		points, err := monitor.metricWindowPoints(alerts.MetricWindowRequest{
			ResourceID:   "vm-1",
			ResourceType: "vm",
			Metric:       "cpu",
			Start:        now.Add(-5 * time.Minute),
			End:          now,
		})
		result <- requestResult{points: points, err: err}
	}()

	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("metric request did not reach target resolution after snapshot")
	}
	monitor.mu.Lock()
	monitor.metricsHistory = NewMetricsHistory(32, time.Hour)
	monitor.mu.Unlock()
	resumeRequest()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("metricWindowPoints returned error: %v", got.err)
		}
		if len(got.points) != 1 || got.points[0].Value != 42 {
			t.Fatalf("metricWindowPoints = %+v, want cached point from the request snapshot", got.points)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("metricWindowPoints did not finish after history replacement")
	}
}

func TestMetricWindowPointsPreservesCompleteWindowAtCacheCapacity(t *testing.T) {
	for _, pressure := range []string{"none", "entries", "bytes"} {
		t.Run(pressure, func(t *testing.T) {
			monitor := newChartFallbackTestMonitor(t)
			history := monitor.metricsHistory
			now := time.Now().UTC().Truncate(time.Second)
			writeRawMetricBatch(t, monitor.metricsStore, "vm", "vm-1", "cpu", []MetricPoint{
				{Timestamp: now.Add(-4 * time.Minute), Value: 0},
				{Timestamp: now.Add(-2 * time.Minute), Value: 99},
				{Timestamp: now.Add(-time.Minute), Value: 30},
			})
			history.AddGuestMetric("vm-1", "cpu", 55, now.Add(-2*time.Minute))
			history.AddGuestMetric("vm-1", "cpu", 60, now.Add(-30*time.Second))

			// Fill the optional cache, not the query source or live history.
			// Its complete source result must still reach alert evaluation.
			cacheNow := time.Now()
			switch pressure {
			case "entries":
				for i := 0; i < metricWindowCacheMaxEntries; i++ {
					history.cacheMetricWindow(fmt.Sprintf("other-%04d", i), nil, cacheNow)
				}
			case "bytes":
				points := make([]MetricPoint, 4096)
				for i := 0; i < 63; i++ {
					history.cacheMetricWindow(fmt.Sprintf("other-%04d", i), points, cacheNow)
				}
				remaining := metricWindowCacheMaxBytes - history.metricWindowBytes
				if remaining <= metricWindowCacheEntryBytes {
					t.Fatal("fixture has no room for the final exact-budget entry")
				}
				history.cacheMetricWindow(strings.Repeat("x", remaining-metricWindowCacheEntryBytes), nil, cacheNow)
				if history.metricWindowBytes != metricWindowCacheMaxBytes {
					t.Fatalf("fixture filled %d bytes, want %d", history.metricWindowBytes, metricWindowCacheMaxBytes)
				}
			}
			retainedEntries, retainedBytes := len(history.metricWindowCache), history.metricWindowBytes
			request := alerts.MetricWindowRequest{
				ResourceID: "vm-1", ResourceType: "vm", Metric: "cpu",
				Start: now.Add(-5 * time.Minute), End: now,
			}
			want := []alerts.MetricWindowPoint{
				{Timestamp: now.Add(-4 * time.Minute), Value: 0},
				{Timestamp: now.Add(-2 * time.Minute), Value: 55},
				{Timestamp: now.Add(-time.Minute), Value: 30},
				{Timestamp: now.Add(-30 * time.Second), Value: 60},
			}
			assertWindow := func() {
				t.Helper()
				got, err := monitor.metricWindowPoints(request)
				if err != nil || len(got) != len(want) {
					t.Fatalf("window lost source coverage: got=%+v err=%v want=%+v", got, err, want)
				}
				for i := range want {
					if !got[i].Timestamp.Equal(want[i].Timestamp) || got[i].Value != want[i].Value {
						t.Fatalf("window changed source time/value or live-tail authority: got=%+v want=%+v", got, want)
					}
				}
				// A returned result never owns the cache or next query's data.
				got[0].Value = -1
			}
			assertWindow()
			assertWindow()
			cacheKey := strings.Join([]string{"vm", "vm-1", "cpu", "300"}, "\x00")
			_, cached := history.cachedMetricWindow(cacheKey, time.Now())
			if cached != (pressure == "none") {
				t.Fatalf("optional copy admitted=%v under %q pressure", cached, pressure)
			}
			if pressure != "none" && (len(history.metricWindowCache) != retainedEntries || history.metricWindowBytes != retainedBytes) {
				t.Fatal("refused optional copy changed the retained working set")
			}
			assertMetricWindowCacheAccounting(t, history)

			// Reset releases capacity and live history, not the persistent source.
			// The ordinary reader can cache again and still returns every point.
			history.Reset()
			history.AddGuestMetric("vm-1", "cpu", 55, now.Add(-2*time.Minute))
			history.AddGuestMetric("vm-1", "cpu", 60, now.Add(-30*time.Second))
			assertWindow()
			if _, ok := history.cachedMetricWindow(cacheKey, time.Now()); !ok {
				t.Fatal("reset did not restore ordinary cache admission")
			}
			assertMetricWindowCacheAccounting(t, history)
		})
	}
}
