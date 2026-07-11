package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func normalizeSourceID(sourceID string) string {
	return strings.TrimSpace(sourceID)
}

// SourceSpecificID returns the deterministic ID used for non-host resources when the registry
// does not have a canonical identity to key off of.
//
// This matches the ResourceRegistry's internal ID derivation:
// stable := fmt.Sprintf("%s:%s", source, sourceID)
// id := fmt.Sprintf("%s-%s", resourceType, hex(sha256(stable)[:8]))
func SourceSpecificID(resourceType ResourceType, source DataSource, sourceID string) string {
	resourceType = CanonicalResourceType(resourceType)
	stable := fmt.Sprintf("%s:%s", source, normalizeSourceID(sourceID))
	hash := sha256.Sum256([]byte(stable))
	return fmt.Sprintf("%s-%s", resourceType, hex.EncodeToString(hash[:8]))
}

// CanonicalResourceID returns the canonical v6 resource identifier.
func CanonicalResourceID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	return trimmed
}

// ResourceIdentityPin is the durable identity→canonical-ID record persisted in
// the resource store (resource_identities table). It exists because canonical
// ID derivation keys off the strongest identity field known at mint time, and
// merged-source hosts (PVE node + pulse-agent) expose different identity
// subsets depending on which records are present when the registry rebuilds:
// the agent record knows the machine ID, the Proxmox node record only knows
// cluster+hostname. Pins let a rebuild that only sees the weak subset recover
// the strong keys, so the same physical host derives the same canonical ID in
// every boot, in every registry instance.
type ResourceIdentityPin struct {
	CanonicalID  string
	ResourceType ResourceType
	MachineID    string
	DMIUUID      string
	ClusterName  string
	// Hostname is the normalized primary hostname (NormalizePrimaryHostname).
	Hostname string
}

func (p ResourceIdentityPin) normalized() ResourceIdentityPin {
	p.CanonicalID = CanonicalResourceID(p.CanonicalID)
	p.ResourceType = CanonicalResourceType(p.ResourceType)
	p.MachineID = strings.TrimSpace(p.MachineID)
	p.DMIUUID = strings.TrimSpace(p.DMIUUID)
	p.ClusterName = strings.TrimSpace(p.ClusterName)
	p.Hostname = NormalizePrimaryHostname(p.Hostname)
	return p
}

func (p ResourceIdentityPin) hasStrongKey() bool {
	return p.MachineID != "" || p.DMIUUID != "" || (p.ClusterName != "" && p.Hostname != "")
}

// EraIDs returns every canonical ID this pin's identity keys derive under the
// historical chooseNewID ladder (machine > dmi > cluster+hostname > hostname).
// Change-journal rows written in boots that only knew a weaker key sit under
// the weaker key's hash; expanding a read to the full era set merges those
// journal eras without rewriting history.
func (p ResourceIdentityPin) EraIDs() []string {
	p = p.normalized()
	ids := make([]string, 0, 5)
	if p.CanonicalID != "" {
		ids = append(ids, p.CanonicalID)
	}
	if p.MachineID != "" {
		ids = append(ids, buildHashID(p.ResourceType, "machine:"+p.MachineID))
	}
	if p.DMIUUID != "" {
		ids = append(ids, buildHashID(p.ResourceType, "dmi:"+p.DMIUUID))
	}
	if p.Hostname != "" {
		if p.ClusterName != "" {
			ids = append(ids, buildHashID(p.ResourceType, fmt.Sprintf("cluster:%s:%s", p.ClusterName, p.Hostname)))
		}
		ids = append(ids, buildHashID(p.ResourceType, "hostname:"+p.Hostname))
		if short := NormalizeHostname(p.Hostname); short != "" && short != p.Hostname {
			if p.ClusterName != "" {
				ids = append(ids, buildHashID(p.ResourceType, fmt.Sprintf("cluster:%s:%s", p.ClusterName, short)))
			}
			ids = append(ids, buildHashID(p.ResourceType, "hostname:"+short))
		}
	}
	return uniqueTrimmed(ids...)
}

// identityPinForResource derives the persistable pin for a canonical resource.
// Only host resources with at least one strong identity key are pinned; a
// hostname-only host has a single possible derivation and needs no pin.
func identityPinForResource(resource *Resource) (ResourceIdentityPin, bool) {
	if resource == nil || CanonicalResourceType(resource.Type) != ResourceTypeAgent {
		return ResourceIdentityPin{}, false
	}
	pin := ResourceIdentityPin{
		CanonicalID:  resource.ID,
		ResourceType: ResourceTypeAgent,
		MachineID:    resource.Identity.MachineID,
		DMIUUID:      resource.Identity.DMIUUID,
		ClusterName:  resource.Identity.ClusterName,
		Hostname:     firstIdentityHostname(resource.Identity),
	}.normalized()
	if pin.CanonicalID == "" || !pin.hasStrongKey() {
		return ResourceIdentityPin{}, false
	}
	return pin, true
}
