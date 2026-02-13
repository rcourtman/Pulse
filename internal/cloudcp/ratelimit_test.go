package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewCPRateLimiter_InvalidConfigUsesDefaults(t *testing.T) {
	rl := NewCPRateLimiter(0, 0)
	if rl.limit != defaultCPRateLimit {
		t.Fatalf("limit = %d, want %d", rl.limit, defaultCPRateLimit)
	}
	if rl.window != defaultCPRateLimitWindow {
		t.Fatalf("window = %s, want %s", rl.window, defaultCPRateLimitWindow)
	}
}

func TestCPRateLimiterMiddleware_SetsRetryAfterOn429(t *testing.T) {
	rl := NewCPRateLimiter(1, time.Minute)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/api/stripe/webhook", nil)
	req1.RemoteAddr = "198.51.100.10:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request code = %d, want %d", rr1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/stripe/webhook", nil)
	req2.RemoteAddr = "198.51.100.10:9999"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request code = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header should be set on 429 responses")
	}
	if rr2.Header().Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("X-RateLimit-Limit = %q, want %q", rr2.Header().Get("X-RateLimit-Limit"), "1")
	}
}

func TestClientIP_DoesNotTrustForwardedForFromUntrustedPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.20:1111"
	req.Header.Set("X-Forwarded-For", "203.0.113.40")

	if got := clientIP(req); got != "198.51.100.20" {
		t.Fatalf("clientIP() = %q, want %q", got, "198.51.100.20")
	}
}

func TestClientIP_UsesForwardedForFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1111"
	req.Header.Set("X-Forwarded-For", "203.0.113.40, 10.0.0.5")

	if got := clientIP(req); got != "203.0.113.40" {
		t.Fatalf("clientIP() = %q, want %q", got, "203.0.113.40")
	}
}
