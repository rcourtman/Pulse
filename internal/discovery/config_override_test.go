package discovery

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery/envdetect"
)

func mustCIDR(t *testing.T, value string) net.IPNet {
	t.Helper()
	_, cidr, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatalf("parse CIDR %s: %v", value, err)
	}
	return *cidr
}

func resetDetectEnvironment() {
	detectEnvironmentFn = envdetect.DetectEnvironment
}

func TestParseCIDRs(t *testing.T) {
	tests := []struct {
		name         string
		values       []string
		wantCount    int
		wantWarnings int
	}{
		{
			name:         "empty input",
			values:       []string{},
			wantCount:    0,
			wantWarnings: 0,
		},
		{
			name:         "nil input",
			values:       nil,
			wantCount:    0,
			wantWarnings: 0,
		},
		{
			name:         "single valid CIDR",
			values:       []string{"192.168.1.0/24"},
			wantCount:    1,
			wantWarnings: 0,
		},
		{
			name:         "multiple valid CIDRs",
			values:       []string{"192.168.1.0/24", "10.0.0.0/8", "172.16.0.0/12"},
			wantCount:    3,
			wantWarnings: 0,
		},
		{
			name:         "CIDR with whitespace",
			values:       []string{"  192.168.1.0/24  ", " 10.0.0.0/8"},
			wantCount:    2,
			wantWarnings: 0,
		},
		{
			name:         "empty string in list",
			values:       []string{"192.168.1.0/24", "", "10.0.0.0/8"},
			wantCount:    2,
			wantWarnings: 0,
		},
		{
			name:         "whitespace-only string",
			values:       []string{"192.168.1.0/24", "   "},
			wantCount:    1,
			wantWarnings: 0,
		},
		{
			name:         "invalid CIDR generates warning",
			values:       []string{"not-a-cidr"},
			wantCount:    0,
			wantWarnings: 1,
		},
		{
			name:         "IP without mask generates warning",
			values:       []string{"192.168.1.1"},
			wantCount:    0,
			wantWarnings: 1,
		},
		{
			name:         "mixed valid and invalid",
			values:       []string{"192.168.1.0/24", "invalid", "10.0.0.0/8"},
			wantCount:    2,
			wantWarnings: 1,
		},
		{
			name:         "IPv6 CIDR",
			values:       []string{"fe80::/10"},
			wantCount:    1,
			wantWarnings: 0,
		},
		{
			name:         "invalid mask",
			values:       []string{"192.168.1.0/33"},
			wantCount:    0,
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warnings []string
			result := parseCIDRs(tt.values, &warnings)

			if len(result) != tt.wantCount {
				t.Errorf("parseCIDRs() returned %d CIDRs, want %d", len(result), tt.wantCount)
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("parseCIDRs() generated %d warnings, want %d", len(warnings), tt.wantWarnings)
			}
		})
	}
}

func TestParseCIDRs_NilWarnings(t *testing.T) {
	// Test that nil warnings pointer doesn't panic
	result := parseCIDRs([]string{"192.168.1.0/24", "invalid"}, nil)
	if len(result) != 1 {
		t.Errorf("parseCIDRs() with nil warnings returned %d CIDRs, want 1", len(result))
	}
}

func TestParseCIDRMap(t *testing.T) {
	tests := []struct {
		name      string
		values    []string
		wantCount int
	}{
		{
			name:      "empty input",
			values:    []string{},
			wantCount: 0,
		},
		{
			name:      "single CIDR",
			values:    []string{"192.168.1.0/24"},
			wantCount: 1,
		},
		{
			name:      "multiple CIDRs",
			values:    []string{"192.168.1.0/24", "10.0.0.0/8"},
			wantCount: 2,
		},
		{
			name:      "duplicate CIDRs are deduplicated",
			values:    []string{"192.168.1.0/24", "192.168.1.0/24"},
			wantCount: 1,
		},
		{
			name:      "invalid CIDR excluded",
			values:    []string{"192.168.1.0/24", "invalid"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warnings []string
			result := parseCIDRMap(tt.values, &warnings)

			if len(result) != tt.wantCount {
				t.Errorf("parseCIDRMap() returned %d entries, want %d", len(result), tt.wantCount)
			}
		})
	}
}

func TestParseCIDRMap_ContainsExpectedCIDR(t *testing.T) {
	values := []string{"192.168.1.0/24", "10.0.0.0/8"}
	var warnings []string
	result := parseCIDRMap(values, &warnings)

	// The canonical form of the CIDR should be in the map
	if _, ok := result["192.168.1.0/24"]; !ok {
		t.Error("parseCIDRMap() result should contain 192.168.1.0/24")
	}
	if _, ok := result["10.0.0.0/8"]; !ok {
		t.Error("parseCIDRMap() result should contain 10.0.0.0/8")
	}
}

func TestEnvironmentFromOverride(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantEnv envdetect.Environment
		wantOK  bool
	}{
		{
			name:    "empty string",
			value:   "",
			wantEnv: envdetect.Unknown,
			wantOK:  false,
		},
		{
			name:    "auto",
			value:   "auto",
			wantEnv: envdetect.Unknown,
			wantOK:  false,
		},
		{
			name:    "AUTO uppercase",
			value:   "AUTO",
			wantEnv: envdetect.Unknown,
			wantOK:  false,
		},
		{
			name:    "Auto mixed case",
			value:   "Auto",
			wantEnv: envdetect.Unknown,
			wantOK:  false,
		},
		{
			name:    "native",
			value:   "native",
			wantEnv: envdetect.Native,
			wantOK:  true,
		},
		{
			name:    "NATIVE uppercase",
			value:   "NATIVE",
			wantEnv: envdetect.Native,
			wantOK:  true,
		},
		{
			name:    "docker_host",
			value:   "docker_host",
			wantEnv: envdetect.DockerHost,
			wantOK:  true,
		},
		{
			name:    "docker_bridge",
			value:   "docker_bridge",
			wantEnv: envdetect.DockerBridge,
			wantOK:  true,
		},
		{
			name:    "lxc_privileged",
			value:   "lxc_privileged",
			wantEnv: envdetect.LXCPrivileged,
			wantOK:  true,
		},
		{
			name:    "lxc_unprivileged",
			value:   "lxc_unprivileged",
			wantEnv: envdetect.LXCUnprivileged,
			wantOK:  true,
		},
		{
			name:    "value with whitespace",
			value:   "  native  ",
			wantEnv: envdetect.Native,
			wantOK:  true,
		},
		{
			name:    "unknown value",
			value:   "unknown_env",
			wantEnv: envdetect.Unknown,
			wantOK:  false,
		},
		{
			name:    "partial match not accepted",
			value:   "docker",
			wantEnv: envdetect.Unknown,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEnv, gotOK := environmentFromOverride(tt.value)
			if gotEnv != tt.wantEnv {
				t.Errorf("environmentFromOverride(%q) environment = %v, want %v", tt.value, gotEnv, tt.wantEnv)
			}
			if gotOK != tt.wantOK {
				t.Errorf("environmentFromOverride(%q) ok = %v, want %v", tt.value, gotOK, tt.wantOK)
			}
		})
	}
}

func TestBuildScanner(t *testing.T) {
	t.Cleanup(resetDetectEnvironment)

	profile := &envdetect.EnvironmentProfile{
		Type:   envdetect.Native,
		Phases: []envdetect.SubnetPhase{{Name: "local", Subnets: []net.IPNet{mustCIDR(t, "192.168.1.0/24")}}},
		Policy: envdetect.DefaultScanPolicy(),
	}
	detectEnvironmentFn = func() (*envdetect.EnvironmentProfile, error) {
		return profile, nil
	}

	scanner, err := BuildScanner(config.DefaultDiscoveryConfig())
	if err != nil {
		t.Fatalf("BuildScanner error: %v", err)
	}
	if scanner == nil {
		t.Fatalf("expected scanner")
	}
}

func TestBuildScannerError(t *testing.T) {
	t.Cleanup(resetDetectEnvironment)

	detectErr := errors.New("detect failed")
	detectEnvironmentFn = func() (*envdetect.EnvironmentProfile, error) {
		return nil, detectErr
	}

	_, err := BuildScanner(config.DefaultDiscoveryConfig())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, detectErr) {
		t.Fatalf("expected wrapped detect error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "discovery.BuildScanner: detect environment") {
		t.Fatalf("expected BuildScanner context in error, got: %v", err)
	}
}

func TestApplyConfigToProfileOverridesAndPolicies(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type: envdetect.Unknown,
		Phases: []envdetect.SubnetPhase{
			{Name: "container_network", Subnets: []net.IPNet{mustCIDR(t, "10.0.0.0/24"), mustCIDR(t, "192.168.0.0/24")}},
			{Name: "local", Subnets: []net.IPNet{mustCIDR(t, "172.16.0.0/24")}},
		},
		Policy: envdetect.DefaultScanPolicy(),
	}

	cfg := config.DiscoveryConfig{
		EnvironmentOverride: "docker_bridge",
		SubnetBlocklist:     []string{"192.168.0.0/24"},
		SubnetAllowlist:     []string{"10.0.0.0/24", "192.168.0.0/24", "invalid"},
		MaxHostsPerScan:     10,
		MaxConcurrent:       20,
		EnableReverseDNS:    true,
		ScanGateways:        true,
		DialTimeout:         1500,
		HTTPTimeout:         2500,
		IPBlocklist:         []string{"192.168.1.10", "invalid"},
	}

	ApplyConfigToProfile(profile, cfg)

	if profile.Type != envdetect.DockerBridge {
		t.Fatalf("expected DockerBridge env, got %v", profile.Type)
	}
	if len(profile.Phases) == 0 || profile.Phases[0].Name != "config_allowlist" {
		t.Fatalf("expected allowlist phase first, got %#v", profile.Phases)
	}
	if len(profile.Phases[0].Subnets) != 1 {
		t.Fatalf("expected allowlist to filter blocklisted subnet")
	}
	if profile.Policy.MaxHostsPerScan != 10 || profile.Policy.MaxConcurrent != 20 {
		t.Fatalf("policy not updated: %+v", profile.Policy)
	}
	if !profile.Policy.EnableReverseDNS || !profile.Policy.ScanGateways {
		t.Fatalf("policy flags not updated")
	}
	if profile.Policy.DialTimeout != 1500*time.Millisecond || profile.Policy.HTTPTimeout != 2500*time.Millisecond {
		t.Fatalf("policy timeouts not updated: %+v", profile.Policy)
	}
	if len(profile.IPBlocklist) != 1 {
		t.Fatalf("expected one IP in blocklist, got %d", len(profile.IPBlocklist))
	}
}

func TestApplyConfigToProfileSkipsEmptyIPBlocklistEntries(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type:   envdetect.Native,
		Policy: envdetect.DefaultScanPolicy(),
	}

	cfg := config.DiscoveryConfig{
		IPBlocklist: []string{"", "   ", "192.168.1.10"},
	}

	ApplyConfigToProfile(profile, cfg)

	if len(profile.IPBlocklist) != 1 {
		t.Fatalf("expected 1 IP in blocklist, got %d", len(profile.IPBlocklist))
	}
}

func TestApplyConfigToProfileInvalidEnvironmentOverride(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type:   envdetect.Native,
		Phases: []envdetect.SubnetPhase{},
		Policy: envdetect.DefaultScanPolicy(),
	}
	cfg := config.DiscoveryConfig{EnvironmentOverride: "invalid_env"}

	ApplyConfigToProfile(profile, cfg)
	if len(profile.Warnings) == 0 {
		t.Fatalf("expected warning for invalid environment override")
	}
}

func TestApplyConfigToProfilePrunesContainerPhase(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type: envdetect.LXCUnprivileged,
		Phases: []envdetect.SubnetPhase{
			{Name: "container_phase", Subnets: []net.IPNet{mustCIDR(t, "10.0.0.0/24")}},
			{Name: "lxc_parent", Subnets: []net.IPNet{mustCIDR(t, "192.168.0.0/24")}},
		},
		Policy: envdetect.DefaultScanPolicy(),
	}

	ApplyConfigToProfile(profile, config.DefaultDiscoveryConfig())
	if len(profile.Phases) != 1 || profile.Phases[0].Name != "lxc_parent" {
		t.Fatalf("expected container phase pruned, got %#v", profile.Phases)
	}
}

func TestApplyConfigToProfileBlocklistWarnings(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type:   envdetect.Unknown,
		Phases: []envdetect.SubnetPhase{},
		Policy: envdetect.DefaultScanPolicy(),
	}

	cfg := config.DiscoveryConfig{
		SubnetBlocklist: []string{"invalid"},
	}
	ApplyConfigToProfile(profile, cfg)
	if len(profile.Warnings) == 0 {
		t.Fatalf("expected warnings for invalid CIDR")
	}
}

func TestApplyConfigToProfileAllowsConfigAllowlist(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type: envdetect.Native,
		Phases: []envdetect.SubnetPhase{
			{Name: "local", Subnets: []net.IPNet{mustCIDR(t, "10.0.0.0/24")}},
		},
		Policy: envdetect.DefaultScanPolicy(),
	}

	cfg := config.DiscoveryConfig{
		SubnetAllowlist: []string{"10.0.0.0/24"},
	}

	ApplyConfigToProfile(profile, cfg)
	if len(profile.Phases) == 0 || profile.Phases[0].Name != "config_allowlist" {
		t.Fatalf("expected allowlist phase, got %#v", profile.Phases)
	}
}

func TestApplyConfigToProfileNoProfileWarnings(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type:         envdetect.Native,
		Phases:       []envdetect.SubnetPhase{},
		Policy:       envdetect.DefaultScanPolicy(),
		Warnings:     nil,
		ExtraTargets: []net.IP{},
	}
	cfg := config.DiscoveryConfig{
		IPBlocklist: []string{"bad"},
	}
	ApplyConfigToProfile(profile, cfg)
	if len(profile.Warnings) == 0 {
		t.Fatalf("expected warnings for invalid IP")
	}
}

func TestApplyConfigToProfileNilProfile(t *testing.T) {
	cfg := config.DiscoveryConfig{
		EnvironmentOverride: "native",
		SubnetAllowlist:     []string{"10.0.0.0/24"},
		SubnetBlocklist:     []string{"10.0.1.0/24"},
		IPBlocklist:         []string{"10.0.0.8"},
	}

	ApplyConfigToProfile(nil, cfg)
}

func TestApplyConfigToProfileBlocksSubnets(t *testing.T) {
	subnet := mustCIDR(t, "10.0.0.0/24")
	profile := &envdetect.EnvironmentProfile{
		Type: envdetect.Native,
		Phases: []envdetect.SubnetPhase{
			{Name: "local", Subnets: []net.IPNet{subnet}},
		},
		Policy: envdetect.DefaultScanPolicy(),
	}
	cfg := config.DiscoveryConfig{
		SubnetBlocklist: []string{"10.0.0.0/24"},
	}

	ApplyConfigToProfile(profile, cfg)
	if len(profile.Phases) != 0 {
		t.Fatalf("expected phases filtered, got %#v", profile.Phases)
	}
}

func TestApplyConfigToProfileAllowsBlockedAllowlist(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type:   envdetect.Native,
		Phases: []envdetect.SubnetPhase{},
		Policy: envdetect.DefaultScanPolicy(),
	}
	cfg := config.DiscoveryConfig{
		SubnetAllowlist: []string{"10.0.0.0/24"},
		SubnetBlocklist: []string{"10.0.0.0/24"},
	}

	ApplyConfigToProfile(profile, cfg)
	if len(profile.Phases) != 0 {
		t.Fatalf("expected allowlist filtered by blocklist")
	}
}

func TestApplyConfigToProfileKeepsUnknownEnvironmentPhases(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type: envdetect.Unknown,
		Phases: []envdetect.SubnetPhase{
			{Name: "local", Subnets: []net.IPNet{mustCIDR(t, "10.0.0.0/24")}},
		},
		Policy: envdetect.DefaultScanPolicy(),
	}
	cfg := config.DiscoveryConfig{
		EnvironmentOverride: "auto",
	}

	ApplyConfigToProfile(profile, cfg)
	if len(profile.Phases) != 1 {
		t.Fatalf("expected phases kept for unknown env")
	}
}

func TestApplyConfigToProfileUsesNewScanner(t *testing.T) {
	profile := &envdetect.EnvironmentProfile{
		Type:   envdetect.Native,
		Phases: []envdetect.SubnetPhase{},
		Policy: envdetect.DefaultScanPolicy(),
	}

	cfg := config.DefaultDiscoveryConfig()
	ApplyConfigToProfile(profile, cfg)

	scanner, err := pkgdiscovery.NewScannerWithProfile(profile), error(nil)
	if err != nil || scanner == nil {
		t.Fatalf("expected scanner from profile")
	}
}

func TestShouldPruneContainerNetworks(t *testing.T) {
	tests := []struct {
		name string
		env  envdetect.Environment
		want bool
	}{
		{
			name: "DockerBridge should prune",
			env:  envdetect.DockerBridge,
			want: true,
		},
		{
			name: "LXCUnprivileged should prune",
			env:  envdetect.LXCUnprivileged,
			want: true,
		},
		{
			name: "Native should not prune",
			env:  envdetect.Native,
			want: false,
		},
		{
			name: "DockerHost should not prune",
			env:  envdetect.DockerHost,
			want: false,
		},
		{
			name: "LXCPrivileged should not prune",
			env:  envdetect.LXCPrivileged,
			want: false,
		},
		{
			name: "Unknown should not prune",
			env:  envdetect.Unknown,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPruneContainerNetworks(tt.env); got != tt.want {
				t.Errorf("shouldPruneContainerNetworks(%v) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestIsLikelyContainerPhase(t *testing.T) {
	tests := []struct {
		name      string
		phaseName string
		want      bool
	}{
		{
			name:      "contains container lowercase",
			phaseName: "docker_container_network",
			want:      true,
		},
		{
			name:      "contains Container mixed case",
			phaseName: "DockerContainer",
			want:      true,
		},
		{
			name:      "contains CONTAINER uppercase",
			phaseName: "CONTAINER_NETWORK",
			want:      true,
		},
		{
			name:      "local_network does not contain container",
			phaseName: "local_network",
			want:      false,
		},
		{
			name:      "host_network does not contain container",
			phaseName: "host_network",
			want:      false,
		},
		{
			name:      "empty string",
			phaseName: "",
			want:      false,
		},
		{
			name:      "whitespace only",
			phaseName: "   ",
			want:      false,
		},
		{
			name:      "container with surrounding whitespace",
			phaseName: "  container_net  ",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLikelyContainerPhase(tt.phaseName); got != tt.want {
				t.Errorf("isLikelyContainerPhase(%q) = %v, want %v", tt.phaseName, got, tt.want)
			}
		})
	}
}

func TestFilterPhasesForEnvironment(t *testing.T) {
	// Helper to create a subnet phase
	makePhase := func(name string) envdetect.SubnetPhase {
		_, subnet, _ := net.ParseCIDR("192.168.1.0/24")
		return envdetect.SubnetPhase{
			Name:    name,
			Subnets: []net.IPNet{*subnet},
		}
	}

	tests := []struct {
		name       string
		phases     []envdetect.SubnetPhase
		env        envdetect.Environment
		wantPhases []string // names of phases that should remain
	}{
		{
			name:       "empty phases",
			phases:     []envdetect.SubnetPhase{},
			env:        envdetect.Native,
			wantPhases: []string{},
		},
		{
			name: "Native keeps local and host phases",
			phases: []envdetect.SubnetPhase{
				makePhase("local_network"),
				makePhase("host_network"),
				makePhase("container_network"),
			},
			env:        envdetect.Native,
			wantPhases: []string{"local_network", "host_network"},
		},
		{
			name: "DockerHost keeps local and host phases",
			phases: []envdetect.SubnetPhase{
				makePhase("local_network"),
				makePhase("container_network"),
			},
			env:        envdetect.DockerHost,
			wantPhases: []string{"local_network"},
		},
		{
			name: "LXCPrivileged keeps local and host phases",
			phases: []envdetect.SubnetPhase{
				makePhase("host_interface"),
				makePhase("container_network"),
			},
			env:        envdetect.LXCPrivileged,
			wantPhases: []string{"host_interface"},
		},
		{
			name: "DockerBridge keeps container, inferred, and host phases",
			phases: []envdetect.SubnetPhase{
				makePhase("local_network"),
				makePhase("container_network"),
				makePhase("inferred_subnet"),
				makePhase("host_network"),
			},
			env:        envdetect.DockerBridge,
			wantPhases: []string{"container_network", "inferred_subnet", "host_network"},
		},
		{
			name: "LXCUnprivileged keeps lxc, container, and parent phases",
			phases: []envdetect.SubnetPhase{
				makePhase("lxc_network"),
				makePhase("container_network"),
				makePhase("parent_host"),
				makePhase("local_network"),
			},
			env:        envdetect.LXCUnprivileged,
			wantPhases: []string{"lxc_network", "container_network", "parent_host"},
		},
		{
			name: "Unknown keeps all phases",
			phases: []envdetect.SubnetPhase{
				makePhase("local_network"),
				makePhase("container_network"),
			},
			env:        envdetect.Unknown,
			wantPhases: []string{"local_network", "container_network"},
		},
		{
			name: "no matching phases preserves original",
			phases: []envdetect.SubnetPhase{
				makePhase("random_network"),
			},
			env:        envdetect.Native,
			wantPhases: []string{"random_network"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &envdetect.EnvironmentProfile{
				Type:   tt.env,
				Phases: tt.phases,
			}

			filterPhasesForEnvironment(profile, tt.env)

			if len(profile.Phases) != len(tt.wantPhases) {
				t.Errorf("filterPhasesForEnvironment() left %d phases, want %d", len(profile.Phases), len(tt.wantPhases))
				return
			}

			for i, want := range tt.wantPhases {
				if profile.Phases[i].Name != want {
					t.Errorf("filterPhasesForEnvironment() phase[%d].Name = %q, want %q", i, profile.Phases[i].Name, want)
				}
			}
		})
	}
}
