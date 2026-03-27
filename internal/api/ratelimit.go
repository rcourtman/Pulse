package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultRateLimiterLimit  = 60
	defaultRateLimiterWindow = time.Minute
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
	if limit <= 0 {
		log.Warn().
			Int("configured_limit", limit).
			Int("default_limit", defaultRateLimiterLimit).
			Msg("Invalid rate limiter limit; using default")
		limit = defaultRateLimiterLimit
	}
	if window <= 0 {
		log.Warn().
			Dur("configured_window", window).
			Dur("default_window", defaultRateLimiterWindow).
			Msg("Invalid rate limiter window; using default")
		window = defaultRateLimiterWindow
	}

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
	allowed, _ := rl.allowAt(ip, time.Now())
	return allowed
}

// allowAt checks whether a request is allowed at the provided time and reports
// how long the caller should wait before retrying when the limit is exceeded.
func (rl *RateLimiter) allowAt(ip string, now time.Time) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

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
		retryAfter := rl.window
		if len(validAttempts) > 0 {
			retryAfter = validAttempts[0].Add(rl.window).Sub(now)
		}
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	// Add new attempt
	validAttempts = append(validAttempts, now)
	rl.attempts[ip] = validAttempts

	return true, 0
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
