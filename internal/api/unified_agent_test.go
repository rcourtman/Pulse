package api

import "testing"

func TestNormalizeUnifiedAgentArch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Linux AMD64 variants
		{name: "linux-amd64 canonical", input: "linux-amd64", expected: "linux-amd64"},
		{name: "amd64 short", input: "amd64", expected: "linux-amd64"},
		{name: "x86_64 uname style", input: "x86_64", expected: "linux-amd64"},

		// Linux ARM64 variants
		{name: "linux-arm64 canonical", input: "linux-arm64", expected: "linux-arm64"},
		{name: "arm64 short", input: "arm64", expected: "linux-arm64"},
		{name: "aarch64 uname style", input: "aarch64", expected: "linux-arm64"},

		// Linux ARMv7 variants
		{name: "linux-armv7 canonical", input: "linux-armv7", expected: "linux-armv7"},
		{name: "armv7 short", input: "armv7", expected: "linux-armv7"},
		{name: "armv7l uname style", input: "armv7l", expected: "linux-armv7"},
		{name: "armhf debian style", input: "armhf", expected: "linux-armv7"},

		// Linux ARMv6 variants
		{name: "linux-armv6 canonical", input: "linux-armv6", expected: "linux-armv6"},
		{name: "armv6 short", input: "armv6", expected: "linux-armv6"},

		// Linux 386 variants
		{name: "linux-386 canonical", input: "linux-386", expected: "linux-386"},
		{name: "386 short", input: "386", expected: "linux-386"},
		{name: "i386 style", input: "i386", expected: "linux-386"},
		{name: "i686 style", input: "i686", expected: "linux-386"},

		// macOS variants
		{name: "darwin-amd64 canonical", input: "darwin-amd64", expected: "darwin-amd64"},
		{name: "macos-amd64 alias", input: "macos-amd64", expected: "darwin-amd64"},
		{name: "darwin-arm64 canonical", input: "darwin-arm64", expected: "darwin-arm64"},
		{name: "macos-arm64 alias", input: "macos-arm64", expected: "darwin-arm64"},

		// Windows variants
		{name: "windows-amd64 canonical", input: "windows-amd64", expected: "windows-amd64"},
		{name: "windows-arm64 canonical", input: "windows-arm64", expected: "windows-arm64"},
		{name: "windows-386 canonical", input: "windows-386", expected: "windows-386"},

		// FreeBSD variants
		{name: "freebsd-amd64 canonical", input: "freebsd-amd64", expected: "freebsd-amd64"},
		{name: "freebsd-arm64 canonical", input: "freebsd-arm64", expected: "freebsd-arm64"},

		// Case insensitivity
		{name: "uppercase AMD64", input: "AMD64", expected: "linux-amd64"},
		{name: "mixed case Linux-AMD64", input: "Linux-AMD64", expected: "linux-amd64"},
		{name: "uppercase X86_64", input: "X86_64", expected: "linux-amd64"},
		{name: "mixed case AARCH64", input: "AARCH64", expected: "linux-arm64"},
		{name: "uppercase ARMHF", input: "ARMHF", expected: "linux-armv7"},
		{name: "uppercase DARWIN-ARM64", input: "DARWIN-ARM64", expected: "darwin-arm64"},
		{name: "uppercase FREEBSD-AMD64", input: "FREEBSD-AMD64", expected: "freebsd-amd64"},
		{name: "uppercase WINDOWS-AMD64", input: "WINDOWS-AMD64", expected: "windows-amd64"},

		// Whitespace handling
		{name: "leading space", input: " amd64", expected: "linux-amd64"},
		{name: "trailing space", input: "amd64 ", expected: "linux-amd64"},
		{name: "both spaces", input: " arm64 ", expected: "linux-arm64"},
		{name: "tab chars", input: "\tamd64\t", expected: "linux-amd64"},

		// Invalid/unknown architectures
		{name: "empty string", input: "", expected: ""},
		{name: "unknown arch", input: "sparc", expected: ""},
		{name: "invalid value", input: "not-a-real-arch", expected: ""},
		{name: "partial match", input: "amd", expected: ""},
		{name: "numeric only", input: "64", expected: ""},
		{name: "special characters", input: "amd64!", expected: ""},
		{name: "linux alone", input: "linux", expected: ""},
		{name: "darwin alone", input: "darwin", expected: ""},
		{name: "windows alone", input: "windows", expected: ""},
		{name: "riscv64 unsupported", input: "riscv64", expected: ""},
		{name: "mips unsupported", input: "mips", expected: ""},
		{name: "ppc64 unsupported", input: "ppc64", expected: ""},

		// Edge cases for similar strings
		{name: "armv8 not matched", input: "armv8", expected: ""},
		{name: "arm not matched", input: "arm", expected: ""},
		{name: "x86 not matched", input: "x86", expected: ""},
		{name: "x64 not matched", input: "x64", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeUnifiedAgentArch(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeUnifiedAgentArch(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
