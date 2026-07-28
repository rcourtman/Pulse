package monitoring

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

// discoveryPolicyLookupTimeout bounds a policy resolution so a poll cycle can
// never stall on an unresponsive resolver.
const discoveryPolicyLookupTimeout = 5 * time.Second

// lookupIPFunc resolves an endpoint hostname for the discovery-policy check.
//
// It goes through the process-global cached resolver that pkg/tlsutil dials
// with, rather than a bare net.LookupIP, for two reasons: the policy verdict is
// then made against the same addresses the connection will actually reach, and
// repeat poll cycles cost a cache hit instead of a DNS query. The resolver
// caches failures as well as answers, so an endpoint whose name does not
// resolve costs one query per cache refresh too (#1638). Tests replace it.
var lookupIPFunc = func(host string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(context.Background(), discoveryPolicyLookupTimeout)
	defer cancel()
	return tlsutil.LookupHostCached(ctx, host)
}

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

func clusterEndpointEffectiveURL(endpoint config.ClusterEndpoint, verifySSL bool, _ bool) string {
	// A fingerprint only applies to the specific endpoint it was captured from.
	// The primary node's (cluster-level) fingerprint must not be treated as valid
	// for every cluster member, otherwise we route to per-node IPs while pinning
	// the wrong certificate. So derive this strictly from the endpoint's own
	// fingerprint and ignore any cluster-level value the caller passed. Refs: #1199
	// (and #620 for why a fingerprint lets us prefer IP to reduce DNS lookups).
	hasEndpointFingerprint := strings.TrimSpace(endpoint.Fingerprint) != ""
	requiresHostnameForTLS := verifySSL && !hasEndpointFingerprint

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

// discoveryPolicyLiteralIPs returns the IPs knowable for an endpoint without
// touching the resolver: a literal IP in the candidate URL, else the
// endpoint's recorded effective IP.
func discoveryPolicyLiteralIPs(endpoint config.ClusterEndpoint, candidateURL string) []net.IP {
	if host := normalizeEndpointHost(candidateURL); host != "" {
		if ip := net.ParseIP(host); ip != nil {
			return []net.IP{ip}
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(endpoint.EffectiveIP())); ip != nil {
		return []net.IP{ip}
	}
	return nil
}

// clusterEndpointAllowedByDiscoveryPolicy evaluates the configured discovery
// policy against every address the endpoint would be dialled at.
//
// There is deliberately no memoized verdict here. Resolution goes through the
// shared cached resolver (see lookupIPFunc), which already collapses repeat
// poll cycles to a cache hit, so a second decision cache would buy nothing
// while making the policy trail the configuration and, worse, freezing a
// fail-open "resolution failed, allow" verdict in place for minutes at a time.
// Every policy — including the default link-local blocklist that
// NormalizeDiscoveryConfig injects — is therefore enforced against resolved
// addresses, not just literal ones (#1638).
func clusterEndpointAllowedByDiscoveryPolicy(endpoint config.ClusterEndpoint, candidateURL string, discoveryCfg config.DiscoveryConfig) bool {
	if len(discoveryCfg.SubnetAllowlist) == 0 && len(discoveryCfg.SubnetBlocklist) == 0 && len(discoveryCfg.IPBlocklist) == 0 {
		return true
	}

	allowlist := discoveryPolicyCIDRs(discoveryCfg.SubnetAllowlist)
	blocklist := discoveryPolicyCIDRs(discoveryCfg.SubnetBlocklist)
	blockedIPs := discoveryPolicyBlockedIPs(discoveryCfg.IPBlocklist)

	resolvedIPs := discoveryPolicyIPsForEndpointHost(candidateURL)
	if len(resolvedIPs) == 0 {
		resolvedIPs = discoveryPolicyLiteralIPs(endpoint, candidateURL)
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
