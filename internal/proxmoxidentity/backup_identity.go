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
