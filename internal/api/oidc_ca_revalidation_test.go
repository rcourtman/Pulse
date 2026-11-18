package api

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

func TestOIDCCABundleRevalidationFileBoundary(t *testing.T) {
	for _, kind := range []string{"empty", "blank", "regular", "padded", "relative", "trailing-slash", "exact-limit", "oversized", "symlink", "directory", "missing"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "CA bundle.pem")
			data := []byte("synthetic trust bytes\n")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			var wantErr error
			switch kind {
			case "empty":
				path = ""
				data = nil
			case "blank":
				path = " \t\n"
				data = nil
			case "trailing-slash":
				path += "/"
			case "padded":
				path = " \t" + path + "\t\n"
			case "relative":
				t.Chdir(root)
				path = " \t./CA bundle.pem\n"
			case "exact-limit", "oversized":
				size := maxSSOFileSize
				if kind == "oversized" {
					size++
					wantErr = securityutil.ErrUnsafeStorageFile
				}
				data = bytes.Repeat([]byte{'x'}, size)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				link := filepath.Join(root, "linked.pem")
				if err := os.Symlink(path, link); err != nil {
					t.Fatal(err)
				}
				path = link
				wantErr = securityutil.ErrUnsafeStorageFile
			case "directory":
				path = root
				wantErr = securityutil.ErrUnsafeStorageFile
			case "missing":
				path = filepath.Join(root, "missing.pem")
				wantErr = os.ErrNotExist
			}
			got, err := hashCABundle(path)
			if wantErr != nil {
				if !errors.Is(err, wantErr) || got != "" {
					t.Fatalf("%s: hash=%q err=%v, want no hash and %v", kind, got, err, wantErr)
				}
				return
			}
			want := ""
			if data != nil {
				want = fmt.Sprintf("%x", sha256.Sum256(data))
			}
			if err != nil || got != want {
				t.Fatalf("%s: hash=%q err=%v, want %q", kind, got, err, want)
			}
		})
	}
}

type oidcCABundleLoginResult struct {
	PathKind             string   `json:"pathKind"`
	DiscoveryUnavailable bool     `json:"discoveryUnavailable"`
	LoginStatus          []int    `json:"loginStatus"`
	CallbackStatus       []int    `json:"callbackStatus"`
	CallbackTarget       []string `json:"callbackTarget"`
	SessionVerified      []bool   `json:"sessionVerified"`
	ReplayTarget         []string `json:"replayTarget"`
	DiscoveryCalls       int32    `json:"discoveryCalls"`
	TokenCalls           int32    `json:"tokenCalls"`
	JWKSCalls            int32    `json:"jwksCalls"`
	ServiceReused        bool     `json:"serviceReused"`
}

// The real native GET and POST login paths share one locally trusted TLS IdP.
// Two outstanding logins must survive CA revalidation at both callbacks; no
// verifier/nonce/PKCE, TLS or session-storage override is used. The result also
// lets an evidence-only producer exercise the existing Login page consumer.
func runOIDCCABundleNativeLogins(t *testing.T, pathKind string, discoveryUnavailable bool) oidcCABundleLoginResult {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "synthetic-ca", Use: "sig", Algorithm: "RS256"}}})
	if err != nil {
		t.Fatal(err)
	}
	var discoveries, exchanges, keyReads atomic.Int32
	var discoveryRefused atomic.Bool
	var authMu sync.Mutex
	auth := map[string]url.Values{}
	idp := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		base := "https://" + req.Host
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			discoveries.Add(1)
			if discoveryRefused.Load() {
				http.Error(w, "synthetic discovery outage", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			keyReads.Add(1)
			_, _ = w.Write(jwks)
		case "/token":
			exchanges.Add(1)
			if err := req.ParseForm(); err != nil {
				t.Error(err)
				http.Error(w, "bad synthetic request", 400)
				return
			}
			authMu.Lock()
			query := auth[req.Form.Get("code")]
			authMu.Unlock()
			user, password, ok := req.BasicAuth()
			if !ok || user != "pulse-client" || password != "client-secret" || query == nil || req.Form.Get("redirect_uri") != query.Get("redirect_uri") || req.Form.Get("grant_type") != "authorization_code" || query.Get("nonce") == "" || query.Get("code_challenge_method") != "S256" || oidcInitializationChallenge(req.Form.Get("code_verifier")) != query.Get("code_challenge") {
				http.Error(w, "synthetic client/callback/PKCE mismatch", 400)
				return
			}
			claims := map[string]any{"iss": base, "aud": "pulse-client", "sub": req.Form.Get("code"), "nonce": query.Get("nonce"), "preferred_username": "Synthetic operator", "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			signed := signOIDCJOSEFixture(t, key, "RS256", "synthetic-ca", claims, false)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access", "id_token": signed, "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(idp.Close)
	caPath := filepath.Join(t.TempDir(), "CA bundle.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.Certificate().Raw})
	if err := os.WriteFile(caPath, cert, 0600); err != nil {
		t.Fatal(err)
	}
	p := newRecoveryTestProvider("ca-path", true, "")
	p.OIDC.IssuerURL = idp.URL
	p.OIDC.CABundle = caPath
	if pathKind == "padded" {
		p.OIDC.CABundle = " \t" + caPath + "\t\n"
	}
	if pathKind == "trailing-slash" {
		p.OIDC.CABundle += "/"
	}
	if pathKind != "plain" && pathKind != "padded" && pathKind != "trailing-slash" && pathKind != "symlink" {
		t.Fatalf("unknown fixture kind %q", pathKind)
	}
	useSAMLTestAuthStores(t) // Existing isolated session/CSRF save-and-restore helper.
	router := newRecoveryTestRouter(t, []config.SSOProvider{p}, nil)
	router.config.PublicURL = "https://pulse.example.test"
	t.Cleanup(func() { router.oidcManager.RemoveService(p.ID) })
	var original *OIDCService
	result := oidcCABundleLoginResult{PathKind: pathKind, DiscoveryUnavailable: discoveryUnavailable, ServiceReused: true}
	callbacks := make([]string, 0, 2)
	for i, method := range []string{http.MethodGet, http.MethodPost} {
		var body *strings.Reader
		target := "https://pulse.example.test/api/oidc/ca-path/login?returnTo=/alerts"
		body = strings.NewReader(`{"returnTo":"/alerts"}`)
		req := httptest.NewRequest(method, target, body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.handleSSOOIDCLogin(rec, req)
		result.LoginStatus = append(result.LoginStatus, rec.Code)
		authorizationURL := rec.Header().Get("Location")
		if method == http.MethodPost {
			var payload struct {
				AuthorizationURL string `json:"authorizationUrl"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			authorizationURL = payload.AuthorizationURL
		}
		authorization, err := url.Parse(authorizationURL)
		if err != nil || authorization.Query().Get("state") == "" || authorization.Scheme != "https" {
			t.Fatalf("native login %d did not issue secure authorization state: %v", i, err)
		}
		if i == 0 {
			original = router.oidcManager.GetService(p.ID)
		}
		result.ServiceReused = result.ServiceReused && router.oidcManager.GetService(p.ID) == original
		code := fmt.Sprintf("synthetic-ca-subject-%d", i)
		authMu.Lock()
		auth[code] = authorization.Query()
		authMu.Unlock()
		callbacks = append(callbacks, "https://pulse.example.test/api/oidc/ca-path/callback?code="+code+"&state="+url.QueryEscape(authorization.Query().Get("state")))
	}
	if pathKind == "symlink" {
		held := caPath + ".original"
		if err := os.Rename(caPath, held); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(held, caPath); err != nil {
			t.Fatal(err)
		}
	}
	discoveryRefused.Store(discoveryUnavailable)
	for i, callback := range callbacks {
		rec := doOIDCCallback(t, router, callback)
		result.CallbackStatus = append(result.CallbackStatus, rec.Code)
		result.CallbackTarget = append(result.CallbackTarget, rec.Header().Get("Location"))
		result.ServiceReused = result.ServiceReused && router.oidcManager.GetService(p.ID) == original
		verified := false
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name != sessionCookieName(true) {
				continue
			}
			session := GetSessionStore().GetSession(cookie.Value)
			want, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, p.ID, fmt.Sprintf("synthetic-ca-subject-%d", i))
			if err != nil {
				t.Fatal(err)
			}
			verified = cookie.HttpOnly && cookie.Secure && cookie.SameSite == http.SameSiteLaxMode && session != nil && session.Username == want && session.DisplayUsername == "Synthetic operator" && session.OIDCIssuer == idp.URL && GetSessionStore().ValidateSession(cookie.Value)
		}
		result.SessionVerified = append(result.SessionVerified, verified)
		replay := doOIDCCallback(t, router, callback)
		result.ReplayTarget = append(result.ReplayTarget, replay.Header().Get("Location"))
		if len(replay.Result().Cookies()) != 0 {
			t.Error("replayed callback wrote cookies")
		}
	}
	result.DiscoveryCalls, result.TokenCalls, result.JWKSCalls = discoveries.Load(), exchanges.Load(), keyReads.Load()
	return result
}

func TestOIDCCABundleRevalidationPreservesNativeLogins(t *testing.T) {
	for _, pathKind := range []string{"plain", "padded", "trailing-slash"} {
		for _, unavailable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/discovery-unavailable-%t", pathKind, unavailable), func(t *testing.T) {
				got := runOIDCCABundleNativeLogins(t, pathKind, unavailable)
				if len(got.LoginStatus) != 2 || got.LoginStatus[0] != http.StatusFound || got.LoginStatus[1] != http.StatusOK {
					t.Fatalf("native GET/POST login status: %+v", got)
				}
				for i := range 2 {
					if got.CallbackStatus[i] != http.StatusFound || got.CallbackTarget[i] != "/alerts?oidc=success" || !got.SessionVerified[i] {
						t.Errorf("login %d did not complete a verified session: %+v", i, got)
					}
					if got.ReplayTarget[i] != "/?oidc=error&oidc_error=invalid_state" {
						t.Errorf("login %d replay did not fail closed: %+v", i, got)
					}
				}
				if !got.ServiceReused || got.DiscoveryCalls != 1 || got.TokenCalls != 2 || got.JWKSCalls != 1 {
					t.Errorf("unchanged CA did not retain the initialized native owner: %+v", got)
				}
			})
		}
	}
}

func TestOIDCCABundleRevalidationTracksTrustContents(t *testing.T) {
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		base := "http://" + req.Host
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
	}))
	t.Cleanup(idp.Close)
	certServer := newIPv4TLSServer(t, http.NotFoundHandler())
	t.Cleanup(certServer.Close)
	path := filepath.Join(t.TempDir(), "CA bundle.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certServer.Certificate().Raw})
	if err := os.WriteFile(path, cert, 0600); err != nil {
		t.Fatal(err)
	}
	p := newRecoveryTestProvider("ca-rotation", true, "https://pulse.example.test/api/oidc/ca-rotation/callback")
	p.OIDC.IssuerURL, p.OIDC.CABundle = idp.URL, " \t"+path+"\n"
	manager := NewOIDCServiceManager()
	t.Cleanup(func() { manager.RemoveService(p.ID) })
	if err := manager.EnsureProvider(t.Context(), p.ID, &p, p.OIDC.RedirectURL); err != nil {
		t.Fatal(err)
	}
	old := manager.GetService(p.ID)
	state, _, err := old.newStateEntry(p.ID, "/alerts")
	if err != nil {
		t.Fatal(err)
	}
	if !old.Matches(ssoProviderToOIDCConfig(&p, p.OIDC.RedirectURL)) {
		t.Fatal("unchanged padded CA path failed cache matching")
	}
	// Even an otherwise valid trust edit at the same path changes the snapshot.
	if err := os.WriteFile(path, append(cert, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if old.Matches(ssoProviderToOIDCConfig(&p, p.OIDC.RedirectURL)) {
		t.Fatal("changed trust contents matched the old owner")
	}
	if err := manager.EnsureProvider(t.Context(), p.ID, &p, p.OIDC.RedirectURL); err != nil {
		t.Fatal(err)
	}
	current := manager.GetService(p.ID)
	if current == old {
		t.Fatal("changed trust reused the old owner")
	}
	if _, ok := current.peekState(state); ok {
		t.Fatal("pending nonce/PKCE state crossed changed trust")
	}
	select {
	case <-old.stateStore.stopCleanup:
	default:
		t.Error("replaced state owner not stopped")
	}
}

// Revalidation must not keep a cached provider usable after its formerly valid
// trust file becomes unreadable/unsafe, nor delete that owner's pending state.
// Restoring the original regular bytes makes the original owner usable again.
func TestOIDCCABundleRevalidationRefusesReplacedTrustFile(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "oversized", "missing"} {
		t.Run(kind, func(t *testing.T) {
			var discoveries atomic.Int32
			idp := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				discoveries.Add(1)
				base := "https://" + req.Host
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
			}))
			t.Cleanup(idp.Close)
			path := filepath.Join(t.TempDir(), "trust.pem")
			cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.Certificate().Raw})
			if err := os.WriteFile(path, cert, 0600); err != nil {
				t.Fatal(err)
			}
			p := newRecoveryTestProvider("ca-path", true, "https://pulse.example.test/api/oidc/ca-path/callback")
			p.OIDC.IssuerURL, p.OIDC.CABundle = idp.URL, path
			router := newRecoveryTestRouter(t, []config.SSOProvider{p}, nil)
			router.config.PublicURL = "https://pulse.example.test"
			manager := router.oidcManager
			t.Cleanup(func() { manager.RemoveService(p.ID) })
			if err := manager.EnsureProvider(t.Context(), p.ID, &p, p.OIDC.RedirectURL); err != nil {
				t.Fatal(err)
			}
			original := manager.GetService(p.ID)
			state, _, err := original.newStateEntry(p.ID, "/alerts")
			if err != nil {
				t.Fatal(err)
			}
			held := filepath.Join(filepath.Dir(path), "original.pem")
			if err := os.Rename(path, held); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				err = os.Symlink(held, path)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "oversized":
				err = os.WriteFile(path, bytes.Repeat([]byte{'x'}, maxSSOFileSize+1), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if original.Matches(ssoProviderToOIDCConfig(&p, p.OIDC.RedirectURL)) {
				t.Fatal("unsafe/missing replacement matched cached trust")
			}
			if err := manager.EnsureProvider(t.Context(), p.ID, &p, p.OIDC.RedirectURL); err == nil {
				t.Fatal("unsafe/missing trust initialized")
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				rec := httptest.NewRecorder()
				router.handleSSOOIDCLogin(rec, httptest.NewRequest(method, "https://pulse.example.test/api/oidc/ca-path/login", strings.NewReader(`{}`)))
				if method == http.MethodGet {
					if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?oidc=error&oidc_error=oidc_init_failed" {
						t.Fatalf("unsafe trust GET status/redirect = %d %q", rec.Code, rec.Header().Get("Location"))
					}
				} else if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":"oidc_init_failed"`) {
					t.Fatalf("unsafe trust POST response = %d %s", rec.Code, rec.Body.String())
				}
				if len(rec.Result().Cookies()) != 0 || strings.Contains(rec.Body.String(), path) {
					t.Error("refused login issued cookies or disclosed configured path")
				}
			}
			callback := doOIDCCallback(t, router, "https://pulse.example.test/api/oidc/ca-path/callback?state="+url.QueryEscape(state)+"&code=unused-synthetic-code")
			if callback.Code != http.StatusFound || callback.Header().Get("Location") != "/?oidc=error&oidc_error=oidc_init_failed" || len(callback.Result().Cookies()) != 0 {
				t.Fatal("unsafe trust callback did not fail closed before code exchange")
			}
			if manager.GetService(p.ID) != original || discoveries.Load() != 1 {
				t.Fatal("refused trust changed the owner or reached discovery")
			}
			if _, ok := original.peekState(state); !ok {
				t.Fatal("failed revalidation deleted retained pending state")
			}
			if kind != "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(held, path); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			router.handleSSOOIDCLogin(rec, httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/oidc/ca-path/login", nil))
			location, err := url.Parse(rec.Header().Get("Location"))
			if err != nil || rec.Code != http.StatusFound || location.Query().Get("state") == "" || manager.GetService(p.ID) != original || discoveries.Load() != 1 {
				t.Fatal("restored regular trust did not recover the original native owner")
			}
			if _, ok := original.peekState(state); !ok {
				t.Fatal("restored trust lost the earlier pending login")
			}
		})
	}
}

func TestOIDCCABundleRevalidationNativeSymlinkRefusal(t *testing.T) {
	got := runOIDCCABundleNativeLogins(t, "symlink", false)
	for i := range 2 {
		if got.CallbackStatus[i] != http.StatusFound || got.CallbackTarget[i] != "/?oidc=error&oidc_error=oidc_init_failed" || got.SessionVerified[i] {
			t.Errorf("symlink callback %d did not refuse before a session: %+v", i, got)
		}
	}
	if !got.ServiceReused || got.DiscoveryCalls != 1 || got.TokenCalls != 0 || got.JWKSCalls != 0 {
		t.Errorf("unsafe trust reached an IdP request or replaced the retained owner: %+v", got)
	}
}
