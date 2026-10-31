//go:build linux || darwin || freebsd

package installtests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

type agentIDRecoveryObservation struct {
	output, phases                      []byte
	err, contextErr                     error
	preparation, start, wait, execution time.Duration
	started                             bool
}

func (o agentIDRecoveryObservation) String() string {
	// fmt does not invoke Error on unexported struct fields. Format the errors
	// explicitly so a failure retains its actual context/exit, not an address.
	return fmt.Sprintf("preparation=%s execution=%s start=%s wait=%s started=%t context=%v exit=%v",
		o.preparation, o.execution, o.start, o.wait, o.started, o.contextErr, o.err)
}

func TestAgentIDRecoveryObservationRetainsContextAndExit(t *testing.T) {
	observed := agentIDRecoveryObservation{contextErr: context.DeadlineExceeded, err: os.ErrNotExist}
	description := fmt.Sprintf("%+v", observed)
	if !strings.Contains(description, "context=context deadline exceeded") ||
		!strings.Contains(description, "exit="+os.ErrNotExist.Error()) {
		t.Fatalf("recovery errors lost their actual diagnostic: %s", description)
	}
}

// Keep fixture extraction and trace-file setup outside the execution budget.
// Both can take time under the race detector without the recovery shell having
// started. Once the command is ready, the same two seconds cover Bash, every
// child, and output collection. The private phase stream never joins stdout.
func observeAgentIDRecovery(t *testing.T, prepare func() string) agentIDRecoveryObservation {
	t.Helper()
	var observed agentIDRecoveryObservation
	prepared := time.Now()
	shell := prepare()
	phasePath := filepath.Join(t.TempDir(), "recovery-phases")
	phaseFile, err := os.OpenFile(phasePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer phaseFile.Close()
	// Only fixed phase names and an exit status are recorded. DEBUG follows
	// functions/subshells so a helper stall differs from a legacy content read;
	// it never records BASH_COMMAND, state contents, paths or credentials.
	shell = `printf 'shell-entered\n' >&3
trap 'rc=$?; printf "shell-exit %s\n" "$rc" >&3' EXIT
set -T
trap 'case "$BASH_COMMAND" in
    read_agent_id_file_safely*) printf "recovery-call\n" >&3 ;;
    collector_lifecycle_binary*) printf "helper-select\n" >&3 ;;
    trusted_lifecycle_regular_file*) printf "trust-check\n" >&3 ;;
    identity=\$*) printf "helper-read\n" >&3 ;;
    identity_size=\$*) printf "size-read\n" >&3 ;;
    IFS=\ read*) printf "content-read\n" >&3 ;;
esac' DEBUG
` + shell
	observed.preparation = time.Since(prepared)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	execution := time.Now()
	cmd := agentIDRecoveryCommand(ctx, "bash", "-c", shell)
	cmd.ExtraFiles = []*os.File{phaseFile}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	observed.err = cmd.Start()
	observed.start = time.Since(execution)
	if observed.err == nil {
		observed.started = true
		waiting := time.Now()
		observed.err = cmd.Wait()
		observed.wait = time.Since(waiting)
	}
	observed.execution = time.Since(execution)
	observed.contextErr = ctx.Err()
	observed.output = output.Bytes()
	if err := phaseFile.Close(); err != nil {
		t.Fatal(err)
	}
	observed.phases, err = os.ReadFile(phasePath)
	if err != nil {
		t.Fatal(err)
	}
	return observed
}

func TestInstallSHAgentIDRecoveryRejectsSymlinkFIFOAndOversizedState(t *testing.T) {
	// The real-agent build and all unsafe-file probes own their paths and
	// process groups; let Go overlap them with independent transport waits.
	t.Parallel()
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
				observed := observeAgentIDRecovery(t, func() string {
					return agentIDRecoveryShell(t, mode.binary, root, path)
				})
				if observed.contextErr != nil || observed.execution >= 2*time.Second {
					t.Fatalf("recovery probe exceeded its deadline: %+v output=%q phases=%q", observed, observed.output, observed.phases)
				}
				if want != "" {
					if observed.err != nil || string(observed.output) != want+"\n" {
						t.Fatalf("valid agent ID recovery failed: %+v output=%q phases=%q", observed, observed.output, observed.phases)
					}
				} else if observed.err == nil || len(observed.output) != 0 {
					t.Fatalf("unsafe agent ID state was accepted or disclosed: %+v output=%q phases=%q", observed, observed.output, observed.phases)
				}
				if !observed.started || !bytes.Contains(observed.phases, []byte("shell-entered\n")) ||
					!bytes.Contains(observed.phases, []byte("recovery-call\n")) ||
					!bytes.Contains(observed.phases, []byte("shell-exit ")) {
					t.Fatalf("recovery probe did not retain its shell outcome: %+v phases=%q", observed, observed.phases)
				}
				if (path == fifoPath || path == symlinkPath) && bytes.Contains(observed.phases, []byte("content-read\n")) {
					t.Fatalf("unsafe file reached the legacy content reader: phases=%q", observed.phases)
				}
				if want != "" {
					phase := "helper-read\n"
					if mode.binary == "" {
						phase = "content-read\n"
					}
					if !bytes.Contains(observed.phases, []byte(phase)) {
						t.Fatalf("valid identity bypassed its expected reader: phases=%q", observed.phases)
					}
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

func TestAgentIDRecoveryBudgetStartsAfterFixturePreparation(t *testing.T) {
	observed := observeAgentIDRecovery(t, func() string {
		// A deliberately slow fixture must not consume the shell's two seconds.
		// The old caller started its context before extracting installer functions.
		time.Sleep(2100 * time.Millisecond)
		return "exit 1"
	})
	if observed.preparation < 2*time.Second || observed.contextErr != nil || observed.execution >= 2*time.Second ||
		!observed.started || observed.err == nil || len(observed.output) != 0 ||
		string(observed.phases) != "shell-entered\nshell-exit 1\n" {
		t.Fatalf("fixture preparation consumed the execution budget or lost the result: %+v phases=%q", observed, observed.phases)
	}
}

func TestAgentIDRecoveryReportsBlockedHelperAndStopsItsChild(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "agent-id")
	childPath := filepath.Join(root, "child-pid")
	if err := os.WriteFile(path, []byte("agent-safe-123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	observed := observeAgentIDRecovery(t, func() string {
		return `set -euo pipefail
collector_lifecycle_binary() {
    sleep 30 & child=$!
    printf '%s\n' "$child" > "` + childPath + `"
    wait
}
` + extractInstallShellFunction(t, "read_agent_id_file_safely") + `
read_agent_id_file_safely "` + path + `"`
	})
	if !errors.Is(observed.contextErr, context.DeadlineExceeded) || !observed.started || observed.err == nil ||
		len(observed.output) != 0 || !bytes.Contains(observed.phases, []byte("helper-select\n")) ||
		bytes.Contains(observed.phases, []byte("content-read\n")) || bytes.Contains(observed.phases, []byte("shell-exit ")) {
		t.Fatalf("blocked helper was not attributed and stopped: %+v phases=%q", observed, observed.phases)
	}
	child, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(child)))
	if err != nil || childPID <= 1 {
		t.Fatalf("invalid child identity %q: %v", child, err)
	}
	defer syscall.Kill(childPID, syscall.SIGKILL)
	deadline := time.Now().Add(time.Second)
	for {
		if errors.Is(syscall.Kill(childPID, 0), syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("blocked helper descendant %d survived the process-group deadline", childPID)
		}
		time.Sleep(10 * time.Millisecond)
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
