package unifiedresources

import (
	"log"
	"strings"
)

// identityPinIndex is the in-memory lookup over the durable identity pins.
// It answers one question at ingest time: "have we ever durably assigned a
// canonical ID to the physical host this (possibly weak) identity refers to,
// and what strong identity keys did we record for it?"
type identityPinIndex struct {
	byCanonicalID map[string]ResourceIdentityPin
	byMachineID   map[string]ResourceIdentityPin
	byDMIUUID     map[string]ResourceIdentityPin
	// byClusterHost buckets pins per cluster + full hostname, and
	// byClusterShortHost per cluster + short hostname; byHostname and
	// byShortHostname are the cluster-less equivalents. Full-hostname hits
	// are authoritative; short buckets only resolve when they are
	// unambiguous AND the pinned hostname is short/FQDN-equivalent to the
	// incoming one, so distinct dotted hostnames that share a short name
	// (cloud.rnd-lax1 vs cloud.gce-or1) never cross-match. Hostnames are
	// not unique across machines, so every bucket lookup requires the
	// bucket to be unambiguous.
	byClusterHost      map[string][]ResourceIdentityPin
	byClusterShortHost map[string][]ResourceIdentityPin
	byHostname         map[string][]ResourceIdentityPin
	byShortHostname    map[string][]ResourceIdentityPin
}

func newIdentityPinIndex(pins []ResourceIdentityPin) *identityPinIndex {
	index := &identityPinIndex{
		byCanonicalID:      make(map[string]ResourceIdentityPin, len(pins)),
		byMachineID:        make(map[string]ResourceIdentityPin),
		byDMIUUID:          make(map[string]ResourceIdentityPin),
		byClusterHost:      make(map[string][]ResourceIdentityPin),
		byClusterShortHost: make(map[string][]ResourceIdentityPin),
		byHostname:         make(map[string][]ResourceIdentityPin),
		byShortHostname:    make(map[string][]ResourceIdentityPin),
	}
	for _, pin := range pins {
		pin = pin.normalized()
		if pin.CanonicalID == "" || !pin.hasStrongKey() {
			continue
		}
		index.byCanonicalID[pin.CanonicalID] = pin
		if pin.MachineID != "" {
			index.byMachineID[pin.MachineID] = pin
		}
		if pin.DMIUUID != "" {
			index.byDMIUUID[pin.DMIUUID] = pin
		}
		if pin.Hostname == "" {
			continue
		}
		shortHostname := NormalizeHostname(pin.Hostname)
		if pin.ClusterName != "" {
			fullKey := clusterHostPinKey(pin.ClusterName, pin.Hostname)
			index.byClusterHost[fullKey] = append(index.byClusterHost[fullKey], pin)
			shortKey := clusterHostPinKey(pin.ClusterName, shortHostname)
			index.byClusterShortHost[shortKey] = append(index.byClusterShortHost[shortKey], pin)
		}
		index.byHostname[pin.Hostname] = append(index.byHostname[pin.Hostname], pin)
		index.byShortHostname[shortHostname] = append(index.byShortHostname[shortHostname], pin)
	}
	return index
}

func clusterHostPinKey(clusterName, hostname string) string {
	return strings.ToLower(strings.TrimSpace(clusterName)) + "\x00" + NormalizeFullHostname(hostname)
}

// find resolves the pin for an incoming identity, strongest key first. A pin
// found through a weaker key is rejected when the incoming identity carries a
// strong key that contradicts the pin (a different machine claiming the same
// cluster slot or hostname must mint fresh, not absorb the old host).
func (index *identityPinIndex) find(identity ResourceIdentity) (ResourceIdentityPin, bool) {
	if index == nil {
		return ResourceIdentityPin{}, false
	}
	machineID := strings.TrimSpace(identity.MachineID)
	dmiUUID := strings.TrimSpace(identity.DMIUUID)
	if machineID != "" {
		if pin, ok := index.byMachineID[machineID]; ok {
			return pin, true
		}
	}
	if dmiUUID != "" {
		if pin, ok := index.byDMIUUID[dmiUUID]; ok && pinCompatible(pin, machineID, "") {
			return pin, true
		}
	}
	clusterName := strings.TrimSpace(identity.ClusterName)
	for _, hostname := range identity.Hostnames {
		fullHostname := NormalizeFullHostname(hostname)
		if fullHostname == "" {
			continue
		}
		shortHostname := NormalizeHostname(fullHostname)
		if clusterName != "" {
			if pin, ok := resolvePinBucket(index.byClusterHost[clusterHostPinKey(clusterName, fullHostname)], fullHostname, machineID, dmiUUID); ok {
				return pin, true
			}
			if pin, ok := resolvePinBucket(index.byClusterShortHost[clusterHostPinKey(clusterName, shortHostname)], fullHostname, machineID, dmiUUID); ok {
				return pin, true
			}
		}
		if pin, ok := resolvePinBucket(index.byHostname[fullHostname], fullHostname, machineID, dmiUUID); ok {
			return pin, true
		}
		if pin, ok := resolvePinBucket(index.byShortHostname[shortHostname], fullHostname, machineID, dmiUUID); ok {
			return pin, true
		}
	}
	return ResourceIdentityPin{}, false
}

// resolvePinBucket resolves a hostname bucket lookup. The bucket must be
// unambiguous (exactly one pin), the pinned hostname must be the incoming one
// or its short/FQDN equivalent (two distinct FQDNs sharing a short name never
// match), and the incoming strong keys must not contradict the pin.
func resolvePinBucket(bucket []ResourceIdentityPin, hostname, machineID, dmiUUID string) (ResourceIdentityPin, bool) {
	if len(bucket) != 1 {
		return ResourceIdentityPin{}, false
	}
	pin := bucket[0]
	if pin.Hostname != hostname && !HostnamesEquivalent(pin.Hostname, hostname) {
		return ResourceIdentityPin{}, false
	}
	if !pinCompatible(pin, machineID, dmiUUID) {
		return ResourceIdentityPin{}, false
	}
	return pin, true
}

// pinCompatible reports whether an incoming identity's strong keys are
// consistent with the pin. Empty incoming keys never contradict; the pin's
// own empty keys never contradict either.
func pinCompatible(pin ResourceIdentityPin, machineID, dmiUUID string) bool {
	if machineID != "" && pin.MachineID != "" && machineID != pin.MachineID {
		return false
	}
	if dmiUUID != "" && pin.DMIUUID != "" && dmiUUID != pin.DMIUUID {
		return false
	}
	return true
}

func (rr *ResourceRegistry) loadIdentityPins() {
	rr.identityPins = newIdentityPinIndex(nil)
	if rr.store == nil {
		return
	}
	pins, err := rr.store.ListResourceIdentityPins()
	if err != nil {
		log.Printf("unifiedresources: failed to load identity pins from store: %v", err)
		return
	}
	rr.identityPins = newIdentityPinIndex(pins)
}

// completeIdentityFromPins fills the machine-level identity keys a weak
// incoming identity lacks, from the durable pin for the same physical host.
// This runs before identity matching and canonical-ID derivation, so a boot
// window where the agent has not checked in yet (the Proxmox node record only
// knows cluster+hostname) still derives the same canonical ID as a steady
// state rebuild that knows the machine ID. Only missing fields are filled; an
// incoming identity that already carries a machine ID is never overridden.
func (rr *ResourceRegistry) completeIdentityFromPins(resourceType ResourceType, identity ResourceIdentity) ResourceIdentity {
	if CanonicalResourceType(resourceType) != ResourceTypeAgent {
		return identity
	}
	if strings.TrimSpace(identity.MachineID) != "" && strings.TrimSpace(identity.DMIUUID) != "" {
		return identity
	}
	pin, ok := rr.identityPins.find(identity)
	if !ok {
		return identity
	}
	if strings.TrimSpace(identity.MachineID) == "" {
		identity.MachineID = pin.MachineID
	}
	if strings.TrimSpace(identity.DMIUUID) == "" {
		identity.DMIUUID = pin.DMIUUID
	}
	return identity
}

// PersistIdentityPins writes the identity pins for the registry's current
// host resources into the resource store. Only new or changed pins are
// written, so steady-state rebuild ticks cost no writes. Call this after a
// rebuild on the durable store-backed registry; ephemeral per-request
// registries consult pins but do not write them.
func (rr *ResourceRegistry) PersistIdentityPins() {
	if rr.store == nil {
		return
	}

	rr.mu.RLock()
	var pins []ResourceIdentityPin
	for _, resource := range rr.resources {
		pin, ok := identityPinForResource(resource)
		if !ok {
			continue
		}
		if existing, known := rr.identityPins.byCanonicalID[pin.CanonicalID]; known && existing == pin {
			continue
		}
		pins = append(pins, pin)
	}
	rr.mu.RUnlock()

	if len(pins) == 0 {
		return
	}
	if err := rr.store.UpsertResourceIdentityPins(pins); err != nil {
		log.Printf("unifiedresources: failed to persist identity pins: %v", err)
		return
	}
	refreshed, err := rr.store.ListResourceIdentityPins()
	if err != nil {
		log.Printf("unifiedresources: failed to reload identity pins after persist: %v", err)
		return
	}
	index := newIdentityPinIndex(refreshed)
	rr.mu.Lock()
	rr.identityPins = index
	rr.mu.Unlock()
}
