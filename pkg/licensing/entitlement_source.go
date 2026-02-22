package licensing

// EntitlementSource provides entitlement data from any backing store.
type EntitlementSource interface {
	Capabilities() []string
	Limits() map[string]int64
	MetersEnabled() []string
	PlanVersion() string
	SubscriptionState() SubscriptionState
	TrialStartedAt() *int64
	TrialEndsAt() *int64
}
