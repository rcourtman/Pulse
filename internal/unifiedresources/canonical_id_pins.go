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
	byClusterHost map[string][]ResourceIdentityPin
	// Hostname indexes carry the preserved primary hostname plus its historical
	// short-name alias. Hostnames are not unique across machines, so lookups
	// through either index require the selected bucket to be unambiguous.
	byHostname map[string][]ResourceIdentityPin
}

func newIdentityPinIndex(pins []ResourceIdentityPin) *identityPinIndex {
	index := &identityPinIndex{
		byCanonicalID: make(map[string]ResourceIdentityPin, len(pins)),
		byMachineID:   make(map[string]ResourceIdentityPin),
		byDMIUUID:     make(map[string]ResourceIdentityPin),
		byClusterHost: make(map[string][]ResourceIdentityPin),
		byHostname:    make(map[string][]ResourceIdentityPin),
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
		for _, hostnameKey := range identityPinHostnameKeys(pin.Hostname) {
			if pin.ClusterName != "" {
				key := clusterHostPinKey(pin.ClusterName, hostnameKey)
				index.byClusterHost[key] = append(index.byClusterHost[key], pin)
			}
			index.byHostname[hostnameKey] = append(index.byHostname[hostnameKey], pin)
		}
	}
	return index
}

func clusterHostPinKey(clusterName, hostname string) string {
	return strings.ToLower(strings.TrimSpace(clusterName)) + "\x00" + NormalizePrimaryHostname(hostname)
}

// identityPinHostnameKeys returns the exact persisted hostname first, followed
// by the historical short-name alias when the two differ. Lookup walks keys in
// this order so distinct dotted names remain distinct while legacy short-name
// pins can still recover an identity when that alias is unambiguous.
func identityPinHostnameKeys(hostname string) []string {
	primary := NormalizePrimaryHostname(hostname)
	if primary == "" {
		return nil
	}
	short := NormalizeHostname(primary)
	if short == "" || short == primary {
		return []string{primary}
	}
	return []string{primary, short}
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
		for _, hostnameKey := range identityPinHostnameKeys(hostname) {
			if clusterName != "" {
				bucket := index.byClusterHost[clusterHostPinKey(clusterName, hostnameKey)]
				if len(bucket) == 1 && pinCompatible(bucket[0], machineID, dmiUUID) {
					return bucket[0], true
				}
			}
			bucket := index.byHostname[hostnameKey]
			if len(bucket) == 1 && pinCompatible(bucket[0], machineID, dmiUUID) {
				return bucket[0], true
			}
		}
	}
	return ResourceIdentityPin{}, false
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
