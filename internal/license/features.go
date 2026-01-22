// Package license handles Pulse Pro license validation and feature gating.
package license

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

	// MSP/Enterprise tier features (for volume deals)
	FeatureMultiUser   = "multi_user"   // Multi-user (likely merged with RBAC)
	FeatureWhiteLabel  = "white_label"  // Custom branding - NOT IMPLEMENTED YET
	FeatureMultiTenant = "multi_tenant" // Multi-tenant - NOT IMPLEMENTED YET
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
	},
	TierPro: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
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
		FeatureUnlimited,
		FeatureSSO,
		FeatureAdvancedSSO,
		FeatureRBAC,
		FeatureAuditLogging,
		FeatureAdvancedReporting,
		FeatureLongTermMetrics,
		// Note: FeatureMultiUser, FeatureWhiteLabel, FeatureMultiTenant
		// are on the roadmap but NOT included until implemented
	},
	TierEnterprise: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureAgentProfiles,
		FeatureUpdateAlerts,
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
		return "AI Patrol (Background Health Checks)"
	case FeatureAIAlerts:
		return "AI Alert Analysis"
	case FeatureAIAutoFix:
		return "AI Auto-Fix"
	case FeatureKubernetesAI:
		return "Kubernetes AI Analysis"
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
	case FeatureAdvancedReporting:
		return "Advanced Infrastructure Reporting (PDF/CSV)"
	case FeatureLongTermMetrics:
		return "90-Day Metric History"
	default:
		return feature
	}
}
