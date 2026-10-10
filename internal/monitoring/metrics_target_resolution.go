package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// mockMetricsTarget resolves a canonical resource ID to its metrics-store
// target in mock mode. The second result is false when mock mode is off, so
// the caller takes its ordinary path; when it is true the answer is final for
// this structure, a nil target meaning the view captured no metrics target for
// the ID (the estate does not list it, or lists a resource that has none, such
// as a VMware network); the caller still tries the raw resource store.
//
// Alert evaluation resolves a target for every resource and every windowed
// metric (CPU by default) on every pass, through MetricsTargetForResource.
// The mock view that answers it is cached per fixture data version, and that
// version advances on every metric tick (2 s by default). Rebuilding the view
// ingests the whole estate twice (the memoized fixture snapshot, then the
// view's own registry), about half a second of CPU for the default estate
// (1,776 resources) on an idle core. A build that takes longer than one tick publishes
// its view under an already-stale version, so the next lookup rebuilds again:
// on a host with CPU to spare that costs one build per tick, but when the
// process is starved (a loaded dev VM, a small demo host) the cache never
// hits, every lookup in the pass builds an estate, and the monitor never
// finishes its first evaluation. Startup stalled in exactly that state,
// before the listener opened, with the watchdog dump parked in the view build
// beneath evaluateMetricWindow.
//
// A metrics target depends only on identity inputs: a resource's type, its
// source mappings, and the technology, Docker container ID and disk identity
// of its facets (unifiedresources.BuildMetricsTarget), falling back to the
// resource's own MetricsTarget. A metric tick moves values and timestamps,
// never those inputs (TestMetricTicksNeverChangeTheFixtureStructure), so the
// captured targets of a view built at any data version of the current
// structure answer the lookup. A data-version change alone therefore never
// triggers a build here: only a structural change (mock mode toggled, the
// fixture reconfigured) or a link change makes the next lookup rebuild the
// view. Two lookups that miss together can each still build one, and a cold
// cache builds even for an ID the estate does not list.
func (m *Monitor) mockMetricsTarget(resourceID string) (*unifiedresources.MetricsTarget, bool) {
	if m == nil || !mock.IsMockEnabled() {
		return nil, false
	}

	view := m.currentStructureUnifiedStateView()
	if view.metricsTargets == nil {
		return nil, true
	}
	return view.metricsTargets.MetricsTargetForResource(resourceID), true
}
