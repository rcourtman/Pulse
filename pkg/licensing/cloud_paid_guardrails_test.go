package licensing

import "testing"

func TestNormalizeBillingStatePreservesMissingPlanVersion(t *testing.T) {
	state := &BillingState{
		PlanVersion:       "   ",
		Limits:            map[string]int64{"max_monitored_systems": 42},
		SubscriptionState: SubscriptionState(" ACTIVE "),
	}

	normalized := NormalizeBillingState(state)
	if normalized.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", normalized.PlanVersion)
	}
	if normalized.SubscriptionState != SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateActive)
	}
	if got := normalized.Limits["max_monitored_systems"]; got != 42 {
		t.Fatalf("limits[max_monitored_systems]=%d, want %d", got, 42)
	}
}

func TestNormalizeEntitlementLeaseClaimsPreservesMissingPlanVersion(t *testing.T) {
	claims := &EntitlementLeaseClaims{
		PlanVersion:       "   ",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_monitored_systems": 42},
	}

	normalizeEntitlementLeaseClaims(claims)
	if claims.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", claims.PlanVersion)
	}
	if got := claims.Limits["max_monitored_systems"]; got != 42 {
		t.Fatalf("limits[max_monitored_systems]=%d, want %d", got, 42)
	}
}

func TestClaimsPreserveMissingPlanVersion(t *testing.T) {
	claims := &Claims{
		Tier:        TierCloud,
		PlanVersion: "   ",
		Limits:      map[string]int64{"max_monitored_systems": 42},
	}

	if got := claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	if got := claims.EffectiveLimits()["max_monitored_systems"]; got != 42 {
		t.Fatalf("EffectiveLimits()[max_monitored_systems]=%d, want %d", got, 42)
	}
}

func TestTokenSourcePreservesMissingPlanVersionContract(t *testing.T) {
	source := NewTokenSource(stubTokenClaims{
		planVersion:       "",
		subscriptionState: SubStateActive,
		limits:            map[string]int64{"max_monitored_systems": 42},
	})

	if got := source.PlanVersion(); got != "" {
		t.Fatalf("PlanVersion()=%q, want empty", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("SubscriptionState()=%q, want %q", got, SubStateActive)
	}
	if got := source.Limits()["max_monitored_systems"]; got != 42 {
		t.Fatalf("Limits()[max_monitored_systems]=%d, want %d", got, 42)
	}
}

func TestCloudClaimsMissingPlanVersionFailClosedOnMonitoredSystemLimit(t *testing.T) {
	claims := &Claims{
		Tier:        TierCloud,
		PlanVersion: "   ",
	}

	if got := claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	if got := claims.EffectiveLimits()["max_monitored_systems"]; got != int64(UnknownPlanDefaultMonitoredSystemLimit) {
		t.Fatalf("EffectiveLimits()[max_monitored_systems]=%d, want %d", got, UnknownPlanDefaultMonitoredSystemLimit)
	}
}
