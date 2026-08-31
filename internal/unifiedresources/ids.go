package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
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

// MachineIdentityCanonicalID returns the canonical ID the registry mints for
// a resource keyed by its machine identity, the strongest arm of the
// chooseNewID ladder. Providers use it to name superseded canonical IDs
// (IngestRecord.SupersededCanonicalIDs) when a derivation correction retires
// a machine key, so operator-owned rows follow the resource to its new ID.
func MachineIdentityCanonicalID(resourceType ResourceType, machineID string) string {
	return buildHashID(resourceType, "machine:"+strings.TrimSpace(machineID))
}

// ProxmoxGuestIdentityKey returns the node-independent identity key for a
// Proxmox guest. VMIDs are unique within a cluster (and an instance maps to
// one cluster or one standalone node), so instance+VMID survives live
// migration between cluster nodes while the node-scoped source ID does not.
func ProxmoxGuestIdentityKey(instance string, vmid int) string {
	instance = strings.TrimSpace(instance)
	if instance == "" || vmid <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", instance, vmid)
}

// ProxmoxGuestCanonicalID returns the canonical ID the registry mints for a
// Proxmox guest keyed by its node-independent identity (see
// ProxmoxGuestIdentityKey). Consumers that persist guest resource IDs
// (recovery subjects, migrations) derive through this instead of hashing the
// node-scoped source ID.
func ProxmoxGuestCanonicalID(resourceType ResourceType, instance string, vmid int) string {
	key := ProxmoxGuestIdentityKey(instance, vmid)
	if key == "" {
		return ""
	}
	return buildHashID(resourceType, "proxmox-guest:"+key)
}

// ParseProxmoxGuestSourceID splits a node-scoped guest source ID
// ("instance:node:vmid", the makeGuestID format) into its components. The
// instance segment may itself contain colons, so the node and VMID are taken
// from the right.
func ParseProxmoxGuestSourceID(sourceID string) (instance, node string, vmid int, ok bool) {
	sourceID = strings.TrimSpace(sourceID)
	lastSep := strings.LastIndex(sourceID, ":")
	if lastSep <= 0 || lastSep == len(sourceID)-1 {
		return "", "", 0, false
	}
	parsedVMID, err := strconv.Atoi(sourceID[lastSep+1:])
	if err != nil || parsedVMID <= 0 {
		return "", "", 0, false
	}
	rest := sourceID[:lastSep]
	nodeSep := strings.LastIndex(rest, ":")
	if nodeSep <= 0 || nodeSep == len(rest)-1 {
		return "", "", 0, false
	}
	instance = strings.TrimSpace(rest[:nodeSep])
	node = strings.TrimSpace(rest[nodeSep+1:])
	if instance == "" || node == "" {
		return "", "", 0, false
	}
	return instance, node, parsedVMID, true
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
	// Hostname is the normalized full hostname (NormalizeFullHostname).
	// Dotted hostnames are preserved so distinct machines that share a short
	// name (cloud.rnd-lax1 vs cloud.gce-or1) keep distinct pins. Rows written
	// before the fix for #1559 hold the collapsed short name; they heal in
	// place on the host's next pin persist.
	Hostname string
}

func (p ResourceIdentityPin) normalized() ResourceIdentityPin {
	p.CanonicalID = CanonicalResourceID(p.CanonicalID)
	p.ResourceType = CanonicalResourceType(p.ResourceType)
	p.MachineID = strings.TrimSpace(p.MachineID)
	p.DMIUUID = strings.TrimSpace(p.DMIUUID)
	p.ClusterName = strings.TrimSpace(p.ClusterName)
	p.Hostname = NormalizeFullHostname(p.Hostname)
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
	ids := make([]string, 0, 7)
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
		// The historical chooseNewID ladder hashed the short hostname; the
		// pin now preserves the full dotted name, so derive eras for both so
		// journal rows written under short-hostname IDs stay readable.
		for _, hostname := range uniqueTrimmed(p.Hostname, NormalizeHostname(p.Hostname)) {
			if p.ClusterName != "" {
				ids = append(ids, buildHashID(p.ResourceType, fmt.Sprintf("cluster:%s:%s", p.ClusterName, hostname)))
			}
			ids = append(ids, buildHashID(p.ResourceType, "hostname:"+hostname))
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
	hostname := firstIdentityHostname(resource.Identity)
	// Proxmox native node names are commonly short and repeat across
	// independent estates. When the merged provider view has a full endpoint,
	// pin that provider-scoped hostname so a provider-only boot can recover the
	// machine identity without making the short name globally authoritative.
	if resource.Proxmox != nil {
		if endpoint := proxmoxProviderPinHostname(*resource); endpoint != "" {
			hostname = endpoint
		}
	}
	pin := ResourceIdentityPin{
		CanonicalID:  resource.ID,
		ResourceType: ResourceTypeAgent,
		MachineID:    resource.Identity.MachineID,
		DMIUUID:      resource.Identity.DMIUUID,
		ClusterName:  resource.Identity.ClusterName,
		Hostname:     hostname,
	}.normalized()
	if pin.CanonicalID == "" || !pin.hasStrongKey() {
		return ResourceIdentityPin{}, false
	}
	return pin, true
}
