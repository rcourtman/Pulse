//go:build !windows

package installtests

import (
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

// A control can invoke Bash, curl or a manager double. Killing only Python
// leaves those processes holding CombinedOutput's pipes, beyond the control's
// existing deadline. Own one process group; bound output drainage separately.
func pythonFixtureOutput(ctx context.Context, args, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stop := func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.Cancel = stop
	cmd.WaitDelay = 2 * time.Second
	output, err := cmd.CombinedOutput()
	// WaitDelay can return after the leader exits while an orphan still holds
	// a pipe. Also clean that group; a passing leader with leftovers is not a
	// passing control. This does not claim cleanup of deliberately escaped groups.
	if cleanupErr := stop(); cleanupErr == nil && err == nil {
		err = errors.New("Python fixture left descendant processes")
	} else if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrProcessDone) {
		err = errors.Join(err, fmt.Errorf("stop Python fixture process group: %w", cleanupErr))
	}
	return output, err
}

const blockedPythonFixture = `import os, pathlib, subprocess, sys, time, unittest
root = pathlib.Path(os.environ["PULSE_PYTHON_PROCESS_FIXTURE"])
def child():
    process = subprocess.Popen([sys.executable, "-I", "-S", "-c",
        "import pathlib, signal, sys, time; signal.signal(signal.SIGTERM, signal.SIG_IGN); "
        "pathlib.Path(sys.argv[1]).touch(); time.sleep(60)", str(root / "ready")],
        stdout=sys.stdout, stderr=sys.stderr)
    (root / "pid").write_text(str(process.pid))
    while not (root / "ready").exists(): time.sleep(0.01)
def block():
    child()
    while True: time.sleep(1)
if os.environ.get("PULSE_PYTHON_PROCESS_PHASE") == "discovery": block()
class Controls(unittest.TestCase):
    def test_blocked(self): block()
    def test_orphan(self): child()
if __name__ == "__main__": unittest.main(verbosity=2)
`

func pythonFixtureChild(t *testing.T, root string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	registered := false
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(root, "pid"))
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err == nil && pid > 1 {
				// Always own cleanup, including when testing the old helper.
				if !registered {
					t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
					registered = true
				}
				if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
					return pid
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Python control did not start its owned child within five seconds")
	return 0
}

func pythonFixtureChildStopped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		// An orphan may briefly await init's reap. A zombie cannot hold pipes
		// or do work; distinguish it from a surviving process without claiming
		// that a successful group signal alone established termination.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		state, err := exec.CommandContext(ctx, "ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		cancel()
		if err == nil && strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned Python fixture child %d is still running", pid)
}

func TestPythonFixtureControlsCancellationStopsOwnedGroup(t *testing.T) {
	for _, phase := range []string{"discovery", "execution"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			script := filepath.Join(root, "controls.py")
			if err := os.WriteFile(script, []byte(blockedPythonFixture), 0600); err != nil {
				t.Fatal(err)
			}
			// This separate process is not part of the control's group. A broad
			// process-name cleanup would wrongly kill it.
			sentinel := exec.Command("python3", "-I", "-S", "-c", "import time; time.sleep(60)")
			if err := sentinel.Start(); err != nil {
				t.Fatal(err)
			}
			sentinelDone := make(chan struct{})
			go func() { _ = sentinel.Wait(); close(sentinelDone) }()
			t.Cleanup(func() { _ = sentinel.Process.Kill(); <-sentinelDone })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			env := []string{"PULSE_PYTHON_PROCESS_FIXTURE=" + root, "PULSE_PYTHON_PROCESS_PHASE=" + phase}
			go func() {
				if phase == "discovery" {
					_, err := discoverPythonFixtureControls(ctx, script, "Controls", []string{"test_blocked"}, env...)
					finished <- err
				} else {
					_, err := executePythonFixtureControl(ctx, script, "Controls", "test_blocked", env...)
					finished <- err
				}
			}()
			pid := pythonFixtureChild(t, root)
			cancel()
			select {
			case err := <-finished:
				if err == nil || !strings.Contains(err.Error(), "context canceled") {
					t.Fatalf("lost the cancelled control's failure: %v", err)
				}
			case <-time.After(4 * time.Second):
				// Release the baseline's inherited pipe before failing the
				// witness; no stuck goroutine or sixty-second child is left.
				_ = syscall.Kill(pid, syscall.SIGKILL)
				<-finished
				t.Fatal("cancelled Python control waited for its descendant's output pipe")
			}
			pythonFixtureChildStopped(t, pid)
			select {
			case <-sentinelDone:
				t.Fatal("unrelated Python process was stopped")
			default:
			}
		})
	}
}

func TestPythonFixtureControlsPassingLeaderWithHeldPipeFails(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "controls.py")
	if err := os.WriteFile(script, []byte(blockedPythonFixture), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := executePythonFixtureControl(ctx, script, "Controls", "test_orphan",
			"PULSE_PYTHON_PROCESS_FIXTURE="+root)
		finished <- err
	}()
	pid := pythonFixtureChild(t, root)
	select {
	case err := <-finished:
		if !errors.Is(err, exec.ErrWaitDelay) || ctx.Err() != nil {
			t.Fatalf("a passing leader with incomplete output must fail before its control deadline: %v (context %v)", err, ctx.Err())
		}
	case <-time.After(4 * time.Second):
		_ = syscall.Kill(pid, syscall.SIGKILL)
		<-finished
		t.Fatal("passing Python leader left the observer waiting for a descendant's pipe")
	}
	pythonFixtureChildStopped(t, pid)
}
