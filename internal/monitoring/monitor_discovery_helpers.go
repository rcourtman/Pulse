package monitoring

import (
	"net"
	"strings"
)

// getConfiguredHostIPs returns a list of IP addresses from all configured Proxmox hosts.
// This is used to prevent discovery from probing hosts we already know about.
// Caller must hold m.mu.RLock or m.mu.Lock.
func (m *Monitor) getConfiguredHostIPs() []string {
	if m.config == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var ips []string

	addHost := func(host string) {
		// Parse the host to extract IP/hostname
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		// Remove scheme if present
		if strings.HasPrefix(host, "https://") {
			host = strings.TrimPrefix(host, "https://")
		} else if strings.HasPrefix(host, "http://") {
			host = strings.TrimPrefix(host, "http://")
		}
		// Remove port if present
		if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
			// Check if it's an IPv6 address
			if !strings.Contains(host[colonIdx:], "]") {
				host = host[:colonIdx]
			}
		}
		// Remove trailing path
		if slashIdx := strings.Index(host, "/"); slashIdx != -1 {
			host = host[:slashIdx]
		}
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		// Check if it's already an IP
		if ip := net.ParseIP(host); ip != nil {
			if _, exists := seen[host]; !exists {
				seen[host] = struct{}{}
				ips = append(ips, host)
			}
			return
		}
		// Try to resolve hostname to IP
		if addrs, err := net.LookupIP(host); err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				// Prefer IPv4
				if v4 := addr.To4(); v4 != nil {
					ipStr := v4.String()
					if _, exists := seen[ipStr]; !exists {
						seen[ipStr] = struct{}{}
						ips = append(ips, ipStr)
					}
					break
				}
			}
		}
	}

	// Add PVE hosts
	for _, pve := range m.config.PVEInstances {
		addHost(pve.Host)
		// Also add cluster endpoints (include both auto-discovered IP and override if set)
		for _, ep := range pve.ClusterEndpoints {
			addHost(ep.Host)
			addHost(ep.IP)
			if ep.IPOverride != "" && ep.IPOverride != ep.IP {
				addHost(ep.IPOverride)
			}
		}
	}

	// Add PBS hosts
	for _, pbs := range m.config.PBSInstances {
		addHost(pbs.Host)
	}

	// Add PMG hosts
	for _, pmg := range m.config.PMGInstances {
		addHost(pmg.Host)
	}

	return ips
}
