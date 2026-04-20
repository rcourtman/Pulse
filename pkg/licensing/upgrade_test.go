package licensing

import "testing"

func TestResolveProTrialSignupURL_DefaultWhenUnset(t *testing.T) {
	if got := ResolveProTrialSignupURL(""); got != DefaultProTrialSignupURL {
		t.Fatalf("ResolveProTrialSignupURL(\"\") = %q, want %q", got, DefaultProTrialSignupURL)
	}
}

func TestResolveProTrialSignupURL_UsesValidOverride(t *testing.T) {
	const override = "https://example.com/start-pro-trial?src=test"

	if got := ResolveProTrialSignupURL(override); got != override {
		t.Fatalf("ResolveProTrialSignupURL() = %q, want %q", got, override)
	}
}

func TestResolveProTrialSignupURL_RejectsInvalidOverrides(t *testing.T) {
	testCases := []string{
		"relative/path",
		"javascript:alert(1)",
		"https://",
		"ftp://example.com/start-pro-trial",
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

func TestResolvePulseAccountPortalURL_RejectsInvalidOverrides(t *testing.T) {
	testCases := []string{
		"relative/path",
		"javascript:alert(1)",
		"https://",
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
		FeatureAIAutoFix:       "Upgrade to Pro so Pulse can move from finding issues to applying safe remediation with your approval or in autonomous mode.",
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
