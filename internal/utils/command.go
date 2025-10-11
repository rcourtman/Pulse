package utils

import (
	"os/exec"
)

// RunCommand executes a system command
func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
