package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryEndpointRejectsForwardedLoopbackFromUntrustedProxy(t *testing.T) {
	router := newRecoveryRouter(t)

	// No trusted proxies configured; forwarded loopback should be rejected.
	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"status"}`))
	req.RemoteAddr = "203.0.113.77:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for forwarded loopback from untrusted proxy, got %d", rec.Code)
	}
}
