package entitlements

// BillingState represents the billing/entitlement state for a tenant.
type BillingState struct {
	Capabilities      []string          `json:"capabilities"`
	Limits            map[string]int64  `json:"limits"`
	MetersEnabled     []string          `json:"meters_enabled"`
	PlanVersion       string            `json:"plan_version"`
	SubscriptionState SubscriptionState `json:"subscription_state"`
}

// BillingStore provides billing state for tenants.
type BillingStore interface {
	GetBillingState(orgID string) (*BillingState, error)
}
