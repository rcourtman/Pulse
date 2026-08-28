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
		if !valid || compareStableVersions(parts, baseParts) >= 0 {
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
		"active customer harm",
		"`no-mobile-impact`",
		"For the active stable `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"`",
	)
	if version == "6.3.1" {
		assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
			"the prior `v"+previous+"` decision could not be reused for this patch",
			"the release owner recorded that separate `v6.3.1` exception",
			"public Unknown Publisher disclosure",
		)
	} else {
		assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
			"Windows Authenticode remains mandatory for `v"+version+"`",
		)
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
	previous, ok := previousStableForPrereleaseVersion(version + "-rc.1")
	if !ok {
		t.Fatal("stable minor release has no earlier stable rollback packet")
	}

	assertFileContainsAll(t, repoFile("docker-compose.yml"),
		"image: ${PULSE_IMAGE:-rcourtman/pulse:"+version+"}",
	)
	assertFileContainsAll(t, repoFile("scripts", "install-docker.sh"),
		`CANONICAL_DEFAULT_PULSE_VERSION="`+version+`"`,
	)
	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active stable `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		"`rollback_version=v"+previous+"`",
		"`hotfix_exception=true` transports that approved waiver through the shared promotion resolver; it does not reclassify v"+version+" as a patch hotfix.",
		"The release owner separately approved a v"+version+"-only unsigned-Windows exception",
		"The Windows packet must disclose the Unknown Publisher warning and retain exact-SHA, checksum, detached-signature, immutable-manifest, and published-digest verification.",
		"For the active stable `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"`",
	)
}

func TestInstallDockerProofTracksPrereleaseContract(t *testing.T) {
	version := currentReleaseVersion(t)
	if !isPrereleaseVersion(version) {
		t.Skip("current release is stable")
	}
	previous, ok := previousStableForPrereleaseVersion(version)
	if !ok {
		t.Skip("current prerelease does not have a previous stable patch")
	}
	stableTarget, _, ok := strings.Cut(version, "-")
	if !ok {
		t.Fatalf("current prerelease %q has no stable target", version)
	}
	comparisonVersion, ok := previousPrereleaseVersion(version)
	comparisonLine := "It follows stable `v" + previous + "` and opens the published `v" + stableTarget + "` candidate line."
	if !ok {
		comparisonVersion = previous
	} else {
		comparisonLine = "It follows `v" + comparisonVersion + "` on the published `v" + stableTarget + "` candidate line."
	}
	if version == "6.4.0-rc.10" {
		comparisonLine = "The `v6.4.0-rc.9` release staged an immutable draft, tag, and exact-version artifacts but did not activate publicly"
	}

	assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
		"The active prerelease `v"+version+"` cut sets the repo-root `VERSION`, repo-root `docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and Helm chart release metadata to the same `"+version+"` release version.",
		comparisonLine,
		"This prerelease keeps `rollback_version=v"+previous+"`, publishes a versioned public GitHub prerelease plus versioned Docker and Helm artifacts, and does not move stable/latest install pointers or stable semver aliases.",
		"add the canonical `alert_fired` mobile push type, but preserve the existing `view_alert` navigation action and all route, request/response, pairing, and authorization contracts.",
		"Published Pulse Mobile iOS build 12 and Android versionCode 9 already route `action_type=view_alert`, so the server cut is classified `existing-mobile-build-compatible`; no companion upload or public mobile-store rollout is part of this candidate.",
		"The prerelease Windows path retains exact-SHA, checksum, and detached-signature verification without Authenticode. Stable `v"+stableTarget+"` also skips SignPath under the standing unavailable policy",
		"For the active prerelease `v"+version+"` cut, the repo-root compose default and `scripts/install-docker.sh` fallback must both pin `"+version+"` until the next governed stable cut moves them forward.",
	)
	if version == "6.3.0-rc.6" {
		assertFileContainsAllNormalized(t, repoFile("docs", "release-control", "v6", "internal", "subsystems", "deployment-installability.md"),
			"For `v6.3.0-rc.6`, the release path retains credential-free PVE compilation while requiring the measured memory floor for API race shards after a bounded admission wait.",
			"Public server and provider control-plane image publication uses independent matrix jobs that each revalidate the exact checkout and candidate manifest.",
			"Private packaging transfers only the products consumed by Pro assembly, and paid-runtime Docker and direct-binary mismatch proofs execute concurrently without weakening either proof.",
		)
	}
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
