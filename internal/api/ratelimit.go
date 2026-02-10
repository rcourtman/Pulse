package api

import (
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	attempts    map[string][]time.Time
	mu          sync.RWMutex
	limit       int
	window      time.Duration
	stopCleanup chan struct{}
}

// NewRateLimiter creates a rate limiter that allows limit requests per window duration.
// It starts a background goroutine to periodically clean up old entries.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts:    make(map[string][]time.Time),
		limit:       limit,
		window:      window,
		stopCleanup: make(chan struct{}),
	}

	// Clean up old entries periodically
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-rl.stopCleanup:
				return
			}
		}
	}()

	return rl
}

// Stop stops the cleanup routine
func (rl *RateLimiter) Stop() {
	select {
	case <-rl.stopCleanup:
		return
	default:
		close(rl.stopCleanup)
	}
}

// Allow checks if a request from the given IP address is within the rate limit.
// Returns true if the request is allowed, false if the rate limit is exceeded.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get attempts for this IP
	attempts := rl.attempts[ip]

	// Filter out old attempts
	var validAttempts []time.Time
	for _, attempt := range attempts {
		if attempt.After(cutoff) {
			validAttempts = append(validAttempts, attempt)
		}
	}

	// Check if under limit
	if len(validAttempts) >= rl.limit {
		rl.attempts[ip] = validAttempts
		return false
	}

	// Add new attempt
	validAttempts = append(validAttempts, now)
	rl.attempts[ip] = validAttempts

	return true
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)

	for ip, attempts := range rl.attempts {
		var validAttempts []time.Time
		for _, attempt := range attempts {
			if attempt.After(cutoff) {
				validAttempts = append(validAttempts, attempt)
			}
		}

		if len(validAttempts) == 0 {
			delete(rl.attempts, ip)
		} else {
			rl.attempts[ip] = validAttempts
		}
	}
}

// Middleware for rate limiting
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)
		if ip == "" {
			ip = extractRemoteIP(r.RemoteAddr)
		}
		if ip == "" {
			ip = r.RemoteAddr
		}

		if !rl.Allow(ip) {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
