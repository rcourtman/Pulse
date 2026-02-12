package system

import (
	"os"
	"strings"
)

var (
	envGetFn   = os.Getenv
	statFn     = os.Stat
	readFileFn = os.ReadFile
	hostnameFn = os.Hostname
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

var dockerRuntimeMarkers = []string{
	"docker",
	"containerd",
	"kubepods",
	"podman",
	"crio",
	"libpod",
}

// InContainer reports whether Pulse is running inside a containerised environment.
func InContainer() bool {
	// Allow operators to force container behaviour when automatic detection falls short.
	if isTruthy(envGetFn("PULSE_FORCE_CONTAINER")) {
		return true
	}

	if _, err := statFn("/.dockerenv"); err == nil {
		return true
	}
	if _, err := statFn("/run/.containerenv"); err == nil {
		return true
	}

	// Check common environment hints provided by systemd/nspawn, LXC, etc.
	if val := strings.ToLower(strings.TrimSpace(envGetFn("container"))); val != "" && val != "host" {
		return true
	}

	// Some distros expose the container hint through PID 1's environment.
	if data, err := readFileFn("/proc/1/environ"); err == nil {
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "container=") && !strings.Contains(lower, "container=host") {
			return true
		}
	}

	// Fall back to cgroup inspection which covers older Docker/LXC setups.
	if data, err := readFileFn("/proc/1/cgroup"); err == nil {
		content := strings.ToLower(string(data))
		if hasAnyMarker(content, containerMarkers) {
			return true
		}
	}

	return false
}

// DetectDockerContainerName attempts to detect the Docker container name.
// Returns empty string if not in Docker or name cannot be determined.
func DetectDockerContainerName() string {
	if !inDockerRuntime() {
		return ""
	}

	// Method 1: Check hostname (Docker uses container ID or name as hostname)
	if hostname, err := hostnameFn(); err == nil && hostname != "" {
		// Docker hostnames are either short container ID (12 chars) or custom name
		// If it looks like a container ID (hex), skip it - user needs to use name
		if !isHexString(hostname) || len(hostname) > 12 {
			return hostname
		}
	}

	// Method 2: Try reading from /proc/self/cgroup
	if data, err := readFileFn("/proc/self/cgroup"); err == nil {
		// Look for patterns like: 0::/docker/<container-id>
		// But we can't get name from cgroup, only ID
		_ = data // placeholder for future enhancement
	}

	return ""
}

func inDockerRuntime() bool {
	if _, err := statFn("/.dockerenv"); err == nil {
		return true
	}
	if _, err := statFn("/run/.containerenv"); err == nil {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(envGetFn("container"))) {
	case "docker", "podman", "containerd", "crio", "cri-o", "libpod", "kubepods", "oci":
		return true
	}

	for _, path := range []string{"/proc/1/environ", "/proc/self/environ"} {
		if data, err := readFileFn(path); err == nil {
			lower := strings.ToLower(string(data))
			if strings.Contains(lower, "container=docker") ||
				strings.Contains(lower, "container=podman") ||
				strings.Contains(lower, "container=containerd") ||
				strings.Contains(lower, "container=crio") ||
				strings.Contains(lower, "container=cri-o") {
				return true
			}
		}
	}

	for _, path := range []string{"/proc/1/cgroup", "/proc/self/cgroup"} {
		if data, err := readFileFn(path); err == nil {
			if hasAnyMarker(strings.ToLower(string(data)), dockerRuntimeMarkers) {
				return true
			}
		}
	}

	return false
}

func hasAnyMarker(content string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// DetectLXCCTID attempts to detect the Proxmox LXC container ID.
// Returns empty string if not in an LXC container or CTID cannot be determined.
func DetectLXCCTID() string {
	// Method 1: Parse /proc/1/cgroup for LXC container ID
	if data, err := readFileFn("/proc/1/cgroup"); err == nil {
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
	if hostname, err := hostnameFn(); err == nil && isNumeric(hostname) {
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
