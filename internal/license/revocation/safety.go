package revocation

import (
	"log"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

// SafeEvaluator wraps an entitlements.Evaluator with panic recovery.
// If the underlying evaluator panics, SafeEvaluator recovers, logs a P0 error,
// and returns the fail-open default (allow access) to prevent bricking monitoring.
type SafeEvaluator struct {
	inner *entitlements.Evaluator
}

// NewSafeEvaluator wraps an evaluator with panic safety.
func NewSafeEvaluator(inner *entitlements.Evaluator) *SafeEvaluator {
	return &SafeEvaluator{inner: inner}
}

func safeCall[T any](operation, key, fallbackPolicy string, observed *int64, fallback T, fn func() T) (result T) {
	result = fallback
	defer func() {
		if r := recover(); r != nil {
			if observed == nil {
				log.Printf(
					"CRITICAL [P0]: entitlement evaluator panic recovered operation=%s key=%q fallback_policy=%s panic=%v",
					operation,
					key,
					fallbackPolicy,
					r,
				)
				return
			}

			log.Printf(
				"CRITICAL [P0]: entitlement evaluator panic recovered operation=%s key=%q observed=%d fallback_policy=%s panic=%v",
				operation,
				key,
				*observed,
				fallbackPolicy,
				r,
			)
		}
	}()
	return fn()
}

// HasCapability delegates to inner evaluator with panic recovery.
// On panic: logs P0 error, returns true (fail-open: allow access).
func (s *SafeEvaluator) HasCapability(key string) bool {
	return safeCall("has_capability", key, "allow_access", nil, true, func() bool {
		return s.inner.HasCapability(key)
	})
}

// GetLimit delegates to inner evaluator with panic recovery.
// On panic: logs P0 error, returns (0, false) (no limit = fail-open).
func (s *SafeEvaluator) GetLimit(key string) (int64, bool) {
	type result struct {
		limit int64
		ok    bool
	}

	res := safeCall("get_limit", key, "no_limit", nil, result{limit: 0, ok: false}, func() result {
		limit, ok := s.inner.GetLimit(key)
		return result{
			limit: limit,
			ok:    ok,
		}
	})

	return res.limit, res.ok
}

// CheckLimit delegates to inner evaluator with panic recovery.
// On panic: logs P0 error, returns LimitAllowed (fail-open).
func (s *SafeEvaluator) CheckLimit(key string, observed int64) license.LimitCheckResult {
	return safeCall("check_limit", key, "limit_allowed", &observed, license.LimitAllowed, func() license.LimitCheckResult {
		return s.inner.CheckLimit(key, observed)
	})
}

// MeterEnabled delegates to inner evaluator with panic recovery.
// On panic: logs P0 error, returns false.
func (s *SafeEvaluator) MeterEnabled(key string) bool {
	return safeCall("meter_enabled", key, "disabled", nil, false, func() bool {
		return s.inner.MeterEnabled(key)
	})
}

// EnrollmentRateLimit defines abuse prevention limits for new enrollments.
type EnrollmentRateLimit struct {
	// MaxPerIPPerHour is the maximum new agent registrations per IP per hour.
	MaxPerIPPerHour int

	// MaxPerOrgPerHour is the maximum new agent registrations per org per hour.
	MaxPerOrgPerHour int

	// MaxGlobal is the absolute ceiling for all enrollments to prevent scripted DoS.
	MaxGlobal int
}

// DefaultEnrollmentRateLimit provides sensible defaults.
var DefaultEnrollmentRateLimit = EnrollmentRateLimit{
	MaxPerIPPerHour:  100,
	MaxPerOrgPerHour: 100,
	MaxGlobal:        10000,
}
