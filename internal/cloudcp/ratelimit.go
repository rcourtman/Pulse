package cloudcp

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultCPRateLimit  = 120
	defaultCPRateWindow = time.Minute
)

// CPRateLimiter provides simple IP-based rate limiting for control plane endpoints.
type CPRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewCPRateLimiter creates a rate limiter with the given limit per window.
func NewCPRateLimiter(limit int, window time.Duration) *CPRateLimiter {
	if limit <= 0 {
		limit = defaultCPRateLimit
	}
	if window <= 0 {
		window = defaultCPRateWindow
	}
	return &CPRateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks whether the given IP is within the rate limit.
func (rl *CPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter expired entries
	valid := rl.attempts[ip][:0]
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.attempts[ip] = valid
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

// Middleware wraps an http.Handler with rate limiting.
func (rl *CPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.Allow(ip) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		// Use the first IP in the chain.
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
