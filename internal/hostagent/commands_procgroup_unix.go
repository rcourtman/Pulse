//go:build !windows

package hostagent

import (
	"os"
	"os/exec"
	"syscall"
)

// configureCommandProcessGroup runs cmd in its own process group and replaces
// the default context-cancel behavior (kill only the direct child) with a kill
// of the entire group. Without this, timing out a command like
// `pct exec <vmid> -- sh -c '...'` kills the outer shell but orphans
// lxc-attach and everything below it; against a wedged container those
// orphans never exit and pile up on the host every poll cycle.
func configureCommandProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		// With Setpgid the child's pgid equals its pid; a negative pid
		// signals the whole group.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
