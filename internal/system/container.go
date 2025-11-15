package system

import (
	"os"
	"strings"
)

var containerMarkers = []string{
	"docker",
	"lxc",
	"containerd",
	"kubepods",
	"podman",
	"crio",
	"libpod",
	"lxcfs",
}

// InContainer reports whether Pulse is running inside a containerised environment.
func InContainer() bool {
	// Allow operators to force container behaviour when automatic detection falls short.
	if isTruthy(os.Getenv("PULSE_FORCE_CONTAINER")) {
		return true
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}

	// Check common environment hints provided by systemd/nspawn, LXC, etc.
	if val := strings.ToLower(strings.TrimSpace(os.Getenv("container"))); val != "" && val != "host" {
		return true
	}

	// Some distros expose the container hint through PID 1's environment.
	if data, err := os.ReadFile("/proc/1/environ"); err == nil {
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "container=") && !strings.Contains(lower, "container=host") {
			return true
		}
	}

	// Fall back to cgroup inspection which covers older Docker/LXC setups.
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := strings.ToLower(string(data))
		for _, marker := range containerMarkers {
			if strings.Contains(content, marker) {
				return true
			}
		}
	}

	return false
}

// DetectLXCCTID attempts to detect the Proxmox LXC container ID.
// Returns empty string if not in an LXC container or CTID cannot be determined.
func DetectLXCCTID() string {
	// Method 1: Parse /proc/1/cgroup for LXC container ID
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			// Look for patterns like: 0::/lxc/123 or 0::/lxc.payload.123
			if strings.Contains(line, "/lxc") || strings.Contains(line, "lxc.payload") {
				// Extract digits after lxc/ or lxc.payload.
				parts := strings.Split(line, "/")
				for _, part := range parts {
					part = strings.TrimPrefix(part, "lxc.payload.")
					part = strings.TrimPrefix(part, "lxc.")
					if isNumeric(part) {
						return part
					}
				}
			}
			// Also check for machine-lxc-NNN pattern
			if strings.Contains(line, "machine-lxc") {
				fields := strings.Split(line, "-")
				for _, field := range fields {
					if isNumeric(field) {
						return field
					}
				}
			}
		}
	}

	// Method 2: Check hostname (some LXC containers use CTID as hostname)
	if hostname, err := os.Hostname(); err == nil && isNumeric(hostname) {
		return hostname
	}

	return ""
}

func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
