package licensing

// TokenSource implements EntitlementSource from JWT claims.
// Uses claims effective capability/limit derivation for backward compatibility.
type TokenSource struct {
	claims TokenClaims
}

// TokenClaims is the minimal claim view TokenSource needs.
type TokenClaims interface {
	EffectiveCapabilities() []string
	EffectiveLimits() map[string]int64
	EntitlementMetersEnabled() []string
	EntitlementPlanVersion() string
	EntitlementSubscriptionState() SubscriptionState
}

// NewTokenSource creates a TokenSource from token claims.
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

// TrialStartedAt returns nil for token-backed entitlements.
func (t *TokenSource) TrialStartedAt() *int64 {
	return nil
}

// TrialEndsAt returns nil for token-backed entitlements.
func (t *TokenSource) TrialEndsAt() *int64 {
	return nil
}

// OverflowGrantedAt returns nil for token-backed entitlements.
// Onboarding overflow is managed via billing state, not JWT claims.
func (t *TokenSource) OverflowGrantedAt() *int64 {
	return nil
}
