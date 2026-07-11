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

// AgentCommandPlatform resolves the install/upgrade command platform for an
// agent-reported platform string. Anything without a canonical non-Linux
// mapping resolves to linux: the long tail of unmatched values is Linux
// distro names, for which the shell installer is correct.
func AgentCommandPlatform(platform string) string {
	switch NormalizeAgentReportedPlatform(platform) {
	case RuntimePlatformWindows:
		return RuntimePlatformWindows
	case RuntimePlatformMacOS:
		return RuntimePlatformMacOS
	case RuntimePlatformFreeBSD:
		return RuntimePlatformFreeBSD
	default:
		return RuntimePlatformLinux
	}
}
