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
	normalized.OverflowGrantedAt = cloneInt64Ptr(state.OverflowGrantedAt)
	normalized.QuickstartCreditsGrantedAt = cloneInt64Ptr(state.QuickstartCreditsGrantedAt)
	normalized.CommercialMigration = CloneCommercialMigrationStatus(state.CommercialMigration)

	// Normalize string fields.
	normalized.PlanVersion = strings.TrimSpace(normalized.PlanVersion)
	normalized.SubscriptionState = SubscriptionState(strings.ToLower(strings.TrimSpace(string(normalized.SubscriptionState))))
	normalized.EntitlementJWT = strings.TrimSpace(normalized.EntitlementJWT)
	normalized.EntitlementRefreshToken = strings.TrimSpace(normalized.EntitlementRefreshToken)
	normalized.StripeCustomerID = strings.TrimSpace(normalized.StripeCustomerID)
	normalized.StripeSubscriptionID = strings.TrimSpace(normalized.StripeSubscriptionID)
	normalized.StripePriceID = strings.TrimSpace(normalized.StripePriceID)
	normalized.CommercialMigration = NormalizeCommercialMigrationStatus(normalized.CommercialMigration)

	// Migration shim: rename legacy "max_nodes" key to "max_agents".
	// Existing billing.json files may still use the old key name.
	if v, hasOld := normalized.Limits["max_nodes"]; hasOld {
		if _, hasNew := normalized.Limits["max_agents"]; !hasNew {
			normalized.Limits["max_agents"] = v
		}
		delete(normalized.Limits, "max_nodes")
	}

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
	// Preserve absence when the stored hosted billing record has no plan label.
	// Canonical defaults still come from DefaultBillingState()/call-site defaults.
	normalized.PlanVersion = CanonicalizePlanVersion(normalized.PlanVersion)
	if limit, known := CloudPlanAgentLimits[normalized.PlanVersion]; known {
		normalized.Limits["max_agents"] = int64(limit)
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
