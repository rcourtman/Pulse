// Package license handles Pulse Pro license validation and feature gating.
package license

import "sort"

// Feature constants represent gated features in Pulse Pro.
// These are embedded in license JWTs and checked at runtime.
const (
	// Pro tier features - AI
	FeatureAIPatrol     = "ai_patrol"     // Background AI health monitoring
	FeatureAIAlerts     = "ai_alerts"     // AI analysis when alerts fire
	FeatureAIAutoFix    = "ai_autofix"    // Automatic remediation
	FeatureKubernetesAI = "kubernetes_ai" // AI analysis of K8s (NOT basic monitoring)

	// Pro tier features - Fleet Management
	FeatureAgentProfiles = "agent_profiles" // Centralized agent configuration profiles

	// Free tier features - Monitoring
	FeatureUpdateAlerts = "update_alerts" // Alerts for pending container/package updates (free feature)

	// Pro tier features - Team & Compliance
	FeatureRBAC              = "rbac"               // Role-Based Access Control
	FeatureAuditLogging      = "audit_logging"      // Persistent audit logs with signing
	FeatureSSO               = "sso"                // OIDC/SSO authentication (Basic)
	FeatureAdvancedSSO       = "advanced_sso"       // SAML, Multi-provider, Role Mapping
	FeatureAdvancedReporting = "advanced_reporting" // PDF/CSV reporting engine
	FeatureLongTermMetrics   = "long_term_metrics"  // 90-day historical metrics (SQLite)

	// Pro tier features - Remote Access
	FeatureRelay = "relay" // Mobile relay for remote access

	// MSP/Enterprise tier features (for volume deals)
	FeatureMultiUser   = "multi_user"   // Multi-user (likely merged with RBAC)
	FeatureWhiteLabel  = "white_label"  // Custom branding - NOT IMPLEMENTED YET
	FeatureMultiTenant = "multi_tenant" // Multi-tenant organizations
	FeatureUnlimited   = "unlimited"    // Unlimited instances (for MSP/volume deals)
)

// Tier represents a license tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierProAnnual  Tier = "pro_annual"
	TierLifetime   Tier = "lifetime"
	TierMSP        Tier = "msp"
	TierEnterprise Tier = "enterprise"
)

// TierFeatures maps each tier to its included features.
var TierFeatures = map[Tier][]string{
	TierFree: {
		// Free tier includes update alerts (container image updates) - basic monitoring feature
		FeatureUpdateAlerts,
		FeatureSSO,
		FeatureAIPatrol, // Patrol is free with BYOK - auto-fix requires Pro
	},
	TierPro: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
		FeatureRelay,
		FeatureSSO,
		FeatureAdvancedSSO,
		FeatureRBAC,
		FeatureAuditLogging,
		FeatureAdvancedReporting,
		FeatureLongTermMetrics,
	},
	TierProAnnual: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
		FeatureRelay,
		FeatureSSO,
		FeatureAdvancedSSO,
		FeatureRBAC,
		FeatureAuditLogging,
		FeatureAdvancedReporting,
		FeatureLongTermMetrics,
	},
	TierLifetime: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
		FeatureRelay,
		FeatureSSO,
		FeatureAdvancedSSO,
		FeatureRBAC,
		FeatureAuditLogging,
		FeatureAdvancedReporting,
		FeatureLongTermMetrics,
	},
	TierMSP: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
		FeatureRelay,
		FeatureUnlimited,
		FeatureSSO,
		FeatureAdvancedSSO,
		FeatureRBAC,
		FeatureAuditLogging,
		FeatureAdvancedReporting,
		FeatureLongTermMetrics,
		// Note: FeatureMultiUser, FeatureWhiteLabel are not yet included in MSP tier
	},
	TierEnterprise: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
		FeatureRelay,
		FeatureUnlimited,
		FeatureMultiUser,
		FeatureWhiteLabel,
		FeatureMultiTenant,
		FeatureAuditLogging,
		FeatureSSO,
		FeatureAdvancedSSO,
		FeatureRBAC,
		FeatureAdvancedReporting,
		FeatureLongTermMetrics,
	},
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
func DeriveEntitlements(tier Tier, features []string, maxNodes int, maxGuests int) (capabilities []string, limits map[string]int64) {
	capabilities = DeriveCapabilitiesFromTier(tier, features)

	limits = make(map[string]int64)
	if maxNodes > 0 {
		limits["max_nodes"] = int64(maxNodes)
	}
	if maxGuests > 0 {
		limits["max_guests"] = int64(maxGuests)
	}

	return capabilities, limits
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
		return "Free"
	case TierPro:
		return "Pro Intelligence (Monthly)"
	case TierProAnnual:
		return "Pro Intelligence (Annual)"
	case TierLifetime:
		return "Pro Intelligence (Lifetime)"
	case TierMSP:
		return "MSP"
	case TierEnterprise:
		return "Enterprise"
	default:
		return "Unknown"
	}
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
		return "Enterprise Audit Logging"
	case FeatureSSO:
		return "Basic SSO (OIDC)"
	case FeatureAdvancedSSO:
		return "Advanced SSO (SAML/Multi-Provider)"
	case FeatureRelay:
		return "Remote Access (Mobile Relay)"
	case FeatureAdvancedReporting:
		return "Advanced Infrastructure Reporting (PDF/CSV)"
	case FeatureLongTermMetrics:
		return "90-Day Metric History"
	default:
		return feature
	}
}
