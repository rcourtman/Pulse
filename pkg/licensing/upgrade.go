package licensing

import (
	"net/url"
	"strings"
)

// DefaultUpgradeURL is used when no feature-specific URL mapping exists.
const DefaultUpgradeURL = "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade"

// DefaultProTrialSignupURL is the hosted signup/checkout entrypoint for Pulse Pro trials.
const DefaultProTrialSignupURL = "https://pulserelay.pro/start-pro-trial?utm_source=pulse&utm_medium=app&utm_campaign=trial_signup"

// ProTrialSignupURLEnvVar overrides the hosted signup URL for Pulse Pro trials.
const ProTrialSignupURLEnvVar = "PULSE_PRO_TRIAL_SIGNUP_URL"

// ResolveProTrialSignupURL returns the canonical hosted signup URL for Pulse Pro trials.
// Invalid overrides are ignored and the default URL is returned.
func ResolveProTrialSignupURL(override string) string {
	if normalized, ok := validateProTrialSignupURLOverride(override); ok {
		return normalized
	}
	return DefaultProTrialSignupURL
}

// ProTrialSignupURL returns the default hosted signup URL for Pulse Pro trials.
func ProTrialSignupURL() string {
	return DefaultProTrialSignupURL
}

func validateProTrialSignupURLOverride(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String(), true
	default:
		return "", false
	}
}

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
