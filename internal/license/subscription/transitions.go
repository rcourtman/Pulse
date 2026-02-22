package subscription

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type Transition = pkglicensing.Transition
type DowngradePolicy = pkglicensing.DowngradePolicy

// CanTransition checks if a transition from one state to another is valid.
func CanTransition(from, to license.SubscriptionState) bool {
	return pkglicensing.CanTransition(from, to)
}

// ValidTransitionsFrom returns all valid target states from the given state.
func ValidTransitionsFrom(from license.SubscriptionState) []license.SubscriptionState {
	return pkglicensing.ValidTransitionsFrom(from)
}

// DefaultDowngradePolicy is the default policy for subscription downgrades.
var DefaultDowngradePolicy = pkglicensing.DefaultDowngradePolicy
