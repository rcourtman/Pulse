//go:build !windows

package agentupdate

import (
	"fmt"
	"os"
	"syscall"
)

// restartProcess replaces the current process with a new instance.
// On Unix-like systems, this uses syscall.Exec for an in-place restart.
func restartProcess(execPath string) error {
	args := os.Args
	env := os.Environ()

	if err := syscall.Exec(execPath, args, env); err != nil {
		return fmt.Errorf("failed to restart: %w", err)
	}

	return nil
}
