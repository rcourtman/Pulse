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
		wantNotContains  []string
	}{
		{
			name:             "free_tier_includes_base_features",
			tier:             TierFree,
			explicitFeatures: nil,
			wantLen:          len(freeFeatures),
			wantContains:     []string{FeatureUpdateAlerts, FeatureSSO, FeatureAIPatrol},
			wantNotContains:  []string{FeatureRelay, FeatureAIAutoFix, FeatureLongTermMetrics},
		},
		{
			name:             "relay_tier_includes_remote_access",
			tier:             TierRelay,
			explicitFeatures: nil,
			wantLen:          len(relayFeatures),
			wantContains:     []string{FeatureRelay, FeatureMobileApp, FeaturePushNotifications, FeatureLongTermMetrics},
			wantNotContains:  []string{FeatureAIAutoFix, FeatureAIAlerts, FeatureRBAC},
		},
		{
			name:             "pro_tier_includes_all_ai_and_compliance",
			tier:             TierPro,
			explicitFeatures: nil,
			wantLen:          len(proFeatures),
			wantContains:     []string{FeatureAIAutoFix, FeatureAIAlerts, FeatureRBAC, FeatureAuditLogging, FeatureRelay},
			wantNotContains:  []string{FeatureUnlimited, FeatureMultiTenant},
		},
		{
			name:             "pro_plus_same_features_as_pro",
			tier:             TierProPlus,
			explicitFeatures: nil,
			wantLen:          len(proFeatures),
			wantContains:     []string{FeatureAIAutoFix, FeatureRelay, FeatureRBAC},
		},
		{
			name:             "pro_annual_legacy_same_as_pro",
			tier:             TierProAnnual,
			explicitFeatures: nil,
			wantLen:          len(proFeatures),
			wantContains:     []string{FeatureAIAutoFix, FeatureRelay},
		},
		{
			name:             "lifetime_legacy_same_as_pro",
			tier:             TierLifetime,
			explicitFeatures: nil,
			wantLen:          len(proFeatures),
			wantContains:     []string{FeatureAIAutoFix, FeatureRelay},
		},
		{
			name:             "cloud_includes_pro_features",
			tier:             TierCloud,
			explicitFeatures: nil,
			wantLen:          len(proFeatures),
			wantContains:     []string{FeatureAIAutoFix, FeatureRelay, FeatureLongTermMetrics},
		},
		{
			name:             "msp_tier_includes_unlimited_and_multi_tenant",
			tier:             TierMSP,
			explicitFeatures: nil,
			wantLen:          len(mspFeatures),
			wantContains:     []string{FeatureUnlimited, FeatureMultiTenant},
		},
		{
			name:             "enterprise_tier_includes_white_label",
			tier:             TierEnterprise,
			explicitFeatures: nil,
			wantLen:          len(enterpriseFeatures),
			wantContains:     []string{FeatureMultiTenant, FeatureWhiteLabel, FeatureMultiUser},
		},
		{
			name:             "explicit_features_merged",
			tier:             TierFree,
			explicitFeatures: []string{"custom_feature"},
			wantContains:     []string{"custom_feature", FeatureUpdateAlerts},
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
			for _, f := range tt.wantNotContains {
				if gotSet[f] {
					t.Errorf("DeriveCapabilitiesFromTier() should NOT include feature %q for tier %q", f, tt.tier)
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
		{TierPro, FeatureRelay, true},
		{TierFree, FeatureAIAutoFix, false},
		{TierFree, FeatureRelay, false},
		{TierFree, FeatureUpdateAlerts, true},
		{TierFree, FeatureAIPatrol, true},
		{TierRelay, FeatureRelay, true},
		{TierRelay, FeatureMobileApp, true},
		{TierRelay, FeaturePushNotifications, true},
		{TierRelay, FeatureLongTermMetrics, true},
		{TierRelay, FeatureAIAutoFix, false},
		{TierRelay, FeatureRBAC, false},
		{TierProPlus, FeatureAIAutoFix, true},
		{TierProPlus, FeatureRelay, true},
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
		{TierFree, "Community"},
		{TierRelay, "Relay"},
		{TierPro, "Pro"},
		{TierProPlus, "Pro+"},
		{TierProAnnual, "Pro (Annual)"},
		{TierLifetime, "Pro (Lifetime)"},
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
		{FeatureAuditLogging, "Audit Logging"},
		{FeatureSSO, "Basic SSO (OIDC)"},
		{FeatureAdvancedSSO, "Advanced SSO (SAML/Multi-Provider)"},
		{FeatureRelay, "Pulse Relay (Remote Access)"},
		{FeatureMobileApp, "Mobile App Access"},
		{FeaturePushNotifications, "Push Notifications"},
		{FeatureAdvancedReporting, "PDF/CSV Reporting"},
		{FeatureLongTermMetrics, "Extended Metric History"},
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

func TestTierHostLimits(t *testing.T) {
	tests := []struct {
		tier Tier
		want int
	}{
		{TierFree, 5},
		{TierRelay, 8},
		{TierPro, 15},
		{TierProPlus, 50},
		{TierProAnnual, 15},
		{TierLifetime, 15},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.tier), func(t *testing.T) {
			got, ok := TierHostLimits[tt.tier]
			if !ok {
				t.Fatalf("TierHostLimits missing entry for tier %q", tt.tier)
			}
			if got != tt.want {
				t.Errorf("TierHostLimits[%q] = %d, want %d", tt.tier, got, tt.want)
			}
		})
	}
}

func TestTierHistoryDays(t *testing.T) {
	tests := []struct {
		tier Tier
		want int
	}{
		{TierFree, 7},
		{TierRelay, 14},
		{TierPro, 90},
		{TierProPlus, 90},
		{TierCloud, 90},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.tier), func(t *testing.T) {
			got, ok := TierHistoryDays[tt.tier]
			if !ok {
				t.Fatalf("TierHistoryDays missing entry for tier %q", tt.tier)
			}
			if got != tt.want {
				t.Errorf("TierHistoryDays[%q] = %d, want %d", tt.tier, got, tt.want)
			}
		})
	}
}

// TestTierFeatureInheritance verifies that higher tiers include all features from lower tiers.
func TestTierFeatureInheritance(t *testing.T) {
	asSet := func(features []string) map[string]bool {
		s := make(map[string]bool, len(features))
		for _, f := range features {
			s[f] = true
		}
		return s
	}

	// Relay must include all Free features.
	freeSet := asSet(TierFeatures[TierFree])
	relaySet := asSet(TierFeatures[TierRelay])
	for f := range freeSet {
		if !relaySet[f] {
			t.Errorf("Relay tier missing Free feature %q", f)
		}
	}

	// Pro must include all Relay features.
	proSet := asSet(TierFeatures[TierPro])
	for f := range relaySet {
		if !proSet[f] {
			t.Errorf("Pro tier missing Relay feature %q", f)
		}
	}

	// MSP must include all Pro features.
	mspSet := asSet(TierFeatures[TierMSP])
	for f := range proSet {
		if !mspSet[f] {
			t.Errorf("MSP tier missing Pro feature %q", f)
		}
	}

	// Enterprise must include all MSP features.
	entSet := asSet(TierFeatures[TierEnterprise])
	for f := range mspSet {
		if !entSet[f] {
			t.Errorf("Enterprise tier missing MSP feature %q", f)
		}
	}
}

// TestAllTiersHaveHostLimitsAndHistoryDays ensures every tier in TierFeatures
// also has entries in TierHostLimits and TierHistoryDays.
func TestAllTiersHaveHostLimitsAndHistoryDays(t *testing.T) {
	for tier := range TierFeatures {
		if _, ok := TierHostLimits[tier]; !ok {
			t.Errorf("TierHostLimits missing entry for tier %q", tier)
		}
		if _, ok := TierHistoryDays[tier]; !ok {
			t.Errorf("TierHistoryDays missing entry for tier %q", tier)
		}
	}
}
