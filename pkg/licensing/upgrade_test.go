package licensing

import (
	"strings"
	"testing"
)

func TestResolveProTrialSignupURL_DefaultWhenUnset(t *testing.T) {
	if got := ResolveProTrialSignupURL(""); got != DefaultProTrialSignupURL {
		t.Fatalf("ResolveProTrialSignupURL(\"\") = %q, want %q", got, DefaultProTrialSignupURL)
	}
}

func TestResolveProTrialSignupURL_DefaultIsHostedCommercialBase(t *testing.T) {
	if strings.Contains(DefaultProTrialSignupURL, "start-pro-trial") {
		t.Fatalf("DefaultProTrialSignupURL=%q must not point at retired trial signup route", DefaultProTrialSignupURL)
	}
}

func TestResolveProTrialSignupURL_UsesValidOverride(t *testing.T) {
	const override = "https://example.com/commercial-base?src=test"

	if got := ResolveProTrialSignupURL(override); got != override {
		t.Fatalf("ResolveProTrialSignupURL() = %q, want %q", got, override)
	}
}

func TestResolveProTrialSignupURL_AllowsLoopbackHTTPOverride(t *testing.T) {
	const override = "http://127.0.0.1:8080/commercial-base?src=test"

	if got := ResolveProTrialSignupURL(override); got != override {
		t.Fatalf("ResolveProTrialSignupURL() = %q, want %q", got, override)
	}
}

func TestResolveProTrialSignupURL_RejectsInvalidOverrides(t *testing.T) {
	testCases := []string{
		"relative/path",
		"javascript:alert(1)",
		"https://",
		"http://192.168.1.20/commercial-base",
		"ftp://example.com/commercial-base",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			if got := ResolveProTrialSignupURL(tc); got != DefaultProTrialSignupURL {
				t.Fatalf("ResolveProTrialSignupURL() = %q, want default %q", got, DefaultProTrialSignupURL)
			}
		})
	}
}

func TestResolvePulseAccountPortalURL_DefaultWhenUnset(t *testing.T) {
	if got := ResolvePulseAccountPortalURL(""); got != DefaultPulseAccountPortalURL {
		t.Fatalf("ResolvePulseAccountPortalURL(\"\") = %q, want %q", got, DefaultPulseAccountPortalURL)
	}
}

func TestResolvePulseAccountPortalURL_UsesValidOverride(t *testing.T) {
	const override = "https://example.com/portal?src=test"

	if got := ResolvePulseAccountPortalURL(override); got != override {
		t.Fatalf("ResolvePulseAccountPortalURL() = %q, want %q", got, override)
	}
}

func TestResolvePulseAccountPortalURL_AllowsLoopbackHTTPOverride(t *testing.T) {
	const override = "http://localhost:8080/portal?src=test"

	if got := ResolvePulseAccountPortalURL(override); got != override {
		t.Fatalf("ResolvePulseAccountPortalURL() = %q, want %q", got, override)
	}
}

func TestResolvePulseAccountPortalURL_RejectsInvalidOverrides(t *testing.T) {
	testCases := []string{
		"relative/path",
		"javascript:alert(1)",
		"https://",
		"http://10.0.0.25/portal",
		"ftp://example.com/portal",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			if got := ResolvePulseAccountPortalURL(tc); got != DefaultPulseAccountPortalURL {
				t.Fatalf("ResolvePulseAccountPortalURL() = %q, want default %q", got, DefaultPulseAccountPortalURL)
			}
		})
	}
}

func TestOperatorOutcomeUpgradeReasonsRemainCanonical(t *testing.T) {
	expected := map[string]string{
		FeatureRelay:           "Get Relay so Pulse stays reachable securely from anywhere instead of only on the local dashboard.",
		FeatureLongTermMetrics: "Get Relay for 14 days of history, or Pro for 90 days, so you can see what changed before and after an incident.",
		FeatureAIAlerts:        "Upgrade to Pro so alerts arrive with root-cause analysis instead of a stack of symptoms.",
		FeatureAIAutoFix:       "Upgrade to Pro so Pulse can move from finding issues to governed remediation with your approval or autonomy policy.",
	}

	for _, entry := range UpgradeReasonMatrix {
		want, ok := expected[entry.Feature]
		if !ok {
			continue
		}
		if entry.Reason != want {
			t.Fatalf("reason for %q = %q, want %q", entry.Feature, entry.Reason, want)
		}
	}
}

func TestCompatibilityOnlyFeaturesStayOutOfGenericUpgradeReasons(t *testing.T) {
	reasons := GenerateUpgradeReasons(nil)
	for _, entry := range reasons {
		if entry.Feature == FeatureKubernetesAI {
			t.Fatalf("compatibility-only feature %q should not produce a generic upgrade reason", entry.Feature)
		}
	}
}

func TestCompatibilityOnlyFeatureUpgradeURLsFallBackToGenericPricing(t *testing.T) {
	foundCompatibilityOnlyFeature := false

	for _, entry := range AllFeatureMetadata() {
		if !IsCompatibilityOnlyFeature(entry.Key) {
			continue
		}
		foundCompatibilityOnlyFeature = true
		if got := UpgradeURLForFeature(entry.Key); got != DefaultUpgradeURL {
			t.Fatalf("UpgradeURLForFeature(%q) = %q, want generic pricing URL %q", entry.Key, got, DefaultUpgradeURL)
		}
	}

	if !foundCompatibilityOnlyFeature {
		t.Fatal("expected at least one compatibility-only feature in metadata")
	}
}

func TestUpgradeReasonMatrixDerivesFromCanonicalFeatureMetadata(t *testing.T) {
	expectedOrder := []string{
		FeatureMobileApp,
		FeaturePushNotifications,
		FeatureRelay,
		FeatureLongTermMetrics,
		FeatureAIAutoFix,
		FeatureAIAlerts,
		FeatureRBAC,
		FeatureAgentProfiles,
		FeatureAdvancedSSO,
		FeatureAuditLogging,
		FeatureAdvancedReporting,
	}

	if len(UpgradeReasonMatrix) != len(expectedOrder) {
		t.Fatalf("UpgradeReasonMatrix length = %d, want %d", len(UpgradeReasonMatrix), len(expectedOrder))
	}

	for idx, feature := range expectedOrder {
		if UpgradeReasonMatrix[idx].Feature != feature {
			t.Fatalf("UpgradeReasonMatrix[%d].Feature = %q, want %q", idx, UpgradeReasonMatrix[idx].Feature, feature)
		}
	}
}
