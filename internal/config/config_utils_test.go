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
