package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Exercise actual execute/Kubernetes handlers, a held local text-only provider
// and the full Router. Markup must round-trip as data, not escaped twice or
// rendered as an HTML response. This is not a private-binary/licence proof.
func assistantJSONMediaReply(t *testing.T, kind, transport string) (http.Header, int, []byte) {
	t.Helper()
	f := newAssistantJSONFixture(t, kind, []string{config.ScopeAIExecute})
	f.primary.result = responseMediaMarkup
	raw := f.token
	if transport == "session" {
		raw = "synthetic-assistant-media-session"
		GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
		t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
	}
	result := f.begin(t, "primary", raw, transport)
	f.primary.unblock()
	var got assistantJSONResult
	select {
	case got = <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("JSON media response did not finish")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	defer got.response.Body.Close()
	body, err := io.ReadAll(got.response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var value AIExecuteResponse
	if got.response.StatusCode != http.StatusOK || json.Unmarshal(body, &value) != nil || value.Content != responseMediaMarkup {
		t.Fatalf("actual %s response did not round-trip: %d %q", kind, got.response.StatusCode, body)
	}
	if got.response.Header.Get("Content-Type") != "application/json" || got.response.Header.Get("X-Content-Type-Options") != "nosniff" || strings.Contains(string(body), "<script>") {
		t.Fatalf("actual %s response not safe as data: %v %q", kind, got.response.Header, body)
	}
	return got.response.Header, got.response.StatusCode, body
}
