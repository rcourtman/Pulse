package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// An SSO provider decides who can sign in and which roles its groups receive,
// the built-in administrator role included, and the session that follows is
// not limited by token scopes. Creating, updating or deleting one therefore
// needs instance-wide authority from a token caller: settings:write alone
// must not be enough to point a provider at the caller's own issuer and sign
// in as an administrator.
func TestSSOProviderWritesRequireInstanceWideToken(t *testing.T) {
	const (
		fullRaw      = "sso-write-authority-full-token.12345678"
		settingsRaw  = "sso-write-authority-settings-token.12345678"
		writeOnlyRaw = "sso-write-authority-write-only-token.12345678"

		collection = "/api/security/sso/providers"
		saved      = collection + "/saved"

		savedBody    = `{"id":"saved","name":"saved","type":"oidc","enabled":true,"oidc":{"issuerUrl":"https://auth.example.com","clientId":"pulse","clientSecret":"secret"}}`
		attackerBody = `{"id":"attacker","name":"attacker","type":"oidc","enabled":true,"groupsClaim":"groups","groupRoleMappings":{"pwn":"admin"},"oidc":{"issuerUrl":"https://attacker.example.com","clientId":"pulse","clientSecret":"secret"}}`
		takeoverBody = `{"name":"saved","type":"oidc","enabled":true,"groupsClaim":"groups","groupRoleMappings":{"pwn":"admin"},"oidc":{"issuerUrl":"https://attacker.example.com","clientId":"pulse","clientSecret":"secret"}}`
	)
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, fullRaw, []string{config.ScopeWildcard}, nil),
		newTokenRecord(t, settingsRaw, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil),
		newTokenRecord(t, writeOnlyRaw, []string{config.ScopeSettingsWrite}, nil),
	)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")
	cleanupTestRouter(t, router)
	handler := router.Handler()

	serve := func(method, path, body, raw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", raw)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := serve(http.MethodPost, collection, savedBody, fullRaw); rec.Code != http.StatusCreated {
		t.Fatalf("full-access token create = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	for _, caller := range []struct{ name, raw string }{
		{"settings read and write", settingsRaw},
		{"settings:write only", writeOnlyRaw},
	} {
		t.Run(caller.name, func(t *testing.T) {
			for _, write := range []struct{ method, path, body string }{
				{http.MethodPost, collection, attackerBody},
				{http.MethodPut, saved, takeoverBody},
				{http.MethodDelete, saved, ""},
			} {
				if rec := serve(write.method, write.path, write.body, caller.raw); rec.Code != http.StatusForbidden {
					t.Fatalf("%s %s = %d, want 403: %s", write.method, write.path, rec.Code, rec.Body.String())
				}
			}

			ssoCfg := router.currentSSOConfig()
			if len(ssoCfg.Providers) != 1 {
				t.Fatalf("refused writes changed the provider list: %+v", ssoCfg.Providers)
			}
			provider := ssoCfg.GetProvider("saved")
			if provider == nil || provider.OIDC == nil {
				t.Fatalf("refused writes removed the saved provider: %+v", ssoCfg.Providers)
			}
			if provider.OIDC.IssuerURL != "https://auth.example.com" || len(provider.GroupRoleMappings) != 0 {
				t.Fatalf("refused writes changed the saved provider: issuer %q, mappings %v", provider.OIDC.IssuerURL, provider.GroupRoleMappings)
			}
		})
	}

	// Reading providers stays a settings:read operation.
	if rec := serve(http.MethodGet, collection, "", settingsRaw); rec.Code != http.StatusOK {
		t.Fatalf("settings token list = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if rec := serve(http.MethodPut, saved, takeoverBody, fullRaw); rec.Code != http.StatusOK {
		t.Fatalf("full-access token update = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodDelete, saved, "", fullRaw); rec.Code != http.StatusNoContent {
		t.Fatalf("full-access token delete = %d, want 204: %s", rec.Code, rec.Body.String())
	}
}
