package envdetect

import (
	"net"
	"testing"
	"time"
)

func TestEnvironment_String(t *testing.T) {
	tests := []struct {
		env      Environment
		expected string
	}{
		{Unknown, "unknown"},
		{Native, "native"},
		{DockerHost, "docker_host"},
		{DockerBridge, "docker_bridge"},
		{LXCPrivileged, "lxc_privileged"},
		{LXCUnprivileged, "lxc_unprivileged"},
		{Environment(99), "unknown"}, // Invalid value
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.env.String()
			if result != tc.expected {
				t.Errorf("Environment(%d).String() = %q, want %q", tc.env, result, tc.expected)
			}
		})
	}
}

func TestDefaultScanPolicy(t *testing.T) {
	policy := DefaultScanPolicy()

	if policy.MaxConcurrent != 50 {
		t.Errorf("MaxConcurrent = %d, want 50", policy.MaxConcurrent)
	}
	if policy.DialTimeout != time.Second {
		t.Errorf("DialTimeout = %v, want 1s", policy.DialTimeout)
	}
	if policy.HTTPTimeout != 2*time.Second {
		t.Errorf("HTTPTimeout = %v, want 2s", policy.HTTPTimeout)
	}
	if policy.MaxHostsPerScan != 1024 {
		t.Errorf("MaxHostsPerScan = %d, want 1024", policy.MaxHostsPerScan)
	}
	if !policy.EnableReverseDNS {
		t.Error("EnableReverseDNS should be true by default")
	}
	if !policy.ScanGateways {
		t.Error("ScanGateways should be true by default")
	}
}

func TestScanPolicy_Fields(t *testing.T) {
	policy := ScanPolicy{
		MaxConcurrent:    100,
		DialTimeout:      5 * time.Second,
		HTTPTimeout:      10 * time.Second,
		MaxHostsPerScan:  2048,
		EnableReverseDNS: false,
		ScanGateways:     false,
	}

	if policy.MaxConcurrent != 100 {
		t.Errorf("MaxConcurrent = %d, want 100", policy.MaxConcurrent)
	}
	if policy.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout = %v, want 5s", policy.DialTimeout)
	}
	if policy.HTTPTimeout != 10*time.Second {
		t.Errorf("HTTPTimeout = %v, want 10s", policy.HTTPTimeout)
	}
	if policy.MaxHostsPerScan != 2048 {
		t.Errorf("MaxHostsPerScan = %d, want 2048", policy.MaxHostsPerScan)
	}
	if policy.EnableReverseDNS {
		t.Error("EnableReverseDNS should be false")
	}
	if policy.ScanGateways {
		t.Error("ScanGateways should be false")
	}
}

func TestSubnetPhase_Fields(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("192.168.1.0/24")

	phase := SubnetPhase{
		Name:       "test_phase",
		Subnets:    []net.IPNet{*subnet},
		Confidence: 0.85,
		Priority:   2,
	}

	if phase.Name != "test_phase" {
		t.Errorf("Name = %q, want test_phase", phase.Name)
	}
	if len(phase.Subnets) != 1 {
		t.Errorf("Subnets count = %d, want 1", len(phase.Subnets))
	}
	if phase.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", phase.Confidence)
	}
	if phase.Priority != 2 {
		t.Errorf("Priority = %d, want 2", phase.Priority)
	}
}

func TestEnvironmentProfile_Fields(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
	extraIP := net.ParseIP("192.168.1.1")

	profile := EnvironmentProfile{
		Type: Native,
		Phases: []SubnetPhase{
			{Name: "local_networks", Subnets: []net.IPNet{*subnet}},
		},
		ExtraTargets: []net.IP{extraIP},
		Policy:       DefaultScanPolicy(),
		Confidence:   0.95,
		Warnings:     []string{"test warning"},
		Metadata:     map[string]string{"key": "value"},
	}

	if profile.Type != Native {
		t.Errorf("Type = %v, want Native", profile.Type)
	}
	if len(profile.Phases) != 1 {
		t.Errorf("Phases count = %d, want 1", len(profile.Phases))
	}
	if len(profile.ExtraTargets) != 1 {
		t.Errorf("ExtraTargets count = %d, want 1", len(profile.ExtraTargets))
	}
	if profile.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", profile.Confidence)
	}
	if len(profile.Warnings) != 1 {
		t.Errorf("Warnings count = %d, want 1", len(profile.Warnings))
	}
	if profile.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %q, want value", profile.Metadata["key"])
	}
}

func TestParseHexIP(t *testing.T) {
	tests := []struct {
		name      string
		hexIP     string
		expected  net.IP
		expectErr bool
	}{
		{
			name:     "valid gateway 192.168.1.1",
			hexIP:    "0101A8C0", // 192.168.1.1 in little-endian hex
			expected: net.IPv4(192, 168, 1, 1),
		},
		{
			name:     "valid gateway 10.0.0.1",
			hexIP:    "0100000A", // 10.0.0.1 in little-endian hex
			expected: net.IPv4(10, 0, 0, 1),
		},
		{
			name:     "all zeros",
			hexIP:    "00000000",
			expected: net.IPv4(0, 0, 0, 0),
		},
		{
			name:     "broadcast",
			hexIP:    "FFFFFFFF",
			expected: net.IPv4(255, 255, 255, 255),
		},
		{
			name:      "too short",
			hexIP:     "0101A8",
			expectErr: true,
		},
		{
			name:      "too long",
			hexIP:     "0101A8C0FF",
			expectErr: true,
		},
		{
			name:      "invalid hex",
			hexIP:     "ZZZZZZZZ",
			expectErr: true,
		},
		{
			name:      "empty",
			hexIP:     "",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseHexIP(tc.hexIP)
			if tc.expectErr {
				if err == nil {
					t.Errorf("parseHexIP(%q) expected error, got nil", tc.hexIP)
				}
				return
			}
			if err != nil {
				t.Errorf("parseHexIP(%q) unexpected error: %v", tc.hexIP, err)
				return
			}
			if !result.Equal(tc.expected) {
				t.Errorf("parseHexIP(%q) = %v, want %v", tc.hexIP, result, tc.expected)
			}
		})
	}
}

func TestTryCommonSubnets(t *testing.T) {
	subnets := tryCommonSubnets()

	if len(subnets) == 0 {
		t.Error("tryCommonSubnets() returned empty slice")
	}

	// Verify we get expected common subnets
	expectedCIDRs := map[string]bool{
		"192.168.1.0/24": true,
		"192.168.0.0/24": true,
		"10.0.0.0/24":    true,
		"172.16.0.0/24":  true,
		"192.168.2.0/24": true,
	}

	for _, subnet := range subnets {
		cidr := subnet.String()
		if !expectedCIDRs[cidr] {
			t.Errorf("Unexpected subnet %s in fallback list", cidr)
		}
		delete(expectedCIDRs, cidr)
	}

	for cidr := range expectedCIDRs {
		t.Errorf("Expected subnet %s not found in fallback list", cidr)
	}
}

func TestProfileWithWarning(t *testing.T) {
	profile := &EnvironmentProfile{
		Warnings: []string{"existing warning"},
	}

	result := profileWithWarning(profile, "new warning")

	// Should return same profile
	if result != profile {
		t.Error("profileWithWarning should return the same profile instance")
	}

	// Should have appended warning
	if len(result.Warnings) != 2 {
		t.Errorf("Warnings count = %d, want 2", len(result.Warnings))
	}
	if result.Warnings[1] != "new warning" {
		t.Errorf("Warnings[1] = %q, want 'new warning'", result.Warnings[1])
	}
}

func TestProfileWithWarning_EmptyWarning(t *testing.T) {
	profile := &EnvironmentProfile{
		Warnings: []string{"existing"},
	}

	result := profileWithWarning(profile, "")

	// Empty warning should not be appended
	if len(result.Warnings) != 1 {
		t.Errorf("Warnings count = %d, want 1 (empty warning should not be appended)", len(result.Warnings))
	}
}

func TestAddFallbackSubnets(t *testing.T) {
	profile := &EnvironmentProfile{
		Phases:     []SubnetPhase{},
		Confidence: 0.0,
		Warnings:   []string{},
	}

	result, err := addFallbackSubnets(profile)
	if err != nil {
		t.Errorf("addFallbackSubnets unexpected error: %v", err)
	}

	// Should have added a phase
	if len(result.Phases) != 1 {
		t.Errorf("Phases count = %d, want 1", len(result.Phases))
	}

	// Phase should have correct properties
	phase := result.Phases[0]
	if phase.Name != "fallback_common_subnets" {
		t.Errorf("Phase name = %q, want fallback_common_subnets", phase.Name)
	}
	if phase.Confidence != 0.3 {
		t.Errorf("Phase confidence = %f, want 0.3", phase.Confidence)
	}
	if phase.Priority != 10 {
		t.Errorf("Phase priority = %d, want 10", phase.Priority)
	}
	if len(phase.Subnets) == 0 {
		t.Error("Phase should have subnets")
	}

	// Confidence should be set if it was 0
	if result.Confidence != 0.3 {
		t.Errorf("Confidence = %f, want 0.3", result.Confidence)
	}

	// Should have warning
	foundWarning := false
	for _, w := range result.Warnings {
		if w == "Using fallback private subnets; consider manual subnet configuration" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected fallback warning not found")
	}
}

func TestAddFallbackSubnets_PreservesExistingConfidence(t *testing.T) {
	profile := &EnvironmentProfile{
		Phases:     []SubnetPhase{},
		Confidence: 0.7,
		Warnings:   []string{},
	}

	result, _ := addFallbackSubnets(profile)

	// Existing confidence should be preserved
	if result.Confidence != 0.7 {
		t.Errorf("Confidence = %f, want 0.7 (should preserve existing)", result.Confidence)
	}
}

func TestEnvironmentConstants(t *testing.T) {
	// Verify enum values are distinct
	values := map[Environment]bool{}
	envs := []Environment{Unknown, Native, DockerHost, DockerBridge, LXCPrivileged, LXCUnprivileged}

	for _, env := range envs {
		if values[env] {
			t.Errorf("Duplicate environment value: %d", env)
		}
		values[env] = true
	}

	// Unknown should be 0 (default value)
	if Unknown != 0 {
		t.Errorf("Unknown = %d, want 0", Unknown)
	}
}

func TestSubnetPhase_MultipleSubnets(t *testing.T) {
	_, subnet1, _ := net.ParseCIDR("192.168.1.0/24")
	_, subnet2, _ := net.ParseCIDR("10.0.0.0/8")
	_, subnet3, _ := net.ParseCIDR("172.16.0.0/16")

	phase := SubnetPhase{
		Name:    "multi_subnet_phase",
		Subnets: []net.IPNet{*subnet1, *subnet2, *subnet3},
	}

	if len(phase.Subnets) != 3 {
		t.Errorf("Subnets count = %d, want 3", len(phase.Subnets))
	}

	// Verify each subnet
	if phase.Subnets[0].String() != "192.168.1.0/24" {
		t.Errorf("Subnet[0] = %s, want 192.168.1.0/24", phase.Subnets[0].String())
	}
	if phase.Subnets[1].String() != "10.0.0.0/8" {
		t.Errorf("Subnet[1] = %s, want 10.0.0.0/8", phase.Subnets[1].String())
	}
	if phase.Subnets[2].String() != "172.16.0.0/16" {
		t.Errorf("Subnet[2] = %s, want 172.16.0.0/16", phase.Subnets[2].String())
	}
}

func TestEnvironmentProfile_EmptyMetadata(t *testing.T) {
	profile := EnvironmentProfile{
		Type:     Native,
		Metadata: nil,
	}

	// Accessing nil map should not panic for reads
	if profile.Metadata != nil {
		t.Error("Metadata should be nil")
	}

	// Verify we can initialize and use metadata
	profile.Metadata = make(map[string]string)
	profile.Metadata["test"] = "value"

	if profile.Metadata["test"] != "value" {
		t.Errorf("Metadata[test] = %q, want value", profile.Metadata["test"])
	}
}

func TestEnvironmentProfile_MultiplePhases(t *testing.T) {
	_, subnet1, _ := net.ParseCIDR("192.168.1.0/24")
	_, subnet2, _ := net.ParseCIDR("10.0.0.0/24")

	profile := EnvironmentProfile{
		Type: DockerBridge,
		Phases: []SubnetPhase{
			{Name: "container_network", Subnets: []net.IPNet{*subnet1}, Priority: 1, Confidence: 0.95},
			{Name: "inferred_host", Subnets: []net.IPNet{*subnet2}, Priority: 2, Confidence: 0.7},
		},
		Confidence: 0.85,
	}

	if len(profile.Phases) != 2 {
		t.Errorf("Phases count = %d, want 2", len(profile.Phases))
	}

	// Verify phase ordering by priority
	if profile.Phases[0].Priority >= profile.Phases[1].Priority {
		t.Error("First phase should have lower priority than second")
	}
}

func TestParseHexIP_LittleEndian(t *testing.T) {
	// Test that little-endian parsing is correct
	// 0101A8C0 should parse to 192.168.1.1
	// Bytes: 01 01 A8 C0
	// Little-endian: C0 A8 01 01 = 192.168.1.1
	ip, err := parseHexIP("0101A8C0")
	if err != nil {
		t.Fatalf("parseHexIP unexpected error: %v", err)
	}

	expected := net.IPv4(192, 168, 1, 1)
	if !ip.Equal(expected) {
		t.Errorf("parseHexIP(0101A8C0) = %v, want %v", ip, expected)
	}
}

func TestScanPolicy_ZeroValues(t *testing.T) {
	policy := ScanPolicy{}

	// Verify zero values
	if policy.MaxConcurrent != 0 {
		t.Error("MaxConcurrent should be 0 by default")
	}
	if policy.DialTimeout != 0 {
		t.Error("DialTimeout should be 0 by default")
	}
	if policy.HTTPTimeout != 0 {
		t.Error("HTTPTimeout should be 0 by default")
	}
	if policy.MaxHostsPerScan != 0 {
		t.Error("MaxHostsPerScan should be 0 by default")
	}
	if policy.EnableReverseDNS {
		t.Error("EnableReverseDNS should be false by default")
	}
	if policy.ScanGateways {
		t.Error("ScanGateways should be false by default")
	}
}

func TestEnvironmentProfile_WithExtraTargets(t *testing.T) {
	targets := []net.IP{
		net.ParseIP("192.168.1.1"),
		net.ParseIP("192.168.1.254"),
		net.ParseIP("10.0.0.1"),
	}

	profile := EnvironmentProfile{
		ExtraTargets: targets,
	}

	if len(profile.ExtraTargets) != 3 {
		t.Errorf("ExtraTargets count = %d, want 3", len(profile.ExtraTargets))
	}

	// Verify IP addresses
	if !profile.ExtraTargets[0].Equal(net.ParseIP("192.168.1.1")) {
		t.Errorf("ExtraTargets[0] = %v, want 192.168.1.1", profile.ExtraTargets[0])
	}
}

func TestEnvironmentProfile_WarningsSlice(t *testing.T) {
	profile := EnvironmentProfile{
		Warnings: []string{
			"Warning 1",
			"Warning 2",
			"Warning 3",
		},
	}

	if len(profile.Warnings) != 3 {
		t.Errorf("Warnings count = %d, want 3", len(profile.Warnings))
	}

	// Append more warnings
	profile.Warnings = append(profile.Warnings, "Warning 4")
	if len(profile.Warnings) != 4 {
		t.Errorf("Warnings count after append = %d, want 4", len(profile.Warnings))
	}
}
