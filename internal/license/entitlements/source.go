package entitlements

// EntitlementSource provides entitlement data from any backing store.
// Implementation A: TokenSource (stateless JWT claims for self-hosted).
// Implementation B: DatabaseSource (direct DB lookup for SaaS/hosted) - future.
type EntitlementSource interface {
	Capabilities() []string
	Limits() map[string]int64
	MetersEnabled() []string
	PlanVersion() string
	SubscriptionState() SubscriptionState
}
