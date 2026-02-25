package licensing

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
	// TrialExtendedAt is the Unix timestamp when the trial was extended (one extension per trial).
	TrialExtendedAt *int64 `json:"trial_extended_at,omitempty"`

	// OverflowGrantedAt is the Unix timestamp when the onboarding overflow (+1 host)
	// was granted. Set-once on first free-tier entitlement access; never reset.
	// The overflow window is 14 days from this timestamp.
	OverflowGrantedAt *int64 `json:"overflow_granted_at,omitempty"`

	// Quickstart credits: 25 free hosted Patrol runs for every new workspace.
	// No API key needed — uses the Pulse-hosted MiniMax proxy.
	QuickstartCreditsGranted   bool   `json:"quickstart_credits_granted,omitempty"`
	QuickstartCreditsUsed      int    `json:"quickstart_credits_used,omitempty"`
	QuickstartCreditsGrantedAt *int64 `json:"quickstart_credits_granted_at,omitempty"`

	// Integrity is an HMAC-SHA256 over critical billing fields.
	// Used for tamper detection on self-hosted installations.
	Integrity string `json:"integrity,omitempty"`

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
