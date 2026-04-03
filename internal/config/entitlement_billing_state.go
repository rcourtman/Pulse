package config

import (
	"fmt"
	"strings"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

const entitlementBillingDefaultOrgID = "default"

// NormalizeEntitlementBillingOrgID canonicalizes the org scope used for
// entitlement-backed billing reads.
func NormalizeEntitlementBillingOrgID(raw string) string {
	orgID := strings.TrimSpace(raw)
	if orgID == "" {
		return entitlementBillingDefaultOrgID
	}
	return orgID
}

// LoadEffectiveEntitlementBillingState loads the effective billing state for an
// entitlement-backed runtime. Multi-tenant hosted lanes can store the
// canonical entitlement lease under the default org and inherit it from
// org-scoped runtime directories.
func LoadEffectiveEntitlementBillingState(baseDataDir, orgID string) (*pkglicensing.BillingState, string, error) {
	baseDataDir = strings.TrimSpace(baseDataDir)
	if baseDataDir == "" {
		return nil, "", nil
	}

	billingStore := NewFileBillingStore(baseDataDir)
	effectiveOrgID := NormalizeEntitlementBillingOrgID(orgID)
	state, err := billingStore.GetBillingState(effectiveOrgID)
	if err != nil {
		return nil, "", fmt.Errorf("load billing state for org %q: %w", effectiveOrgID, err)
	}
	if effectiveOrgID == entitlementBillingDefaultOrgID || billingStateHasEntitlementAuthority(state) {
		return state, effectiveOrgID, nil
	}

	defaultState, err := billingStore.GetBillingState(entitlementBillingDefaultOrgID)
	if err != nil {
		return nil, "", fmt.Errorf("load default entitlement billing state: %w", err)
	}
	if billingStateHasEntitlementAuthority(defaultState) {
		return defaultState, entitlementBillingDefaultOrgID, nil
	}

	return state, effectiveOrgID, nil
}

func billingStateHasEntitlementAuthority(state *pkglicensing.BillingState) bool {
	if state == nil {
		return false
	}
	return strings.TrimSpace(string(state.SubscriptionState)) != "" ||
		strings.TrimSpace(state.EntitlementJWT) != "" ||
		strings.TrimSpace(state.EntitlementRefreshToken) != ""
}
