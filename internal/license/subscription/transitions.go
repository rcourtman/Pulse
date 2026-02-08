package subscription

import (
	"slices"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

// Transition represents a valid state transition.
type Transition struct {
	From license.SubscriptionState
	To   license.SubscriptionState
}

// validTransitions defines all allowed state transitions.
var validTransitions = map[Transition]bool{
	{license.SubStateTrial, license.SubStateActive}:      true, // Trial converted to paid
	{license.SubStateTrial, license.SubStateExpired}:     true, // Trial expired without conversion
	{license.SubStateActive, license.SubStateGrace}:      true, // Payment failed, entering grace
	{license.SubStateActive, license.SubStateSuspended}:  true, // Admin suspension
	{license.SubStateGrace, license.SubStateActive}:      true, // Payment recovered
	{license.SubStateGrace, license.SubStateExpired}:     true, // Grace period ended
	{license.SubStateExpired, license.SubStateActive}:    true, // Re-subscription
	{license.SubStateSuspended, license.SubStateActive}:  true, // Admin unsuspension
	{license.SubStateSuspended, license.SubStateExpired}: true, // Suspension expired to full expiry
}

// CanTransition checks if a transition from one state to another is valid.
func CanTransition(from, to license.SubscriptionState) bool {
	return validTransitions[Transition{from, to}]
}

// ValidTransitionsFrom returns all valid target states from the given state.
func ValidTransitionsFrom(from license.SubscriptionState) []license.SubscriptionState {
	targets := make([]license.SubscriptionState, 0)
	for t := range validTransitions {
		if t.From == from {
			targets = append(targets, t.To)
		}
	}

	// Stabilize ordering for deterministic callers/tests.
	slices.Sort(targets)
	return targets
}

// DowngradePolicy defines behavior when a subscription downgrades.
type DowngradePolicy struct {
	// SoftHideGraceDays is the number of days data exceeding new limits
	// is soft-hidden (accessible but flagged) after downgrade.
	SoftHideGraceDays int

	// HardDeleteAfterDays is the number of days after downgrade when
	// data exceeding new limits is permanently deleted.
	HardDeleteAfterDays int
}

// DefaultDowngradePolicy is the default policy for subscription downgrades.
var DefaultDowngradePolicy = DowngradePolicy{
	SoftHideGraceDays:   30,
	HardDeleteAfterDays: 60,
}
