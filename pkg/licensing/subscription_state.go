package licensing

// OperationClass categorizes what operations are allowed in a given state.
type OperationClass string

const (
	OpFull     OperationClass = "full"     // All operations allowed
	OpDegraded OperationClass = "degraded" // Existing resources work, new ones blocked
	OpLocked   OperationClass = "locked"   // All operations blocked, contact support
)

// StateBehavior describes what is allowed in a specific subscription state.
type StateBehavior struct {
	// State is the subscription state this behavior applies to.
	State SubscriptionState

	// Operations describes what operations are allowed.
	Operations OperationClass

	// FeaturesAvailable indicates whether paid features are accessible.
	FeaturesAvailable bool

	// ShowWarning indicates whether the UI should show a warning banner.
	ShowWarning bool

	// Description is a human-readable description of the state behavior.
	Description string
}

// StateBehaviors maps each subscription state to its behavior rules.
var StateBehaviors = map[SubscriptionState]StateBehavior{
	SubStateTrial: {
		State:             SubStateTrial,
		Operations:        OpFull,
		FeaturesAvailable: true,
		ShowWarning:       false,
		Description:       "Full capabilities with trial expiry timer.",
	},
	SubStateActive: {
		State:             SubStateActive,
		Operations:        OpFull,
		FeaturesAvailable: true,
		ShowWarning:       false,
		Description:       "Normal enforcement, all paid features active.",
	},
	SubStateGrace: {
		State:             SubStateGrace,
		Operations:        OpFull,
		FeaturesAvailable: true,
		ShowWarning:       true,
		Description:       "Features preserved with warning and countdown.",
	},
	SubStateExpired: {
		State:             SubStateExpired,
		Operations:        OpDegraded,
		FeaturesAvailable: false,
		ShowWarning:       true,
		Description:       "Fallback capabilities only; no data loss.",
	},
	SubStateSuspended: {
		State:             SubStateSuspended,
		Operations:        OpLocked,
		FeaturesAvailable: false,
		ShowWarning:       true,
		Description:       "Administrative lock; contact support.",
	},
	SubStateCanceled: {
		State:             SubStateCanceled,
		Operations:        OpDegraded,
		FeaturesAvailable: false,
		ShowWarning:       true,
		Description:       "Subscription canceled; paid capabilities revoked.",
	},
}

// GetBehavior returns the behavior rules for the given state.
// Returns expired behavior as default for unknown states.
func GetBehavior(state SubscriptionState) StateBehavior {
	if b, ok := StateBehaviors[state]; ok {
		return b
	}
	return StateBehaviors[SubStateExpired]
}
