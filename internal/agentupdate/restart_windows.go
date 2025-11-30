//go:build windows

package agentupdate

import (
	"os"
)

// restartProcess triggers a restart on Windows.
// For Windows services, we exit cleanly and rely on the Service Control Manager
// to restart the service (services are typically configured with automatic recovery).
// For non-service processes, os.Exit(0) is still appropriate as the process will
// restart with the new binary on next invocation.
func restartProcess(execPath string) error {
	// Exit with code 0 to signal clean shutdown.
	// Windows Service Control Manager will restart the service if configured
	// for automatic recovery (which is the default for our PowerShell installer).
	os.Exit(0)

	// This line is never reached, but satisfies the compiler
	return nil
}
