package entitlements

// TokenSource implements EntitlementSource from JWT Claims.
// Uses Claims.EffectiveCapabilities() and Claims.EffectiveLimits()
// for backward-compatible derivation when explicit B1 fields are absent.
type TokenSource struct {
	claims TokenClaims
}

// TokenClaims is the minimal claim view TokenSource needs.
// license.Claims satisfies this interface.
type TokenClaims interface {
	EffectiveCapabilities() []string
	EffectiveLimits() map[string]int64
	EntitlementMetersEnabled() []string
	EntitlementPlanVersion() string
	EntitlementSubscriptionState() SubscriptionState
}

// NewTokenSource creates a TokenSource from Claims.
func NewTokenSource(claims TokenClaims) *TokenSource {
	return &TokenSource{claims: claims}
}

// Capabilities returns effective capabilities (explicit or tier-derived).
func (t *TokenSource) Capabilities() []string {
	if t == nil || t.claims == nil {
		return nil
	}
	return t.claims.EffectiveCapabilities()
}

// Limits returns effective limits (explicit or derived from MaxNodes/MaxGuests).
func (t *TokenSource) Limits() map[string]int64 {
	if t == nil || t.claims == nil {
		return nil
	}
	return t.claims.EffectiveLimits()
}

// MetersEnabled returns the meters_enabled list from claims.
func (t *TokenSource) MetersEnabled() []string {
	if t == nil || t.claims == nil {
		return nil
	}
	return t.claims.EntitlementMetersEnabled()
}

// PlanVersion returns the plan_version from claims.
func (t *TokenSource) PlanVersion() string {
	if t == nil || t.claims == nil {
		return ""
	}
	return t.claims.EntitlementPlanVersion()
}

// SubscriptionState returns the subscription_state from claims.
// Returns SubStateActive as default if not explicitly set.
func (t *TokenSource) SubscriptionState() SubscriptionState {
	if t == nil || t.claims == nil {
		return SubStateActive
	}
	return t.claims.EntitlementSubscriptionState()
}
