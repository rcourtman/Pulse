package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsPrereleaseTagRecognizesPrereleases asserts that any semver tag with
// a hyphen after the patch component is flagged as a prerelease. This is the
// pattern used for Pulse RCs (`-rc.N`), betas (`-beta.N`), alphas, nightlies,
// etc. The unattended updater must refuse these on the stable channel.
func TestIsPrereleaseTagRecognizesPrereleases(t *testing.T) {
	cases := []string{
		"v6.0.0-rc.2",
		"v5.1.28-rc.3",
		"v5.1.28-beta.1",
		"v5.1.28-alpha.2",
		"v5.1.28-nightly",
		"v5.1.28-pre.1",
		"6.0.0-rc.2",
		"5.1.28-beta",
	}

	for _, tag := range cases {
		tag := tag
		t.Run(tag, func(t *testing.T) {
			script := extractAutoUpdateFunction(t, "is_prerelease_tag") + `
is_prerelease_tag "` + tag + `"
echo $?
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != "0" {
				t.Fatalf("is_prerelease_tag %q returned %q, want 0 (is prerelease)", tag, got)
			}
		})
	}
}

// TestIsPrereleaseTagAcceptsStableTags asserts that plain MAJOR.MINOR.PATCH
// tags - the only thing the unattended stable updater should ever accept -
// are not flagged.
func TestIsPrereleaseTagAcceptsStableTags(t *testing.T) {
	cases := []string{
		"v5.1.28",
		"v5.1.27",
		"v6.0.0",
		"v6.1.0",
		"5.1.28",
		"6.0.0",
	}

	for _, tag := range cases {
		tag := tag
		t.Run(tag, func(t *testing.T) {
			script := extractAutoUpdateFunction(t, "is_prerelease_tag") + `
is_prerelease_tag "` + tag + `"
echo $?
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != "1" {
				t.Fatalf("is_prerelease_tag %q returned %q, want 1 (stable)", tag, got)
			}
		})
	}
}

// TestIsPrereleaseTagFailsClosed asserts that empty or malformed input is
// treated as a prerelease - callers should refuse to act on it rather than
// proceed with an unrecognized tag.
func TestIsPrereleaseTagFailsClosed(t *testing.T) {
	cases := []string{
		"",
		"latest",
		"v5",
		"v5.1",
		"garbage",
		"main",
	}

	for _, tag := range cases {
		tag := tag
		name := tag
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			script := extractAutoUpdateFunction(t, "is_prerelease_tag") + `
is_prerelease_tag "` + tag + `"
echo $?
`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != "0" {
				t.Fatalf("is_prerelease_tag %q returned %q, want 0 (fail closed)", tag, got)
			}
		})
	}
}

// TestPerformUpdateRefusesPrereleaseTag asserts the defense-in-depth guard
// at the perform_update entry: even if every caller above this point thinks
// the tag is safe, perform_update itself refuses prerelease-shaped tags
// before touching the installer. This is the last line of defense against
// the 2026-04-16 incident recurring.
func TestPerformUpdateRefusesPrereleaseTag(t *testing.T) {
	script := `
set -u
GITHUB_REPO="rcourtman/Pulse"
INSTALL_DIR="/tmp/pulse-nonexistent-test-install"
log() { echo "[$1] $2"; }
detect_service_name() { echo pulse; }
get_current_version() { echo v5.1.27; }
systemctl() { return 0; }
# If any of these get called, the guard has failed.
curl() { echo "FAIL: curl invoked during refused update"; exit 99; }
mktemp() { echo "FAIL: mktemp invoked during refused update"; exit 99; }
verify_release_signature() { echo "FAIL: signature verify invoked"; exit 99; }
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
` + extractAutoUpdateFunction(t, "resolve_install_script_url") + `
` + extractAutoUpdateFunction(t, "perform_update") + `
if perform_update v6.0.0-rc.2; then
  echo "ACCEPTED"
else
  echo "REFUSED"
fi
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "REFUSED") {
		t.Fatalf("perform_update did not refuse prerelease tag:\n%s", got)
	}
	if strings.Contains(got, "FAIL:") {
		t.Fatalf("perform_update invoked installer machinery despite refusal:\n%s", got)
	}
	if !strings.Contains(got, "Refusing to install prerelease") {
		t.Fatalf("perform_update did not log refusal reason:\n%s", got)
	}
}

// TestGetLatestStableVersionRefusesPrereleaseFlag asserts that even if the
// tag name looks stable, a `"prerelease": true` flag in the API response
// causes the function to return empty. This was the suspected trigger for
// the 2026-04-16 jump from 5.1.27 to 6.0.0-rc.2.
func TestGetLatestStableVersionRefusesPrereleaseFlag(t *testing.T) {
	script := `
set -u
GITHUB_REPO="rcourtman/Pulse"
log() { echo "[$1] $2" >&2; }
# Stub curl: respond with JSON where prerelease=true.
curl() {
  # Drain flags; we don't care what was requested.
  cat <<'EOF'
{
  "tag_name": "v5.1.28",
  "prerelease": true,
  "name": "Pulse v5.1.28"
}
EOF
}
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
` + extractAutoUpdateFunction(t, "version_greater_than") + `
` + extractAutoUpdateFunction(t, "pick_highest_stable_tag") + `
` + extractAutoUpdateFunction(t, "get_latest_stable_version") + `
result=$(get_latest_stable_version)
echo "RESULT=[$result]"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "RESULT=[]") {
		t.Fatalf("get_latest_stable_version returned a tag despite prerelease=true:\n%s", got)
	}
}

// TestGetLatestStableVersionRefusesPrereleaseShapedTag asserts the shape
// check: if the API says `prerelease=false` but the tag itself is clearly a
// prerelease (e.g. during a brief window where the flag was miswritten),
// the function still refuses.
func TestGetLatestStableVersionRefusesPrereleaseShapedTag(t *testing.T) {
	script := `
set -u
GITHUB_REPO="rcourtman/Pulse"
log() { echo "[$1] $2" >&2; }
curl() {
  cat <<'EOF'
{
  "tag_name": "v6.0.0-rc.2",
  "prerelease": false,
  "name": "Pulse v6.0.0-rc.2"
}
EOF
}
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
` + extractAutoUpdateFunction(t, "version_greater_than") + `
` + extractAutoUpdateFunction(t, "pick_highest_stable_tag") + `
` + extractAutoUpdateFunction(t, "get_latest_stable_version") + `
result=$(get_latest_stable_version)
echo "RESULT=[$result]"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "RESULT=[]") {
		t.Fatalf("get_latest_stable_version returned prerelease-shaped tag v6.0.0-rc.2 despite flag=false:\n%s", got)
	}
}

// TestGetLatestStableVersionAcceptsStable asserts the happy path: a stable
// tag with prerelease=false flows through unchanged.
func TestGetLatestStableVersionAcceptsStable(t *testing.T) {
	script := `
set -u
GITHUB_REPO="rcourtman/Pulse"
log() { echo "[$1] $2" >&2; }
curl() {
  cat <<'EOF'
{
  "tag_name": "v5.1.28",
  "prerelease": false,
  "name": "Pulse v5.1.28"
}
EOF
}
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
` + extractAutoUpdateFunction(t, "version_greater_than") + `
` + extractAutoUpdateFunction(t, "pick_highest_stable_tag") + `
` + extractAutoUpdateFunction(t, "get_latest_stable_version") + `
result=$(get_latest_stable_version)
echo "RESULT=[$result]"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "RESULT=[v5.1.28]") {
		t.Fatalf("get_latest_stable_version did not return expected stable tag:\n%s", got)
	}
}

// TestGetLatestStableVersionPrefersHighestVersion asserts that the updater
// selects the highest stable version from the release list rather than the
// most recently created one. This repo interleaves v5-line maintenance
// releases with v6 releases (v5.1.36 was created the day before v6.0.5), so
// GitHub's created_at ordering — and its /releases/latest pointer — can name
// an older version line, which would strand v6 installs until the next v6
// release ships. Draft releases, metadata-flagged prereleases, and
// prerelease- or non-semver-shaped tags (helm-chart-*) must all be ignored.
func TestGetLatestStableVersionPrefersHighestVersion(t *testing.T) {
	script := `
set -u
GITHUB_REPO="rcourtman/Pulse"
log() { echo "[$1] $2" >&2; }
# Stub curl: the /releases list in GitHub's created_at ordering, with the
# v5-line release first. Any fallback call (e.g. /releases/latest) would
# return the same list and must not be needed for selection.
curl() {
  cat <<'EOF'
[
  {
    "tag_name": "v5.1.37",
    "draft": false,
    "prerelease": false
  },
  {
    "tag_name": "v9.9.9",
    "draft": true,
    "prerelease": false
  },
  {
    "tag_name": "helm-chart-6.0.5",
    "draft": false,
    "prerelease": true
  },
  {
    "tag_name": "v6.0.5",
    "draft": false,
    "prerelease": false
  },
  {
    "tag_name": "v6.0.5-rc.4",
    "draft": false,
    "prerelease": true
  },
  {
    "tag_name": "v6.0.4",
    "draft": false,
    "prerelease": false
  }
]
EOF
}
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
` + extractAutoUpdateFunction(t, "version_greater_than") + `
` + extractAutoUpdateFunction(t, "pick_highest_stable_tag") + `
` + extractAutoUpdateFunction(t, "get_latest_stable_version") + `
result=$(get_latest_stable_version)
echo "RESULT=[$result]"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "RESULT=[v6.0.5]") {
		t.Fatalf("get_latest_stable_version = %s, want v6.0.5 (highest stable, not most recently created)", got)
	}
}

// TestInstalledBinaryIsPulseProGuard proves the unattended updater's Pro
// guard: a binary whose --version reports "Pulse Pro" is detected (so main()
// exits before any community download can replace it), while the community
// "Pulse vX" binary is not flagged. The community auto-update flow must never
// reinstall the public build over a paid Pro runtime.
func TestInstalledBinaryIsPulseProGuard(t *testing.T) {
	fn := extractAutoUpdateFunction(t, "installed_binary_is_pulse_pro")

	cases := []struct {
		name        string
		versionLine string
		wantPro     bool
	}{
		{"pro binary detected", "Pulse Pro v6.0.5", true},
		{"community binary not flagged", "Pulse v6.0.5", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			installDir := t.TempDir()
			binDir := filepath.Join(installDir, "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatalf("mkdir bin: %v", err)
			}
			stub := "#!/bin/bash\necho \"" + tc.versionLine + "\"\n"
			if err := os.WriteFile(filepath.Join(binDir, "pulse"), []byte(stub), 0o755); err != nil {
				t.Fatalf("write pulse stub: %v", err)
			}

			script := fn + "\nINSTALL_DIR=" + installDir + "\nif installed_binary_is_pulse_pro; then echo pro; else echo community; fi\n"
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if err != nil {
				t.Fatalf("bash: %v\n%s", err, out)
			}
			got := strings.TrimSpace(string(out))
			want := "community"
			if tc.wantPro {
				want = "pro"
			}
			if got != want {
				t.Fatalf("installed_binary_is_pulse_pro on %q = %q, want %q", tc.versionLine, got, want)
			}
		})
	}

	t.Run("missing binary is not flagged", func(t *testing.T) {
		script := fn + "\nINSTALL_DIR=" + t.TempDir() + "\nif installed_binary_is_pulse_pro; then echo pro; else echo community; fi\n"
		out, err := exec.Command("bash", "-c", script).CombinedOutput()
		if err != nil {
			t.Fatalf("bash: %v\n%s", err, out)
		}
		if got := strings.TrimSpace(string(out)); got != "community" {
			t.Fatalf("missing binary should not be flagged as Pro, got %q", got)
		}
	})
}

// TestPerformUpdateRestartsServiceWhenInstallerFails asserts the #1630
// guarantee: when the downloaded installer exits non-zero (for example a
// write to a read-only path aborting it under errexit) after it has already
// stopped the service, perform_update must restore the backup AND leave the
// service running. The generated pulse-update.service gates on
// ExecCondition=systemctl is-active, so a service left stopped would also
// disable every future unattended run.
func TestPerformUpdateRestartsServiceWhenInstallerFails(t *testing.T) {
	script := `
set -uo pipefail
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
GITHUB_REPO="rcourtman/Pulse"
INSTALL_DIR="$TMP/opt/pulse"
CONFIG_DIR="$TMP/etc/pulse"
mkdir -p "$INSTALL_DIR/bin" "$CONFIG_DIR"
printf 'v5.1.24\n' > "$INSTALL_DIR/VERSION"
printf '#!/usr/bin/env bash\necho v5.1.24\n' > "$INSTALL_DIR/bin/pulse"
chmod +x "$INSTALL_DIR/bin/pulse"
export INSTALL_DIR

log() { echo "[$1] ${*:2}"; }
detect_service_name() { echo pulse; }
get_current_version() { tr -d '\r\n' < "$INSTALL_DIR/VERSION"; }
verify_release_signature() { return 0; }
sleep() { :; }

# curl writes the installer / signature to the -o target; the fake installer
# fails outright, like the real one aborting on a read-only filesystem after
# it has stopped the service.
curl() {
  local out="" prev="" arg
  for arg in "$@"; do
    if [[ "$prev" == "-o" ]]; then out="$arg"; fi
    prev="$arg"
  done
  if [[ -n "$out" ]]; then
    case "$out" in
      *.sig.*) printf 'dummy-signature\n' > "$out" ;;
      *) printf '#!/usr/bin/env bash\nexit 1\n' > "$out" ;;
    esac
  fi
  return 0
}

# Service is running at the was-active capture, then down (the installer
# stopped it before failing) until an explicit start/restart.
IS_ACTIVE_CALLS=0
SERVICE_UP="no"
STARTS=0
systemctl() {
  case "$1" in
    is-active)
      ((IS_ACTIVE_CALLS += 1))
      if (( IS_ACTIVE_CALLS == 1 )); then return 0; fi
      [[ "$SERVICE_UP" == "yes" ]] && return 0 || return 1
      ;;
    start|restart)
      ((STARTS += 1))
      SERVICE_UP="yes"
      return 0
      ;;
  esac
  return 1
}
` + extractAutoUpdateFunction(t, "is_prerelease_tag") + `
` + extractAutoUpdateFunction(t, "resolve_install_script_url") + `
` + extractAutoUpdateFunction(t, "wait_for_service_active") + `
` + extractAutoUpdateFunction(t, "ensure_service_restarted") + `
` + extractAutoUpdateFunction(t, "perform_update") + `
if perform_update v5.1.25; then
  echo "RESULT:succeeded"
else
  echo "RESULT:failed"
fi
echo "STARTS:$STARTS"
echo "SERVICE:$SERVICE_UP"
echo "VERSION:$(tr -d '\r\n' < "$INSTALL_DIR/VERSION")"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"RESULT:failed", "SERVICE:yes", "VERSION:v5.1.24"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in perform_update installer-failure output:\n%s", want, got)
		}
	}
	if strings.Contains(got, "STARTS:0") {
		t.Fatalf("service was never restarted after installer failure:\n%s", got)
	}
}

// TestEnsureServiceRestartedHonorsPriorServiceState asserts the RETURN-trap
// backstop behind the #1630 fix: a service that was inactive before the
// update is left alone, and a service that was active is started again.
func TestEnsureServiceRestartedHonorsPriorServiceState(t *testing.T) {
	script := `
set -uo pipefail
SERVICE_UP="no"
STARTS=0
systemctl() {
  case "$1" in
    is-active) [[ "$SERVICE_UP" == "yes" ]] && return 0 || return 1 ;;
    start|restart) ((STARTS += 1)); SERVICE_UP="yes"; return 0 ;;
  esac
  return 1
}
sleep() { :; }
log() { echo "[$1] ${*:2}"; }
` + extractAutoUpdateFunction(t, "wait_for_service_active") + `
` + extractAutoUpdateFunction(t, "ensure_service_restarted") + `
ensure_service_restarted pulse false || echo "INACTIVE_PATH_FAILED"
echo "STARTS_AFTER_INACTIVE:$STARTS"
ensure_service_restarted pulse true || echo "ACTIVE_PATH_FAILED"
echo "STARTS_AFTER_ACTIVE:$STARTS"
echo "SERVICE:$SERVICE_UP"
`

	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"STARTS_AFTER_INACTIVE:0", "STARTS_AFTER_ACTIVE:1", "SERVICE:yes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in ensure_service_restarted output:\n%s", want, got)
		}
	}
	for _, reject := range []string{"INACTIVE_PATH_FAILED", "ACTIVE_PATH_FAILED"} {
		if strings.Contains(got, reject) {
			t.Fatalf("ensure_service_restarted must always return 0 (it runs in a RETURN trap under set -e):\n%s", got)
		}
	}
}
