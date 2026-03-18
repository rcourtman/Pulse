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
		filepath.Join(root, "billing.json"),
		filepath.Join(secretsDir, "handoff.key"),
		filepath.Join(root, ".cloud_handoff_key"),
	}
	mutablePaths := []string{
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

func repoRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(cwd, "..", ".."))
}
