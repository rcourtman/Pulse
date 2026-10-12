package unifiedresources

import "slices"

// NewRegistryWithManualLinks creates a storeless registry with explicit
// operator links. MonitorAdapter builds each generation using the same link,
// source-order and freshness boundary as the live monitor. No identity pins,
// canonical-ID successions or change records are persisted.
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
