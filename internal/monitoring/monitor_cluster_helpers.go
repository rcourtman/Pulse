package monitoring

import (
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

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

// discoveryPolicyDecisionTTL bounds how long a cached discovery-policy verdict
// (and the DNS answer behind it) is reused before re-evaluating. It matches the
// tlsutil DNS cache refresh interval so policy decisions never trail the
// resolver view used for actual connections by more than one refresh.
const discoveryPolicyDecisionTTL = 5 * time.Minute

// discoveryPolicyDecisionCacheLimit caps the decision cache. Keys derive from
// configured endpoints and the discovery policy, so the map stays tiny in
// practice; the cap only guards against pathological configs.
const discoveryPolicyDecisionCacheLimit = 1024

type discoveryPolicyDecision struct {
	allowed   bool
	expiresAt time.Time
}

var (
	discoveryPolicyDecisionMu    sync.Mutex
	discoveryPolicyDecisionCache = map[string]discoveryPolicyDecision{}
	discoveryPolicyTimeNow       = time.Now
)

func resetDiscoveryPolicyDecisionCache() {
	discoveryPolicyDecisionMu.Lock()
	discoveryPolicyDecisionCache = map[string]discoveryPolicyDecision{}
	discoveryPolicyDecisionMu.Unlock()
}

// discoveryPolicyIsDefaultOnly reports whether the effective policy consists
// solely of the injected default link-local blocklist. NormalizeDiscoveryConfig
// always adds 169.254.0.0/16, so every install has a non-empty policy and the
// zero-policy fast path above never fires (#1638). Link-local addresses are
// not routable across segments, so a hostname is never legitimately served by
// one; only literal link-local IPs need blocking, which requires no DNS.
func discoveryPolicyIsDefaultOnly(cfg config.DiscoveryConfig) bool {
	if len(cfg.SubnetAllowlist) != 0 || len(cfg.IPBlocklist) != 0 {
		return false
	}
	defaults := config.DefaultDiscoveryConfig().SubnetBlocklist
	defaultSet := make(map[string]struct{}, len(defaults))
	for _, cidr := range defaults {
		defaultSet[strings.TrimSpace(cidr)] = struct{}{}
	}
	for _, cidr := range cfg.SubnetBlocklist {
		if _, ok := defaultSet[strings.TrimSpace(cidr)]; !ok {
			return false
		}
	}
	return true
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

func discoveryPolicyDecisionKey(endpoint config.ClusterEndpoint, candidateURL string, cfg config.DiscoveryConfig) string {
	parts := []string{
		candidateURL,
		strings.TrimSpace(endpoint.EffectiveIP()),
		strings.Join(cfg.SubnetAllowlist, ","),
		strings.Join(cfg.SubnetBlocklist, ","),
		strings.Join(cfg.IPBlocklist, ","),
	}
	return strings.Join(parts, "|")
}

// evaluateClusterEndpointDiscoveryPolicy is the uncached policy check,
// including DNS resolution of hostname endpoints.
func evaluateClusterEndpointDiscoveryPolicy(endpoint config.ClusterEndpoint, candidateURL string, discoveryCfg config.DiscoveryConfig) bool {
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

func clusterEndpointAllowedByDiscoveryPolicy(endpoint config.ClusterEndpoint, candidateURL string, discoveryCfg config.DiscoveryConfig) bool {
	if len(discoveryCfg.SubnetAllowlist) == 0 && len(discoveryCfg.SubnetBlocklist) == 0 && len(discoveryCfg.IPBlocklist) == 0 {
		return true
	}

	// The policy is a function of configuration, not poll state, but since
	// c5f5af7ab it is re-evaluated per node per poll cycle. With the default
	// link-local-only blocklist no resolution is needed at all, and custom
	// policies memoize their verdict so repeat polls stay off the resolver
	// (#1638).
	if discoveryPolicyIsDefaultOnly(discoveryCfg) {
		blocklist := discoveryPolicyCIDRs(discoveryCfg.SubnetBlocklist)
		for _, ip := range discoveryPolicyLiteralIPs(endpoint, candidateURL) {
			if !discoveryPolicyAllowsIP(ip, nil, blocklist, nil) {
				return false
			}
		}
		return true
	}

	key := discoveryPolicyDecisionKey(endpoint, candidateURL, discoveryCfg)
	now := discoveryPolicyTimeNow()

	discoveryPolicyDecisionMu.Lock()
	if cached, ok := discoveryPolicyDecisionCache[key]; ok && now.Before(cached.expiresAt) {
		discoveryPolicyDecisionMu.Unlock()
		return cached.allowed
	}
	discoveryPolicyDecisionMu.Unlock()

	allowed := evaluateClusterEndpointDiscoveryPolicy(endpoint, candidateURL, discoveryCfg)

	discoveryPolicyDecisionMu.Lock()
	if len(discoveryPolicyDecisionCache) >= discoveryPolicyDecisionCacheLimit {
		discoveryPolicyDecisionCache = map[string]discoveryPolicyDecision{}
	}
	discoveryPolicyDecisionCache[key] = discoveryPolicyDecision{
		allowed:   allowed,
		expiresAt: now.Add(discoveryPolicyDecisionTTL),
	}
	discoveryPolicyDecisionMu.Unlock()

	return allowed
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
