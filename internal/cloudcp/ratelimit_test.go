package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/proxytrust"
)

func resetTrustedProxyConfig() {
	proxytrust.ResetForTesting()
}

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

// A client behind an appending proxy can send its own X-Forwarded-For line or
// entries; rotating them must not move it into a fresh rate-limit bucket.
func TestCPRateLimiterMiddleware_SpoofedForwardedForSharesBucket(t *testing.T) {
	t.Setenv("CP_TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	resetTrustedProxyConfig()
	t.Cleanup(resetTrustedProxyConfig)

	cases := []struct {
		name   string
		first  []string
		second []string
	}{
		{"client header line before proxy line", []string{"198.51.100.1", "203.0.113.50"}, []string{"198.51.100.2", "203.0.113.50"}},
		{"client entry before host:port hop", []string{"198.51.100.1, 203.0.113.50:5555"}, []string{"198.51.100.2, 203.0.113.50:6666"}},
		{"client entry before unreadable hop", []string{"198.51.100.1, unknown"}, []string{"198.51.100.2, unknown"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rl := NewCPRateLimiter(1, time.Minute)
			h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			send := func(values []string) int {
				req := httptest.NewRequest(http.MethodPost, "/auth/magic-link/request", nil)
				req.RemoteAddr = "10.0.0.2:4444"
				req.Header["X-Forwarded-For"] = values
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				return rec.Code
			}
			if code := send(tc.first); code != http.StatusNoContent {
				t.Fatalf("first request status = %d, want %d", code, http.StatusNoContent)
			}
			if code := send(tc.second); code != http.StatusTooManyRequests {
				t.Fatalf("rotated client claim status = %d, want %d", code, http.StatusTooManyRequests)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	t.Run("x-forwarded-for-single-value", func(t *testing.T) {
		t.Setenv("CP_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
		resetTrustedProxyConfig()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.9")
		req.RemoteAddr = "127.0.0.1:9999"

		if got := clientIP(req); got != "203.0.113.9" {
			t.Fatalf("clientIP = %q, want %q", got, "203.0.113.9")
		}
	})

	t.Run("x-forwarded-for-right-most-untrusted-hop", func(t *testing.T) {
		t.Setenv("CP_TRUSTED_PROXY_CIDRS", "127.0.0.1/32,10.0.0.0/8")
		resetTrustedProxyConfig()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", " 203.0.113.1 , 10.0.0.1 ")
		req.RemoteAddr = "127.0.0.1:9999"

		if got := clientIP(req); got != "203.0.113.1" {
			t.Fatalf("clientIP = %q, want %q", got, "203.0.113.1")
		}
	})

	t.Run("remote-addr-host-port", func(t *testing.T) {
		t.Setenv("CP_TRUSTED_PROXY_CIDRS", "")
		resetTrustedProxyConfig()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "198.51.100.2:7777"

		if got := clientIP(req); got != "198.51.100.2" {
			t.Fatalf("clientIP = %q, want %q", got, "198.51.100.2")
		}
	})

	t.Run("remote-addr-unparseable", func(t *testing.T) {
		t.Setenv("CP_TRUSTED_PROXY_CIDRS", "")
		resetTrustedProxyConfig()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "not-a-host-port"

		if got := clientIP(req); got != "not-a-host-port" {
			t.Fatalf("clientIP = %q, want %q", got, "not-a-host-port")
		}
	})

	t.Run("untrusted-forwarded-for-ignored", func(t *testing.T) {
		t.Setenv("CP_TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
		resetTrustedProxyConfig()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.77")
		req.RemoteAddr = "198.51.100.20:7777"

		if got := clientIP(req); got != "198.51.100.20" {
			t.Fatalf("clientIP = %q, want %q", got, "198.51.100.20")
		}
	})
}
