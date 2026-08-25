package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func resetMockMetricsSeedCacheForTest(t *testing.T) {
	t.Helper()
	mockMetricsSeedCache.Lock()
	previousKey := mockMetricsSeedCache.key
	previousHistory := mockMetricsSeedCache.history
	previousGeneration := mockMetricsSeedCache.generation
	mockMetricsSeedCache.generation++
	mockMetricsSeedCache.key = mockMetricsSeedCacheKey{}
	mockMetricsSeedCache.history = nil
	mockMetricsSeedCache.Unlock()
	t.Cleanup(func() {
		mockMetricsSeedCache.Lock()
		mockMetricsSeedCache.generation++
		mockMetricsSeedCache.key = previousKey
		mockMetricsSeedCache.history = previousHistory
		if previousHistory != nil {
			mockMetricsSeedCache.generation = previousGeneration
		}
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

	mockMetricsSeedCache.Lock()
	firstGeneration := mockMetricsSeedCache.generation
	mockMetricsSeedCache.Unlock()

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
	mockMetricsSeedCache.Lock()
	replacementGeneration := mockMetricsSeedCache.generation
	mockMetricsSeedCache.Unlock()

	expireMockMetricsSeedCache(firstGeneration)
	mockMetricsSeedCache.Lock()
	replacementRetained := mockMetricsSeedCache.history != nil && mockMetricsSeedCache.key.fixtureRevision == fixtureRevision+1
	mockMetricsSeedCache.Unlock()
	if !replacementRetained {
		t.Fatal("stale seed expiry cleared a newer cached fixture revision")
	}

	expireMockMetricsSeedCache(replacementGeneration)
	mockMetricsSeedCache.Lock()
	cacheReleased := mockMetricsSeedCache.history == nil
	mockMetricsSeedCache.Unlock()
	if !cacheReleased {
		t.Fatal("seed template remained cached after its startup reuse window expired")
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

}

func TestPrepareMockMetricsHistoryBoundsFullDemoEstateBeforeCaching(t *testing.T) {
	resetMockMetricsSeedCacheForTest(t)

	cfg := mock.DefaultConfig
	cfg.NodeCount = 50
	cfg.VMsPerNode = 10
	cfg.LXCsPerNode = 8

	previousEnabled := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	t.Cleanup(func() {
		mustSetMockEnabled(t, false)
		mock.SetMockConfig(previousConfig)
		if previousEnabled {
			mustSetMockEnabled(t, true)
		}
	})
	mustSetMockEnabled(t, false)
	mock.SetMockConfig(cfg)
	mustSetMockEnabled(t, true)

	full := mock.CurrentFixtureGraph()
	fullPVEGuests := make(map[string]struct{}, len(full.State.VMs)+len(full.State.Containers))
	for _, vm := range full.State.VMs {
		fullPVEGuests[vm.ID] = struct{}{}
	}
	for _, container := range full.State.Containers {
		fullPVEGuests[container.ID] = struct{}{}
	}
	if len(fullPVEGuests) <= mockEagerHistoryPVEGuestLimit {
		t.Fatalf("fixture contains %d PVE guests, want more than eager limit %d", len(fullPVEGuests), mockEagerHistoryPVEGuestLimit)
	}

	now := time.Now().UTC().Truncate(time.Minute)
	history, cacheHit := prepareMockMetricsHistory(full, 99, now, time.Hour, time.Minute, 3500, nil)
	if cacheHit {
		t.Fatal("first full-estate seed unexpectedly reported a cache hit")
	}

	countEagerPVEGuests := func(name string, candidate *MetricsHistory) int {
		t.Helper()
		candidate.mu.RLock()
		defer candidate.mu.RUnlock()
		count := 0
		for id := range candidate.guestMetrics {
			if _, exists := fullPVEGuests[id]; exists {
				count++
			}
		}
		if count != mockEagerHistoryPVEGuestLimit {
			t.Fatalf("%s eagerly retained %d PVE guests, want %d", name, count, mockEagerHistoryPVEGuestLimit)
		}
		return count
	}

	countEagerPVEGuests("active history", history)
	mockMetricsSeedCache.Lock()
	cached := mockMetricsSeedCache.history
	mockMetricsSeedCache.Unlock()
	if cached == nil {
		t.Fatal("full-estate seed did not populate the reusable cache")
	}
	countEagerPVEGuests("cached template", cached)
}
