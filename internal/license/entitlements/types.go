package entitlements

// SubscriptionState represents the subscription lifecycle state.
type SubscriptionState string

const (
	SubStateTrial     SubscriptionState = "trial"
	SubStateActive    SubscriptionState = "active"
	SubStateGrace     SubscriptionState = "grace"
	SubStateExpired   SubscriptionState = "expired"
	SubStateSuspended SubscriptionState = "suspended"
)

// LimitCheckResult represents the result of evaluating a limit.
type LimitCheckResult string

const (
	LimitAllowed   LimitCheckResult = "allowed"
	LimitSoftBlock LimitCheckResult = "soft_block"
	LimitHardBlock LimitCheckResult = "hard_block"
)
