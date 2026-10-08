package unifiedresources

// ManualLinkFold records one operator link that applyManualLinks applied: the
// resource FoldedID names was merged into the one HolderID names. The two IDs
// are the link row's own pair, so an exclusion naming them replaces the link
// (the store keeps one decision per pair). A candidate ID derived from the
// merged resource's type cannot: for an agent linked into a VM it is a vm- ID
// that names neither side. Sources lists what the folded side brought,
// including anything links had already folded into it.
type ManualLinkFold struct {
	HolderID string
	FoldedID string
	Sources  []DataSource
}

// recordManualLinkFold notes on the link's merged resource that it took in
// other through the link pair (primaryID, otherID), together with the folds
// other already held, so each link along a chain keeps its own pair. The
// record is unexported and rides in-memory clones, so a registry seeded from
// another registry's listing, as the resources API's is, sees the folds the
// monitor applied although it never holds the folded rows itself. A pair is
// recorded once: record ingest can recreate a folded side whose source
// mapping points at a holder of another type and fold it again, possibly
// with fewer sources than it brought before, so a repeated pair adds its
// sources to the fold already recorded. The holder still carries what the
// earlier fold brought.
func recordManualLinkFold(primary *Resource, primaryID string, other *Resource, otherID string) {
	added := make([]ManualLinkFold, 0, len(other.linkFolds)+1)
	added = append(added, other.linkFolds...)
	added = append(added, ManualLinkFold{
		HolderID: primaryID,
		FoldedID: otherID,
		Sources:  cloneDataSourceSlice(other.Sources),
	})
	folds := make([]ManualLinkFold, 0, len(primary.linkFolds)+len(added))
	recorded := make(map[string]int, len(primary.linkFolds)+len(added))
	record := func(fold ManualLinkFold) {
		key := exclusionKey(fold.HolderID, fold.FoldedID)
		if i, ok := recorded[key]; ok {
			folds[i].Sources = addSources(cloneDataSourceSlice(folds[i].Sources), fold.Sources)
			return
		}
		recorded[key] = len(folds)
		folds = append(folds, fold)
	}
	for _, fold := range primary.linkFolds {
		record(fold)
	}
	for _, fold := range added {
		record(fold)
	}
	primary.linkFolds = folds
}

// ManualLinkFolds lists the operator links folded into a resource, one per
// link pair: the resource's own earlier folds first, then each folded side's
// folds ahead of the link that took that side in.
func (rr *ResourceRegistry) ManualLinkFolds(resourceID string) []ManualLinkFold {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	resource := rr.resources[CanonicalResourceID(resourceID)]
	if resource == nil {
		return nil
	}
	return cloneManualLinkFolds(resource.linkFolds)
}

func cloneManualLinkFolds(in []ManualLinkFold) []ManualLinkFold {
	if in == nil {
		return nil
	}
	out := make([]ManualLinkFold, len(in))
	for i, fold := range in {
		fold.Sources = cloneDataSourceSlice(fold.Sources)
		out[i] = fold
	}
	return out
}
