package hostagent

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
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

func TestGetReliableMachineID(t *testing.T) {
	logger := zerolog.Nop()

	// Check if we're running in an LXC container
	inLXC := isLXCContainer()

	t.Run("returns non-empty ID", func(t *testing.T) {
		result := getReliableMachineID("test-gopsutil-id", logger)
		if result == "" {
			t.Error("getReliableMachineID returned empty string")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result := getReliableMachineID("  test-id  ", logger)
		if result == "  test-id  " {
			t.Error("getReliableMachineID did not trim whitespace")
		}
	})

	if inLXC {
		t.Run("LXC uses machine-id", func(t *testing.T) {
			// In LXC, we should get a machine-id regardless of gopsutil input
			result := getReliableMachineID("gopsutil-product-uuid", logger)
			if result == "gopsutil-product-uuid" {
				t.Error("In LXC, getReliableMachineID should use /etc/machine-id, not gopsutil ID")
			}
			// Verify it looks like a formatted UUID
			if len(result) < 32 {
				t.Errorf("Expected UUID-like result, got %q", result)
			}
		})
	} else {
		t.Run("non-LXC uses gopsutil ID", func(t *testing.T) {
			result := getReliableMachineID("12345678-1234-1234-1234-123456789abc", logger)
			if result != "12345678-1234-1234-1234-123456789abc" {
				t.Errorf("Expected gopsutil ID, got %q", result)
			}
		})
	}
}

func TestIsLXCContainer(t *testing.T) {
	// This test documents the detection behavior.
	// On non-LXC systems, isLXCContainer should return false.
	// We can't easily test the true case without mocking filesystem.
	result := isLXCContainer()

	// Check if we're actually in an LXC container
	isActuallyLXC := false
	if data, err := os.ReadFile("/run/systemd/container"); err == nil {
		if string(data) == "lxc" || string(data) == "lxc\n" {
			isActuallyLXC = true
		}
	}

	if result != isActuallyLXC {
		t.Logf("isLXCContainer() = %v (expected %v based on environment)", result, isActuallyLXC)
	}
}
