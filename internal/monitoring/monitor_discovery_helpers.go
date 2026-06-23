package monitoring

import (
	"net"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

var lookupConfiguredHostIP = net.LookupIP

// getConfiguredHostIPs returns a list of IP addresses from all configured Proxmox hosts.
// This is used to prevent discovery from probing hosts we already know about.
func (m *Monitor) getConfiguredHostIPs() []string {
	m.mu.RLock()
	if m.config == nil {
		m.mu.RUnlock()
		return nil
	}
	cfg := m.config.DeepCopy()
	m.mu.RUnlock()

	return configuredHostIPsFromConfig(cfg)
}

func configuredHostIPsFromConfig(cfg *config.Config) []string {
	if cfg == nil {
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
		if addrs, err := lookupConfiguredHostIP(host); err == nil && len(addrs) > 0 {
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
	for _, pve := range cfg.PVEInstances {
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
	for _, pbs := range cfg.PBSInstances {
		addHost(pbs.Host)
	}

	// Add PMG hosts
	for _, pmg := range cfg.PMGInstances {
		addHost(pmg.Host)
	}

	return ips
}

func mergeDiscoveryIPBlocklist(existing []string, discovered []string) []string {
	if len(existing) == 0 && len(discovered) == 0 {
		return nil
	}

	merged := make([]string, 0, len(existing)+len(discovered))
	seen := make(map[string]struct{}, len(existing)+len(discovered))
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		merged = append(merged, ip)
	}

	for _, ip := range existing {
		add(ip)
	}
	for _, ip := range discovered {
		add(ip)
	}
	return merged
}

func (m *Monitor) discoveryConfigSnapshot() config.DiscoveryConfig {
	m.mu.RLock()
	if m.config == nil {
		m.mu.RUnlock()
		return config.DefaultDiscoveryConfig()
	}
	cfg := m.config.DeepCopy()
	m.mu.RUnlock()

	discoveryCfg := config.CloneDiscoveryConfig(cfg.Discovery)
	discoveryCfg.IPBlocklist = mergeDiscoveryIPBlocklist(discoveryCfg.IPBlocklist, configuredHostIPsFromConfig(cfg))
	return discoveryCfg
}
