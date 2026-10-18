package monitoring

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// cancelAfterChecks reports cancellation once Err has been consulted more
// than allowed times, so a test can cancel seeding partway through without
// racing a timer.
type cancelAfterChecks struct {
	context.Context
	remaining atomic.Int32
}

func newCancelAfterChecks(allowed int32) *cancelAfterChecks {
	ctx := &cancelAfterChecks{Context: context.Background()}
	ctx.remaining.Store(allowed)
	return ctx
}

func (c *cancelAfterChecks) Err() error {
	if c.remaining.Add(-1) < 0 {
		return context.Canceled
	}
	return nil
}

func threeNodeMockGraph() mock.FixtureGraph {
	return fixtureGraphWithState(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-a", Name: "node-a", Status: "online", CPU: 0.2, Memory: models.Memory{Usage: 40, Total: 1024}},
			{ID: "node-b", Name: "node-b", Status: "online", CPU: 0.3, Memory: models.Memory{Usage: 50, Total: 1024}},
			{ID: "node-c", Name: "node-c", Status: "online", CPU: 0.4, Memory: models.Memory{Usage: 60, Total: 1024}},
		},
	})
}

// Org deletion refused with tenant_shutdown_incomplete because a tenant
// monitor's loop only sees its cancelled context after mock seeding returns,
// and seeding a large demo estate took 4-11s in CI. Seeding must stop at the
// next resource instead of finishing the estate.
func TestSeedMockMetricsHistoryStopsPartwayWhenCancelled(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	mh := NewMetricsHistory(3500, time.Hour)

	err := seedMockMetricsHistory(newCancelAfterChecks(1), mh, nil, threeNodeMockGraph(), now, time.Hour, time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("seedMockMetricsHistory error = %v, want context.Canceled", err)
	}
	if len(mh.GetNodeMetrics("node-a", "cpu", time.Hour)) == 0 {
		t.Fatal("node seeded before cancellation has no history")
	}
	for _, nodeID := range []string{"node-b", "node-c"} {
		if got := len(mh.GetNodeMetrics(nodeID, "cpu", time.Hour)); got != 0 {
			t.Fatalf("node %s seeded %d points after cancellation", nodeID, got)
		}
	}
}

func TestPrepareMockMetricsHistoryDoesNotCacheCancelledSeed(t *testing.T) {
	resetMockMetricsSeedCacheForTest(t)
	now := time.Now().UTC().Truncate(time.Minute)
	graph := threeNodeMockGraph()

	// One check passes the cache lock, one seeds node-a, then cancellation.
	history, cacheHit, err := prepareMockMetricsHistory(newCancelAfterChecks(2), graph, 7, now, time.Hour, time.Minute, 3500, nil)
	if !errors.Is(err, context.Canceled) || history != nil || cacheHit {
		t.Fatalf("cancelled prepare = (%v, %v, %v), want (nil, false, context.Canceled)", history, cacheHit, err)
	}
	mockMetricsSeedCache.Lock()
	cached := mockMetricsSeedCache.history
	mockMetricsSeedCache.Unlock()
	if cached != nil {
		t.Fatal("cancelled seed left a partial template for other tenants to reuse")
	}

	history, cacheHit, err = prepareMockMetricsHistory(context.Background(), graph, 7, now, time.Hour, time.Minute, 3500, nil)
	if err != nil || cacheHit {
		t.Fatalf("seed after cancellation = (cacheHit %v, err %v), want a fresh seed", cacheHit, err)
	}
	if len(history.GetNodeMetrics("node-c", "cpu", time.Hour)) == 0 {
		t.Fatal("fresh seed after cancellation is incomplete")
	}
}

func TestStartMockMetricsSamplerSkipsSeedingWhenMonitorStopping(t *testing.T) {
	setMockSamplerTestEnv(t, time.Hour, time.Minute)
	resetMockMetricsSeedCacheForTest(t)
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
	mock.SetMockConfig(compactMockFixtureConfig())
	mustSetMockEnabled(t, true)

	nodes := mock.CurrentFixtureGraph().State.Nodes
	if len(nodes) == 0 {
		t.Fatal("expected mock fixture nodes")
	}
	monitor := &Monitor{
		metricsHistory: NewMetricsHistory(1000, time.Hour),
		state:          models.NewState(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	monitor.startMockMetricsSampler(ctx)
	t.Cleanup(monitor.stopMockMetricsSampler)

	if got := len(monitor.metricsHistory.GetNodeMetrics(nodes[0].ID, "cpu", time.Hour)); got != 0 {
		t.Fatalf("stopping monitor seeded %d points for node %s", got, nodes[0].ID)
	}
}
