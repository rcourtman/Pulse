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
			if hostID == "" {
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
		if host := hostByID[inferredHostID]; host != nil {
			out[nodeID] = host
		}
	}

	return out
}

func proxmoxNodeLinkKeys(node models.Node) []string {
	name := NormalizeHostname(node.Name)
	if name == "" {
		return nil
	}

	keys := make([]string, 0, 4)
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
