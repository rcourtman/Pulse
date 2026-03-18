package licensing

import (
	"slices"
)

// Transition represents a valid state transition.
type Transition struct {
	From SubscriptionState
	To   SubscriptionState
}

// validTransitions defines all allowed state transitions.
var validTransitions = map[Transition]bool{
	{SubStateTrial, SubStateActive}:      true, // Trial converted to paid
	{SubStateTrial, SubStateExpired}:     true, // Trial expired without conversion
	{SubStateActive, SubStateGrace}:      true, // Payment failed, entering grace
	{SubStateActive, SubStateSuspended}:  true, // Admin suspension
	{SubStateGrace, SubStateActive}:      true, // Payment recovered
	{SubStateGrace, SubStateExpired}:     true, // Grace period ended
	{SubStateExpired, SubStateActive}:    true, // Re-subscription
	{SubStateSuspended, SubStateActive}:  true, // Admin unsuspension
	{SubStateSuspended, SubStateExpired}: true, // Suspension expired to full expiry
}

// CanTransition checks if a transition from one state to another is valid.
func CanTransition(from, to SubscriptionState) bool {
	return validTransitions[Transition{from, to}]
}

// ValidTransitionsFrom returns all valid target states from the given state.
func ValidTransitionsFrom(from SubscriptionState) []SubscriptionState {
	targets := make([]SubscriptionState, 0)
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
