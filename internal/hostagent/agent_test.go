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

