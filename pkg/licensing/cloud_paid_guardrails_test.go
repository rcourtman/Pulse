package licensing

import "testing"

func TestNormalizeBillingStatePreservesMissingPlanVersionAndScrubsRetiredMonitoringLimit(t *testing.T) {
	state := &BillingState{
		PlanVersion:       "   ",
		Limits:            map[string]int64{"max_monitored_systems": 42, "max_guests": 7},
		SubscriptionState: SubscriptionState(" ACTIVE "),
	}

	normalized := NormalizeBillingState(state)
	if normalized.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", normalized.PlanVersion)
	}
	if normalized.SubscriptionState != SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateActive)
	}
	if _, ok := normalized.Limits["max_monitored_systems"]; ok {
		t.Fatalf("limits retained retired max_monitored_systems: %v", normalized.Limits)
	}
	if got := normalized.Limits["max_guests"]; got != 7 {
		t.Fatalf("limits[max_guests]=%d, want %d", got, 7)
	}
}

func TestNormalizeEntitlementLeaseClaimsPreservesMissingPlanVersionAndScrubsRetiredMonitoringLimit(t *testing.T) {
	claims := &EntitlementLeaseClaims{
		PlanVersion:       "   ",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_monitored_systems": 42, "max_guests": 7},
	}

	normalizeEntitlementLeaseClaims(claims)
	if claims.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", claims.PlanVersion)
	}
	if _, ok := claims.Limits["max_monitored_systems"]; ok {
		t.Fatalf("limits retained retired max_monitored_systems: %v", claims.Limits)
	}
	if got := claims.Limits["max_guests"]; got != 7 {
		t.Fatalf("limits[max_guests]=%d, want %d", got, 7)
	}
}

func TestClaimsPreserveMissingPlanVersionAndScrubRetiredMonitoringLimit(t *testing.T) {
	claims := &Claims{
		Tier:        TierCloud,
		PlanVersion: "   ",
		Limits:      map[string]int64{"max_monitored_systems": 42, "max_guests": 7},
	}

	if got := claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	limits := claims.EffectiveLimits()
	if _, ok := limits["max_monitored_systems"]; ok {
		t.Fatalf("EffectiveLimits retained retired max_monitored_systems: %v", limits)
	}
	if got := limits["max_guests"]; got != 7 {
		t.Fatalf("EffectiveLimits()[max_guests]=%d, want %d", got, 7)
	}
}

func TestTokenSourcePreservesMissingPlanVersionContract(t *testing.T) {
	source := NewTokenSource(stubTokenClaims{
		planVersion:       "",
		subscriptionState: SubStateActive,
		limits:            map[string]int64{"max_guests": 7},
	})

	if got := source.PlanVersion(); got != "" {
		t.Fatalf("PlanVersion()=%q, want empty", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("SubscriptionState()=%q, want %q", got, SubStateActive)
	}
	if got := source.Limits()["max_guests"]; got != 7 {
		t.Fatalf("Limits()[max_guests]=%d, want %d", got, 7)
	}
}

func TestCloudClaimsMissingPlanVersionDoesNotReintroduceMonitoringLimit(t *testing.T) {
	claims := &Claims{
		Tier:        TierCloud,
		PlanVersion: "   ",
	}

	if got := claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	if _, ok := claims.EffectiveLimits()["max_monitored_systems"]; ok {
		t.Fatalf("EffectiveLimits retained retired max_monitored_systems: %v", claims.EffectiveLimits())
	}
}
