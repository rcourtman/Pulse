package monitoring

import (
	"net"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

var lookupIPFunc = net.LookupIP

func lookupClusterEndpointLabel(instance *config.PVEInstance, nodeName string) string {
	if instance == nil {
		return ""
	}

	for _, endpoint := range instance.ClusterEndpoints {
		if !strings.EqualFold(endpoint.NodeName, nodeName) {
			continue
		}

		if host := strings.TrimSpace(endpoint.Host); host != "" {
			if label := normalizeEndpointHost(host); label != "" && !isLikelyIPAddress(label) {
				return label
			}
		}

		if nodeNameLabel := strings.TrimSpace(endpoint.NodeName); nodeNameLabel != "" {
			return nodeNameLabel
		}

		if ip := strings.TrimSpace(endpoint.IP); ip != "" {
			return ip
		}
	}

	return ""
}

func normalizeEndpointHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		host := parsed.Hostname()
		if host != "" {
			return host
		}
		return parsed.Host
	}

	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if idx := strings.Index(value, "/"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}

	if idx := strings.Index(value, ":"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}

	return value
}

func isLikelyIPAddress(value string) bool {
	if value == "" {
		return false
	}

	if ip := net.ParseIP(value); ip != nil {
		return true
	}

	// Handle IPv6 with zone identifier (fe80::1%eth0)
	if i := strings.Index(value, "%"); i > 0 {
		if ip := net.ParseIP(value[:i]); ip != nil {
			return true
		}
	}

	return false
}

func ensureClusterEndpointURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return value
	}

	if _, _, err := net.SplitHostPort(value); err == nil {
		return "https://" + value
	}

	return "https://" + net.JoinHostPort(value, "8006")
}

func clusterEndpointEffectiveURL(endpoint config.ClusterEndpoint, verifySSL bool, hasFingerprint bool) string {
	// When TLS hostname verification is required (VerifySSL=true and no fingerprint),
	// prefer hostname over IP to ensure certificate CN/SAN validation works correctly.
	// When TLS is not verified (VerifySSL=false) or a fingerprint is provided (which
	// bypasses hostname checks), prefer IP to reduce DNS lookups (refs #620).
	requiresHostnameForTLS := verifySSL && !hasFingerprint

	// Use EffectiveIP() which prefers user-specified IPOverride over auto-discovered IP
	effectiveIP := endpoint.EffectiveIP()

	if requiresHostnameForTLS {
		// Prefer hostname for proper TLS certificate validation
		if endpoint.Host != "" {
			return ensureClusterEndpointURL(endpoint.Host)
		}
		if effectiveIP != "" {
			return ensureClusterEndpointURL(effectiveIP)
		}
	} else {
		// Prefer IP address to avoid excessive DNS lookups
		if effectiveIP != "" {
			return ensureClusterEndpointURL(effectiveIP)
		}
		if endpoint.Host != "" {
			return ensureClusterEndpointURL(endpoint.Host)
		}
	}
	return ""
}

func discoveryPolicyCIDRs(cidrs []string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		networks = append(networks, network)
	}
	return networks
}

func discoveryPolicyBlockedIPs(ips []string) map[string]struct{} {
	blocked := make(map[string]struct{}, len(ips))
	for _, raw := range ips {
		ip := net.ParseIP(strings.TrimSpace(raw))
		if ip == nil {
			continue
		}
		blocked[ip.String()] = struct{}{}
	}
	return blocked
}

func discoveryPolicyAllowsIP(ip net.IP, allowlist, blocklist []*net.IPNet, blockedIPs map[string]struct{}) bool {
	if ip == nil {
		return false
	}

	if _, blocked := blockedIPs[ip.String()]; blocked {
		return false
	}

	for _, network := range blocklist {
		if network.Contains(ip) {
			return false
		}
	}

	if len(allowlist) == 0 {
		return true
	}

	for _, network := range allowlist {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func discoveryPolicyIPsForEndpointHost(candidateURL string) []net.IP {
	if candidateURL == "" {
		return nil
	}

	host := normalizeEndpointHost(candidateURL)
	if host == "" {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}

	ips, err := lookupIPFunc(host)
	if err != nil {
		return nil
	}

	filtered := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		filtered = append(filtered, ip)
	}
	return filtered
}

func clusterEndpointAllowedByDiscoveryPolicy(endpoint config.ClusterEndpoint, candidateURL string, discoveryCfg config.DiscoveryConfig) bool {
	if len(discoveryCfg.SubnetAllowlist) == 0 && len(discoveryCfg.SubnetBlocklist) == 0 && len(discoveryCfg.IPBlocklist) == 0 {
		return true
	}

	allowlist := discoveryPolicyCIDRs(discoveryCfg.SubnetAllowlist)
	blocklist := discoveryPolicyCIDRs(discoveryCfg.SubnetBlocklist)
	blockedIPs := discoveryPolicyBlockedIPs(discoveryCfg.IPBlocklist)

	resolvedIPs := discoveryPolicyIPsForEndpointHost(candidateURL)
	if len(resolvedIPs) == 0 {
		if ip := net.ParseIP(strings.TrimSpace(endpoint.EffectiveIP())); ip != nil {
			resolvedIPs = []net.IP{ip}
		}
	}
	if len(resolvedIPs) == 0 {
		return true
	}

	for _, ip := range resolvedIPs {
		if !discoveryPolicyAllowsIP(ip, allowlist, blocklist, blockedIPs) {
			return false
		}
	}

	return true
}

func clusterEndpointRuntimeURL(endpoint config.ClusterEndpoint, verifySSL bool, hasFingerprint bool, discoveryCfg config.DiscoveryConfig) string {
	candidateURL := clusterEndpointEffectiveURL(endpoint, verifySSL, hasFingerprint)
	if candidateURL == "" {
		return ""
	}
	if !clusterEndpointAllowedByDiscoveryPolicy(endpoint, candidateURL, discoveryCfg) {
		return ""
	}
	return candidateURL
}

func monitorDiscoveryConfig(m *Monitor) config.DiscoveryConfig {
	if m == nil || m.config == nil {
		return config.DiscoveryConfig{}
	}
	return m.config.Discovery
}
