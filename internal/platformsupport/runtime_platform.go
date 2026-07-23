package platformsupport

import "strings"

// Canonical agent runtime platforms used by agent install/upgrade command
// surfaces.
const (
	RuntimePlatformLinux   = "linux"
	RuntimePlatformMacOS   = "macos"
	RuntimePlatformFreeBSD = "freebsd"
	RuntimePlatformWindows = "windows"
)

// NormalizeAgentReportedPlatform maps a raw agent-reported platform string
// onto the canonical runtime platform vocabulary. Legacy agents report
// gopsutil's host.Info().Platform verbatim — a descriptive OS caption such as
// "microsoft windows 11 pro" on Windows or "darwin" on macOS — so matching
// must tolerate captions, not just exact tokens (refs #1555). Values without
// a canonical mapping (Linux distro names such as "ubuntu") are returned
// trimmed and lowercased so distro identity is preserved.
func NormalizeAgentReportedPlatform(platform string) string {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	switch {
	case normalized == "":
		return ""
	case strings.Contains(normalized, "windows"):
		return RuntimePlatformWindows
	case normalized == "darwin" || normalized == "mac" || normalized == "macos" ||
		strings.Contains(normalized, "mac os") || strings.Contains(normalized, "os x"):
		return RuntimePlatformMacOS
	case strings.Contains(normalized, "freebsd"):
		return RuntimePlatformFreeBSD
	}
	return normalized
}

// ResolveAgentRuntimePlatform maps an agent-reported platform onto the
// runtime families for which Pulse publishes agent binaries. Linux agents
// historically reported gopsutil's distro identifier (for example "ubuntu"
// or "mageia") instead of GOOS, so any non-empty value that is not a known
// non-Linux runtime is a Linux distro unless it names an explicitly
// unsupported OS family.
func ResolveAgentRuntimePlatform(platform string) (string, bool) {
	normalized := NormalizeAgentReportedPlatform(platform)
	switch normalized {
	case "":
		return "", false
	case RuntimePlatformWindows,
		RuntimePlatformMacOS,
		RuntimePlatformFreeBSD,
		RuntimePlatformLinux:
		return normalized, true
	}
	if unsupportedAgentRuntimePlatform(normalized) {
		return "", false
	}
	return RuntimePlatformLinux, true
}

func unsupportedAgentRuntimePlatform(platform string) bool {
	for _, token := range []string{
		"aix", "android", "beos", "dragonfly", "haiku", "hurd", "illumos",
		"ios", "js", "netbsd", "openbsd", "plan9", "solaris", "wasip1",
	} {
		if platform == token ||
			strings.HasPrefix(platform, token+" ") ||
			strings.HasPrefix(platform, token+"-") {
			return true
		}
	}
	return false
}

// AgentCommandPlatform resolves the install/upgrade command platform for an
// agent-reported platform string. Anything without a canonical non-Linux
// mapping resolves to linux: the long tail of unmatched values is Linux
// distro names, for which the shell installer is correct.
func AgentCommandPlatform(platform string) string {
	if resolved, ok := ResolveAgentRuntimePlatform(platform); ok {
		return resolved
	}
	return RuntimePlatformLinux
}
