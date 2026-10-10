package unifiedresources

// ManualLinkFold records one operator link that applyManualLinks applied: the
// resource FoldedID names was merged into the one HolderID names. The two IDs
// are the link row's own pair, so an exclusion naming them replaces the link
// (the store keeps one decision per pair). A candidate ID derived from the
// merged resource's type cannot: for an agent linked into a VM it is a vm- ID
// that names neither side. Sources lists what the folded side brought,
// including anything links had already folded into it. HolderOwn and
// FoldedOwn list what each side is on its own: the sources its row listed
// before any link folded something into it. A row that took other members in
// lists theirs too, so Sources alone cannot say which member of a chain a
// source belongs to.
type ManualLinkFold struct {
	HolderID  string
	FoldedID  string
	Sources   []DataSource
	HolderOwn []DataSource
	FoldedOwn []DataSource
	// shapes keeps the two members' link shapes for later link passes
	// (manual_link_chains.go); ManualLinkFolds leaves them out.
	shapes linkFoldShapes
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
//
// Call it before the other side's data is merged into primary: the sources
// primary lists at that point are its own unless an earlier fold on it
// recorded them (linkMemberOwnSources).
func recordManualLinkFold(primary *Resource, primaryID string, other *Resource, otherID string) {
	added := make([]ManualLinkFold, 0, len(other.linkFolds)+1)
	added = append(added, other.linkFolds...)
	added = append(added, ManualLinkFold{
		HolderID:  primaryID,
		FoldedID:  otherID,
		Sources:   cloneDataSourceSlice(other.Sources),
		HolderOwn: linkMemberOwnSources(primary, primaryID),
		FoldedOwn: linkMemberOwnSources(other, otherID),
	})
	folds := make([]ManualLinkFold, 0, len(primary.linkFolds)+len(added))
	recorded := make(map[string]int, len(primary.linkFolds)+len(added))
	record := func(fold ManualLinkFold) {
		key := exclusionKey(fold.HolderID, fold.FoldedID)
		if i, ok := recorded[key]; ok {
			folds[i].Sources = addSources(cloneDataSourceSlice(folds[i].Sources), fold.Sources)
			holderOwn, foldedOwn := fold.HolderOwn, fold.FoldedOwn
			if folds[i].HolderID != fold.HolderID {
				holderOwn, foldedOwn = foldedOwn, holderOwn
			}
			folds[i].HolderOwn = addSources(cloneDataSourceSlice(folds[i].HolderOwn), holderOwn)
			folds[i].FoldedOwn = addSources(cloneDataSourceSlice(folds[i].FoldedOwn), foldedOwn)
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
	folds := cloneManualLinkFolds(resource.linkFolds)
	for i := range folds {
		folds[i].shapes = linkFoldShapes{}
	}
	return folds
}

func cloneManualLinkFolds(in []ManualLinkFold) []ManualLinkFold {
	if in == nil {
		return nil
	}
	out := make([]ManualLinkFold, len(in))
	for i, fold := range in {
		fold.Sources = cloneDataSourceSlice(fold.Sources)
		fold.HolderOwn = cloneDataSourceSlice(fold.HolderOwn)
		fold.FoldedOwn = cloneDataSourceSlice(fold.FoldedOwn)
		out[i] = fold
	}
	return out
}

// linkMemberOwnSources returns what the member id is on its own: the sources
// an earlier fold on row recorded for it, else the sources row lists, which
// are its own while no link has folded anything into it.
func linkMemberOwnSources(row *Resource, id string) []DataSource {
	for _, fold := range row.linkFolds {
		if fold.HolderID == id && fold.HolderOwn != nil {
			return cloneDataSourceSlice(fold.HolderOwn)
		}
		if fold.FoldedID == id && fold.FoldedOwn != nil {
			return cloneDataSourceSlice(fold.FoldedOwn)
		}
	}
	return cloneDataSourceSlice(row.Sources)
}

// ReportedManualLinkFolds picks, from the folds of the merged resource rootID,
// the links a report-merge undoes. reported says whether any of the sources
// it is given was named by the report; a report naming no source reports
// every source.
//
// A report names sources, and the sources belong to link members: the
// resources the folds' pairs name. A member is reported when its own sources
// include a reported one, whatever it took in; the resource being reported on
// is never one, since it is what stays. Reported members leave the part of
// the chain that stays, so a link is undone when it joins a reported member
// to a member that stays or to another reported member. A member that hangs
// below a reported one and carries no reported source goes with it, and the
// link that holds it there stays: the report names a source that is not
// theirs to undo. A member that stays is one the root still reaches without
// passing through a reported member, which matters where links form a cycle:
// a reported member linked both to the root and to a member that stays is cut
// from both, so it cannot rejoin through the other.
//
// A fold's Sources lists its folded side's whole subtree, so selecting folds
// by them undid every link along the way to a reported source: a source
// brought by a leaf split the leaf's holder from the root as well.
func ReportedManualLinkFolds(rootID string, folds []ManualLinkFold, reported func(...DataSource) bool) []ManualLinkFold {
	if len(folds) == 0 {
		return nil
	}
	// A member named by several folds takes the union of what they record: a
	// repeated pair unions into its own fold only, so a member whose refold
	// gained a source would otherwise read differently from fold to fold.
	own := make(map[string][]DataSource, len(folds)+1)
	adjacent := make(map[string][]string, len(folds)+1)
	folded := make(map[string]bool, len(folds))
	for _, fold := range folds {
		if fold.HolderOwn != nil {
			own[fold.HolderID] = addSources(own[fold.HolderID], fold.HolderOwn)
		}
		if fold.FoldedOwn != nil {
			own[fold.FoldedID] = addSources(own[fold.FoldedID], fold.FoldedOwn)
		}
		adjacent[fold.HolderID] = append(adjacent[fold.HolderID], fold.FoldedID)
		adjacent[fold.FoldedID] = append(adjacent[fold.FoldedID], fold.HolderID)
		folded[fold.FoldedID] = true
	}
	// A fold recorded without its sides' own sources lists the folded side's
	// whole subtree, the most that side can have been.
	for _, fold := range folds {
		if _, ok := own[fold.FoldedID]; !ok {
			own[fold.FoldedID] = fold.Sources
		}
	}

	root := rootID
	if _, ok := adjacent[root]; !ok {
		// The root was re-keyed after the links folded: the one holder no
		// link folded is the resource the records hang from.
		root = ""
		for _, fold := range folds {
			if folded[fold.HolderID] {
				continue
			}
			if root != "" && root != fold.HolderID {
				root = ""
				break
			}
			root = fold.HolderID
		}
	}
	if root == "" {
		var selected []ManualLinkFold
		for _, fold := range folds {
			if reported(fold.Sources...) {
				selected = append(selected, fold)
			}
		}
		return selected
	}

	away := make(map[string]bool, len(adjacent))
	for id := range adjacent {
		if id != root && reported(own[id]...) {
			away[id] = true
		}
	}
	stays := map[string]bool{root: true}
	for queue := []string{root}; len(queue) > 0; {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacent[current] {
			if away[next] || stays[next] {
				continue
			}
			stays[next] = true
			queue = append(queue, next)
		}
	}
	carried := func(id string) bool { return !away[id] && !stays[id] }

	var selected []ManualLinkFold
	for _, fold := range folds {
		if (away[fold.HolderID] || away[fold.FoldedID]) && !carried(fold.HolderID) && !carried(fold.FoldedID) {
			selected = append(selected, fold)
		}
	}
	return selected
}
