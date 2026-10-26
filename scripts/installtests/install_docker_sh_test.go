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

// previousPrereleaseVersion names the predecessor of a numbered pre-release in its own
// stage, as the release metadata tool does: rc.2 follows rc.1 and beta.2 follows beta.1.
// The first of a stage has none, even when an earlier stage shipped.
func previousPrereleaseVersion(version string) (string, bool) {
	for _, stage := range []string{"alpha", "beta", "rc"} {
		base, suffix, ok := strings.Cut(version, "-"+stage+".")
		if !ok {
			continue
		}
		number, err := strconv.Atoi(suffix)
		if err != nil || number <= 1 {
			return "", false
		}
		return fmt.Sprintf("%s-%s.%d", base, stage, number-1), true
	}
	return "", false
}

// This predecessor is the metadata tool's default comparison base. A cumulative
// changelog can explicitly compare with the previous stable instead.
func TestPreviousPrereleaseVersionFollowsItsOwnStage(t *testing.T) {
	for _, tc := range []struct {
		version, want string
		ok            bool
	}{
		{"6.6.0-rc.2", "6.6.0-rc.1", true},
		{"6.6.0-rc.12", "6.6.0-rc.11", true},
		{"6.6.0-beta.2", "6.6.0-beta.1", true},
		{"6.6.0-beta.7", "6.6.0-beta.6", true},
		{"6.6.0-alpha.3", "6.6.0-alpha.2", true},
		{"6.6.0-beta.1", "", false},
		{"6.6.0-rc.1", "", false},
		{"6.6.0-alpha.1", "", false},
		{"6.6.0-beta.x", "", false},
		{"6.6.0", "", false},
	} {
		got, ok := previousPrereleaseVersion(tc.version)
		if got != tc.want || ok != tc.ok {
			t.Errorf("previousPrereleaseVersion(%q) = %q, %v; want %q, %v", tc.version, got, ok, tc.want, tc.ok)
		}
	}
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
	out, err := runInstallDockerScriptResult(t, workDir, envVars...)
	if err != nil {
		t.Fatalf("run install-docker.sh: %v\n%s", err, out)
	}
}

func runInstallDockerScriptResult(t *testing.T, workDir string, envVars ...string) (string, error) {
	t.Helper()

	scriptPath := repoFile("scripts", "install-docker.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install-docker.sh: %v", err)
	}

	script := string(content)
	// Only the two fixed host-admission checks are replaced. The generator,
	// path admission, writes and messages execute unchanged in a private fixture.
	if strings.Count(script, rootCheckBlock) != 1 || strings.Count(script, containerCheckBlock) != 1 {
		t.Fatal("installer host-admission boundaries changed; update the explicit test fixture")
	}
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
	writeTestStub(t, filepath.Join(binDir, "docker"), `#!/bin/sh
printf '%s\n' "$*" >> docker-calls
if [ "$*" != 'compose version' ]; then exit 91; fi
if [ "${TEST_RACE_COMPOSE:-}" = true ]; then
    printf '%s\n' 'racing owner configuration' > docker-compose.yml
fi
exit 0
`)
	writeTestStub(t, filepath.Join(binDir, "timedatectl"), "#!/bin/sh\necho Europe/London\n")
	writeTestStub(t, filepath.Join(binDir, "hostname"), "#!/bin/sh\nif [ \"$1\" = \"-I\" ]; then echo 192.0.2.10; else echo pulse-host; fi\n")

	cmd := exec.Command("bash", tmpScript)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), append([]string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}, envVars...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
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

func assertDockerSetupBytes(t *testing.T, path, want string, mode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("configuration bytes changed at %s (read error: %v)", filepath.Base(path), err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != mode {
		t.Fatalf("configuration mode changed at %s (stat error: %v)", filepath.Base(path), err)
	}
}

func TestInstallDockerScriptKeepsExistingDeployment(t *testing.T) {
	workDir := t.TempDir()
	runInstallDockerScript(t, workDir)
	compose := filepath.Join(workDir, "docker-compose.yml")
	env := filepath.Join(workDir, ".env")
	backup := compose + ".backup"
	// Model a real operator's bind mounts, image pin, port and private settings.
	custom := "services:\n  pulse:\n    image: example/pulse:fixed\n    ports: ['127.0.0.1:8765:7655']\n    volumes: ['./existing-state:/data']\n"
	private := "PULSE_IMAGE=example/pulse:fixed\nPULSE_PORT=8765\nSYNTHETIC_SECRET=fixture-only\n"
	for path, value := range map[string]string{compose: custom, env: private, backup: "older recovery copy\n"} {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(compose)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(workDir, "docker-calls")); err != nil {
		t.Fatal(err)
	}
	out, resultErr := runInstallDockerScriptResult(t, workDir)
	assertDockerSetupBytes(t, compose, custom, 0o600)
	assertDockerSetupBytes(t, env, private, 0o600)
	assertDockerSetupBytes(t, backup, "older recovery copy\n", 0o600)
	after, err := os.Stat(compose)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("existing Compose inode was replaced")
	}
	if resultErr == nil {
		t.Fatal("repeated setup accepted an existing deployment")
	}
	for _, message := range []string{"only prepares a new deployment", "existing Compose configuration", "left unchanged"} {
		if !strings.Contains(out, message) {
			t.Errorf("missing actionable preservation message %q", message)
		}
	}
	if strings.Contains(out, "SYNTHETIC_SECRET") || strings.Contains(out, "fixture-only") || strings.Contains(out, "Installation Complete") {
		t.Fatal("refusal leaked configuration or claimed installation")
	}
	if _, err := os.Stat(filepath.Join(workDir, "docker-calls")); !os.IsNotExist(err) {
		t.Fatal("refused setup called Docker")
	}
}

func TestInstallDockerScriptRefusesAllExistingComposeNames(t *testing.T) {
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		t.Run(name, func(t *testing.T) {
			workDir := t.TempDir()
			path := filepath.Join(workDir, name)
			if err := os.WriteFile(path, []byte("existing operator configuration\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			if _, err := runInstallDockerScriptResult(t, workDir); err == nil {
				t.Fatal("setup accepted an existing Compose file")
			}
			assertDockerSetupBytes(t, path, "existing operator configuration\n", 0o640)
			for _, output := range []string{".env", "docker-calls", "docker-compose.yml.backup"} {
				if _, err := os.Lstat(filepath.Join(workDir, output)); !os.IsNotExist(err) {
					t.Fatalf("refused setup created %s", output)
				}
			}
			if name != "docker-compose.yml" {
				if _, err := os.Lstat(filepath.Join(workDir, "docker-compose.yml")); !os.IsNotExist(err) {
					t.Fatal("setup created a competing Compose file")
				}
			}
		})
	}
}

func TestInstallDockerScriptRefusesUnsafeOutputPaths(t *testing.T) {
	for _, output := range []string{"docker-compose.yml", ".env"} {
		for _, shape := range []string{"symlink", "dangling-symlink", "directory", "fifo"} {
			t.Run(output+"/"+shape, func(t *testing.T) {
				workDir := t.TempDir()
				target := filepath.Join(workDir, "untouched-target")
				path := filepath.Join(workDir, output)
				if shape != "dangling-symlink" {
					if err := os.WriteFile(target, []byte("unrelated private bytes\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				switch shape {
				case "symlink", "dangling-symlink":
					if err := os.Symlink(target, path); err != nil {
						t.Skipf("symlink fixture unavailable: %v", err)
					}
				case "directory":
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				case "fifo":
					if _, err := exec.LookPath("mkfifo"); err != nil {
						t.Skip("mkfifo fixture unavailable")
					}
					if err := exec.Command("mkfifo", path).Run(); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runInstallDockerScriptResult(t, workDir); err == nil {
					t.Fatal("setup accepted an unsafe output path")
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() {
					t.Fatal("unsafe output path changed")
				}
				if shape == "dangling-symlink" {
					if _, err := os.Lstat(target); !os.IsNotExist(err) {
						t.Fatal("setup wrote through a dangling symlink")
					}
				} else {
					assertDockerSetupBytes(t, target, "unrelated private bytes\n", 0o600)
				}
				for _, absent := range []string{"docker-calls", "docker-compose.yml.backup"} {
					if _, err := os.Lstat(filepath.Join(workDir, absent)); !os.IsNotExist(err) {
						t.Fatalf("refused setup created %s", absent)
					}
				}
				other := ".env"
				if output == ".env" {
					other = "docker-compose.yml"
				}
				if _, err := os.Lstat(filepath.Join(workDir, other)); !os.IsNotExist(err) {
					t.Fatal("unsafe path admission partially generated configuration")
				}
			})
		}
	}
}

func TestInstallDockerScriptPreservesRegularEnvironment(t *testing.T) {
	workDir := t.TempDir()
	env := filepath.Join(workDir, ".env")
	if err := os.WriteFile(env, []byte("PULSE_IMAGE=example/pulse:operator-pin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runInstallDockerScript(t, workDir)
	assertDockerSetupBytes(t, env, "PULSE_IMAGE=example/pulse:operator-pin\n", 0o600)
	info, err := os.Stat(filepath.Join(workDir, "docker-compose.yml"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("new Compose output is not private")
	}
}

func TestInstallDockerScriptDoesNotClobberRacingCompose(t *testing.T) {
	workDir := t.TempDir()
	out, err := runInstallDockerScriptResult(t, workDir, "TEST_RACE_COMPOSE=true")
	if err == nil || strings.Contains(out, "Configuration prepared") {
		t.Fatal("setup claimed success over a competing output")
	}
	assertDockerSetupBytes(t, filepath.Join(workDir, "docker-compose.yml"), "racing owner configuration\n", 0o600)
	if _, err := os.Lstat(filepath.Join(workDir, "docker-compose.yml.backup")); !os.IsNotExist(err) {
		t.Fatal("racing setup replaced a backup")
	}
	if _, err := os.Lstat(filepath.Join(workDir, ".env")); !os.IsNotExist(err) {
		t.Fatal("racing setup continued after the Compose collision")
	}
}

func TestInstallDockerScriptReportsPreparationNotInstallation(t *testing.T) {
	workDir := t.TempDir()
	out, err := runInstallDockerScriptResult(t, workDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Configuration prepared; Pulse has not been started") || strings.Contains(out, "Installation Complete") {
		t.Fatal("generator claimed an unexecuted installation")
	}
	for _, output := range []string{"docker-compose.yml", ".env"} {
		info, err := os.Stat(filepath.Join(workDir, output))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("new %s output is not private", output)
		}
	}
	calls, err := os.ReadFile(filepath.Join(workDir, "docker-calls"))
	if err != nil || string(calls) != "compose version\n" {
		t.Fatal("generator called a Docker operation other than compose version")
	}
}
