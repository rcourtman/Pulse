package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

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
	AllowedSourceSubnets []string `yaml:"allowed_source_subnets"`
	MetricsAddress       string   `yaml:"metrics_address"`
	LogLevel             string   `yaml:"log_level"`

	AllowIDMappedRoot bool     `yaml:"allow_idmapped_root"`
	AllowedPeerUIDs   []uint32 `yaml:"allowed_peer_uids"`
	AllowedPeerGIDs   []uint32 `yaml:"allowed_peer_gids"`
	AllowedIDMapUsers []string `yaml:"allowed_idmap_users"`

	RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty"`
}

// loadConfig loads configuration from file and environment variables
func loadConfig(configPath string) (*Config, error) {
	cfg := &Config{
		AllowIDMappedRoot: true,
		AllowedIDMapUsers: []string{"root"},
		LogLevel:          "info", // Default log level
	}

	// Try to load config file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
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

	// Append from environment variable if set
	if envSubnets := os.Getenv("PULSE_SENSOR_PROXY_ALLOWED_SUBNETS"); envSubnets != "" {
		envList := strings.Split(envSubnets, ",")
		cfg.AllowedSourceSubnets = append(cfg.AllowedSourceSubnets, envList...)
		log.Info().
			Int("env_subnet_count", len(envList)).
			Msg("Appended subnets from environment variable")
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
