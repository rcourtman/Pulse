package updates

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Pre-compiled regexes for performance (avoid recompilation on each call)
var (
	semverRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([^+]+))?(?:\+(.+))?$`)
	rcNumRe  = regexp.MustCompile(`rc\.?(\d+)`)
)

// Version represents a semantic version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// VersionInfo contains detailed version information
type VersionInfo struct {
	Version        string `json:"version"`
	Build          string `json:"build"`
	Runtime        string `json:"runtime"`
	Channel        string `json:"channel,omitempty"`
	IsDocker       bool   `json:"isDocker"`
	IsSourceBuild  bool   `json:"isSourceBuild"`
	IsDevelopment  bool   `json:"isDevelopment"`
	DeploymentType string `json:"deploymentType"`
}

// ParseVersion parses a version string into a Version struct
func ParseVersion(versionStr string) (*Version, error) {
	// Remove 'v' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")

	matches := semverRe.FindStringSubmatch(versionStr)

	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	version := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		version += "-" + v.Prerelease
	}
	if v.Build != "" {
		version += "+" + v.Build
	}
	return version
}

// Compare compares two versions
// Returns:
//
//	-1 if v < other
//	 0 if v == other
//	 1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		return compareInts(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return compareInts(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return compareInts(v.Patch, other.Patch)
	}

	// Handle prerelease versions
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1 // v is release, other is prerelease
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1 // v is prerelease, other is release
	}
	if v.Prerelease != other.Prerelease {
		// Try to parse RC numbers for comparison
		vRC := extractRCNumber(v.Prerelease)
		otherRC := extractRCNumber(other.Prerelease)
		if vRC >= 0 && otherRC >= 0 {
			return compareInts(vRC, otherRC)
		}
		return strings.Compare(v.Prerelease, other.Prerelease)
	}

	return 0
}

// IsNewerThan returns true if v is newer than other
func (v *Version) IsNewerThan(other *Version) bool {
	return v.Compare(other) > 0
}

// IsPrerelease returns true if this is a prerelease version
func (v *Version) IsPrerelease() bool {
	return v.Prerelease != ""
}

// GetCurrentVersion gets the current running version
func GetCurrentVersion() (*VersionInfo, error) {
	allowDockerUpdates := dockerUpdatesAllowed()

	buildInfo := func(raw string, build string, isDev bool) *VersionInfo {
		normalized := normalizeVersionString(raw)
		info := &VersionInfo{
			Version:        normalized,
			Build:          build,
			Runtime:        "go",
			Channel:        detectChannelFromVersion(normalized),
			IsDevelopment:  isDev,
			IsDocker:       isDockerEnvironment(),
			IsSourceBuild:  isSourceBuildEnvironment(),
			DeploymentType: GetDeploymentType(),
		}

		if allowDockerUpdates && info.IsDocker {
			info.IsDocker = false
		}

		return info
	}

	if gitVersion, err := getGitVersion(); err == nil && gitVersion != "" {
		return buildInfo(gitVersion, "development", true), nil
	}

	// Try to read from VERSION file first (release builds)
	versionPaths := []string{
		"VERSION",
		"/opt/pulse/VERSION",
		filepath.Join(filepath.Dir(os.Args[0]), "VERSION"),
	}

	for _, path := range versionPaths {
		versionBytes, err := os.ReadFile(path)
		if err == nil {
			if raw := strings.TrimSpace(string(versionBytes)); raw != "" {
				return buildInfo(raw, "release", false), nil
			}
		}
	}

	// Fall back to git (development builds)
	gitVersion, err := getGitVersion()
	if err == nil && gitVersion != "" {
		return buildInfo(gitVersion, "development", true), nil
	}

	// Final fallback
	return buildInfo("4.26.0", "release", false), nil
}

// normalizeVersionString ensures any version string can be parsed as semantic version.
// Release builds are returned unchanged, while git describe strings and branch names are
// converted to a safe 0.0.0-based prerelease format.
func normalizeVersionString(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")

	if version == "" {
		return "0.0.0-dev"
	}

	if normalized, ok := normalizeGitDescribeVersion(version); ok {
		return normalized
	}

	if _, err := ParseVersion(version); err == nil {
		return version
	}

	return fmt.Sprintf("0.0.0-%s", sanitizePrereleaseIdentifier(version))
}

var gitDescribeRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+(?:-[0-9A-Za-z\.-]+)?)-(\d+)-g([0-9a-fA-F]+)(-dirty)?$`)

func normalizeGitDescribeVersion(version string) (string, bool) {
	matches := gitDescribeRegex.FindStringSubmatch(version)
	if matches == nil {
		return "", false
	}

	base := matches[1]
	if _, err := ParseVersion(base); err != nil {
		return "", false
	}

	build := fmt.Sprintf("git.%s.g%s", matches[2], strings.ToLower(matches[3]))
	if matches[4] != "" {
		build += ".dirty"
	}

	return fmt.Sprintf("%s+%s", base, build), true
}

func sanitizePrereleaseIdentifier(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "dev"
	}

	var b strings.Builder
	lastSep := false

	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9',
			r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSep = false
		case r == '-' || r == '.':
			if lastSep {
				continue
			}
			b.WriteRune(r)
			lastSep = true
		default:
			if !lastSep {
				b.WriteRune('-')
				lastSep = true
			}
		}
	}

	clean := strings.Trim(b.String(), "-.")
	if clean == "" {
		return "dev"
	}
	return strings.ToLower(clean)
}

func detectChannelFromVersion(version string) string {
	versionLower := strings.ToLower(version)
	if strings.Contains(versionLower, "rc") {
		return "rc"
	}
	return "stable"
}

// getGitVersion gets version information from git
func getGitVersion() (string, error) {
	// Get the latest tag
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("no version found")
	}

	return version, nil
}

// isDockerEnvironment checks if running in Docker
func isDockerEnvironment() bool {
	if dockerUpdatesAllowed() {
		return false
	}

	// Check for Docker-specific files
	if fileExists("/.dockerenv") {
		return true
	}

	// Check cgroup for Docker - with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cat", "/proc/1/cgroup")
	data, err := cmd.Output()
	if err == nil && strings.Contains(string(data), "docker") {
		return true
	}

	return false
}

// isSourceBuildEnvironment checks if running from a source build
func isSourceBuildEnvironment() bool {
	markerPaths := []string{
		"BUILD_FROM_SOURCE",
		"/opt/pulse/BUILD_FROM_SOURCE",
		filepath.Join(filepath.Dir(os.Args[0]), "BUILD_FROM_SOURCE"),
	}

	for _, path := range markerPaths {
		if fileExists(path) {
			return true
		}
	}

	return false
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	// Use os.Stat instead of exec.Command for better performance and reliability
	_, err := os.Stat(path)
	return err == nil
}

// GetDeploymentType determines how Pulse was deployed
func GetDeploymentType() string {
	if envBool("PULSE_MOCK_MODE") {
		return "mock"
	}

	// Check if running in Docker
	if isDockerEnvironment() {
		return "docker"
	}

	if isSourceBuildEnvironment() {
		return "source"
	}

	// Check for ProxmoxVE LXC installation (has update command)
	if fileExists("/bin/update") {
		// Read file directly instead of using exec.Command
		data, err := os.ReadFile("/bin/update")
		if err == nil && (strings.Contains(string(data), "pulse.sh") || strings.Contains(string(data), "install.sh")) {
			return "proxmoxve"
		}
	}

	// Check for systemd service to determine installation type
	if fileExists("/etc/systemd/system/pulse-backend.service") {
		// Read file directly instead of using exec.Command
		data, err := os.ReadFile("/etc/systemd/system/pulse-backend.service")
		if err == nil {
			content := string(data)
			if strings.Contains(content, "User=pulse") && strings.Contains(content, "/opt/pulse/bin/pulse") {
				return "proxmoxve"
			}
		}
		return "systemd"
	}

	if fileExists("/etc/systemd/system/pulse.service") {
		return "systemd"
	}

	// Development or manual run
	if strings.Contains(os.Args[0], "go-build") || fileExists(".git") {
		return "development"
	}

	return "manual"
}

// dockerUpdatesAllowed returns true when Docker environments should expose update functionality.
func dockerUpdatesAllowed() bool {
	if envBool("PULSE_ALLOW_DOCKER_UPDATES") {
		return true
	}
	return envBool("PULSE_MOCK_MODE")
}

func envBool(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// compareInts compares two integers
func compareInts(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// extractRCNumber extracts the RC number from a prerelease string like "rc.9" or "rc9"
func extractRCNumber(prerelease string) int {
	matches := rcNumRe.FindStringSubmatch(strings.ToLower(prerelease))
	if len(matches) > 1 {
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return num
		}
	}
	return -1
}
