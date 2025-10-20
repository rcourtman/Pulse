package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestUniversalRateLimitMiddleware_HeaderFormat(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := UniversalRateLimitMiddleware(handler)

	limiter := GetRateLimiterForEndpoint("/api/login", http.MethodPost)
	testIP := "203.0.113.55"
	t.Cleanup(func() {
		ResetRateLimitForIP(testIP)
	})

	makeRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		req.RemoteAddr = testIP + ":12345"
		return req
	}

	// Make requests up to the limit - all should succeed
	for i := 0; i < limiter.limit; i++ {
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, makeRequest())
		if rr.Code != http.StatusOK {
			t.Fatalf("expected OK while under limit, got %d on request %d", rr.Code, i+1)
		}
	}

	// Next request should trigger rate limit
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, makeRequest())
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate-limit status, got %d", rr.Code)
	}

	// Verify X-RateLimit-Limit header is a proper decimal string
	limitHeader := rr.Result().Header.Get("X-RateLimit-Limit")
	expected := strconv.Itoa(limiter.limit)
	if limitHeader != expected {
		t.Fatalf("expected X-RateLimit-Limit %q, got %q", expected, limitHeader)
	}

	// Verify the header parses as a valid integer (not a control character)
	if _, err := strconv.Atoi(limitHeader); err != nil {
		t.Fatalf("header %q should parse as decimal: %v", limitHeader, err)
	}
}
