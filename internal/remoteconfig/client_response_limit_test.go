package remoteconfig

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientFetchRejectsUndeclaredOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/agent/agent-1/config" {
			http.NotFound(w, r)
			return
		}
		writeStreamedJSONWithPadding(w,
			`{"success":true,"agentId":"agent-1","config":{"settings":{"interval":"1m"}}}`,
			maxConfigResponseBodyBytes,
		)
	}))
	defer server.Close()

	client := New(Config{
		PulseURL: server.URL,
		APIToken: "token",
		AgentID:  "agent-1",
	})
	defer client.Close()

	_, _, err := client.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("response body exceeds %d bytes", maxConfigResponseBodyBytes)) {
		t.Fatalf("Fetch() error = %v, want response size limit error", err)
	}
}

func TestClientResolveAgentIDRejectsUndeclaredOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != agentLookupPath {
			http.NotFound(w, r)
			return
		}
		writeStreamedJSONWithPadding(w,
			`{"success":true,"agent":{"id":"agent-1"}}`,
			maxAgentLookupResponseBodyBytes,
		)
	}))
	defer server.Close()

	client := New(Config{
		PulseURL: server.URL,
		APIToken: "token",
		Hostname: "node-1",
	})
	defer client.Close()

	_, err := client.resolveAgentID(context.Background())
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("response body exceeds %d bytes", maxAgentLookupResponseBodyBytes)) {
		t.Fatalf("resolveAgentID() error = %v, want response size limit error", err)
	}
}

// writeStreamedJSONWithPadding flushes the valid JSON prefix before writing the
// padding so net/http cannot provide a Content-Length. This exercises the
// streaming-response boundary rather than the declared-size fast path.
func writeStreamedJSONWithPadding(w http.ResponseWriter, payload string, paddingBytes int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(payload)); err != nil {
		return
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	// The client is expected to close the response once the cap is crossed, so
	// a write error here is a successful rejection rather than a server failure.
	_, _ = w.Write([]byte(strings.Repeat(" ", int(paddingBytes))))
}
