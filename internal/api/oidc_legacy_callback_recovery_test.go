package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"golang.org/x/oauth2"
)

const (
	testRecoveryProviderID      = "0b6a7f0e-8d5f-4a83-9a52-9a4a2f3b9d10"
	testRecoveryOtherProviderID = "5f8e2c1a-73d4-4b6e-8f21-0c9d7e5a4b32"
	testLegacyCallbackRedirect  = "http://pulse.example.com/api/oidc/callback"
)

// newRecoveryTestProvider returns an SSO OIDC provider the way the v6 UI
// creates them (UUID id), with the IdP redirect URI registered at redirectURL.
func newRecoveryTestProvider(id string, enabled bool, redirectURL string) config.SSOProvider {
	return config.SSOProvider{
		ID:      id,
		Name:    "Authentik",
		Type:    config.SSOProviderTypeOIDC,
		Enabled: enabled,
		OIDC: &config.OIDCProviderConfig{
			IssuerURL:    "https://idp.example.com",
			ClientID:     "pulse-client",
			ClientSecret: "client-secret",
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "profile", "email"},
		},
	}
}

// newRecoveryTestService hand-builds an OIDCService whose snapshot matches
// ssoProviderToOIDCConfig(provider, redirectURL), so the callback handler
// reuses it instead of re-running issuer discovery over the network.
func newRecoveryTestService(provider *config.SSOProvider, redirectURL, tokenURL string, client *http.Client) *OIDCService {
	return &OIDCService{
		snapshot: oidcSnapshot{
			issuer:       provider.OIDC.IssuerURL,
			clientID:     provider.OIDC.ClientID,
			clientSecret: provider.OIDC.ClientSecret,
			redirectURL:  redirectURL,
			scopes:       []string{"openid", "profile", "email"},
		},
		oauth2Cfg: &oauth2.Config{
			ClientID:     provider.OIDC.ClientID,
			ClientSecret: provider.OIDC.ClientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     oauth2.Endpoint{TokenURL: tokenURL},
		},
		stateStore: newOIDCStateStore(),
		httpClient: client,
	}
}

func newRecoveryTestRouter(t *testing.T, providers []config.SSOProvider, services map[string]*OIDCService) *Router {
	t.Helper()
	manager := NewOIDCServiceManager()
	for id, svc := range services {
		manager.services[id] = svc
		s := svc
		t.Cleanup(s.Stop)
	}
	return &Router{
		config:      &config.Config{},
		ssoConfig:   &config.SSOConfig{Providers: providers},
		oidcManager: manager,
	}
}

func doOIDCCallback(t *testing.T, router *Router, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	router.handleSSOOIDCCallback(rec, req)
	return rec
}

// TestSSOOIDCCallbackLegacyPathResolvesInitiatingProvider reproduces GitHub
// issue #1533: a provider created in the v6 UI gets a UUID id, but its IdP is
// registered with the legacy v5 redirect URI /api/oidc/callback. The callback
// arrives on the 3-part legacy path, which hard-maps to the legacy-oidc
// sentinel and fails provider lookup even though the pending state entry
// records the provider that actually initiated the flow. The callback must
// recover that provider and proceed down the normal state/token path.
func TestSSOOIDCCallbackLegacyPathResolvesInitiatingProvider(t *testing.T) {
	tokenSrv := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Deliberately no id_token: the test only needs to prove the callback
		// reached the code-exchange stage for the recovered provider.
		fmt.Fprint(w, `{"access_token":"access","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenSrv.Close()

	provider := newRecoveryTestProvider(testRecoveryProviderID, true, testLegacyCallbackRedirect)
	svc := newRecoveryTestService(&provider, testLegacyCallbackRedirect, tokenSrv.URL, tokenSrv.Client())
	router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{testRecoveryProviderID: svc})

	state, _, err := svc.newStateEntry(testRecoveryProviderID, "/after-login")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}

	target := testLegacyCallbackRedirect + "?state=" + state + "&code=auth-code"
	rec := doOIDCCallback(t, router, target)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if loc == "/?oidc=error&oidc_error=provider_not_found" {
		t.Fatalf("legacy callback rejected the initiating provider: %q", loc)
	}
	// Past provider resolution, state consumption, the provider-mismatch
	// check, and code exchange, the flow fails only on the stubbed token
	// response missing an id_token — with the state entry's returnTo intact.
	want := "/after-login?oidc=error&oidc_error=missing_id_token"
	if loc != want {
		t.Fatalf("Location = %q, want %q", loc, want)
	}

	// State stays single-use: replaying the same callback must fail closed.
	rec2 := doOIDCCallback(t, router, target)
	if loc2 := rec2.Header().Get("Location"); loc2 != "/?oidc=error&oidc_error=provider_not_found" {
		t.Fatalf("replayed state Location = %q, want provider_not_found fail-closed", loc2)
	}
}

// TestSSOOIDCCallbackLegacyPathFailsClosed pins the security invariants of the
// legacy-path provider recovery: anything that cannot be resolved to a valid,
// enabled OIDC provider through a live state entry keeps failing closed.
func TestSSOOIDCCallbackLegacyPathFailsClosed(t *testing.T) {
	const providerNotFound = "/?oidc=error&oidc_error=provider_not_found"

	t.Run("missing state parameter", func(t *testing.T) {
		provider := newRecoveryTestProvider(testRecoveryProviderID, true, testLegacyCallbackRedirect)
		svc := newRecoveryTestService(&provider, testLegacyCallbackRedirect, "http://127.0.0.1:0", nil)
		router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{testRecoveryProviderID: svc})

		rec := doOIDCCallback(t, router, testLegacyCallbackRedirect+"?code=auth-code")
		if loc := rec.Header().Get("Location"); loc != providerNotFound {
			t.Fatalf("Location = %q, want %q", loc, providerNotFound)
		}
	})

	t.Run("unknown state", func(t *testing.T) {
		provider := newRecoveryTestProvider(testRecoveryProviderID, true, testLegacyCallbackRedirect)
		svc := newRecoveryTestService(&provider, testLegacyCallbackRedirect, "http://127.0.0.1:0", nil)
		router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{testRecoveryProviderID: svc})

		rec := doOIDCCallback(t, router, testLegacyCallbackRedirect+"?state=never-issued&code=auth-code")
		if loc := rec.Header().Get("Location"); loc != providerNotFound {
			t.Fatalf("Location = %q, want %q", loc, providerNotFound)
		}
	})

	t.Run("expired state", func(t *testing.T) {
		provider := newRecoveryTestProvider(testRecoveryProviderID, true, testLegacyCallbackRedirect)
		svc := newRecoveryTestService(&provider, testLegacyCallbackRedirect, "http://127.0.0.1:0", nil)
		router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{testRecoveryProviderID: svc})

		svc.stateStore.Put("expired-state", &oidcStateEntry{
			ProviderID: testRecoveryProviderID,
			ExpiresAt:  time.Now().Add(-time.Minute),
		})

		rec := doOIDCCallback(t, router, testLegacyCallbackRedirect+"?state=expired-state&code=auth-code")
		if loc := rec.Header().Get("Location"); loc != providerNotFound {
			t.Fatalf("Location = %q, want %q", loc, providerNotFound)
		}
	})

	t.Run("disabled provider is not recovered", func(t *testing.T) {
		provider := newRecoveryTestProvider(testRecoveryProviderID, false, testLegacyCallbackRedirect)
		svc := newRecoveryTestService(&provider, testLegacyCallbackRedirect, "http://127.0.0.1:0", nil)
		router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{testRecoveryProviderID: svc})

		state, _, err := svc.newStateEntry(testRecoveryProviderID, "/after-login")
		if err != nil {
			t.Fatalf("newStateEntry error: %v", err)
		}

		rec := doOIDCCallback(t, router, testLegacyCallbackRedirect+"?state="+state+"&code=auth-code")
		if loc := rec.Header().Get("Location"); loc != providerNotFound {
			t.Fatalf("Location = %q, want %q", loc, providerNotFound)
		}
	})

	t.Run("state naming an unconfigured provider is not recovered", func(t *testing.T) {
		provider := newRecoveryTestProvider(testRecoveryProviderID, true, testLegacyCallbackRedirect)
		ghost := newRecoveryTestProvider("ghost-provider", true, testLegacyCallbackRedirect)
		ghostSvc := newRecoveryTestService(&ghost, testLegacyCallbackRedirect, "http://127.0.0.1:0", nil)
		// The service exists in the manager but no matching provider is configured.
		router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{"ghost-provider": ghostSvc})

		state, _, err := ghostSvc.newStateEntry("ghost-provider", "/after-login")
		if err != nil {
			t.Fatalf("newStateEntry error: %v", err)
		}

		rec := doOIDCCallback(t, router, testLegacyCallbackRedirect+"?state="+state+"&code=auth-code")
		if loc := rec.Header().Get("Location"); loc != providerNotFound {
			t.Fatalf("Location = %q, want %q", loc, providerNotFound)
		}
	})
}

// TestSSOOIDCCallbackProviderMismatchStillRejected pins the existing 4-part
// path protection: a state minted for provider A must not be accepted on a
// callback path claiming provider B.
func TestSSOOIDCCallbackProviderMismatchStillRejected(t *testing.T) {
	redirect := "http://pulse.example.com/api/oidc/" + testRecoveryOtherProviderID + "/callback"
	provider := newRecoveryTestProvider(testRecoveryOtherProviderID, true, redirect)
	svc := newRecoveryTestService(&provider, redirect, "http://127.0.0.1:0", nil)
	router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{testRecoveryOtherProviderID: svc})

	// Plant a state naming a different provider into this provider's store.
	svc.stateStore.Put("cross-state", &oidcStateEntry{
		ProviderID: testRecoveryProviderID,
		ExpiresAt:  time.Now().Add(time.Minute),
	})

	rec := doOIDCCallback(t, router, redirect+"?state=cross-state&code=auth-code")
	want := "/?oidc=error&oidc_error=provider_mismatch"
	if loc := rec.Header().Get("Location"); loc != want {
		t.Fatalf("Location = %q, want %q", loc, want)
	}
}

func TestOIDCStateStorePeek(t *testing.T) {
	store := &oidcStateStore{entries: make(map[string]*oidcStateEntry), stopCleanup: make(chan struct{})}
	store.Put("active", &oidcStateEntry{ProviderID: "p1", ExpiresAt: time.Now().Add(time.Minute)})
	store.Put("expired", &oidcStateEntry{ProviderID: "p1", ExpiresAt: time.Now().Add(-time.Minute)})

	entry, ok := store.Peek("active")
	if !ok || entry == nil || entry.ProviderID != "p1" {
		t.Fatalf("Peek(active) = %v, %v; want live entry for p1", entry, ok)
	}
	// Peek must not consume.
	if _, ok := store.Peek("active"); !ok {
		t.Fatal("second Peek missed: Peek consumed the entry")
	}
	if _, ok := store.Consume("active"); !ok {
		t.Fatal("expected Consume to succeed after Peek")
	}
	if _, ok := store.Peek("active"); ok {
		t.Fatal("expected entry to be gone after Consume")
	}

	if _, ok := store.Peek("expired"); ok {
		t.Fatal("expected expired entry to be treated as absent")
	}
	if _, ok := store.Peek("missing"); ok {
		t.Fatal("expected missing entry to be absent")
	}
}

func TestOIDCServiceManagerLookupStateProvider(t *testing.T) {
	manager := NewOIDCServiceManager()
	svc := &OIDCService{stateStore: newOIDCStateStore()}
	t.Cleanup(svc.Stop)
	manager.services["provider-a"] = svc

	state, _, err := svc.newStateEntry("provider-a", "/home")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}

	id, ok := manager.LookupStateProvider(state)
	if !ok || id != "provider-a" {
		t.Fatalf("LookupStateProvider = %q, %v; want provider-a, true", id, ok)
	}

	// Lookup is peek-only: the state must still be consumable exactly once.
	if _, ok := svc.consumeState(state); !ok {
		t.Fatal("expected state to remain consumable after lookup")
	}
	if _, ok := manager.LookupStateProvider(state); ok {
		t.Fatal("expected lookup to miss after the state was consumed")
	}

	if _, ok := manager.LookupStateProvider(""); ok {
		t.Fatal("expected empty state to miss")
	}
	if _, ok := manager.LookupStateProvider("never-issued"); ok {
		t.Fatal("expected unknown state to miss")
	}

	// An entry naming a different provider than the service holding it must
	// not resolve.
	svc.stateStore.Put("foreign", &oidcStateEntry{ProviderID: "provider-b", ExpiresAt: time.Now().Add(time.Minute)})
	if _, ok := manager.LookupStateProvider("foreign"); ok {
		t.Fatal("expected foreign entry to be ignored")
	}

	// Expired entries must not resolve.
	svc.stateStore.Put("stale", &oidcStateEntry{ProviderID: "provider-a", ExpiresAt: time.Now().Add(-time.Minute)})
	if _, ok := manager.LookupStateProvider("stale"); ok {
		t.Fatal("expected expired entry to be ignored")
	}

	var nilManager *OIDCServiceManager
	if _, ok := nilManager.LookupStateProvider("anything"); ok {
		t.Fatal("expected nil manager to miss safely")
	}
}

// TestSSOOIDCCallbackLegacyProviderStillServed is the control: a genuinely
// migrated legacy provider keeps working on the legacy path exactly as before.
func TestSSOOIDCCallbackLegacyProviderStillServed(t *testing.T) {
	provider := newRecoveryTestProvider(config.LegacyOIDCProviderID, true, testLegacyCallbackRedirect)
	svc := newRecoveryTestService(&provider, testLegacyCallbackRedirect, "http://127.0.0.1:0", nil)
	router := newRecoveryTestRouter(t, []config.SSOProvider{provider}, map[string]*OIDCService{config.LegacyOIDCProviderID: svc})

	state, _, err := svc.newStateEntry(config.LegacyOIDCProviderID, "/legacy-home")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}

	// No code parameter: reaching missing_code with the state's returnTo
	// proves the provider resolved and the state was consumed as before.
	rec := doOIDCCallback(t, router, testLegacyCallbackRedirect+"?state="+state)
	want := "/legacy-home?oidc=error&oidc_error=missing_code"
	if loc := rec.Header().Get("Location"); loc != want {
		t.Fatalf("Location = %q, want %q", loc, want)
	}
}
