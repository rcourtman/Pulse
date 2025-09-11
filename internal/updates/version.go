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
	Version       string `json:"version"`
	Build         string `json:"build"`
	Runtime       string `json:"runtime"`
	Channel       string `json:"channel,omitempty"`
	IsDocker      bool   `json:"isDocker"`
	IsDevelopment bool   `json:"isDevelopment"`
	DeploymentType string `json:"deploymentType"`
}

// ParseVersion parses a version string into a Version struct
func ParseVersion(versionStr string) (*Version, error) {
	// Remove 'v' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")
	
	// Regular expression for semantic versioning
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([^+]+))?(?:\+(.+))?$`)
	matches := re.FindStringSubmatch(versionStr)
	
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
//   -1 if v < other
//    0 if v == other
//    1 if v > other
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
	// Try to get version from git first (development)
	gitVersion, err := getGitVersion()
	if err == nil && gitVersion != "" {
		// Determine channel from git version
		channel := "stable"
		if strings.Contains(strings.ToLower(gitVersion), "rc") {
			channel = "rc"
		}
		return &VersionInfo{
			Version:        gitVersion,
			Build:          "development",
			Runtime:        "go",
			Channel:        channel,
			IsDevelopment:  true,
			IsDocker:       isDockerEnvironment(),
			DeploymentType: GetDeploymentType(),
		}, nil
	}
	
	// Try to read from VERSION file in multiple locations
	versionPaths := []string{
		"VERSION",
		"/opt/pulse/VERSION",
		filepath.Join(filepath.Dir(os.Args[0]), "VERSION"),
	}
	
	for _, path := range versionPaths {
		versionBytes, err := os.ReadFile(path)
		if err == nil {
			version := strings.TrimSpace(string(versionBytes))
			// Determine channel from version string
			channel := "stable"
			if strings.Contains(strings.ToLower(version), "rc") {
				channel = "rc"
			}
			return &VersionInfo{
				Version:        version,
				Build:          "release",
				Runtime:        "go",
				Channel:        channel,
				IsDevelopment:  false,
				IsDocker:       isDockerEnvironment(),
				DeploymentType: GetDeploymentType(),
			}, nil
		}
	}
	
	// Final fallback
	version := "4.15.0-test.448"
	channel := "stable"
	if strings.Contains(strings.ToLower(version), "rc") {
		channel = "rc"
	}
	return &VersionInfo{
		Version:        version,
		Build:          "release",
		Runtime:        "go",
		Channel:        channel,
		IsDevelopment:  false,
		IsDocker:       isDockerEnvironment(),
		DeploymentType: GetDeploymentType(),
	}, nil
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

// fileExists checks if a file exists
func fileExists(path string) bool {
	// Use os.Stat instead of exec.Command for better performance and reliability
	_, err := os.Stat(path)
	return err == nil
}

// GetDeploymentType determines how Pulse was deployed
func GetDeploymentType() string {
	// Check if running in Docker
	if isDockerEnvironment() {
		return "docker"
	}
	
	// Check for ProxmoxVE LXC installation (has update command)
	if fileExists("/bin/update") {
		// Read file directly instead of using exec.Command
		data, err := os.ReadFile("/bin/update")
		if err == nil && strings.Contains(string(data), "pulse.sh") {
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
	re := regexp.MustCompile(`rc\.?(\d+)`)
	matches := re.FindStringSubmatch(strings.ToLower(prerelease))
	if len(matches) > 1 {
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return num
		}
	}
	return -1
}