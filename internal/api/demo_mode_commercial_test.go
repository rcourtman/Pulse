package api

import (
	"strings"
	"testing"
)

func TestPublicDemoCommercialRouteInventoryCoverage(t *testing.T) {
	literalRoutes, _, _ := parseRouterRoutes(t)

	var actualCommercialRoutes []string
	for _, route := range literalRoutes {
		if routeBelongsToPublicDemoCommercialBoundary(route) {
			actualCommercialRoutes = append(actualCommercialRoutes, route)
		}
	}

	actualSet := sliceToSet(t, actualCommercialRoutes, "public demo commercial boundary routes")
	expectedSet := sliceToSet(t, publicDemoCommercialRouteInventory(), "public demo commercial route inventory")

	if missing := setDifference(actualSet, expectedSet); len(missing) > 0 {
		t.Fatalf(
			"commercial routes missing demo policy coverage: %s",
			strings.Join(sortedKeys(missing), ", "),
		)
	}
	if stale := setDifference(expectedSet, actualSet); len(stale) > 0 {
		t.Fatalf(
			"demo commercial route inventory contains routes outside the boundary family: %s",
			strings.Join(sortedKeys(stale), ", "),
		)
	}
}

func TestSanitizeEntitlementPayloadForPublicDemo(t *testing.T) {
	trialExpiresAt := int64(1_746_892_800)
	trialDaysRemaining := 7
	expiresAt := "2026-05-05T00:00:00Z"
	gracePeriodEnd := "2026-05-12T00:00:00Z"
	overflowDaysRemaining := 11

	sanitized := sanitizeEntitlementPayloadForPublicDemo(EntitlementPayload{
		Capabilities: []string{"relay", "ai_patrol"},
		Limits: []LimitStatus{
			{
				Key:     maxMonitoredSystemsLicenseGateKey,
				Limit:   5,
				Current: 16,
				State:   "enforced",
			},
		},
		SubscriptionState:      string(subscriptionStateTrialValue),
		UpgradeReasons:         []UpgradeReason{{Key: "relay", Reason: "Upgrade", ActionURL: "/pricing"}},
		PlanVersion:            "pro_monthly",
		Tier:                   "pro",
		TrialExpiresAt:         &trialExpiresAt,
		TrialDaysRemaining:     &trialDaysRemaining,
		HostedMode:             true,
		Valid:                  true,
		LicensedEmail:          "owner@example.com",
		ExpiresAt:              &expiresAt,
		IsLifetime:             true,
		DaysRemaining:          30,
		InGracePeriod:          true,
		GracePeriodEnd:         &gracePeriodEnd,
		TrialEligible:          true,
		TrialEligibilityReason: "eligible",
		MaxHistoryDays:         90,
		OverflowDaysRemaining:  &overflowDaysRemaining,
		LegacyConnections: legacyConnectionCountsModel{
			ProxmoxNodes:       1,
			DockerHosts:        2,
			KubernetesClusters: 3,
		},
		HasMigrationGap: true,
		CommercialMigration: &commercialMigrationStatusModel{
			State: "failed",
		},
	})

	if len(sanitized.Capabilities) != 2 {
		t.Fatalf("capabilities=%v, want original capabilities preserved", sanitized.Capabilities)
	}
	if len(sanitized.Limits) != 1 {
		t.Fatalf("limits=%v, want one sanitized limit", sanitized.Limits)
	}
	if sanitized.Limits[0].Limit != 0 || sanitized.Limits[0].Current != 0 || sanitized.Limits[0].State != "ok" {
		t.Fatalf("sanitized limit=%+v, want limit=0 current=0 state=ok", sanitized.Limits[0])
	}
	if sanitized.SubscriptionState != string(subscriptionStateActiveValue) {
		t.Fatalf("subscription_state=%q, want %q", sanitized.SubscriptionState, subscriptionStateActiveValue)
	}
	if len(sanitized.UpgradeReasons) != 0 {
		t.Fatalf("upgrade_reasons=%v, want empty", sanitized.UpgradeReasons)
	}
	if sanitized.PlanVersion != "" || sanitized.Tier != "free" {
		t.Fatalf("plan/tier=(%q,%q), want empty/free", sanitized.PlanVersion, sanitized.Tier)
	}
	if sanitized.TrialExpiresAt != nil || sanitized.TrialDaysRemaining != nil {
		t.Fatalf("trial fields should be cleared, got expires=%v days=%v", sanitized.TrialExpiresAt, sanitized.TrialDaysRemaining)
	}
	if sanitized.Valid {
		t.Fatal("valid should be false in sanitized public demo entitlements")
	}
	if sanitized.LicensedEmail != "" || sanitized.ExpiresAt != nil || sanitized.GracePeriodEnd != nil {
		t.Fatalf("commercial identity should be cleared, got email=%q expires=%v grace_end=%v", sanitized.LicensedEmail, sanitized.ExpiresAt, sanitized.GracePeriodEnd)
	}
	if sanitized.IsLifetime || sanitized.DaysRemaining != 0 || sanitized.InGracePeriod {
		t.Fatalf("lifecycle display should be cleared, got %+v", sanitized)
	}
	if sanitized.TrialEligible || sanitized.TrialEligibilityReason != "" {
		t.Fatalf("trial prompt should be cleared, got eligible=%v reason=%q", sanitized.TrialEligible, sanitized.TrialEligibilityReason)
	}
	if sanitized.MaxHistoryDays != 90 || !sanitized.HostedMode {
		t.Fatalf("non-commercial runtime capability fields should be preserved, got max_history_days=%d hosted_mode=%v", sanitized.MaxHistoryDays, sanitized.HostedMode)
	}
	if sanitized.OverflowDaysRemaining != nil {
		t.Fatalf("overflow days should be cleared, got %v", sanitized.OverflowDaysRemaining)
	}
	if sanitized.LegacyConnections.Total() != 0 {
		t.Fatalf("legacy connections should be cleared, got %+v", sanitized.LegacyConnections)
	}
	if sanitized.HasMigrationGap || sanitized.CommercialMigration != nil {
		t.Fatalf("migration state should be cleared, got has_gap=%v migration=%+v", sanitized.HasMigrationGap, sanitized.CommercialMigration)
	}
}

func routeBelongsToPublicDemoCommercialBoundary(route string) bool {
	switch {
	case route == "/auth/trial-activate":
		return true
	case strings.HasPrefix(route, "/api/license/"):
		return true
	case strings.HasPrefix(route, "GET /api/license/"):
		return true
	case strings.HasPrefix(route, "POST /api/license/"):
		return true
	case strings.HasPrefix(route, "GET /api/upgrade-metrics/"):
		return true
	case strings.HasPrefix(route, "POST /api/upgrade-metrics/"):
		return true
	case strings.HasPrefix(route, "PUT /api/upgrade-metrics/"):
		return true
	case route == "GET /api/admin/upgrade-metrics-funnel":
		return true
	case route == "GET /api/admin/orgs/{id}/billing-state":
		return true
	case route == "PUT /api/admin/orgs/{id}/billing-state":
		return true
	default:
		return false
	}
}
