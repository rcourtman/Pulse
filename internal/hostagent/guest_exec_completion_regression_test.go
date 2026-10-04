package hostagent

import (
	"context"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// Uses only the previously existing execute/guard boundary, so the unchanged
// parent can demonstrate that zero local CLI exit is not guest completion.
func TestGuestExecZeroCLIExitDoesNotProveGuestCompletion(t *testing.T) {
	for _, response := range []string{
		`{"pid":731}`, `{"exited":0,"pid":731}`, `{"exited":null}`,
		`{"exited":1,"exited":0}`, `{"exited":1,"EXITED":0}`,
		`{"exited":1} {"exited":0}`, `{"exited":1`, `not-json`,
	} {
		t.Run(response, func(t *testing.T) {
			var calls atomic.Int32
			g := newGuestExecGuard(func(context.Context, string) ([]byte, error) { return []byte("name: vm\n"), nil })
			c := &CommandClient{guestExecAdmission: g}
			oldExec := execCommandContext
			execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
				calls.Add(1)
				return exec.CommandContext(ctx, "sh", "-c", "printf '%s' "+shellQuote(response))
			}
			t.Cleanup(func() { execCommandContext = oldExec })
			payload := testApprovedCommandPayload(t, c, executeCommandPayload{Command: "echo original", TargetType: "vm", TargetID: "105", Trusted: true})
			got := c.executeCommand(context.Background(), payload)
			if got.Success || !agentexec.IsGuestExecDeferred(got.Error) || !strings.Contains(got.Error, "completion") || got.Stdout != response || got.ExitCode != 0 {
				t.Fatalf("CLI receipt became guest success: %#v", got)
			}
			if next := c.executeCommand(context.Background(), payload); next.Success || calls.Load() != 1 || !strings.Contains(next.Error, "uncertain") {
				t.Fatalf("unverified completion permitted another handoff: %#v calls=%d", next, calls.Load())
			}
		})
	}
}
