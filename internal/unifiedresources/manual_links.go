package unifiedresources

import "slices"

// NewRegistryWithManualLinks creates a registry that applies the given
// operator links on every ingest but has no store behind it. Read views built
// from data that never passed through the durable registry, such as the mock
// fixture graph, use it to show the operator's links the way the store-backed
// registry does, while nothing they ingest can persist identity pins,
// canonical-ID successions or change records. It carries no exclusions, so
// its identity matching and ListForPresentation see none.
func NewRegistryWithManualLinks(links []ResourceLink) *ResourceRegistry {
	rr := NewRegistry(nil)
	rr.links = slices.Clone(links)
	return rr
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
