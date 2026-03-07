package unifiedresources

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func inferLinkedHostsForProxmoxNodes(nodes []models.Node, hostByID map[string]*models.Host) map[string]*models.Host {
	if len(nodes) == 0 || len(hostByID) == 0 {
		return nil
	}

	keyToHostID := make(map[string]string)
	ambiguousKeys := make(map[string]struct{})
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
