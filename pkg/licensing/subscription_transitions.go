package licensing

import (
	"slices"
	"strings"
	"time"
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

// DowngradeRetentionState is the durable local handoff between a reduced
// commercial history entitlement and storage retention. Access is restricted
// immediately by the entitlement evaluator; physical deletion cannot begin
// before PurgeEligibleAt.
type DowngradeRetentionState struct {
	PreviousTier         Tier  `json:"previous_tier"`
	CurrentTier          Tier  `json:"current_tier"`
	PreviousHistoryDays  int   `json:"previous_history_days"`
	CurrentHistoryDays   int   `json:"current_history_days"`
	DetectedAt           int64 `json:"detected_at"`
	RecoveryGuaranteedTo int64 `json:"recovery_guaranteed_to"`
	PurgeEligibleAt      int64 `json:"purge_eligible_at"`
}

// NormalizeDowngradeRetentionState canonicalizes persisted downgrade state.
func NormalizeDowngradeRetentionState(state DowngradeRetentionState) DowngradeRetentionState {
	state.PreviousTier = Tier(strings.ToLower(strings.TrimSpace(string(state.PreviousTier))))
	state.CurrentTier = Tier(strings.ToLower(strings.TrimSpace(string(state.CurrentTier))))
	return state
}

// AdvanceDowngradeRetention records a new history reduction, preserves an
// existing window for same-tier refreshes, and clears it on entitlement
// expansion before data has been purged.
func AdvanceDowngradeRetention(existing *DowngradeRetentionState, previousTier, currentTier Tier, now time.Time) *DowngradeRetentionState {
	previousDays := historyDaysForDowngradeRetention(previousTier)
	currentDays := historyDaysForDowngradeRetention(currentTier)
	if currentDays >= previousDays {
		if currentDays > previousDays {
			return nil
		}
		if existing == nil {
			return nil
		}
		preserved := NormalizeDowngradeRetentionState(*existing)
		return &preserved
	}

	detectedAt := now.UTC().Unix()
	state := &DowngradeRetentionState{
		PreviousTier:         previousTier,
		CurrentTier:          currentTier,
		PreviousHistoryDays:  previousDays,
		CurrentHistoryDays:   currentDays,
		DetectedAt:           detectedAt,
		RecoveryGuaranteedTo: now.UTC().AddDate(0, 0, DefaultDowngradePolicy.SoftHideGraceDays).Unix(),
		PurgeEligibleAt:      now.UTC().AddDate(0, 0, DefaultDowngradePolicy.HardDeleteAfterDays).Unix(),
	}
	return state
}

func historyDaysForDowngradeRetention(tier Tier) int {
	if days, ok := TierHistoryDays[tier]; ok && days > 0 {
		return days
	}
	return TierHistoryDays[TierFree]
}
