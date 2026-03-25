package cloudcp

import (
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultCPRateLimit  = 120
	defaultCPRateWindow = time.Minute
)

var (
	trustedProxyOnce  sync.Once
	trustedProxyCIDRs []*net.IPNet
)

// CPRateLimiter provides simple IP-based rate limiting for control plane endpoints.
type CPRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

type CPRateLimitRejectedHandler func(w http.ResponseWriter, r *http.Request, retryAfter int)

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
	allowed, _ := rl.allowAt(ip, time.Now())
	return allowed
}

// allowAt checks whether the given IP is within the rate limit at the provided
// time and reports how long the caller should wait before retrying when
// blocked.
func (rl *CPRateLimiter) allowAt(ip string, now time.Time) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

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
		retryAfter := rl.window
		if len(valid) > 0 {
			retryAfter = valid[0].Add(rl.window).Sub(now)
		}
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	rl.attempts[ip] = append(valid, now)
	return true, 0
}

// Middleware wraps an http.Handler with rate limiting.
func (rl *CPRateLimiter) Middleware(next http.Handler) http.Handler {
	return rl.MiddlewareWithRejected(next, nil)
}

// MiddlewareWithRejected wraps an http.Handler with rate limiting and allows
// callers to provide a custom blocked-response handler for customer-facing
// flows such as hosted trial signup.
func (rl *CPRateLimiter) MiddlewareWithRejected(next http.Handler, rejected CPRateLimitRejectedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if allowed, retryDelay := rl.allowAt(ip, time.Now()); !allowed {
			retryAfter := int(math.Ceil(retryDelay.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}

			rl.writeRateLimitHeaders(w, retryAfter)
			if rejected != nil {
				rejected(w, r, retryAfter)
				return
			}
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *CPRateLimiter) writeRateLimitHeaders(w http.ResponseWriter, retryAfter int) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
	w.Header().Set("X-RateLimit-Remaining", "0")
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

	trustedProxyOnce.Do(loadTrustedProxyCIDRs)
	if len(trustedProxyCIDRs) == 0 {
		return false
	}
	for _, network := range trustedProxyCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func loadTrustedProxyCIDRs() {
	raw := strings.TrimSpace(os.Getenv("CP_TRUSTED_PROXY_CIDRS"))
	if raw == "" {
		// Backward-compatible fallback to the shared setting used by the app server.
		raw = strings.TrimSpace(os.Getenv("PULSE_TRUSTED_PROXY_CIDRS"))
	}
	if raw == "" {
		return
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			network.IP = network.IP.Mask(network.Mask)
			trustedProxyCIDRs = append(trustedProxyCIDRs, network)
			continue
		}

		ip := net.ParseIP(entry)
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		mask := net.CIDRMask(bits, bits)
		trustedProxyCIDRs = append(trustedProxyCIDRs, &net.IPNet{
			IP:   ip.Mask(mask),
			Mask: mask,
		})
	}
}
