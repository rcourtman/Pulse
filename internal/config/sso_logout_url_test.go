package config

import (
	"strings"
	"testing"
)

func TestOIDCLogoutURLValidation(t *testing.T) {
	for _, tt := range []struct {
		name, raw string
		valid     bool
	}{
		{"omitted", "", true},
		{"https", "https://identity.example.test/logout?return=app#done", true},
		{"http compatibility", "http://identity.example.test/logout", true},
		{"loopback", "http://127.0.0.1:8080/logout", true},
		{"ipv6", "https://[::1]:8443/logout", true},
		{"uppercase scheme", "HTTPS://identity.example.test/logout", true},
		{"script", "javascript:void(0)", false},
		{"mixed case script", "JaVaScRiPt:void(0)", false},
		{"data", "data:text/plain,inert", false},
		{"relative", "/logout", false},
		{"protocol relative", "//identity.example.test/logout", false},
		{"userinfo", "https://fixture:synthetic-url-secret@identity.example.test/logout", false},
		{"opaque", "https:logout", false},
		{"empty host", "https:///logout", false},
		{"leading whitespace", " https://identity.example.test/logout", false},
		{"trailing whitespace", "https://identity.example.test/logout ", false},
		{"blank", " \t", false},
		{"control", "https://identity.example.test/\nlogout", false},
		{"damaged escape", "https://identity.example.test/%zz", false},
		{"overlong", "https://identity.example.test/" + strings.Repeat("x", 2048), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOIDCLogoutURL(tt.raw)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%t, want %t", err == nil, tt.valid)
			}
			if err != nil && err.Error() != "invalid oidc logout url" {
				t.Fatal("validation reflected rejected input")
			}
			for _, enabled := range []bool{false, true} {
				cfg := &SSOConfig{Providers: []SSOProvider{{ID: "fixture", Name: "Fixture", Type: SSOProviderTypeOIDC, Enabled: enabled, OIDC: &OIDCProviderConfig{IssuerURL: "https://identity.example.test", ClientID: "fixture", LogoutURL: tt.raw}}}}
				if (cfg.Validate() == nil) != tt.valid {
					t.Errorf("config enabled=%t lost logout validation", enabled)
				}
			}
		})
	}
}
