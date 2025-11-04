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

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
