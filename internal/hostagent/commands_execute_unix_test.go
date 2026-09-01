//go:build !windows

package hostagent

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// waitForProcessGone fails the test if the given pid is still alive after the
// deadline. Signal 0 probes for existence; ESRCH means the kill got it.
func waitForProcessGone(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("pid %d still alive; process group was not killed (kill probe err=%v)", pid, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func readPidFile(t *testing.T, path string) int {
	t.Helper()
	pidBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading grandchild pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatalf("parsing grandchild pid %q: %v", pidBytes, err)
	}
	return pid
}

// Regression test for the delly fork-bomb incident (2026-07-07) and the minipc
// probe storm (2026-08-20): timing out a command must kill the whole process
// group, not just the direct shell. Before the fix, grandchildren (pct exec →
// lxc-attach → docker) survived the timeout and accumulated forever, and
// cmd.Wait() blocked on the stdout pipe the orphans kept open.
func TestCommandClient_executeCommand_TimeoutKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")

	c := &CommandClient{}
	payload := testApprovedCommandPayload(t, c, executeCommandPayload{
		RequestID: "r1",
		// The backgrounded sleep is a grandchild that inherits our stdout
		// pipe; `wait` keeps the shell alive until the timeout fires.
		Command: "sleep 300 & echo $! > " + pidFile + "; wait",
		Timeout: 1,
	})

	start := time.Now()
	result := c.executeCommand(context.Background(), payload)
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("executeCommand took %v; Wait must not hang on pipes held by orphaned grandchildren", elapsed)
	}
	if result.Success || result.Error != "command timed out" {
		t.Fatalf("expected timeout failure, got %#v", result)
	}

	waitForProcessGone(t, readPidFile(t, pidFile), 5*time.Second)
}

// A command that exits promptly but leaves a background child holding the
// stdout pipe must not block Wait past WaitDelay, and the command's own
// success must be preserved.
func TestCommandClient_executeCommand_WaitDelayUnblocksInheritedPipes(t *testing.T) {
	oldDelay := commandWaitDelay
	commandWaitDelay = 500 * time.Millisecond
	defer func() { commandWaitDelay = oldDelay }()

	c := &CommandClient{}
	payload := testApprovedCommandPayload(t, c, executeCommandPayload{
		RequestID: "r1",
		// No timeout fires here, so nothing kills the background child; it
		// holds the inherited stdout pipe and only WaitDelay unblocks Wait.
		Command: "sleep 30 & echo done",
		Timeout: 20,
	})

	start := time.Now()
	result := c.executeCommand(context.Background(), payload)
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("executeCommand took %v; WaitDelay should have unblocked Wait", elapsed)
	}
	if !result.Success || result.ExitCode != 0 {
		t.Fatalf("expected success, got %#v", result)
	}
	if !strings.Contains(result.Stdout, "done") {
		t.Fatalf("stdout = %q, expected to contain %q", result.Stdout, "done")
	}
}

// A server-issued cancel_command must stop the running command promptly, kill
// its whole process group, and report a distinct "command canceled" failure
// (regression for the minipc incident, where the agent kept probes running
// for 5+ minutes after the server had abandoned them).
func TestCommandClient_executeCommand_CancelKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")

	c := &CommandClient{logger: zerolog.Nop()}
	payload := testApprovedCommandPayload(t, c, executeCommandPayload{
		RequestID: "r1",
		Command:   "sleep 300 & echo $! > " + pidFile + "; wait",
		Timeout:   60,
	})

	cmdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state, _ := c.registerActiveCommand(nil, payload.RequestID, cancel)
	defer c.finishCancellableRequest(nil, payload.RequestID, state)

	go func() {
		// Give the shell time to start, then cancel the way handleMessages
		// does when a cancel_command arrives.
		time.Sleep(300 * time.Millisecond)
		c.handleCancelCommand(nil, cancelCommandPayload{RequestID: payload.RequestID})
	}()

	start := time.Now()
	result := c.executeCommand(cmdCtx, payload)
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("executeCommand took %v; cancel must stop the command promptly", elapsed)
	}
	if result.Success || result.Error != "command canceled" {
		t.Fatalf("expected canceled failure, got %#v", result)
	}

	waitForProcessGone(t, readPidFile(t, pidFile), 5*time.Second)
}
