package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// Config holds proxy configuration
type Config struct {
	AllowedSourceSubnets []string `yaml:"allowed_source_subnets"`
	MetricsAddress       string   `yaml:"metrics_address"`
}

// loadConfig loads configuration from file and environment variables
func loadConfig(configPath string) (*Config, error) {
	cfg := &Config{}

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

	return cfg, nil
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
