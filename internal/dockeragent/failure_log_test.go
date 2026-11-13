package dockeragent

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

// dockerLogFrames builds the multiplexed stream Docker sends for a container
// without a TTY: an 8-byte header (stream, three zero bytes, length) before
// each chunk. Stream 1 is stdout and 2 is stderr.
func dockerLogFrames(chunks ...[2]string) io.ReadCloser {
	var out bytes.Buffer
	for _, chunk := range chunks {
		stream := byte(1)
		if chunk[0] == "stderr" {
			stream = 2
		}
		header := make([]byte, 8)
		header[0] = stream
		binary.BigEndian.PutUint32(header[4:], uint32(len(chunk[1])))
		out.Write(header)
		out.WriteString(chunk[1])
	}
	return io.NopCloser(&out)
}

func failedInspect(status containertypes.ContainerState, exitCode int) containertypes.InspectResponse {
	inspect := baseInspect()
	inspect.RestartCount = 0
	inspect.State = &containertypes.State{Status: status, ExitCode: exitCode, FinishedAt: "2026-11-13T02:00:00Z", StartedAt: "2026-11-13T01:59:00Z"}
	return inspect
}

func failureLogAgent(enabled bool, tail func(ctx context.Context, id string, lines int) (io.ReadCloser, error)) *Agent {
	return &Agent{
		cfg:    Config{CollectFailureLogs: enabled},
		logger: zerolog.Nop(),
		docker: &fakeDockerClient{containerLogTailFn: tail},
	}
}

// A container has failed when its own state says so: a non-zero exit, a
// memory kill, a restart after either, or a failing health check. A clean
// stop, a clean exit Docker restarts, a healthy container and one that was
// never started have not, and nothing is read for them.
func TestFailureLogReasonNamesOnlyAFailure(t *testing.T) {
	now := time.Date(2026, 11, 13, 2, 5, 0, 0, time.UTC)
	running := func(restarts int, started time.Time, health containertypes.HealthStatus) containertypes.InspectResponse {
		inspect := baseInspect()
		inspect.RestartCount = restarts
		inspect.State = &containertypes.State{Status: containertypes.StateRunning, Running: true, StartedAt: started.Format(time.RFC3339Nano)}
		if health != "" {
			inspect.State.Health = &containertypes.Health{Status: health}
		}
		return inspect
	}
	oom := failedInspect(containertypes.StateExited, 137)
	oom.State.OOMKilled = true
	restarting := failedInspect(containertypes.StateRestarting, 1)
	restarting.RestartCount = 7
	restartingCleanly := failedInspect(containertypes.StateRestarting, 0)
	restartingCleanly.RestartCount = 7
	noState := baseInspect()
	noState.State = nil

	for name, tc := range map[string]struct {
		inspect containertypes.InspectResponse
		want    string
	}{
		"exited with an error":            {failedInspect(containertypes.StateExited, 1), agentsdocker.FailureLogReasonExited},
		"dead":                            {failedInspect(containertypes.StateDead, 2), agentsdocker.FailureLogReasonExited},
		"killed for memory":               {oom, agentsdocker.FailureLogReasonOOMKilled},
		"restarting":                      {restarting, agentsdocker.FailureLogReasonRestarting},
		"restarting after a clean exit":   {restartingCleanly, ""},
		"just restarted by Docker":        {running(3, now.Add(-20*time.Second), ""), ""},
		"just restarted and healthy":      {running(3, now.Add(-20*time.Second), containertypes.Healthy), ""},
		"unhealthy":                       {running(0, now.Add(-time.Hour), containertypes.Unhealthy), agentsdocker.FailureLogReasonUnhealthy},
		"stopped cleanly":                 {failedInspect(containertypes.StateExited, 0), ""},
		"running and healthy":             {running(0, now.Add(-time.Hour), containertypes.Healthy), ""},
		"restarted once, long ago":        {running(1, now.Add(-time.Hour), ""), ""},
		"started a moment ago, by a user": {running(0, now.Add(-10*time.Second), ""), ""},
		"health check still starting":     {running(0, now.Add(-10*time.Second), containertypes.Starting), ""},
		"created, never started":          {failedInspect(containertypes.StateCreated, 0), ""},
		"no state":                        {noState, ""},
	} {
		reason, key := failureLogReason(tc.inspect, now)
		if reason != tc.want || (reason == "") != (key == "") {
			t.Errorf("%s: reason %q key %q, want %q", name, reason, key, tc.want)
		}
	}

	// A new failure of the same container has a new key, so it is read again.
	first := failedInspect(containertypes.StateExited, 1)
	second := failedInspect(containertypes.StateExited, 1)
	second.State.FinishedAt = "2026-11-13T02:04:00Z"
	_, firstKey := failureLogReason(first, now)
	_, secondKey := failureLogReason(second, now)
	if firstKey == secondKey {
		t.Fatalf("two exits share the key %q", firstKey)
	}
}

// Log text is content. Without the operator's switch on this host the agent
// does not even ask the daemon for it.
func TestFailureLogIsNotReadUnlessTheHostTurnedItOn(t *testing.T) {
	calls := 0
	agent := failureLogAgent(false, func(context.Context, string, int) (io.ReadCloser, error) {
		calls++
		return dockerLogFrames([2]string{"stderr", "boom\n"}), nil
	})
	if log := agent.containerFailureLog(context.Background(), "c1", failedInspect(containertypes.StateExited, 1)); log != nil {
		t.Fatalf("a host that did not opt in reported %+v", log)
	}
	if calls != 0 {
		t.Fatalf("the daemon was asked for logs %d times", calls)
	}
}

func TestFailureLogIsMaskedBoundedAndReadOncePerFailure(t *testing.T) {
	calls, asked := 0, 0
	agent := failureLogAgent(true, func(_ context.Context, _ string, lines int) (io.ReadCloser, error) {
		calls++
		asked = lines
		return dockerLogFrames(
			[2]string{"stdout", "[vaultwarden::api][INFO] starting\n"},
			[2]string{"stderr", "\x1b[31m[vaultwarden::db][ERROR]\x1b[0m invalid DATABASE_URL \"10.0.0.5:5432/vault\": relative URL without a base\n"},
			[2]string{"stderr", "retrying with postgres://vault:s3cr3t@10.0.0.5:5432/vault\n\n"},
			[2]string{"stdout", "ADMIN_TOKEN=hunter2 loaded\n"},
		), nil
	})
	inspect := failedInspect(containertypes.StateExited, 1)

	log := agent.containerFailureLog(context.Background(), "c1", inspect)
	if log == nil {
		t.Fatal("a failed container on a host that opted in reported nothing")
	}
	text := strings.Join(log.Lines, "\n")
	for _, secret := range []string{"s3cr3t", "hunter2", "\x1b"} {
		if strings.Contains(text, secret) {
			t.Fatalf("%q left the host: %q", secret, text)
		}
	}
	for _, keep := range []string{"invalid DATABASE_URL", "relative URL without a base", "[vaultwarden::api][INFO] starting"} {
		if !strings.Contains(text, keep) {
			t.Fatalf("%q was lost: %q", keep, text)
		}
	}
	if len(log.Lines) != 4 || log.Reason != agentsdocker.FailureLogReasonExited || log.Redactions < 2 || log.CapturedAt.IsZero() || log.Truncated {
		t.Fatalf("log = %+v", log)
	}
	if asked != agentsdocker.FailureLogMaxLines {
		t.Fatalf("asked the daemon for %d lines, want %d", asked, agentsdocker.FailureLogMaxLines)
	}

	// The next report serves the same failure from memory, as a copy.
	again := agent.containerFailureLog(context.Background(), "c1", inspect)
	if calls != 1 || again == nil || strings.Join(again.Lines, "\n") != text {
		t.Fatalf("the daemon was read %d times for one failure", calls)
	}
	again.Lines[0] = "changed"
	if third := agent.containerFailureLog(context.Background(), "c1", inspect); third.Lines[0] == "changed" {
		t.Fatal("a report shares its lines with the cache")
	}

	// It fails again: read again.
	inspect.State.FinishedAt = "2026-11-13T02:10:00Z"
	agent.failureLogs.prune(map[string]struct{}{"c1": {}})
	if agent.containerFailureLog(context.Background(), "c1", inspect); calls != 2 {
		t.Fatalf("a new failure was not read (calls = %d)", calls)
	}

	// It recovers: nothing is reported and nothing is kept.
	healthy := baseInspect()
	healthy.State = &containertypes.State{Status: containertypes.StateRunning, Running: true, StartedAt: "2026-11-01T00:00:00Z"}
	healthy.RestartCount = 0
	if log := agent.containerFailureLog(context.Background(), "c1", healthy); log != nil {
		t.Fatalf("a running container reported %+v", log)
	}
	if _, held := agent.failureLogs.entries["c1"]; held {
		t.Fatal("lines of a recovered container are still held")
	}
}

func TestFailureLogReadIsBoundedAndHandlesATTY(t *testing.T) {
	// A container with a TTY sends a raw stream, with no frames.
	agent := failureLogAgent(true, func(context.Context, string, int) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("panic: nil map\r\ngoroutine 1 [running]:\r\n")), nil
	})
	inspect := failedInspect(containertypes.StateExited, 2)
	inspect.Config.Tty = true
	log := agent.containerFailureLog(context.Background(), "tty", inspect)
	if log == nil || strings.Join(log.Lines, "|") != "panic: nil map|goroutine 1 [running]:" {
		t.Fatalf("tty log = %+v", log)
	}

	// One endless line: the read stops at its limit and the result says so.
	agent = failureLogAgent(true, func(context.Context, string, int) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Repeat("x", failureLogKeepBytes*3))), nil
	})
	inspect = failedInspect(containertypes.StateExited, 2)
	inspect.Config.Tty = true
	log = agent.containerFailureLog(context.Background(), "huge", inspect)
	if log == nil || !log.Truncated || len(log.Lines) != 1 || len(log.Lines[0]) > agentsdocker.FailureLogMaxLineBytes {
		t.Fatalf("huge log = %+v", log)
	}

	// A container that logged nothing reports nothing, and is not asked again.
	calls := 0
	agent = failureLogAgent(true, func(context.Context, string, int) (io.ReadCloser, error) {
		calls++
		return dockerLogFrames(), nil
	})
	silent := failedInspect(containertypes.StateExited, 1)
	if log := agent.containerFailureLog(context.Background(), "silent", silent); log != nil {
		t.Fatalf("an empty log reported %+v", log)
	}
	if agent.containerFailureLog(context.Background(), "silent", silent); calls != 1 {
		t.Fatalf("an empty log was read %d times", calls)
	}
}

// A restart loop is "running" for a moment on every turn. Lines captured while
// it was failing are held through that moment, and nothing is read in it: a
// running container shows no evidence of a failure by itself.
func TestFailureLogHoldsARestartLoopsLinesWithoutReadingARunningContainer(t *testing.T) {
	calls := 0
	agent := failureLogAgent(true, func(context.Context, string, int) (io.ReadCloser, error) {
		calls++
		return dockerLogFrames([2]string{"stderr", "Error: invalid DATABASE_URL\n"}), nil
	})
	looping := failedInspect(containertypes.StateRestarting, 1)
	looping.RestartCount = 4
	if log := agent.containerFailureLog(context.Background(), "loop", looping); log == nil || calls != 1 {
		t.Fatalf("restarting container: log %+v after %d reads", log, calls)
	}

	between := baseInspect()
	between.RestartCount = 5
	between.State = &containertypes.State{Status: containertypes.StateRunning, Running: true, StartedAt: time.Now().Add(-5 * time.Second).UTC().Format(time.RFC3339Nano)}
	held := agent.containerFailureLog(context.Background(), "loop", between)
	if held == nil || held.Lines[0] != "Error: invalid DATABASE_URL" || calls != 1 {
		t.Fatalf("between two restarts: log %+v after %d reads, want the held lines and no new read", held, calls)
	}

	// A container that was never seen failing is not read just for having
	// been restarted.
	if log := agent.containerFailureLog(context.Background(), "other", between); log != nil || calls != 1 {
		t.Fatalf("a running container was read: %+v after %d reads", log, calls)
	}

	// Once it has stayed up, the lines go.
	between.State.StartedAt = time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339Nano)
	if log := agent.containerFailureLog(context.Background(), "loop", between); log != nil {
		t.Fatalf("a container that stayed up still reports %+v", log)
	}
	if _, kept := agent.failureLogs.entries["loop"]; kept {
		t.Fatal("lines of a recovered container are still held")
	}
}

// Cleaning comes before masking. A secret split by a control character
// matches no masking rule until the character is gone, and cleaning it
// afterwards would send the secret whole.
func TestFailureLogMasksWhatCleaningWouldOtherwiseRebuild(t *testing.T) {
	lines, _, redactions := prepareFailureLogLines("starting\nDB_PASS\x00WORD=hunter2\nAPI_\x1b[0mTOKEN=tok-abc123\npostgres://vault:s3c\x07r3t@db/vault\n")
	text := strings.Join(lines, "\n")
	for _, secret := range []string{"hunter2", "tok-abc123", "s3cr3t"} {
		if strings.Contains(text, secret) {
			t.Fatalf("%q left the host: %q", secret, text)
		}
	}
	if redactions < 3 || !strings.Contains(text, "starting") {
		t.Fatalf("redactions = %d in %q", redactions, text)
	}
}

// The end of the log says why the container failed. When there is more than
// can be kept, the newest text stays, and a frame that claims an enormous
// length is read as far as it goes and no further.
func TestFailureLogStreamKeepsTheNewestAndTrustsNoFrameLength(t *testing.T) {
	var chunks [][2]string
	for i := 0; i < 40; i++ {
		chunks = append(chunks, [2]string{"stdout", strings.Repeat("old line of noise ", 400) + "\n"})
	}
	chunks = append(chunks, [2]string{"stderr", "FATAL: the line that matters\n"})
	text, truncated, err := readFailureLogStream(dockerLogFrames(chunks...), false)
	if err != nil || !truncated || len(text) > failureLogKeepBytes || !strings.HasSuffix(text, "FATAL: the line that matters\n") {
		t.Fatalf("kept %d bytes, truncated=%t, err=%v, ends %q", len(text), truncated, err, text[max(0, len(text)-40):])
	}
	if !strings.HasPrefix(text, "old line of noise ") {
		t.Fatalf("the kept text starts mid-line: %q", text[:40])
	}

	// A header that declares 4 GiB, followed by a few bytes.
	header := []byte{2, 0, 0, 0, 0xff, 0xff, 0xff, 0xff}
	text, _, err = readFailureLogStream(io.NopCloser(io.MultiReader(bytes.NewReader(header), strings.NewReader("short\n"))), false)
	if err != nil || text != "short\n" {
		t.Fatalf("oversized frame: %q, %v", text, err)
	}

	// A runtime that sends a raw stream for a container without a TTY.
	text, _, err = readFailureLogStream(io.NopCloser(strings.NewReader("plain text, no frames at all\n")), false)
	if err != nil || text != "plain text, no frames at all\n" {
		t.Fatalf("raw stream: %q, %v", text, err)
	}
}

// A read that fails is tried again later, not on every collection, and lines
// held for an earlier failure are never passed off as this one's.
func TestFailureLogReadFailureWaitsAndServesNothingStale(t *testing.T) {
	calls, fail := 0, false
	agent := failureLogAgent(true, func(context.Context, string, int) (io.ReadCloser, error) {
		calls++
		if fail {
			return nil, errors.New("daemon is busy")
		}
		return dockerLogFrames([2]string{"stderr", "first failure\n"}), nil
	})
	first := failedInspect(containertypes.StateExited, 1)
	if log := agent.containerFailureLog(context.Background(), "c1", first); log == nil || log.Lines[0] != "first failure" {
		t.Fatalf("first failure: %+v", log)
	}

	fail = true
	second := failedInspect(containertypes.StateExited, 1)
	second.State.FinishedAt = "2026-11-13T03:00:00Z"
	if log := agent.containerFailureLog(context.Background(), "c1", second); log != nil {
		t.Fatalf("a failed read served %+v, the lines of an earlier failure", log)
	}
	agent.failureLogs.prune(map[string]struct{}{"c1": {}})
	if agent.containerFailureLog(context.Background(), "c1", second); calls != 2 {
		t.Fatalf("a failed read was retried at once (%d reads)", calls)
	}

	// Past the wait, it is read again.
	fail = false
	entry := agent.failureLogs.entries["c1"]
	entry.retryAt = time.Now().Add(-time.Second)
	agent.failureLogs.entries["c1"] = entry
	if log := agent.containerFailureLog(context.Background(), "c1", second); log == nil || calls != 3 {
		t.Fatalf("after the wait: %+v, %d reads", log, calls)
	}
}

// When everything fails at once the report must still go out on time: only a
// few logs are read per collection and the rest wait for the next ones.
func TestFailureLogReadsAreLimitedPerCollection(t *testing.T) {
	calls := 0
	agent := failureLogAgent(true, func(context.Context, string, int) (io.ReadCloser, error) {
		calls++
		return dockerLogFrames([2]string{"stderr", "failed\n"}), nil
	})
	active := map[string]struct{}{}
	reported := 0
	for i := 0; i < failureLogReadsPerCycle+4; i++ {
		id := "c" + string(rune('a'+i))
		active[id] = struct{}{}
		if agent.containerFailureLog(context.Background(), id, failedInspect(containertypes.StateExited, 1)) != nil {
			reported++
		}
	}
	if calls != failureLogReadsPerCycle || reported != failureLogReadsPerCycle {
		t.Fatalf("one collection read %d logs and reported %d, want %d", calls, reported, failureLogReadsPerCycle)
	}
	agent.failureLogs.prune(active)
	for id := range active {
		agent.containerFailureLog(context.Background(), id, failedInspect(containertypes.StateExited, 1))
	}
	if calls != failureLogReadsPerCycle+4 {
		t.Fatalf("after the next collection %d logs were read, want %d", calls, failureLogReadsPerCycle+4)
	}

	// A container that is gone is forgotten.
	agent.failureLogs.prune(map[string]struct{}{"ca": {}})
	if len(agent.failureLogs.entries) != 1 {
		t.Fatalf("%d entries held after all but one container went away", len(agent.failureLogs.entries))
	}
}

// The collector attaches the lines to the container it reports.
func TestCollectContainerCarriesTheFailureLogWhenTurnedOn(t *testing.T) {
	inspect := failedInspect(containertypes.StateExited, 1)
	for _, enabled := range []bool{false, true} {
		a := &Agent{
			cfg:              Config{CollectFailureLogs: enabled},
			logger:           zerolog.Nop(),
			runtime:          RuntimeDocker,
			prevContainerCPU: make(map[string]cpuSample),
			docker: &fakeDockerClient{
				containerInspectWithRawFn: func(context.Context, string, bool) (containertypes.InspectResponse, []byte, error) {
					return inspect, nil, nil
				},
				containerLogTailFn: func(context.Context, string, int) (io.ReadCloser, error) {
					return dockerLogFrames([2]string{"stderr", "Error: invalid DATABASE_URL\n"}), nil
				},
			},
		}
		got, err := a.collectContainer(context.Background(), containertypes.Summary{ID: "vault", Names: []string{"/vaultwarden"}, Image: "vaultwarden/server", State: "exited", Status: "Exited (1) 2 minutes ago"})
		if err != nil {
			t.Fatal(err)
		}
		if enabled && (got.FailureLog == nil || len(got.FailureLog.Lines) != 1 || got.FailureLog.Lines[0] != "Error: invalid DATABASE_URL") {
			t.Fatalf("turned on: failure log = %+v", got.FailureLog)
		}
		if !enabled && got.FailureLog != nil {
			t.Fatalf("turned off: failure log = %+v", got.FailureLog)
		}
	}
}
