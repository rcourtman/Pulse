package notifications

import (
	"context"
	"os/exec"
	"sync/atomic"
	"time"
)

// A helper may inherit a stream and keep it open after Apprise exits or is
// killed at its delivery deadline. Bound the additional pipe-drain wait too.
const appriseOutputWaitDelay = time.Second

// Apprise output can contain credentials or private messages. Only its size is
// used by diagnostics, so drain both streams without retaining any of it. The
// counter uses constant space even when a provider helper is very noisy.
type appriseOutputCounter struct {
	bytes atomic.Int64
}

func (c *appriseOutputCounter) Write(p []byte) (int, error) {
	c.bytes.Add(int64(len(p)))
	return len(p), nil
}

func defaultAppriseExec(ctx context.Context, args []string) (int64, error) {
	cmd := exec.CommandContext(ctx, "apprise", args...)
	output := &appriseOutputCounter{}
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.WaitDelay = appriseOutputWaitDelay
	err := cmd.Run()
	return output.bytes.Load(), err
}
