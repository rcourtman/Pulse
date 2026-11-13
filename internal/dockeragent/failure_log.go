package dockeragent

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rcourtman/pulse-go-rewrite/pkg/secretredact"
)

const (
	// failureLogKeepBytes is how much of the end of the log stream is kept.
	// Thirty lines are asked for; one of them can still be megabytes long, and
	// the newest text is the part that says why the container failed.
	failureLogKeepBytes = 64 * 1024
	// failureLogReadLimit bounds how much of the stream is read at all.
	failureLogReadLimit = 4 * 1024 * 1024
	// failureLogReadTimeout bounds one read. A daemon that stalls on a log
	// driver must not hold up the report.
	failureLogReadTimeout = 5 * time.Second
	// failureLogReadsPerCycle bounds new reads in one collection, so a host
	// where everything fails at once still reports on time. The rest are read
	// on the next cycles.
	failureLogReadsPerCycle = 5
	// failureLogRefresh is how often the lines of a container that stays in a
	// failing state are read again: one that is unhealthy, or in a restart
	// loop, has no single event to key on, and a loop restarting every few
	// seconds must not be read on every turn.
	failureLogRefresh = 5 * time.Minute
	// failureLogRestartHold keeps lines already captured from a restart loop
	// while the container is briefly "running" between two failures, so they
	// do not come and go with it. Nothing is read in that state.
	failureLogRestartHold = time.Minute
	// failureLogRetryAfter is how long a failed read waits before it is tried
	// again, so a daemon that keeps failing does not use up every collection.
	failureLogRetryAfter = time.Minute
)

// ANSI escape sequences: colour and cursor codes that loggers add for a
// terminal.
var failureLogANSIPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)

type failureLogEntry struct {
	key string
	log *agentsdocker.ContainerFailureLog
	// retryAt is set when the read failed: the key is not settled and the
	// read is tried again after this time.
	retryAt time.Time
}

// failureLogCache holds one captured log per failing container, so a failure
// is read from the daemon once and not on every report.
type failureLogCache struct {
	mu      sync.Mutex
	entries map[string]failureLogEntry
	reads   int
}

func (c *failureLogCache) prune(active map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id := range c.entries {
		if _, ok := active[id]; !ok {
			delete(c.entries, id)
		}
	}
	c.reads = 0
}

func (c *failureLogCache) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}

// failureLogReason says whether a container has failed and how, and returns a
// key that changes when it fails again. Every reason needs evidence of a
// failure in the container's own state: a non-zero exit code, an
// out-of-memory kill, or a failing health check. A container that stopped
// cleanly, one Docker restarts after a clean exit, one that is running
// normally and one that was never started have no reason, and are not read.
func failureLogReason(inspect containertypes.InspectResponse, now time.Time) (reason, key string) {
	state := inspect.State
	if state == nil {
		return "", ""
	}
	status := strings.ToLower(strings.TrimSpace(string(state.Status)))
	finished := strings.TrimSpace(state.FinishedAt)
	started := strings.TrimSpace(state.StartedAt)
	slot := now.Truncate(failureLogRefresh).Format(time.RFC3339)
	switch {
	case (status == "exited" || status == "dead") && state.OOMKilled:
		return agentsdocker.FailureLogReasonOOMKilled, "oom|" + finished
	case (status == "exited" || status == "dead") && state.ExitCode != 0:
		return agentsdocker.FailureLogReasonExited, "exit|" + finished
	case status == "restarting" && (state.ExitCode != 0 || state.OOMKilled):
		return agentsdocker.FailureLogReasonRestarting, "restart|" + slot
	case status == "running" && state.Health != nil && state.Health.Status == containertypes.Unhealthy:
		return agentsdocker.FailureLogReasonUnhealthy, "unhealthy|" + started + "|" + slot
	}
	return "", ""
}

// betweenRestarts reports whether a container with no failure in its present
// state is one Docker restarted a moment ago. Lines captured from its restart
// loop are held through that moment.
func betweenRestarts(inspect containertypes.InspectResponse, now time.Time) bool {
	state := inspect.State
	if state == nil || inspect.RestartCount == 0 || !strings.EqualFold(strings.TrimSpace(string(state.Status)), "running") {
		return false
	}
	started := parseTime(state.StartedAt)
	return !started.IsZero() && now.Sub(started) < failureLogRestartHold
}

// containerFailureLog returns the last log lines of a failed container when
// the operator has turned that on for this host. It reads the daemon once per
// failure and serves the next reports from memory. A container that is not
// failing is never read and loses whatever was held for it.
func (a *Agent) containerFailureLog(ctx context.Context, containerID string, inspect containertypes.InspectResponse) *agentsdocker.ContainerFailureLog {
	if !a.cfg.CollectFailureLogs || a.docker == nil {
		return nil
	}
	now := time.Now()
	reason, key := failureLogReason(inspect, now)
	if reason == "" {
		a.failureLogs.mu.Lock()
		entry, held := a.failureLogs.entries[containerID]
		a.failureLogs.mu.Unlock()
		if held && entry.log != nil && entry.log.Reason == agentsdocker.FailureLogReasonRestarting && betweenRestarts(inspect, now) {
			return cloneFailureLog(entry.log)
		}
		a.failureLogs.forget(containerID)
		return nil
	}

	a.failureLogs.mu.Lock()
	if entry, ok := a.failureLogs.entries[containerID]; ok && entry.key == key {
		if entry.retryAt.IsZero() || now.Before(entry.retryAt) {
			a.failureLogs.mu.Unlock()
			return cloneFailureLog(entry.log)
		}
	}
	if a.failureLogs.reads >= failureLogReadsPerCycle {
		// Lines held for an earlier failure are not this failure's evidence.
		a.failureLogs.mu.Unlock()
		return nil
	}
	a.failureLogs.reads++
	a.failureLogs.mu.Unlock()

	tty := inspect.Config != nil && inspect.Config.Tty
	readCtx, cancel := context.WithTimeout(ctx, failureLogReadTimeout)
	defer cancel()
	var raw string
	var truncated bool
	stream, err := a.docker.ContainerLogTail(readCtx, containerID, agentsdocker.FailureLogMaxLines)
	if err == nil {
		raw, truncated, err = readFailureLogStream(stream, tty)
		_ = stream.Close()
	}

	entry := failureLogEntry{key: key}
	lines, cut, redactions := prepareFailureLogLines(raw)
	if len(lines) > 0 {
		entry.log = &agentsdocker.ContainerFailureLog{
			Reason:     reason,
			Lines:      lines,
			Truncated:  truncated || cut || err != nil,
			Redactions: redactions,
			CapturedAt: now.UTC(),
		}
	}
	if err != nil {
		a.logger.Debug().Err(err).Str("container_id", containerID).Msg("Failure log read failed")
		if entry.log == nil {
			// Nothing was read. Try again later, not on every collection.
			entry.retryAt = now.Add(failureLogRetryAfter)
		}
	}
	// A read that ended cleanly with nothing in it is remembered too: the
	// container logged nothing, and the daemon is not asked again.
	a.failureLogs.mu.Lock()
	if a.failureLogs.entries == nil {
		a.failureLogs.entries = make(map[string]failureLogEntry)
	}
	a.failureLogs.entries[containerID] = entry
	a.failureLogs.mu.Unlock()
	return cloneFailureLog(entry.log)
}

func cloneFailureLog(in *agentsdocker.ContainerFailureLog) *agentsdocker.ContainerFailureLog {
	if in == nil {
		return nil
	}
	out := *in
	out.Lines = append([]string(nil), in.Lines...)
	return &out
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	limit   int
	data    []byte
	dropped bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n >= t.limit {
		t.dropped = t.dropped || n > t.limit || len(t.data) > 0
		t.data = append(t.data[:0], p[n-t.limit:]...)
		return n, nil
	}
	if over := len(t.data) + n - t.limit; over > 0 {
		t.data = t.data[over:]
		t.dropped = true
	}
	t.data = append(t.data, p...)
	return n, nil
}

// readFailureLogStream reads a log stream and returns its end. Docker frames
// stdout and stderr into chunks unless the container has a TTY: an 8-byte
// header (stream, three zero bytes, big-endian length) before each. The
// frames are removed here, with a fixed buffer, so a header that declares a
// huge length cannot make the agent allocate it. At most failureLogReadLimit
// bytes are read and the last failureLogKeepBytes of them kept. An error is
// returned with whatever was read before it.
func readFailureLogStream(stream io.Reader, tty bool) (text string, truncated bool, err error) {
	tail := &tailBuffer{limit: failureLogKeepBytes}
	limited := &io.LimitedReader{R: stream, N: failureLogReadLimit}
	buf := make([]byte, 32*1024)
	if tty {
		_, err = io.CopyBuffer(tail, limited, buf)
	} else {
		header := make([]byte, 8)
		for first := true; ; first = false {
			var n int
			if n, err = io.ReadFull(limited, header); err != nil {
				// A stream shorter than one header is not framed.
				if first && n > 0 {
					_, _ = tail.Write(header[:n])
				}
				break
			}
			if header[0] > 3 || header[1] != 0 || header[2] != 0 || header[3] != 0 {
				if !first {
					err = errors.New("log stream lost its framing")
					break
				}
				// Some runtimes send a raw stream for a container without a
				// TTY. What looked like a header is its first eight bytes.
				_, _ = tail.Write(header)
				_, err = io.CopyBuffer(tail, limited, buf)
				break
			}
			remaining := int64(binary.BigEndian.Uint32(header[4:]))
			if _, err = io.CopyBuffer(tail, io.LimitReader(limited, remaining), buf); err != nil {
				break
			}
		}
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	text = string(tail.data)
	if tail.dropped {
		// The kept text starts mid-line: drop the partial first line.
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	return text, tail.dropped || limited.N <= 0, err
}

// prepareFailureLogLines turns raw log text into what may leave the host:
// terminal codes and control characters removed, then secrets masked, then
// each line and the whole bounded with the newest lines kept. The order
// matters. Masking has to see the text as it will be sent.
func prepareFailureLogLines(raw string) (lines []string, truncated bool, redactions int) {
	raw = failureLogANSIPattern.ReplaceAllString(raw, "")
	clean := agentsdocker.CleanFailureLogText(raw)
	// Masked as a whole and not line by line: a PEM block spans lines.
	masked, redactions := secretredact.Text(clean)
	lines, truncated = agentsdocker.BoundFailureLogLines(strings.Split(masked, "\n"))
	return lines, truncated, redactions
}
