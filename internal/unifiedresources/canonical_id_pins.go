package unifiedresources

import (
	"log"
	"net"
	"slices"
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

// findProxmoxNode resolves a weak provider node only inside identity evidence
// owned by that provider. A standalone node name is not an estate-wide key:
// pve in one DNS zone may coexist with pve in another. Falling through to the
// generic short-hostname buckets lets the first site's durable agent pin lend
// its machine ID to every same-named provider during the boot window before
// agents check in, collapsing those provider rows into one canonical resource.
//
// A full configured endpoint is provider-scoped evidence and is safe when the
// durable pin recorded that exact endpoint. Named clusters may retain their
// historical cluster+native-name lookup; duplicate cluster labels are already
// marked provider-scoped by the adapter and arrive without ClusterName.
func (index *identityPinIndex) findProxmoxNode(resource Resource, identity ResourceIdentity) (ResourceIdentityPin, bool) {
	if index == nil || resource.Proxmox == nil {
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
	endpoint := proxmoxProviderPinHostname(resource)
	if endpoint != "" {
		if clusterName != "" {
			if pin, ok := resolvePinBucket(index.byClusterHost[clusterHostPinKey(clusterName, endpoint)], endpoint, machineID, dmiUUID); ok {
				return pin, true
			}
		}
		if pin, ok := resolvePinBucket(index.byHostname[endpoint], endpoint, machineID, dmiUUID); ok {
			return pin, true
		}
	}

	if clusterName != "" {
		for _, hostname := range identity.Hostnames {
			hostname = NormalizeFullHostname(hostname)
			if hostname == "" {
				continue
			}
			if pin, ok := resolvePinBucket(index.byClusterHost[clusterHostPinKey(clusterName, hostname)], hostname, machineID, dmiUUID); ok {
				return pin, true
			}
		}
	}
	return ResourceIdentityPin{}, false
}

// proxmoxProviderPinHostname returns endpoint evidence that is sufficiently
// scoped to persist and recover a Proxmox node pin. IP addresses and
// single-label names are commonly reused across independent private networks,
// so neither can distinguish estates on its own. A qualified configured name
// can preserve standalone continuity without falling back to the native node's
// typically short hostname.
func proxmoxProviderPinHostname(resource Resource) string {
	if resource.Proxmox == nil {
		return ""
	}
	endpoint := NormalizeFullHostname(extractHostname(resource.Proxmox.HostURL))
	if endpoint == "" || net.ParseIP(endpoint) != nil || !strings.Contains(endpoint, ".") {
		return ""
	}
	return endpoint
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
func (rr *ResourceRegistry) completeIdentityFromPins(source DataSource, resource Resource, identity ResourceIdentity) ResourceIdentity {
	if CanonicalResourceType(resource.Type) != ResourceTypeAgent {
		return identity
	}
	if strings.TrimSpace(identity.MachineID) != "" && strings.TrimSpace(identity.DMIUUID) != "" {
		return identity
	}
	var pin ResourceIdentityPin
	var ok bool
	if source == SourceProxmox && resource.Proxmox != nil {
		pin, ok = rr.identityPins.findProxmoxNode(resource, identity)
	} else {
		pin, ok = rr.identityPins.find(identity)
	}
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

// successionsFor reports the pinned canonical IDs the given (new or changed)
// pin supersedes: pins for the same physical host whose canonical ID was
// minted in an earlier era of the chooseNewID ladder. Same-machine proof is a
// matching strong key; a machine-keyless pin in the same cluster whose
// hostname is the incoming pin's hostname or its collapsed short form is the
// same host re-derived after the short→full hostname derivation fix. A
// machine-keyless old pin claimed by a machine-keyed incoming pin is the
// common "agent gained /etc/machine-id" upgrade. Rows with a contradicting
// machine key (host reinstalled, cluster slot re-occupied by a different
// machine) never succeed: the new machine must mint fresh, not absorb the
// old host's operator state.
func (index *identityPinIndex) successionsFor(pin ResourceIdentityPin) []CanonicalIDSuccession {
	if index == nil {
		return nil
	}
	pin = pin.normalized()
	if pin.CanonicalID == "" {
		return nil
	}
	var successions []CanonicalIDSuccession
	seen := make(map[string]struct{})
	consider := func(old ResourceIdentityPin) {
		if old.CanonicalID == "" || old.CanonicalID == pin.CanonicalID {
			return
		}
		if _, dup := seen[old.CanonicalID]; dup {
			return
		}
		seen[old.CanonicalID] = struct{}{}
		successions = append(successions, CanonicalIDSuccession{
			OldCanonicalID: old.CanonicalID,
			NewCanonicalID: pin.CanonicalID,
		})
	}
	if pin.MachineID != "" {
		if old, ok := index.byMachineID[pin.MachineID]; ok {
			consider(old)
		}
	}
	if pin.DMIUUID != "" {
		if old, ok := index.byDMIUUID[pin.DMIUUID]; ok {
			consider(old)
		}
	}
	if pin.ClusterName != "" && pin.Hostname != "" {
		shortHostname := NormalizeHostname(pin.Hostname)
		for _, old := range index.byClusterShortHost[clusterHostPinKey(pin.ClusterName, shortHostname)] {
			if old.MachineID != "" || old.DMIUUID != "" {
				// Machine-keyed rows only succeed through their own keys.
				continue
			}
			if old.Hostname != pin.Hostname && old.Hostname != shortHostname {
				// A different FQDN sharing the short name is a different host.
				continue
			}
			consider(old)
		}
	}
	return successions
}

// upsertedOnto returns the row UpsertResourceIdentityPins stores when pin is
// written over existing: an empty field keeps the stored value.
func (pin ResourceIdentityPin) upsertedOnto(existing ResourceIdentityPin) ResourceIdentityPin {
	if pin.MachineID == "" {
		pin.MachineID = existing.MachineID
	}
	if pin.DMIUUID == "" {
		pin.DMIUUID = existing.DMIUUID
	}
	if pin.ClusterName == "" {
		pin.ClusterName = existing.ClusterName
	}
	if pin.Hostname == "" {
		pin.Hostname = existing.Hostname
	}
	return pin
}

// keptOver returns the row a manual-link side writes over existing: like an
// upsert it keeps the stored fields its own pin leaves empty, so evidence
// from earlier observations survives a weaker one, but it never keeps a
// strong key another resource holds. Earlier releases pinned a link's
// primary with the merged identity, so such a key may be the other side's,
// and a key a resource reports now beats one a row only kept, as the store
// already hands a moved key to the new pin.
func (pin ResourceIdentityPin) keptOver(existing ResourceIdentityPin, held map[string]string) ResourceIdentityPin {
	if pin.Hostname == "" && (pin.ClusterName == "" || !heldByOther(held, clusterPinKey(pin.ClusterName, existing.Hostname), pin.CanonicalID)) {
		pin.Hostname = existing.Hostname
	}
	if pin.MachineID == "" && existing.MachineID != "" && !heldByOther(held, machinePinKey(existing.MachineID), pin.CanonicalID) {
		pin.MachineID = existing.MachineID
	}
	if pin.DMIUUID == "" && existing.DMIUUID != "" && !heldByOther(held, dmiPinKey(existing.DMIUUID), pin.CanonicalID) {
		pin.DMIUUID = existing.DMIUUID
	}
	if pin.ClusterName == "" && existing.ClusterName != "" && !heldByOther(held, clusterPinKey(existing.ClusterName, pin.Hostname), pin.CanonicalID) {
		pin.ClusterName = existing.ClusterName
	}
	return pin
}

// withoutKeysHeldElsewhere drops the strong keys another resource holds.
func (pin ResourceIdentityPin) withoutKeysHeldElsewhere(held map[string]string) ResourceIdentityPin {
	if pin.MachineID != "" && heldByOther(held, machinePinKey(pin.MachineID), pin.CanonicalID) {
		pin.MachineID = ""
	}
	if pin.DMIUUID != "" && heldByOther(held, dmiPinKey(pin.DMIUUID), pin.CanonicalID) {
		pin.DMIUUID = ""
	}
	if pin.ClusterName != "" && heldByOther(held, clusterPinKey(pin.ClusterName, pin.Hostname), pin.CanonicalID) {
		pin.ClusterName = ""
	}
	return pin
}

func heldByOther(held map[string]string, key, canonicalID string) bool {
	owner, taken := held[key]
	return taken && owner != canonicalID
}

// ownerKeys lists the strong keys the store lets only one pin hold, matching
// the conflict rule in UpsertResourceIdentityPins.
func (pin ResourceIdentityPin) ownerKeys() []string {
	var keys []string
	if pin.MachineID != "" {
		keys = append(keys, machinePinKey(pin.MachineID))
	}
	if pin.DMIUUID != "" {
		keys = append(keys, dmiPinKey(pin.DMIUUID))
	}
	if pin.ClusterName != "" {
		keys = append(keys, clusterPinKey(pin.ClusterName, pin.Hostname))
	}
	return keys
}

func machinePinKey(machineID string) string { return "machine:" + machineID }

func dmiPinKey(dmiUUID string) string { return "dmi:" + dmiUUID }

func clusterPinKey(clusterName, hostname string) string {
	return "cluster:" + clusterName + "\x00" + hostname
}

// PersistIdentityPins writes the identity pins for the registry's current
// host resources into the resource store. Only new or changed pins are
// written, so steady-state rebuild ticks cost no writes. Call this after a
// rebuild on the durable store-backed registry; ephemeral per-request
// registries consult pins but do not write them.
//
// Each resource is pinned from its own sources. Both sides of a manual link
// are pinned from the identity captured before the link merged them
// (recordLinkOwnPin), and a pinnable folded side keeps its pin under its own
// canonical ID: a link is display intent, so the merged identity must not
// lend one machine's keys to the other's pin, or a later rebuild would
// complete those keys, mint the same canonical ID for both sides and keep
// them merged after the link is gone. A link side's row is rewritten whole
// (see keptOver), because an upsert would keep a key an earlier release
// borrowed into it from the merged identity.
//
// When a new pin supersedes an earlier era's pin for the same physical host
// (see successionsFor), the store re-keys the host's operator-owned rows to
// the new canonical ID before the pin write, so operator intent
// (never-auto-remediate, maintenance windows) and action-audit history
// survive the era change. Successions are skipped while the old canonical ID
// still belongs to a live resource, or to one a manual link folded into
// another: a genuinely short-named host must not have its rows stolen by an
// FQDN sibling. Change-journal rows are never
// rewritten; EraIDs merges those at read time.
func (rr *ResourceRegistry) PersistIdentityPins() {
	if rr.store == nil {
		return
	}

	rr.mu.RLock()
	// held maps each strong key to the resource that holds it this rebuild:
	// a live resource's row (an upsert only ever widens it), each link
	// side's own pin, and then the keys link sides keep from their rows.
	held := make(map[string]string)
	hold := func(pin ResourceIdentityPin) {
		for _, key := range pin.ownerKeys() {
			if _, taken := held[key]; !taken {
				held[key] = pin.CanonicalID
			}
		}
	}
	var pins, linkPins []ResourceIdentityPin
	for id, resource := range rr.resources {
		if own, linked := rr.linkOwnPins[id]; linked {
			if own != nil {
				hold(*own)
				linkPins = append(linkPins, *own)
			}
			continue
		}
		pin, ok := identityPinForResource(resource)
		if !ok {
			continue
		}
		effective := pin
		if existing, known := rr.identityPins.byCanonicalID[pin.CanonicalID]; known {
			effective = pin.upsertedOnto(existing)
		}
		hold(effective)
		pins = append(pins, pin)
	}
	folded := make([]string, 0, len(rr.linkOwnPins))
	for id, own := range rr.linkOwnPins {
		if _, live := rr.resources[id]; !live && own != nil {
			folded = append(folded, id)
		}
	}
	slices.Sort(folded)
	for _, id := range folded {
		// The store gives each strong key one owner. A folded side drops a
		// key a live resource, or an earlier folded side, already holds
		// rather than taking it back on every other rebuild. A live non-link
		// row is upserted, which cannot drop a field, so its kept fields
		// hold their keys here. A side that would lose its machine evidence
		// writes no pin: a cluster-only row reads as a machine-keyless
		// host's, which cluster-slot succession lets a later machine absorb.
		own := *rr.linkOwnPins[id]
		pin := own.withoutKeysHeldElsewhere(held)
		if !pin.hasStrongKey() || (pin.MachineID == "" && pin.DMIUUID == "" && (own.MachineID != "" || own.DMIUUID != "")) {
			continue
		}
		hold(pin)
		linkPins = append(linkPins, pin)
	}
	slices.SortFunc(linkPins, func(a, b ResourceIdentityPin) int { return strings.Compare(a.CanonicalID, b.CanonicalID) })
	for i, pin := range linkPins {
		if existing, known := rr.identityPins.byCanonicalID[pin.CanonicalID]; known {
			linkPins[i] = pin.keptOver(existing, held)
			hold(linkPins[i])
		}
	}

	var successions []CanonicalIDSuccession
	changed := func(batch []ResourceIdentityPin) []ResourceIdentityPin {
		var out []ResourceIdentityPin
		for _, pin := range batch {
			if existing, known := rr.identityPins.byCanonicalID[pin.CanonicalID]; known && existing == pin {
				continue
			}
			out = append(out, pin)
			for _, succession := range rr.identityPins.successionsFor(pin) {
				// A resource a manual link folded into its primary is still
				// observed. Succeeding it would re-key the link onto the
				// primary itself, splitting the pair on the next rebuild.
				if rr.canonicalIDObservedLocked(succession.OldCanonicalID) {
					continue
				}
				successions = append(successions, succession)
			}
		}
		return out
	}
	pins = changed(pins)
	linkPins = changed(linkPins)
	rr.mu.RUnlock()

	if len(pins) == 0 && len(linkPins) == 0 {
		return
	}
	if len(successions) > 0 {
		if successor, ok := rr.store.(canonicalIDSuccessor); ok {
			if err := successor.ApplyCanonicalIDSuccessions(successions); err != nil {
				log.Printf("unifiedresources: failed to apply canonical ID successions: %v", err)
			}
		}
	}
	// Upserts go first: one that deletes a link side's old row, which still
	// held a key the side has now dropped, is then followed by the side's
	// replace, which writes the row again.
	if len(pins) > 0 {
		if err := rr.store.UpsertResourceIdentityPins(pins); err != nil {
			log.Printf("unifiedresources: failed to persist identity pins: %v", err)
			return
		}
	}
	if len(linkPins) > 0 {
		if err := rr.store.ReplaceResourceIdentityPins(linkPins); err != nil {
			log.Printf("unifiedresources: failed to persist manual-link identity pins: %v", err)
			return
		}
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
