package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type oidcMissingCodeResult struct {
	Scenario                 string `json:"scenario"`
	CallbackTarget           string `json:"callbackTarget"`
	CallbackStatus           int    `json:"callbackStatus"`
	CallbackCookies          int    `json:"callbackCookies"`
	CallbackRemoteCalls      int32  `json:"callbackRemoteCalls"`
	CallbackLocalWrites      int    `json:"callbackLocalWrites"`
	ExistingSessionPreserved bool   `json:"existingSessionPreserved"`
	OwnerPreserved           bool   `json:"ownerPreserved"`
	PendingPreserved         bool   `json:"pendingPreserved"`
	OtherPendingPreserved    bool   `json:"otherPendingPreserved"`
	FollowTarget             string `json:"followTarget"`
	SessionVerified          bool   `json:"sessionVerified"`
	ReplayTarget             string `json:"replayTarget"`
	TokenCalls               int32  `json:"tokenCalls"`
}

// Exercise the registered routes with real discovery, PKCE, signed tokens and
// session admission. An incomplete callback must not spend either this login's
// binding or another browser's. All identities, keys and HTTP are synthetic.
func runOIDCMissingCode(t *testing.T, scenario string) oidcMissingCodeResult {
	t.Helper()
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	InitPersistentAuthStores(t.TempDir())
	previousManager := internalauth.GetManager()
	manager, err := internalauth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	recorder := &oidcCallbackPolicyRecorder{Manager: manager}
	internalauth.SetManager(recorder)
	t.Cleanup(func() { internalauth.SetManager(previousManager) })
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "missing-code-fixture", Use: "sig", Algorithm: "RS256"}}})
	if err != nil {
		t.Fatal(err)
	}
	var calls, exchanges atomic.Int32
	var unavailable atomic.Bool
	var authorization url.Values
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if unavailable.Load() {
			http.Error(w, "synthetic IdP outage", http.StatusServiceUnavailable)
			return
		}
		base := "http://" + req.Host
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_, _ = w.Write(jwks)
		case "/token":
			exchanges.Add(1)
			if err := req.ParseForm(); err != nil {
				t.Error(err)
			}
			if req.Form.Get("code") != "synthetic-code" || req.Form.Get("redirect_uri") != authorization.Get("redirect_uri") || oidcInitializationChallenge(req.Form.Get("code_verifier")) != authorization.Get("code_challenge") || authorization.Get("code_challenge_method") != "S256" {
				http.Error(w, "synthetic code/PKCE/callback mismatch", http.StatusBadRequest)
				return
			}
			claims := map[string]any{"iss": base, "aud": authorization.Get("client_id"), "sub": "synthetic-subject", "nonce": authorization.Get("nonce"), "preferred_username": "Synthetic operator", "email": "operator@example.test", "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			signed := signOIDCJOSEFixture(t, key, "RS256", "missing-code-fixture", claims, false)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access", "id_token": signed, "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(idp.Close)
	p := newRecoveryTestProvider("callback-code", true, "")
	p.Name = "Synthetic OIDC"
	p.OIDC.IssuerURL = idp.URL
	callbackPath := "/api/oidc/" + p.ID + "/callback"
	if scenario == "legacy" {
		callbackPath = "/api/oidc/callback"
		p.OIDC.RedirectURL = "https://pulse.example.test" + callbackPath
	}
	r := newRecoveryTestRouter(t, []config.SSOProvider{p}, nil)
	r.mux = http.NewServeMux()
	r.registerAuthSecurityInstallRoutes()
	t.Cleanup(func() { r.oidcManager.RemoveService(p.ID) })
	login := httptest.NewRecorder()
	r.mux.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/oidc/"+p.ID+"/login?returnTo=/alerts", nil))
	authURL, err := url.Parse(login.Header().Get("Location"))
	if err != nil || authURL.Query().Get("state") == "" {
		t.Fatalf("native login failed: %d %v", login.Code, err)
	}
	authorization = authURL.Query()
	owner := r.oidcManager.GetService(p.ID)
	other, _, err := owner.newStateEntry(p.ID, "/other-browser")
	if err != nil {
		t.Fatal(err)
	}
	const existing = "synthetic-existing-session"
	GetSessionStore().CreateSession(existing, time.Hour, "fixture", "192.0.2.87", "existing-local-user")
	old := GetSessionStore().GetSession(existing)
	callbackHost := "pulse.example.test"
	switch scenario {
	case "alternate", "outage":
		callbackHost = "alternate-pulse.example.test"
	case "unread-trust":
		r.ssoConfig.Providers[0].OIDC.CABundle = filepath.Join(t.TempDir(), "missing-ca.pem")
	}
	unavailable.Store(scenario == "outage")
	request := func(host string, query url.Values) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "https://"+host+callbackPath+"?"+query.Encode(), nil)
		req.RemoteAddr = "192.0.2.87:4567"
		req.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: existing})
		rec := httptest.NewRecorder()
		r.mux.ServeHTTP(rec, req)
		return rec
	}
	before := calls.Load()
	query := url.Values{"state": {authorization.Get("state")}}
	if scenario == "empty" {
		query.Set("code", "")
	}
	missing := request(callbackHost, query)
	result := oidcMissingCodeResult{Scenario: scenario, CallbackTarget: missing.Header().Get("Location"), CallbackStatus: missing.Code, CallbackCookies: len(missing.Result().Cookies()), CallbackRemoteCalls: calls.Load() - before, CallbackLocalWrites: recorder.roleWrites + recorder.identityWrites, ExistingSessionPreserved: reflect.DeepEqual(old, GetSessionStore().GetSession(existing)), OwnerPreserved: r.oidcManager.GetService(p.ID) == owner}
	_, result.PendingPreserved = owner.peekState(authorization.Get("state"))
	_, result.OtherPendingPreserved = r.oidcManager.GetService(p.ID).peekState(other)
	// Restore the fixture's outage/trust so the original correctly addressed,
	// signed completion can test whether incomplete input erased its binding.
	unavailable.Store(false)
	r.ssoConfig.Providers[0].OIDC.CABundle = ""
	if scenario == "withdrawn" {
		if apply, save := r.commitSSOConfig(func(cfg *config.SSOConfig) error {
			provider := *cfg.GetProvider(p.ID)
			provider.Enabled = false
			return cfg.UpdateProvider(provider)
		}); apply != nil || save != nil {
			t.Fatalf("withdraw provider: %v %v", apply, save)
		}
	}
	query.Set("code", "synthetic-code")
	complete := request("pulse.example.test", query)
	result.FollowTarget = complete.Header().Get("Location")
	for _, cookie := range complete.Result().Cookies() {
		if cookie.Name != sessionCookieName(true) {
			continue
		}
		principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, p.ID, "synthetic-subject")
		if err != nil {
			t.Fatal(err)
		}
		session := GetSessionStore().GetSession(cookie.Value)
		result.SessionVerified = cookie.HttpOnly && cookie.Secure && session != nil && session.Username == principal && session.DisplayUsername == "Synthetic operator" && session.OIDCIssuer == idp.URL && GetSessionStore().ValidateSession(cookie.Value)
	}
	result.TokenCalls = exchanges.Load()
	result.ReplayTarget = request("pulse.example.test", query).Header().Get("Location")
	if exchanges.Load() != result.TokenCalls {
		t.Error("replay reached token exchange")
	}
	return result
}

func TestSSOOIDCCallbackMissingCodePreservesNativeLogin(t *testing.T) {
	for _, scenario := range []string{"same-origin", "empty", "alternate", "outage", "unread-trust", "legacy", "withdrawn"} {
		t.Run(scenario, func(t *testing.T) {
			result := runOIDCMissingCode(t, scenario)
			if result.CallbackStatus != http.StatusFound || result.CallbackTarget != "/alerts?oidc=error&oidc_error=missing_code" || result.CallbackCookies != 0 || result.CallbackRemoteCalls != 0 || result.CallbackLocalWrites != 0 || !result.ExistingSessionPreserved || !result.OwnerPreserved || !result.PendingPreserved || !result.OtherPendingPreserved {
				t.Fatalf("incomplete callback changed sign-in state or reached provider work: %+v", result)
			}
			if scenario == "withdrawn" {
				if result.FollowTarget != "/?oidc=error&oidc_error=provider_not_found" || result.SessionVerified || result.TokenCalls != 0 {
					t.Fatalf("preserved pending state revived withdrawn authority: %+v", result)
				}
				return
			}
			if result.FollowTarget != "/alerts?oidc=success" || !result.SessionVerified || result.TokenCalls != 1 {
				t.Fatalf("valid signed completion failed after incomplete callback: %+v", result)
			}
			if u, err := url.Parse(result.ReplayTarget); err != nil || u.Query().Get("oidc") != "error" {
				t.Fatalf("signed completion was not single-use: %+v", result)
			}
		})
	}
}

func TestSSOOIDCCallbackMissingCodeKeepsAdmissionPrecedence(t *testing.T) {
	for _, state := range []string{"missing", "unknown", "expired", "consumed", "denied", "disabled"} {
		t.Run(state, func(t *testing.T) {
			r, calls, _, pending := newOIDCCallbackRequestFixture(t, "callback-gate", true)
			owner := r.oidcManager.GetService("callback-gate")
			query := url.Values{}
			want := "invalid_state"
			switch state {
			case "missing":
				want = "missing_state"
			case "unknown":
				query.Set("state", "never-issued")
			case "expired":
				query.Set("state", "expired-state")
				owner.stateStore.Put("expired-state", &oidcStateEntry{ProviderID: "callback-gate", ExpiresAt: time.Now().Add(-time.Minute)})
			case "consumed":
				used, _, err := owner.newStateEntry("callback-gate", "/used")
				if err != nil {
					t.Fatal(err)
				}
				owner.consumeState(used)
				query.Set("state", used)
			case "denied":
				query.Set("state", pending)
				query.Set("error", "access_denied")
				want = "access_denied"
			case "disabled":
				query.Set("state", pending)
				r.ssoConfig.Providers[0].Enabled = false
				want = "provider_not_found"
			}
			before := calls.Load()
			rec := oidcCallbackRequest(r, "callback-gate", "alternate-pulse.example.test", query)
			if rec.Header().Get("Location") != "/?oidc=error&oidc_error="+want || rec.Code != http.StatusFound || len(rec.Result().Cookies()) != 0 || calls.Load() != before || r.oidcManager.GetService("callback-gate") != owner {
				t.Fatalf("missing code changed prior refusal: %d %q", rec.Code, rec.Header().Get("Location"))
			}
			if _, ok := owner.peekState(pending); !ok {
				t.Fatal("refused callback discarded another pending login")
			}
		})
	}
}
