package cloudcp

import (
	"testing"
	"time"
)

func TestNewCPRateLimiter_NormalizesInvalidConfig(t *testing.T) {
	rl := NewCPRateLimiter(0, 0)

	if rl.limit != defaultCPRateLimit {
		t.Fatalf("limit = %d, want %d", rl.limit, defaultCPRateLimit)
	}
	if rl.window != defaultCPRateWindow {
		t.Fatalf("window = %v, want %v", rl.window, defaultCPRateWindow)
	}
}

func TestCPRateLimiter_AllowEnforcesLimit(t *testing.T) {
	rl := NewCPRateLimiter(2, time.Hour)

	if !rl.Allow("127.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	if !rl.Allow("127.0.0.1") {
		t.Fatal("second request should be allowed")
	}
	if rl.Allow("127.0.0.1") {
		t.Fatal("third request should be blocked")
	}
}
