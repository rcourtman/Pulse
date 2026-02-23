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
