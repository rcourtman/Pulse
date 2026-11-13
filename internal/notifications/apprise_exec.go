package notifications

import (
	"context"
	"os/exec"
	"sync/atomic"
)

// CLI output can be arbitrarily large and can contain private notification
// content. Its only consumer is a diagnostic byte count: drain both streams
// without retaining their bytes, truncating the command or changing its exit.
type appriseOutputCounter struct {
	bytes atomic.Int64
}

func (c *appriseOutputCounter) Write(p []byte) (int, error) {
	c.bytes.Add(int64(len(p)))
	return len(p), nil
}

func defaultAppriseExec(ctx context.Context, args []string) (int64, error) {
	cmd := exec.CommandContext(ctx, "apprise", args...)
	var output appriseOutputCounter
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.bytes.Load(), err
}
