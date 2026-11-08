package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestOIDCInitializationReusesMatchingService(t *testing.T) {
	var discoveries atomic.Int32
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		discoveries.Add(1)
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
	}))
	t.Cleanup(idp.Close)
	p := newRecoveryTestProvider("initialization", true, "")
	p.OIDC.IssuerURL = idp.URL
	manager := NewOIDCServiceManager()
	t.Cleanup(func() { manager.RemoveService(p.ID) })
	const redirect = "https://pulse.example.test/api/oidc/initialization/callback"
	if err := manager.EnsureProvider(t.Context(), p.ID, &p, redirect); err != nil {
		t.Fatal(err)
	}
	original := manager.GetService(p.ID)
	state, entry, err := original.newStateEntry(p.ID, "/alerts")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := manager.EnsureProvider(t.Context(), p.ID, &p, redirect); err != nil {
			t.Fatal(err)
		}
	}
	current := manager.GetService(p.ID)
	if current != original {
		t.Error("unchanged initialization replaced the service holding pending logins")
	}
	if discoveries.Load() != 1 {
		t.Errorf("unchanged initialization repeated discovery: %d calls", discoveries.Load())
	}
	if got, ok := current.consumeState(state); !ok || !reflect.DeepEqual(got, entry) {
		t.Error("unchanged initialization lost nonce, PKCE, expiry or return path")
	}
	if _, ok := current.consumeState(state); ok {
		t.Error("reused service accepted replayed state")
	}
	// Management still owns an explicit reset, even for identical settings.
	pending, _, err := current.newStateEntry(p.ID, "/alerts")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.InitializeProvider(t.Context(), p.ID, &p, redirect); err != nil {
		t.Fatal(err)
	}
	reset := manager.GetService(p.ID)
	if reset == current {
		t.Error("explicit management initialization did not replace the service")
	}
	if _, ok := reset.peekState(pending); ok {
		t.Error("explicit management reset retained old pending authority")
	}
}

func TestOIDCInitializationChangedSettingsReplaceState(t *testing.T) {
	for _, field := range []string{"issuer", "client", "secret", "redirect", "scopes", "ca-content"} {
		t.Run(field, func(t *testing.T) {
			idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				base := "http://" + req.Host
				_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base + strings.TrimSuffix(req.URL.Path, "/.well-known/openid-configuration"), "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
			}))
			t.Cleanup(idp.Close)
			p := newRecoveryTestProvider("initialization", true, "")
			p.OIDC.IssuerURL = idp.URL
			redirect := "https://pulse.example.test/api/oidc/initialization/callback"
			if field == "ca-content" {
				// Both trust files are valid PEM. Change bytes at the same path;
				// path equality must not keep a stale trust snapshot.
				p.OIDC.CABundle = filepath.Join(t.TempDir(), "ca.pem")
				certificateServer := newIPv4TLSServer(t, http.NotFoundHandler())
				t.Cleanup(certificateServer.Close)
				certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateServer.Certificate().Raw})
				if err := os.WriteFile(p.OIDC.CABundle, certificate, 0600); err != nil {
					t.Fatal(err)
				}
			}
			manager := NewOIDCServiceManager()
			t.Cleanup(func() { manager.RemoveService(p.ID) })
			if err := manager.EnsureProvider(t.Context(), p.ID, &p, redirect); err != nil {
				t.Fatal(err)
			}
			original := manager.GetService(p.ID)
			state, _, err := original.newStateEntry(p.ID, "/alerts")
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "issuer":
				// A different valid issuer on the same synthetic server.
				p.OIDC.IssuerURL += "/changed"
			case "client":
				p.OIDC.ClientID += "-changed"
			case "secret":
				p.OIDC.ClientSecret += "-changed"
			case "redirect":
				redirect += "?changed=yes"
			case "scopes":
				p.OIDC.Scopes = append(p.OIDC.Scopes, "offline_access")
			case "ca-content":
				certificate, err := os.ReadFile(p.OIDC.CABundle)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p.OIDC.CABundle, append(certificate, byte('\n')), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := manager.EnsureProvider(t.Context(), p.ID, &p, redirect); err != nil {
				t.Fatal(err)
			}
			current := manager.GetService(p.ID)
			if current == original {
				t.Error("changed authentication/trust settings reused old service")
			}
			if _, ok := current.peekState(state); ok {
				t.Error("pending state crossed a changed authentication/trust snapshot")
			}
			select {
			case <-original.stateStore.stopCleanup:
			default:
				t.Error("replaced service cleanup was not stopped")
			}
		})
	}
}

// This result carries no raw credentials. It is also used by an evidence-only
// producer to bind native callback results to the existing login-page consumer.
type oidcInitializationResult struct {
	LoginStatus     []int    `json:"loginStatus"`
	CallbackStatus  []int    `json:"callbackStatus"`
	CallbackTarget  []string `json:"callbackTarget"`
	SessionVerified []bool   `json:"sessionVerified"`
	ReplayTarget    []string `json:"replayTarget"`
	DiscoveryCalls  int32    `json:"discoveryCalls"`
	TokenCalls      int32    `json:"tokenCalls"`
}

// Place the second discovery completion after the first login's state is
// issued. Both handlers used the real initially empty manager. No sleeps,
// alternate auth path, verifier overrides or pre-seeded state are involved.
func runOIDCInitializationConcurrentLogins(t *testing.T) oidcInitializationResult {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "synthetic-init", Use: "sig", Algorithm: "RS256"}}})
	if err != nil {
		t.Fatal(err)
	}
	var discoveries, exchanges atomic.Int32
	entered := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	release := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	var once [2]sync.Once
	unblock := func(i int) { once[i].Do(func() { close(release[i]) }) }
	var authMu sync.Mutex
	auth := map[string]url.Values{}
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			call := discoveries.Add(1)
			if call <= 2 {
				close(entered[call-1])
				select {
				case <-release[call-1]:
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
			query := auth[req.Form.Get("code")]
			authMu.Unlock()
			if query == nil || req.Form.Get("redirect_uri") != query.Get("redirect_uri") {
				http.Error(w, "synthetic authorization missing or callback changed", 400)
				return
			}
			// Verify Pulse sent the verifier corresponding to its S256 challenge.
			if got := oidcInitializationChallenge(req.Form.Get("code_verifier")); got == "" || got != query.Get("code_challenge") || query.Get("code_challenge_method") != "S256" {
				http.Error(w, "synthetic PKCE mismatch", 400)
				return
			}
			claims := map[string]any{"iss": base, "aud": "pulse-client", "sub": req.Form.Get("code"), "nonce": query.Get("nonce"), "preferred_username": "Synthetic operator", "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			signed := signOIDCJOSEFixture(t, key, "RS256", "synthetic-init", claims, false)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access", "id_token": signed, "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(idp.Close)
	t.Cleanup(func() { unblock(0); unblock(1) })
	InitPersistentAuthStores(t.TempDir())
	t.Cleanup(idp.Close)
	p := newRecoveryTestProvider("initialization", true, "")
	p.OIDC.IssuerURL = idp.URL
	router := newRecoveryTestRouter(t, []config.SSOProvider{p}, nil)
	router.config.PublicURL = "https://pulse.example.test"
	t.Cleanup(func() { router.oidcManager.RemoveService(p.ID) })
	responses := [2]*httptest.ResponseRecorder{httptest.NewRecorder(), httptest.NewRecorder()}
	done := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
	await := func(ch <-chan struct{}, action string) {
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for %s", action)
		}
	}
	for i := range 2 {
		go func(i int) {
			defer close(done[i])
			req := httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/oidc/initialization/login?returnTo=/alerts", nil)
			router.handleSSOOIDCLogin(responses[i], req)
		}(i)
		await(entered[i], "native login discovery")
	}
	unblock(0)
	await(done[0], "first native login")
	unblock(1)
	await(done[1], "second native login")

	result := oidcInitializationResult{DiscoveryCalls: discoveries.Load()}
	for i, response := range responses {
		result.LoginStatus = append(result.LoginStatus, response.Code)
		authorization, err := url.Parse(response.Header().Get("Location"))
		if err != nil || authorization.Query().Get("state") == "" {
			t.Fatalf("native login %d did not issue authorization state: %v", i, err)
		}
		code := []string{"synthetic-first-subject", "synthetic-second-subject"}[i]
		authMu.Lock()
		auth[code] = authorization.Query()
		authMu.Unlock()
		target := "/api/oidc/initialization/callback?code=" + code + "&state=" + url.QueryEscape(authorization.Query().Get("state"))
		response := doOIDCCallback(t, router, "https://pulse.example.test"+target)
		result.CallbackStatus = append(result.CallbackStatus, response.Code)
		result.CallbackTarget = append(result.CallbackTarget, response.Header().Get("Location"))
		verified := false
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name != sessionCookieName(true) {
				continue
			}
			session := GetSessionStore().GetSession(cookie.Value)
			want, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, p.ID, code)
			if err != nil {
				t.Fatal(err)
			}
			verified = cookie.HttpOnly && cookie.Secure && session != nil && session.Username == want && session.DisplayUsername == "Synthetic operator" && session.OIDCIssuer == idp.URL && GetSessionStore().ValidateSession(cookie.Value)
		}
		result.SessionVerified = append(result.SessionVerified, verified)
		result.ReplayTarget = append(result.ReplayTarget, doOIDCCallback(t, router, "https://pulse.example.test"+target).Header().Get("Location"))
	}
	result.TokenCalls = exchanges.Load()
	return result
}

func TestOIDCInitializationConcurrentNativeLogins(t *testing.T) {
	got := runOIDCInitializationConcurrentLogins(t)
	for i := range 2 {
		if got.LoginStatus[i] != 302 || got.CallbackStatus[i] != 302 || got.CallbackTarget[i] != "/alerts?oidc=success" || !got.SessionVerified[i] {
			t.Errorf("login %d lost its independently verified native session: %+v", i, got)
		}
		if got.ReplayTarget[i] != "/?oidc=error&oidc_error=invalid_state" {
			t.Errorf("login %d replay did not fail closed: %+v", i, got)
		}
	}
	if got.DiscoveryCalls != 2 || got.TokenCalls != 2 {
		t.Errorf("expected two deliberately overlapping discoveries and two code exchanges: %+v", got)
	}
}

// A failed replacement must leave the original service/state usable. Removal
// must withdraw the mapping; identical later initialization is a fresh service.
func TestOIDCInitializationFailureAndRemoval(t *testing.T) {
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
	}))
	t.Cleanup(idp.Close)
	p := newRecoveryTestProvider("initialization", true, "")
	p.OIDC.IssuerURL = idp.URL
	manager := NewOIDCServiceManager()
	t.Cleanup(func() { manager.RemoveService(p.ID) })
	if err := manager.EnsureProvider(t.Context(), p.ID, &p, "https://pulse.example.test/callback"); err != nil {
		t.Fatal(err)
	}
	original := manager.GetService(p.ID)
	state, entry, err := original.newStateEntry(p.ID, "/alerts")
	if err != nil {
		t.Fatal(err)
	}
	changed := p
	copy := *p.OIDC
	changed.OIDC = &copy
	changed.OIDC.ClientID += "-changed"
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := manager.EnsureProvider(ctx, p.ID, &changed, "https://pulse.example.test/callback"); err == nil {
		t.Error("canceled discovery unexpectedly replaced the service")
	}
	if manager.GetService(p.ID) != original {
		t.Error("failed initialization replaced the live service")
	}
	if got, ok := original.peekState(state); !ok || !reflect.DeepEqual(got, entry) {
		t.Error("failed initialization erased pending login")
	}
	manager.RemoveService(p.ID)
	if manager.GetService(p.ID) != nil {
		t.Error("removed provider still published a service")
	}
	if err := manager.EnsureProvider(t.Context(), p.ID, &p, "https://pulse.example.test/callback"); err != nil {
		t.Fatal(err)
	}
	current := manager.GetService(p.ID)
	if current == original {
		t.Error("removed service was reused")
	}
	if _, ok := current.peekState(state); ok {
		t.Error("removed provider's pending state was resurrected")
	}
}

func oidcInitializationChallenge(verifier string) string {
	if verifier == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
