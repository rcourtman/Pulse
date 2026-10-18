package hostagent

import (
	"context"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
)

// No PVE config exists in the offline guest. A command policy approval (or
// Pulse's internal Trusted marker) is not evidence that this VM is unlocked.
func testVMGuestExecNoUnverifiedHandoff(t *testing.T) {
	for _, target := range []struct{ kind, id string }{
		{"vm", "105"}, {" VM ", "105"}, {"vm", "000105"},
		{"vm", "0"}, {"vm", "not-a-vmid"},
	} {
		for _, trusted := range []bool{false, true} {
			t.Run(target.kind+"/"+target.id+"/trusted="+map[bool]string{true: "yes", false: "no"}[trusted], func(t *testing.T) {
				var handoffs atomic.Int32
				oldExec := execCommandContext
				execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
					handoffs.Add(1)
					// Simulate a successful provider boundary, never invoke QGA.
					return exec.CommandContext(ctx, "sh", "-c", "printf 'guest-output'")
				}
				t.Cleanup(func() { execCommandContext = oldExec })
				c := &CommandClient{}
				payload := testApprovedCommandPayload(t, c, executeCommandPayload{
					RequestID: "unverified-vm", Command: "echo inspected", TargetType: target.kind,
					TargetID: target.id, Trusted: trusted,
				})
				got := c.executeCommand(context.Background(), payload)
				if handoffs.Load() != 0 {
					t.Errorf("unverified VM crossed command handoff %d times", handoffs.Load())
				}
				if got.Success || got.Stdout != "" || !strings.Contains(got.Error, "guest execution deferred") {
					t.Errorf("unverified VM was not an explicit output-free deferral: %#v", got)
				}
			})
		}
	}
}
