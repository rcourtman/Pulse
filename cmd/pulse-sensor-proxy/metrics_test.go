package main

import "testing"

func TestSanitizeNodeLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Basic cases
		{
			name:  "simple hostname",
			input: "node1",
			want:  "node1",
		},
		{
			name:  "hostname with hyphen",
			input: "node-1",
			want:  "node-1",
		},
		{
			name:  "hostname with underscore",
			input: "node_1",
			want:  "node_1",
		},
		{
			name:  "hostname with dot",
			input: "node.local",
			want:  "node.local",
		},
		{
			name:  "fqdn",
			input: "node1.example.com",
			want:  "node1.example.com",
		},

		// Case conversion
		{
			name:  "uppercase converted to lowercase",
			input: "Node1",
			want:  "node1",
		},
		{
			name:  "mixed case",
			input: "MyNode-Server",
			want:  "mynode-server",
		},
		{
			name:  "all uppercase",
			input: "PRODSERVER01",
			want:  "prodserver01",
		},

		// Special character handling
		{
			name:  "space replaced with underscore",
			input: "node 1",
			want:  "node_1",
		},
		{
			name:  "colon replaced with underscore",
			input: "node:1",
			want:  "node_1",
		},
		{
			name:  "at sign replaced with underscore",
			input: "user@node",
			want:  "user_node",
		},
		{
			name:  "forward slash replaced",
			input: "path/to/node",
			want:  "path_to_node",
		},
		{
			name:  "multiple special chars",
			input: "node!@#$%^&*()+=[]{}|\\:;<>,?/~`",
			want:  "node___________________________",
		},
		{
			name:  "unicode replaced with underscore",
			input: "节点1",
			want:  "__1",
		},
		{
			name:  "emoji replaced",
			input: "server🔥hot",
			want:  "server_hot",
		},

		// Length limits
		{
			name:  "exactly 63 chars preserved",
			input: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0",
			want:  "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0",
		},
		{
			name:  "longer than 63 chars truncated",
			input: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789",
			want:  "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0",
		},
		{
			name:  "very long hostname truncated",
			input: "this-is-a-very-long-hostname-that-exceeds-the-prometheus-label-value-limit-of-sixty-three-characters",
			want:  "this-is-a-very-long-hostname-that-exceeds-the-prometheus-label-",
		},

		// Edge cases
		{
			name:  "empty string returns unknown",
			input: "",
			want:  "unknown",
		},
		{
			name:  "all special chars returns unknown",
			input: "!@#$%",
			want:  "_____",
		},
		{
			name:  "single char",
			input: "a",
			want:  "a",
		},
		{
			name:  "numbers only",
			input: "12345",
			want:  "12345",
		},
		{
			name:  "hyphen start preserved",
			input: "-node",
			want:  "-node",
		},
		{
			name:  "underscore start preserved",
			input: "_node",
			want:  "_node",
		},
		{
			name:  "dot start preserved",
			input: ".hidden",
			want:  ".hidden",
		},

		// Realistic node names
		{
			name:  "proxmox node style",
			input: "pve-node1",
			want:  "pve-node1",
		},
		{
			name:  "ip address style",
			input: "192.168.1.100",
			want:  "192.168.1.100",
		},
		{
			name:  "kubernetes node",
			input: "k8s-worker-01.cluster.local",
			want:  "k8s-worker-01.cluster.local",
		},
		{
			name:  "docker container id prefix",
			input: "abc123def456",
			want:  "abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeNodeLabel(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeNodeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeNodeLabel_MaxLength(t *testing.T) {
	// Test that all outputs are <= 63 chars
	longInputs := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"very-long-hostname-with-many-hyphens-and-lots-of-segments-here",
		"node123456789012345678901234567890123456789012345678901234567890",
	}

	for _, input := range longInputs {
		result := sanitizeNodeLabel(input)
		if len(result) > 63 {
			t.Errorf("sanitizeNodeLabel(%q) length = %d, want <= 63", input, len(result))
		}
	}
}

func TestSanitizeNodeLabel_NeverEmpty(t *testing.T) {
	// Test that output is never empty
	emptyishInputs := []string{
		"",
		"   ",
	}

	for _, input := range emptyishInputs {
		result := sanitizeNodeLabel(input)
		if result == "" {
			t.Errorf("sanitizeNodeLabel(%q) returned empty string, should return 'unknown' or sanitized value", input)
		}
	}
}

func TestSanitizeNodeLabel_Idempotent(t *testing.T) {
	// Applying sanitize twice should give same result as once
	inputs := []string{
		"node1",
		"Node@Server",
		"test.node.local",
		"mixed-CASE_name.123",
	}

	for _, input := range inputs {
		once := sanitizeNodeLabel(input)
		twice := sanitizeNodeLabel(once)
		if once != twice {
			t.Errorf("sanitizeNodeLabel is not idempotent: sanitizeNodeLabel(%q) = %q, sanitizeNodeLabel(%q) = %q",
				input, once, once, twice)
		}
	}
}
