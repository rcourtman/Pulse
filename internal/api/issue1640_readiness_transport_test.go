package api

// Regression coverage for issue #1640: POST /api/ai/patrol/readiness runs up
// to four sequential provider calls (~45s and more on slow local hardware)
// and previously wrote nothing to the response until the evaluation finished,
// so any reverse proxy with a ~30-second read timeout severed the request.
// The handler now commits a 200 up front and streams JSON-whitespace
// keepalives while the evaluation runs; these tests pin that transport shape.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// firstByte reports the first byte of a body in a form that is safe to print
// when the body is empty, so a transport regression fails the assertion
// instead of panicking inside it.
func firstByte(body string) string {
	if body == "" {
		return "<empty body>"
	}
	return body[:1]
}

func passingReadinessResult(summary string) ai.PatrolModelReadinessResult {
	out := ai.PatrolModelReadinessResult{}
	out.ProbeVersion = ai.PatrolModelReadinessProbeVersion
	out.Status = ai.PatrolModelReadinessPass
	out.Summary = summary
	return out
}

func TestIssue1640KeepalivesFlowWhileEvaluationRuns(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	result := streamPatrolModelReadinessKeepalives(rec, 5*time.Millisecond, func() ai.PatrolModelReadinessResult {
		time.Sleep(60 * time.Millisecond)
		return passingReadinessResult("verified")
	})
	response := patrolModelReadinessSnapshot(&result, time.Now())
	require.NoError(t, utils.WriteJSONResponse(rec, response))

	body := rec.Body.String()
	// Bytes were on the wire before the evaluation completed, and they were
	// flushed so intermediaries actually observed them.
	assert.True(t, strings.HasPrefix(body, "\n"), "expected keepalive padding before the payload, got %q", firstByte(body))
	assert.GreaterOrEqual(t, strings.Count(body, "\n")-strings.Count(strings.TrimLeft(body, "\n"), "\n"), 2,
		"expected multiple keepalives during a slow evaluation")
	assert.True(t, rec.Flushed, "keepalives must be flushed, not buffered")

	// The padding is insignificant JSON whitespace: the final payload still
	// parses as a single ordinary JSON document.
	var snapshot PatrolModelReadinessSnapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snapshot))
	require.NotNil(t, snapshot.PatrolModelReadinessResult)
	assert.Equal(t, "verified", snapshot.Summary)
}

func TestIssue1640FastEvaluationEmitsNoPadding(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	result := streamPatrolModelReadinessKeepalives(rec, time.Hour, func() ai.PatrolModelReadinessResult {
		out := ai.PatrolModelReadinessResult{}
		out.ProbeVersion = ai.PatrolModelReadinessProbeVersion
		return out
	})
	response := patrolModelReadinessSnapshot(&result, time.Now())
	require.NoError(t, utils.WriteJSONResponse(rec, response))

	assert.True(t, strings.HasPrefix(rec.Body.String(), "{"),
		"a fast evaluation should produce a plain JSON body")
}

// The transport must survive a real HTTP connection, not just a recorder: the
// status line has to be committed and flushed before the evaluation starts,
// because a proxy with a time-to-first-byte timeout shorter than the keepalive
// interval would otherwise still sever a slow readiness run.
func TestIssue1640ResponseStartsBeforeEvaluationCompletes(t *testing.T) {
	t.Parallel()

	const evaluationDuration = 300 * time.Millisecond
	completed := make(chan time.Time, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		result := streamPatrolModelReadinessKeepalives(w, 50*time.Millisecond, func() ai.PatrolModelReadinessResult {
			time.Sleep(evaluationDuration)
			completed <- time.Now()
			return passingReadinessResult("verified over the wire")
		})
		response := patrolModelReadinessSnapshot(&result, time.Now())
		if err := utils.WriteJSONResponse(w, response); err != nil {
			t.Errorf("write readiness response: %v", err)
		}
	}))
	defer server.Close()

	started := time.Now()
	resp, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Response headers alone prove the status line was committed early: the
	// client's Get returns as soon as they arrive.
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
	assert.Less(t, time.Since(started), evaluationDuration,
		"headers must reach the client before the evaluation finishes")

	// Read the first body byte under a deadline well short of the evaluation.
	// A buffered or unstarted response cannot satisfy it.
	one := make([]byte, 1)
	readDone := make(chan struct{})
	var (
		firstByteAt time.Time
		readErr     error
	)
	go func() {
		defer close(readDone)
		_, readErr = io.ReadFull(resp.Body, one)
		firstByteAt = time.Now()
	}()
	select {
	case <-readDone:
	case <-time.After(evaluationDuration - 100*time.Millisecond):
		t.Fatal("no body byte arrived before the evaluation completed")
	}
	require.NoError(t, readErr)
	assert.Equal(t, "\n", string(one), "the first body byte should be a keepalive newline")

	evaluationFinishedAt := <-completed
	assert.True(t, firstByteAt.Before(evaluationFinishedAt),
		"keepalive byte at %s must precede evaluation completion at %s", firstByteAt, evaluationFinishedAt)

	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// The padded body is still one ordinary JSON document.
	body := string(one) + string(rest)
	assert.True(t, strings.HasPrefix(body, "\n"), "expected leading keepalive whitespace, got %q", firstByte(body))
	var snapshot PatrolModelReadinessSnapshot
	require.NoError(t, json.Unmarshal([]byte(body), &snapshot))
	require.NotNil(t, snapshot.PatrolModelReadinessResult)
	assert.Equal(t, "verified over the wire", snapshot.Summary)
	assert.Equal(t, ai.PatrolModelReadinessPass, snapshot.Status)
}

// A panic inside provider streaming or validation runs on the evaluation
// goroutine, outside the server's per-request recovery. Unrecovered it takes
// the whole Pulse process down; it must become a failed readiness result.
func TestIssue1640EvaluationPanicBecomesFailedResult(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	result := streamPatrolModelReadinessKeepalives(rec, 5*time.Millisecond, func() ai.PatrolModelReadinessResult {
		panic("provider stream decoder exploded")
	})

	assert.Equal(t, ai.PatrolModelReadinessFail, result.Status)
	assert.Equal(t, ai.PatrolFailureCauseInternalError, result.Cause)
	assert.NotEmpty(t, result.Summary)
	assert.False(t, result.Success)
	assert.False(t, result.PatrolCapable)
	// Nothing was measured, so nothing may be reported as measured.
	assert.Equal(t, ai.PatrolModelReadinessNotAssessed, result.Dimensions.Connectivity.Status)
	assert.Equal(t, ai.PatrolModeNotAssessed, result.Modes.Monitor.Status)

	// The response still completes as a normal 200 JSON document.
	response := patrolModelReadinessSnapshot(&result, time.Now())
	require.NoError(t, utils.WriteJSONResponse(rec, response))
	assert.Equal(t, http.StatusOK, rec.Code)
	var snapshot PatrolModelReadinessSnapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snapshot))
	require.NotNil(t, snapshot.PatrolModelReadinessResult)
	assert.Equal(t, ai.PatrolModelReadinessFail, snapshot.Status)
}

// A writer that cannot flush must not take the readiness run down with it: the
// keepalives degrade to buffered writes, but the response still completes.
func TestIssue1640NonFlushableWriterStillCompletes(t *testing.T) {
	t.Parallel()

	w := newNonFlushingResponseWriter()
	if _, ok := http.ResponseWriter(w).(http.Flusher); ok {
		t.Fatal("test writer must not implement http.Flusher")
	}

	result := streamPatrolModelReadinessKeepalives(w, 5*time.Millisecond, func() ai.PatrolModelReadinessResult {
		time.Sleep(20 * time.Millisecond)
		return passingReadinessResult("verified without a flusher")
	})
	response := patrolModelReadinessSnapshot(&result, time.Now())
	require.NoError(t, utils.WriteJSONResponse(w, response))

	assert.Equal(t, http.StatusOK, w.statusCode)
	var snapshot PatrolModelReadinessSnapshot
	require.NoError(t, json.Unmarshal([]byte(w.body.String()), &snapshot))
	require.NotNil(t, snapshot.PatrolModelReadinessResult)
	assert.Equal(t, "verified without a flusher", snapshot.Summary)
}
