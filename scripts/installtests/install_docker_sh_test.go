package installtests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func currentReleaseVersion(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(repoFile("VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(content))
	if version == "" {
		t.Fatal("VERSION is empty")
	}
	return version
}

func requiredReleaseBranchForVersion(t *testing.T, version string) string {
	t.Helper()
	cmd := exec.Command("python3", repoFile("scripts", "release_control", "control_plane.py"), "--branch-for-version", version)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve release branch for %s: %v", version, err)
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		t.Fatalf("release branch for %s is empty", version)
	}
	return branch
}

func isPrereleaseVersion(version string) bool {
	return strings.Contains(version, "-")
}

// unpublishedStableVersions lists stable versions that never activated as a
// public GitHub release, whether or not a same-version tag or packet was
// staged. They are not valid rollback targets and must not be treated as the
// previous stable.
var unpublishedStableVersions = map[string]bool{
	// Tagged 2026-08-31; the private Pro build failed its memory gate and the
	// release commit verdict failed, so the packet shipped through v6.4.3 instead.
	"6.4.2": true,
	// v6.4.3 and v6.4.4 were only ever published as prereleases
	// (v6.4.3-rc.1, v6.4.4-beta.N). The v6.4.5 stable patch promotes the
	// exercised v6.4.5-rc.N candidate, so the last published stable remains
	// v6.4.1 and is the rollback target.
	"6.4.3": true,
	"6.4.4": true,
}

func previousStablePatchVersion(version string) (string, bool) {
	if isPrereleaseVersion(version) {
		return "", false
	}
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", false
	}
	for patch > 0 {
		patch--
		candidate := fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch)
		if !unpublishedStableVersions[candidate] {
			return candidate, true
		}
	}
	return "", false
}

// stablePatchPromotedPrerelease reports the same-version release candidate a
// stable patch promotes, if one exists. A stable patch with a same-version RC
// is RC-derived: the promotion resolver selects promotion_mode
// "stable-rc-promotion" and refuses the emergency no-RC hotfix path, so the
// install-metadata contract must assert that shape instead of the emergency
// one. Detection reads the published candidate notes so it stays offline and
// matches the release line's retained metadata.
func stablePatchPromotedPrerelease(docsReleasesDir, version string) (string, bool) {
	if isPrereleaseVersion(version) {
		return "", false
	}
	matches, err := filepath.Glob(filepath.Join(docsReleasesDir, "RELEASE_NOTES_v"+version+"-rc.*.md"))
	if err != nil {
		return "", false
	}
	best := ""
	bestNumber := -1
	for _, match := range matches {
		filename := filepath.Base(match)
		tag := strings.TrimSuffix(strings.TrimPrefix(filename, "RELEASE_NOTES_v"), ".md")
		marker := strings.LastIndex(tag, "-rc.")
		if marker < 0 {
			continue
		}
		number, err := strconv.Atoi(tag[marker+len("-rc."):])
		if err != nil {
			continue
		}
		if number > bestNumber {
			bestNumber = number
			best = "v" + tag
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

func currentStablePatchPromotedPrerelease(t *testing.T, version string) (string, bool) {
	t.Helper()
	return stablePatchPromotedPrerelease(repoFile("docs", "releases"), version)
}

func previousStableForPrereleaseVersion(version string) (string, bool) {
	if !isPrereleaseVersion(version) {
		return "", false
	}
	base, _, ok := strings.Cut(version, "-")
	if !ok {
		return "", false
	}
	baseParts, ok := parseStableVersion(base)
	if !ok {
		return "", false
	}

	releaseNotes, err := filepath.Glob(repoFile("docs", "releases", "RELEASE_NOTES_v*.md"))
	if err != nil {
		return "", false
	}

	best := [3]int{-1, -1, -1}
	found := false
	for _, releaseNote := range releaseNotes {
		filename := filepath.Base(releaseNote)
		candidate := strings.TrimSuffix(strings.TrimPrefix(filename, "RELEASE_NOTES_v"), ".md")
		parts, valid := parseStableVersion(candidate)
		if !valid || compareStableVersions(parts, baseParts) >= 0 || unpublishedStableVersions[candidate] {
			continue
		}
		if !found || compareStableVersions(parts, best) > 0 {
			best = parts
			found = true
		}
	}
	if !found {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d", best[0], best[1], best[2]), true
}

func parseStableVersion(version string) ([3]int, bool) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	parsed := [3]int{}
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return [3]int{}, false
		}
		parsed[index] = value
	}
	return parsed, true
}

func compareStableVersions(left, right [3]int) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func TestPreviousStableForPrereleaseVersionCrossesMinorBoundaries(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{version: "6.0.5-rc.4", want: "6.0.4"},
		{version: "6.2.0-rc.4", want: "6.1.2"},
		{version: "6.2.0-rc.5", want: "6.1.2"},
		{version: "6.2.0-rc.6", want: "6.1.2"},
		{version: "6.2.0-rc.7", want: "6.1.2"},
		{version: "6.2.0-rc.8", want: "6.1.2"},
		{version: "6.2.0-rc.9", want: "6.1.2"},
		{version: "6.2.0-rc.10", want: "6.1.2"},
		{version: "6.2.0-rc.11", want: "6.1.2"},
		{version: "6.2.2-rc.1", want: "6.2.1"},
		{version: "6.2.2-rc.2", want: "6.2.1"},
		{version: "6.2.2-rc.3", want: "6.2.1"},
		{version: "6.3.0-rc.1", want: "6.2.1"},
		{version: "6.3.0-rc.2", want: "6.2.1"},
		{version: "6.4.0-rc.1", want: "6.3.2"},
		{version: "6.4.0-rc.2", want: "6.3.2"},
		{version: "6.4.0-rc.3", want: "6.3.2"},
		{version: "6.4.0-rc.4", want: "6.3.2"},
		{version: "6.4.0-rc.5", want: "6.3.2"},
		{version: "6.4.0-rc.6", want: "6.3.2"},
		{version: "6.4.0-rc.7", want: "6.3.2"},
		{version: "6.4.0-rc.8", want: "6.3.2"},
		{version: "6.4.0-rc.9", want: "6.3.2"},
		{version: "6.4.0-rc.10", want: "6.3.2"},
		{version: "6.4.0-rc.11", want: "6.3.2"},
		{version: "6.4.0-rc.12", want: "6.3.2"},
		{version: "6.4.0-rc.13", want: "6.3.2"},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			got, ok := previousStableForPrereleaseVersion(test.version)
			if !ok {
				t.Fatalf("previousStableForPrereleaseVersion(%q) did not find a stable release", test.version)
			}
			if got != test.want {
				t.Fatalf("previousStableForPrereleaseVersion(%q) = %q, want %q", test.version, got, test.want)
			}
		})
	}
}

func TestPreviousStablePatchVersionSkipsUnpublishedStables(t *testing.T) {
	tests := []struct {
		version string
		want    string
		ok      bool
	}{
		{version: "6.4.1", want: "6.4.0", ok: true},
		{version: "6.4.2", want: "6.4.1", ok: true},
		// v6.4.2 was tagged but never published; v6.4.3 and v6.4.4 only ever
		// shipped as prereleases, so v6.4.5 rolls back to the published v6.4.1.
		{version: "6.4.5", want: "6.4.1", ok: true},
		{version: "6.4.0", ok: false},
		{version: "6.4.5-rc.2", ok: false},
	}
	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			got, ok := previousStablePatchVersion(test.version)
			if ok != test.ok || got != test.want {
				t.Fatalf("previousStablePatchVersion(%q) = %q, %v; want %q, %v", test.version, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestStablePatchPromotedPrereleaseDetectsSameVersionCandidate(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"RELEASE_NOTES_v6.4.5-rc.1.md",
		"RELEASE_NOTES_v6.4.5-rc.2.md",
		"RELEASE_NOTES_v6.4.4-beta.4.md",
		"RELEASE_NOTES_v6.4.5-beta.1.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if got, ok := stablePatchPromotedPrerelease(dir, "6.4.5"); !ok || got != "v6.4.5-rc.2" {
		t.Fatalf("stablePatchPromotedPrerelease(6.4.5) = %q, %v; want v6.4.5-rc.2, true", got, ok)
	}
	if got, ok := stablePatchPromotedPrerelease(dir, "6.4.6"); ok {
		t.Fatalf("stablePatchPromotedPrerelease(6.4.6) = %q, true; want no same-version candidate", got)
	}
	if got, ok := stablePatchPromotedPrerelease(dir, "6.4.5-rc.2"); ok {
		t.Fatalf("stablePatchPromotedPrerelease(6.4.5-rc.2) = %q, true; want prerelease ignored", got)
	}
}

func previousPrereleaseVersion(version string) (string, bool) {
	base, suffix, ok := strings.Cut(version, "-rc.")
	if !ok {
		return "", false
	}
	rc, err := strconv.Atoi(suffix)
	if err != nil || rc <= 1 {
		return "", false
	}
	return fmt.Sprintf("%s-rc.%d", base, rc-1), true
}

func TestInstallDockerScriptUsesConfiguredImageRepoDefault(t *testing.T) {
	workDir := t.TempDir()
	version := currentReleaseVersion(t)
	runInstallDockerScript(t, workDir, "DOCKER_IMAGE_REPO=example/pulse-enterprise")

	composePath := filepath.Join(workDir, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	composeText := string(composeContent)
	if !strings.Contains(composeText, "image: ${PULSE_IMAGE:-example/pulse-enterprise:"+version+"}") {
		t.Fatalf("docker-compose.yml missing configured image default:\n%s", composeText)
	}
	if strings.Contains(composeText, ":latest") {
		t.Fatalf("docker-compose.yml must not default to a floating latest tag:\n%s", composeText)
	}

	envPath := filepath.Join(workDir, ".env")
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	envText := string(envContent)
	if !strings.Contains(envText, "PULSE_IMAGE=example/pulse-enterprise:"+version) {
		t.Fatalf(".env missing configured image default:\n%s", envText)
	}
}

func TestInstallDockerScriptPrefersExplicitPulseImage(t *testing.T) {
	workDir := t.TempDir()
	version := currentReleaseVersion(t)
	runInstallDockerScript(
		t,
		workDir,
		"DOCKER_IMAGE_REPO=example/pulse-enterprise",
		"PULSE_IMAGE=ghcr.io/example/pulse-enterprise:v9.9.9",
	)

	composePath := filepath.Join(workDir, "docker-compose.yml")
	composeContent, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	composeText := string(composeContent)
	if !strings.Contains(composeText, "image: ${PULSE_IMAGE:-example/pulse-enterprise:"+version+"}") {
		t.Fatalf("docker-compose.yml lost configured default image:\n%s", composeText)
	}

	envPath := filepath.Join(workDir, ".env")
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	envText := string(envContent)
	if !strings.Contains(envText, "PULSE_IMAGE=ghcr.io/example/pulse-enterprise:v9.9.9") {
		t.Fatalf(".env did not preserve explicit image override:\n%s", envText)
	}
}

func TestRepoDockerComposeDefaultPinsCurrentVersion(t *testing.T) {
	version := currentReleaseVersion(t)
	content, err := os.ReadFile(repoFile("docker-compose.yml"))
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}") {
		t.Fatalf("repo docker-compose.yml must pin the current release version:\n%s", text)
	}
	if !isPrereleaseVersion(version) && strings.Contains(text, "-rc.") {
		t.Fatalf("stable repo docker-compose.yml must not keep a prerelease image default:\n%s", text)
	}
	if !isPrereleaseVersion(version) && version == "6.0.0" && !strings.Contains(text, "rcourtman/pulse:6.0.0") {
		t.Fatalf("v6 GA repo docker-compose.yml must default to the stable v6 image:\n%s", text)
	}
	if !isPrereleaseVersion(version) && version != "6.0.0" && strings.Contains(text, "rcourtman/pulse:6.0.0") {
		t.Fatalf("stable patch repo docker-compose.yml must move off the initial GA image tag:\n%s", text)
	}
	if previous, ok := previousStablePatchVersion(version); ok && strings.Contains(text, "rcourtman/pulse:"+previous) {
		t.Fatalf("repo docker-compose.yml must not retain the previous stable patch image tag %s:\n%s", previous, text)
	}
	if previous, ok := previousPrereleaseVersion(version); ok && strings.Contains(text, "rcourtman/pulse:"+previous) {
		t.Fatalf("repo docker-compose.yml must not retain the previous prerelease image tag %s:\n%s", previous, text)
	}
	if strings.Contains(text, ":latest") {
		t.Fatalf("repo docker-compose.yml must not default to a floating latest tag:\n%s", text)
	}
}

func TestInstallDockerScriptFallbackPinsCurrentVersion(t *testing.T) {
	version := currentReleaseVersion(t)
	content, err := os.ReadFile(repoFile("scripts", "install-docker.sh"))
	if err != nil {
		t.Fatalf("read install-docker.sh: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, `CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`) {
		t.Fatalf("install-docker.sh fallback must pin the current release version:\n%s", text)
	}
	if !isPrereleaseVersion(version) && strings.Contains(text, `CANONICAL_DEFAULT_PULSE_VERSION="`) && strings.Contains(text, "-rc.") {
		t.Fatalf("stable install-docker.sh fallback must not keep a prerelease default:\n%s", text)
	}
	if !isPrereleaseVersion(version) && version == "6.0.0" && !strings.Contains(text, `CANONICAL_DEFAULT_PULSE_VERSION="6.0.0"`) {
		t.Fatalf("v6 GA install-docker.sh fallback must default to the stable v6 image tag:\n%s", text)
	}
	if !isPrereleaseVersion(version) && version != "6.0.0" && strings.Contains(text, `CANONICAL_DEFAULT_PULSE_VERSION="6.0.0"`) {
		t.Fatalf("stable patch install-docker.sh fallback must move off the initial GA image tag:\n%s", text)
	}
	if previous, ok := previousStablePatchVersion(version); ok && strings.Contains(text, `CANONICAL_DEFAULT_PULSE_VERSION="`+previous+`"`) {
		t.Fatalf("install-docker.sh fallback must not retain the previous stable patch version %s:\n%s", previous, text)
	}
	if previous, ok := previousPrereleaseVersion(version); ok && strings.Contains(text, `CANONICAL_DEFAULT_PULSE_VERSION="`+previous+`"`) {
		t.Fatalf("install-docker.sh fallback must not retain the previous prerelease version %s:\n%s", previous, text)
	}
}

func TestInstallDockerProofTracksStableMinorContract(t *testing.T) {
	version := currentReleaseVersion(t)
	if isPrereleaseVersion(version) {
		t.Skip("current release is a prerelease")
	}
	parts, valid := parseStableVersion(version)
	if !valid || parts[1] == 0 || parts[2] != 0 {
		t.Skip("current release is not a stable minor release")
	}
	if _, ok := previousStableForPrereleaseVersion(version + "-rc.1"); !ok {
		t.Fatal("stable minor release has no earlier stable rollback packet")
	}

	assertFileContainsAll(t, repoFile("docker-compose.yml"),
		"image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}",
	)
	assertFileContainsAll(t, repoFile("scripts", "install-docker.sh"),
		`CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`,
	)
}

func runInstallDockerScript(t *testing.T, workDir string, envVars ...string) {
	t.Helper()

	scriptPath := repoFile("scripts", "install-docker.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install-docker.sh: %v", err)
	}

	script := string(content)
	script = strings.Replace(script, rootCheckBlock, ":", 1)
	script = strings.Replace(script, containerCheckBlock, ":", 1)

	tmpScript := filepath.Join(workDir, "install-docker.sh")
	if err := os.WriteFile(tmpScript, []byte(script), 0o755); err != nil {
		t.Fatalf("write temp install-docker.sh: %v", err)
	}

	binDir := filepath.Join(workDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeTestStub(t, filepath.Join(binDir, "docker"), "#!/bin/sh\nif [ \"$1\" = \"compose\" ] && [ \"$2\" = \"version\" ]; then exit 0; fi\nexit 0\n")
	writeTestStub(t, filepath.Join(binDir, "timedatectl"), "#!/bin/sh\necho Europe/London\n")
	writeTestStub(t, filepath.Join(binDir, "hostname"), "#!/bin/sh\nif [ \"$1\" = \"-I\" ]; then echo 192.0.2.10; else echo pulse-host; fi\n")

	cmd := exec.Command("bash", tmpScript)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), append([]string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}, envVars...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run install-docker.sh: %v\n%s", err, out)
	}
}

func writeTestStub(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}

const rootCheckBlock = `# Check if running as root (early check for better error messages)
if [ "$EUID" -ne 0 ]; then
    echo "❌ ERROR: This script must be run as root"
    echo ""
    echo "Please run: sudo $0"
    exit 1
fi
`

const containerCheckBlock = `# Detect if running in a container
if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
    echo "❌ ERROR: This script must run on the Docker host, not inside a container"
    echo ""
    echo "Please run this script on your Docker host machine."
    exit 1
fi
`
