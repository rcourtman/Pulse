package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func inferLinkedHostsForProxmoxNodes(nodes []models.Node, hostByID map[string]*models.Host) map[string]*models.Host {
	if len(nodes) == 0 || len(hostByID) == 0 {
		return nil
	}

	nodeByID := make(map[string]*models.Node, len(nodes))
	for i := range nodes {
		nodeID := strings.TrimSpace(nodes[i].ID)
		if nodeID == "" {
			continue
		}
		nodeByID[nodeID] = &nodes[i]
	}

	keyToHostID := make(map[string]string)
	ambiguousKeys := make(map[string]struct{})
	nodeIDToHostID := make(map[string]string)
	ambiguousNodeIDs := make(map[string]struct{})
	trustedHostIDs := make(map[string]struct{})
	hostClusterByID := make(map[string]string)
	hostClusterAmbiguous := make(map[string]struct{})
	hostProviderNodeByID := make(map[string]models.Node)
	hostProviderNodeAmbiguous := make(map[string]struct{})
	recordHostCluster := func(hostID string, node models.Node) {
		hostID = strings.TrimSpace(hostID)
		cluster := strings.TrimSpace(strings.ToLower(node.ClusterName))
		if hostID == "" || cluster == "" {
			return
		}
		if _, ambiguous := hostClusterAmbiguous[hostID]; ambiguous {
			return
		}
		if existing, ok := hostClusterByID[hostID]; ok && existing != cluster {
			delete(hostClusterByID, hostID)
			hostClusterAmbiguous[hostID] = struct{}{}
			return
		}
		hostClusterByID[hostID] = cluster
	}
	// Short hostnames and short endpoint aliases collide across estates
	// (#1753: pve01 in staging vs pve01 in production), so a host whose
	// trusted link pins it to one cluster must never be inferred for a node
	// from a different cluster, whichever inference path proposed it.
	hostClusterConflicts := func(hostID string, node models.Node) bool {
		hostCluster := hostClusterByID[strings.TrimSpace(hostID)]
		if hostCluster == "" {
			return false
		}
		nodeCluster := strings.TrimSpace(strings.ToLower(node.ClusterName))
		return nodeCluster != "" && nodeCluster != hostCluster
	}
	recordHostProviderNode := func(hostID string, node models.Node) {
		hostID = strings.TrimSpace(hostID)
		if hostID == "" {
			return
		}
		if _, ambiguous := hostProviderNodeAmbiguous[hostID]; ambiguous {
			return
		}
		if existing, ok := hostProviderNodeByID[hostID]; ok &&
			!proxmoxProviderNodesProveSameMachine(existing, node, hostByID[hostID]) {
			delete(hostProviderNodeByID, hostID)
			hostProviderNodeAmbiguous[hostID] = struct{}{}
			return
		}
		hostProviderNodeByID[hostID] = node
	}
	hostProviderConflicts := func(hostID string, node models.Node) bool {
		hostID = strings.TrimSpace(hostID)
		if _, ambiguous := hostProviderNodeAmbiguous[hostID]; ambiguous {
			return true
		}
		existing, ok := hostProviderNodeByID[strings.TrimSpace(hostID)]
		if !ok {
			return false
		}
		existingInstance := strings.TrimSpace(strings.ToLower(existing.Instance))
		candidateInstance := strings.TrimSpace(strings.ToLower(node.Instance))
		if existingInstance == "" || candidateInstance == "" || existingInstance == candidateInstance {
			return false
		}
		return !proxmoxProviderNodesProveSameMachine(existing, node, hostByID[hostID])
	}
	register := func(key, hostID string) {
		key = strings.TrimSpace(key)
		hostID = strings.TrimSpace(hostID)
		if key == "" || hostID == "" {
			return
		}
		if _, ambiguous := ambiguousKeys[key]; ambiguous {
			return
		}
		if existing, ok := keyToHostID[key]; ok && existing != hostID {
			delete(keyToHostID, key)
			ambiguousKeys[key] = struct{}{}
			return
		}
		keyToHostID[key] = hostID
	}
	registerNode := func(nodeID, hostID string) {
		nodeID = strings.TrimSpace(nodeID)
		hostID = strings.TrimSpace(hostID)
		if nodeID == "" || hostID == "" {
			return
		}
		if _, ambiguous := ambiguousNodeIDs[nodeID]; ambiguous {
			return
		}
		if existing, ok := nodeIDToHostID[nodeID]; ok && existing != hostID {
			delete(nodeIDToHostID, nodeID)
			ambiguousNodeIDs[nodeID] = struct{}{}
			return
		}
		nodeIDToHostID[nodeID] = hostID
	}

	for _, node := range nodes {
		hostID := strings.TrimSpace(node.LinkedAgentID)
		if hostID == "" {
			continue
		}
		host := hostByID[hostID]
		if host == nil || !trustedProxmoxNodeHostLink(node, *host) {
			continue
		}
		trustedHostIDs[hostID] = struct{}{}
		recordHostCluster(hostID, node)
		recordHostProviderNode(hostID, node)
		for _, key := range proxmoxNodeLinkKeys(node) {
			register(key, hostID)
		}
	}

	for hostID, host := range hostByID {
		if host == nil {
			continue
		}
		nodeID := strings.TrimSpace(host.LinkedNodeID)
		if nodeID == "" {
			continue
		}
		node := nodeByID[nodeID]
		if node == nil || !trustedHostProxmoxNodeLink(*host, *node) {
			continue
		}
		trustedHostIDs[hostID] = struct{}{}
		recordHostCluster(hostID, *node)
		recordHostProviderNode(hostID, *node)
		registerNode(nodeID, hostID)
		for _, key := range proxmoxNodeLinkKeys(*node) {
			register(key, hostID)
		}
	}

	out := make(map[string]*models.Host, len(nodes))
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			continue
		}

		if hostID := strings.TrimSpace(node.LinkedAgentID); hostID != "" {
			if host := hostByID[hostID]; host != nil && trustedProxmoxNodeHostLink(node, *host) {
				out[nodeID] = host
				continue
			}
		}

		if _, ambiguous := ambiguousNodeIDs[nodeID]; !ambiguous {
			if hostID := strings.TrimSpace(nodeIDToHostID[nodeID]); hostID != "" {
				if host := hostByID[hostID]; host != nil {
					out[nodeID] = host
					continue
				}
			}
		}

		inferredHostID := ""
		for _, key := range proxmoxNodeLinkKeys(node) {
			if _, ambiguous := ambiguousKeys[key]; ambiguous {
				continue
			}
			hostID := strings.TrimSpace(keyToHostID[key])
			if hostID == "" || hostClusterConflicts(hostID, node) || hostProviderConflicts(hostID, node) {
				continue
			}
			if inferredHostID != "" && inferredHostID != hostID {
				inferredHostID = ""
				break
			}
			inferredHostID = hostID
		}
		if inferredHostID == "" {
			for hostID := range trustedHostIDs {
				host := hostByID[hostID]
				if host == nil ||
					proxmoxNodeUsesProviderScopedIdentity(node) ||
					!proxmoxNodeCorroboratesHost(node, *host) {
					continue
				}
				if hostClusterConflicts(hostID, node) {
					continue
				}
				if hostProviderConflicts(hostID, node) {
					continue
				}
				if inferredHostID != "" && inferredHostID != hostID {
					inferredHostID = ""
					break
				}
				inferredHostID = hostID
			}
			if inferredHostID == "" {
				continue
			}
		}
		if host := hostByID[inferredHostID]; host != nil {
			out[nodeID] = host
		}
	}

	return out
}

// proxmoxProviderNodesProveSameMachine is the provider-scope boundary for
// lending one trusted host-agent identity to another node view. Native PVE
// names are commonly short and repeat across independent standalone sites, so
// a shared name is deliberately absent here. Distinct configured instances
// may share a host only when provider-owned evidence says they are the same
// (the same node identity, named cluster, or exact configured endpoint), or
// when both views independently match the trusted host's full endpoint/IP.
func proxmoxProviderNodesProveSameMachine(left, right models.Node, host *models.Host) bool {
	if leftID, rightID := strings.TrimSpace(left.ID), strings.TrimSpace(right.ID); leftID != "" && leftID == rightID {
		return true
	}
	if leftIdentity, rightIdentity := strings.TrimSpace(left.NodeIdentity), strings.TrimSpace(right.NodeIdentity); leftIdentity != "" && leftIdentity == rightIdentity {
		return true
	}
	leftInstance := strings.TrimSpace(strings.ToLower(left.Instance))
	rightInstance := strings.TrimSpace(strings.ToLower(right.Instance))
	if leftInstance != "" && leftInstance == rightInstance {
		return true
	}
	leftCluster := strings.TrimSpace(strings.ToLower(left.ClusterName))
	rightCluster := strings.TrimSpace(strings.ToLower(right.ClusterName))
	if leftCluster != "" && leftCluster == rightCluster {
		return true
	}
	leftEndpoint := strings.TrimSpace(strings.ToLower(extractHostname(left.Host)))
	rightEndpoint := strings.TrimSpace(strings.ToLower(extractHostname(right.Host)))
	if leftEndpoint != "" && leftEndpoint == rightEndpoint {
		return true
	}
	return host != nil &&
		proxmoxNodeStronglyCorroboratesHost(left, *host) &&
		proxmoxNodeStronglyCorroboratesHost(right, *host)
}

func proxmoxNodeStronglyCorroboratesHost(node models.Node, host models.Host) bool {
	hostIPs := make(map[string]struct{})
	if ip := NormalizeIP(host.ReportIP); ip != "" {
		hostIPs[ip] = struct{}{}
	}
	for _, network := range host.NetworkInterfaces {
		for _, address := range network.Addresses {
			if ip := NormalizeIP(address); ip != "" {
				hostIPs[ip] = struct{}{}
			}
		}
	}

	endpoint := strings.TrimSpace(strings.ToLower(extractHostname(node.Host)))
	if endpointIP := NormalizeIP(endpoint); endpointIP != "" {
		_, ok := hostIPs[endpointIP]
		return ok
	}
	hostname := NormalizeFullHostname(host.Hostname)
	if strings.Contains(endpoint, ".") && endpoint == hostname {
		return true
	}
	if nodeName := NormalizeFullHostname(node.Name); strings.Contains(nodeName, ".") && nodeName == hostname {
		return true
	}
	for _, network := range node.NetworkInterfaces {
		for _, address := range network.Addresses {
			if ip := NormalizeIP(address); ip != "" {
				if _, ok := hostIPs[ip]; ok {
					return true
				}
			}
		}
	}
	return false
}

func proxmoxNodeLinkKeys(node models.Node) []string {
	name := NormalizeHostname(node.Name)
	if name == "" {
		return nil
	}

	keys := make([]string, 0, 4)
	if proxmoxNodeUsesProviderScopedIdentity(node) {
		instance := strings.TrimSpace(strings.ToLower(node.Instance))
		if instance == "" {
			return nil
		}
		return []string{"provider:" + instance + ":" + name}
	}
	if cluster := strings.TrimSpace(strings.ToLower(node.ClusterName)); cluster != "" {
		keys = append(keys, "cluster:"+cluster+":"+name)
	}

	if endpoint := strings.TrimSpace(strings.ToLower(extractHostname(node.Host))); endpoint != "" {
		keys = append(keys, "endpoint-host:"+endpoint+":"+name)
		if short := NormalizeHostname(endpoint); short != "" && short != endpoint {
			keys = append(keys, "endpoint-host:"+short+":"+name)
		}
		if ip := NormalizeIP(endpoint); ip != "" {
			keys = append(keys, "endpoint-ip:"+ip+":"+name)
		}
	}

	return uniqueStrings(keys)
}

func trustedProxmoxNodeHostLink(node models.Node, host models.Host) bool {
	if strings.TrimSpace(host.LinkedNodeID) == strings.TrimSpace(node.ID) && strings.TrimSpace(node.ID) != "" {
		return true
	}
	if proxmoxNodeUsesProviderScopedIdentity(node) {
		return false
	}
	return proxmoxNodeCorroboratesHost(node, host)
}

func trustedHostProxmoxNodeLink(host models.Host, node models.Node) bool {
	hostLinkedNodeID := strings.TrimSpace(host.LinkedNodeID)
	nodeID := strings.TrimSpace(node.ID)
	if hostLinkedNodeID == "" || nodeID == "" || hostLinkedNodeID != nodeID {
		return false
	}
	if strings.TrimSpace(node.LinkedAgentID) == strings.TrimSpace(host.ID) && strings.TrimSpace(host.ID) != "" {
		return true
	}
	return proxmoxNodeCorroboratesHost(node, host)
}

func proxmoxNodeCorroboratesHost(node models.Node, host models.Host) bool {
	nodeName := NormalizeHostname(node.Name)
	hostName := NormalizeHostname(host.Hostname)
	if nodeName != "" && hostName != "" && nodeName == hostName {
		return true
	}

	endpoint := strings.TrimSpace(strings.ToLower(extractHostname(node.Host)))
	if endpoint == "" {
		return false
	}

	if ip := NormalizeIP(endpoint); ip != "" {
		if NormalizeIP(host.ReportIP) == ip {
			return true
		}
		for _, iface := range host.NetworkInterfaces {
			for _, address := range iface.Addresses {
				if NormalizeIP(address) == ip {
					return true
				}
			}
		}
		return false
	}

	endpointHost := NormalizeHostname(endpoint)
	return endpointHost != "" && endpointHost == hostName
}
