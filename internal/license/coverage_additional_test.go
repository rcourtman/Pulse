package license

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

func TestClaimsEntitlementMetadataHelpers(t *testing.T) {
	t.Run("nil receiver defaults", func(t *testing.T) {
		var claims *Claims

		if got := claims.EntitlementMetersEnabled(); got != nil {
			t.Fatalf("EntitlementMetersEnabled() = %v, want nil", got)
		}
		if got := claims.EntitlementPlanVersion(); got != "" {
			t.Fatalf("EntitlementPlanVersion() = %q, want empty", got)
		}
		if got := claims.EntitlementSubscriptionState(); got != SubStateActive {
			t.Fatalf("EntitlementSubscriptionState() = %q, want %q", got, SubStateActive)
		}
	})

	t.Run("explicit values", func(t *testing.T) {
		claims := &Claims{
			MetersEnabled: []string{"events_ingested", "alerts_sent"},
			PlanVersion:   "2026-02",
			SubState:      SubStateSuspended,
		}

		if got := claims.EntitlementPlanVersion(); got != "2026-02" {
			t.Fatalf("EntitlementPlanVersion() = %q, want %q", got, "2026-02")
		}
		if got := claims.EntitlementSubscriptionState(); got != SubStateSuspended {
			t.Fatalf("EntitlementSubscriptionState() = %q, want %q", got, SubStateSuspended)
		}
		if got := claims.EntitlementMetersEnabled(); !reflect.DeepEqual(got, claims.MetersEnabled) {
			t.Fatalf("EntitlementMetersEnabled() = %v, want %v", got, claims.MetersEnabled)
		}
	})

	t.Run("empty subscription state defaults to active", func(t *testing.T) {
		claims := &Claims{SubState: ""}
		if got := claims.EntitlementSubscriptionState(); got != SubStateActive {
			t.Fatalf("EntitlementSubscriptionState() = %q, want %q", got, SubStateActive)
		}
	})
}

func TestGetTierDisplayNameCoversAllKnownTiers(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{tier: TierFree, want: "Free"},
		{tier: TierPro, want: "Pro Intelligence (Monthly)"},
		{tier: TierProAnnual, want: "Pro Intelligence (Annual)"},
		{tier: TierLifetime, want: "Pro Intelligence (Lifetime)"},
		{tier: TierCloud, want: "Cloud"},
		{tier: TierMSP, want: "MSP"},
		{tier: TierEnterprise, want: "Enterprise"},
		{tier: Tier("unknown"), want: "Unknown"},
	}

	for _, tt := range tests {
		if got := GetTierDisplayName(tt.tier); got != tt.want {
			t.Fatalf("GetTierDisplayName(%q) = %q, want %q", tt.tier, got, tt.want)
		}
	}
}

func TestGetFeatureDisplayNameCoversKnownFeaturesAndFallback(t *testing.T) {
	tests := []struct {
		feature string
		want    string
	}{
		{feature: FeatureAIPatrol, want: "Pulse Patrol (Background Health Checks)"},
		{feature: FeatureAIAlerts, want: "Alert Analysis"},
		{feature: FeatureAIAutoFix, want: "Pulse Patrol Auto-Fix"},
		{feature: FeatureKubernetesAI, want: "Kubernetes Analysis"},
		{feature: FeatureUpdateAlerts, want: "Update Alerts (Container/Package Updates)"},
		{feature: FeatureRBAC, want: "Role-Based Access Control (RBAC)"},
		{feature: FeatureMultiUser, want: "Multi-User Mode"},
		{feature: FeatureWhiteLabel, want: "White-Label Branding"},
		{feature: FeatureMultiTenant, want: "Multi-Tenant Mode"},
		{feature: FeatureUnlimited, want: "Unlimited Instances"},
		{feature: FeatureAgentProfiles, want: "Centralized Agent Profiles"},
		{feature: FeatureAuditLogging, want: "Enterprise Audit Logging"},
		{feature: FeatureSSO, want: "Basic SSO (OIDC)"},
		{feature: FeatureAdvancedSSO, want: "Advanced SSO (SAML/Multi-Provider)"},
		{feature: FeatureRelay, want: "Remote Access (Mobile Relay)"},
		{feature: FeatureAdvancedReporting, want: "Advanced Infrastructure Reporting (PDF/CSV)"},
		{feature: FeatureLongTermMetrics, want: "90-Day Metric History"},
		{feature: "custom_feature", want: "custom_feature"},
	}

	for _, tt := range tests {
		if got := GetFeatureDisplayName(tt.feature); got != tt.want {
			t.Fatalf("GetFeatureDisplayName(%q) = %q, want %q", tt.feature, got, tt.want)
		}
	}
}

func TestStatusHostedEvaluatorClampsNegativeLimits(t *testing.T) {
	t.Setenv("PULSE_DEV", "false")
	t.Setenv("PULSE_MOCK_MODE", "false")

	svc := NewService()
	svc.SetEvaluator(entitlements.NewEvaluator(staticEntitlementSource{
		capabilities:      []string{FeatureAIPatrol},
		limits:            map[string]int64{"max_nodes": -5, "max_guests": -10},
		subscriptionState: entitlements.SubStateActive,
	}))

	status := svc.Status()
	if !status.Valid {
		t.Fatalf("Status().Valid = %v, want true", status.Valid)
	}
	if status.MaxNodes != 0 {
		t.Fatalf("Status().MaxNodes = %d, want 0", status.MaxNodes)
	}
	if status.MaxGuests != 0 {
		t.Fatalf("Status().MaxGuests = %d, want 0", status.MaxGuests)
	}
}

