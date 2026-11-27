package unifiedresources

import (
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const hostSocketPeerRelationshipDiscoverer = "host_socket_table"

var socketPeerCGNATPrefix = netip.MustParsePrefix("100.64.0.0/10")

type hostActivePeerSnapshot struct {
	hostID   string
	lastSeen time.Time
	peers    []models.HostNetworkPeer
}

func hostActivePeerSnapshotsOf(hosts []models.Host) []hostActivePeerSnapshot {
	if len(hosts) == 0 {
		return nil
	}
	out := make([]hostActivePeerSnapshot, 0, len(hosts))
	for _, host := range hosts {
		if len(host.ActivePeers) == 0 {
			continue
		}
		out = append(out, hostActivePeerSnapshot{
			hostID:   host.ID,
			lastSeen: host.LastSeen,
			peers:    append([]models.HostNetworkPeer(nil), host.ActivePeers...),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// refreshHostSocketPeerRelationshipsLocked projects passive internal TCP peers
// reported by host agents into canonical RelCommunicatesWith relationships
// strictly between monitored resources in this registry. Unknown LAN IPs and
// IPs shared ambiguously by multiple resources are skipped.
func (rr *ResourceRegistry) refreshHostSocketPeerRelationshipsLocked() {
	if !rr.hostActivePeersRead {
		return
	}
	// Index every unambiguous internal IP address owned by a monitored
	// compute/infrastructure resource in this registry.
	ipOwners := make(map[string]map[string]struct{})
	addResourceIP := func(resourceID, rawIP string) {
		addr, ok := normalizeSocketPeerIP(rawIP)
		if !ok {
			return
		}
		owners := ipOwners[addr]
		if owners == nil {
			owners = make(map[string]struct{}, 1)
			ipOwners[addr] = owners
		}
		owners[resourceID] = struct{}{}
	}

	for id, resource := range rr.resources {
		if resource == nil || !eligibleSocketPeerTarget(resource.Type) {
			continue
		}
		for _, ip := range resource.Identity.IPAddresses {
			addResourceIP(id, ip)
		}
		if resource.Agent != nil {
			for _, nic := range resource.Agent.NetworkInterfaces {
				for _, ip := range nic.Addresses {
					addResourceIP(id, ip)
				}
			}
		}
		if resource.Proxmox != nil {
			for _, nic := range resource.Proxmox.NetworkInterfaces {
				for _, ip := range nic.Addresses {
					addResourceIP(id, ip)
				}
			}
		}
	}

	uniqueResourceByIP := make(map[string]string, len(ipOwners))
	for ip, owners := range ipOwners {
		if len(owners) != 1 {
			continue
		}
		for ownerID := range owners {
			uniqueResourceByIP[ip] = ownerID
		}
	}

	type edgeKey struct {
		sourceID string
		targetID string
	}
	type edgeAcc struct {
		ports      map[int]struct{}
		observedAt time.Time
	}
	edges := make(map[edgeKey]*edgeAcc)

	agentSource := rr.bySource[SourceAgent]
	for _, host := range rr.hostActivePeers {
		if len(host.peers) == 0 {
			continue
		}
		sourceID := agentSource[normalizeSourceID(host.hostID)]
		if sourceID == "" {
			continue
		}
		hostResource := rr.resources[sourceID]
		if hostResource == nil {
			continue
		}
		observedAt := host.lastSeen
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}

		for _, peer := range host.peers {
			remoteIP, ok := normalizeSocketPeerIP(peer.RemoteIP)
			if !ok {
				continue
			}
			peerResourceID := uniqueResourceByIP[remoteIP]
			if peerResourceID == "" || peerResourceID == sourceID {
				continue
			}
			peerResource := rr.resources[peerResourceID]
			if peerResource == nil {
				continue
			}
			// Skip direct parent-child links (a host talking to a guest
			// already contained inside it); cross-host and cross-guest edges
			// carry the real architectural dependency signal.
			if isDirectParentChild(hostResource, peerResource) {
				continue
			}

			var edgeSourceID, edgeTargetID string
			var port int
			if peer.Inbound {
				if peer.LocalPort == 0 {
					continue
				}
				edgeSourceID = peerResourceID
				edgeTargetID = sourceID
				port = int(peer.LocalPort)
			} else {
				if peer.RemotePort == 0 {
					continue
				}
				edgeSourceID = sourceID
				edgeTargetID = peerResourceID
				port = int(peer.RemotePort)
			}

			key := edgeKey{sourceID: edgeSourceID, targetID: edgeTargetID}
			acc := edges[key]
			if acc == nil {
				acc = &edgeAcc{
					ports:      make(map[int]struct{}, 2),
					observedAt: observedAt,
				}
				edges[key] = acc
			}
			acc.ports[port] = struct{}{}
			if observedAt.After(acc.observedAt) {
				acc.observedAt = observedAt
			}
		}
	}

	relationshipsBySource := make(map[string][]ResourceRelationship, len(edges))
	for key, acc := range edges {
		ports := make([]int, 0, len(acc.ports))
		for port := range acc.ports {
			ports = append(ports, port)
		}
		sort.Ints(ports)
		if len(ports) > 8 {
			ports = ports[:8]
		}
		rel := ResourceRelationship{
			SourceID:   key.sourceID,
			TargetID:   key.targetID,
			Type:       RelCommunicatesWith,
			Confidence: 0.9,
			Active:     true,
			Discoverer: hostSocketPeerRelationshipDiscoverer,
			ObservedAt: acc.observedAt,
			LastSeenAt: acc.observedAt,
			Metadata: map[string]any{
				"port":     ports[0],
				"ports":    ports,
				"protocol": "tcp",
			},
		}
		normalizeResourceRelationship(&rel)
		relationshipsBySource[key.sourceID] = append(relationshipsBySource[key.sourceID], rel)
	}

	changed := false
	for id, resource := range rr.resources {
		if resource == nil {
			continue
		}
		nextEdges := relationshipsBySource[id]
		sort.Slice(nextEdges, func(i, j int) bool {
			return nextEdges[i].TargetID < nextEdges[j].TargetID
		})
		if rr.setHostSocketPeerRelationshipsLocked(resource, nextEdges) {
			changed = true
		}
	}
	if changed {
		rr.invalidateViewsLocked()
	}
}

func (rr *ResourceRegistry) setHostSocketPeerRelationshipsLocked(resource *Resource, nextEdges []ResourceRelationship) bool {
	if resource == nil {
		return false
	}

	hadExisting := false
	base := make([]ResourceRelationship, 0, len(resource.Relationships)+len(nextEdges))
	existingByTarget := make(map[string]ResourceRelationship)
	for _, rel := range resource.Relationships {
		if rel.Discoverer == hostSocketPeerRelationshipDiscoverer && rel.Type == RelCommunicatesWith {
			hadExisting = true
			existingByTarget[CanonicalResourceID(rel.TargetID)] = rel
			continue
		}
		base = append(base, rel)
	}
	if !hadExisting && len(nextEdges) == 0 {
		return false
	}

	// Preserve ObservedAt from an already-active edge to the same target so
	// poll refreshes do not churn first-seen timestamps.
	for i := range nextEdges {
		if prev, ok := existingByTarget[CanonicalResourceID(nextEdges[i].TargetID)]; ok && !prev.ObservedAt.IsZero() {
			nextEdges[i].ObservedAt = prev.ObservedAt
		}
	}

	resource.Relationships = append(base, nextEdges...)
	normalizeResourceRelationships(resource)
	return true
}

func eligibleSocketPeerTarget(resourceType ResourceType) bool {
	switch CanonicalResourceType(resourceType) {
	case ResourceTypeAgent,
		ResourceTypeK8sNode,
		ResourceTypeVM,
		ResourceTypeSystemContainer,
		ResourceTypeAppContainer,
		ResourceTypePod,
		ResourceTypePBS,
		ResourceTypePMG:
		return true
	default:
		return false
	}
}

func isDirectParentChild(a, b *Resource) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ParentID != nil && CanonicalResourceID(*a.ParentID) == CanonicalResourceID(b.ID) {
		return true
	}
	if b.ParentID != nil && CanonicalResourceID(*b.ParentID) == CanonicalResourceID(a.ID) {
		return true
	}
	return false
}

func normalizeSocketPeerIP(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		trimmed = trimmed[:slash]
	}
	addr, err := netip.ParseAddr(trimmed)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	if !addr.IsValid() ||
		addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() {
		return "", false
	}
	if !addr.IsPrivate() && !socketPeerCGNATPrefix.Contains(addr) {
		return "", false
	}
	return addr.String(), true
}
