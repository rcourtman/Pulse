package cloudcp

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultCPRateLimit       = 120
	defaultCPRateLimitWindow = time.Minute
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
		window = defaultCPRateLimitWindow
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
			retryAfter := int(math.Ceil(rl.window.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}

			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	remote := extractRemoteIP(r.RemoteAddr)
	if remote == "" {
		return ""
	}

	if isTrustedProxyIP(remote) {
		if xff := firstValidForwardedIP(r.Header.Get("X-Forwarded-For")); xff != "" {
			return xff
		}

		if realIP := strings.TrimSpace(strings.Trim(r.Header.Get("X-Real-IP"), "[]")); net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	return remote
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.Trim(remoteAddr, "[]")
	}
	return strings.Trim(host, "[]")
}

func firstValidForwardedIP(header string) string {
	for _, part := range strings.Split(header, ",") {
		candidate := strings.TrimSpace(strings.Trim(part, "[]"))
		if net.ParseIP(candidate) != nil {
			return candidate
		}
	}
	return ""
}

func isTrustedProxyIP(rawIP string) bool {
	ip := net.ParseIP(strings.Trim(rawIP, "[]"))
	if ip == nil {
		return false
	}

	// Only trust forwarding headers from local/private upstream peers.
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
