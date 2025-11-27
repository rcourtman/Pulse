package config

import (
	"net"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const (
	defaultPVEPort = "8006"
	defaultPBSPort = "8007"
)

const (
	// DefaultPVEPort is the standard API port for Proxmox VE.
	DefaultPVEPort = defaultPVEPort
	// DefaultPBSPort is the standard API port for Proxmox Backup Server.
	DefaultPBSPort = defaultPBSPort
	// DefaultPMGPort reuses the PVE API port.
	DefaultPMGPort = defaultPVEPort
)

// normalizeHostPort ensures we always have a scheme and explicit port when talking to
// Proxmox APIs. It preserves existing ports/schemes and strips any path/query segments.
func normalizeHostPort(host, defaultPort string) string {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" || defaultPort == "" {
		return trimmed
	}

	candidate := trimmed
	if !strings.HasPrefix(candidate, "http://") && !strings.HasPrefix(candidate, "https://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return trimmed
	}

	// Drop any path fragments so we only persist host:port
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	if parsed.Port() == "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), defaultPort)
	}

	return parsed.Scheme + "://" + parsed.Host
}

// CreateProxmoxConfig creates a proxmox.ClientConfig from a PVEInstance
func CreateProxmoxConfig(node *PVEInstance) proxmox.ClientConfig {
	return createProxmoxConfigWithHost(node, node.Host, true)
}

// CreateProxmoxConfigWithHost builds a proxmox.ClientConfig using an explicit host.
// When normalizeHost is true, the host will be normalized (scheme + default port);
// when false, the host is used verbatim (useful for fallback attempts).
func CreateProxmoxConfigWithHost(node *PVEInstance, host string, normalizeHost bool) proxmox.ClientConfig {
	return createProxmoxConfigWithHost(node, host, normalizeHost)
}

func createProxmoxConfigWithHost(node *PVEInstance, host string, normalizeHost bool) proxmox.ClientConfig {
	user := node.User
	if node.TokenName == "" && node.TokenValue == "" && user != "" && !strings.Contains(user, "@") {
		user = user + "@pam"
	}

	if normalizeHost {
		host = normalizeHostPort(host, defaultPVEPort)
	}

	return proxmox.ClientConfig{
		Host:        host,
		User:        user,
		Password:    node.Password,
		TokenName:   node.TokenName,
		TokenValue:  node.TokenValue,
		VerifySSL:   node.VerifySSL,
		Fingerprint: node.Fingerprint,
	}
}

// CreatePBSConfig creates a pbs.ClientConfig from a PBSInstance
func CreatePBSConfig(node *PBSInstance) pbs.ClientConfig {
	return pbs.ClientConfig{
		Host:        normalizeHostPort(node.Host, defaultPBSPort),
		User:        node.User,
		Password:    node.Password,
		TokenName:   node.TokenName,
		TokenValue:  node.TokenValue,
		VerifySSL:   node.VerifySSL,
		Fingerprint: node.Fingerprint,
	}
}

// CreatePMGConfig creates a pmg.ClientConfig from a PMGInstance
func CreatePMGConfig(node *PMGInstance) pmg.ClientConfig {
	return pmg.ClientConfig{
		Host:        normalizeHostPort(node.Host, defaultPVEPort),
		User:        node.User,
		Password:    node.Password,
		TokenName:   node.TokenName,
		TokenValue:  node.TokenValue,
		VerifySSL:   node.VerifySSL,
		Fingerprint: node.Fingerprint,
	}
}

// CreateProxmoxConfigFromFields creates a proxmox.ClientConfig from individual fields
func CreateProxmoxConfigFromFields(host, user, password, tokenName, tokenValue, fingerprint string, verifySSL bool) proxmox.ClientConfig {
	if tokenName == "" && tokenValue == "" && user != "" && !strings.Contains(user, "@") {
		user = user + "@pam"
	}

	return proxmox.ClientConfig{
		Host:        normalizeHostPort(host, defaultPVEPort),
		User:        user,
		Password:    password,
		TokenName:   tokenName,
		TokenValue:  tokenValue,
		VerifySSL:   verifySSL,
		Fingerprint: fingerprint,
	}
}

// CreatePMGConfigFromFields creates a pmg.ClientConfig from individual fields
func CreatePMGConfigFromFields(host, user, password, tokenName, tokenValue, fingerprint string, verifySSL bool) pmg.ClientConfig {
	return pmg.ClientConfig{
		Host:        normalizeHostPort(host, defaultPVEPort),
		User:        user,
		Password:    password,
		TokenName:   tokenName,
		TokenValue:  tokenValue,
		VerifySSL:   verifySSL,
		Fingerprint: fingerprint,
	}
}

// StripDefaultPort removes the supplied defaultPort from a host URL if it matches.
// Useful for retrying portless endpoints (e.g., reverse proxies on 443) when the
// normalized :8006/:8007 host is unreachable.
func StripDefaultPort(host, defaultPort string) string {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" || defaultPort == "" {
		return trimmed
	}

	candidate := trimmed
	if !strings.HasPrefix(candidate, "http://") && !strings.HasPrefix(candidate, "https://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return trimmed
	}

	if parsed.Port() != defaultPort {
		return trimmed
	}

	parsed.Host = parsed.Hostname()
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.Scheme + "://" + parsed.Host + strings.TrimSuffix(parsed.Path, "/")
}
