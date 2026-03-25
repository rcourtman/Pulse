package api

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func loadHostedEffectiveBillingState(billingStore *config.FileBillingStore, orgID string) (*billingState, string, error) {
	if billingStore == nil {
		return nil, "", nil
	}

	effectiveOrgID := normalizeHostedEntitlementOrgID(orgID)
	state, err := billingStore.GetBillingState(effectiveOrgID)
	if err != nil {
		return nil, "", fmt.Errorf("load hosted billing state for org %q: %w", effectiveOrgID, err)
	}
	if effectiveOrgID == hostedRelayBootstrapOrgID || hostedBillingStateHasSubscription(state) {
		return state, effectiveOrgID, nil
	}

	defaultState, err := billingStore.GetBillingState(hostedRelayBootstrapOrgID)
	if err != nil {
		return nil, "", fmt.Errorf("load hosted default billing state fallback: %w", err)
	}
	if hostedBillingStateHasSubscription(defaultState) {
		return defaultState, hostedRelayBootstrapOrgID, nil
	}
	return state, effectiveOrgID, nil
}

func hostedBillingStateHasSubscription(state *billingState) bool {
	return state != nil && strings.TrimSpace(string(state.SubscriptionState)) != ""
}
