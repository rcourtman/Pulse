package licensing

import "strings"

func DefaultBillingState() *BillingState {
	return &BillingState{
		Capabilities:      []string{},
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       string(SubStateTrial),
		SubscriptionState: SubStateTrial,
	}
}

func NormalizeBillingState(state *BillingState) *BillingState {
	if state == nil {
		return DefaultBillingState()
	}

	normalized := &BillingState{
		Capabilities:      append([]string(nil), state.Capabilities...),
		Limits:            make(map[string]int64, len(state.Limits)),
		MetersEnabled:     append([]string(nil), state.MetersEnabled...),
		PlanVersion:       strings.TrimSpace(state.PlanVersion),
		SubscriptionState: SubscriptionState(strings.ToLower(strings.TrimSpace(string(state.SubscriptionState)))),
		TrialStartedAt:    state.TrialStartedAt,
		TrialEndsAt:       state.TrialEndsAt,

		StripeCustomerID:     strings.TrimSpace(state.StripeCustomerID),
		StripeSubscriptionID: strings.TrimSpace(state.StripeSubscriptionID),
		StripePriceID:        strings.TrimSpace(state.StripePriceID),
	}
	for key, value := range state.Limits {
		normalized.Limits[key] = value
	}

	if normalized.Capabilities == nil {
		normalized.Capabilities = []string{}
	}
	if normalized.Limits == nil {
		normalized.Limits = map[string]int64{}
	}
	if normalized.MetersEnabled == nil {
		normalized.MetersEnabled = []string{}
	}
	if normalized.PlanVersion == "" && normalized.SubscriptionState != "" {
		normalized.PlanVersion = string(normalized.SubscriptionState)
	}

	return normalized
}

func IsValidBillingSubscriptionState(state SubscriptionState) bool {
	switch state {
	case SubStateTrial,
		SubStateActive,
		SubStateGrace,
		SubStateExpired,
		SubStateSuspended,
		SubStateCanceled:
		return true
	default:
		return false
	}
}
