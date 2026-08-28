package notifications

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const MaxDeadManPingURLLength = 2048

// DeadManConfig is encrypted destination configuration. PingURL is
// credential-like because possession lets a caller forge healthy signals.
type DeadManConfig struct {
	PingURL string `json:"pingUrl,omitempty"`
}

func NormalizeDeadManConfig(config DeadManConfig) DeadManConfig {
	config.PingURL = strings.TrimSpace(config.PingURL)
	return config
}

// ValidateDeadManPingURL validates a healthchecks-compatible base ping URL.
// Private-network destinations remain supported for separately hosted LAN
// watchdogs, but literal same-host addresses are rejected because they cannot
// detect loss of the Pulse host. The runtime repeats the check after DNS
// resolution and rejects redirects so the secret URL path cannot be forwarded
// to another origin.
func ValidateDeadManPingURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > MaxDeadManPingURLLength {
		return fmt.Errorf("dead-man ping URL exceeds %d characters", MaxDeadManPingURLLength)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("dead-man ping URL is invalid")
	}
	if !parsed.IsAbs() || parsed.Opaque != "" {
		return fmt.Errorf("dead-man ping URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("dead-man ping URL must use http or https")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("dead-man ping URL must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("dead-man ping URL must not contain user credentials")
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("dead-man ping URL must not contain a fragment")
	}

	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return fmt.Errorf("dead-man monitoring must use a different host from Pulse")
	}
	if ip := net.ParseIP(hostname); ip != nil {
		sameHost, err := IsDeadManSameHostIP(ip)
		if err != nil {
			return fmt.Errorf("could not verify dead-man host separation: %w", err)
		}
		if sameHost {
			return fmt.Errorf("dead-man monitoring must use a different host from Pulse")
		}
	}

	path := strings.TrimSuffix(strings.ToLower(parsed.EscapedPath()), "/")
	for _, suffix := range []string{"/start", "/fail", "/log"} {
		if strings.HasSuffix(path, suffix) {
			return fmt.Errorf("dead-man ping URL must be the base success URL, without %s", suffix)
		}
	}
	return nil
}

// IsDeadManSameHostIP reports whether an endpoint address belongs to the
// Pulse machine. Dead-man delivery calls this after DNS resolution as well as
// during literal-IP configuration validation, so aliases and LAN addresses
// cannot silently point the watchdog back at Pulse itself.
func IsDeadManSameHostIP(ip net.IP) (bool, error) {
	if ip == nil {
		return true, nil
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true, nil
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return false, fmt.Errorf("enumerate Pulse host addresses: %w", err)
	}
	return isDeadManSameHostIP(ip, addresses), nil
}

func isDeadManSameHostIP(ip net.IP, addresses []net.Addr) bool {
	for _, address := range addresses {
		var localIP net.IP
		switch value := address.(type) {
		case *net.IPNet:
			localIP = value.IP
		case *net.IPAddr:
			localIP = value.IP
		default:
			host, _, err := net.ParseCIDR(address.String())
			if err == nil {
				localIP = host
			}
		}
		if localIP != nil && ip.Equal(localIP) {
			return true
		}
	}
	return false
}
