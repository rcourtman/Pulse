package licensing

import (
	"sort"
	"time"
)

// Claims represents the JWT claims in a Pulse Pro license.
type Claims struct {
	// License ID (unique identifier)
	LicenseID string `json:"lid"`

	// Email of the license holder
	Email string `json:"email"`

	// License tier (pro, pro_annual, lifetime, msp, enterprise)
	Tier Tier `json:"tier"`

	// Issued at (Unix timestamp)
	IssuedAt int64 `json:"iat"`

	// Expires at (Unix timestamp, 0 for lifetime)
	ExpiresAt int64 `json:"exp,omitempty"`

	// Features explicitly granted (optional, tier implies features)
	Features []string `json:"features,omitempty"`

	// Max nodes (0 = unlimited)
	MaxNodes int `json:"max_nodes,omitempty"`

	// Max guests (0 = unlimited)
	MaxGuests int `json:"max_guests,omitempty"`

	// Entitlement primitives (B1) - when present, these override tier-based derivation.
	// When absent (nil/empty), entitlements are derived from Tier + existing fields.
	Capabilities  []string          `json:"capabilities,omitempty"`
	Limits        map[string]int64  `json:"limits,omitempty"`
	MetersEnabled []string          `json:"meters_enabled,omitempty"`
	PlanVersion   string            `json:"plan_version,omitempty"`
	SubState      SubscriptionState `json:"subscription_state,omitempty"`
}

// EffectiveCapabilities returns explicit capabilities when present; otherwise tier-derived capabilities.
func (c Claims) EffectiveCapabilities() []string {
	if c.Capabilities != nil && len(c.Capabilities) > 0 {
		return c.Capabilities
	}
	return DeriveCapabilitiesFromTier(c.Tier, c.Features)
}

// EffectiveLimits returns explicit limits when present; otherwise limits derived from legacy fields.
func (c Claims) EffectiveLimits() map[string]int64 {
	if c.Limits != nil && len(c.Limits) > 0 {
		return c.Limits
	}

	limits := make(map[string]int64)
	if c.MaxNodes > 0 {
		limits["max_nodes"] = int64(c.MaxNodes)
	}
	if c.MaxGuests > 0 {
		limits["max_guests"] = int64(c.MaxGuests)
	}
	return limits
}

// EntitlementMetersEnabled returns metering keys for evaluator sources.
func (c *Claims) EntitlementMetersEnabled() []string {
	if c == nil {
		return nil
	}
	return c.MetersEnabled
}

// EntitlementPlanVersion returns plan metadata for evaluator sources.
func (c *Claims) EntitlementPlanVersion() string {
	if c == nil {
		return ""
	}
	return c.PlanVersion
}

// EntitlementSubscriptionState returns normalized subscription state for evaluator sources.
func (c *Claims) EntitlementSubscriptionState() SubscriptionState {
	if c == nil || c.SubState == "" {
		return SubStateActive
	}
	return c.SubState
}

// License represents a validated Pulse Pro license.
type License struct {
	// Raw JWT token
	Raw string `json:"-"`

	// Validated claims
	Claims Claims `json:"claims"`

	// Validation metadata
	ValidatedAt time.Time `json:"validated_at"`

	// Grace period end (if license was validated during grace period)
	GracePeriodEnd *time.Time `json:"grace_period_end,omitempty"`
}

// IsExpired checks if the license has expired.
func (l *License) IsExpired() bool {
	if l.Claims.ExpiresAt == 0 {
		return false // Lifetime license never expires
	}
	return time.Now().Unix() > l.Claims.ExpiresAt
}

// IsLifetime returns true if this is a lifetime license.
func (l *License) IsLifetime() bool {
	return l.Claims.ExpiresAt == 0 || l.Claims.Tier == TierLifetime
}

// DaysRemaining returns the number of days until expiration.
// Returns -1 for lifetime licenses.
func (l *License) DaysRemaining() int {
	if l.IsLifetime() {
		return -1
	}
	remaining := time.Until(time.Unix(l.Claims.ExpiresAt, 0))
	if remaining < 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

// ExpiresAt returns the expiration time, or nil for lifetime.
func (l *License) ExpiresAt() *time.Time {
	if l.IsLifetime() {
		return nil
	}
	t := time.Unix(l.Claims.ExpiresAt, 0)
	return &t
}

// HasFeature checks if the license grants a specific feature.
func (l *License) HasFeature(feature string) bool {
	for _, capability := range l.Claims.EffectiveCapabilities() {
		if capability == feature {
			return true
		}
	}
	return false
}

// AllFeatures returns all features granted by this license.
func (l *License) AllFeatures() []string {
	features := append([]string(nil), l.Claims.EffectiveCapabilities()...)
	sort.Strings(features)
	return features
}

// LicenseState represents the current state of the license.
type LicenseState string

const (
	LicenseStateNone        LicenseState = "none"
	LicenseStateActive      LicenseState = "active"
	LicenseStateExpired     LicenseState = "expired"
	LicenseStateGracePeriod LicenseState = "grace_period"
)

// LicenseStatus is the JSON response for license status API.
type LicenseStatus struct {
	Valid          bool     `json:"valid"`
	Tier           Tier     `json:"tier"`
	Email          string   `json:"email,omitempty"`
	ExpiresAt      *string  `json:"expires_at,omitempty"`
	IsLifetime     bool     `json:"is_lifetime"`
	DaysRemaining  int      `json:"days_remaining"`
	Features       []string `json:"features"`
	MaxNodes       int      `json:"max_nodes,omitempty"`
	MaxGuests      int      `json:"max_guests,omitempty"`
	InGracePeriod  bool     `json:"in_grace_period,omitempty"`
	GracePeriodEnd *string  `json:"grace_period_end,omitempty"`
}
