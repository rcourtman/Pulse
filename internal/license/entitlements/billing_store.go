package entitlements

// BillingState represents the billing/entitlement state for a tenant.
type BillingState struct {
	Capabilities      []string          `json:"capabilities"`
	Limits            map[string]int64  `json:"limits"`
	MetersEnabled     []string          `json:"meters_enabled"`
	PlanVersion       string            `json:"plan_version"`
	SubscriptionState SubscriptionState `json:"subscription_state"`
	// TrialStartedAt is the Unix timestamp when a trial was started (one per org ever).
	TrialStartedAt *int64 `json:"trial_started_at,omitempty"`
	// TrialEndsAt is the Unix timestamp when a trial expires.
	TrialEndsAt *int64 `json:"trial_ends_at,omitempty"`

	// Stripe identifiers (Cloud).
	// These fields allow webhook events to reconcile Stripe customers/subscriptions back to an org.
	StripeCustomerID     string `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string `json:"stripe_subscription_id,omitempty"`
	StripePriceID        string `json:"stripe_price_id,omitempty"`
}

// BillingStore provides billing state for tenants.
type BillingStore interface {
	GetBillingState(orgID string) (*BillingState, error)
}
