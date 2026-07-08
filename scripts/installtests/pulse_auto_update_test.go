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
