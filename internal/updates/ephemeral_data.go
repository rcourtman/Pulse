package updates

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
)

const defaultMountInfoPath = "/proc/self/mountinfo"

// IsDockerDataDirEphemeral reports whether this process is running in Docker
// with its runtime data directory on the container's root filesystem rather
// than on a mounted volume or bind mount. Non-Docker deployments, unreadable
// mountinfo environments, and installs setting PULSE_SUPPRESS_EPHEMERAL_DATA_WARNING
// return false so operators never receive false positives.
func IsDockerDataDirEphemeral(dataDir string) bool {
	return isDockerDataDirEphemeral(
		isRunningInDockerForPersistenceCheck(),
		ResolveDockerDataDir(dataDir),
		os.Getenv("PULSE_SUPPRESS_EPHEMERAL_DATA_WARNING"),
		defaultMountInfoPath,
	)
}

// ResolveDockerDataDir returns the normalized container data directory used for
// Docker persistence checks and operator guidance.
func ResolveDockerDataDir(explicit string) string {
	target := strings.TrimSpace(explicit)
	if target == "" {
		target = strings.TrimSpace(os.Getenv("PULSE_DATA_DIR"))
	}
	if target == "" {
		target = "/data"
	}
	return path.Clean(target)
}

// DockerEphemeralDataWarning returns an actionable operator warning when Pulse
// is running in Docker without a persistent mount covering dataDir, or an empty
// string otherwise.
func DockerEphemeralDataWarning(dataDir string) string {
	resolved := ResolveDockerDataDir(dataDir)
	if !IsDockerDataDirEphemeral(resolved) {
		return ""
	}
	return formatDockerEphemeralDataWarning(resolved)
}

func formatDockerEphemeralDataWarning(dataDir string) string {
	resolved := ResolveDockerDataDir(dataDir)
	return fmt.Sprintf(
		"Container storage is not persistent: %s is not mounted on a volume, so recreating this container will erase your configuration, authentication, and history. Export a backup and mount a persistent volume (for example: -v pulse-data:%s) before updating.",
		resolved,
		resolved,
	)
}

func isRunningInDockerForPersistenceCheck() bool {
	if dockerUpdatesAllowed() {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_DOCKER")), "true") {
		return true
	}
	if isDockerEnvironment() {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(GetDeploymentType()), "docker")
}

func isEphemeralDataWarningSuppressed(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func isDockerDataDirEphemeral(isDocker bool, dataDir string, suppressEnv string, mountinfoPath string) bool {
	if !isDocker || isEphemeralDataWarningSuppressed(suppressEnv) {
		return false
	}
	target := path.Clean(strings.TrimSpace(dataDir))
	if target == "" || target == "." || target == "/" || !strings.HasPrefix(target, "/") {
		return false
	}

	mounts, ok := readMountInfoPoints(mountinfoPath)
	if !ok || len(mounts) == 0 {
		return false
	}

	return !isPathCoveredByNonRootMount(target, mounts)
}

func readMountInfoPoints(mountinfoPath string) (map[string]struct{}, bool) {
	file, err := os.Open(mountinfoPath)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	mounts := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		mountPoint := path.Clean(unescapeMountInfoField(fields[4]))
		if mountPoint != "" {
			mounts[mountPoint] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, false
	}
	return mounts, true
}

func isPathCoveredByNonRootMount(target string, mounts map[string]struct{}) bool {
	check := path.Clean(target)
	for check != "" && check != "/" {
		if _, ok := mounts[check]; ok {
			return true
		}
		parent := path.Dir(check)
		if parent == check {
			break
		}
		check = parent
	}
	return false
}

func unescapeMountInfoField(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	)
	return replacer.Replace(value)
}

func combineUpdateWarnings(warnings ...string) string {
	parts := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if trimmed := strings.TrimSpace(warning); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}
