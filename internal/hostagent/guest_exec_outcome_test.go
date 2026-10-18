package hostagent

import (
	"context"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// The provider is a local fixture, never QGA. Exercise the production command
// boundary on the unchanged parent as well as on the corrected source.
func testGuestExecProjectsGuestOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, reply, stdout, stderr string
		exitCode                    int
		success, deferred           bool
	}{
		{"successful guest", `{"exited":1,"exitcode":0,"out-data":"guest\nobservation café","err-data":"guest warning"}`, "guest\nobservation café", "guest warning", 0, true, false},
		{"empty success", `{"exited":true,"exitcode":0}`, "", "", 0, true, false},
		{"guest failure", `{"exited":1,"exitcode":7,"out-data":"misleading service identity","err-data":"failed probe"}`, "misleading service identity", "failed probe", 7, false, false},
		{"guest signal", `{"exited":1,"signal":15,"out-data":"partial"}`, "partial", "", 143, false, false},
		{"truncated stdout", `{"exited":1,"exitcode":0,"out-data":"partial","out-truncated":true}`, "partial", "", 0, false, false},
		{"truncated stderr", `{"exited":1,"exitcode":0,"err-data":"partial","err-truncated":1}`, "", "partial", 0, false, false},
		{"missing outcome", `{"exited":1}`, "", "", 0, false, true},
		{"null outcome", `{"exited":1,"exitcode":null}`, "", "", 0, false, true},
		{"string outcome", `{"exited":1,"exitcode":"0"}`, "", "", 0, false, true},
		{"fractional outcome", `{"exited":1,"exitcode":0.5}`, "", "", 0, false, true},
		{"negative outcome", `{"exited":1,"exitcode":-1}`, "", "", 0, false, true},
		{"out of range outcome", `{"exited":1,"exitcode":256}`, "", "", 0, false, true},
		{"competing outcomes", `{"exited":1,"exitcode":0,"signal":15}`, "", "", 0, false, true},
		{"duplicate outcome", `{"exited":1,"exitcode":7,"exitcode":0}`, "", "", 0, false, true},
		{"case conflicting outcome", `{"exited":1,"exitcode":0,"EXITCODE":7}`, "", "", 0, false, true},
		{"wrong output type", `{"exited":1,"exitcode":0,"out-data":{}}`, "", "", 0, false, true},
		{"ambiguous truncation", `{"exited":1,"exitcode":0,"out-truncated":"false"}`, "", "", 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls, reads atomic.Int32
			guard := newGuestExecGuard(func(context.Context, string) ([]byte, error) {
				reads.Add(1)
				return []byte("name: vm\n"), nil
			})
			c := &CommandClient{guestExecAdmission: guard}
			oldExec := execCommandContext
			execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
				calls.Add(1)
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' "+shellQuote(tc.reply))
			}
			t.Cleanup(func() { execCommandContext = oldExec })
			payload := testApprovedCommandPayload(t, c, executeCommandPayload{Command: "echo observed", TargetType: "vm", TargetID: "105", Trusted: true})
			got := c.executeCommand(context.Background(), payload)
			if tc.deferred {
				if got.Success || !agentexec.IsGuestExecDeferred(got.Error) || got.Stdout != tc.reply || got.ExitCode != 0 {
					t.Fatalf("ambiguous result became guest evidence: %#v", got)
				}
				next := c.executeCommand(context.Background(), payload)
				if next.Success || calls.Load() != 1 || reads.Load() != 1 || !agentexec.IsGuestExecDeferred(next.Error) {
					t.Fatalf("ambiguous result released admission: %#v calls=%d reads=%d", next, calls.Load(), reads.Load())
				}
				return
			}
			if got.Success != tc.success || got.ExitCode != tc.exitCode || got.Stdout != tc.stdout || got.Stderr != tc.stderr || agentexec.IsGuestExecDeferred(got.Error) || (got.Error == "") != tc.success {
				t.Fatalf("CLI success replaced actual guest outcome: %#v", got)
			}
			if strings.Contains(got.Error, "misleading") || strings.Contains(got.Error, "failed probe") {
				t.Fatal("guest output entered the authored error")
			}
			// A known failed/truncated terminal result is not an unknown QGA
			// handoff. A distinct explicit request still checks the VM lock.
			_ = c.executeCommand(context.Background(), payload)
			if calls.Load() != 2 || reads.Load() != 4 {
				t.Fatalf("terminal failure acquired uncertainty or skipped lock checks: calls=%d reads=%d", calls.Load(), reads.Load())
			}
		})
	}
}
