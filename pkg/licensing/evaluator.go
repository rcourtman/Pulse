package licensing

import "log"

// Evaluator is the canonical entitlement evaluator used by runtime surfaces.
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

// CheckLimit evaluates observed value against the limit for the given key.
func (e *Evaluator) CheckLimit(key string, observed int64) LimitCheckResult {
	limit, ok := e.GetLimit(key)
	if !ok || limit <= 0 {
		return LimitAllowed
	}

	if observed >= limit {
		return LimitHardBlock
	}

	if observed*10 >= limit*9 {
		return LimitSoftBlock
	}

	return LimitAllowed
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

// PlanVersion returns the current plan_version from the source.
func (e *Evaluator) PlanVersion() string {
	if e == nil || e.source == nil {
		return ""
	}
	return e.source.PlanVersion()
}

// SubscriptionState returns the current subscription state from the source.
func (e *Evaluator) SubscriptionState() SubscriptionState {
	if e == nil || e.source == nil {
		return SubStateActive
	}
	return e.source.SubscriptionState()
}

// TrialStartedAt returns the trial start timestamp (Unix seconds) when available.
func (e *Evaluator) TrialStartedAt() *int64 {
	if e == nil || e.source == nil {
		return nil
	}
	return cloneInt64Ptr(e.source.TrialStartedAt())
}

// TrialEndsAt returns the trial end timestamp (Unix seconds) when available.
func (e *Evaluator) TrialEndsAt() *int64 {
	if e == nil || e.source == nil {
		return nil
	}
	return cloneInt64Ptr(e.source.TrialEndsAt())
}

func cloneInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
