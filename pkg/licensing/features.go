// Package licensing defines shared Pulse feature and tier contracts.
//
// This package exists so private extension modules can depend on canonical
// licensing metadata without importing internal packages.
package licensing

import (
	"sort"
	"time"
)

// Feature constants represent gated features in Pulse.
// These are embedded in license JWTs and checked at runtime.
const (
	// Free tier features
	FeatureUpdateAlerts = "update_alerts" // Alerts for pending container/package updates
	FeatureSSO          = "sso"           // Basic OIDC/SSO authentication
	FeatureAIPatrol     = "ai_patrol"     // Background AI health monitoring (BYOK, free with own key)

	// Relay tier features (everything in Free, plus:)
	FeatureRelay             = "relay"              // Relay remote access
	FeatureMobileApp         = "mobile_app"         // Mobile app access
	FeaturePushNotifications = "push_notifications" // Push notifications
	FeatureLongTermMetrics   = "long_term_metrics"  // Extended historical metrics (14d Relay, 90d Pro)

	// Pro tier features (everything in Relay, plus:)
	FeatureAIAlerts          = "ai_alerts"          // AI analysis when alerts fire
	FeatureAIAutoFix         = "ai_autofix"         // Automatic remediation (one-click apply)
	FeatureKubernetesAI      = "kubernetes_ai"      // AI analysis of K8s (NOT basic monitoring)
	FeatureAgentProfiles     = "agent_profiles"     // Centralized agent configuration profiles
	FeatureRBAC              = "rbac"               // Role-Based Access Control
	FeatureAuditLogging      = "audit_logging"      // Persistent audit logs with signing
	FeatureAdvancedSSO       = "advanced_sso"       // SAML, Multi-provider, Role Mapping
	FeatureAdvancedReporting = "advanced_reporting" // PDF/CSV reporting engine

	// MSP/Enterprise tier features
	FeatureMultiUser   = "multi_user"   // Multi-user (likely merged with RBAC)
	FeatureWhiteLabel  = "white_label"  // Custom branding - NOT IMPLEMENTED YET
	FeatureMultiTenant = "multi_tenant" // Multi-tenant organizations
	FeatureUnlimited   = "unlimited"    // Unlimited instances (for MSP/volume deals)
)

// Tier represents a license tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierRelay      Tier = "relay"
	TierPro        Tier = "pro"
	TierProPlus    Tier = "pro_plus"
	TierProAnnual  Tier = "pro_annual" // Legacy: same features as TierPro
	TierLifetime   Tier = "lifetime"   // Legacy: same features as TierPro
	TierCloud      Tier = "cloud"
	TierMSP        Tier = "msp"
	TierEnterprise Tier = "enterprise"
)

// TierAgentLimits defines the maximum agent count per tier.
// A value of 0 means unlimited (enforced at the MSP/Enterprise level).
var TierAgentLimits = map[Tier]int{
	TierFree:       5,
	TierRelay:      8,
	TierPro:        15,
	TierProPlus:    50,
	TierProAnnual:  15, // Legacy: same as Pro
	TierLifetime:   15, // Legacy: same as Pro
	TierCloud:      0,  // Cloud tiers have per-plan limits set in license claims
	TierMSP:        0,  // MSP tiers have per-plan pool limits set in license claims
	TierEnterprise: 0,  // Custom
}

// TierHistoryDays defines the maximum metrics history retention per tier.
var TierHistoryDays = map[Tier]int{
	TierFree:       7,
	TierRelay:      14,
	TierPro:        90,
	TierProPlus:    90,
	TierProAnnual:  90,
	TierLifetime:   90,
	TierCloud:      90,
	TierMSP:        90,
	TierEnterprise: 90,
}

// freeFeatures are the base capabilities available to all users.
var freeFeatures = []string{
	FeatureUpdateAlerts,
	FeatureSSO,
	FeatureAIPatrol, // Patrol is free with BYOK — auto-fix requires Pro
}

// relayFeatures adds remote access and mobile on top of free.
var relayFeatures = appendFeatures(freeFeatures,
	FeatureRelay,
	FeatureMobileApp,
	FeaturePushNotifications,
	FeatureLongTermMetrics, // 14 days (vs 7 for free)
)

// proFeatures adds AI automation, fleet management, and compliance on top of relay.
var proFeatures = appendFeatures(relayFeatures,
	FeatureAIAlerts,
	FeatureAIAutoFix,
	FeatureKubernetesAI,
	FeatureAgentProfiles,
	FeatureAdvancedSSO,
	FeatureRBAC,
	FeatureAuditLogging,
	FeatureAdvancedReporting,
)

// mspFeatures adds multi-tenant and unlimited on top of pro.
var mspFeatures = appendFeatures(proFeatures,
	FeatureUnlimited,
	FeatureMultiTenant,
)

// enterpriseFeatures adds white-label and multi-user on top of MSP.
var enterpriseFeatures = appendFeatures(mspFeatures,
	FeatureMultiUser,
	FeatureWhiteLabel,
)

// appendFeatures returns a new slice with extra features appended (no mutation).
func appendFeatures(base []string, extra ...string) []string {
	result := make([]string, len(base), len(base)+len(extra))
	copy(result, base)
	return append(result, extra...)
}

// TierFeatures maps each tier to its included features.
var TierFeatures = map[Tier][]string{
	TierFree:       freeFeatures,
	TierRelay:      relayFeatures,
	TierPro:        proFeatures,
	TierProPlus:    proFeatures, // Same features as Pro, just higher host limit
	TierProAnnual:  proFeatures, // Legacy: same features as Pro
	TierLifetime:   proFeatures, // Legacy: same features as Pro
	TierCloud:      proFeatures, // Cloud includes all Pro features + managed hosting
	TierMSP:        mspFeatures,
	TierEnterprise: enterpriseFeatures,
}

// DeriveCapabilitiesFromTier derives effective capabilities from tier and explicit features.
func DeriveCapabilitiesFromTier(tier Tier, explicitFeatures []string) []string {
	featureSet := make(map[string]struct{})
	for _, feature := range TierFeatures[tier] {
		featureSet[feature] = struct{}{}
	}
	for _, feature := range explicitFeatures {
		featureSet[feature] = struct{}{}
	}

	capabilities := make([]string, 0, len(featureSet))
	for feature := range featureSet {
		capabilities = append(capabilities, feature)
	}
	sort.Strings(capabilities)
	return capabilities
}

// DeriveEntitlements derives capabilities and limits from tier and legacy claim fields.
func DeriveEntitlements(tier Tier, features []string, maxAgents int, maxGuests int) (capabilities []string, limits map[string]int64) {
	capabilities = DeriveCapabilitiesFromTier(tier, features)

	limits = make(map[string]int64)
	if maxAgents > 0 {
		limits["max_agents"] = int64(maxAgents)
	}
	if maxGuests > 0 {
		limits["max_guests"] = int64(maxGuests)
	}

	return capabilities, limits
}

// OnboardingOverflowDuration is the window during which free-tier workspaces
// receive +1 host slot after initial setup.
const OnboardingOverflowDuration = 14 * 24 * time.Hour

// OverflowBonus returns the number of bonus host slots granted by the
// onboarding overflow. Returns 1 if the tier is free, overflowGrantedAt
// is set, and the current time is within 14 days of the grant. Otherwise 0.
func OverflowBonus(tier Tier, overflowGrantedAt *int64, now time.Time) int {
	if tier != TierFree || overflowGrantedAt == nil {
		return 0
	}
	grantedAt := time.Unix(*overflowGrantedAt, 0)
	elapsed := now.Sub(grantedAt)
	if elapsed < 0 {
		// Future timestamp — treat as not yet granted.
		return 0
	}
	if elapsed < OnboardingOverflowDuration {
		return 1
	}
	return 0
}

// OverflowDaysRemaining returns the number of days remaining in the overflow
// window. Returns 0 if overflow is not active.
func OverflowDaysRemaining(tier Tier, overflowGrantedAt *int64, now time.Time) int {
	if OverflowBonus(tier, overflowGrantedAt, now) == 0 {
		return 0
	}
	grantedAt := time.Unix(*overflowGrantedAt, 0)
	expiresAt := grantedAt.Add(OnboardingOverflowDuration)
	remaining := expiresAt.Sub(now)
	days := int(remaining.Hours()/24) + 1 // ceiling: partial day counts as 1
	if days < 0 {
		return 0
	}
	return days
}

// TierHasFeature checks if a tier includes a specific feature.
func TierHasFeature(tier Tier, feature string) bool {
	features, ok := TierFeatures[tier]
	if !ok {
		return false
	}
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// GetTierDisplayName returns a human-readable name for the tier.
func GetTierDisplayName(tier Tier) string {
	switch tier {
	case TierFree:
		return "Community"
	case TierRelay:
		return "Relay"
	case TierPro:
		return "Pro"
	case TierProPlus:
		return "Pro+"
	case TierProAnnual:
		return "Pro (Annual)"
	case TierLifetime:
		return "Pro (Lifetime)"
	case TierCloud:
		return "Cloud"
	case TierMSP:
		return "MSP"
	case TierEnterprise:
		return "Enterprise"
	default:
		return "Unknown"
	}
}

// GetFeatureMinTierName returns the display name of the lowest tier that includes the given feature.
// This is used for user-facing messages like "requires Pulse Relay or above".
// The tier ordering is: Free < Relay < Pro < MSP < Enterprise.
func GetFeatureMinTierName(feature string) string {
	orderedTiers := []Tier{TierFree, TierRelay, TierPro, TierMSP, TierEnterprise}
	for _, tier := range orderedTiers {
		if TierHasFeature(tier, feature) {
			return GetTierDisplayName(tier)
		}
	}
	return "Pro" // fallback
}

// GetFeatureDisplayName returns a human-readable name for a feature.
func GetFeatureDisplayName(feature string) string {
	switch feature {
	case FeatureAIPatrol:
		return "Pulse Patrol (Background Health Checks)"
	case FeatureAIAlerts:
		return "Alert Analysis"
	case FeatureAIAutoFix:
		return "Pulse Patrol Auto-Fix"
	case FeatureKubernetesAI:
		return "Kubernetes Analysis"
	case FeatureUpdateAlerts:
		return "Update Alerts (Container/Package Updates)"
	case FeatureRBAC:
		return "Role-Based Access Control (RBAC)"
	case FeatureMultiUser:
		return "Multi-User Mode"
	case FeatureWhiteLabel:
		return "White-Label Branding"
	case FeatureMultiTenant:
		return "Multi-Tenant Mode"
	case FeatureUnlimited:
		return "Unlimited Instances"
	case FeatureAgentProfiles:
		return "Centralized Agent Profiles"
	case FeatureAuditLogging:
		return "Audit Logging"
	case FeatureSSO:
		return "Basic SSO (OIDC)"
	case FeatureAdvancedSSO:
		return "Advanced SSO (SAML/Multi-Provider)"
	case FeatureRelay:
		return "Pulse Relay (Remote Access)"
	case FeatureMobileApp:
		return "Mobile App Access"
	case FeaturePushNotifications:
		return "Push Notifications"
	case FeatureAdvancedReporting:
		return "PDF/CSV Reporting"
	case FeatureLongTermMetrics:
		return "Extended Metric History"
	default:
		return feature
	}
}
