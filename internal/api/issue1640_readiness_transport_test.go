package api

// Regression coverage for issue #1640: POST /api/ai/patrol/readiness runs up
// to four sequential provider calls (~45s and more on slow local hardware)
// and previously wrote nothing to the response until the evaluation finished,
// so any reverse proxy with a ~30-second read timeout severed the request.
// The handler now streams JSON-whitespace keepalives while the evaluation
// runs; these tests pin that transport shape.

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue1640KeepalivesFlowWhileEvaluationRuns(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	result := streamPatrolModelReadinessKeepalives(rec, 5*time.Millisecond, func() ai.PatrolModelReadinessResult {
		time.Sleep(60 * time.Millisecond)
		out := ai.PatrolModelReadinessResult{}
		out.ProbeVersion = ai.PatrolModelReadinessProbeVersion
		out.Status = ai.PatrolModelReadinessPass
		out.Summary = "verified"
		return out
	})
	response := patrolModelReadinessSnapshot(&result, time.Now())
	require.NoError(t, utils.WriteJSONResponse(rec, response))

	body := rec.Body.String()
	// Bytes were on the wire before the evaluation completed, and they were
	// flushed so intermediaries actually observed them.
	assert.True(t, strings.HasPrefix(body, "\n"), "expected keepalive padding before the payload, got %q", body[:1])
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

// The handler must actually route through the keepalive transport and commit
// proxy-friendly headers before the first byte. Mirrors the source-shape
// assertion style used for the preflight deadline delegation.
func TestIssue1640HandlerUsesKeepaliveTransport(t *testing.T) {
	source, err := os.ReadFile("ai_handlers.go")
	require.NoError(t, err)
	text := string(source)
	start := strings.Index(text, "func (h *AISettingsHandler) HandlePatrolModelReadiness(")
	require.NotEqual(t, -1, start)
	end := strings.Index(text[start:], "\nfunc ")
	if end < 0 {
		end = len(text) - start
	}
	body := text[start : start+end]
	require.Contains(t, body, "streamPatrolModelReadinessKeepalives(")
	require.Contains(t, body, `w.Header().Set("X-Accel-Buffering", "no")`)
	require.Contains(t, body, "patrolModelReadinessKeepaliveInterval")
}
