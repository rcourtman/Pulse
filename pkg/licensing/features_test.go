package licensing

import (
	"sort"
	"testing"
)

func TestDeriveCapabilitiesFromTier(t *testing.T) {
	tests := []struct {
		name             string
		tier             Tier
		explicitFeatures []string
		wantLen          int
		wantContains     []string
	}{
		{
			name:             "free_tier_includes_update_alerts",
			tier:             TierFree,
			explicitFeatures: nil,
			wantLen:          3,
			wantContains:     []string{FeatureUpdateAlerts, FeatureSSO, FeatureAIPatrol},
		},
		{
			name:             "pro_tier_includes_all_ai_features",
			tier:             TierPro,
			explicitFeatures: nil,
			wantContains:     []string{FeatureAIAutoFix, FeatureAIAlerts, FeatureRBAC, FeatureAuditLogging},
		},
		{
			name:             "msp_tier_includes_unlimited",
			tier:             TierMSP,
			explicitFeatures: nil,
			wantLen:          14,
			wantContains:     []string{FeatureUnlimited},
		},
		{
			name:             "enterprise_tier_includes_multi_tenant",
			tier:             TierEnterprise,
			explicitFeatures: nil,
			wantContains:     []string{FeatureMultiTenant, FeatureWhiteLabel},
		},
		{
			name:             "explicit_features_merged",
			tier:             TierFree,
			explicitFeatures: []string{"custom_feature"},
			wantContains:     []string{"custom_feature"},
		},
		{
			name:             "unknown_tier_returns_empty",
			tier:             Tier("unknown"),
			explicitFeatures: nil,
			wantLen:          0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveCapabilitiesFromTier(tt.tier, tt.explicitFeatures)
			if tt.wantLen > 0 && len(got) != tt.wantLen {
				t.Errorf("DeriveCapabilitiesFromTier() returned %d capabilities, want %d", len(got), tt.wantLen)
			}
			gotSet := make(map[string]bool)
			for _, f := range got {
				gotSet[f] = true
			}
			for _, f := range tt.wantContains {
				if !gotSet[f] {
					t.Errorf("DeriveCapabilitiesFromTier() missing expected feature %q", f)
				}
			}
		})
	}
}

func TestDeriveCapabilitiesFromTier_Sorted(t *testing.T) {
	got := DeriveCapabilitiesFromTier(TierPro, nil)
	if !sort.StringsAreSorted(got) {
		t.Errorf("DeriveCapabilitiesFromTier() should return sorted capabilities")
	}
}

func TestDeriveEntitlements(t *testing.T) {
	caps, limits := DeriveEntitlements(TierPro, []string{"custom"}, 50, 100)

	if len(caps) == 0 {
		t.Error("DeriveEntitlements() returned no capabilities")
	}

	if limits["max_nodes"] != 50 {
		t.Errorf("max_nodes limit = %d, want 50", limits["max_nodes"])
	}
	if limits["max_guests"] != 100 {
		t.Errorf("max_guests limit = %d, want 100", limits["max_guests"])
	}
}

func TestDeriveEntitlements_ZeroLimitsNotIncluded(t *testing.T) {
	_, limits := DeriveEntitlements(TierPro, nil, 0, 0)

	if _, ok := limits["max_nodes"]; ok {
		t.Error("max_nodes should not be in limits when 0")
	}
	if _, ok := limits["max_guests"]; ok {
		t.Error("max_guests should not be in limits when 0")
	}
}

func TestTierHasFeature(t *testing.T) {
	tests := []struct {
		tier    Tier
		feature string
		want    bool
	}{
		{TierPro, FeatureAIAutoFix, true},
		{TierPro, FeatureRBAC, true},
		{TierFree, FeatureAIAutoFix, false},
		{TierFree, FeatureUpdateAlerts, true},
		{TierEnterprise, FeatureMultiTenant, true},
		{Tier("unknown"), FeatureAIAutoFix, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.tier)+"_"+tt.feature, func(t *testing.T) {
			got := TierHasFeature(tt.tier, tt.feature)
			if got != tt.want {
				t.Errorf("TierHasFeature(%q, %q) = %v, want %v", tt.tier, tt.feature, got, tt.want)
			}
		})
	}
}

func TestGetTierDisplayName(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierFree, "Free"},
		{TierPro, "Pro Intelligence (Monthly)"},
		{TierProAnnual, "Pro Intelligence (Annual)"},
		{TierLifetime, "Pro Intelligence (Lifetime)"},
		{TierCloud, "Cloud"},
		{TierMSP, "MSP"},
		{TierEnterprise, "Enterprise"},
		{Tier("unknown"), "Unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.tier), func(t *testing.T) {
			got := GetTierDisplayName(tt.tier)
			if got != tt.want {
				t.Errorf("GetTierDisplayName(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestGetFeatureDisplayName(t *testing.T) {
	tests := []struct {
		feature string
		want    string
	}{
		{FeatureAIPatrol, "Pulse Patrol (Background Health Checks)"},
		{FeatureAIAlerts, "Alert Analysis"},
		{FeatureAIAutoFix, "Pulse Patrol Auto-Fix"},
		{FeatureKubernetesAI, "Kubernetes Analysis"},
		{FeatureUpdateAlerts, "Update Alerts (Container/Package Updates)"},
		{FeatureRBAC, "Role-Based Access Control (RBAC)"},
		{FeatureMultiUser, "Multi-User Mode"},
		{FeatureWhiteLabel, "White-Label Branding"},
		{FeatureMultiTenant, "Multi-Tenant Mode"},
		{FeatureUnlimited, "Unlimited Instances"},
		{FeatureAgentProfiles, "Centralized Agent Profiles"},
		{FeatureAuditLogging, "Enterprise Audit Logging"},
		{FeatureSSO, "Basic SSO (OIDC)"},
		{FeatureAdvancedSSO, "Advanced SSO (SAML/Multi-Provider)"},
		{FeatureRelay, "Remote Access (Mobile Relay)"},
		{FeatureAdvancedReporting, "Advanced Infrastructure Reporting (PDF/CSV)"},
		{FeatureLongTermMetrics, "90-Day Metric History"},
		{"unknown_feature", "unknown_feature"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.feature, func(t *testing.T) {
			got := GetFeatureDisplayName(tt.feature)
			if got != tt.want {
				t.Errorf("GetFeatureDisplayName(%q) = %q, want %q", tt.feature, got, tt.want)
			}
		})
	}
}
