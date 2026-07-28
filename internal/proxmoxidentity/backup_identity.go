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
type PBSSourceLearner struct {
	instancesByKey map[string]map[string]struct{}
}

func NewPBSSourceLearner() *PBSSourceLearner {
	return &PBSSourceLearner{instancesByKey: make(map[string]map[string]struct{})}
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
	for _, key := range pbsSourceKeys(pbsInstance, datastore, owner) {
		set, ok := l.instancesByKey[key]
		if !ok {
			set = make(map[string]struct{})
			l.instancesByKey[key] = set
		}
		set[pveInstance] = struct{}{}
	}
}

// Resolve reports the single PVE connection the backup's source evidence
// identifies. Evidence is consulted strongest-first; a source component that
// maps to several connections is not a discriminator and defers to the next
// one, but a component that was never observed stops resolution entirely —
// an unfamiliar owner or datastore means the backup may belong to a cluster
// we have no evidence for, and guessing from weaker components would
// attribute it to the wrong cluster.
func (l *PBSSourceLearner) Resolve(pbsInstance, datastore, owner string) (string, bool) {
	if l == nil {
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
