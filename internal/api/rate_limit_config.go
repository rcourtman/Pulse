package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EndpointRateLimitConfig defines rate limiting configuration for different endpoint categories
type EndpointRateLimitConfig struct {
	AuthEndpoints      *RateLimiter // Login, logout, password change
	ConfigEndpoints    *RateLimiter // Node configuration changes
	ExportEndpoints    *RateLimiter // Export/import operations
	RecoveryEndpoints  *RateLimiter // Recovery operations
	UpdateEndpoints    *RateLimiter // Update checks and operations
	WebSocketEndpoints *RateLimiter // WebSocket connections
	GeneralAPI         *RateLimiter // General API calls
	PublicEndpoints    *RateLimiter // Public endpoints (health, version)
}

var globalRateLimitConfig *EndpointRateLimitConfig

// InitializeRateLimiters sets up rate limiters for all endpoint categories
func InitializeRateLimiters() {
	globalRateLimitConfig = &EndpointRateLimitConfig{
		// Authentication endpoints: strict limits to prevent brute force
		AuthEndpoints: NewRateLimiter(10, 1*time.Minute), // 10 attempts per minute

		// Configuration changes: moderate limits
		ConfigEndpoints: NewRateLimiter(30, 1*time.Minute), // 30 changes per minute

		// Export/import: very strict limits
		ExportEndpoints: NewRateLimiter(5, 5*time.Minute), // 5 exports per 5 minutes

		// Recovery operations: extremely strict
		RecoveryEndpoints: NewRateLimiter(3, 10*time.Minute), // 3 attempts per 10 minutes

		// Update operations: allow frequent polling during updates
		UpdateEndpoints: NewRateLimiter(60, 1*time.Minute), // 60 checks per minute (modal polls every 2s)

		// WebSocket connections: per-connection limits
		WebSocketEndpoints: NewRateLimiter(30, 1*time.Minute), // 30 new connections per minute

		// General API: higher limits for normal operations
		GeneralAPI: NewRateLimiter(500, 1*time.Minute), // 500 requests per minute

		// Public endpoints: very high limits (health checks, etc.)
		PublicEndpoints: NewRateLimiter(1000, 1*time.Minute), // 1000 requests per minute
	}
}

// GetRateLimiterForEndpoint returns the appropriate rate limiter for a given endpoint
func GetRateLimiterForEndpoint(path string, method string) *RateLimiter {
	if globalRateLimitConfig == nil {
		InitializeRateLimiters()
	}

	// Normalize path
	path = strings.ToLower(path)

	// Authentication endpoints
	if strings.Contains(path, "/api/login") ||
		strings.Contains(path, "/api/logout") ||
		strings.Contains(path, "/api/security/change-password") ||
		strings.Contains(path, "/api/auth") {
		return globalRateLimitConfig.AuthEndpoints
	}

	// Recovery endpoints
	if strings.Contains(path, "/api/security/recovery") {
		return globalRateLimitConfig.RecoveryEndpoints
	}

	// Export/Import endpoints
	if strings.Contains(path, "/api/config/export") ||
		strings.Contains(path, "/api/config/import") {
		return globalRateLimitConfig.ExportEndpoints
	}

	// Configuration endpoints (write operations only)
	if method != "GET" && (strings.Contains(path, "/api/config/nodes") ||
		strings.Contains(path, "/api/config/system") ||
		strings.Contains(path, "/api/config/webhooks") ||
		strings.Contains(path, "/api/config/alerts")) {
		return globalRateLimitConfig.ConfigEndpoints
	}

	// Configuration read endpoints get higher limits to prevent UI issues
	if method == "GET" && (strings.Contains(path, "/api/config/") ||
		strings.Contains(path, "/api/discover") ||
		strings.Contains(path, "/api/security/status")) {
		return globalRateLimitConfig.PublicEndpoints // Use higher limit for reads
	}

	// Update endpoints
	if strings.Contains(path, "/api/updates") {
		return globalRateLimitConfig.UpdateEndpoints
	}

	// WebSocket endpoints
	if strings.Contains(path, "/ws") {
		return globalRateLimitConfig.WebSocketEndpoints
	}

	// Public endpoints (no auth required)
	if strings.Contains(path, "/api/health") ||
		strings.Contains(path, "/api/version") ||
		strings.Contains(path, "/api/security/status") ||
		strings.Contains(path, "/api/security/validate-bootstrap-token") ||
		strings.Contains(path, "/api/temperature-proxy/authorized-nodes") ||
		strings.Contains(path, "/metrics") {
		return globalRateLimitConfig.PublicEndpoints
	}

	// Default to general API rate limiter
	return globalRateLimitConfig.GeneralAPI
}

// UniversalRateLimitMiddleware applies appropriate rate limiting to all endpoints
func UniversalRateLimitMiddleware(next http.Handler) http.Handler {
	// Initialize rate limiters if not already done
	if globalRateLimitConfig == nil {
		InitializeRateLimiters()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for static assets
		if !strings.HasPrefix(r.URL.Path, "/api") && !strings.HasPrefix(r.URL.Path, "/ws") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip rate limiting for real-time data endpoints that are polled frequently
		// These endpoints are essential for UI functionality and should not be rate limited
		skipPaths := []string{
			"/api/state",           // Real-time state updates
			"/api/guests/metadata", // Guest metadata (polled frequently)
		}
		for _, path := range skipPaths {
			if strings.Contains(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract client IP
		ip := GetClientIP(r)

		// Skip rate limiting only for direct loopback requests (no proxy headers)
		if (ip == "127.0.0.1" || ip == "::1" || ip == "localhost") && isDirectLoopbackRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Get appropriate rate limiter for this endpoint
		limiter := GetRateLimiterForEndpoint(r.URL.Path, r.Method)

		// Check rate limit
		if !limiter.Allow(ip) {
			// Add retry-after header matching the limiter's actual window
			w.Header().Set("Retry-After", strconv.Itoa(int(limiter.window.Seconds())))
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.limit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", time.Now().Add(limiter.window).Format(time.RFC3339))

			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// ResetRateLimitForIP resets rate limit counters for a specific IP (use carefully)
func ResetRateLimitForIP(ip string) {
	if globalRateLimitConfig == nil {
		return
	}

	// Reset for all rate limiters
	limiters := []*RateLimiter{
		globalRateLimitConfig.AuthEndpoints,
		globalRateLimitConfig.ConfigEndpoints,
		globalRateLimitConfig.ExportEndpoints,
		globalRateLimitConfig.RecoveryEndpoints,
		globalRateLimitConfig.UpdateEndpoints,
		globalRateLimitConfig.WebSocketEndpoints,
		globalRateLimitConfig.GeneralAPI,
		globalRateLimitConfig.PublicEndpoints,
	}

	for _, limiter := range limiters {
		limiter.Reset(ip)
	}
}

// Reset clears rate limit history for a specific IP
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}
