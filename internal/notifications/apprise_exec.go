package notifications

import (
	"context"
	"os/exec"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

const appriseCLITitleBytes = 1024

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

// Alert text carries names chosen by whoever reports a resource. The operating
// system refuses to start a process when one argument holds a NUL byte or is
// larger than its per-argument limit (128 KiB on Linux), and a batch that cannot
// start the CLI loses every alert grouped into it. Only the title remains an
// argument: keep it one bounded line. Truncation never splits a UTF-8 character.
func appriseCLITitle(title string) string {
	title = emailSubjectLabel(title)
	if len(title) <= appriseCLITitleBytes {
		return title
	}
	end := appriseCLITitleBytes - len("…")
	for !utf8.RuneStart(title[end]) {
		end--
	}
	return title[:end] + "…"
}

// The body travels on standard input, which has no size limit. Do not hand the
// CLI bytes it could never receive as an argument, or text its decoder rejects.
func appriseCLIBody(body string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(body, "\x00", ""), "\uFFFD")
}

func defaultAppriseExec(ctx context.Context, args []string, body string) (int64, error) {
	cmd := exec.CommandContext(ctx, "apprise", args...)
	cmd.Stdin = strings.NewReader(body)
	var output appriseOutputCounter
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.bytes.Load(), err
}
