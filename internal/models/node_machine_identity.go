package models

import "strings"

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
