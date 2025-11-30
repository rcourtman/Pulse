package hostagent

import (
	"testing"
)

func TestNormalisePlatform(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected string
	}{
		{
			name:     "darwin becomes macos",
			platform: "darwin",
			expected: "macos",
		},
		{
			name:     "Darwin uppercase becomes macos",
			platform: "Darwin",
			expected: "macos",
		},
		{
			name:     "DARWIN all caps becomes macos",
			platform: "DARWIN",
			expected: "macos",
		},
		{
			name:     "linux unchanged",
			platform: "linux",
			expected: "linux",
		},
		{
			name:     "Linux mixed case becomes lowercase",
			platform: "Linux",
			expected: "linux",
		},
		{
			name:     "windows unchanged",
			platform: "windows",
			expected: "windows",
		},
		{
			name:     "freebsd unchanged",
			platform: "freebsd",
			expected: "freebsd",
		},
		{
			name:     "empty string",
			platform: "",
			expected: "",
		},
		{
			name:     "whitespace trimmed",
			platform: "  linux  ",
			expected: "linux",
		},
		{
			name:     "darwin with whitespace",
			platform: "  darwin  ",
			expected: "macos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalisePlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("normalisePlatform(%q) = %q, want %q", tt.platform, result, tt.expected)
			}
		})
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{
			name:     "loopback lowercase",
			flags:    []string{"up", "loopback"},
			expected: true,
		},
		{
			name:     "LOOPBACK uppercase",
			flags:    []string{"up", "LOOPBACK"},
			expected: true,
		},
		{
			name:     "Loopback mixed case",
			flags:    []string{"up", "Loopback"},
			expected: true,
		},
		{
			name:     "loopback only flag",
			flags:    []string{"loopback"},
			expected: true,
		},
		{
			name:     "no loopback flag",
			flags:    []string{"up", "broadcast", "running"},
			expected: false,
		},
		{
			name:     "empty flags",
			flags:    []string{},
			expected: false,
		},
		{
			name:     "nil flags",
			flags:    nil,
			expected: false,
		},
		{
			name:     "loopback first",
			flags:    []string{"loopback", "up"},
			expected: true,
		},
		{
			name:     "loopback in middle",
			flags:    []string{"up", "loopback", "running"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLoopback(tt.flags)
			if result != tt.expected {
				t.Errorf("isLoopback(%v) = %v, want %v", tt.flags, result, tt.expected)
			}
		})
	}
}
