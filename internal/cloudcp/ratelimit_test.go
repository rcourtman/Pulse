package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCPRateLimiterAllow_WithinLimitThenRejects(t *testing.T) {
	rl := NewCPRateLimiter(2, time.Minute)
	ip := "203.0.113.10"

	if !rl.Allow(ip) {
		t.Fatal("expected first request to be allowed")
	}
	if !rl.Allow(ip) {
		t.Fatal("expected second request to be allowed")
	}
	if rl.Allow(ip) {
		t.Fatal("expected third request to be rejected")
	}
}

func TestCPRateLimiterAllow_PrunesExpiredAttempts(t *testing.T) {
	rl := NewCPRateLimiter(1, time.Minute)
	ip := "203.0.113.20"
	rl.attempts[ip] = []time.Time{time.Now().Add(-2 * time.Minute)}

	if !rl.Allow(ip) {
		t.Fatal("expected request to be allowed after expired attempt is pruned")
	}
	if got := len(rl.attempts[ip]); got != 1 {
		t.Fatalf("expected one retained attempt, got %d", got)
	}
}

func TestCPRateLimiterMiddleware_TooManyRequests(t *testing.T) {
	rl := NewCPRateLimiter(1, time.Minute)
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	h := rl.Middleware(next)

	req1 := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", nil)
	req1.RemoteAddr = "198.51.100.5:1234"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusNoContent)
	}
	if calls != 1 {
		t.Fatalf("next handler calls = %d, want 1", calls)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", nil)
	req2.RemoteAddr = "198.51.100.5:1234"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
	if calls != 1 {
		t.Fatalf("next handler calls after reject = %d, want 1", calls)
	}
}

func TestClientIP(t *testing.T) {
	t.Run("x-forwarded-for-single-value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		req.RemoteAddr = "127.0.0.1:9999"

		if got := clientIP(req); got != "203.0.113.9" {
			t.Fatalf("clientIP = %q, want %q", got, "203.0.113.9")
		}
	})

	t.Run("x-forwarded-for-first-value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", " 203.0.113.1 , 10.0.0.1 ")
		req.RemoteAddr = "127.0.0.1:9999"

		if got := clientIP(req); got != "203.0.113.1" {
			t.Fatalf("clientIP = %q, want %q", got, "203.0.113.1")
		}
	})

	t.Run("remote-addr-host-port", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "198.51.100.2:7777"

		if got := clientIP(req); got != "198.51.100.2" {
			t.Fatalf("clientIP = %q, want %q", got, "198.51.100.2")
		}
	})

	t.Run("remote-addr-unparseable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "not-a-host-port"

		if got := clientIP(req); got != "not-a-host-port" {
			t.Fatalf("clientIP = %q, want %q", got, "not-a-host-port")
		}
	})
}
