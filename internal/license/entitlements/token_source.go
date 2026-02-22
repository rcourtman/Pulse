package entitlements

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// TokenSource implements EntitlementSource from JWT Claims.
// Uses Claims.EffectiveCapabilities() and Claims.EffectiveLimits()
// for backward-compatible derivation when explicit B1 fields are absent.
type TokenSource = pkglicensing.TokenSource

// TokenClaims is the minimal claim view TokenSource needs.
// license.Claims satisfies this interface.
type TokenClaims = pkglicensing.TokenClaims

// NewTokenSource creates a TokenSource from Claims.
func NewTokenSource(claims TokenClaims) *TokenSource {
	return pkglicensing.NewTokenSource(claims)
}
