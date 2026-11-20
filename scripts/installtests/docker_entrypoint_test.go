package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerEntrypointSkipsImmutableOwnershipPaths(t *testing.T) {
	repoRoot := repoRoot(t)
	entrypointPath := filepath.Join(repoRoot, "docker-entrypoint.sh")

	entrypoint, err := os.ReadFile(entrypointPath)
	if err != nil {
		t.Fatalf("read docker-entrypoint.sh: %v", err)
	}

	const marker = "# Only adjust permissions if running as root"
	prefix, _, found := strings.Cut(string(entrypoint), marker)
	if !found {
		t.Fatalf("docker-entrypoint.sh missing marker %q", marker)
	}

	root := filepath.Join(t.TempDir(), "etc", "pulse")
	secretsDir := filepath.Join(root, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("mkdir secrets dir: %v", err)
	}

	immutablePaths := []string{
		filepath.Join(secretsDir, "handoff.key"),
		filepath.Join(root, ".cloud_handoff_key"),
	}
	mutablePaths := []string{
		filepath.Join(root, "billing.json"),
		filepath.Join(root, "system.json"),
		filepath.Join(root, ".env"),
	}
	for _, path := range append(append([]string{}, immutablePaths...), mutablePaths...) {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	chownLog := filepath.Join(t.TempDir(), "chown.log")
	chownStub := filepath.Join(binDir, "chown")
	chownScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$CHOWN_LOG"
for arg in "$@"; do
    case ":$PULSE_IMMUTABLE_OWNERSHIP_PATHS:" in
        *:"$arg":*)
            echo "immutable path should not be chowned: $arg" >&2
            exit 44
            ;;
    esac
done
exit 0
`
	if err := os.WriteFile(chownStub, []byte(chownScript), 0o755); err != nil {
		t.Fatalf("write chown stub: %v", err)
	}

	shell := prefix + `
set -eu
root="` + root + `"
export PULSE_IMMUTABLE_OWNERSHIP_PATHS="` + strings.Join(immutablePaths, ":") + `"
export CHOWN_LOG="` + chownLog + `"
export PATH="` + binDir + `:$PATH"
chown_tree_skipping_immutable_paths pulse:pulse "$root"
`

	cmd := exec.Command("sh", "-c", shell)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run helper: %v\n%s", err, out)
	}

	logData, err := os.ReadFile(chownLog)
	if err != nil {
		t.Fatalf("read chown log: %v", err)
	}
	logText := string(logData)
	for _, immutablePath := range immutablePaths {
		if strings.Contains(logText, immutablePath) {
			t.Fatalf("immutable path was chowned: %s\nlog:\n%s", immutablePath, logText)
		}
	}
	for _, mutablePath := range mutablePaths {
		if !strings.Contains(logText, mutablePath) {
			t.Fatalf("mutable path missing from chown log: %s\nlog:\n%s", mutablePath, logText)
		}
	}
	if !strings.Contains(logText, root) {
		t.Fatalf("root directory missing from chown log:\n%s", logText)
	}
}

func TestDockerEntrypointDoesNotChownAlreadyOwnedTree(t *testing.T) {
	repoRoot := repoRoot(t)
	entrypointPath := filepath.Join(repoRoot, "docker-entrypoint.sh")

	entrypoint, err := os.ReadFile(entrypointPath)
	if err != nil {
		t.Fatalf("read docker-entrypoint.sh: %v", err)
	}

	const marker = "# Only adjust permissions if running as root"
	prefix, _, found := strings.Cut(string(entrypoint), marker)
	if !found {
		t.Fatalf("docker-entrypoint.sh missing marker %q", marker)
	}

	root := t.TempDir()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	chownLog := filepath.Join(t.TempDir(), "chown.log")
	chownStub := filepath.Join(binDir, "chown")
	chownScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$CHOWN_LOG"
exit 0
`
	if err := os.WriteFile(chownStub, []byte(chownScript), 0o755); err != nil {
		t.Fatalf("write chown stub: %v", err)
	}
	systemChown, err := exec.LookPath("chown")
	if err != nil {
		t.Fatalf("find system chown: %v", err)
	}

	shell := prefix + `
set -eu
root="` + root + `"
uid="$(id -u)"
gid="$(id -g)"
"` + systemChown + `" "$uid:$gid" "$root"
export CHOWN_LOG="` + chownLog + `"
export PATH="` + binDir + `:$PATH"
chown_tree_if_owner_mismatch pulse:pulse "$uid" "$gid" "$root"
`

	cmd := exec.Command("sh", "-c", shell)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run helper: %v\n%s", err, out)
	}

	if logData, err := os.ReadFile(chownLog); err == nil && len(logData) > 0 {
		t.Fatalf("already-owned tree was chowned:\n%s\nhelper output:\n%s", logData, out)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read chown log: %v", err)
	}
}

func TestDockerEntrypointWarnsWhenDataDirIsEphemeral(t *testing.T) {
	repoRoot := repoRoot(t)
	entrypointPath := filepath.Join(repoRoot, "docker-entrypoint.sh")

	entrypoint, err := os.ReadFile(entrypointPath)
	if err != nil {
		t.Fatalf("read docker-entrypoint.sh: %v", err)
	}

	const marker = "# Only adjust permissions if running as root"
	prefix, _, found := strings.Cut(string(entrypoint), marker)
	if !found {
		t.Fatalf("docker-entrypoint.sh missing marker %q", marker)
	}

	dataDir := filepath.Join(t.TempDir(), "data")
	nestedDir := filepath.Join(dataDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested data dir: %v", err)
	}

	unmountedMountInfo := filepath.Join(t.TempDir(), "mountinfo-ephemeral")
	if err := os.WriteFile(unmountedMountInfo, []byte("25 1 0:23 / / rw,relatime - overlay overlay rw\n26 25 0:4 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw\n"), 0o644); err != nil {
		t.Fatalf("write unmounted mountinfo: %v", err)
	}

	mountedMountInfo := filepath.Join(t.TempDir(), "mountinfo-persistent")
	if err := os.WriteFile(mountedMountInfo, []byte("25 1 0:23 / / rw,relatime - overlay overlay rw\n27 25 8:1 /var/lib/docker/volumes/pulse-data/_data "+dataDir+" rw,relatime - ext4 /dev/sda1 rw\n"), 0o644); err != nil {
		t.Fatalf("write mounted mountinfo: %v", err)
	}

	cases := []struct {
		name          string
		target        string
		mountinfo     string
		suppressValue string
		wantWarning   bool
	}{
		{
			name:        "warns when data dir is not a mount point",
			target:      dataDir,
			mountinfo:   unmountedMountInfo,
			wantWarning: true,
		},
		{
			name:        "silent when data dir is mounted",
			target:      dataDir,
			mountinfo:   mountedMountInfo,
			wantWarning: false,
		},
		{
			name:        "silent when nested under mounted data dir",
			target:      nestedDir,
			mountinfo:   mountedMountInfo,
			wantWarning: false,
		},
		{
			name:          "silent when warning explicitly suppressed",
			target:        dataDir,
			mountinfo:     unmountedMountInfo,
			suppressValue: "true",
			wantWarning:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shell := prefix + `
set -eu
export PULSE_SUPPRESS_EPHEMERAL_DATA_WARNING="` + tc.suppressValue + `"
warn_if_ephemeral_data_dir "` + tc.target + `" "` + tc.mountinfo + `"
`
			cmd := exec.Command("sh", "-c", shell)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run warn_if_ephemeral_data_dir: %v\n%s", err, out)
			}
			got := string(out)
			if tc.wantWarning {
				if !strings.Contains(got, "WARNING:") || !strings.Contains(got, "-v pulse-data:/data") {
					t.Fatalf("expected ephemeral data dir warning, got: %q", got)
				}
			} else if strings.TrimSpace(got) != "" {
				t.Fatalf("expected no warning, got: %q", got)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(cwd, "..", ".."))
}
