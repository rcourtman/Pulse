package unifiedresources

import "time"

// presentationExclusionFilter reports whether the presentation host coalesce
// must keep two resources apart because the operator split the pair (unlink
// or report-merge): an exclusion names the two rows, or they are a Proxmox
// node and an agent split across the IDs they can hold (nodeAgentSplit). It
// reads a copy of the registry's decisions, so callers coalesce without
// holding the registry lock; nil means no pair is excluded.
func (rr *ResourceRegistry) presentationExclusionFilter() func(left, right Resource) bool {
	rr.mu.RLock()
	if len(rr.exclusions) == 0 {
		rr.mu.RUnlock()
		return nil
	}
	// The link index is built once per registry and never written again.
	decisions := operatorPairDecisions{
		exclusions: make(map[string]time.Time, len(rr.exclusions)),
		linksByID:  rr.linksByID,
	}
	for key, at := range rr.exclusions {
		decisions.exclusions[key] = at
	}
	rr.mu.RUnlock()

	return func(left, right Resource) bool {
		leftID := CanonicalResourceID(left.ID)
		rightID := CanonicalResourceID(right.ID)
		if leftID == "" || rightID == "" {
			return false
		}
		if _, ok := decisions.exclusions[exclusionKey(leftID, rightID)]; ok {
			return true
		}
		return rr.nodeAgentRowsSplit(decisions, &left, &right)
	}
}

// CoalesceForPresentation applies the presentation host coalesce to resources
// listed from this adapter's current registry generation, honouring the
// operator's merge exclusions through the same filter that
// ResourceRegistry.ListForPresentation uses, so the websocket broadcast keeps
// a pair the operator split apart as the resources API does. It reports
// false, leaving resources untouched, when the registry has no store and so
// carries no operator decisions: the mock view and other read states built
// from an already-unified list.
func (a *MonitorAdapter) CoalesceForPresentation(resources []Resource) ([]Resource, bool) {
	registry := a.currentRegistry()
	if registry == nil || registry.store == nil {
		return resources, false
	}
	return CoalescePresentationHostResourcesWithExclusions(resources, registry.presentationExclusionFilter()), true
}
