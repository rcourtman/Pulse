package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// useMockEstate enables the default mock estate with the given node count,
// whose fixture ticks every interval, and restores the previous mock mode and
// configuration when the test ends.
func useMockEstate(tb testing.TB, nodes int, interval time.Duration) {
	tb.Helper()
	previous := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	if previous {
		mustSetMockEnabled(tb, false)
	}
	// Start from the default estate, not the ambient one: PULSE_MOCK_* settings
	// can remove the standalone agents and clusters these tests look for.
	config := mock.DefaultConfig
	config.NodeCount = nodes
	config.UpdateInterval = interval
	mock.SetMockConfig(config)
	mustSetMockEnabled(tb, true)
	tb.Cleanup(func() {
		mustSetMockEnabled(tb, false)
		mock.SetMockConfig(previousConfig)
		mustSetMockEnabled(tb, previous)
	})
}

func newMockEstateMonitor() *Monitor {
	return &Monitor{
		state:         models.NewState(),
		resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())),
	}
}

// publishedMockView returns the view the monitor currently caches for mock
// mode without building one.
func publishedMockView(m *Monitor) (monitorUnifiedStateView, bool) {
	m.mockUnifiedViewMu.Lock()
	defer m.mockUnifiedViewMu.Unlock()
	return m.mockUnifiedView, m.mockUnifiedViewValid
}

// sameMetricsTarget reports whether two lookups resolved the same target (or
// both resolved none).
func sameMetricsTarget(a, b *unifiedresources.MetricsTarget) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func waitForFixtureTicks(tb testing.TB, ticks uint64) {
	tb.Helper()
	target := mock.FixtureDataVersion() + ticks
	// A starved host ticks slowly; the deadline only catches a ticker that
	// is not running at all.
	deadline := time.Now().Add(5 * time.Minute)
	for mock.FixtureDataVersion() < target {
		if time.Now().After(deadline) {
			tb.Fatalf("fixture data version stayed below %d (at %d) for five minutes; the mock ticker is not running", target, mock.FixtureDataVersion())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func resourceIDsWithTargets(tb testing.TB, view monitorUnifiedStateView) []string {
	tb.Helper()
	ids := make([]string, 0, len(view.resources))
	for _, resource := range view.resources {
		if view.metricsTargets.MetricsTargetForResource(resource.ID) != nil {
			ids = append(ids, resource.ID)
		}
	}
	if len(ids) == 0 {
		tb.Fatal("expected the mock estate to list resources with metrics targets")
	}
	return ids
}

// BenchmarkMockMetricsTargetResolutionAfterTick measures one alert pass's
// lookups (every resource, three windowed metrics) against a view that was
// built before the fixture last ticked. It must stay a map read per lookup, so
// a pass costs microseconds however slowly the process runs, not an estate
// build per lookup.
func BenchmarkMockMetricsTargetResolutionAfterTick(b *testing.B) {
	useMockEstate(b, mock.DefaultConfig.NodeCount, time.Second)
	m := newMockEstateMonitor()
	ids := resourceIDsWithTargets(b, m.currentUnifiedStateView())
	waitForFixtureTicks(b, 1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range ids {
			for metric := 0; metric < 3; metric++ {
				if m.MetricsTargetForResource(id) == nil {
					b.Fatalf("no target for %q", id)
				}
			}
		}
	}
	b.ReportMetric(float64(len(ids)*3), "lookups/op")
}

// BenchmarkLiveRegistryMetricsTargetResolution measures the same pass against a
// store-backed registry, as real mode resolves it, right after an ingest (the
// registry's typed views not yet built) and once they are.
func BenchmarkLiveRegistryMetricsTargetResolution(b *testing.B) {
	useMockEstate(b, mock.DefaultConfig.NodeCount, 5*time.Minute)
	graph := mock.CurrentFixtureGraph()
	snapshot := unifiedresources.SnapshotWithoutSources(graph.State, mock.SupplementalOwnedSources())
	mustSetMockEnabled(b, false)

	for _, views := range []string{"after-ingest", "views-built"} {
		b.Run(views, func(b *testing.B) {
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(snapshot)
			adapter := unifiedresources.NewMonitorAdapter(registry)
			m := &Monitor{state: models.NewState(), resourceStore: adapter}

			listed := registry.List()
			ids := make([]string, 0, len(listed))
			for _, resource := range listed {
				ids = append(ids, resource.ID)
			}
			if views == "views-built" {
				adapter.VMs()
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if views == "after-ingest" {
					b.StopTimer()
					registry = unifiedresources.NewRegistry(nil)
					registry.IngestSnapshot(snapshot)
					adapter = unifiedresources.NewMonitorAdapter(registry)
					m.resourceStore = adapter
					b.StartTimer()
				}
				for _, id := range ids {
					for metric := 0; metric < 3; metric++ {
						m.MetricsTargetForResource(id)
					}
				}
			}
			b.ReportMetric(float64(len(ids)*3), "lookups/op")
		})
	}
}
