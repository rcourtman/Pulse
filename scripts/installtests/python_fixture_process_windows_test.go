package installtests

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// Retain Windows' existing direct-process cancellation. POSIX process-group
// ownership is not a Windows tree-cleanup claim. Pipe drainage is still bounded.
func pythonFixtureOutput(ctx context.Context, args, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = 2 * time.Second
	return cmd.CombinedOutput()
}
