//go:build linux || darwin || freebsd

package installtests

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func agentIDRecoveryCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	// The two-second recovery budget must include descendants, not just Bash.
	// A child retaining the output pipe must not hold the installer suite open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	return cmd
}

func TestInstallSHAgentIDRecoveryRejectsSymlinkFIFOAndOversizedState(t *testing.T) {
	buildStarted := time.Now()
	binaryPath := buildLifecycleAgent(t)
	t.Logf("built actual lifecycle agent in %s; recovery probes have an independent two-second deadline", time.Since(buildStarted))
	root := t.TempDir()
	validPath := filepath.Join(root, "valid-agent-id")
	symlinkPath := filepath.Join(root, "symlink-agent-id")
	fifoPath := filepath.Join(root, "fifo-agent-id")
	if err := os.WriteFile(validPath, []byte("agent-safe-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(validPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, content, want string
	}{
		{"valid", "agent-safe-123\n", "agent-safe-123"},
		{"without-newline", "agent-safe-123", "agent-safe-123"},
		{"maximum-id", strings.Repeat("a", 128) + "\n", strings.Repeat("a", 128)},
		{"oversized", strings.Repeat("a", 5000), ""},
		{"valid-prefix-oversized", "agent-safe-123\n" + strings.Repeat("a", 5000), ""},
		{"two-identities", "agent-safe-123\nother-agent\n", ""},
		{"nul-before-newline", "agent-safe-123\x00\n", ""},
		{"nul-at-eof", "agent-safe-123\x00", ""},
		{"too-long-id", strings.Repeat("a", 129), ""},
		{"empty", "", ""},
		{"newline-only", "\n", ""},
		{"invalid-id", "agent/safe\n", ""},
	}
	for _, mode := range []struct{ name, binary string }{{"descriptor", binaryPath}, {"legacy", ""}} {
		t.Run(mode.name, func(t *testing.T) {
			probe := func(t *testing.T, path, want string) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				started := time.Now()
				out, err := agentIDRecoveryCommand(ctx, "bash", "-c", agentIDRecoveryShell(t, mode.binary, root, path)).CombinedOutput()
				elapsed := time.Since(started)
				if ctx.Err() != nil || elapsed >= 2*time.Second {
					t.Fatalf("recovery probe exceeded its deadline: elapsed=%s context=%v exit=%v output=%q", elapsed, ctx.Err(), err, out)
				}
				if want != "" {
					if err != nil || string(out) != want+"\n" {
						t.Fatalf("valid agent ID recovery failed: elapsed=%s exit=%v output=%q", elapsed, err, out)
					}
				} else if err == nil || len(out) != 0 {
					t.Fatalf("unsafe agent ID state was accepted or disclosed: elapsed=%s exit=%v output=%q", elapsed, err, out)
				}
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					path := filepath.Join(root, mode.name+"-"+tc.name)
					if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
						t.Fatal(err)
					}
					probe(t, path, tc.want)
				})
			}
			t.Run("symlink", func(t *testing.T) { probe(t, symlinkPath, "") })
			t.Run("fifo", func(t *testing.T) { probe(t, fifoPath, "") })
			if mode.binary != "" {
				// The legacy compatibility path requires 0600. A valid 0400
				// file therefore proves the actual descriptor command ran.
				t.Run("descriptor-not-masked-by-fallback", func(t *testing.T) {
					if err := os.Chmod(validPath, 0400); err != nil {
						t.Fatal(err)
					}
					probe(t, validPath, "agent-safe-123")
				})
			}
		})
	}
}

func TestAgentIDRecoveryCancellationStopsDescendants(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	childFile := filepath.Join(t.TempDir(), "child-pid")
	cmd := agentIDRecoveryCommand(ctx, "bash", "-c", `sleep 30 & child=$!; printf '%s\n' "$child" > "`+childFile+`"; wait`)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	finished := make(chan error, 1)
	go func() { finished <- cmd.Run() }()
	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(childFile)
		if err == nil {
			childPID, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if childPID > 1 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID <= 1 {
		cancel()
		<-finished
		t.Fatal("descendant fixture did not start within two seconds")
	}
	defer syscall.Kill(childPID, syscall.SIGKILL)
	cancelled := time.Now()
	cancel()
	if err := <-finished; err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancellation did not fail the command: exit=%v context=%v", err, ctx.Err())
	}
	if elapsed := time.Since(cancelled); elapsed > time.Second {
		t.Fatalf("cancellation held an inherited output pipe for %s", elapsed)
	}
	deadline = time.Now().Add(time.Second)
	for {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant %d survived cancellation: %v", childPID, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
