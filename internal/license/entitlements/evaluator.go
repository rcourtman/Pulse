package entitlements

import (
	"log"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

// Evaluator is the canonical entitlement evaluator used by all runtime surfaces.
type Evaluator struct {
	source EntitlementSource
}

// NewEvaluator creates a new evaluator with the given source.
func NewEvaluator(source EntitlementSource) *Evaluator {
	return &Evaluator{source: source}
}

// HasCapability checks if the given capability key is granted.
// It first checks the key directly, then tries resolving a legacy alias.
func (e *Evaluator) HasCapability(key string) bool {
	if e == nil || e.source == nil {
		return false
	}

	capabilities := e.source.Capabilities()

	// Check the key directly first.
	for _, cap := range capabilities {
		if cap == key {
			// Log deprecation warning if applicable.
			if dep, ok := IsDeprecated(key); ok {
				log.Printf("entitlements: deprecated capability %q used, replacement: %q (sunset: %s)",
					key, dep.ReplacementKey, dep.SunsetAt.Format("2006-01-02"))
			}
			return true
		}
	}

	// Check alias resolution.
	resolved := ResolveAlias(key)
	if resolved != key {
		for _, cap := range capabilities {
			if cap == resolved {
				log.Printf("entitlements: alias %q resolved to %q", key, resolved)
				return true
			}
		}
	}

	return false
}

// GetLimit returns the limit value for the given key and whether the limit exists.
func (e *Evaluator) GetLimit(key string) (int64, bool) {
	if e == nil || e.source == nil {
		return 0, false
	}

	limit, ok := e.source.Limits()[key]
	return limit, ok
}

// CheckLimit evaluates the observed value against the limit for the given key.
// Returns LimitAllowed if no limit exists or observed is within limit.
// Returns LimitSoftBlock if observed >= 90% of limit (but below limit).
// Returns LimitHardBlock if observed >= limit.
func (e *Evaluator) CheckLimit(key string, observed int64) license.LimitCheckResult {
	limit, ok := e.GetLimit(key)
	if !ok || limit <= 0 {
		return license.LimitAllowed
	}

	if observed >= limit {
		return license.LimitHardBlock
	}

	if observed*10 >= limit*9 {
		return license.LimitSoftBlock
	}

	return license.LimitAllowed
}

// MeterEnabled checks if the given meter key is enabled.
func (e *Evaluator) MeterEnabled(key string) bool {
	if e == nil || e.source == nil {
		return false
	}

	for _, meter := range e.source.MetersEnabled() {
		if meter == key {
			return true
		}
	}
	return false
}

// SubscriptionState returns the current subscription state from the source.
func (e *Evaluator) SubscriptionState() license.SubscriptionState {
	if e == nil || e.source == nil {
		return license.SubStateActive
	}
	return e.source.SubscriptionState()
}
