package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultTenantRateLimit       = 2000
	defaultTenantRateLimitWindow = time.Minute
)

// TenantRateLimiter applies request limits per organization ID.
type TenantRateLimiter struct {
	limiter *RateLimiter
}

// NewTenantRateLimiter creates a tenant-aware limiter.
// If limit/window are non-positive, safe defaults are used.
func NewTenantRateLimiter(limit int, window time.Duration) *TenantRateLimiter {
	if limit <= 0 {
		limit = defaultTenantRateLimit
	}
	if window <= 0 {
		window = defaultTenantRateLimitWindow
	}

	return &TenantRateLimiter{
		limiter: NewRateLimiter(limit, window),
	}
}

// Stop stops the underlying limiter cleanup routine.
func (t *TenantRateLimiter) Stop() {
	if t == nil || t.limiter == nil {
		return
	}
	t.limiter.Stop()
}

// Allow checks whether the org is within the configured request budget.
func (t *TenantRateLimiter) Allow(orgID string) bool {
	if t == nil || t.limiter == nil {
		return true
	}
	if orgID == "" {
		orgID = "default"
	}
	return t.limiter.Allow(orgID)
}

// TenantRateLimitMiddleware enforces per-org limits after tenant resolution.
func TenantRateLimitMiddleware(trl *TenantRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trl == nil || trl.limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			orgID := GetOrgID(r.Context())
			if orgID == "default" {
				next.ServeHTTP(w, r)
				return
			}

			if !trl.Allow(orgID) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(int(trl.limiter.window.Seconds())))
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(trl.limiter.limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-Pulse-Org-ID", orgID)
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "tenant_rate_limit_exceeded",
					"message": "Organization rate limit exceeded",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
