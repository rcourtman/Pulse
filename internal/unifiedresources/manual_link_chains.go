package unifiedresources

import (
	"maps"
	"slices"
	"strings"
	"time"
)

// Operator links apply as chains. Links that share a member name one
// identity, so a link pass folds every connected set of observed members into
// one row, chosen by an explicit precedence (linkChainRoot), whatever order
// the store lists the links in. Applying the links one at a time made the
// result depend on that order: once an earlier link folded a member away, a
// later link naming it found nothing and left its other side standing, and
// standing in the row holding the folded member for it let that row outrank
// the link's own primary and lose the agent-into-guest direction.
//
// Links are judged member by member. A member is the resource a link names:
// its own row, or the row an earlier pass of the same rebuild, or the monitor
// a registry was seeded from, already folded it into (linkMemberLocked).

// linkMemberShape is what link precedence reads from a link member: whether
// it is an agent, and whether a hypervisor platform accounts it as a guest.
type linkMemberShape struct {
	agent bool
	guest bool
}

func linkShapeOfResource(resource *Resource) linkMemberShape {
	return linkMemberShape{agent: resource.Type == ResourceTypeAgent, guest: hypervisorManagedGuest(resource)}
}

// linkFoldShapes keeps the shapes of a fold record's two members as the link
// pass judged them, before either took in the other. A row that takes in a
// guest also takes its Proxmox payload and would read as a guest itself, and
// a folded member has no row of its own, so a later pass of the same rebuild
// and a registry seeded from a listing read the shapes from the record.
type linkFoldShapes struct {
	recorded bool
	holder   linkMemberShape
	folded   linkMemberShape
}

func (s linkFoldShapes) turned() linkFoldShapes {
	return linkFoldShapes{recorded: s.recorded, holder: s.folded, folded: s.holder}
}

// linkMember is one resource a stored link names, as the pass finds it.
type linkMember struct {
	id    string
	row   *Resource
	shape linkMemberShape
	saved bool
}

// linkEdge is a stored link in its effective direction: other folds into,
// or is held under, the primary's side.
type linkEdge struct {
	primary   linkMember
	other     linkMember
	createdAt time.Time
}

// live reports whether both members are observed. A link with a member that
// exists only through saved-host continuity never merges anything.
func (e linkEdge) live() bool {
	return !e.primary.saved && !e.other.saved
}

// linkMemberLocked finds the resource a link names: its own listed row, which
// answers for itself, or else the one row a link folded it into. A member
// that is neither, or that two rows hold, takes no part in the pass.
func (rr *ResourceRegistry) linkMemberLocked(id string) (linkMember, bool) {
	row := rr.resources[id]
	if row == nil {
		holderID, ambiguous := rr.linkFoldHolderLocked(id)
		if holderID == "" || ambiguous {
			return linkMember{}, false
		}
		row = rr.resources[holderID]
		if row == nil {
			return linkMember{}, false
		}
	}
	shape, recorded := recordedLinkMemberShape(row, CanonicalResourceID(id))
	if !recorded && row.ID == id {
		shape = linkShapeOfResource(row)
	}
	return linkMember{id: id, row: row, shape: shape, saved: row.continuityOnly}, true
}

func recordedLinkMemberShape(row *Resource, id string) (linkMemberShape, bool) {
	for _, fold := range row.linkFolds {
		if !fold.shapes.recorded {
			continue
		}
		if fold.FoldedID == id {
			return fold.shapes.folded, true
		}
		if fold.HolderID == id {
			return fold.shapes.holder, true
		}
	}
	return linkMemberShape{}, false
}

// manualLinkEdgesLocked resolves every stored link the pass can apply, in a
// canonical order so nothing downstream depends on the store's.
func (rr *ResourceRegistry) manualLinkEdgesLocked() []linkEdge {
	rr.recordJoinedPairFoldsLocked()
	edges := make([]linkEdge, 0, len(rr.links))
	decisions := rr.operatorPairDecisionsLocked()
	for _, link := range rr.links {
		// A link joins only the pair its row names. Canonical-ID succession
		// can leave a row whose endpoint re-key collided with the successor's
		// own row while its primary moved on to the successor; honouring that
		// primary merged the successor through a row naming a retired ID, which
		// an unlink or report-merge of the merged pair leaves behind.
		primaryID := link.PrimaryID
		if primaryID != link.ResourceA && primaryID != link.ResourceB {
			primaryID = link.ResourceA
		}
		otherID := link.ResourceB
		if otherID == primaryID {
			otherID = link.ResourceA
		}
		if otherID == primaryID {
			continue
		}
		primary, primaryFound := rr.linkMemberLocked(primaryID)
		other, otherFound := rr.linkMemberLocked(otherID)
		if !primaryFound || !otherFound {
			continue
		}
		// Availability checks retain their source-owned identity even when an
		// operator links them to another resource. Correlation is represented
		// by RelChecks plus the additive facet projection.
		if isAvailabilityOwnedResource(*primary.row) || isAvailabilityOwnedResource(*other.row) {
			continue
		}
		// A link between a Proxmox node and an agent loses to a newer split
		// of the two read across their IDs: report-merge cannot delete a
		// link naming the rows an earlier split left (nodeAgentSplit).
		if rr.nodeAgentRowsSplit(decisions, primary.row, other.row) {
			continue
		}
		// A manual link records operator intent to unify identities, but its
		// direction must not change the semantic resource shape. In particular,
		// an agent running inside a VM/LXC supplements that guest; it does not
		// turn the guest into an agent resource with an agent metrics target.
		// The members' own shapes decide, never the rows holding them.
		if primary.shape.agent && other.shape.guest {
			primary, other = other, primary
		}
		edges = append(edges, linkEdge{primary: primary, other: other, createdAt: link.CreatedAt})
	}
	slices.SortFunc(edges, func(a, b linkEdge) int {
		if c := strings.Compare(a.primary.id, b.primary.id); c != 0 {
			return c
		}
		return strings.Compare(a.other.id, b.other.id)
	})
	return edges
}

// recordJoinedPairFoldsLocked records, for every link with exactly one member
// no row holds, the fold of that member on a joined Proxmox node+agent row
// that holds the other member: a relink of a split node and its agent names
// the two rows, and the declared link folds one of them into the other's ID
// once the relink rejoins them. Recording the fold keeps pin succession from
// re-keying the link onto the joined ID, which would let the split decide
// again, and lets report-merge of the joined row name this pair. It runs
// before any link is resolved to its members, because a recorded fold lets
// the absent ID resolve and other links naming it join the chain, whatever
// order the store lists them in; a recorded fold can in turn make another
// link's one absent member resolve, so it repeats until none is left (a fold
// the row already holds is not recorded again, so an ambiguous holder cannot
// keep it going).
func (rr *ResourceRegistry) recordJoinedPairFoldsLocked() {
	for recorded := true; recorded; {
		recorded = false
		for _, link := range rr.links {
			a, aFound := rr.linkMemberLocked(link.ResourceA)
			b, bFound := rr.linkMemberLocked(link.ResourceB)
			if aFound == bFound {
				continue
			}
			present, absentID := a, link.ResourceB
			if bFound {
				present, absentID = b, link.ResourceA
			}
			if present.id != present.row.ID || holdsLinkFold(present.row, absentID) {
				continue
			}
			source, ok := rr.joinedPairSideLocked(present.row, absentID)
			if !ok {
				continue
			}
			recordManualLinkFold(present.row, present.id, &Resource{Sources: []DataSource{source}}, absentID)
			// Both sides of a joined node and agent are agent-type rows, so
			// the absent side keeps the agent shape a guest linked to it is
			// judged against.
			stampManualLinkFoldShapes(present.row, present.id, absentID, present.shape, linkMemberShape{agent: true})
			rr.indexLinkFoldsLocked(present.row)
			recorded = true
		}
	}
}

// linkChainRootLocked picks the row a connected set of linked members folds
// into and returns that row's own member. The candidates are the members no
// link folds (each link's effective primary, never its other side), or every
// member when the links' directions form a cycle. Rows compete, each by its
// own identity: a member an earlier pass of the rebuild already folded stands
// as the row holding it, because a fold cannot be undone, so the row's own
// identity is what would survive. When every candidate row is an agent and the
// chain holds a guest, the rows holding guests compete instead, including a
// row an earlier fold put a guest into: an agent never ends up holding a
// guest, as the pair rule already guarantees for one link. Among candidate
// rows:
//  1. an observed row before a saved one (holds only);
//  2. a hypervisor-managed guest, the most specific identity a chain can
//     reach, as the pair rule already decides between an agent and its guest;
//  3. any other resource before an agent, which supplements the resource it
//     runs on;
//  4. the primary of the earliest-created link, so the operator's first
//     decision stands;
//  5. the lowest canonical ID.
//
// A single link keeps its effective primary, and so does a chain whose
// directions agree unless that primary is an agent and the chain holds a
// guest, whichever order the links are stored in.
func (rr *ResourceRegistry) linkChainRootLocked(members map[string]linkMember, edges []linkEdge) linkMember {
	folded := make(map[string]bool, len(edges))
	firstLinked := make(map[string]time.Time, len(edges))
	for _, edge := range edges {
		folded[edge.other.id] = true
		if at, ok := firstLinked[edge.primary.id]; !ok || edge.createdAt.Before(at) {
			firstLinked[edge.primary.id] = edge.createdAt
		}
	}
	owners := make(map[string]linkMember, len(members))
	ownerOf := func(member linkMember) linkMember {
		owner, ok := owners[member.row.ID]
		if !ok {
			owner = rr.linkRowOwnerLocked(member.row, members)
			owners[member.row.ID] = owner
		}
		return owner
	}
	collect := func(include func(linkMember) bool) []linkMember {
		var rows []linkMember
		seen := make(map[string]bool, len(members))
		for _, member := range members {
			owner := ownerOf(member)
			if seen[owner.id] || !include(member) {
				continue
			}
			seen[owner.id] = true
			rows = append(rows, owner)
		}
		return rows
	}
	candidates := collect(func(member linkMember) bool { return !folded[member.id] })
	if len(candidates) == 0 {
		candidates = collect(func(linkMember) bool { return true })
	}
	if !slices.ContainsFunc(candidates, func(row linkMember) bool { return !row.shape.agent }) {
		guests := collect(func(member linkMember) bool { return member.shape.guest || ownerOf(member).shape.guest })
		if len(guests) > 0 {
			candidates = guests
		}
	}
	return bestLinkRoot(candidates, firstLinked)
}

// linkRowOwnerLocked returns the member a row is: its own canonical ID, with
// the shape the row had before any link folded something into it.
func (rr *ResourceRegistry) linkRowOwnerLocked(row *Resource, members map[string]linkMember) linkMember {
	if member, ok := members[row.ID]; ok && member.row == row {
		return member
	}
	shape, recorded := recordedLinkMemberShape(row, row.ID)
	if !recorded {
		shape = linkShapeOfResource(row)
	}
	return linkMember{id: row.ID, row: row, shape: shape, saved: row.continuityOnly}
}

func bestLinkRoot(candidates []linkMember, firstLinked map[string]time.Time) linkMember {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if linkRootBefore(candidate, best, firstLinked) {
			best = candidate
		}
	}
	return best
}

func linkRootBefore(a, b linkMember, firstLinked map[string]time.Time) bool {
	if a.saved != b.saved {
		return !a.saved
	}
	if a.shape.guest != b.shape.guest {
		return a.shape.guest
	}
	if a.shape.agent != b.shape.agent {
		return !a.shape.agent
	}
	aAt, aLinked := firstLinked[a.id]
	bAt, bLinked := firstLinked[b.id]
	if aLinked != bLinked {
		return aLinked
	}
	if aLinked && !aAt.Equal(bAt) {
		return aAt.Before(bAt)
	}
	return a.id < b.id
}

// foldLinkChainsLocked folds each connected set of observed link members into
// its root row.
func (rr *ResourceRegistry) foldLinkChainsLocked(edges []linkEdge, thresholds map[DataSource]time.Duration) {
	// Rows join through links between their members; members a row already
	// holds belong to it even when no stored link names them both any more.
	sets := make(map[string]string)
	var find func(string) string
	find = func(id string) string {
		parent, ok := sets[id]
		if !ok || parent == id {
			sets[id] = id
			return id
		}
		root := find(parent)
		sets[id] = root
		return root
	}
	for _, edge := range edges {
		if !edge.live() {
			continue
		}
		a, b := find(edge.primary.row.ID), find(edge.other.row.ID)
		if a == b {
			continue
		}
		if b < a {
			a, b = b, a
		}
		sets[b] = a
	}
	chains := make(map[string][]linkEdge)
	for _, edge := range edges {
		if edge.live() {
			key := find(edge.primary.row.ID)
			chains[key] = append(chains[key], edge)
		}
	}

	for _, key := range slices.Sorted(maps.Keys(chains)) {
		chain := chains[key]
		members := make(map[string]linkMember, len(chain)+1)
		for _, edge := range chain {
			members[edge.primary.id] = edge.primary
			members[edge.other.id] = edge.other
		}
		root := rr.linkChainRootLocked(members, chain)
		rr.foldLinkChainLocked(members, chain, root.row, thresholds)
	}
}

// foldLinkChainLocked merges every row of one chain into the root's row. Rows
// fold along the chain's links, deepest first, so each fold record names a
// stored link's own pair and lists what its folded side brought, including
// what that side had already taken in. A row joins only through a link naming
// its own ID, or a member whose fold records can be turned to hang from it
// (rehangManualLinkFolds); a row neither reaches, such as one re-keyed after
// it took the member in, stays standing, since folding it would leave its own
// ID resolving nowhere.
func (rr *ResourceRegistry) foldLinkChainLocked(members map[string]linkMember, edges []linkEdge, root *Resource, thresholds map[DataSource]time.Duration) {
	type rowLink struct {
		to       *Resource
		near     string // the member on this row's side
		far      string // the member on the neighbouring row's side
		recorded bool   // a fold record on either row says this link folded far in
	}
	// Every fold record any row of the chain carries, in the direction it
	// folded. A link counts as having taken its far member in only when a
	// record says so in that direction: a cycle's closing link is recorded
	// the other way round.
	foldedThrough := make(map[[2]string]bool)
	for _, edge := range edges {
		for _, row := range []*Resource{edge.primary.row, edge.other.row} {
			for _, fold := range row.linkFolds {
				foldedThrough[[2]string{fold.HolderID, fold.FoldedID}] = true
			}
		}
	}
	adjacent := make(map[string][]rowLink)
	join := func(from, to linkMember) {
		if to.id != to.row.ID && !canHangManualLinkFolds(to.row, to.id) && !canHangManualLinkFolds(to.row, from.row.ID) {
			return
		}
		adjacent[from.row.ID] = append(adjacent[from.row.ID], rowLink{
			to: to.row, near: from.id, far: to.id,
			recorded: foldedThrough[[2]string{from.id, to.id}],
		})
	}
	for _, edge := range edges {
		if edge.primary.row == edge.other.row {
			continue
		}
		join(edge.primary, edge.other)
		join(edge.other, edge.primary)
	}
	for rowID, links := range adjacent {
		// Prefer the link a fold record says took the neighbour in, so a row
		// a record pass recreated folds back through that same link; then
		// links naming each row by its own ID, so records re-hang only when
		// no such link joins the row.
		slices.SortFunc(links, func(a, b rowLink) int {
			if c := strings.Compare(a.to.ID, b.to.ID); c != 0 {
				return c
			}
			if c := compareFalseFirst(!a.recorded, !b.recorded); c != 0 {
				return c
			}
			if c := compareFalseFirst(a.far != a.to.ID, b.far != b.to.ID); c != 0 {
				return c
			}
			if c := compareFalseFirst(a.near != rowID, b.near != rowID); c != 0 {
				return c
			}
			if c := strings.Compare(a.far, b.far); c != 0 {
				return c
			}
			return strings.Compare(a.near, b.near)
		})
	}

	type step struct {
		holder *Resource
		link   rowLink
	}
	// The spanning tree follows the recorded folds first, wherever they lead:
	// a row reached through a member another row already holds would turn
	// that member's record round and lose its ID (recreated interior rows of
	// a chain fold back along the chain, not through the root's shortcuts).
	// Links no record names join only once the recorded ones are exhausted,
	// in breadth-first order, so a fresh rebuild keeps its breadth-first tree.
	type deferredLink struct {
		holder *Resource
		link   rowLink
	}
	order := []*Resource{root}
	steps := map[string]step{root.ID: {}}
	var deferred []deferredLink
	visit := func(holder *Resource, link rowLink) {
		steps[link.to.ID] = step{holder: holder, link: link}
		order = append(order, link.to)
	}
	for scanned := 0; ; {
		for ; scanned < len(order); scanned++ {
			row := order[scanned]
			for _, link := range adjacent[row.ID] {
				if _, seen := steps[link.to.ID]; seen {
					continue
				}
				if link.recorded {
					visit(row, link)
				} else {
					deferred = append(deferred, deferredLink{holder: row, link: link})
				}
			}
		}
		joinedLater := false
		for len(deferred) > 0 && !joinedLater {
			next := deferred[0]
			deferred = deferred[1:]
			if _, seen := steps[next.link.to.ID]; !seen {
				visit(next.holder, next.link)
				joinedLater = true
			}
		}
		if !joinedLater {
			break
		}
	}
	joined := make(map[string]bool, len(order))
	for _, row := range order {
		joined[row.ID] = true
		if len(order) > 1 {
			rr.recordLinkOwnPin(row.ID, row)
		}
	}
	for i := len(order) - 1; i > 0; i-- {
		folded := order[i]
		s := steps[folded.ID]
		if anchor := manualLinkFoldAnchor(folded, s.holder.ID, s.link.far); anchor != folded.ID {
			rehangManualLinkFolds(folded, anchor)
		}
		// The fold records what each side is on its own, so it reads the
		// holder before the folded row's sources join its list.
		recordManualLinkFold(s.holder, s.link.near, folded, s.link.far)
		rr.mergeResourceData(s.holder, folded, thresholds)
		stampManualLinkFoldShapes(s.holder, s.link.near, s.link.far, members[s.link.near].shape, members[s.link.far].shape)
		delete(rr.resources, folded.ID)
		// The fold record names every ID along a chain of links, so the chain
		// resolves to its root; the folded row's own index entries lapse on
		// read now that it has left the registry.
		rr.indexLinkFoldsLocked(s.holder)
		rr.updateSourceMappings(folded.ID, s.holder.ID)
	}
	recordChainLinkFolds(root, slices.DeleteFunc(slices.Clone(edges), func(edge linkEdge) bool {
		return !joined[edge.primary.row.ID] || !joined[edge.other.row.ID]
	}))
	rr.indexLinkFoldsLocked(root)
}

// recordChainLinkFolds records on a chain's root row every link of the chain
// that no fold names yet: the link that closes a cycle, or one joining two
// members an earlier pass already put in one row. The row holds both sides,
// and report-merge undoes a merge by excluding each recorded pair, so a link
// left unrecorded would keep the pair joined after a report. The link's folded
// side is its effective other unless that is the row itself, and the record
// lists what that side brought, as the fold that took it in recorded.
func recordChainLinkFolds(root *Resource, edges []linkEdge) {
	recorded := make(map[string]bool, len(root.linkFolds))
	brought := make(map[string][]DataSource, len(root.linkFolds))
	for _, fold := range root.linkFolds {
		recorded[exclusionKey(fold.HolderID, fold.FoldedID)] = true
		if _, ok := brought[fold.FoldedID]; !ok {
			brought[fold.FoldedID] = fold.Sources
		}
	}
	for _, edge := range edges {
		key := exclusionKey(edge.primary.id, edge.other.id)
		if recorded[key] {
			continue
		}
		holder, folded := edge.primary, edge.other
		if folded.id == root.ID {
			holder, folded = folded, holder
		}
		recorded[key] = true
		root.linkFolds = append(root.linkFolds, ManualLinkFold{
			HolderID:  holder.id,
			FoldedID:  folded.id,
			Sources:   cloneDataSourceSlice(brought[folded.id]),
			HolderOwn: linkMemberOwnSources(root, holder.id),
			FoldedOwn: linkMemberOwnSources(root, folded.id),
			shapes:    linkFoldShapes{recorded: true, holder: holder.shape, folded: folded.shape},
		})
	}
}

func compareFalseFirst(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	default:
		return 1
	}
}

// canHangManualLinkFolds reports whether a row's fold records join memberID to
// the row's own ID, treating each record as a link between its two IDs. A row
// can hang its records from a member only when they do; the row's own ID
// counts trivially.
func canHangManualLinkFolds(row *Resource, memberID string) bool {
	start := CanonicalResourceID(memberID)
	if start == row.ID {
		return true
	}
	adjacent := make(map[string][]string, len(row.linkFolds))
	for _, fold := range row.linkFolds {
		adjacent[fold.HolderID] = append(adjacent[fold.HolderID], fold.FoldedID)
		adjacent[fold.FoldedID] = append(adjacent[fold.FoldedID], fold.HolderID)
	}
	seen := map[string]bool{start: true}
	for queue := []string{start}; len(queue) > 0; {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacent[current] {
			if next == row.ID {
				return true
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// manualLinkFoldAnchor picks the ID a row's fold records hang from when the
// row folds into holderID through the link's member memberID: the holder's
// own ID when the row's records name it (the holder is one of the row's folds,
// as when a recreated row becomes the chain's root, or a link changed after
// the listing was published), else the member. Hanging from the member would
// turn the records between the two round the holder's ID and leave a member
// the row held with no incoming fold.
func manualLinkFoldAnchor(row *Resource, holderID, memberID string) string {
	if canHangManualLinkFolds(row, holderID) {
		return holderID
	}
	if CanonicalResourceID(memberID) == row.ID {
		return row.ID
	}
	return memberID
}

// rehangManualLinkFolds turns a row's fold records round so they hang from
// anchorID, an ID the row took in, before a chain folds the row into another
// (manualLinkFoldAnchor). Every record turns to point away from the anchor,
// from the side nearer it to the side farther, so the row's own ID becomes a
// folded ID like any other and keeps resolving, and the anchor is never a
// folded ID: a record that names it as folded (a cycle's closing link, or one
// an older generation of the links made) would leave the surviving row
// looking like a member some link folded, and the one holder no link folded,
// which report-merge infers the root from, would be gone. Records between two
// members equally far from the anchor keep their direction. This happens when
// a member the row already holds is its only link to the chain's root: in a
// live refresh, or in a registry seeded from a listing. The row no longer
// knows which of its sources each member brought, so a turned record lists
// all of them, which errs towards undoing the link on a report-merge.
func rehangManualLinkFolds(row *Resource, anchorID string) {
	anchor := CanonicalResourceID(anchorID)
	if !canHangManualLinkFolds(row, anchor) {
		return
	}
	adjacent := make(map[string][]string, len(row.linkFolds))
	for _, fold := range row.linkFolds {
		adjacent[fold.HolderID] = append(adjacent[fold.HolderID], fold.FoldedID)
		adjacent[fold.FoldedID] = append(adjacent[fold.FoldedID], fold.HolderID)
	}
	depth := map[string]int{anchor: 0}
	for queue := []string{anchor}; len(queue) > 0; {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacent[current] {
			if _, seen := depth[next]; seen {
				continue
			}
			depth[next] = depth[current] + 1
			queue = append(queue, next)
		}
	}
	folds := cloneManualLinkFolds(row.linkFolds)
	for i, fold := range folds {
		holderDepth, holderIn := depth[fold.HolderID]
		foldedDepth, foldedIn := depth[fold.FoldedID]
		if !holderIn || !foldedIn || foldedDepth >= holderDepth {
			continue
		}
		folds[i] = ManualLinkFold{
			HolderID:  fold.FoldedID,
			FoldedID:  fold.HolderID,
			Sources:   cloneDataSourceSlice(row.Sources),
			HolderOwn: fold.FoldedOwn,
			FoldedOwn: fold.HolderOwn,
			shapes:    fold.shapes.turned(),
		}
	}
	row.linkFolds = folds
}

// stampManualLinkFoldShapes records on a holder's fold record for the pair
// (holderID, foldedID) the shapes the pass judged its members by.
func stampManualLinkFoldShapes(holder *Resource, holderID, foldedID string, holderShape, foldedShape linkMemberShape) {
	key := exclusionKey(holderID, foldedID)
	for i, fold := range holder.linkFolds {
		if exclusionKey(fold.HolderID, fold.FoldedID) != key {
			continue
		}
		shapes := linkFoldShapes{recorded: true, holder: holderShape, folded: foldedShape}
		if fold.HolderID != holderID {
			shapes = shapes.turned()
		}
		holder.linkFolds[i].shapes = shapes
		return
	}
}

// holdSavedLinkMembersLocked holds each saved member a link would fold under
// the row its partner folds into. A saved member several links would fold
// answers to the best of those rows by the chain precedence, judged on each
// row's root member; the earliest link to each counts.
func (rr *ResourceRegistry) holdSavedLinkMembersLocked(edges []linkEdge) {
	members := make(map[string]linkMember)
	for _, edge := range edges {
		members[edge.primary.id] = edge.primary
		members[edge.other.id] = edge.other
	}
	targets := make(map[string]map[string]linkMember)
	linked := make(map[string]map[string]time.Time)
	for _, edge := range edges {
		// A live member is never held, and a saved primary holds nothing: a
		// live row never resolves to a saved one.
		if !edge.other.saved {
			continue
		}
		// The partner's row after the folds: the row the edge named may have
		// folded into another since, and ranking it would answer for a row
		// that has left the registry.
		partner, ok := rr.linkMemberLocked(edge.primary.id)
		if !ok {
			continue
		}
		target := rr.linkRowOwnerLocked(partner.row, members)
		if target.id == edge.other.id {
			continue
		}
		savedID := edge.other.id
		if targets[savedID] == nil {
			targets[savedID] = make(map[string]linkMember)
			linked[savedID] = make(map[string]time.Time)
		}
		targets[savedID][target.id] = target
		if at, ok := linked[savedID][target.id]; !ok || edge.createdAt.Before(at) {
			linked[savedID][target.id] = edge.createdAt
		}
	}
	for savedID, candidates := range targets {
		rr.holdLinkedResourceLocked(savedID, bestLinkRoot(slices.Collect(maps.Values(candidates)), linked[savedID]).id)
	}
}

// holdLinkedResourceLocked keeps a saved link member out of the fold. A saved
// enrollment is not an observation: folded into a live resource it would lend
// that resource its offline verdict and old payload, and a live resource
// folded into it would hide live telemetry behind it. The link still names
// one identity, so the saved member answers to the row the fold would have
// taken it into, as a folded member does.
func (rr *ResourceRegistry) holdLinkedResourceLocked(savedID, targetID string) {
	if rr.linkHolds == nil {
		rr.linkHolds = make(map[string]string)
	}
	rr.linkHolds[savedID] = targetID
	rr.canonicalIdentityIndex = nil
}
