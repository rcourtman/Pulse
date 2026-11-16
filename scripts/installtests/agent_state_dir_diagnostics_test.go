//go:build unix

package installtests

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func waitForLifecycleOutput(t *testing.T, proc *runningLifecycleAgent, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(proc.logs.String(), marker) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture child did not write its readiness marker: %s", proc.diagnostics())
}

// Observe the real testing.T failure/cleanup path in a separate test process.
// These intentionally failing children must never turn into passing controls.
func TestLifecycleAgentFailureEvidence(t *testing.T) {
	if mode := os.Getenv("PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL"); mode != "" {
		script := `printf 'fixture stdout bootstrap-one runtime-one\n'; printf 'fixture stderr bootstrap-two runtime-two\n' >&2; `
		switch mode {
		case "exit-zero", "shutdown-after-exit":
			script += "exit 0"
		case "exit-seven":
			script += "exit 7"
		case "timeout":
			script += "exec sleep 60" // No descendant holds the fixture output pipes.
		default:
			t.Fatalf("unknown diagnostic control %q", mode)
		}
		proc := startLifecycleProcess(t, exec.Command("bash", "-c", script))
		waitForLifecycleOutput(t, proc, "fixture stderr")
		if mode != "timeout" {
			select {
			case <-proc.done:
			case <-time.After(5 * time.Second):
				t.Fatal("fixture child exit was not observed")
			}
		}
		if mode == "shutdown-after-exit" {
			proc.stop(t)
		} else {
			waitForLifecycleState(t, 100*time.Millisecond, "controlled report",
				func(s agentLifecycleSnapshot) bool { return s.reportCount > 0 },
				&agentLifecycleControlPlane{}, proc)
		}
		t.Fatal("failure control unexpectedly returned")
		return
	}

	for _, tc := range []struct {
		mode, failure, result string
	}{
		{"exit-zero", "pulse-agent exited before controlled report: exit status 0", "process result: exit status 0"},
		{"exit-seven", "pulse-agent exited before controlled report: exit status 7", "process result: exit status 7"},
		{"shutdown-after-exit", "pulse-agent exited before requested shutdown: exit status 0", "process result: exit status 0"},
		{"timeout", "timed out waiting for controlled report", "process result: signal: killed"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLifecycleAgentFailureEvidence$", "-test.v", "-test.timeout=15s")
			cmd.Env = append(os.Environ(), "PULSE_LIFECYCLE_DIAGNOSTIC_CONTROL="+tc.mode)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("control did not finish with the expected test failure: context=%v err=%v\n%s", ctx.Err(), err, output)
			}
			text := string(output)
			for _, want := range []string{tc.failure, tc.result, "pulse-agent failure evidence (owned leader/output only):",
				"fixture stdout [fixture credential] [fixture credential]", "fixture stderr [fixture credential] [fixture credential]"} {
				if !strings.Contains(text, want) {
					t.Errorf("control lost %q:\n%s", want, text)
				}
			}
			for _, secret := range []string{"bootstrap-one", "runtime-one", "bootstrap-two", "runtime-two"} {
				if strings.Contains(text, secret) {
					t.Errorf("failure diagnostic disclosed a synthetic credential")
				}
			}
			if strings.Contains(text, "WARNING: DATA RACE") {
				t.Fatalf("control raced while retaining its output:\n%s", text)
			}
		})
	}
}

func TestLifecycleAgentStopDrainsOutputOnce(t *testing.T) {
	proc := startLifecycleProcess(t, exec.Command("bash", "-c", `
trap 'printf "shutdown stdout\n"; printf "shutdown stderr\n" >&2; exit 0' TERM
printf 'fixture ready bootstrap-one\n'
while :; do sleep 0.05; done
`))
	waitForLifecycleOutput(t, proc, "fixture ready")
	output := proc.stop(t)
	for _, want := range []string{"shutdown stdout", "shutdown stderr", "bootstrap-one"} {
		if !strings.Contains(output, want) {
			t.Errorf("stop lost raw %q; credential assertions must see unredacted output: %q", want, output)
		}
	}
	if got := proc.stop(t); got != output {
		t.Fatalf("second stop did not retain the first drained result: %q", got)
	}
	if result, exited := proc.result(); !exited || result != "exit status 0" {
		t.Fatalf("shutdown exit was not retained: %q, exited=%t", result, exited)
	}
}

func TestLifecycleAgentConcurrentOutputSnapshots(t *testing.T) {
	var logs lifecycleAgentOutput
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				_, _ = logs.Write([]byte("fixture output\n"))
				_ = logs.String()
			}
		})
	}
	workers.Wait()
	if got := strings.Count(logs.String(), "fixture output\n"); got != 400 {
		t.Fatalf("snapshots lost child output: got %d lines, want 400", got)
	}
}
