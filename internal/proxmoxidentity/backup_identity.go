package proxmoxidentity

import "strings"

const (
	NamespaceNoLocationMatch = iota
	NamespaceWeakInstanceMatch
	NamespaceInstanceMatch
	NamespaceNodeMatch
)

func normalizeLocationLabel(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NamespaceMatchesLocation reports whether a PBS namespace likely identifies a
// Proxmox location label such as a node name or a single-node connection name.
func NamespaceMatchesLocation(namespace, location string) bool {
	ns := normalizeLocationLabel(namespace)
	loc := normalizeLocationLabel(location)
	if ns == "" || loc == "" {
		return false
	}
	if ns == loc {
		return true
	}
	return strings.HasSuffix(loc, ns) || strings.HasSuffix(ns, loc)
}

// NamespaceLocationScore ranks how strongly a PBS namespace identifies the
// current guest placement. Node matches are strongest because clustered PVE
// connections can use an API entrypoint name that is not the guest's node.
func NamespaceLocationScore(namespace, instanceName, nodeName string) int {
	nodeMatches := NamespaceMatchesLocation(namespace, nodeName)
	instanceMatches := NamespaceMatchesLocation(namespace, instanceName)

	switch {
	case nodeMatches:
		return NamespaceNodeMatch
	case instanceMatches && (normalizeLocationLabel(nodeName) == "" ||
		normalizeLocationLabel(nodeName) == normalizeLocationLabel(instanceName)):
		return NamespaceInstanceMatch
	case instanceMatches:
		return NamespaceWeakInstanceMatch
	default:
		return NamespaceNoLocationMatch
	}
}

func PreferredPBSBackupSubjectName(comment, vmid string) string {
	comment = strings.TrimSpace(comment)
	vmid = strings.TrimSpace(vmid)
	if comment == "" {
		return ""
	}
	if vmid != "" {
		if comment == vmid {
			return ""
		}
		parts := strings.Split(comment, ",")
		if len(parts) >= 2 {
			last := strings.TrimSpace(parts[len(parts)-1])
			first := strings.TrimSpace(parts[0])
			if last == vmid && first != "" && first != vmid {
				return first
			}
		}
	}
	return comment
}

func BackupCommentMatchesGuestName(comment, vmid, guestName string) bool {
	subjectName := strings.ToLower(strings.TrimSpace(PreferredPBSBackupSubjectName(comment, vmid)))
	guestName = strings.ToLower(strings.TrimSpace(guestName))
	return subjectName != "" && guestName != "" && subjectName == guestName
}

func BackupGuestMatchScore(namespace, comment, vmid, guestName, instanceName, nodeName string) int {
	score := NamespaceLocationScore(namespace, instanceName, nodeName) * 10
	if BackupCommentMatchesGuestName(comment, vmid, guestName) {
		score += 5
	}
	return score
}

// LocationLabelsEqual reports whether two location labels normalize to the
// same non-empty token. Unlike NamespaceMatchesLocation it never suffix
// matches: callers use it when the label is a connection or instance name
// rather than a PBS namespace segment, where loose suffix matching can
// cross-attribute clusters that share a VMID (#1639).
func LocationLabelsEqual(a, b string) bool {
	na := normalizeLocationLabel(a)
	nb := normalizeLocationLabel(b)
	return na != "" && na == nb
}

const (
	pbsSourceOwnerKey     = "owner"
	pbsSourceDatastoreKey = "datastore"
	pbsSourceInstanceKey  = "instance"
)

// PBSSourceLearner accumulates which PVE connection each PBS submission
// source has been positively attributed to. A "source" is, strongest first,
// the backup owner token, the datastore, and the PBS instance — each scoped
// to the PBS instance the backup came from. Clusters usually push to PBS
// with a per-cluster token and often a per-cluster datastore, so evidence
// learned from unambiguous guests can attribute root-namespace,
// comment-less snapshots whose VMID exists on more than one cluster (#1639).
//
// Evidence is only ever positive: a cluster becomes visible to the learner
// by having a snapshot attributed to it. A source key mapping to exactly one
// visible cluster therefore proves nothing on its own, because a cluster
// with no attributable snapshot at all is indistinguishable from a cluster
// that does not use that source. Callers must declare every cluster that
// could have authored a backup on a PBS instance with RegisterCandidate, and
// resolution stays inconclusive until each of them has been observed.
type PBSSourceLearner struct {
	instancesByKey map[string]map[string]struct{}
	// candidatesByPBS lists, per PBS instance, the PVE connections that own
	// a guest one of that instance's backups could belong to.
	candidatesByPBS map[string]map[string]struct{}
	// observed is every PVE connection that has had at least one snapshot
	// positively attributed to it, from any PBS instance.
	observed map[string]struct{}
}

func NewPBSSourceLearner() *PBSSourceLearner {
	return &PBSSourceLearner{
		instancesByKey:  make(map[string]map[string]struct{}),
		candidatesByPBS: make(map[string]map[string]struct{}),
		observed:        make(map[string]struct{}),
	}
}

// RegisterCandidate declares that pveInstance owns a guest that a backup on
// pbsInstance could belong to. Resolve refuses to be decisive for that PBS
// instance until every registered candidate has been observed, so a cluster
// that contributes no attributable snapshot cannot be silently written out of
// the candidate set and have its backups handed to a cluster that shares its
// owner token or datastore (#1639).
func (l *PBSSourceLearner) RegisterCandidate(pbsInstance, pveInstance string) {
	if l == nil {
		return
	}
	pveInstance = strings.TrimSpace(pveInstance)
	if pveInstance == "" {
		return
	}
	key := strings.TrimSpace(pbsInstance)
	set, ok := l.candidatesByPBS[key]
	if !ok {
		set = make(map[string]struct{})
		l.candidatesByPBS[key] = set
	}
	set[pveInstance] = struct{}{}
}

func pbsSourceKeys(pbsInstance, datastore, owner string) []string {
	pbsInstance = strings.TrimSpace(pbsInstance)
	datastore = strings.TrimSpace(datastore)
	owner = strings.ToLower(strings.TrimSpace(owner))
	keys := make([]string, 0, 3)
	if owner != "" {
		keys = append(keys, pbsSourceOwnerKey+"\x00"+pbsInstance+"\x00"+owner)
	}
	if datastore != "" {
		keys = append(keys, pbsSourceDatastoreKey+"\x00"+pbsInstance+"\x00"+datastore)
	}
	if pbsInstance != "" {
		keys = append(keys, pbsSourceInstanceKey+"\x00"+pbsInstance)
	}
	return keys
}

// Observe records that a backup from the given PBS source was positively
// attributed to a PVE connection (via a unique VMID, namespace placement,
// or guest-name match).
func (l *PBSSourceLearner) Observe(pbsInstance, datastore, owner, pveInstance string) {
	if l == nil {
		return
	}
	pveInstance = strings.TrimSpace(pveInstance)
	if pveInstance == "" {
		return
	}
	l.observed[pveInstance] = struct{}{}
	for _, key := range pbsSourceKeys(pbsInstance, datastore, owner) {
		set, ok := l.instancesByKey[key]
		if !ok {
			set = make(map[string]struct{})
			l.instancesByKey[key] = set
		}
		set[pveInstance] = struct{}{}
	}
}

// candidatesAllObserved reports whether every PVE connection that could have
// authored a backup on this PBS instance has had at least one snapshot
// positively attributed to it. Until that holds, a source key mapping to a
// single connection is not evidence the source is exclusive to it — the
// unobserved connection may be pushing through the very same owner token or
// datastore, and its snapshots would be handed to the observed one (#1639).
//
// Observation is not scoped to the PBS instance on purpose: a cluster seen
// submitting to a different PBS instance is visible evidence about where its
// backups land, which is exactly what distinguishes clusters that each own a
// PBS server. A cluster with no attributed snapshot anywhere is invisible and
// blocks resolution.
func (l *PBSSourceLearner) candidatesAllObserved(pbsInstance string) bool {
	candidates := l.candidatesByPBS[strings.TrimSpace(pbsInstance)]
	if len(candidates) == 0 {
		return false
	}
	for candidate := range candidates {
		if _, ok := l.observed[candidate]; !ok {
			return false
		}
	}
	return true
}

// Resolve reports the single PVE connection the backup's source evidence
// identifies. Evidence is consulted strongest-first; a source component that
// maps to several connections is not a discriminator and defers to the next
// one, but a component that was never observed stops resolution entirely —
// an unfamiliar owner or datastore means the backup may belong to a cluster
// we have no evidence for, and guessing from weaker components would
// attribute it to the wrong cluster.
//
// Resolution is inconclusive for the whole PBS instance while any registered
// candidate cluster is still unobserved, because a singleton mapping then
// only reflects which clusters happened to be attributable, not that the
// source is exclusive to one of them.
func (l *PBSSourceLearner) Resolve(pbsInstance, datastore, owner string) (string, bool) {
	if l == nil {
		return "", false
	}
	if !l.candidatesAllObserved(pbsInstance) {
		return "", false
	}
	for _, key := range pbsSourceKeys(pbsInstance, datastore, owner) {
		set, ok := l.instancesByKey[key]
		if !ok {
			return "", false
		}
		if len(set) == 1 {
			for instance := range set {
				return instance, true
			}
		}
	}
	return "", false
}
