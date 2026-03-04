package api

import (
	"net/http/httptest"
	"testing"
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
