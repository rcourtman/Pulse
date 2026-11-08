package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func awaitOIDCInitialization(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("OIDC initialization did not reach its bounded barrier")
	}
}

// Both native lazy initialization and explicit management discovery can be in
// flight when a provider is removed/reset. Completion is not new authority.
func TestOIDCInitializationManagementOwnership(t *testing.T) {
	for _, native := range []bool{true, false} {
		for _, action := range []string{"remove", "replace", "same-settings-reset", "failed-reset", "remove-reenable", "other-provider"} {
			name := "management/"
			if native {
				name = "native/"
			}
			t.Run(name+action, func(t *testing.T) {
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				var calls atomic.Int32
				idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if calls.Add(1) == 1 {
						close(entered)
						select {
						case <-release:
						case <-req.Context().Done():
							return
						}
					}
					w.Header().Set("Content-Type", "application/json")
					base := "http://" + req.Host
					_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
				}))
				t.Cleanup(idp.Close)
				t.Cleanup(unblock)
				p := newRecoveryTestProvider("ownership", true, "")
				p.OIDC.IssuerURL = idp.URL
				manager := NewOIDCServiceManager()
				t.Cleanup(func() { manager.RemoveService(p.ID); manager.RemoveService("other") })
				const redirect = "https://pulse.example.test/api/oidc/ownership/callback"
				done := make(chan struct{})
				var staleErr error
				go func() {
					defer close(done)
					if native {
						staleErr = manager.EnsureProvider(t.Context(), p.ID, &p, redirect)
					} else {
						staleErr = manager.InitializeProvider(t.Context(), p.ID, &p, redirect)
					}
				}()
				awaitOIDCInitialization(t, entered)
				updated := (&config.SSOConfig{Providers: []config.SSOProvider{p}}).Clone().Providers[0]
				updated.OIDC.ClientSecret = "synthetic-new-secret"
				switch action {
				case "remove":
					manager.RemoveService(p.ID)
				case "replace":
					if err := manager.InitializeProvider(t.Context(), p.ID, &updated, redirect); err != nil {
						t.Fatal(err)
					}
				case "same-settings-reset":
					if err := manager.InitializeProvider(t.Context(), p.ID, &p, redirect); err != nil {
						t.Fatal(err)
					}
				case "failed-reset":
					ctx, cancel := context.WithCancel(t.Context())
					cancel()
					if err := manager.InitializeProvider(ctx, p.ID, &updated, redirect); err == nil {
						t.Fatal("canceled replacement succeeded")
					}
				case "remove-reenable":
					manager.RemoveService(p.ID)
					if err := manager.EnsureProvider(t.Context(), p.ID, &p, redirect); err != nil {
						t.Fatal(err)
					}
				case "other-provider":
					manager.RemoveService("other")
					if err := manager.InitializeProvider(t.Context(), "other", &updated, redirect); err != nil {
						t.Fatal(err)
					}
				}
				current := manager.GetService(p.ID)
				var pending string
				if current != nil {
					var err error
					pending, _, err = current.newStateEntry(p.ID, "/alerts")
					if err != nil {
						t.Fatal(err)
					}
				}
				unblock()
				awaitOIDCInitialization(t, done)
				if action == "other-provider" {
					if staleErr != nil || manager.GetService(p.ID) == nil {
						t.Fatalf("another provider interfered with valid initialization: %v", staleErr)
					}
					return
				}
				if staleErr == nil {
					t.Error("discovery superseded by management reported success")
				}
				if manager.GetService(p.ID) != current {
					t.Fatal("stale discovery republished a withdrawn or replaced service")
				}
				if current != nil {
					if _, ok := current.peekState(pending); !ok {
						t.Error("stale discovery erased current pending login")
					}
					select {
					case <-current.stateStore.stopCleanup:
						t.Error("stale discovery stopped the current service")
					default:
					}
				}
			})
		}
	}
}

type oidcManagementHandoverResult struct {
	ManagementStatus      int    `json:"managementStatus"`
	StaleLoginStatus      int    `json:"staleLoginStatus"`
	StaleLoginError       string `json:"staleLoginError"`
	CurrentLoginStatus    int    `json:"currentLoginStatus"`
	CurrentClientRetained bool   `json:"currentClientRetained"`
	CallbackTarget        string `json:"callbackTarget"`
	SessionVerified       bool   `json:"sessionVerified"`
	ReplayTarget          string `json:"replayTarget"`
	DiscoveryCalls        int32  `json:"discoveryCalls"`
	TokenCalls            int32  `json:"tokenCalls"`
}

// A real saved client change and a second native login complete while the old
// login's discovery is blocked. The old completion must not evict the new
// client's nonce/PKCE binding. Everything here is synthetic and guest-local.
func runOIDCInitializationManagementHandover(t *testing.T) oidcManagementHandoverResult {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "synthetic-handover", Use: "sig", Algorithm: "RS256"}}})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var discoveries, exchanges atomic.Int32
	var authMu sync.Mutex
	var authorization url.Values
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		base := "http://" + req.Host
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			if discoveries.Add(1) == 1 {
				close(entered)
				select {
				case <-release:
				case <-req.Context().Done():
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_, _ = w.Write(jwks)
		case "/token":
			exchanges.Add(1)
			if err := req.ParseForm(); err != nil {
				t.Error(err)
			}
			authMu.Lock()
			query := authorization
			authMu.Unlock()
			if query == nil || req.Form.Get("redirect_uri") != query.Get("redirect_uri") || req.Form.Get("code") != "synthetic-current-subject" || oidcInitializationChallenge(req.Form.Get("code_verifier")) != query.Get("code_challenge") || query.Get("code_challenge_method") != "S256" {
				http.Error(w, "synthetic authorization/PKCE mismatch", 400)
				return
			}
			claims := map[string]any{"iss": base, "aud": query.Get("client_id"), "sub": "synthetic-current-subject", "nonce": query.Get("nonce"), "preferred_username": "Synthetic operator", "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			signed := signOIDCJOSEFixture(t, key, "RS256", "synthetic-handover", claims, false)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access", "id_token": signed, "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(idp.Close)
	t.Cleanup(unblock)
	InitPersistentAuthStores(t.TempDir())
	router, _ := setupTestRouter(t)
	router.config.PublicURL = "https://pulse.example.test"
	p := newRecoveryTestProvider("ownership", true, "")
	p.OIDC.IssuerURL = idp.URL
	if err := router.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := router.saveSSOConfig(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { router.oidcManager.RemoveService(p.ID) })
	staleResponse, done := httptest.NewRecorder(), make(chan struct{})
	go func() {
		defer close(done)
		router.handleSSOOIDCLogin(staleResponse, httptest.NewRequest("GET", "https://pulse.example.test/api/oidc/ownership/login?returnTo=/alerts", nil))
	}()
	awaitOIDCInitialization(t, entered)
	updated := (&config.SSOConfig{Providers: []config.SSOProvider{p}}).Clone().Providers[0]
	updated.OIDC.ClientID = "pulse-current-client"
	payload, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	management := httptest.NewRecorder()
	router.handleSSOProvider(management, httptest.NewRequest("PUT", "/api/security/sso/providers/ownership", bytes.NewReader(payload)))
	if management.Code != http.StatusOK {
		t.Fatalf("management save failed: %d %s", management.Code, management.Body.String())
	}
	currentResponse := httptest.NewRecorder()
	router.handleSSOOIDCLogin(currentResponse, httptest.NewRequest("GET", "https://pulse.example.test/api/oidc/ownership/login?returnTo=/alerts", nil))
	authURL, err := url.Parse(currentResponse.Header().Get("Location"))
	if err != nil || authURL.Query().Get("state") == "" || authURL.Query().Get("client_id") != updated.OIDC.ClientID {
		t.Fatalf("current login did not issue state for saved client: %v", err)
	}
	authMu.Lock()
	authorization = authURL.Query()
	authMu.Unlock()
	current := router.oidcManager.GetService(p.ID)
	unblock()
	awaitOIDCInitialization(t, done)
	staleTarget, err := url.Parse(staleResponse.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	result := oidcManagementHandoverResult{ManagementStatus: management.Code, StaleLoginStatus: staleResponse.Code, StaleLoginError: staleTarget.Query().Get("oidc_error"), CurrentLoginStatus: currentResponse.Code, CurrentClientRetained: router.oidcManager.GetService(p.ID) == current}
	target := "https://pulse.example.test/api/oidc/ownership/callback?code=synthetic-current-subject&state=" + url.QueryEscape(authURL.Query().Get("state"))
	callback := doOIDCCallback(t, router, target)
	result.CallbackTarget = callback.Header().Get("Location")
	for _, cookie := range callback.Result().Cookies() {
		if cookie.Name != sessionCookieName(true) {
			continue
		}
		session := GetSessionStore().GetSession(cookie.Value)
		principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, p.ID, "synthetic-current-subject")
		if err != nil {
			t.Fatal(err)
		}
		result.SessionVerified = cookie.HttpOnly && cookie.Secure && session != nil && session.Username == principal && session.DisplayUsername == "Synthetic operator" && session.OIDCIssuer == idp.URL && session.OIDCClientID == updated.OIDC.ClientID && GetSessionStore().ValidateSession(cookie.Value)
	}
	result.ReplayTarget = doOIDCCallback(t, router, target).Header().Get("Location")
	result.DiscoveryCalls, result.TokenCalls = discoveries.Load(), exchanges.Load()
	return result
}

func TestOIDCInitializationManagementHandover(t *testing.T) {
	got := runOIDCInitializationManagementHandover(t)
	if got.ManagementStatus != 200 || got.StaleLoginStatus != 302 || got.StaleLoginError != "oidc_init_failed" || got.CurrentLoginStatus != 302 || !got.CurrentClientRetained || got.CallbackTarget != "/alerts?oidc=success" || !got.SessionVerified || got.ReplayTarget != "/?oidc=error&oidc_error=invalid_state" || got.DiscoveryCalls != 3 || got.TokenCalls != 1 {
		t.Errorf("management handover did not preserve current native login: %+v", got)
	}
}
