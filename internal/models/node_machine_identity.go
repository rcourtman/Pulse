package models

import (
	"net"
	"strings"
)

// NodeLinkAddress normalizes an address that can help prove a host agent and a
// Proxmox node are one machine, and returns "" for any other. Loopback and
// link-local addresses exist on unrelated machines.
func NodeLinkAddress(address string) string {
	normalized := normalizeIPAddress(address)
	if ip := net.ParseIP(normalized); ip != nil && ip.IsGlobalUnicast() {
		return normalized
	}
	return ""
}

// IsHostLocalLinkInterface reports whether an interface's addresses are repeated
// independently on each machine, so they cannot help prove one. Not every bridge
// is: Proxmox management addresses commonly use vmbr0, and operators may use names
// such as br-mgmt. Docker's automatically named user-defined bridges use br-
// followed by the first 12 hex characters of the network ID.
func IsHostLocalLinkInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "lo" || strings.HasPrefix(name, "docker") {
		return true
	}

	bridgeID, found := strings.CutPrefix(name, "br-")
	if !found || len(bridgeID) != 12 {
		return false
	}
	for _, character := range bridgeID {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// LinkAddressCanRepeatAcrossSites reports whether separate sites can both use an
// address: the private ranges, and the shared range CGNAT and overlay networks hand
// out.
func LinkAddressCanRepeatAcrossSites(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64
}

func linkNetworkAddresses(network []HostNetworkInterface) map[string]struct{} {
	addresses := make(map[string]struct{})
	for _, nic := range network {
		if IsHostLocalLinkInterface(nic.Name) {
			continue
		}
		for _, address := range nic.Addresses {
			if normalized := NodeLinkAddress(address); normalized != "" {
				addresses[normalized] = struct{}{}
			}
		}
	}
	return addresses
}

// nodeInventoryIndex records which nodes list each address in the network
// inventory Proxmox itself reports for them, and which nodes list none.
type nodeInventoryIndex struct {
	holders map[string][]Node
	unknown map[string][]Node // by short node name
}

func newNodeInventoryIndex(nodeSets ...map[string]Node) nodeInventoryIndex {
	index := nodeInventoryIndex{holders: make(map[string][]Node), unknown: make(map[string][]Node)}
	for _, nodes := range nodeSets {
		for _, node := range nodes {
			addresses := linkNetworkAddresses(node.NetworkInterfaces)
			if len(addresses) == 0 {
				name := shortHostname(node.Name)
				index.unknown[name] = append(index.unknown[name], node)
			}
			for address := range addresses {
				index.holders[address] = append(index.holders[address], node)
			}
		}
	}
	return index
}

// corroborates reports whether a node's inventory ties it to a host agent: it
// lists an address the agent reports that no other machine's inventory lists. A
// shared address proves nothing: sites reuse private ranges and every node can
// show one bridge address. A private address also takes more than that, because
// any machine at another site can hold it, monitored or not: the agent must carry
// the node's name, and no same-named node may be one whose addresses Proxmox has
// not told us.
func (index nodeInventoryIndex) corroborates(node Node, host Host) bool {
	reported := linkNetworkAddresses(host.NetworkInterfaces)
	if address := NodeLinkAddress(host.ReportIP); address != "" {
		reported[address] = struct{}{}
	}
	namedAlike := hostCarriesNodeName(host, node) &&
		!anotherMachineAmong(index.unknown[shortHostname(node.Name)], node)
	for address := range linkNetworkAddresses(node.NetworkInterfaces) {
		if _, ok := reported[address]; !ok || anotherMachineAmong(index.holders[address], node) {
			continue
		}
		if namedAlike || !LinkAddressCanRepeatAcrossSites(address) {
			return true
		}
	}
	return false
}

func anotherMachineAmong(nodes []Node, node Node) bool {
	for _, other := range nodes {
		if !NodeObservationsSameMachine(node, other) {
			return true
		}
	}
	return false
}

// hostCarriesNodeName reports whether an agent's hostname and a node's name can be
// one machine's. One may be the other's short form, but two different full
// hostnames stay different.
func hostCarriesNodeName(host Host, node Node) bool {
	hostname := normalizeNodeIdentityPart(host.Hostname)
	name := normalizeNodeIdentityPart(node.Name)
	if hostname == "" || name == "" {
		return false
	}
	if hostname == name {
		return true
	}
	short := shortHostname(hostname)
	return short == shortHostname(name) && (hostname == short || name == short)
}

// NodeObservationsSameMachine reports whether two observations of a Proxmox
// node are one machine by the evidence UpdateNodesForInstance folds node views
// into one slot on: the same logical key, or the same node name with no
// provider identity conflict and either positive cross-view proof (the same
// connection, the same cluster with matching TLS fingerprints, matching
// fingerprints, or for unclustered views the same endpoint) or a shared
// endpoint address. A cluster added through two connections is such a pair.
// Callers that see a polled node before it is merged, such as the node's host
// agent lookup, use this so they agree with the state on which views are the
// same node.
func NodeObservationsSameMachine(a, b Node) bool {
	if key := nodeLogicalKey(a); key != "" && key == nodeLogicalKey(b) {
		return true
	}
	name := normalizeNodeIdentityPart(a.Name)
	if name == "" || name != normalizeNodeIdentityPart(b.Name) || nodeProviderIdentityConflicts(a, b) {
		return false
	}
	if nodeCrossViewMergeProven(a, b) {
		return true
	}
	// An address alias folds on the conflict check alone (resolveNodeMergeKey).
	otherAliases := nodeEndpointMergeAliases(b)
	for _, alias := range nodeEndpointMergeAliases(a) {
		if !strings.HasPrefix(alias, "endpoint-ip:") {
			continue
		}
		for _, other := range otherAliases {
			if alias == other {
				return true
			}
		}
	}
	return false
}

// HostAgentBridgesNodeViews reports whether a host agent linked to view a proves
// view b is the same machine, the way UpdateNodesForInstance lets that agent
// fold b into a's slot: neither view names a cluster, their TLS fingerprints do
// not contradict each other (the state rejects an agent for a node whose known
// fingerprint differs from its linked node's), and the agent's own report names
// both endpoints, by exact address or full hostname. A host with several
// addresses added through two standalone connections is such a pair.
func HostAgentBridgesNodeViews(host Host, a, b Node) bool {
	if normalizeNodeIdentityPart(a.ClusterName) != "" || normalizeNodeIdentityPart(b.ClusterName) != "" {
		return false
	}
	fpA := normalizeNodeTLSFingerprint(a.TLSFingerprint)
	fpB := normalizeNodeTLSFingerprint(b.TLSFingerprint)
	if fpA != "" && fpB != "" && fpA != fpB {
		return false
	}
	return hostReportsNodeEndpoint(host, a) && hostReportsNodeEndpoint(host, b)
}

// hostReportsNodeEndpoint reports whether a host agent's own report names the
// node's endpoint: the endpoint address among the addresses it reports, or the
// endpoint's full (dotted) hostname as its hostname.
func hostReportsNodeEndpoint(host Host, node Node) bool {
	endpoint := extractHostEndpoint(node.Host)
	if endpoint == "" {
		return false
	}
	if ip := normalizeIPAddress(endpoint); ip != "" {
		if normalizeIPAddress(host.ReportIP) == ip {
			return true
		}
		for _, iface := range host.NetworkInterfaces {
			for _, address := range iface.Addresses {
				if normalizeIPAddress(address) == ip {
					return true
				}
			}
		}
		return false
	}
	return strings.Contains(endpoint, ".") && endpoint == strings.TrimSpace(strings.ToLower(host.Hostname))
}
