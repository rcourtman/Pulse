package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestOIDCEmailUnverified(t *testing.T) {
	for _, tc := range []struct {
		name       string
		claims     map[string]any
		emailClaim string
		want       bool
	}{
		{"verified", map[string]any{"email_verified": true}, "email", false},
		{"unverified", map[string]any{"email_verified": false}, "email", true},
		{"verified as text", map[string]any{"email_verified": " True "}, "email", false},
		{"unverified as text", map[string]any{"email_verified": "false"}, "email", true},
		{"not a boolean", map[string]any{"email_verified": float64(1)}, "email", true},
		{"omitted", map[string]any{}, "email", false},
		{"null", map[string]any{"email_verified": nil}, "email", false},
		{"address mapped from another claim", map[string]any{"email_verified": false}, "upn", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := oidcEmailUnverified(tc.claims, tc.emailClaim); got != tc.want {
				t.Fatalf("oidcEmailUnverified = %v, want %v", got, tc.want)
			}
		})
	}
}

// runOIDCEmailVerifiedCallback signs in once through the real login and
// callback handlers against a guest-local provider that issues extra as the
// user's claims, and returns where the callback sent the browser. All
// identities and keys are synthetic.
func runOIDCEmailVerifiedCallback(t *testing.T, key *rsa.PrivateKey, manager internalauth.Manager, configure func(*config.SSOProvider), extra map[string]any) string {
	t.Helper()
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	InitPersistentAuthStores(t.TempDir())
	previousManager := internalauth.GetManager()
	internalauth.SetManager(manager)
	t.Cleanup(func() { internalauth.SetManager(previousManager) })

	jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "synthetic-email", Use: "sig", Algorithm: "RS256"}}})
	if err != nil {
		t.Fatal(err)
	}
	var authorization url.Values
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_, _ = w.Write(jwks)
		case "/token":
			claims := map[string]any{"iss": base, "aud": authorization.Get("client_id"), "sub": "synthetic-subject", "nonce": authorization.Get("nonce"), "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			for name, value := range extra {
				claims[name] = value
			}
			signed := signOIDCJOSEFixture(t, key, "RS256", "synthetic-email", claims, false)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access", "id_token": signed, "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(idp.Close)

	r, _ := setupTestRouter(t)
	r.config.PublicURL = "https://pulse.example.test"
	p := newRecoveryTestProvider("email-verified", true, "")
	p.OIDC.IssuerURL = idp.URL
	configure(&p)
	if err := r.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := r.saveSSOConfig(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.oidcManager.RemoveService(p.ID) })

	login := httptest.NewRecorder()
	r.handleSSOOIDCLogin(login, httptest.NewRequest("GET", "https://pulse.example.test/api/oidc/"+p.ID+"/login?returnTo=/alerts", nil))
	authURL, err := url.Parse(login.Header().Get("Location"))
	if err != nil || authURL.Query().Get("state") == "" {
		t.Fatalf("native login failed: %d %v", login.Code, err)
	}
	authorization = authURL.Query()
	target := "https://pulse.example.test/api/oidc/" + p.ID + "/callback?code=synthetic-code&state=" + url.QueryEscape(authorization.Get("state"))
	return doOIDCCallback(t, r, target).Header().Get("Location")
}

// An address the provider itself reports as unverified is text the account
// holder typed. It must not satisfy a restriction on who may sign in.
func TestSSOOIDCCallbackEmailRestrictionsRequireVerifiedEmail(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const success, refused = "/alerts?oidc=success", "/alerts?oidc=error&oidc_error=email_unverified"
	domains := func(p *config.SSOProvider) { p.AllowedDomains = []string{"corp.example.test"} }
	emails := func(p *config.SSOProvider) { p.AllowedEmails = []string{"mallory@corp.example.test"} }
	for _, tc := range []struct {
		name      string
		configure func(*config.SSOProvider)
		claims    map[string]any
		want      string
	}{
		{"domain list, verified", domains, map[string]any{"email": "alice@corp.example.test", "email_verified": true}, success},
		{"domain list, provider omits email_verified", domains, map[string]any{"email": "alice@corp.example.test"}, success},
		{"domain list, unverified", domains, map[string]any{"email": "mallory@corp.example.test", "email_verified": false}, refused},
		{"domain list, unverified as text", domains, map[string]any{"email": "mallory@corp.example.test", "email_verified": "false"}, refused},
		{"email list, unverified", emails, map[string]any{"email": "mallory@corp.example.test", "email_verified": false}, refused},
		{"email list, verified", emails, map[string]any{"email": "mallory@corp.example.test", "email_verified": true}, success},
		{"domain list, verified elsewhere", domains, map[string]any{"email": "outsider@elsewhere.example.test", "email_verified": true}, "/alerts?oidc=error&oidc_error=domain_restricted"},
		{"domain list, address mapped from another claim", func(p *config.SSOProvider) { domains(p); p.OIDC.EmailClaim = "upn" }, map[string]any{"upn": "alice@corp.example.test", "email_verified": false}, success},
		{"no email restriction, unverified", func(*config.SSOProvider) {}, map[string]any{"email": "mallory@corp.example.test", "email_verified": false}, success},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := internalauth.NewFileManager(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if got := runOIDCEmailVerifiedCallback(t, key, manager, tc.configure, tc.claims); got != tc.want {
				t.Fatalf("callback redirected to %q, want %q", got, tc.want)
			}
		})
	}
}

// Roles saved under an address before stable principals existed move to the
// subject that signs in with that address. An unverified address must not
// claim them, including when another claim repeats it or it stands in for a
// missing username.
func TestSSOOIDCCallbackUnverifiedEmailDoesNotClaimLegacyRoles(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const legacy = "admin@corp.example.test"
	for _, tc := range []struct {
		name     string
		claims   map[string]any
		migrates bool
	}{
		{"verified", map[string]any{"email": legacy, "email_verified": true}, true},
		{"provider omits email_verified", map[string]any{"email": legacy}, true},
		{"unverified", map[string]any{"email": legacy, "email_verified": false}, false},
		{"unverified, address as username", map[string]any{"email": legacy, "preferred_username": "Admin@Corp.Example.Test", "email_verified": false}, false},
		{"unverified, address as display name", map[string]any{"email": legacy, "preferred_username": "mallory", "name": legacy, "email_verified": false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := internalauth.NewFileManager(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.UpdateUserRoles(legacy, []string{internalauth.RoleAdmin}); err != nil {
				t.Fatal(err)
			}
			if got := runOIDCEmailVerifiedCallback(t, key, manager, func(*config.SSOProvider) {}, tc.claims); got != "/alerts?oidc=success" {
				t.Fatalf("callback redirected to %q", got)
			}
			principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "email-verified", "synthetic-subject")
			if err != nil {
				t.Fatal(err)
			}
			assignment, _ := manager.GetUserAssignment(principal)
			if migrated := len(assignment.RoleIDs) > 0; migrated != tc.migrates {
				t.Fatalf("principal roles = %v, want migration %v", assignment.RoleIDs, tc.migrates)
			}
		})
	}
}
