package api

import (
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

const (
	availabilityPushPathPrefix    = "/api/push/"
	maxAvailabilityPushBodyBytes  = 1024
	pushTokenRateLimitBurst       = 10
	pushTokenRateLimitWindow      = 10 * time.Second
	pushInvalidIPRateLimitBurst   = 30
	pushInvalidIPRateLimitWindow  = time.Minute
	pushRateLimiterMaxTrackedKeys = 4096
)

type availabilityPushRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]pushRateBucket
}

type pushRateBucket struct {
	tokens     float64
	lastRefill time.Time
}

func newAvailabilityPushRateLimiter() *availabilityPushRateLimiter {
	return &availabilityPushRateLimiter{
		buckets: make(map[string]pushRateBucket),
	}
}

func (l *availabilityPushRateLimiter) allow(key string, burst int, window time.Duration, now time.Time) bool {
	if l == nil || key == "" || burst <= 0 || window <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = make(map[string]pushRateBucket)
	}
	if len(l.buckets) > pushRateLimiterMaxTrackedKeys {
		cutoff := now.Add(-5 * time.Minute)
		for k, b := range l.buckets {
			if b.lastRefill.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
	}
	ratePerSec := float64(burst) / window.Seconds()
	bucket, ok := l.buckets[key]
	if !ok {
		bucket = pushRateBucket{
			tokens:     float64(burst),
			lastRefill: now,
		}
	} else {
		elapsed := now.Sub(bucket.lastRefill).Seconds()
		if elapsed > 0 {
			bucket.tokens += elapsed * ratePerSec
			if bucket.tokens > float64(burst) {
				bucket.tokens = float64(burst)
			}
			bucket.lastRefill = now
		}
	}
	if bucket.tokens < 1.0 {
		l.buckets[key] = bucket
		return false
	}
	bucket.tokens -= 1.0
	l.buckets[key] = bucket
	return true
}

func (h *AvailabilityHandlers) getPushRateLimiter() *availabilityPushRateLimiter {
	if h.pushRateLimiter == nil {
		h.pushRateLimiter = newAvailabilityPushRateLimiter()
	}
	return h.pushRateLimiter
}

// HandleRotatePushToken replaces a push check's secret ping token, immediately
// revoking the previous URL.
func (h *AvailabilityHandlers) HandleRotatePushToken(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}
	targetID, ok := availabilityTargetIDFromPath(strings.TrimSuffix(strings.TrimRight(r.URL.Path, "/"), "/rotate-push-token"))
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_target_id", "Availability target ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	index := -1
	for i := range targets {
		if strings.TrimSpace(targets[i].ID) == targetID {
			index = i
			break
		}
	}
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "availability_not_found", "Availability target not found", nil)
		return
	}
	target := config.NormalizeAvailabilityTarget(targets[index])
	if target.Protocol != config.AvailabilityProbePush || target.Push == nil {
		writeErrorResponse(w, http.StatusBadRequest, "not_push_target", "Only push availability checks have a ping token to rotate", nil)
		return
	}
	token, err := config.GenerateAvailabilityPushToken()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "push_token_generation_failed", "Failed to generate push check token", nil)
		return
	}
	pushCopy := *target.Push
	pushCopy.Token = token
	target.Push = &pushCopy
	targets[index] = target
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_save_failed", "Failed to save availability targets", map[string]string{"error": err.Error()})
		return
	}
	h.refreshMonitor(r.Context())
	writeJSON(w, http.StatusOK, availabilityTargetAPIResponse(target))
}

// HandlePushPing accepts unauthenticated inbound heartbeat pings authenticated
// solely by a per-check 256-bit URL token:
//   - GET/POST/HEAD /api/push/{token}
//   - GET/POST/HEAD /api/push/{token}/start
//   - GET/POST/HEAD /api/push/{token}/fail
// Supports Uptime Kuma (?status=up|down&msg=...&ping=<ms>) and Healthchecks.io
// (/start, /fail, optional bounded plain-text POST body) conventions.
func (h *AvailabilityHandlers) HandlePushPing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodHead:
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusNotFound, "push_not_found", "Push check not found", nil)
		return
	}

	token, suffix, ok := parseAvailabilityPushPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "push_not_found", "Push check not found", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", nil)
		return
	}

	// Constant-time scan across all push targets without short-circuiting.
	var matched *config.AvailabilityTarget
	candidateBytes := []byte(token)
	for i := range targets {
		normalized := config.NormalizeAvailabilityTarget(targets[i])
		if normalized.Protocol != config.AvailabilityProbePush || normalized.Push == nil || normalized.Push.Token == "" {
			continue
		}
		storedBytes := []byte(normalized.Push.Token)
		if len(candidateBytes) == len(storedBytes) && subtle.ConstantTimeCompare(candidateBytes, storedBytes) == 1 {
			copyTarget := normalized
			matched = &copyTarget
		}
	}

	now := time.Now().UTC()
	limiter := h.getPushRateLimiter()
	if matched == nil {
		clientIP := GetClientIP(r)
		if !limiter.allow("invalid:"+clientIP, pushInvalidIPRateLimitBurst, pushInvalidIPRateLimitWindow, now) {
			writeErrorResponse(w, http.StatusTooManyRequests, "push_rate_limited", "Too many requests", nil)
			return
		}
		writeErrorResponse(w, http.StatusNotFound, "push_not_found", "Push check not found", nil)
		return
	}
	if !limiter.allow("target:"+matched.ID, pushTokenRateLimitBurst, pushTokenRateLimitWindow, now) {
		writeErrorResponse(w, http.StatusTooManyRequests, "push_rate_limited", "Too many push pings for this check", nil)
		return
	}

	event, validEvent := resolveAvailabilityPushEvent(suffix, r.URL.Query().Get("status"))
	if !validEvent {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_push_status", "Unsupported push status parameter", nil)
		return
	}

	var durationOverride *int64
	if rawPing := strings.TrimSpace(r.URL.Query().Get("ping")); rawPing != "" {
		parsed, err := strconv.ParseInt(rawPing, 10, 64)
		if err != nil || parsed < 0 {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_push_ping", "Push ping duration must be a non-negative integer in milliseconds", nil)
			return
		}
		durationOverride = &parsed
	}

	message := strings.TrimSpace(r.URL.Query().Get("msg"))
	if r.Method == http.MethodPost && r.Body != nil {
		limited := http.MaxBytesReader(w, r.Body, maxAvailabilityPushBodyBytes)
		defer limited.Close()
		bodyBytes, err := io.ReadAll(limited)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeErrorResponse(w, http.StatusRequestEntityTooLarge, "push_body_too_large", "Push request body must be 1024 bytes or less", nil)
				return
			}
			writeErrorResponse(w, http.StatusBadRequest, "invalid_push_body", "Failed to read push request body", nil)
			return
		}
		if message == "" {
			message = strings.TrimSpace(string(bodyBytes))
		}
	}

	monitor := h.monitorForRequest(r.Context())
	var result monitoring.AvailabilityPushPingResult
	if monitor != nil {
		result = monitor.RecordAvailabilityPushPing(*matched, event, message, durationOverride, now)
	} else {
		result = monitoring.AvailabilityPushPingResult{
			Accepted:   true,
			TargetID:   matched.ID,
			Event:      string(event),
			ReceivedAt: now,
		}
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func parseAvailabilityPushPath(rawPath string) (token string, suffix string, ok bool) {
	trimmed := strings.TrimPrefix(rawPath, availabilityPushPathPrefix)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) > 2 {
		return "", "", false
	}
	token = parts[0]
	if len(token) < 16 || len(token) > 128 {
		return "", "", false
	}
	for _, ch := range token {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return "", "", false
		}
	}
	if len(parts) == 2 {
		suffix = strings.ToLower(parts[1])
		if suffix != "start" && suffix != "fail" {
			return "", "", false
		}
	}
	return token, suffix, true
}

func resolveAvailabilityPushEvent(suffix, statusQuery string) (monitoring.AvailabilityPushEvent, bool) {
	switch suffix {
	case "start":
		return monitoring.AvailabilityPushEventStart, true
	case "fail":
		return monitoring.AvailabilityPushEventFail, true
	}
	switch strings.ToLower(strings.TrimSpace(statusQuery)) {
	case "", "up", "ok":
		return monitoring.AvailabilityPushEventSuccess, true
	case "down", "fail", "error":
		return monitoring.AvailabilityPushEventFail, true
	case "start":
		return monitoring.AvailabilityPushEventStart, true
	default:
		return "", false
	}
}
