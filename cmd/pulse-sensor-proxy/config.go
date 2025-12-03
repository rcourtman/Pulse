package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	PerPeerIntervalMs int `yaml:"per_peer_interval_ms"` // Milliseconds between requests per peer
	PerPeerBurst      int `yaml:"per_peer_burst"`       // Number of requests allowed in a burst
}

// Config holds proxy configuration
type Config struct {
	AllowedSourceSubnets   []string      `yaml:"allowed_source_subnets"`
	MetricsAddress         string        `yaml:"metrics_address"`
	LogLevel               string        `yaml:"log_level"`
	AllowedNodes           []string      `yaml:"allowed_nodes"`
	AllowedNodesFile       string        `yaml:"allowed_nodes_file"`
	StrictNodeValidation   bool          `yaml:"strict_node_validation"`
	ReadTimeout            time.Duration `yaml:"read_timeout"`
	WriteTimeout           time.Duration `yaml:"write_timeout"`
	MaxSSHOutputBytes      int64         `yaml:"max_ssh_output_bytes"`
	RequireProxmoxHostkeys bool          `yaml:"require_proxmox_hostkeys"`
	AllowedPeers           []PeerConfig  `yaml:"allowed_peers"`

	AllowIDMappedRoot bool     `yaml:"allow_idmapped_root"`
	AllowedPeerUIDs   []uint32 `yaml:"allowed_peer_uids"`
	AllowedPeerGIDs   []uint32 `yaml:"allowed_peer_gids"`
	AllowedIDMapUsers []string `yaml:"allowed_idmap_users"`

	RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty"`

	// HTTP mode configuration
	HTTPEnabled     bool   `yaml:"http_enabled"`     // Enable HTTP server mode
	HTTPListenAddr  string `yaml:"http_listen_addr"` // Address to listen on (e.g., ":8443")
	HTTPTLSCertFile string `yaml:"http_tls_cert"`    // Path to TLS certificate
	HTTPTLSKeyFile  string `yaml:"http_tls_key"`     // Path to TLS private key
	HTTPAuthToken   string `yaml:"http_auth_token"`  // Bearer token for authentication

	PulseControlPlane *ControlPlaneConfig `yaml:"pulse_control_plane"`
}

type ControlPlaneConfig struct {
	URL                string `yaml:"url"`
	TokenFile          string `yaml:"token_file"`
	RefreshIntervalSec int    `yaml:"refresh_interval"` // seconds
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

const defaultAllowedNodesFile = "/etc/pulse-sensor-proxy/allowed_nodes.yaml"

// PeerConfig represents a peer entry with capabilities.
type PeerConfig struct {
	UID          uint32   `yaml:"uid"`
	Capabilities []string `yaml:"capabilities"`
}

// loadConfig loads configuration from file and environment variables
func loadConfig(configPath string) (*Config, error) {
	cfg := &Config{
		AllowIDMappedRoot: true,
		AllowedIDMapUsers: []string{"root"},
		LogLevel:          "info", // Default log level
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxSSHOutputBytes: 1 * 1024 * 1024, // 1 MiB
	}

	// Try to load config file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			if sanitized, newData := sanitizeDuplicateAllowedNodesBlocks(configPath, data); sanitized {
				data = newData
			}

			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}

			log.Info().
				Str("config_file", configPath).
				Int("subnet_count", len(cfg.AllowedSourceSubnets)).
				Msg("Loaded configuration from file")
		}
	}

	// Read timeout override
	if envReadTimeout := os.Getenv("PULSE_SENSOR_PROXY_READ_TIMEOUT"); envReadTimeout != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(envReadTimeout)); err != nil {
			log.Warn().Str("value", envReadTimeout).Err(err).Msg("Invalid PULSE_SENSOR_PROXY_READ_TIMEOUT value, ignoring")
		} else {
			cfg.ReadTimeout = parsed
			log.Info().Dur("read_timeout", cfg.ReadTimeout).Msg("Configured read timeout from environment")
		}
	}

	// Write timeout override
	if envWriteTimeout := os.Getenv("PULSE_SENSOR_PROXY_WRITE_TIMEOUT"); envWriteTimeout != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(envWriteTimeout)); err != nil {
			log.Warn().Str("value", envWriteTimeout).Err(err).Msg("Invalid PULSE_SENSOR_PROXY_WRITE_TIMEOUT value, ignoring")
		} else {
			cfg.WriteTimeout = parsed
			log.Info().Dur("write_timeout", cfg.WriteTimeout).Msg("Configured write timeout from environment")
		}
	}

	// Append from environment variable if set
	if envSubnets := os.Getenv("PULSE_SENSOR_PROXY_ALLOWED_SUBNETS"); envSubnets != "" {
		envList := strings.Split(envSubnets, ",")
		cfg.AllowedSourceSubnets = append(cfg.AllowedSourceSubnets, envList...)
		log.Info().
			Int("env_subnet_count", len(envList)).
			Msg("Appended subnets from environment variable")
	}

	// Ensure timeouts have sane defaults
	if cfg.ReadTimeout <= 0 {
		log.Warn().Dur("configured_value", cfg.ReadTimeout).Msg("Read timeout must be positive; using default 5s")
		cfg.ReadTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout <= 0 {
		log.Warn().Dur("configured_value", cfg.WriteTimeout).Msg("Write timeout must be positive; using default 10s")
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.MaxSSHOutputBytes <= 0 {
		log.Warn().Int64("configured_value", cfg.MaxSSHOutputBytes).Msg("max_ssh_output_bytes must be positive; using default 1MiB")
		cfg.MaxSSHOutputBytes = 1 * 1024 * 1024
	}

	// Allow ID-mapped root override
	if envAllowIDMap := os.Getenv("PULSE_SENSOR_PROXY_ALLOW_IDMAPPED_ROOT"); envAllowIDMap != "" {
		parsed, err := parseBool(envAllowIDMap)
		if err != nil {
			log.Warn().
				Str("value", envAllowIDMap).
				Err(err).
				Msg("Invalid PULSE_SENSOR_PROXY_ALLOW_IDMAPPED_ROOT value, ignoring")
		} else {
			cfg.AllowIDMappedRoot = parsed
			log.Info().
				Bool("allow_idmapped_root", parsed).
				Msg("Configured allow_idmapped_root from environment variable")
		}
	}

	// Allowed ID map users override
	if envIDMapUsers := os.Getenv("PULSE_SENSOR_PROXY_ALLOWED_IDMAP_USERS"); envIDMapUsers != "" {
		envList := splitAndTrim(envIDMapUsers)
		if len(envList) > 0 {
			cfg.AllowedIDMapUsers = envList
			log.Info().
				Strs("allowed_idmap_users", cfg.AllowedIDMapUsers).
				Msg("Configured allowed ID map users from environment")
		}
	}

	// Allowed peer UID overrides
	if envUIDs := os.Getenv("PULSE_SENSOR_PROXY_ALLOWED_PEER_UIDS"); envUIDs != "" {
		parsed, err := parseUint32List(envUIDs)
		if err != nil {
			log.Warn().
				Str("value", envUIDs).
				Err(err).
				Msg("Invalid PULSE_SENSOR_PROXY_ALLOWED_PEER_UIDS value, ignoring")
		} else {
			cfg.AllowedPeerUIDs = append(cfg.AllowedPeerUIDs, parsed...)
			log.Info().
				Int("env_uid_count", len(parsed)).
				Msg("Appended allowed peer UIDs from environment")
		}
	}

	// Allowed peer GID overrides
	if envGIDs := os.Getenv("PULSE_SENSOR_PROXY_ALLOWED_PEER_GIDS"); envGIDs != "" {
		parsed, err := parseUint32List(envGIDs)
		if err != nil {
			log.Warn().
				Str("value", envGIDs).
				Err(err).
				Msg("Invalid PULSE_SENSOR_PROXY_ALLOWED_PEER_GIDS value, ignoring")
		} else {
			cfg.AllowedPeerGIDs = append(cfg.AllowedPeerGIDs, parsed...)
			log.Info().
				Int("env_gid_count", len(parsed)).
				Msg("Appended allowed peer GIDs from environment")
		}
	}

	// Allowed node overrides
	if envNodes := os.Getenv("PULSE_SENSOR_PROXY_ALLOWED_NODES"); envNodes != "" {
		envList := splitAndTrim(envNodes)
		if len(envList) > 0 {
			cfg.AllowedNodes = append(cfg.AllowedNodes, envList...)
			log.Info().
				Int("env_allowed_nodes", len(envList)).
				Msg("Appended allowed nodes from environment")
		}
	}

	if cfg.AllowedNodesFile == "" {
		if _, err := os.Stat(defaultAllowedNodesFile); err == nil {
			cfg.AllowedNodesFile = defaultAllowedNodesFile
		}
	}

	if cfg.AllowedNodesFile != "" {
		if fileNodes, err := loadAllowedNodesFile(cfg.AllowedNodesFile); err != nil {
			log.Warn().
				Err(err).
				Str("allowed_nodes_file", cfg.AllowedNodesFile).
				Msg("Failed to load allowed nodes file")
		} else if len(fileNodes) > 0 {
			cfg.AllowedNodes = append(cfg.AllowedNodes, fileNodes...)
			log.Info().
				Str("allowed_nodes_file", cfg.AllowedNodesFile).
				Int("allowed_node_count", len(fileNodes)).
				Msg("Loaded allowed nodes from file")
		}
	}

	cfg.AllowedNodes = normalizeNodes(cfg.AllowedNodes)

	// Strict node validation override
	if envStrict := os.Getenv("PULSE_SENSOR_PROXY_STRICT_NODE_VALIDATION"); envStrict != "" {
		parsed, err := parseBool(envStrict)
		if err != nil {
			log.Warn().
				Str("value", envStrict).
				Err(err).
				Msg("Invalid PULSE_SENSOR_PROXY_STRICT_NODE_VALIDATION value, ignoring")
		} else {
			cfg.StrictNodeValidation = parsed
			log.Info().
				Bool("strict_node_validation", parsed).
				Msg("Configured strict node validation from environment")
		}
	}

	// SSH output limit override
	if envMaxSSH := os.Getenv("PULSE_SENSOR_PROXY_MAX_SSH_OUTPUT_BYTES"); envMaxSSH != "" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(envMaxSSH), 10, 64); err != nil {
			log.Warn().Str("value", envMaxSSH).Err(err).Msg("Invalid PULSE_SENSOR_PROXY_MAX_SSH_OUTPUT_BYTES value, ignoring")
		} else {
			cfg.MaxSSHOutputBytes = parsed
			log.Info().Int64("max_ssh_output_bytes", cfg.MaxSSHOutputBytes).Msg("Configured max SSH output bytes from environment")
		}
	}

	// Require Proxmox host keys override
	if envReq := os.Getenv("PULSE_SENSOR_PROXY_REQUIRE_PROXMOX_HOSTKEYS"); envReq != "" {
		if parsed, err := parseBool(envReq); err != nil {
			log.Warn().Str("value", envReq).Err(err).Msg("Invalid PULSE_SENSOR_PROXY_REQUIRE_PROXMOX_HOSTKEYS value, ignoring")
		} else {
			cfg.RequireProxmoxHostkeys = parsed
			log.Info().Bool("require_proxmox_hostkeys", parsed).Msg("Configured Proxmox host key requirement from environment")
		}
	}

	// Metrics address from environment variable
	if envMetrics := os.Getenv("PULSE_SENSOR_PROXY_METRICS_ADDR"); envMetrics != "" {
		cfg.MetricsAddress = envMetrics
		log.Info().Str("metrics_addr", envMetrics).Msg("Metrics address set from environment")
	}

	// Default metrics address if not configured
	if cfg.MetricsAddress == "" {
		cfg.MetricsAddress = "default" // Will use defaultMetricsAddr
	}

	// Parse and validate all subnets
	if len(cfg.AllowedSourceSubnets) > 0 {
		normalized, err := parseAllowedSubnets(cfg.AllowedSourceSubnets)
		if err != nil {
			return nil, fmt.Errorf("invalid subnet configuration: %w", err)
		}
		cfg.AllowedSourceSubnets = normalized
		log.Info().
			Strs("allowed_subnets", cfg.AllowedSourceSubnets).
			Msg("Validated and normalized subnet configuration")
	} else {
		// Auto-detect if no configuration provided
		detected := detectHostCIDRs()
		if len(detected) == 0 {
			log.Warn().Msg("No allowed_source_subnets configured and no host addresses detected")
		} else {
			cfg.AllowedSourceSubnets = detected
			log.Warn().
				Strs("auto_detected_subnets", detected).
				Msg("No allowed_source_subnets configured; using detected host addresses (recommended to configure explicitly)")
		}
	}

	// Log rate limit configuration if provided
	if cfg.RateLimit != nil {
		log.Info().
			Int("per_peer_interval_ms", cfg.RateLimit.PerPeerIntervalMs).
			Int("per_peer_burst", cfg.RateLimit.PerPeerBurst).
			Msg("Rate limit configuration loaded from config file")
	}

	// Log level from environment variable
	if envLogLevel := os.Getenv("PULSE_SENSOR_PROXY_LOG_LEVEL"); envLogLevel != "" {
		cfg.LogLevel = strings.ToLower(strings.TrimSpace(envLogLevel))
		log.Info().Str("log_level", cfg.LogLevel).Msg("Log level set from environment")
	}

	// HTTP mode configuration from environment variables
	if envHTTPEnabled := os.Getenv("PULSE_SENSOR_PROXY_HTTP_ENABLED"); envHTTPEnabled != "" {
		if parsed, err := parseBool(envHTTPEnabled); err != nil {
			log.Warn().Str("value", envHTTPEnabled).Err(err).Msg("Invalid PULSE_SENSOR_PROXY_HTTP_ENABLED value, ignoring")
		} else {
			cfg.HTTPEnabled = parsed
			log.Info().Bool("http_enabled", parsed).Msg("HTTP mode enabled from environment")
		}
	}

	if envHTTPAddr := os.Getenv("PULSE_SENSOR_PROXY_HTTP_ADDR"); envHTTPAddr != "" {
		cfg.HTTPListenAddr = strings.TrimSpace(envHTTPAddr)
		log.Info().Str("http_addr", cfg.HTTPListenAddr).Msg("HTTP listen address set from environment")
	}

	if envHTTPCert := os.Getenv("PULSE_SENSOR_PROXY_HTTP_TLS_CERT"); envHTTPCert != "" {
		cfg.HTTPTLSCertFile = strings.TrimSpace(envHTTPCert)
		log.Info().Str("tls_cert", cfg.HTTPTLSCertFile).Msg("HTTP TLS cert path set from environment")
	}

	if envHTTPKey := os.Getenv("PULSE_SENSOR_PROXY_HTTP_TLS_KEY"); envHTTPKey != "" {
		cfg.HTTPTLSKeyFile = strings.TrimSpace(envHTTPKey)
		log.Info().Str("tls_key", cfg.HTTPTLSKeyFile).Msg("HTTP TLS key path set from environment")
	}

	if envHTTPToken := os.Getenv("PULSE_SENSOR_PROXY_HTTP_AUTH_TOKEN"); envHTTPToken != "" {
		cfg.HTTPAuthToken = strings.TrimSpace(envHTTPToken)
		log.Info().Msg("HTTP auth token set from environment")
	}

	// Validate HTTP configuration if enabled
	if cfg.HTTPEnabled {
		if cfg.HTTPListenAddr == "" {
			// Use 0.0.0.0:8443 explicitly for IPv4 binding.
			// Using just ":8443" can result in IPv6-only binding on systems
			// with net.ipv6.bindv6only=1 (e.g., some Proxmox 8 configurations).
			cfg.HTTPListenAddr = "0.0.0.0:8443"
			log.Info().Str("http_addr", cfg.HTTPListenAddr).Msg("Using default HTTP listen address")
		}
		if cfg.HTTPAuthToken == "" {
			return nil, fmt.Errorf("http_enabled=true requires http_auth_token to be configured")
		}
		if cfg.HTTPTLSCertFile == "" || cfg.HTTPTLSKeyFile == "" {
			return nil, fmt.Errorf("http_enabled=true requires both http_tls_cert and http_tls_key")
		}
	}

	if cfg.PulseControlPlane != nil {
		if cfg.PulseControlPlane.TokenFile == "" {
			cfg.PulseControlPlane.TokenFile = defaultControlPlaneTokenPath
		}
		if cfg.PulseControlPlane.RefreshIntervalSec <= 0 {
			cfg.PulseControlPlane.RefreshIntervalSec = defaultControlPlaneRefreshSecs
		}
	}

	return cfg, nil
}

// parseBool returns boolean value for various truthy/falsy strings
func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", raw)
	}
}

// parseUint32List parses comma-separated uint32 list
func parseUint32List(raw string) ([]uint32, error) {
	var parsed []uint32
	parts := splitAndTrim(raw)
	for _, part := range parts {
		if part == "" {
			continue
		}
		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid uint32 %q: %w", part, err)
		}
		parsed = append(parsed, uint32(val))
	}
	return parsed, nil
}

// splitAndTrim splits a comma-separated string and trims whitespace
func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func normalizeNodes(nodes []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, node := range nodes {
		trimmed := strings.TrimSpace(node)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func loadAllowedNodesFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	type wrapper struct {
		AllowedNodes []string `yaml:"allowed_nodes"`
	}

	var w wrapper
	if err := yaml.Unmarshal(data, &w); err == nil && len(w.AllowedNodes) > 0 {
		return normalizeNodes(w.AllowedNodes), nil
	}

	var list []string
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return normalizeNodes(list), nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var plain []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		}
		if line != "" {
			plain = append(plain, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return normalizeNodes(plain), err
	}
	return normalizeNodes(plain), nil
}

// detectHostCIDRs detects local host IP addresses as /32 (IPv4) or /128 (IPv6) CIDRs
func detectHostCIDRs() []string {
	var cidrs []string

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to enumerate network interfaces")
		return cidrs
	}

	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			log.Warn().Str("iface", iface.Name).Err(err).Msg("Address lookup failed")
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			// Skip loopback and link-local addresses
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			// Add as /32 for IPv4, /128 for IPv6
			if ip.To4() != nil {
				cidrs = append(cidrs, ip.String()+"/32")
			} else if ip.To16() != nil {
				cidrs = append(cidrs, ip.String()+"/128")
			}
		}
	}

	return cidrs
}

var allowedNodesBlockPattern = regexp.MustCompile(`(?m)(?:^[ \t]*#.*\n)*^[ \t]*allowed_nodes:\n(?:^[ \t]+-.*\n?)+`)

func sanitizeDuplicateAllowedNodesBlocks(path string, data []byte) (bool, []byte) {
	matches := allowedNodesBlockPattern.FindAllIndex(data, -1)
	if len(matches) <= 1 {
		return false, data
	}

	var buf bytes.Buffer
	last := 0
	for idx, match := range matches {
		start, end := match[0], match[1]
		if idx == 0 {
			buf.Write(data[last:end])
		} else {
			buf.Write(data[last:start])
		}
		last = end
	}
	buf.Write(data[last:])

	cleaned := buf.Bytes()

	// Phase 2: DO NOT write the file back - that would create an uncoordinated writer
	// The sanitizer only fixes the in-memory copy for this startup
	// Admins should run `pulse-sensor-proxy config migrate-to-file` to fix the file atomically
	if path != "" {
		log.Warn().
			Str("config_file", path).
			Int("removed_duplicate_blocks", len(matches)-1).
			Msg("CONFIG SANITIZED (in-memory only) – duplicate allowed_nodes blocks detected; run 'pulse-sensor-proxy config migrate-to-file' to fix the file permanently")
	} else {
		log.Warn().
			Int("removed_duplicate_blocks", len(matches)-1).
			Msg("CONFIG SANITIZED – duplicate allowed_nodes blocks detected in-memory – this should not happen with Phase 2 config CLI")
	}

	return true, cleaned
}

// parseAllowedSubnets validates and normalizes subnet specifications
func parseAllowedSubnets(cfg []string) ([]string, error) {
	seen := make(map[string]struct{})
	var normalized []string

	for _, raw := range cfg {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		// Try parsing as CIDR
		if _, _, err := net.ParseCIDR(entry); err == nil {
			if _, exists := seen[entry]; !exists {
				seen[entry] = struct{}{}
				normalized = append(normalized, entry)
			}
			continue
		}

		// Try parsing as single IP
		if ip := net.ParseIP(entry); ip != nil {
			norm := entry + "/32"
			if ip.To4() == nil {
				norm = entry + "/128"
			}
			if _, exists := seen[norm]; !exists {
				seen[norm] = struct{}{}
				normalized = append(normalized, norm)
			}
			continue
		}

		// Invalid format
		return nil, fmt.Errorf("invalid subnet or address: %s", entry)
	}

	return normalized, nil
}

const (
	defaultControlPlaneTokenPath   = "/etc/pulse-sensor-proxy/.pulse-control-token"
	defaultControlPlaneRefreshSecs = 60
)
