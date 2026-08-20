//go:build windows

package hostagent

import (
	"os/exec"
	"strconv"
)

// configureCommandProcessGroup arranges for the command's whole process tree
// to be killed when its context is canceled. Windows has no process groups in
// the POSIX sense; short of managing a job object, taskkill /T is the
// standard way to terminate a tree, with a direct Kill as fallback for the
// root process.
func configureCommandProcessGroup(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		return cmd.Process.Kill()
	}
}
