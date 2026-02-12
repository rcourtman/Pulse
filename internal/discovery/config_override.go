package discovery

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

var detectEnvironmentFn = envdetect.DetectEnvironment

const (
	maxDiscoveryCIDREntries        = 512
	maxDiscoveryCIDRLength         = 64
	maxDiscoveryIPBlocklistEntries = 4096
	maxDiscoveryIPLength           = 64
)

// BuildScanner creates a discovery scanner configured using the supplied discovery config.
func BuildScanner(cfg config.DiscoveryConfig) (*pkgdiscovery.Scanner, error) {
	cfg = config.NormalizeDiscoveryConfig(cfg)

	profile, err := detectEnvironmentFn()
	if err != nil {
		return nil, fmt.Errorf("discovery.BuildScanner: detect environment: %w", err)
	}

	ApplyConfigToProfile(profile, cfg)
	return pkgdiscovery.NewScannerWithProfile(profile), nil
}

// ApplyConfigToProfile mutates the supplied environment profile according to the discovery config.
func ApplyConfigToProfile(profile *envdetect.EnvironmentProfile, cfg config.DiscoveryConfig) {
	if profile == nil {
		return
	}

	// Environment override
	if env, ok := environmentFromOverride(cfg.EnvironmentOverride); ok {
		profile.Type = env
		filterPhasesForEnvironment(profile, env)
	} else if cfg.EnvironmentOverride != "" && strings.ToLower(cfg.EnvironmentOverride) != "auto" {
		profile.Warnings = append(profile.Warnings, fmt.Sprintf("Unknown environment override: %s", cfg.EnvironmentOverride))
	}

	// Apply subnet blocklist
	blocked := parseCIDRMap(cfg.SubnetBlocklist, &profile.Warnings)
	if len(blocked) > 0 {
		var filtered []envdetect.SubnetPhase
		for _, phase := range profile.Phases {
			var kept []net.IPNet
			for _, subnet := range phase.Subnets {
				if _, blocked := blocked[subnet.String()]; blocked {
					continue
				}
				kept = append(kept, subnet)
			}
			if len(kept) > 0 {
				phase.Subnets = kept
				filtered = append(filtered, phase)
			}
		}
		profile.Phases = filtered
	}

	// Apply subnet allowlist as highest priority phase
	if len(cfg.SubnetAllowlist) > 0 {
		allowlist := parseCIDRs(cfg.SubnetAllowlist, &profile.Warnings)
		if len(allowlist) > 0 {
			if len(blocked) > 0 {
				filtered := allowlist[:0]
				for _, subnet := range allowlist {
					if _, skip := blocked[subnet.String()]; skip {
						continue
					}
					filtered = append(filtered, subnet)
				}
				allowlist = filtered
			}
		}
		if len(allowlist) > 0 {
			allowPhase := envdetect.SubnetPhase{
				Name:       "config_allowlist",
				Subnets:    allowlist,
				Confidence: 1.0,
				Priority:   0,
			}
			profile.Phases = append([]envdetect.SubnetPhase{allowPhase}, profile.Phases...)
		}
	}

	if len(cfg.SubnetAllowlist) == 0 && shouldPruneContainerNetworks(profile.Type) {
		pruned := make([]envdetect.SubnetPhase, 0, len(profile.Phases))
		for _, phase := range profile.Phases {
			if isLikelyContainerPhase(phase.Name) {
				continue
			}
			pruned = append(pruned, phase)
		}
		if len(pruned) > 0 {
			profile.Phases = pruned
		}
	}

	// Override scan policy
	if cfg.MaxHostsPerScan > 0 {
		profile.Policy.MaxHostsPerScan = cfg.MaxHostsPerScan
	}
	if cfg.MaxConcurrent > 0 {
		profile.Policy.MaxConcurrent = cfg.MaxConcurrent
	}
	profile.Policy.EnableReverseDNS = cfg.EnableReverseDNS
	profile.Policy.ScanGateways = cfg.ScanGateways

	if cfg.DialTimeout > 0 {
		profile.Policy.DialTimeout = time.Duration(cfg.DialTimeout) * time.Millisecond
	}
	if cfg.HTTPTimeout > 0 {
		profile.Policy.HTTPTimeout = time.Duration(cfg.HTTPTimeout) * time.Millisecond
	}

	// Apply IP blocklist (individual IPs to skip, e.g. already-configured Proxmox hosts)
	limit := len(cfg.IPBlocklist)
	if limit > maxDiscoveryIPBlocklistEntries {
		limit = maxDiscoveryIPBlocklistEntries
		profile.Warnings = append(profile.Warnings, fmt.Sprintf("IP blocklist exceeds max entries (%d); extra entries ignored", maxDiscoveryIPBlocklistEntries))
	}

	seenIPs := make(map[string]struct{}, len(profile.IPBlocklist)+limit)
	for _, existingIP := range profile.IPBlocklist {
		seenIPs[existingIP.String()] = struct{}{}
	}

	for _, ipStr := range cfg.IPBlocklist[:limit] {
		ipStr = strings.TrimSpace(ipStr)
		if ipStr == "" {
			continue
		}
		if len(ipStr) > maxDiscoveryIPLength {
			profile.Warnings = append(profile.Warnings, fmt.Sprintf("IP blocklist entry exceeds max length (%d) and was ignored", maxDiscoveryIPLength))
			continue
		}
		if ip := net.ParseIP(ipStr); ip != nil {
			canonicalIP := ip.String()
			if _, exists := seenIPs[canonicalIP]; exists {
				continue
			}
			seenIPs[canonicalIP] = struct{}{}
			profile.IPBlocklist = append(profile.IPBlocklist, ip)
		} else {
			profile.Warnings = append(profile.Warnings, fmt.Sprintf("Invalid IP in blocklist: %s", ipStr))
		}
	}
}

func shouldPruneContainerNetworks(env envdetect.Environment) bool {
	return env == envdetect.DockerBridge || env == envdetect.LXCUnprivileged
}

func isLikelyContainerPhase(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "container")
}

func parseCIDRs(values []string, warnings *[]string) []net.IPNet {
	limit := len(values)
	if limit > maxDiscoveryCIDREntries {
		limit = maxDiscoveryCIDREntries
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("CIDR list exceeds max entries (%d); extra entries ignored", maxDiscoveryCIDREntries))
		}
	}

	var subnets []net.IPNet
	for _, value := range values[:limit] {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > maxDiscoveryCIDRLength {
			if warnings != nil {
				*warnings = append(*warnings, fmt.Sprintf("CIDR entry exceeds max length (%d) and was ignored", maxDiscoveryCIDRLength))
			}
			continue
		}
		_, ipNet, err := net.ParseCIDR(value)
		if err != nil {
			if warnings != nil {
				*warnings = append(*warnings, fmt.Sprintf("Invalid CIDR '%s' ignored", value))
			}
			continue
		}
		subnets = append(subnets, *ipNet)
	}
	return subnets
}

func parseCIDRMap(values []string, warnings *[]string) map[string]struct{} {
	cidrs := parseCIDRs(values, warnings)
	result := make(map[string]struct{}, len(cidrs))
	for _, cidr := range cidrs {
		result[cidr.String()] = struct{}{}
	}
	return result
}

func environmentFromOverride(value string) (envdetect.Environment, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", "auto":
		return envdetect.Unknown, false
	case "native":
		return envdetect.Native, true
	case "docker_host":
		return envdetect.DockerHost, true
	case "docker_bridge":
		return envdetect.DockerBridge, true
	case "lxc_privileged":
		return envdetect.LXCPrivileged, true
	case "lxc_unprivileged":
		return envdetect.LXCUnprivileged, true
	default:
		return envdetect.Unknown, false
	}
}

func filterPhasesForEnvironment(profile *envdetect.EnvironmentProfile, env envdetect.Environment) {
	if len(profile.Phases) == 0 {
		return
	}

	var keep []envdetect.SubnetPhase
	for _, phase := range profile.Phases {
		name := strings.ToLower(phase.Name)
		switch env {
		case envdetect.Native, envdetect.DockerHost, envdetect.LXCPrivileged:
			if strings.Contains(name, "local") || strings.Contains(name, "host") {
				keep = append(keep, phase)
			}
		case envdetect.DockerBridge:
			if strings.Contains(name, "container") || strings.Contains(name, "inferred") || strings.Contains(name, "host") {
				keep = append(keep, phase)
			}
		case envdetect.LXCUnprivileged:
			if strings.Contains(name, "lxc") || strings.Contains(name, "container") || strings.Contains(name, "parent") {
				keep = append(keep, phase)
			}
		default:
			keep = append(keep, phase)
		}
	}

	if len(keep) > 0 {
		profile.Phases = keep
	}
}
