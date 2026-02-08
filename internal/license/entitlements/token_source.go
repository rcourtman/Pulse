package entitlements

import "github.com/rcourtman/pulse-go-rewrite/internal/license"

// TokenSource implements EntitlementSource from JWT Claims.
// Uses Claims.EffectiveCapabilities() and Claims.EffectiveLimits()
// for backward-compatible derivation when explicit B1 fields are absent.
type TokenSource struct {
	claims *license.Claims
}

// NewTokenSource creates a TokenSource from Claims.
func NewTokenSource(claims *license.Claims) *TokenSource {
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
	return t.claims.MetersEnabled
}

// PlanVersion returns the plan_version from claims.
func (t *TokenSource) PlanVersion() string {
	if t == nil || t.claims == nil {
		return ""
	}
	return t.claims.PlanVersion
}

// SubscriptionState returns the subscription_state from claims.
// Returns SubStateActive as default if not explicitly set.
func (t *TokenSource) SubscriptionState() license.SubscriptionState {
	if t == nil || t.claims == nil {
		return license.SubStateActive
	}
	if t.claims.SubState == "" {
		return license.SubStateActive
	}
	return t.claims.SubState
}
