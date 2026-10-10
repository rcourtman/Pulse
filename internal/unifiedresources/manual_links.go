package unifiedresources

import (
	"slices"
	"time"
)

// NewRegistryWithManualLinks creates a registry that applies the given
// operator links on every ingest but has no store behind it. Read views built
// from data that never passed through the durable registry, such as the mock
// fixture graph, use it to show the operator's links the way the store-backed
// registry does, while nothing they ingest can persist identity pins,
// canonical-ID successions or change records. It carries no exclusions, so
// its identity matching and ListForPresentation see none. A builder that
// ingests the snapshot and several record sources in turn calls
// DeferManualLinks first, so the links are judged once over the whole estate
// as the monitor's rebuild does.
func NewRegistryWithManualLinks(links []ResourceLink) *ResourceRegistry {
	rr := NewRegistry(nil)
	rr.links = slices.Clone(links)
	return rr
}

// DeferManualLinks holds the link pass back from this registry's snapshot and
// record ingests until ApplyDeferredManualLinks, the boundary the monitor's
// rebuild draws (MonitorAdapter.replaceRegistryLocked): a chain of links is
// judged over every member's source at once, because a fold cannot be undone
// when a later source brings a member that outranks the earlier choice. The
// caller ingests availability checks after ApplyDeferredManualLinks, so a
// check linked to a folded resource projects onto its primary. A registry with
// no links has no pass to hold back and ingests as it always did.
func (rr *ResourceRegistry) DeferManualLinks() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if len(rr.links) == 0 {
		return
	}
	rr.deferManualLinks = true
}

// ApplyDeferredManualLinks runs the link pass DeferManualLinks held back, then
// refreshes what folding changes. Freshness is judged by thresholds, which the
// caller passes as it passed them to the ingests (nil takes the registry's own,
// as IngestSnapshot and IngestRecords do): a source whose reading is stale
// under one set can be current under the other, and the merge keeps the
// current one. It does nothing once the pass has run, or on a registry that
// was not deferring, so a builder may call it at every boundary without
// tracking whether the pass already ran.
func (rr *ResourceRegistry) ApplyDeferredManualLinks(thresholds map[DataSource]time.Duration) {
	rr.applyDeferredManualLinks(rr.thresholdsOrOwn(thresholds))
}

// ManualLinks returns the operator's manual links that the adapter's current
// registry generation applied, as loaded from its store when that generation
// was built. A link added to the store since then appears after the next
// rebuild, which is when the adapter's own listing folds it too.
func (a *MonitorAdapter) ManualLinks() []ResourceLink {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return slices.Clone(registry.links)
}
