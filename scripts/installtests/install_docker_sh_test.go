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

func previousStablePatchVersion(version string) (string, bool) {
	if isPrereleaseVersion(version) {
		return "", false
	}
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch <= 0 {
		return "", false
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch-1), true
}

func previousStableForPrereleaseVersion(version string) (string, bool) {
	if !isPrereleaseVersion(version) {
		return "", false
	}
	base, _, ok := strings.Cut(version, "-")
	if !ok {
		return "", false
	}
	return previousStablePatchVersion(base)
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

func TestInstallDockerProofTracksStablePatchReleaseContract(t *testing.T) {
	version := currentReleaseVersion(t)
	if isPrereleaseVersion(version) {
		t.Skip("current release is a prerelease")
	}
	previous, ok := previousStablePatchVersion(version)
	if !ok {
		t.Skip("current release is not a stable patch release")
	}

	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active stable `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		"This patch release uses the stable hotfix path with `rollback_version=v"+previous+"`, `hotfix_exception=true`, a release-owner reason, and no fabricated same-version RC tag.",
		"It advances the v"+previous+" stable line with customer-support fixes",
		"For the active stable `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"`",
	)
}

func TestInstallDockerProofTracksSupportPrereleaseContract(t *testing.T) {
	version := currentReleaseVersion(t)
	if !isPrereleaseVersion(version) {
		t.Skip("current release is stable")
	}
	previous, ok := previousStableForPrereleaseVersion(version)
	if !ok {
		t.Skip("current prerelease does not have a previous stable patch")
	}

	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active support prerelease `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		"This support prerelease keeps `rollback_version=v"+previous+"`, publishes a versioned public GitHub prerelease plus versioned Docker and Helm artifacts, and does not move stable/latest install pointers or stable semver aliases.",
		"legacy agent update token recovery, threshold-aware temperature display severity, PBS backup polling memory bounds, physical disk SMART/Proxmox merge identity, Proxmox token preservation diagnostics, legacy OIDC SSO discovery with CSP nonce handling, mobile onboarding URL handoff sanitization, SSO browser-session display labels, route-aware Proxmox host URLs, MSP report scheduling with a portal alert rollup, per-disk-type temperature thresholds, PBS backup discovery recovery, host fingerprint auto-discovery, and surfaced agent auth failures behind RC validation",
		"For the active support prerelease `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"` until the next governed stable cut moves them forward.",
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
