package api

import "testing"

func TestNormalizeDockerAgentArch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty string
		{"empty string", "", ""},

		// amd64 variants
		{"linux-amd64", "linux-amd64", "linux-amd64"},
		{"amd64", "amd64", "linux-amd64"},
		{"x86_64", "x86_64", "linux-amd64"},
		{"x86-64", "x86-64", "linux-amd64"},

		// arm64 variants
		{"linux-arm64", "linux-arm64", "linux-arm64"},
		{"arm64", "arm64", "linux-arm64"},
		{"aarch64", "aarch64", "linux-arm64"},

		// armv7 variants
		{"linux-armv7", "linux-armv7", "linux-armv7"},
		{"armv7", "armv7", "linux-armv7"},
		{"armv7l", "armv7l", "linux-armv7"},
		{"armhf", "armhf", "linux-armv7"},

		// armv6 variants
		{"linux-armv6", "linux-armv6", "linux-armv6"},
		{"armv6", "armv6", "linux-armv6"},
		{"armv6l", "armv6l", "linux-armv6"},

		// 386 variants
		{"linux-386", "linux-386", "linux-386"},
		{"386", "386", "linux-386"},
		{"i386", "i386", "linux-386"},
		{"i686", "i686", "linux-386"},

		// Unknown architecture
		{"unknown arch", "unknown", ""},
		{"invalid arch", "powerpc", ""},
		{"mips", "mips", ""},

		// Case insensitivity
		{"AMD64 uppercase", "AMD64", "linux-amd64"},
		{"X86_64 uppercase", "X86_64", "linux-amd64"},
		{"AARCH64 uppercase", "AARCH64", "linux-arm64"},
		{"ARMV7 uppercase", "ARMV7", "linux-armv7"},
		{"mixed case LinuX-AmD64", "LinuX-AmD64", "linux-amd64"},

		// Whitespace trimming
		{"leading space", "  amd64", "linux-amd64"},
		{"trailing space", "amd64  ", "linux-amd64"},
		{"both spaces", "  amd64  ", "linux-amd64"},
		{"tabs", "\tamd64\t", "linux-amd64"},
		{"mixed whitespace", " \t arm64 \t ", "linux-arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeDockerAgentArch(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDockerAgentArch(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
