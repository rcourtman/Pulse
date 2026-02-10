package subscription

import "github.com/rcourtman/pulse-go-rewrite/internal/license"

// OperationClass categorizes what operations are allowed in a given state.
type OperationClass string

const (
	OpFull     OperationClass = "full"      // All operations allowed
	OpReadOnly OperationClass = "read_only" // Read operations only, no new resource creation
	OpDegraded OperationClass = "degraded"  // Existing resources work, new ones blocked
	OpLocked   OperationClass = "locked"    // All operations blocked, contact support
)

// StateBehavior describes what is allowed in a specific subscription state.
type StateBehavior struct {
	// State is the subscription state this behavior applies to.
	State license.SubscriptionState

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
var StateBehaviors = map[license.SubscriptionState]StateBehavior{
	license.SubStateTrial: {
		State:             license.SubStateTrial,
		Operations:        OpFull,
		FeaturesAvailable: true,
		ShowWarning:       false,
		Description:       "Full capabilities with trial expiry timer.",
	},
	license.SubStateActive: {
		State:             license.SubStateActive,
		Operations:        OpFull,
		FeaturesAvailable: true,
		ShowWarning:       false,
		Description:       "Normal enforcement, all paid features active.",
	},
	license.SubStateGrace: {
		State:             license.SubStateGrace,
		Operations:        OpFull,
		FeaturesAvailable: true,
		ShowWarning:       true,
		Description:       "Features preserved with warning and countdown.",
	},
	license.SubStateExpired: {
		State:             license.SubStateExpired,
		Operations:        OpDegraded,
		FeaturesAvailable: false,
		ShowWarning:       true,
		Description:       "Fallback capabilities only; no data loss.",
	},
	license.SubStateSuspended: {
		State:             license.SubStateSuspended,
		Operations:        OpLocked,
		FeaturesAvailable: false,
		ShowWarning:       true,
		Description:       "Administrative lock; contact support.",
	},
	license.SubStateCanceled: {
		State:             license.SubStateCanceled,
		Operations:        OpDegraded,
		FeaturesAvailable: false,
		ShowWarning:       true,
		Description:       "Subscription canceled; paid capabilities revoked.",
	},
}

// GetBehavior returns the behavior rules for the given state.
// Returns expired behavior as default for unknown states.
func GetBehavior(state license.SubscriptionState) StateBehavior {
	if b, ok := StateBehaviors[state]; ok {
		return b
	}
	return StateBehaviors[license.SubStateExpired]
}
