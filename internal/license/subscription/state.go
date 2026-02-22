package subscription

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type OperationClass = pkglicensing.OperationClass
type StateBehavior = pkglicensing.StateBehavior

const (
	OpFull     = pkglicensing.OpFull
	OpDegraded = pkglicensing.OpDegraded
	OpLocked   = pkglicensing.OpLocked
)

var StateBehaviors = map[license.SubscriptionState]StateBehavior{
	license.SubStateTrial:     pkglicensing.StateBehaviors[pkglicensing.SubStateTrial],
	license.SubStateActive:    pkglicensing.StateBehaviors[pkglicensing.SubStateActive],
	license.SubStateGrace:     pkglicensing.StateBehaviors[pkglicensing.SubStateGrace],
	license.SubStateExpired:   pkglicensing.StateBehaviors[pkglicensing.SubStateExpired],
	license.SubStateSuspended: pkglicensing.StateBehaviors[pkglicensing.SubStateSuspended],
	license.SubStateCanceled:  pkglicensing.StateBehaviors[pkglicensing.SubStateCanceled],
}

// GetBehavior returns the behavior rules for the given state.
// Returns expired behavior as default for unknown states.
func GetBehavior(state license.SubscriptionState) StateBehavior {
	return pkglicensing.GetBehavior(state)
}
