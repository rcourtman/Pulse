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

	// Start with a full struct copy so new fields are never silently dropped.
	cp := *state
	normalized := &cp

	// Deep-clone reference types to avoid aliasing the original.
	normalized.Capabilities = append([]string(nil), state.Capabilities...)
	normalized.MetersEnabled = append([]string(nil), state.MetersEnabled...)
	normalized.Limits = make(map[string]int64, len(state.Limits))
	for key, value := range state.Limits {
		normalized.Limits[key] = value
	}

	// Clone pointer fields so the caller can't mutate through the original.
	normalized.TrialStartedAt = cloneInt64Ptr(state.TrialStartedAt)
	normalized.TrialEndsAt = cloneInt64Ptr(state.TrialEndsAt)
	normalized.TrialExtendedAt = cloneInt64Ptr(state.TrialExtendedAt)

	// Normalize string fields.
	normalized.PlanVersion = strings.TrimSpace(normalized.PlanVersion)
	normalized.SubscriptionState = SubscriptionState(strings.ToLower(strings.TrimSpace(string(normalized.SubscriptionState))))
	normalized.StripeCustomerID = strings.TrimSpace(normalized.StripeCustomerID)
	normalized.StripeSubscriptionID = strings.TrimSpace(normalized.StripeSubscriptionID)
	normalized.StripePriceID = strings.TrimSpace(normalized.StripePriceID)

	// Ensure slices/maps are never nil (JSON marshals as [] / {} instead of null).
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
