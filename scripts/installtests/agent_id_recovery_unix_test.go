//go:build linux || darwin || freebsd

package installtests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInstallSHAgentIDRecoveryRejectsSymlinkFIFOAndOversizedState(t *testing.T) {
	binaryPath := buildLifecycleAgent(t)
	root := t.TempDir()
	validPath := filepath.Join(root, "valid-agent-id")
	oversizedPath := filepath.Join(root, "oversized-agent-id")
	symlinkPath := filepath.Join(root, "symlink-agent-id")
	fifoPath := filepath.Join(root, "fifo-agent-id")
	if err := os.WriteFile(validPath, []byte("agent-safe-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oversizedPath, []byte(strings.Repeat("a", 5000)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(validPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		t.Fatal(err)
	}

	harness := func(path string) ([]byte, error) {
		script := `
			set -euo pipefail
			COLLECTOR_LIFECYCLE_BINARY_PATH="` + binaryPath + `"
			INSTALL_DIR="` + root + `"
			BINARY_NAME="pulse-agent"
			LEAST_PRIVILEGE_USER="pulse-agent-test-missing"
` + extractLifecycleTrustShellFunctions(t) + `
` + extractInstallShellFunction(t, "collector_lifecycle_binary") + `
` + extractInstallShellFunction(t, "read_agent_id_file_safely") + `
			read_agent_id_file_safely "` + path + `"
		`
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "bash", "-c", script).CombinedOutput()
	}
	if out, err := harness(validPath); err != nil || strings.TrimSpace(string(out)) != "agent-safe-123" {
		t.Fatalf("valid descriptor-bound agent ID recovery failed: %v\n%s", err, out)
	}
	for _, path := range []string{symlinkPath, fifoPath, oversizedPath} {
		started := time.Now()
		if out, err := harness(path); err == nil {
			t.Fatalf("unsafe agent ID path %s was accepted:\n%s", path, out)
		}
		if elapsed := time.Since(started); elapsed >= 2*time.Second {
			t.Fatalf("unsafe agent ID path %s blocked for %s", path, elapsed)
		}
	}
}
