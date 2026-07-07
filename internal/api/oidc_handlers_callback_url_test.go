package api

import (
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestBuildSSOOIDCCallbackURL(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "http://pulse.example.com/api/oidc/legacy-oidc/login", nil)

	t.Run("builds provider scoped callback when not configured", func(t *testing.T) {
		t.Parallel()
		got := buildSSOOIDCCallbackURL(req, "legacy-oidc", "")
		want := "http://pulse.example.com/api/oidc/legacy-oidc/callback"
		if got != want {
			t.Fatalf("buildSSOOIDCCallbackURL() = %q, want %q", got, want)
		}
	})

	t.Run("keeps configured callback unchanged", func(t *testing.T) {
		t.Parallel()
		got := buildSSOOIDCCallbackURL(req, "legacy-oidc", "http://pulse.example.com/api/oidc/callback")
		want := "http://pulse.example.com/api/oidc/callback"
		if got != want {
			t.Fatalf("buildSSOOIDCCallbackURL() = %q, want %q", got, want)
		}
	})
}

func TestExtractOIDCProviderIDSupportsLegacyPaths(t *testing.T) {
	t.Parallel()

	if got := extractOIDCProviderID("/api/oidc/legacy-oidc/login", "login"); got != "legacy-oidc" {
		t.Fatalf("provider scoped login id = %q, want legacy-oidc", got)
	}
	if got := extractOIDCProviderID("/api/oidc/callback", "callback"); got != config.LegacyOIDCProviderID {
		t.Fatalf("legacy callback id = %q, want %s", got, config.LegacyOIDCProviderID)
	}
	if got := extractOIDCProviderID("/api/oidc/login", "login"); got != config.LegacyOIDCProviderID {
		t.Fatalf("legacy login id = %q, want %s", got, config.LegacyOIDCProviderID)
	}
}
