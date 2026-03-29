package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	subjectKeyResourcePrefix = "res:"
	subjectKeyExternalPrefix = "ext:"
)

// SubjectKeyForPoint returns a stable grouping key for rollups. When a recovery point can be linked
// to a unified resource, the key is "res:<resource-id>". Otherwise it is a hashed external key
// derived from provider + external ref fields.
func SubjectKeyForPoint(p RecoveryPoint) string {
	return SubjectKey(p.Provider, p.SubjectResourceID, p.SubjectRef)
}

// SubjectKey builds a stable grouping key from the point provider + subject identifiers.
func SubjectKey(provider Provider, subjectResourceID string, subjectRef *ExternalRef) string {
	rid := canonicalizeProxmoxLinkedSubjectResourceID(provider, subjectResourceID, subjectRef)
	if rid != "" {
		return subjectKeyResourcePrefix + rid
	}
	return externalKey(provider, subjectRef)
}

// RepositoryKey builds a stable grouping key for the repository/destination side (best-effort).
func RepositoryKey(provider Provider, repositoryResourceID string, repositoryRef *ExternalRef) string {
	rid := strings.TrimSpace(repositoryResourceID)
	if rid != "" {
		return subjectKeyResourcePrefix + rid
	}
	return externalKey(provider, repositoryRef)
}

func externalKey(provider Provider, ref *ExternalRef) string {
	if ref == nil {
		return ""
	}

	// Use the strongest identifier available in a predictable order.
	typeValue := strings.TrimSpace(ref.Type)
	if typeValue == "" {
		return ""
	}

	if key := proxmoxGuestExternalKey(provider, ref); key != "" {
		return key
	}

	h := sha256.New()
	write := func(s string) {
		h.Write([]byte(strings.TrimSpace(s)))
		h.Write([]byte{0})
	}

	write(subjectKeyExternalPrefix)
	write(string(provider))
	write(typeValue)
	write(ref.UID)
	write(ref.ID)
	write(ref.Namespace)
	write(ref.Name)
	write(ref.Class)

	// Extra is intentionally excluded: it is often high-entropy and non-stable.
	return subjectKeyExternalPrefix + hex.EncodeToString(h.Sum(nil))
}

func isProxmoxProvider(provider Provider) bool {
	switch provider {
	case ProviderProxmoxPVE, ProviderProxmoxPBS:
		return true
	default:
		return false
	}
}

func proxmoxGuestResourceType(ref *ExternalRef, subjectResourceID string) unifiedresources.ResourceType {
	if ref != nil {
		switch strings.TrimSpace(ref.Type) {
		case "proxmox-vm":
			return unifiedresources.ResourceTypeVM
		case "proxmox-lxc":
			return unifiedresources.ResourceTypeSystemContainer
		}
	}

	rid := strings.TrimSpace(subjectResourceID)
	switch {
	case strings.HasPrefix(rid, "vm-"):
		return unifiedresources.ResourceTypeVM
	case strings.HasPrefix(rid, "system-container-"), strings.HasPrefix(rid, "lxc-"):
		return unifiedresources.ResourceTypeSystemContainer
	default:
		return ""
	}
}

func normalizeProxmoxGuestSourceID(resourceType unifiedresources.ResourceType, sourceID string) string {
	normalized := strings.TrimSpace(sourceID)
	if normalized == "" {
		return ""
	}
	if resourceType == unifiedresources.ResourceTypeSystemContainer && strings.HasPrefix(normalized, "lxc-") {
		return "system-container-" + strings.TrimPrefix(normalized, "lxc-")
	}
	return normalized
}

func canonicalizeProxmoxLinkedSubjectResourceID(provider Provider, subjectResourceID string, subjectRef *ExternalRef) string {
	rid := strings.TrimSpace(subjectResourceID)
	if rid == "" || !isProxmoxProvider(provider) {
		return rid
	}

	resourceType := proxmoxGuestResourceType(subjectRef, rid)
	if resourceType == "" {
		return rid
	}

	normalizedRID := normalizeProxmoxGuestSourceID(resourceType, rid)
	refID := ""
	if subjectRef != nil {
		refID = normalizeProxmoxGuestSourceID(resourceType, subjectRef.ID)
	}

	if refID != "" {
		canonicalFromRef := unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, refID)
		if rid == canonicalFromRef || normalizedRID == canonicalFromRef {
			if strings.Contains(refID, ":") {
				return unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, normalizedRID)
			}
			return canonicalFromRef
		}
		if normalizedRID == refID {
			return canonicalFromRef
		}
	}

	if normalizedRID != rid {
		return unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, normalizedRID)
	}

	// Legacy linked guest rows sometimes kept a raw source-derived subject resource ID while the
	// ref ID had already fallen back to the stable instance:node:vmid identity.
	if refID != "" && strings.Contains(refID, ":") {
		return unifiedresources.SourceSpecificID(resourceType, unifiedresources.SourceProxmox, normalizedRID)
	}

	return rid
}

func proxmoxGuestExternalKey(provider Provider, ref *ExternalRef) string {
	if ref == nil || !isProxmoxProvider(provider) {
		return ""
	}

	resourceType := proxmoxGuestResourceType(ref, "")
	if resourceType == "" {
		return ""
	}

	typeValue := strings.TrimSpace(ref.Type)
	if typeValue == "" {
		return ""
	}

	h := sha256.New()
	write := func(s string) {
		h.Write([]byte(strings.TrimSpace(s)))
		h.Write([]byte{0})
	}

	write(subjectKeyExternalPrefix)
	write(string(provider))
	write(typeValue)
	write(ref.UID)
	write(normalizeProxmoxGuestSourceID(resourceType, ref.ID))
	write(ref.Namespace)
	write(ref.Class)

	// Proxmox guest names are presentation data, not identity. Excluding Name prevents one guest
	// from splitting into multiple rollups when comments or display labels change.
	return subjectKeyExternalPrefix + hex.EncodeToString(h.Sum(nil))
}

func normalizeContinuityLabel(value string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// ProxmoxPBSGuestContinuityKey returns a conservative continuity key for PBS guest backups.
// It intentionally requires a friendly subject label plus guest type, namespace, and VMID so the
// store can repair historically split rollups without guessing across unrelated reused VMIDs.
func ProxmoxPBSGuestContinuityKey(subjectLabel, itemType, namespaceLabel, entityIDLabel string) string {
	itemType = NormalizeRecoveryItemType(itemType)
	switch itemType {
	case "vm", "system-container":
	default:
		return ""
	}

	namespaceLabel = strings.ToLower(strings.TrimSpace(namespaceLabel))
	entityIDLabel = strings.TrimSpace(entityIDLabel)
	subjectLabel = normalizeContinuityLabel(subjectLabel)
	if namespaceLabel == "" || entityIDLabel == "" || subjectLabel == "" || isNumericOnlyLabel(subjectLabel) {
		return ""
	}

	return strings.Join([]string{
		"proxmox-pbs-guest",
		itemType,
		namespaceLabel,
		entityIDLabel,
		subjectLabel,
	}, ":")
}

// ProxmoxPBSGuestLooseContinuityKey deliberately omits namespace. It is only safe to use when the
// caller has already proven there is a single linked historical candidate for the label/type/vmid.
func ProxmoxPBSGuestLooseContinuityKey(subjectLabel, itemType, entityIDLabel string) string {
	itemType = NormalizeRecoveryItemType(itemType)
	switch itemType {
	case "vm", "system-container":
	default:
		return ""
	}

	entityIDLabel = strings.TrimSpace(entityIDLabel)
	subjectLabel = normalizeContinuityLabel(subjectLabel)
	if entityIDLabel == "" || subjectLabel == "" || isNumericOnlyLabel(subjectLabel) {
		return ""
	}

	return strings.Join([]string{
		"proxmox-pbs-guest-loose",
		itemType,
		entityIDLabel,
		subjectLabel,
	}, ":")
}

// ProxmoxPBSGuestContinuityKeyForPoint derives the conservative PBS guest continuity key from a
// normalized recovery point. It is used by the store to preserve historical identity continuity.
func ProxmoxPBSGuestContinuityKeyForPoint(p RecoveryPoint) string {
	if p.Provider != ProviderProxmoxPBS {
		return ""
	}
	idx := DeriveIndex(p)
	return ProxmoxPBSGuestContinuityKey(
		idx.SubjectLabel,
		idx.ItemType,
		idx.NamespaceLabel,
		idx.EntityIDLabel,
	)
}

// RollupResourceID returns the public-facing rollup ID:
// - linked resources return the raw unified resource ID
// - external refs return the external key
func RollupResourceID(subjectKey string) string {
	subjectKey = strings.TrimSpace(subjectKey)
	if strings.HasPrefix(subjectKey, subjectKeyResourcePrefix) {
		return strings.TrimPrefix(subjectKey, subjectKeyResourcePrefix)
	}
	return subjectKey
}
