package agenttarget

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// These are declared synthetic markers, not captured credentials or customer URLs.
func TestNormalizePulseURLDoesNotDiscloseRejectedInput(t *testing.T) {
	for _, mode := range []struct {
		name          string
		authoritative bool
		plaintext     bool
	}{{"primary", true, false}, {"observer", false, false}, {"observer-explicit-http", false, true}} {
		t.Run(mode.name, func(t *testing.T) {
			for _, test := range []struct{ name, raw string }{
				{"userinfo", "https://synthetic-user:synthetic-password@pulse.invalid"},
				{"query", "https://pulse.invalid/?token=synthetic-query-secret"},
				{"fragment", "https://pulse.invalid/#synthetic-fragment-secret"},
				{"port", "https://pulse.invalid:synthetic-port-secret"},
				{"escape", "https://pulse.invalid/%synthetic-escape-secret"},
				{"missing-host", "https:///synthetic-path-secret"},
				{"scheme", "synthetic-scheme-secret://pulse.invalid/"},
				{"controls", "https://pulse.invalid/\nsynthetic-control-secret"},
			} {
				t.Run(test.name, func(t *testing.T) {
					got, err := NormalizePulseURL(test.raw, mode.authoritative, mode.plaintext)
					if err == nil || got != "" {
						t.Fatal("invalid destination was admitted")
					}
					if strings.Contains(err.Error(), "synthetic-") || strings.Contains(err.Error(), "pulse.invalid") {
						t.Fatal("validation diagnostic disclosed the rejected URL")
					}
					if !strings.Contains(strings.ToLower(err.Error()), "url") {
						t.Fatal("validation diagnostic lost its actionable URL context")
					}
				})
			}
		})
	}
	// This observer-only policy check runs before the shared normalizer, so
	// it must not reflect even a malformed/userinfo/query-bearing HTTP URL.
	for _, raw := range []string{
		"http://203.0.113.10/synthetic-path-secret",
		"http://synthetic-user:synthetic-password@203.0.113.10/?token=synthetic-query-secret",
	} {
		if _, err := NormalizePulseURL(raw, false, false); err == nil || strings.Contains(err.Error(), "synthetic-") || strings.Contains(err.Error(), "203.0.113.10") || !strings.Contains(strings.ToLower(err.Error()), "https") {
			t.Fatal("observer plaintext refusal disclosed its input or lost HTTPS guidance")
		}
	}
}

func TestLoadObserversDoesNotDiscloseRejectedURL(t *testing.T) {
	for _, primary := range []bool{false, true} {
		t.Run(map[bool]string{false: "observer", true: "primary"}[primary], func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "observers.json")
			tokenPath := filepath.Join(dir, "observer.token")
			writePrivateFile(t, tokenPath, "synthetic-file-token")
			primaryURL, observerURL := "https://primary.invalid", "https://observer.invalid"
			bad := "https://synthetic-user:synthetic-password@private-fixture.invalid/?token=synthetic-query-secret"
			if primary {
				primaryURL = bad
			} else {
				observerURL = bad
			}
			data, err := json.Marshal(map[string]any{"version": 1, "observers": []map[string]any{{"name": "report-only", "url": observerURL, "tokenFile": tokenPath}}})
			if err != nil {
				t.Fatal(err)
			}
			writePrivateFile(t, configPath, string(data))
			observers, err := LoadObservers(configPath, primaryURL)
			if err == nil || len(observers) != 0 {
				t.Fatal("invalid configuration produced an observer")
			}
			for _, marker := range []string{"synthetic-", "private-fixture.invalid", tokenPath} {
				if strings.Contains(err.Error(), marker) {
					t.Fatal("configuration error disclosed a rejected URL or credential")
				}
			}
			if primary && !strings.Contains(err.Error(), "primary Pulse URL") || !primary && !strings.Contains(err.Error(), "observer 1") {
				t.Fatal("configuration error lost destination selection guidance")
			}
		})
	}
}

func TestNormalizePulseURLPreservesDestinationPolicy(t *testing.T) {
	for _, test := range []struct {
		name, raw, want string
		authoritative   bool
		plaintext       bool
		wantError       string
	}{
		{"primary-https", " HTTPS://Pulse.Invalid:7655/base/ ", "https://pulse.invalid:7655/base", true, false, ""},
		{"observer-https", " HTTPS://Pulse.Invalid:7655/base/ ", "https://pulse.invalid:7655/base", false, false, ""},
		{"observer-loopback", "http://127.0.0.1:7655/", "http://127.0.0.1:7655", false, false, ""},
		{"primary-private-http", "http://192.168.50.10:7655/", "http://192.168.50.10:7655", true, false, ""},
		{"observer-private-http", "http://192.168.50.10:7655/", "", false, false, "explicitly allows plaintext HTTP"},
		{"observer-explicit-http", "http://203.0.113.10:7655/", "http://203.0.113.10:7655", false, true, ""},
		{"primary-public-http", "http://203.0.113.10:7655/", "", true, false, "must use https"},
		{"primary-explicit-http", "http://203.0.113.10:7655/", "http://203.0.113.10:7655", true, true, ""},
		{"empty", " ", "", true, false, "is required"},
		{"scheme-missing", "pulse.invalid/base", "", true, false, "must include scheme"},
		{"host-missing", "https:///base", "", true, false, "must include host"},
		{"port-low", "https://pulse.invalid:0", "", true, false, "invalid port"},
		{"port-high", "https://pulse.invalid:65536", "", true, false, "invalid port"},
		{"scheme-unsupported", "ftp://pulse.invalid", "", true, false, "unsupported scheme"},
		{"userinfo", "https://user:password@pulse.invalid", "", true, false, "must not include user credentials"},
		{"query", "https://pulse.invalid/?token=value", "", true, false, "must not include query or fragment"},
		{"fragment", "https://pulse.invalid/#value", "", true, false, "must not include query or fragment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizePulseURL(test.raw, test.authoritative, test.plaintext)
			if test.wantError == "" {
				if err != nil || got != test.want {
					t.Fatalf("normalization = %q, %v; want %q", got, err, test.want)
				}
			} else if err == nil || got != "" || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("refusal = %q, %v; want %q", got, err, test.wantError)
			}
		})
	}
}
