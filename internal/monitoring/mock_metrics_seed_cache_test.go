package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func resetMockMetricsSeedCacheForTest(t *testing.T) {
	t.Helper()
	mockMetricsSeedCache.Lock()
	previousKey := mockMetricsSeedCache.key
	previousHistory := mockMetricsSeedCache.history
	mockMetricsSeedCache.key = mockMetricsSeedCacheKey{}
	mockMetricsSeedCache.history = nil
	mockMetricsSeedCache.Unlock()
	t.Cleanup(func() {
		mockMetricsSeedCache.Lock()
		mockMetricsSeedCache.key = previousKey
		mockMetricsSeedCache.history = previousHistory
		mockMetricsSeedCache.Unlock()
	})
}

func TestPrepareMockMetricsHistoryReusesRevisionSeedWithIndependentHistories(t *testing.T) {
	resetMockMetricsSeedCacheForTest(t)

	now := time.Now().UTC().Truncate(time.Minute)
	const (
		fixtureRevision = uint64(41)
		seedDuration    = time.Hour
		sampleInterval  = time.Minute
		maxDataPoints   = 3500
	)
	graph := fixtureGraphWithState(models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:     "vm-cache",
				Status: "running",
				CPU:    0.42,
				Memory: models.Memory{Usage: 55, Total: 1024},
				Disk:   models.Disk{Usage: 61, Total: 1024, Used: 625},
			},
		},
	})

	first, firstCacheHit := prepareMockMetricsHistory(
		graph,
		fixtureRevision,
		now,
		seedDuration,
		sampleInterval,
		maxDataPoints,
		nil,
	)
	if firstCacheHit {
		t.Fatal("first seed unexpectedly reported a cache hit")
	}
	second, secondCacheHit := prepareMockMetricsHistory(
		graph,
		fixtureRevision,
		now,
		seedDuration,
		sampleInterval,
		maxDataPoints,
		nil,
	)
	if !secondCacheHit {
		t.Fatal("second seed did not reuse the matching fixture revision")
	}
	if first == second {
		t.Fatal("cached seed returned shared mutable MetricsHistory pointers")
	}

	firstCPU := first.GetGuestMetrics("vm-cache", "cpu", seedDuration)
	secondCPU := second.GetGuestMetrics("vm-cache", "cpu", seedDuration)
	if len(firstCPU) == 0 || len(firstCPU) != len(secondCPU) {
		t.Fatalf("cached seed coverage mismatch: first=%d second=%d", len(firstCPU), len(secondCPU))
	}

	first.AddGuestMetric("vm-cache", "cpu", 99, now.Add(sampleInterval))
	if got := len(second.GetGuestMetrics("vm-cache", "cpu", seedDuration)); got != len(secondCPU) {
		t.Fatalf("mutating one tenant history changed cached sibling coverage: got=%d want=%d", got, len(secondCPU))
	}

	_, revisionCacheHit := prepareMockMetricsHistory(
		graph,
		fixtureRevision+1,
		now,
		seedDuration,
		sampleInterval,
		maxDataPoints,
		nil,
	)
	if revisionCacheHit {
		t.Fatal("new fixture revision reused a stale seed")
	}
}
