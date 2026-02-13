package config

import (
	"strings"
	"testing"
)

func TestIsPasswordHashed(t *testing.T) {
	// Generate a valid bcrypt hash for testing (60 chars, starts with $2)
	validBcryptHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	tests := []struct {
		name     string
		password string
		expected bool
	}{
		// Valid bcrypt hashes (60 chars, $2a/$2b/$2y prefix)
		{"valid bcrypt 2a", validBcryptHash, true},
		{"valid bcrypt 2b", "$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", true},
		{"valid bcrypt 2y", "$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", true},
		{"valid bcrypt different cost", "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X4.BFB8hMSWy6s/FO", true},

		// Invalid: wrong prefix
		{"no $2 prefix", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456", false},
		{"plain password", "mysecretpassword", false},
		{"$1 prefix (MD5)", "$1$saltsalt$hashed", false},
		{"$5 prefix (SHA-256)", "$5$rounds=5000$saltsalt$hash", false},
		{"$6 prefix (SHA-512)", "$6$rounds=5000$saltsalt$hash", false},

		// Invalid: wrong length
		{"too short", "$2a$10$abc", false},
		{"59 chars (truncated)", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhW", false},
		{"61 chars (too long)", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWyX", false},
		{"55 chars (truncated, logs warning)", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL1", false},

		// Edge cases
		{"empty string", "", false},
		{"just $2", "$2", false},
		{"$2a$", "$2a$", false},
		{"$2a$10$", "$2a$10$", false},
		{"whitespace", "   ", false},
		{"$2 but short", "$2a$10$short", false},

		// Real-world edge cases
		{"hash with newline", validBcryptHash + "\n", false},
		{"hash with trailing space", validBcryptHash + " ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPasswordHashed(tt.password)
			if result != tt.expected {
				t.Errorf("IsPasswordHashed(%q) = %v, want %v", tt.password, result, tt.expected)
			}
		})
	}
}

func TestIsValidDiscoveryEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		// Valid values (case insensitive)
		{"empty string (auto)", "", true},
		{"auto lowercase", "auto", true},
		{"AUTO uppercase", "AUTO", true},
		{"Auto mixed", "Auto", true},
		{"native", "native", true},
		{"NATIVE", "NATIVE", true},
		{"docker_host", "docker_host", true},
		{"DOCKER_HOST", "DOCKER_HOST", true},
		{"docker_bridge", "docker_bridge", true},
		{"DOCKER_BRIDGE", "DOCKER_BRIDGE", true},
		{"lxc_privileged", "lxc_privileged", true},
		{"LXC_PRIVILEGED", "LXC_PRIVILEGED", true},
		{"lxc_unprivileged", "lxc_unprivileged", true},
		{"LXC_UNPRIVILEGED", "LXC_UNPRIVILEGED", true},

		// Valid with whitespace trimming
		{"auto with leading space", " auto", true},
		{"auto with trailing space", "auto ", true},
		{"auto with both spaces", " auto ", true},
		{"native with tabs", "\tnative\t", true},

		// Invalid values
		{"unknown value", "unknown", false},
		{"typo dockerhost", "dockerhost", false},
		{"typo docker-host", "docker-host", false},
		{"kubernetes", "kubernetes", false},
		{"vm", "vm", false},
		{"baremetal", "baremetal", false},
		{"container", "container", false},
		{"partial match docker", "docker", false},
		{"partial match lxc", "lxc", false},
		{"underscore only", "_", false},
		{"random string", "xyzabc", false},
		{"numeric", "123", false},
		{"special chars", "docker@host", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidDiscoveryEnvironment(tt.value)
			if result != tt.expected {
				t.Errorf("IsValidDiscoveryEnvironment(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// Basic splitting
		{"single value", "one", []string{"one"}},
		{"two values", "one,two", []string{"one", "two"}},
		{"three values", "one,two,three", []string{"one", "two", "three"}},

		// Whitespace handling
		{"value with leading space", " one", []string{"one"}},
		{"value with trailing space", "one ", []string{"one"}},
		{"values with spaces around comma", "one , two", []string{"one", "two"}},
		{"values with spaces", " one , two , three ", []string{"one", "two", "three"}},
		{"tabs and spaces", "\tone\t,\ttwo\t", []string{"one", "two"}},

		// Empty handling
		{"empty string", "", []string{}},
		{"only comma", ",", []string{}},
		{"multiple commas", ",,,", []string{}},
		{"comma with spaces", " , , ", []string{}},
		{"empty between values", "one,,two", []string{"one", "two"}},
		{"empty at start", ",one,two", []string{"one", "two"}},
		{"empty at end", "one,two,", []string{"one", "two"}},

		// Real-world examples
		{"CIDR list", "10.0.0.0/8, 192.168.0.0/16", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"hostnames", "node1.local, node2.local, node3.local", []string{"node1.local", "node2.local", "node3.local"}},
		{"mixed content", "http://a.com, https://b.com", []string{"http://a.com", "https://b.com"}},

		// Edge cases
		{"single space", " ", []string{}},
		{"value is just spaces", "   ", []string{}},
		{"comma surrounded by spaces value", "one,   ,two", []string{"one", "two"}},
		{"long value", strings.Repeat("a", 100), []string{strings.Repeat("a", 100)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitAndTrim(%q) returned %d items, want %d", tt.input, len(result), len(tt.expected))
				t.Errorf("got: %v, want: %v", result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestCloneDiscoveryConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  DiscoveryConfig
	}{
		{
			name: "empty config",
			cfg:  DiscoveryConfig{},
		},
		{
			name: "full config with all fields",
			cfg: DiscoveryConfig{
				EnvironmentOverride: "docker_host",
				SubnetAllowlist:     []string{"10.0.0.0/8", "192.168.0.0/16"},
				SubnetBlocklist:     []string{"172.16.0.0/12"},
				MaxHostsPerScan:     512,
				MaxConcurrent:       25,
				EnableReverseDNS:    true,
				ScanGateways:        false,
				DialTimeout:         2000,
				HTTPTimeout:         3000,
			},
		},
		{
			name: "nil slices",
			cfg: DiscoveryConfig{
				EnvironmentOverride: "native",
				SubnetAllowlist:     nil,
				SubnetBlocklist:     nil,
				MaxHostsPerScan:     100,
			},
		},
		{
			name: "empty slices",
			cfg: DiscoveryConfig{
				SubnetAllowlist: []string{},
				SubnetBlocklist: []string{},
			},
		},
		{
			name: "single element slices",
			cfg: DiscoveryConfig{
				SubnetAllowlist: []string{"10.0.0.0/8"},
				SubnetBlocklist: []string{"169.254.0.0/16"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := CloneDiscoveryConfig(tt.cfg)

			// Verify all scalar fields match
			if clone.EnvironmentOverride != tt.cfg.EnvironmentOverride {
				t.Errorf("EnvironmentOverride: got %q, want %q", clone.EnvironmentOverride, tt.cfg.EnvironmentOverride)
			}
			if clone.MaxHostsPerScan != tt.cfg.MaxHostsPerScan {
				t.Errorf("MaxHostsPerScan: got %d, want %d", clone.MaxHostsPerScan, tt.cfg.MaxHostsPerScan)
			}
			if clone.MaxConcurrent != tt.cfg.MaxConcurrent {
				t.Errorf("MaxConcurrent: got %d, want %d", clone.MaxConcurrent, tt.cfg.MaxConcurrent)
			}
			if clone.EnableReverseDNS != tt.cfg.EnableReverseDNS {
				t.Errorf("EnableReverseDNS: got %v, want %v", clone.EnableReverseDNS, tt.cfg.EnableReverseDNS)
			}
			if clone.ScanGateways != tt.cfg.ScanGateways {
				t.Errorf("ScanGateways: got %v, want %v", clone.ScanGateways, tt.cfg.ScanGateways)
			}
			if clone.DialTimeout != tt.cfg.DialTimeout {
				t.Errorf("DialTimeout: got %d, want %d", clone.DialTimeout, tt.cfg.DialTimeout)
			}
			if clone.HTTPTimeout != tt.cfg.HTTPTimeout {
				t.Errorf("HTTPTimeout: got %d, want %d", clone.HTTPTimeout, tt.cfg.HTTPTimeout)
			}

			// Verify slice contents match
			if len(clone.SubnetAllowlist) != len(tt.cfg.SubnetAllowlist) {
				t.Errorf("SubnetAllowlist length: got %d, want %d", len(clone.SubnetAllowlist), len(tt.cfg.SubnetAllowlist))
			} else {
				for i, v := range clone.SubnetAllowlist {
					if v != tt.cfg.SubnetAllowlist[i] {
						t.Errorf("SubnetAllowlist[%d]: got %q, want %q", i, v, tt.cfg.SubnetAllowlist[i])
					}
				}
			}

			if len(clone.SubnetBlocklist) != len(tt.cfg.SubnetBlocklist) {
				t.Errorf("SubnetBlocklist length: got %d, want %d", len(clone.SubnetBlocklist), len(tt.cfg.SubnetBlocklist))
			} else {
				for i, v := range clone.SubnetBlocklist {
					if v != tt.cfg.SubnetBlocklist[i] {
						t.Errorf("SubnetBlocklist[%d]: got %q, want %q", i, v, tt.cfg.SubnetBlocklist[i])
					}
				}
			}
		})
	}
}

// TestCloneDiscoveryConfig_SliceIndependence verifies that modifying the clone
// does not affect the original (deep copy verification).
func TestCloneDiscoveryConfig_SliceIndependence(t *testing.T) {
	original := DiscoveryConfig{
		SubnetAllowlist: []string{"10.0.0.0/8", "192.168.0.0/16"},
		SubnetBlocklist: []string{"172.16.0.0/12", "169.254.0.0/16"},
	}

	clone := CloneDiscoveryConfig(original)

	// Modify the clone's slices
	clone.SubnetAllowlist[0] = "modified"
	clone.SubnetBlocklist[0] = "modified"

	// Original should be unchanged
	if original.SubnetAllowlist[0] != "10.0.0.0/8" {
		t.Errorf("Original SubnetAllowlist was modified: got %q", original.SubnetAllowlist[0])
	}
	if original.SubnetBlocklist[0] != "172.16.0.0/12" {
		t.Errorf("Original SubnetBlocklist was modified: got %q", original.SubnetBlocklist[0])
	}

	// Append to clone slices
	clone.SubnetAllowlist = append(clone.SubnetAllowlist, "new")

	// Original length should be unchanged
	if len(original.SubnetAllowlist) != 2 {
		t.Errorf("Original SubnetAllowlist length changed: got %d", len(original.SubnetAllowlist))
	}
}

// TestCloneDiscoveryConfig_NilSliceIndependence verifies nil slices remain nil in clone.
func TestCloneDiscoveryConfig_NilSliceIndependence(t *testing.T) {
	original := DiscoveryConfig{
		SubnetAllowlist: nil,
		SubnetBlocklist: nil,
	}

	clone := CloneDiscoveryConfig(original)

	if clone.SubnetAllowlist != nil {
		t.Errorf("Expected nil SubnetAllowlist, got %v", clone.SubnetAllowlist)
	}
	if clone.SubnetBlocklist != nil {
		t.Errorf("Expected nil SubnetBlocklist, got %v", clone.SubnetBlocklist)
	}
}

func TestNormalizeDiscoveryConfig(t *testing.T) {
	defaults := DefaultDiscoveryConfig()

	tests := []struct {
		name     string
		cfg      DiscoveryConfig
		expected DiscoveryConfig
	}{
		{
			name: "empty config gets defaults",
			cfg:  DiscoveryConfig{},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "valid environment preserved",
			cfg: DiscoveryConfig{
				EnvironmentOverride: "docker_host",
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: "docker_host",
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "invalid environment falls back to auto",
			cfg: DiscoveryConfig{
				EnvironmentOverride: "invalid_env",
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: "auto",
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "environment with whitespace trimmed",
			cfg: DiscoveryConfig{
				EnvironmentOverride: "  native  ",
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: "native",
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "positive values preserved",
			cfg: DiscoveryConfig{
				MaxHostsPerScan: 100,
				MaxConcurrent:   10,
				DialTimeout:     500,
				HTTPTimeout:     1000,
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     100,
				MaxConcurrent:       10,
				DialTimeout:         500,
				HTTPTimeout:         1000,
			},
		},
		{
			name: "zero values get defaults",
			cfg: DiscoveryConfig{
				MaxHostsPerScan: 0,
				MaxConcurrent:   0,
				DialTimeout:     0,
				HTTPTimeout:     0,
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "negative values get defaults",
			cfg: DiscoveryConfig{
				MaxHostsPerScan: -1,
				MaxConcurrent:   -10,
				DialTimeout:     -100,
				HTTPTimeout:     -1000,
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "subnet allowlist sanitized",
			cfg: DiscoveryConfig{
				SubnetAllowlist: []string{" 10.0.0.0/8 ", "10.0.0.0/8", "", "192.168.0.0/16"},
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{"10.0.0.0/8", "192.168.0.0/16"},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "subnet blocklist sanitized",
			cfg: DiscoveryConfig{
				SubnetBlocklist: []string{" 172.16.0.0/12 ", "", "172.16.0.0/12"},
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     []string{"172.16.0.0/12"},
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "nil blocklist gets defaults",
			cfg: DiscoveryConfig{
				SubnetBlocklist: nil,
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "empty blocklist after sanitization stays empty",
			cfg: DiscoveryConfig{
				SubnetBlocklist: []string{"", "  ", ""},
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     []string{}, // sanitizeCIDRList returns []string{} not nil
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "boolean fields preserved",
			cfg: DiscoveryConfig{
				EnableReverseDNS: false,
				ScanGateways:     true,
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: defaults.EnvironmentOverride,
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				EnableReverseDNS:    false,
				ScanGateways:        true,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
		{
			name: "all valid environments",
			cfg: DiscoveryConfig{
				EnvironmentOverride: "lxc_privileged",
			},
			expected: DiscoveryConfig{
				EnvironmentOverride: "lxc_privileged",
				SubnetAllowlist:     []string{},
				SubnetBlocklist:     defaults.SubnetBlocklist,
				MaxHostsPerScan:     defaults.MaxHostsPerScan,
				MaxConcurrent:       defaults.MaxConcurrent,
				DialTimeout:         defaults.DialTimeout,
				HTTPTimeout:         defaults.HTTPTimeout,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDiscoveryConfig(tt.cfg)

			if result.EnvironmentOverride != tt.expected.EnvironmentOverride {
				t.Errorf("EnvironmentOverride: got %q, want %q", result.EnvironmentOverride, tt.expected.EnvironmentOverride)
			}
			if result.MaxHostsPerScan != tt.expected.MaxHostsPerScan {
				t.Errorf("MaxHostsPerScan: got %d, want %d", result.MaxHostsPerScan, tt.expected.MaxHostsPerScan)
			}
			if result.MaxConcurrent != tt.expected.MaxConcurrent {
				t.Errorf("MaxConcurrent: got %d, want %d", result.MaxConcurrent, tt.expected.MaxConcurrent)
			}
			if result.EnableReverseDNS != tt.expected.EnableReverseDNS {
				t.Errorf("EnableReverseDNS: got %v, want %v", result.EnableReverseDNS, tt.expected.EnableReverseDNS)
			}
			if result.ScanGateways != tt.expected.ScanGateways {
				t.Errorf("ScanGateways: got %v, want %v", result.ScanGateways, tt.expected.ScanGateways)
			}
			if result.DialTimeout != tt.expected.DialTimeout {
				t.Errorf("DialTimeout: got %d, want %d", result.DialTimeout, tt.expected.DialTimeout)
			}
			if result.HTTPTimeout != tt.expected.HTTPTimeout {
				t.Errorf("HTTPTimeout: got %d, want %d", result.HTTPTimeout, tt.expected.HTTPTimeout)
			}

			// Check slice equality
			if len(result.SubnetAllowlist) != len(tt.expected.SubnetAllowlist) {
				t.Errorf("SubnetAllowlist length: got %d, want %d", len(result.SubnetAllowlist), len(tt.expected.SubnetAllowlist))
			} else {
				for i, v := range result.SubnetAllowlist {
					if v != tt.expected.SubnetAllowlist[i] {
						t.Errorf("SubnetAllowlist[%d]: got %q, want %q", i, v, tt.expected.SubnetAllowlist[i])
					}
				}
			}

			if len(result.SubnetBlocklist) != len(tt.expected.SubnetBlocklist) {
				t.Errorf("SubnetBlocklist length: got %d, want %d", len(result.SubnetBlocklist), len(tt.expected.SubnetBlocklist))
			} else {
				for i, v := range result.SubnetBlocklist {
					if v != tt.expected.SubnetBlocklist[i] {
						t.Errorf("SubnetBlocklist[%d]: got %q, want %q", i, v, tt.expected.SubnetBlocklist[i])
					}
				}
			}
		})
	}
}

// TestNormalizeDiscoveryConfig_DoesNotModifyInput verifies the original config is not mutated.
func TestNormalizeDiscoveryConfig_DoesNotModifyInput(t *testing.T) {
	original := DiscoveryConfig{
		EnvironmentOverride: "  docker_host  ",
		SubnetAllowlist:     []string{" 10.0.0.0/8 ", "192.168.0.0/16"},
		SubnetBlocklist:     []string{" 172.16.0.0/12 "},
		MaxHostsPerScan:     -1,
	}

	// Store original values
	origEnv := original.EnvironmentOverride
	origAllowlist := make([]string, len(original.SubnetAllowlist))
	copy(origAllowlist, original.SubnetAllowlist)
	origBlocklist := make([]string, len(original.SubnetBlocklist))
	copy(origBlocklist, original.SubnetBlocklist)
	origMaxHosts := original.MaxHostsPerScan

	_ = NormalizeDiscoveryConfig(original)

	// Verify original is unchanged
	if original.EnvironmentOverride != origEnv {
		t.Errorf("Original EnvironmentOverride modified: got %q, want %q", original.EnvironmentOverride, origEnv)
	}
	if original.MaxHostsPerScan != origMaxHosts {
		t.Errorf("Original MaxHostsPerScan modified: got %d, want %d", original.MaxHostsPerScan, origMaxHosts)
	}
	for i, v := range original.SubnetAllowlist {
		if v != origAllowlist[i] {
			t.Errorf("Original SubnetAllowlist[%d] modified: got %q, want %q", i, v, origAllowlist[i])
		}
	}
	for i, v := range original.SubnetBlocklist {
		if v != origBlocklist[i] {
			t.Errorf("Original SubnetBlocklist[%d] modified: got %q, want %q", i, v, origBlocklist[i])
		}
	}
}

func TestSanitizeCIDRList(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		// Nil and empty handling
		{"nil input", nil, nil},
		{"empty slice", []string{}, nil},

		// Single entry
		{"single valid entry", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}},
		{"single entry with leading space", []string{" 10.0.0.0/8"}, []string{"10.0.0.0/8"}},
		{"single entry with trailing space", []string{"10.0.0.0/8 "}, []string{"10.0.0.0/8"}},
		{"single entry with both spaces", []string{"  10.0.0.0/8  "}, []string{"10.0.0.0/8"}},
		{"single entry with tabs", []string{"\t10.0.0.0/8\t"}, []string{"10.0.0.0/8"}},

		// Multiple valid entries (no duplicates)
		{"two valid entries", []string{"10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"three valid entries", []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}, []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}},

		// Duplicates at various positions
		{"duplicate at end", []string{"10.0.0.0/8", "192.168.0.0/16", "10.0.0.0/8"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"duplicate at start", []string{"10.0.0.0/8", "10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"duplicate in middle", []string{"10.0.0.0/8", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}, []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}},
		{"all duplicates", []string{"10.0.0.0/8", "10.0.0.0/8", "10.0.0.0/8"}, []string{"10.0.0.0/8"}},
		{"multiple duplicates", []string{"a", "b", "a", "c", "b", "d", "a"}, []string{"a", "b", "c", "d"}},

		// Whitespace variations
		{"entry with leading whitespace", []string{"  10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"entry with trailing whitespace", []string{"10.0.0.0/8  ", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"mixed whitespace", []string{" 10.0.0.0/8", "192.168.0.0/16 ", "  172.16.0.0/12  "}, []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}},
		{"tabs and spaces mixed", []string{"\t10.0.0.0/8 ", " \t192.168.0.0/16\t ", "  172.16.0.0/12"}, []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}},

		// Entries that become duplicates after trimming
		{"duplicates after trim", []string{" 10.0.0.0/8", "10.0.0.0/8 ", "  10.0.0.0/8  "}, []string{"10.0.0.0/8"}},
		{"duplicates after trim with others", []string{" 10.0.0.0/8", "192.168.0.0/16", "10.0.0.0/8 "}, []string{"10.0.0.0/8", "192.168.0.0/16"}},

		// All empty/whitespace entries
		{"single empty string", []string{""}, []string{}},
		{"multiple empty strings", []string{"", "", ""}, []string{}},
		{"only whitespace", []string{" ", "  ", "\t", " \t "}, []string{}},
		{"mixed empty and whitespace", []string{"", " ", "", "\t"}, []string{}},

		// Mixed valid and empty entries
		{"valid with empty at start", []string{"", "10.0.0.0/8", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"valid with empty in middle", []string{"10.0.0.0/8", "", "192.168.0.0/16"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"valid with empty at end", []string{"10.0.0.0/8", "192.168.0.0/16", ""}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"valid with multiple empty", []string{"", "10.0.0.0/8", "", "192.168.0.0/16", "", ""}, []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"valid with whitespace entries", []string{" ", "10.0.0.0/8", "  ", "192.168.0.0/16", "\t"}, []string{"10.0.0.0/8", "192.168.0.0/16"}},

		// Order preservation
		{"order preserved basic", []string{"z", "a", "m"}, []string{"z", "a", "m"}},
		{"order preserved with duplicates", []string{"z", "a", "m", "z", "a"}, []string{"z", "a", "m"}},
		{"order preserved complex", []string{"third", "first", "second", "third", "first"}, []string{"third", "first", "second"}},
		{"order preserved after trim", []string{" c ", "a", " b ", "c", " a "}, []string{"c", "a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeCIDRList(tt.input)

			// Check nil vs empty slice distinction
			if tt.expected == nil {
				if result != nil {
					t.Errorf("sanitizeCIDRList(%v) = %v, want nil", tt.input, result)
				}
				return
			}

			if result == nil {
				t.Errorf("sanitizeCIDRList(%v) = nil, want %v", tt.input, tt.expected)
				return
			}

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("sanitizeCIDRList(%v) returned %d items, want %d", tt.input, len(result), len(tt.expected))
				t.Errorf("got: %v, want: %v", result, tt.expected)
				return
			}

			// Check each element
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("sanitizeCIDRList(%v)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestDiscoveryConfigUnmarshalJSON_InvalidJSON(t *testing.T) {
	var cfg DiscoveryConfig
	err := cfg.UnmarshalJSON([]byte(`{invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDiscoveryConfigUnmarshalJSON_ModernFields(t *testing.T) {
	var cfg DiscoveryConfig
	data := `{
		"environment_override": "docker_host",
		"subnet_allowlist": ["192.168.1.0/24"],
		"subnet_blocklist": ["10.0.0.0/8"],
		"max_hosts_per_scan": 100,
		"max_concurrent": 5,
		"enable_reverse_dns": true,
		"scan_gateways": false,
		"dial_timeout_ms": 2000,
		"http_timeout_ms": 5000
	}`

	err := cfg.UnmarshalJSON([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EnvironmentOverride != "docker_host" {
		t.Errorf("EnvironmentOverride = %q, want 'docker_host'", cfg.EnvironmentOverride)
	}
	if len(cfg.SubnetAllowlist) != 1 || cfg.SubnetAllowlist[0] != "192.168.1.0/24" {
		t.Errorf("SubnetAllowlist = %v, want ['192.168.1.0/24']", cfg.SubnetAllowlist)
	}
	if cfg.MaxHostsPerScan != 100 {
		t.Errorf("MaxHostsPerScan = %d, want 100", cfg.MaxHostsPerScan)
	}
}

func TestDiscoveryConfigUnmarshalJSON_LegacyFields(t *testing.T) {
	var cfg DiscoveryConfig
	data := `{
		"environmentOverride": "lxc_privileged",
		"subnetAllowlist": ["172.16.0.0/12"],
		"maxHostsPerScan": 50,
		"enableReverseDns": false
	}`

	err := cfg.UnmarshalJSON([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EnvironmentOverride != "lxc_privileged" {
		t.Errorf("EnvironmentOverride = %q, want 'lxc_privileged'", cfg.EnvironmentOverride)
	}
	if len(cfg.SubnetAllowlist) != 1 || cfg.SubnetAllowlist[0] != "172.16.0.0/12" {
		t.Errorf("SubnetAllowlist = %v, want ['172.16.0.0/12']", cfg.SubnetAllowlist)
	}
	if cfg.MaxHostsPerScan != 50 {
		t.Errorf("MaxHostsPerScan = %d, want 50", cfg.MaxHostsPerScan)
	}
}

func TestDiscoveryConfigUnmarshalJSON_EmptyObject(t *testing.T) {
	var cfg DiscoveryConfig
	err := cfg.UnmarshalJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get defaults
	defaults := DefaultDiscoveryConfig()
	if cfg.MaxHostsPerScan != defaults.MaxHostsPerScan {
		t.Errorf("MaxHostsPerScan = %d, want default %d", cfg.MaxHostsPerScan, defaults.MaxHostsPerScan)
	}
}

func TestConfigDeepCopy_Nil(t *testing.T) {
	var cfg *Config
	if clone := cfg.DeepCopy(); clone != nil {
		t.Fatalf("DeepCopy() = %#v, want nil", clone)
	}
}

func TestConfigDeepCopy_IndependentMutableState(t *testing.T) {
	original := &Config{
		PVEInstances: []PVEInstance{
			{Name: "pve-a", Host: "https://pve-a.local"},
		},
		PBSInstances: []PBSInstance{
			{Name: "pbs-a", Host: "https://pbs-a.local"},
		},
		PMGInstances: []PMGInstance{
			{Name: "pmg-a", Host: "https://pmg-a.local"},
		},
		APITokens: []APITokenRecord{
			{
				ID:     "token-1",
				Scopes: []string{ScopeSettingsRead},
				OrgIDs: []string{"org-a"},
			},
		},
		SuppressedEnvMigrations: []string{"hash-1"},
		EnvOverrides: map[string]bool{
			"PULSE_AUTH_USER": true,
		},
		Discovery: DiscoveryConfig{
			EnvironmentOverride: "docker_host",
			SubnetAllowlist:     []string{"10.0.0.0/8"},
			SubnetBlocklist:     []string{"169.254.0.0/16"},
			IPBlocklist:         []string{"10.0.0.99"},
		},
		OIDC: &OIDCConfig{
			Enabled:        true,
			Scopes:         []string{"openid"},
			AllowedGroups:  []string{"admins"},
			AllowedDomains: []string{"example.com"},
			AllowedEmails:  []string{"admin@example.com"},
			GroupRoleMappings: map[string]string{
				"admins": "admin",
			},
			EnvOverrides: map[string]bool{
				"OIDC_ISSUER_URL": true,
			},
		},
	}

	clone := original.DeepCopy()
	if clone == nil {
		t.Fatalf("DeepCopy() returned nil")
	}

	clone.PVEInstances[0].Host = "https://changed.local"
	clone.PBSInstances[0].Host = "https://changed-pbs.local"
	clone.PMGInstances[0].Host = "https://changed-pmg.local"
	clone.APITokens[0].Scopes[0] = ScopeSettingsWrite
	clone.APITokens[0].OrgIDs[0] = "org-b"
	clone.SuppressedEnvMigrations[0] = "hash-2"
	clone.EnvOverrides["PULSE_AUTH_USER"] = false
	clone.Discovery.SubnetAllowlist[0] = "172.16.0.0/12"
	clone.Discovery.SubnetBlocklist[0] = "10.0.0.0/8"
	clone.Discovery.IPBlocklist[0] = "10.0.0.100"
	clone.OIDC.Scopes[0] = "email"
	clone.OIDC.AllowedGroups[0] = "ops"
	clone.OIDC.AllowedDomains[0] = "changed.example.com"
	clone.OIDC.AllowedEmails[0] = "ops@example.com"
	clone.OIDC.GroupRoleMappings["admins"] = "viewer"
	clone.OIDC.EnvOverrides["OIDC_ISSUER_URL"] = false

	if got := original.PVEInstances[0].Host; got != "https://pve-a.local" {
		t.Fatalf("original PVEInstances mutated: %q", got)
	}
	if got := original.PBSInstances[0].Host; got != "https://pbs-a.local" {
		t.Fatalf("original PBSInstances mutated: %q", got)
	}
	if got := original.PMGInstances[0].Host; got != "https://pmg-a.local" {
		t.Fatalf("original PMGInstances mutated: %q", got)
	}
	if got := original.APITokens[0].Scopes[0]; got != ScopeSettingsRead {
		t.Fatalf("original APIToken scopes mutated: %q", got)
	}
	if got := original.APITokens[0].OrgIDs[0]; got != "org-a" {
		t.Fatalf("original APIToken org IDs mutated: %q", got)
	}
	if got := original.SuppressedEnvMigrations[0]; got != "hash-1" {
		t.Fatalf("original suppressed migrations mutated: %q", got)
	}
	if got := original.EnvOverrides["PULSE_AUTH_USER"]; got != true {
		t.Fatalf("original env override mutated: %v", got)
	}
	if got := original.Discovery.SubnetAllowlist[0]; got != "10.0.0.0/8" {
		t.Fatalf("original subnet allowlist mutated: %q", got)
	}
	if got := original.Discovery.SubnetBlocklist[0]; got != "169.254.0.0/16" {
		t.Fatalf("original subnet blocklist mutated: %q", got)
	}
	if got := original.Discovery.IPBlocklist[0]; got != "10.0.0.99" {
		t.Fatalf("original IP blocklist mutated: %q", got)
	}
	if got := original.OIDC.Scopes[0]; got != "openid" {
		t.Fatalf("original OIDC scopes mutated: %q", got)
	}
	if got := original.OIDC.AllowedGroups[0]; got != "admins" {
		t.Fatalf("original OIDC allowed groups mutated: %q", got)
	}
	if got := original.OIDC.AllowedDomains[0]; got != "example.com" {
		t.Fatalf("original OIDC allowed domains mutated: %q", got)
	}
	if got := original.OIDC.AllowedEmails[0]; got != "admin@example.com" {
		t.Fatalf("original OIDC allowed emails mutated: %q", got)
	}
	if got := original.OIDC.GroupRoleMappings["admins"]; got != "admin" {
		t.Fatalf("original OIDC group role mapping mutated: %q", got)
	}
	if got := original.OIDC.EnvOverrides["OIDC_ISSUER_URL"]; got != true {
		t.Fatalf("original OIDC env override mutated: %v", got)
	}
}

func TestClusterEndpoint_EffectiveIP(t *testing.T) {
	tests := []struct {
		name string
		e    ClusterEndpoint
		want string
	}{
		{
			name: "uses override when set",
			e: ClusterEndpoint{
				IP:         "10.0.0.11",
				IPOverride: "192.168.1.11",
			},
			want: "192.168.1.11",
		},
		{
			name: "falls back to discovered IP when override missing",
			e: ClusterEndpoint{
				IP: "10.0.0.12",
			},
			want: "10.0.0.12",
		},
		{
			name: "returns empty when both are empty",
			e:    ClusterEndpoint{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.EffectiveIP(); got != tt.want {
				t.Fatalf("EffectiveIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
