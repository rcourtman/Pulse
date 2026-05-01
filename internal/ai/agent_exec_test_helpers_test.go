package ai

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func agentExecWSURLForHTTP(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http")
}

func agentExecWSHeadersForHTTP(t *testing.T, serverURL string) http.Header {
	t.Helper()

	origin, err := securityutil.HTTPOriginForWebSocketBaseURL(serverURL)
	if err != nil {
		t.Fatalf("failed to derive websocket origin: %v", err)
	}

	headers := http.Header{}
	headers.Set("Origin", origin)
	return headers
}
