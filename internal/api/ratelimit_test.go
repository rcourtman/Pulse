package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	defer rl.Stop()

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limit != 5 {
		t.Errorf("limit = %d, want 5", rl.limit)
	}
	if rl.window != time.Minute {
		t.Errorf("window = %v, want %v", rl.window, time.Minute)
	}
	if rl.attempts == nil {
		t.Error("attempts map is nil")
	}
}

func TestRateLimiter_Allow_Basic(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	defer rl.Stop()

	ip := "192.168.1.1"

	// First 3 attempts should be allowed
	for i := 1; i <= 3; i++ {
		if !rl.Allow(ip) {
			t.Errorf("attempt %d should be allowed", i)
		}
	}

	// 4th attempt should be denied
	if rl.Allow(ip) {
		t.Error("4th attempt should be denied")
	}

	// 5th attempt should also be denied
	if rl.Allow(ip) {
		t.Error("5th attempt should be denied")
	}
}

func TestRateLimiter_Allow_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	defer rl.Stop()

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// Both IPs should have independent limits
	if !rl.Allow(ip1) {
		t.Error("ip1 attempt 1 should be allowed")
	}
	if !rl.Allow(ip2) {
		t.Error("ip2 attempt 1 should be allowed")
	}
	if !rl.Allow(ip1) {
		t.Error("ip1 attempt 2 should be allowed")
	}
	if !rl.Allow(ip2) {
		t.Error("ip2 attempt 2 should be allowed")
	}

	// Both should now be at their limit
	if rl.Allow(ip1) {
		t.Error("ip1 attempt 3 should be denied")
	}
	if rl.Allow(ip2) {
		t.Error("ip2 attempt 3 should be denied")
	}
}

func TestRateLimiter_Allow_WindowExpiry(t *testing.T) {
	// Use a very short window for testing
	rl := NewRateLimiter(2, 50*time.Millisecond)
	defer rl.Stop()

	ip := "192.168.1.1"

	// Use up the limit
	if !rl.Allow(ip) {
		t.Error("attempt 1 should be allowed")
	}
	if !rl.Allow(ip) {
		t.Error("attempt 2 should be allowed")
	}
	if rl.Allow(ip) {
		t.Error("attempt 3 should be denied")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow(ip) {
		t.Error("attempt after window expiry should be allowed")
	}
}

func TestRateLimiter_Allow_SlidingWindow(t *testing.T) {
	// Test that the window is truly sliding (not fixed intervals)
	rl := NewRateLimiter(2, 100*time.Millisecond)
	defer rl.Stop()

	ip := "192.168.1.1"

	// First attempt
	if !rl.Allow(ip) {
		t.Error("attempt 1 should be allowed")
	}

	// Wait half the window
	time.Sleep(60 * time.Millisecond)

	// Second attempt
	if !rl.Allow(ip) {
		t.Error("attempt 2 should be allowed")
	}

	// Third attempt should be denied (both still in window)
	if rl.Allow(ip) {
		t.Error("attempt 3 should be denied")
	}

	// Wait for first attempt to expire (another 50ms should be enough)
	time.Sleep(50 * time.Millisecond)

	// Now should be allowed (first attempt expired, second still valid)
	if !rl.Allow(ip) {
		t.Error("attempt 4 should be allowed after first expires")
	}
}

func TestRateLimiter_Allow_ZeroLimit(t *testing.T) {
	rl := NewRateLimiter(0, time.Minute)
	defer rl.Stop()

	// Non-positive limits should default to safe values.
	if rl.limit != defaultRateLimiterLimit {
		t.Errorf("limit = %d, want %d", rl.limit, defaultRateLimiterLimit)
	}

	if !rl.Allow("192.168.1.1") {
		t.Error("request should be allowed after defaulting zero limit")
	}
}

func TestRateLimiter_Allow_LargeLimit(t *testing.T) {
	rl := NewRateLimiter(1000, time.Minute)
	defer rl.Stop()

	ip := "192.168.1.1"

	// All 1000 attempts should be allowed
	for i := 1; i <= 1000; i++ {
		if !rl.Allow(ip) {
			t.Errorf("attempt %d should be allowed", i)
		}
	}

	// 1001st should be denied
	if rl.Allow(ip) {
		t.Error("attempt 1001 should be denied")
	}
}

func TestRateLimiter_Allow_EmptyIP(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	defer rl.Stop()

	// Empty string IP should still work as a valid key
	if !rl.Allow("") {
		t.Error("empty IP attempt 1 should be allowed")
	}
	if !rl.Allow("") {
		t.Error("empty IP attempt 2 should be allowed")
	}
	if rl.Allow("") {
		t.Error("empty IP attempt 3 should be denied")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(5, 50*time.Millisecond)
	defer rl.Stop()

	// Add some attempts
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")
	rl.Allow("192.168.1.3")

	// Verify attempts are tracked
	rl.mu.RLock()
	beforeCount := len(rl.attempts)
	rl.mu.RUnlock()

	if beforeCount != 3 {
		t.Errorf("expected 3 IPs tracked, got %d", beforeCount)
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Manually trigger cleanup
	rl.cleanup()

	// All should be cleaned up
	rl.mu.RLock()
	afterCount := len(rl.attempts)
	rl.mu.RUnlock()

	if afterCount != 0 {
		t.Errorf("expected 0 IPs after cleanup, got %d", afterCount)
	}
}

func TestRateLimiter_Cleanup_PartialExpiry(t *testing.T) {
	rl := NewRateLimiter(5, 50*time.Millisecond)
	defer rl.Stop()

	// Add attempt for IP1
	rl.Allow("192.168.1.1")

	// Wait for it to expire
	time.Sleep(60 * time.Millisecond)

	// Add attempt for IP2 (this one is fresh)
	rl.Allow("192.168.1.2")

	// Cleanup should remove IP1 but keep IP2
	rl.cleanup()

	rl.mu.RLock()
	_, hasIP1 := rl.attempts["192.168.1.1"]
	_, hasIP2 := rl.attempts["192.168.1.2"]
	rl.mu.RUnlock()

	if hasIP1 {
		t.Error("IP1 should have been cleaned up")
	}
	if !hasIP2 {
		t.Error("IP2 should still be present")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(100, time.Minute)
	defer rl.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	attemptsPerGoroutine := 20

	// Track results per goroutine
	results := make([][]bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		results[i] = make([]bool, attemptsPerGoroutine)
	}

	// Concurrent access from same IP
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < attemptsPerGoroutine; j++ {
				results[idx][j] = rl.Allow("shared-ip")
			}
		}(i)
	}

	wg.Wait()

	// Count allowed attempts
	allowed := 0
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < attemptsPerGoroutine; j++ {
			if results[i][j] {
				allowed++
			}
		}
	}

	// Should have exactly 100 allowed (the limit)
	if allowed != 100 {
		t.Errorf("expected exactly 100 allowed attempts, got %d", allowed)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	// Should be able to stop without issues
	rl.Stop()

	// After stop, Allow should still work (stop only affects cleanup goroutine)
	if !rl.Allow("192.168.1.1") {
		t.Error("Allow should still work after Stop")
	}
}

func TestRateLimiter_Middleware_Allowed(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	defer rl.Stop()

	handlerCalled := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrapped := rl.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrapped(w, req)

	if !handlerCalled {
		t.Error("handler should have been called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_Middleware_Denied(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	handlerCalls := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
	}

	wrapped := rl.Middleware(handler)

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	wrapped(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	w2 := httptest.NewRecorder()
	wrapped(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}

	if handlerCalls != 1 {
		t.Errorf("handler called %d times, want 1", handlerCalls)
	}
}

func TestRateLimiter_Middleware_XForwardedFor(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	resetTrustedProxyConfig()

	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	handlerCalls := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
	}

	wrapped := rl.Middleware(handler)

	// Request with X-Forwarded-For header
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "127.0.0.1:12345"
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	w1 := httptest.NewRecorder()
	wrapped(w1, req1)

	// Second request with same X-Forwarded-For should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Forwarded-For", "203.0.113.1")
	w2 := httptest.NewRecorder()
	wrapped(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request with same X-Forwarded-For should be rate limited, got %d", w2.Code)
	}

	// Request with different X-Forwarded-For should be allowed
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "127.0.0.1:12345"
	req3.Header.Set("X-Forwarded-For", "203.0.113.2")
	w3 := httptest.NewRecorder()
	wrapped(w3, req3)

	if w3.Code == http.StatusTooManyRequests {
		t.Error("request with different X-Forwarded-For should be allowed")
	}

	if handlerCalls != 2 {
		t.Errorf("handler called %d times, want 2", handlerCalls)
	}
}

func TestRateLimiter_Middleware_ResponseBody(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	handler := func(w http.ResponseWriter, r *http.Request) {}
	wrapped := rl.Middleware(handler)

	// First request consumes the single allowed slot.
	firstReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	firstReq.RemoteAddr = "192.168.1.1:12345"
	firstResp := httptest.NewRecorder()
	wrapped(firstResp, firstReq)

	if firstResp.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", firstResp.Code, http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	wrapped(w, req)

	body := w.Body.String()
	expected := "Rate limit exceeded. Please try again later.\n"
	if body != expected {
		t.Errorf("body = %q, want %q", body, expected)
	}
}

func TestNewRateLimiter_NormalizesInvalidConfig(t *testing.T) {
	rl := NewRateLimiter(-10, -5*time.Second)
	defer rl.Stop()

	if rl.limit != defaultRateLimiterLimit {
		t.Errorf("limit = %d, want %d", rl.limit, defaultRateLimiterLimit)
	}
	if rl.window != defaultRateLimiterWindow {
		t.Errorf("window = %v, want %v", rl.window, defaultRateLimiterWindow)
	}
}
