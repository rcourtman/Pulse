package hostagent

import (
	"errors"
	"net"
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

	mc := &mockCollector{}

	t.Run("trims whitespace", func(t *testing.T) {
		mc.goos = "darwin"
		mc.readFileFn = func(string) ([]byte, error) { return nil, os.ErrNotExist }
		mc.netInterfacesFn = func() ([]net.Interface, error) { return nil, errors.New("no interfaces") }
		result := GetReliableMachineID(mc, "  test-id  ", logger)
		if result != "test-id" {
			t.Errorf("getReliableMachineID trimmed result = %q, want %q", result, "test-id")
		}
	})

	t.Run("Linux prefers /etc/machine-id and formats 32-char IDs", func(t *testing.T) {
		mc.goos = "linux"
		mc.readFileFn = func(name string) ([]byte, error) {
			if name == "/etc/machine-id" {
				return []byte("0123456789abcdef0123456789abcdef\n"), nil
			}
			return nil, os.ErrNotExist
		}
		mc.netInterfacesFn = func() ([]net.Interface, error) { return nil, errors.New("no interfaces") }

		result := GetReliableMachineID(mc, "gopsutil-product-uuid", logger)
		const want = "01234567-89ab-cdef-0123-456789abcdef"
		if result != want {
			t.Errorf("getReliableMachineID() = %q, want %q", result, want)
		}
	})

	t.Run("Linux falls back to MAC when /etc/machine-id missing", func(t *testing.T) {
		mc.goos = "linux"
		mc.readFileFn = func(string) ([]byte, error) { return nil, os.ErrNotExist }
		mc.netInterfacesFn = func() ([]net.Interface, error) {
			return []net.Interface{
				{
					Name:         "eth0",
					HardwareAddr: net.HardwareAddr{0x00, 0x11, 0x22, 0xAA, 0xBB, 0xCC},
				},
			}, nil
		}
		result := GetReliableMachineID(mc, "gopsutil-product-uuid", logger)
		if result != "mac-001122aabbcc" {
			t.Errorf("getReliableMachineID() = %q, want %q", result, "mac-001122aabbcc")
		}
	})

	t.Run("Linux falls back to gopsutil ID when machine-id missing and MAC unavailable", func(t *testing.T) {
		mc.goos = "linux"
		mc.readFileFn = func(string) ([]byte, error) { return nil, os.ErrNotExist }
		mc.netInterfacesFn = func() ([]net.Interface, error) { return nil, errors.New("no interfaces") }
		result := GetReliableMachineID(mc, "gopsutil-product-uuid", logger)
		if result != "gopsutil-product-uuid" {
			t.Errorf("getReliableMachineID() = %q, want %q", result, "gopsutil-product-uuid")
		}
	})

	t.Run("Linux falls back to MAC when machine-id is too short", func(t *testing.T) {
		mc.goos = "linux"
		mc.readFileFn = func(name string) ([]byte, error) {
			if name == "/etc/machine-id" {
				return []byte("short\n"), nil
			}
			return nil, os.ErrNotExist
		}
		mc.netInterfacesFn = func() ([]net.Interface, error) {
			return []net.Interface{
				{
					Name:         "eth0",
					HardwareAddr: net.HardwareAddr{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x01},
				},
			}, nil
		}
		result := GetReliableMachineID(mc, "gopsutil-product-uuid", logger)
		if result != "mac-deadbeef0001" {
			t.Errorf("getReliableMachineID() = %q, want %q", result, "mac-deadbeef0001")
		}
	})

	t.Run("non-Linux uses gopsutil ID", func(t *testing.T) {
		mc.goos = "darwin"
		result := GetReliableMachineID(mc, "12345678-1234-1234-1234-123456789abc", logger)
		if result != "12345678-1234-1234-1234-123456789abc" {
			t.Errorf("Expected gopsutil ID, got %q", result)
		}
	})
}

func TestIsLXCContainer(t *testing.T) {
	mc := &mockCollector{}

	t.Run("/run/systemd/container detects lxc", func(t *testing.T) {
		mc.readFileFn = func(name string) ([]byte, error) {
			if name == "/run/systemd/container" {
				return []byte("lxc\n"), nil
			}
			return nil, os.ErrNotExist
		}
		if !isLXCContainer(mc) {
			t.Fatalf("expected lxc container to be detected")
		}
	})

	t.Run("/proc/1/environ detects container=lxc", func(t *testing.T) {
		mc.readFileFn = func(name string) ([]byte, error) {
			if name == "/proc/1/environ" {
				return []byte("foo=bar\x00container=lxc\x00baz=qux"), nil
			}
			return nil, os.ErrNotExist
		}
		if !isLXCContainer(mc) {
			t.Fatalf("expected lxc container to be detected")
		}
	})

	t.Run("/proc/1/cgroup detects /lxc/", func(t *testing.T) {
		mc.readFileFn = func(name string) ([]byte, error) {
			if name == "/proc/1/cgroup" {
				return []byte("0::/lxc/abcd\n"), nil
			}
			return nil, os.ErrNotExist
		}
		if !isLXCContainer(mc) {
			t.Fatalf("expected lxc container to be detected")
		}
	})

	t.Run("non-lxc container returns false", func(t *testing.T) {
		mc.readFileFn = func(name string) ([]byte, error) {
			if name == "/run/systemd/container" {
				return []byte("docker\n"), nil
			}
			if name == "/proc/1/environ" {
				return []byte("container=podman\x00"), nil
			}
			if name == "/proc/1/cgroup" {
				return []byte("0::/system.slice\n"), nil
			}
			return nil, os.ErrNotExist
		}
		if isLXCContainer(mc) {
			t.Fatalf("expected non-lxc container to not be detected as lxc")
		}
	})
}
