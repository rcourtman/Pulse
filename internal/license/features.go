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

	// Pro tier features - Monitoring
	FeatureUpdateAlerts = "update_alerts" // Alerts for pending container/package updates

	// MSP tier features (FUTURE - not in v1 launch)
	FeatureMultiUser   = "multi_user"   // RBAC - NOT IMPLEMENTED YET
	FeatureWhiteLabel  = "white_label"  // Custom branding - NOT IMPLEMENTED YET
	FeatureMultiTenant = "multi_tenant" // Multi-tenant - NOT IMPLEMENTED YET
	FeatureUnlimited   = "unlimited"    // Unlimited instances (explicit for contracts)
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
		// Free tier has no Pro features, but full monitoring
	},
	TierPro: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureUpdateAlerts,
	},
	TierProAnnual: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureUpdateAlerts,
	},
	TierLifetime: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureUpdateAlerts,
	},
	TierMSP: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureUpdateAlerts,
		FeatureUnlimited,
		// Note: FeatureMultiUser, FeatureWhiteLabel, FeatureMultiTenant
		// are on the roadmap but NOT included until implemented
	},
	TierEnterprise: {
		FeatureAIPatrol,
		FeatureAIAlerts,
		FeatureAIAutoFix,
		FeatureKubernetesAI,
		FeatureUpdateAlerts,
		FeatureUnlimited,
		FeatureMultiUser,
		FeatureWhiteLabel,
		FeatureMultiTenant,
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
	case FeatureMultiUser:
		return "Multi-User / RBAC"
	case FeatureWhiteLabel:
		return "White-Label Branding"
	case FeatureMultiTenant:
		return "Multi-Tenant Mode"
	case FeatureUnlimited:
		return "Unlimited Instances"
	default:
		return feature
	}
}
