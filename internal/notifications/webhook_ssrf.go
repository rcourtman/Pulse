package notifications

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	pkgsecurityutil "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
	"github.com/rs/zerolog/log"
)

// ValidateWebhookURL validates that a webhook URL is safe and properly formed
func (n *NotificationManager) ValidateWebhookURL(webhookURL string) error {
	return n.validateWebhookURL(webhookURL, RedactWebhookURLSecrets)
}

// Validation and redirect security are shared; only diagnostic URL rendering
// differs for Apprise, whose endpoint path may itself be a configuration key.
func (n *NotificationManager) validateWebhookURL(webhookURL string, diagnosticURL func(string) string) error {
	return n.validateWebhookURLWithContext(context.Background(), webhookURL, diagnosticURL)
}

func (n *NotificationManager) validateWebhookURLWithContext(ctx context.Context, webhookURL string, diagnosticURL func(string) string) error {
	if webhookURL == "" {
		return FailfWithClass(NotificationFailureConfiguration, "webhook URL cannot be empty")
	}

	u, err := url.Parse(webhookURL)
	if err != nil {
		return FailfWithClass(NotificationFailureConfiguration, "invalid URL format: %w", err)
	}
	if u.User != nil {
		return FailfWithClass(NotificationFailureConfiguration, "webhook URL userinfo is not allowed")
	}

	// Must be HTTP or HTTPS
	if u.Scheme != "http" && u.Scheme != "https" {
		return FailfWithClass(NotificationFailureConfiguration, "webhook URL must use http or https protocol")
	}

	// Get hostname for validation
	host := u.Hostname()
	if host == "" {
		return FailfWithClass(NotificationFailureConfiguration, "webhook URL missing hostname")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return FailfWithClass(NotificationFailureConfiguration, "webhook URLs pointing to unspecified addresses are not allowed")
	}

	// Block localhost and loopback addresses (SSRF protection) unless allowlisted
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.") {
		// Check if localhost is in the allowlist
		localhostIP := net.ParseIP("127.0.0.1")
		if !n.isIPInAllowlist(localhostIP) {
			return FailfWithClass(NotificationFailureConfiguration, "webhook URLs pointing to localhost are not allowed for security reasons")
		}
		log.Debug().
			Str("host", host).
			Str("url", diagnosticURL(webhookURL)).
			Msg("localhost webhook URL allowed via allowlist")
	}

	// Block link-local addresses
	if strings.HasPrefix(host, "169.254.") || strings.HasPrefix(host, "fe80:") {
		return FailfWithClass(NotificationFailureConfiguration, "webhook URLs pointing to link-local addresses are not allowed")
	}

	// Resolve hostname to IPs and check for private ranges (DNS rebinding protection)
	ips, err := n.lookupWebhookHostIPs(ctx, host)
	if err != nil {
		// DNS resolution failed - reject for security
		return fmt.Errorf("failed to resolve webhook hostname %s: %w (DNS resolution required for security)", host, err)
	}

	// Check all resolved IPs for private ranges
	for _, ip := range ips {
		if isPrivateIP(ip) {
			// Check if this private IP is in the allowlist
			if n.isIPInAllowlist(ip) {
				log.Debug().
					Str("ip", ip.String()).
					Str("url", diagnosticURL(webhookURL)).
					Msg("webhook URL resolves to private IP in allowlist")
			} else {
				return FailfWithClass(NotificationFailureConfiguration, "webhook URL resolves to private IP %s - private networks are not allowed for security (configure allowlist in System Settings)", ip.String())
			}
		}
	}

	// Block common metadata service endpoints (cloud providers)
	metadataHosts := []string{
		"169.254.169.254", // AWS, Azure, GCP metadata
		"metadata.google.internal",
		"metadata.goog",
	}
	for _, metadataHost := range metadataHosts {
		if host == metadataHost {
			return FailfWithClass(NotificationFailureConfiguration, "webhook URLs pointing to cloud metadata services are not allowed")
		}
	}

	// Ensure hostname is not just an IP address without proper DNS
	// This helps prevent SSRF attacks using numeric IPs to bypass filters
	if u.Scheme == "https" && isNumericIP(host) {
		log.Warn().
			Str("url", diagnosticURL(webhookURL)).
			Msg("webhook URL uses numeric IP with HTTPS - certificate validation may fail")
	}

	return nil
}

// isPrivateIP reports whether ip, or any IPv4 destination an IPv6 transition
// address (NAT64, 6to4, Teredo and friends) embeds in it, is in a private or
// special-use range. Without the embedded check 64:ff9b::a9fe:a9fe passes as a
// public address while a NAT64 gateway delivers it to 169.254.169.254.
func isPrivateIP(ip net.IP) bool {
	if isPrivateIPLiteral(ip) {
		return true
	}
	for _, embedded := range pkgsecurityutil.EmbeddedIPv4Candidates(ip) {
		if isPrivateIPLiteral(embedded) {
			return true
		}
	}
	return false
}

// isPrivateIPLiteral checks the address exactly as given against the private
// and special-use ranges.
func isPrivateIPLiteral(ip net.IP) bool {
	// Private and special-use ranges that should not be reachable via webhooks.
	privateRanges := []string{
		"10.0.0.0/8",      // RFC1918
		"172.16.0.0/12",   // RFC1918
		"192.168.0.0/16",  // RFC1918
		"127.0.0.0/8",     // Loopback
		"169.254.0.0/16",  // Link-local
		"100.64.0.0/10",   // CGNAT (RFC6598)
		"198.18.0.0/15",   // Benchmarking (RFC2544)
		"0.0.0.0/8",       // "This" network (RFC1122)
		"192.0.0.0/24",    // IETF protocol assignments (RFC6890)
		"192.0.2.0/24",    // Documentation TEST-NET-1 (RFC5737)
		"198.51.100.0/24", // Documentation TEST-NET-2 (RFC5737)
		"203.0.113.0/24",  // Documentation TEST-NET-3 (RFC5737)
		"::1/128",         // IPv6 loopback
		"::/128",          // IPv6 unspecified (RFC4291)
		"fe80::/10",       // IPv6 link-local
		"fc00::/7",        // IPv6 unique local
	}

	for _, cidr := range privateRanges {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// isNumericIP checks if a string is a numeric IP address
func isNumericIP(host string) bool {
	// Simple check: if it contains only digits, dots, and colons, it's likely an IP
	for _, char := range host {
		if !(char >= '0' && char <= '9') && char != '.' && char != ':' {
			return false
		}
	}
	return len(host) > 0 && (strings.Contains(host, ".") || strings.Contains(host, ":"))
}

// UpdateAllowedPrivateCIDRs parses and updates the list of allowed private CIDR ranges for webhooks
// ParseAllowedPrivateCIDRs validates a comma-separated CIDR string and returns
// the parsed networks. It does NOT modify any runtime state — use
// ApplyAllowedPrivateCIDRs to commit the result.
func ParseAllowedPrivateCIDRs(cidrsString string) ([]*net.IPNet, error) {
	if cidrsString == "" {
		return nil, nil
	}

	cidrs := strings.Split(cidrsString, ",")
	var parsedNets []*net.IPNet

	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		// Support bare IPs by adding /32 or /128
		if !strings.Contains(cidr, "/") {
			ip := net.ParseIP(cidr)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP address: %s", cidr)
			}
			if ip.To4() != nil {
				cidr = cidr + "/32"
			} else {
				cidr = cidr + "/128"
			}
		}

		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR range %s: %w", cidr, err)
		}
		if cidrContainsUnspecifiedIP(ipNet) {
			return nil, fmt.Errorf("CIDR range %s includes an unspecified address", cidr)
		}

		parsedNets = append(parsedNets, ipNet)
	}

	return parsedNets, nil
}

func cidrContainsUnspecifiedIP(ipNet *net.IPNet) bool {
	if ipNet == nil {
		return false
	}
	return ipNet.Contains(net.IPv4zero) || ipNet.Contains(net.IPv6zero)
}

// ApplyAllowedPrivateCIDRs atomically replaces the runtime allowlist with
// pre-validated networks (from ParseAllowedPrivateCIDRs).
func (n *NotificationManager) ApplyAllowedPrivateCIDRs(cidrsString string, nets []*net.IPNet) {
	n.allowedPrivateMu.Lock()
	defer n.allowedPrivateMu.Unlock()

	n.allowedPrivateNets = nets
	if len(nets) == 0 {
		log.Info().Msg("webhook private IP allowlist cleared - all private IPs blocked")
	} else {
		log.Info().
			Str("cidrs", cidrsString).
			Int("count", len(nets)).
			Msg("webhook private IP allowlist updated")
	}
}

// UpdateAllowedPrivateCIDRs validates and applies CIDR changes in one call.
// Kept for backward compatibility; prefer ParseAllowedPrivateCIDRs + ApplyAllowedPrivateCIDRs
// when you need persist-before-mutate semantics.
func (n *NotificationManager) UpdateAllowedPrivateCIDRs(cidrsString string) error {
	nets, err := ParseAllowedPrivateCIDRs(cidrsString)
	if err != nil {
		return err
	}
	n.ApplyAllowedPrivateCIDRs(cidrsString, nets)
	return nil
}

// lookupWebhookHostIPs bounds DNS work for both admission and pinned dialing.
func (n *NotificationManager) lookupWebhookHostIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	resolveCtx, cancel := context.WithTimeout(ctx, webhookDNSLookupTimeout)
	defer cancel()
	var ips []net.IP
	var err error
	if n.resolveWebhookIPs != nil {
		ips, err = n.resolveWebhookIPs(resolveCtx, host)
	} else {
		ips, err = net.DefaultResolver.LookupIP(resolveCtx, "ip", host)
	}
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no addresses", Name: host, IsNotFound: true}
	}
	return ips, nil
}

// isIPInAllowlist checks if an IP is in the configured allowlist
func (n *NotificationManager) isIPInAllowlist(ip net.IP) bool {
	n.allowedPrivateMu.RLock()
	defer n.allowedPrivateMu.RUnlock()

	// No allowlist means block all private IPs
	if len(n.allowedPrivateNets) == 0 {
		return false
	}

	// Check if IP is in any allowed range
	for _, ipNet := range n.allowedPrivateNets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}
