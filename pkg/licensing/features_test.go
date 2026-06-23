package licensing

import (
	"sort"
	"strings"
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
			wantContains:     []string{FeatureUpdateAlerts, FeatureSSO, FeatureAdvancedSSO, FeatureAIPatrol},
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

func TestPatrolGovernedFixesStayProFeature(t *testing.T) {
	if !TierHasFeature(TierFree, FeatureAIPatrol) {
		t.Fatal("community tier must include watch-only Patrol with BYOK")
	}
	if TierHasFeature(TierFree, FeatureAIAutoFix) {
		t.Fatal("community tier must not include governed Patrol fixes")
	}
	if TierHasFeature(TierRelay, FeatureAIAutoFix) {
		t.Fatal("relay tier must not include governed Patrol fixes")
	}
	if !TierHasFeature(TierPro, FeatureAIAutoFix) {
		t.Fatal("pro tier must include governed Patrol fixes")
	}
}

func TestWhiteLabelBrandingRemainsExplicitEnterpriseEntitlement(t *testing.T) {
	if TierHasFeature(TierFree, FeatureWhiteLabel) {
		t.Fatal("community tier must not include white_label report branding")
	}
	if TierHasFeature(TierMSP, FeatureWhiteLabel) {
		t.Fatal("MSP tier defaults must not imply white_label unless the license grants it")
	}
	if !TierHasFeature(TierEnterprise, FeatureWhiteLabel) {
		t.Fatal("enterprise tier must keep white_label report branding entitlement")
	}

	got := DeriveCapabilitiesFromTier(TierMSP, []string{FeatureWhiteLabel})
	found := false
	for _, feature := range got {
		if feature == FeatureWhiteLabel {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("explicit white_label grants must be merged into MSP capabilities")
	}
}

func TestSelfHostedPaidFeatureClaimMatrix(t *testing.T) {
	tests := []struct {
		name                     string
		tier                     Tier
		wantHistoryDays          int
		wantIncludedCapabilities []string
		wantExcludedCapabilities []string
	}{
		{
			name:            "community keeps core monitoring free and does not claim paid extras",
			tier:            TierFree,
			wantHistoryDays: 7,
			wantIncludedCapabilities: []string{
				FeatureUpdateAlerts,
				FeatureSSO,
				FeatureAdvancedSSO,
				FeatureAIPatrol,
			},
			wantExcludedCapabilities: []string{
				FeatureRelay,
				FeatureMobileApp,
				FeaturePushNotifications,
				FeatureLongTermMetrics,
				FeatureAIAlerts,
				FeatureAIAutoFix,
				FeatureRBAC,
				FeatureAdvancedReporting,
			},
		},
		{
			name:            "relay sells remote access mobile handoff push and fourteen day history",
			tier:            TierRelay,
			wantHistoryDays: 14,
			wantIncludedCapabilities: []string{
				FeatureUpdateAlerts,
				FeatureSSO,
				FeatureAdvancedSSO,
				FeatureAIPatrol,
				FeatureRelay,
				FeatureMobileApp,
				FeaturePushNotifications,
				FeatureLongTermMetrics,
			},
			wantExcludedCapabilities: []string{
				FeatureAIAlerts,
				FeatureAIAutoFix,
				FeatureRBAC,
				FeatureAdvancedReporting,
				FeatureAgentProfiles,
			},
		},
		{
			name:            "pro sells operator extras and preserves relay capabilities",
			tier:            TierPro,
			wantHistoryDays: 90,
			wantIncludedCapabilities: []string{
				FeatureRelay,
				FeatureMobileApp,
				FeaturePushNotifications,
				FeatureLongTermMetrics,
				FeatureAIAlerts,
				FeatureAIAutoFix,
				FeatureRBAC,
				FeatureAuditLogging,
				FeatureAdvancedReporting,
				FeatureAgentProfiles,
			},
			wantExcludedCapabilities: []string{
				FeatureMultiTenant,
				FeatureUnlimited,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			gotHistoryDays, ok := TierHistoryDays[tt.tier]
			if !ok {
				t.Fatalf("TierHistoryDays missing entry for tier %q", tt.tier)
			}
			if gotHistoryDays != tt.wantHistoryDays {
				t.Fatalf(
					"TierHistoryDays[%q] = %d, want %d",
					tt.tier,
					gotHistoryDays,
					tt.wantHistoryDays,
				)
			}

			capabilities := make(map[string]struct{})
			for _, capability := range TierFeatures[tt.tier] {
				capabilities[capability] = struct{}{}
			}
			for _, capability := range tt.wantIncludedCapabilities {
				if _, ok := capabilities[capability]; !ok {
					t.Fatalf("tier %q missing paid-feature claim capability %q", tt.tier, capability)
				}
			}
			for _, capability := range tt.wantExcludedCapabilities {
				if _, ok := capabilities[capability]; ok {
					t.Fatalf("tier %q unexpectedly includes capability %q", tt.tier, capability)
				}
			}
		})
	}
}

func TestDeriveEntitlements(t *testing.T) {
	caps, limits := DeriveEntitlements(TierPro, []string{"custom"}, 50, 100)

	if len(caps) == 0 {
		t.Error("DeriveEntitlements() returned no capabilities")
	}

	if _, ok := limits["max_monitored_systems"]; ok {
		t.Errorf("DeriveEntitlements() exposed retired max_monitored_systems limit: %v", limits)
	}
	if limits["max_guests"] != 100 {
		t.Errorf("max_guests limit = %d, want 100", limits["max_guests"])
	}
}

func TestDeriveEntitlements_ZeroLimitsNotIncluded(t *testing.T) {
	_, limits := DeriveEntitlements(TierPro, nil, 0, 0)

	if _, ok := limits["max_monitored_systems"]; ok {
		t.Error("max_monitored_systems should not be in limits when 0")
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
		{TierFree, FeatureAdvancedSSO, true},
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
		{FeatureAIAlerts, "Patrol Investigates Issues"},
		{FeatureAIAutoFix, "Patrol Handles Safe Fixes"},
		{FeatureKubernetesAI, "Kubernetes AI Analysis (Compatibility)"},
		{FeatureUpdateAlerts, "Update Alerts (Container/Package Updates)"},
		{FeatureRBAC, "Role-Based Access Control (RBAC)"},
		{FeatureMultiUser, "Multi-User Mode"},
		{FeatureWhiteLabel, "White-Label Branding"},
		{FeatureMultiTenant, "Multi-Tenant Mode"},
		{FeatureUnlimited, "Hosted Capacity Policy"},
		{FeatureAgentProfiles, "Centralized Agent Profiles"},
		{FeatureAuditLogging, "Audit Logging"},
		{FeatureSSO, "Core SSO (OIDC/SAML)"},
		{FeatureAdvancedSSO, "Multi-Provider SSO"},
		{FeatureRelay, "Pulse Relay (Remote Access)"},
		{FeatureMobileApp, "Pulse Mobile Pairing"},
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

func TestIsCompatibilityOnlyFeature(t *testing.T) {
	if !IsCompatibilityOnlyFeature(FeatureKubernetesAI) {
		t.Fatalf("expected %q to remain compatibility-only", FeatureKubernetesAI)
	}
	if IsCompatibilityOnlyFeature(FeatureAIAlerts) {
		t.Fatalf("did not expect %q to be compatibility-only", FeatureAIAlerts)
	}
}

func TestSelfHostedFeatureMetadataKeepsCanonicalPlanLabelsAndVisibility(t *testing.T) {
	updateAlerts, ok := GetFeatureMetadata(FeatureUpdateAlerts)
	if !ok {
		t.Fatalf("expected metadata for %q", FeatureUpdateAlerts)
	}
	if updateAlerts.DisplayName != "Update Alerts (Container/Package Updates)" {
		t.Fatalf("DisplayName = %q, want detailed runtime label", updateAlerts.DisplayName)
	}
	if updateAlerts.ComparisonName != "Update Alerts" {
		t.Fatalf("ComparisonName = %q, want customer-facing label", updateAlerts.ComparisonName)
	}

	relay, ok := GetFeatureMetadata(FeatureRelay)
	if !ok {
		t.Fatalf("expected metadata for %q", FeatureRelay)
	}
	if relay.ComparisonName != "Pulse Relay (Remote Access)" {
		t.Fatalf("ComparisonName = %q, want canonical Relay marketing label", relay.ComparisonName)
	}
	if GetSelfHostedFeatureRole(FeatureRelay, TierRelay) != SelfHostedFeatureRolePrimaryPillar {
		t.Fatalf("expected %q to stay a Relay primary pillar", FeatureRelay)
	}

	multiTenant, ok := GetFeatureMetadata(FeatureMultiTenant)
	if !ok {
		t.Fatalf("expected metadata for %q", FeatureMultiTenant)
	}
	if multiTenant.DisplayableInPlanUI {
		t.Fatalf("did not expect MSP-only %q to be displayable in self-hosted plan UI", FeatureMultiTenant)
	}
	if GetSelfHostedFeatureRole(FeatureMultiTenant, TierPro) != SelfHostedFeatureRoleHidden {
		t.Fatalf("expected %q to stay hidden from the self-hosted Pro plan role", FeatureMultiTenant)
	}

	kubernetes, ok := GetFeatureMetadata(FeatureKubernetesAI)
	if !ok {
		t.Fatalf("expected metadata for %q", FeatureKubernetesAI)
	}
	if kubernetes.DisplayableInPlanUI {
		t.Fatalf("did not expect compatibility-only %q to be displayable in plan UI", FeatureKubernetesAI)
	}
}

func TestSelfHostedFeatureMetadataMatchesRuntimeTierFeatures(t *testing.T) {
	for _, tier := range []Tier{TierFree, TierRelay, TierPro, TierProPlus, TierProAnnual, TierLifetime} {
		tier := tier
		t.Run(string(tier), func(t *testing.T) {
			got := sortedFeatureSet(TierFeatures[tier])
			want := selfHostedMetadataFeatureKeysForTier(tier)
			if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
				t.Fatalf("TierFeatures[%q]=%v, want self-hosted metadata features %v", tier, got, want)
			}
		})
	}
}

func selfHostedMetadataFeatureKeysForTier(tier Tier) []string {
	out := make([]string, 0)
	for _, entry := range AllFeatureMetadata() {
		if GetSelfHostedFeatureRole(entry.Key, tier) == SelfHostedFeatureRoleHidden {
			continue
		}
		out = append(out, entry.Key)
	}
	return sortedFeatureSet(out)
}

func sortedFeatureSet(features []string) []string {
	seen := make(map[string]struct{}, len(features))
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		seen[feature] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for feature := range seen {
		out = append(out, feature)
	}
	sort.Strings(out)
	return out
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

	// Pro+ is a legacy continuity tier, not a bigger self-hosted monitoring tier.
	proPlusSet := asSet(TierFeatures[TierProPlus])
	if len(proPlusSet) != len(proSet) {
		t.Fatalf("Pro+ feature count = %d, want Pro feature count %d", len(proPlusSet), len(proSet))
	}
	for f := range proSet {
		if !proPlusSet[f] {
			t.Errorf("Pro+ tier missing Pro feature %q", f)
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

func TestPublicTierDefaultsNeverIncludeInternalCapabilities(t *testing.T) {
	for tier, features := range TierFeatures {
		for _, feature := range features {
			if !CapabilityVisibleInPublicPayload(feature) {
				t.Fatalf("TierFeatures[%q] leaked internal capability %q", tier, feature)
			}
		}
	}
}

func TestFilterPublicCapabilitiesStripsInternalOnlyFeatures(t *testing.T) {
	got := FilterPublicCapabilities([]string{
		FeatureRelay,
		FeatureDemoFixtures,
		FeatureAIPatrol,
	})
	want := []string{FeatureRelay, FeatureAIPatrol}
	if len(got) != len(want) {
		t.Fatalf("FilterPublicCapabilities() len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i, feature := range want {
		if got[i] != feature {
			t.Fatalf("FilterPublicCapabilities()[%d]=%q, want %q", i, got[i], feature)
		}
	}
}

// TestAllTiersHaveHistoryDays ensures every tier in TierFeatures also has a
// metrics-retention entry. Core monitoring volume limits are retired.
func TestAllTiersHaveHistoryDays(t *testing.T) {
	for tier := range TierFeatures {
		if _, ok := TierHistoryDays[tier]; !ok {
			t.Errorf("TierHistoryDays missing entry for tier %q", tier)
		}
	}
}

func TestWorkspaceLimitForPlan_KnownPlans(t *testing.T) {
	tests := []struct {
		plan      string
		wantLimit int
	}{
		{"cloud_starter", 1},
		{"cloud_power", 1},
		{"cloud_max", 1},
		{"cloud_founding", 1},
		{"msp_starter", 5},
		{"msp_hosted_v1", 5},
		{"msp_growth", 15},
		{"msp_scale", 40},
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			limit, known := WorkspaceLimitForPlan(tt.plan)
			if !known {
				t.Errorf("WorkspaceLimitForPlan(%q): known = false, want true", tt.plan)
			}
			if limit != tt.wantLimit {
				t.Errorf("WorkspaceLimitForPlan(%q) = %d, want %d", tt.plan, limit, tt.wantLimit)
			}
		})
	}
}

func TestWorkspaceLimitForPlan_UnknownPlanFailsClosed(t *testing.T) {
	unknownPlans := []string{"stripe", "", "unknown_plan", "cloud_unknown"}

	for _, plan := range unknownPlans {
		t.Run(plan, func(t *testing.T) {
			limit, known := WorkspaceLimitForPlan(plan)
			if known {
				t.Errorf("WorkspaceLimitForPlan(%q): known = true, want false", plan)
			}
			if limit != UnknownPlanDefaultWorkspaceLimit {
				t.Errorf("WorkspaceLimitForPlan(%q) = %d, want default %d", plan, limit, UnknownPlanDefaultWorkspaceLimit)
			}
		})
	}
}

// TestCloudWorkspacePlansHaveWorkspaceLimits ensures hosted workspace plans
// still carry a workspace limit entry after monitored-system caps were retired.
func TestCloudWorkspacePlansHaveWorkspaceLimits(t *testing.T) {
	for plan := range CloudPlanWorkspaceLimits {
		if !strings.HasPrefix(plan, "cloud_") && !strings.HasPrefix(plan, "msp_") {
			t.Errorf("CloudPlanWorkspaceLimits includes non-hosted plan %q", plan)
		}
	}
}

func TestPlanVersionForPriceID_KnownPrices(t *testing.T) {
	tests := []struct {
		priceID  string
		wantPlan string
	}{
		// Cloud Starter (monthly, annual, founding)
		{"price_1T5kflBrHBocJIGHUqPv1dzV", "cloud_starter"},
		{"price_1T5kfmBrHBocJIGHTS3ymKxM", "cloud_starter"},
		{"price_1T5kfnBrHBocJIGHATQJr79D", "cloud_founding"},
		// Cloud Power
		{"price_1T5kg2BrHBocJIGHmkoF0zXY", "cloud_power"},
		{"price_1T5kg3BrHBocJIGH2EtzKofV", "cloud_power"},
		// Cloud Max
		{"price_1T5kg4BrHBocJIGHHa8Ecqho", "cloud_max"},
		{"price_1T5kg5BrHBocJIGH5AIJ4nVc", "cloud_max"},
		// MSP Starter
		{"price_1T5kgTBrHBocJIGHjOs15LI2", "msp_starter"},
		{"price_1T5kgUBrHBocJIGHT6PiOn6x", "msp_starter"},
		// MSP Growth
		{"price_1T5kgVBrHBocJIGHulNsCTb1", "msp_growth"},
		{"price_1T5kgWBrHBocJIGHTuaNjnJ2", "msp_growth"},
		// MSP Scale
		{"price_1T5kgWBrHBocJIGHo40iFeRd", "msp_scale"},
		{"price_1T5kgXBrHBocJIGHWlOgTyGV", "msp_scale"},
		// Grandfathered Pulse Pro recurring renewals (v5)
		{"price_1ShIsdBrHBocJIGH71yQusLG", "v5_pro_monthly_grandfathered"},
		{"price_1ShIsnBrHBocJIGHBKkzsZ3T", "v5_pro_annual_grandfathered"},
		// Grandfathered Pulse Pro recurring renewals (legacy v1)
		{"price_1SgDxvBrHBocJIGHStaGuiAX", "v5_pro_monthly_grandfathered"},
		{"price_1SgDxwBrHBocJIGHTKTsIMLc", "v5_pro_annual_grandfathered"},
	}

	for _, tt := range tests {
		t.Run(tt.priceID, func(t *testing.T) {
			plan, ok := PlanVersionForPriceID(tt.priceID)
			if !ok {
				t.Fatalf("PlanVersionForPriceID(%q): not found", tt.priceID)
			}
			if plan != tt.wantPlan {
				t.Errorf("PlanVersionForPriceID(%q) = %q, want %q", tt.priceID, plan, tt.wantPlan)
			}
		})
	}
}

func TestPlanVersionForPriceID_UnknownPrices(t *testing.T) {
	unknowns := []string{"price_unknown", "", "not_a_price"}
	for _, id := range unknowns {
		t.Run(id, func(t *testing.T) {
			_, ok := PlanVersionForPriceID(id)
			if ok {
				t.Errorf("PlanVersionForPriceID(%q): expected not found", id)
			}
		})
	}
}

func TestIsGrandfatheredRecurringV5PlanVersion(t *testing.T) {
	tests := []struct {
		plan string
		want bool
	}{
		{plan: "v5_pro_monthly_grandfathered", want: true},
		{plan: "v5_pro_annual_grandfathered", want: true},
		{plan: "price_1ShIsdBrHBocJIGH71yQusLG", want: false},
		{plan: "cloud_starter", want: false},
		{plan: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			if got := IsGrandfatheredRecurringV5PlanVersion(tt.plan); got != tt.want {
				t.Fatalf("IsGrandfatheredRecurringV5PlanVersion(%q) = %v, want %v", tt.plan, got, tt.want)
			}
		})
	}
}

func TestIsSelfHostedCommunityPlanVersion(t *testing.T) {
	tests := []struct {
		plan string
		want bool
	}{
		{plan: "community", want: true},
		{plan: "Community", want: true},
		{plan: "free", want: true},
		{plan: "relay", want: false},
		{plan: "pro", want: false},
		{plan: "pro_plus", want: false},
		{plan: "pro_annual", want: false},
		{plan: "lifetime", want: false},
		{plan: "v5_lifetime_grandfathered", want: false},
		{plan: "cloud_starter", want: false},
		{plan: "trial", want: false},
		{plan: "pro-v2", want: false},
		{plan: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			if got := IsSelfHostedCommunityPlanVersion(tt.plan); got != tt.want {
				t.Fatalf("IsSelfHostedCommunityPlanVersion(%q) = %v, want %v", tt.plan, got, tt.want)
			}
		})
	}
}

func TestIsSelfHostedCoreMonitoringUncappedPlanVersion(t *testing.T) {
	tests := []struct {
		plan string
		want bool
	}{
		{plan: "community", want: true},
		{plan: "free", want: true},
		{plan: "relay", want: true},
		{plan: "pro", want: true},
		{plan: "pro_plus", want: true},
		{plan: "pro_annual", want: true},
		{plan: "lifetime", want: true},
		{plan: "v5_lifetime_grandfathered", want: true},
		{plan: "v5_pro_monthly_grandfathered", want: true},
		{plan: "v5_pro_annual_grandfathered", want: true},
		{plan: "cloud_starter", want: false},
		{plan: "msp_starter", want: false},
		{plan: "pro-v2", want: false},
		{plan: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			if got := IsSelfHostedCoreMonitoringUncappedPlanVersion(tt.plan); got != tt.want {
				t.Fatalf("IsSelfHostedCoreMonitoringUncappedPlanVersion(%q) = %v, want %v", tt.plan, got, tt.want)
			}
		})
	}
}

func TestIsSelfHostedCoreMonitoringUncappedTier(t *testing.T) {
	tests := []struct {
		tier Tier
		want bool
	}{
		{tier: TierFree, want: true},
		{tier: TierRelay, want: true},
		{tier: TierPro, want: true},
		{tier: TierProPlus, want: true},
		{tier: TierProAnnual, want: true},
		{tier: TierLifetime, want: true},
		{tier: TierCloud, want: false},
		{tier: TierMSP, want: false},
		{tier: TierEnterprise, want: false},
		{tier: Tier(" pro "), want: true},
		{tier: Tier("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := IsSelfHostedCoreMonitoringUncappedTier(tt.tier); got != tt.want {
				t.Fatalf("IsSelfHostedCoreMonitoringUncappedTier(%q) = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

// TestPriceIDToPlanVersion_HostedPlansHaveWorkspaceLimits ensures hosted plans
// in the price map retain workspace policy after monitored-system caps retire.
func TestPriceIDToPlanVersion_AllMapToKnownPlans(t *testing.T) {
	for priceID, plan := range PriceIDToPlanVersion {
		t.Run(priceID, func(t *testing.T) {
			if !strings.HasPrefix(plan, "cloud_") && !strings.HasPrefix(plan, "msp_") {
				return
			}
			if _, known := WorkspaceLimitForPlan(plan); !known {
				t.Errorf("PriceIDToPlanVersion[%q] = %q, but WorkspaceLimitForPlan does not recognize it", priceID, plan)
			}
		})
	}
}
