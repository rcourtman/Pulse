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
// watchdogs, but same-host addresses are rejected because they cannot detect
// loss of the Pulse host. The runtime rejects redirects so the secret URL path
// cannot be forwarded to another origin.
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
	if ip := net.ParseIP(hostname); ip != nil && (ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return fmt.Errorf("dead-man monitoring must use a different host from Pulse")
	}

	path := strings.TrimSuffix(strings.ToLower(parsed.EscapedPath()), "/")
	for _, suffix := range []string{"/start", "/fail", "/log"} {
		if strings.HasSuffix(path, suffix) {
			return fmt.Errorf("dead-man ping URL must be the base success URL, without %s", suffix)
		}
	}
	return nil
}
