package licensing

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

// DefaultUpgradeURL is used when no feature-specific URL mapping exists.
const DefaultUpgradeURL = "https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade"

// DefaultProTrialSignupURL is the legacy hosted commercial base URL used by
// compatibility paths that derive hosted entitlement refresh endpoints.
const DefaultProTrialSignupURL = "https://cloud.pulserelay.pro"

// DefaultPulseAccountPortalURL is the authenticated Pulse Account portal entrypoint.
const DefaultPulseAccountPortalURL = "https://cloud.pulserelay.pro/portal"

// ProTrialSignupURLEnvVar overrides the legacy hosted commercial base URL.
const ProTrialSignupURLEnvVar = "PULSE_PRO_TRIAL_SIGNUP_URL"

// PulseAccountPortalURLEnvVar overrides the Pulse Account portal URL.
const PulseAccountPortalURLEnvVar = "PULSE_ACCOUNT_PORTAL_URL"

// ResolveProTrialSignupURL returns the canonical legacy hosted commercial base URL.
// Invalid overrides are ignored and the default URL is returned.
func ResolveProTrialSignupURL(override string) string {
	if normalized, ok := validateExternalUpgradeURLOverride(override); ok {
		return normalized
	}
	return DefaultProTrialSignupURL
}

// ResolvePulseAccountPortalURL returns the canonical Pulse Account portal URL.
// Invalid overrides are ignored and the default URL is returned.
func ResolvePulseAccountPortalURL(override string) string {
	if normalized, ok := validateExternalUpgradeURLOverride(override); ok {
		return normalized
	}
	return DefaultPulseAccountPortalURL
}

// ProTrialSignupURL returns the default legacy hosted commercial base URL.
func ProTrialSignupURL() string {
	return DefaultProTrialSignupURL
}

func validateExternalUpgradeURLOverride(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	parsed, err := securityutil.NormalizeAbsoluteHTTPURL(raw)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		parsed.Scheme = "https"
	case "http":
		if !securityutil.IsLoopbackHost(parsed.Hostname()) {
			return "", false
		}
		parsed.Scheme = "http"
	default:
		return "", false
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String(), true
}

// UpgradeURLForFeature returns the canonical upgrade URL for a capability key.
func UpgradeURLForFeature(feature string) string {
	switch feature {
	case FeatureRelay:
		return DefaultUpgradeURL + "&feature=relay"
	case FeatureMobileApp:
		return DefaultUpgradeURL + "&feature=mobile_app"
	case FeaturePushNotifications:
		return DefaultUpgradeURL + "&feature=push_notifications"
	case FeatureLongTermMetrics:
		return DefaultUpgradeURL + "&feature=long_term_metrics"
	case FeatureAIAutoFix:
		return DefaultUpgradeURL + "&feature=ai_autofix"
	case FeatureAIAlerts:
		return DefaultUpgradeURL + "&feature=ai_alerts"
	case FeatureKubernetesAI:
		return DefaultUpgradeURL + "&feature=kubernetes_ai"
	case FeatureRBAC:
		return DefaultUpgradeURL + "&feature=rbac"
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
