package licensing

// DefaultUpgradeURL is used when no feature-specific URL mapping exists.
const DefaultUpgradeURL = "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade"

// UpgradeURLForFeature returns the canonical upgrade URL for a capability key.
func UpgradeURLForFeature(feature string) string {
	switch feature {
	case FeatureAIAutoFix:
		return DefaultUpgradeURL + "&feature=ai_autofix"
	case FeatureLongTermMetrics:
		return DefaultUpgradeURL + "&feature=long_term_metrics"
	case FeatureRelay:
		return DefaultUpgradeURL + "&feature=relay"
	case FeatureRBAC:
		return DefaultUpgradeURL + "&feature=rbac"
	case FeatureAIAlerts:
		return DefaultUpgradeURL + "&feature=ai_alerts"
	case FeatureKubernetesAI:
		return DefaultUpgradeURL + "&feature=kubernetes_ai"
	case FeatureAgentProfiles:
		return DefaultUpgradeURL + "&feature=agent_profiles"
	case FeatureAdvancedSSO:
		return DefaultUpgradeURL + "&feature=advanced_sso"
	case FeatureAuditLogging:
		return DefaultUpgradeURL + "&feature=audit_logging"
	case FeatureAdvancedReporting:
		return DefaultUpgradeURL + "&feature=advanced_reporting"
	default:
		return DefaultUpgradeURL
	}
}
