package api

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type oidcCallbackPolicyRecorder struct {
	internalauth.Manager
	roleWrites, identityWrites int
}

func (m *oidcCallbackPolicyRecorder) UpdateUserRoles(user string, roles []string) error {
	m.roleWrites++
	return m.Manager.UpdateUserRoles(user, roles)
}
func (m *oidcCallbackPolicyRecorder) UpsertUserIdentity(string, internalauth.UserIdentityMetadata) error {
	m.identityWrites++
	return nil
}

type oidcCallbackPolicyResult struct {
	Action                   string `json:"action"`
	Barrier                  string `json:"barrier"`
	CallbackTarget           string `json:"callbackTarget"`
	ReplayTarget             string `json:"replayTarget"`
	SessionVerified          bool   `json:"sessionVerified"`
	ExistingSessionPreserved bool   `json:"existingSessionPreserved"`
	RoleWrites               int    `json:"roleWrites"`
	IdentityWrites           int    `json:"identityWrites"`
	TokenCalls               int32  `json:"tokenCalls"`
}

// All identities/keys are synthetic and all HTTP stays guest-local. The channel
// barrier puts a completed management operation inside the real code exchange
// or signature-key fetch. A valid signature, nonce and PKCE are not authority
// to use provider policy which an administrator has since withdrawn.
func runOIDCCallbackPolicy(t *testing.T, key *rsa.PrivateKey, barrier, action string) oidcCallbackPolicyResult {
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
	jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "synthetic-callback", Use: "sig", Algorithm: "RS256"}}})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var authorization url.Values
	var tokenCalls atomic.Int32
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		if req.URL.Path == "/"+barrier {
			enteredOnce.Do(func() { close(entered) })
			select {
			case <-release:
			case <-req.Context().Done():
				return
			}
		}
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_, _ = w.Write(jwks)
		case "/token":
			tokenCalls.Add(1)
			if err := req.ParseForm(); err != nil {
				t.Error(err)
			}
			if req.Form.Get("code") != "synthetic-subject" || req.Form.Get("redirect_uri") != authorization.Get("redirect_uri") || oidcInitializationChallenge(req.Form.Get("code_verifier")) != authorization.Get("code_challenge") || authorization.Get("code_challenge_method") != "S256" {
				http.Error(w, "synthetic PKCE/code/callback mismatch", 400)
				return
			}
			claims := map[string]any{"iss": base, "aud": authorization.Get("client_id"), "sub": "synthetic-subject", "nonce": authorization.Get("nonce"), "preferred_username": "Synthetic operator", "email": "operator@example.test", "groups": []string{"ops"}, "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			signed := signOIDCJOSEFixture(t, key, "RS256", "synthetic-callback", claims, false)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-access", "id_token": signed, "token_type": "Bearer", "expires_in": 3600})
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(idp.Close)
	t.Cleanup(unblock)
	r, _ := setupTestRouter(t)
	r.config.PublicURL = "https://pulse.example.test"
	p := newRecoveryTestProvider("callback-policy", true, "")
	p.OIDC.IssuerURL = idp.URL
	p.GroupsClaim = "groups"
	p.GroupRoleMappings = map[string]string{"ops": internalauth.RoleViewer}
	if err := r.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := r.saveSSOConfig(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.oidcManager.RemoveService(p.ID); r.oidcManager.RemoveService("independent") })
	login := httptest.NewRecorder()
	r.handleSSOOIDCLogin(login, httptest.NewRequest("GET", "https://pulse.example.test/api/oidc/"+p.ID+"/login?returnTo=/alerts", nil))
	authURL, err := url.Parse(login.Header().Get("Location"))
	if err != nil || authURL.Query().Get("state") == "" {
		t.Fatalf("native login failed: %d %v", login.Code, err)
	}
	authorization = authURL.Query()
	const oldSession = "synthetic-existing-local-session"
	GetSessionStore().CreateSession(oldSession, time.Hour, "fixture", "127.0.0.1", "existing-local-user")
	oldSnapshot := GetSessionStore().GetSession(oldSession)
	target := "https://pulse.example.test/api/oidc/" + p.ID + "/callback?code=synthetic-subject&state=" + url.QueryEscape(authorization.Get("state"))
	callbackReq := httptest.NewRequest("GET", target, nil)
	callbackReq.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: oldSession})
	callback, done := httptest.NewRecorder(), make(chan struct{})
	go func() { defer close(done); r.handleSSOOIDCCallback(callback, callbackReq) }()
	awaitOIDCInitialization(t, entered)
	update := func(provider config.SSOProvider, expected int) {
		t.Helper()
		data, err := json.Marshal(provider)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		r.handleSSOProvider(response, httptest.NewRequest("PUT", "/api/security/sso/providers/"+provider.ID, bytes.NewReader(data)))
		if response.Code != expected {
			t.Fatalf("management update: %d, expected %d: %s", response.Code, expected, response.Body.String())
		}
	}
	remove := func() {
		t.Helper()
		response := httptest.NewRecorder()
		r.handleSSOProvider(response, httptest.NewRequest("DELETE", "/api/security/sso/providers/"+p.ID, nil))
		if response.Code != 204 {
			t.Fatalf("management delete: %d", response.Code)
		}
	}
	updated := (&config.SSOConfig{Providers: []config.SSOProvider{p}}).Clone().Providers[0]
	switch action {
	case "unchanged":
	case "presentation":
		updated.Name = "Renamed SSO"
		updated.DisplayName = "Renamed sign-in"
		updated.Priority = 4
		update(updated, 200)
	case "other-provider":
		updated.ID = "independent"
		if apply, save := r.commitSSOConfig(func(cfg *config.SSOConfig) error { return cfg.AddProvider(updated) }); apply != nil || save != nil {
			t.Fatalf("independent save: %v %v", apply, save)
		}
	case "failed-save":
		updated.Enabled = false
		// Use the existing write-failure seam. An invalid persistence root
		// terminates initialization and cannot exercise failed publication.
		r.persistence.SetFileSystem(failingWriteFileSystem{})
		update(updated, 500)
	case "disable":
		updated.Enabled = false
		update(updated, 200)
	case "delete":
		remove()
	case "disable-reenable":
		updated.Enabled = false
		update(updated, 200)
		update(p, 200)
	case "delete-recreate":
		remove()
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		r.handleCreateSSOProvider(response, httptest.NewRequest("POST", "/api/security/sso/providers", bytes.NewReader(data)))
		if response.Code != 201 {
			t.Fatalf("management recreate: %d %s", response.Code, response.Body.String())
		}
	case "email":
		updated.AllowedEmails = []string{"different@example.test"}
		update(updated, 200)
	case "domain":
		updated.AllowedDomains = []string{"different.example.test"}
		update(updated, 200)
	case "group":
		updated.AllowedGroups = []string{"different"}
		update(updated, 200)
	case "role":
		updated.GroupRoleMappings = map[string]string{"ops": internalauth.RoleAuditor}
		update(updated, 200)
	case "claim":
		updated.OIDC.UsernameClaim = "different"
		update(updated, 200)
	case "client":
		updated.OIDC.ClientID = "synthetic-new-client"
		update(updated, 200)
	case "secret":
		updated.OIDC.ClientSecret = "synthetic-new-secret"
		update(updated, 200)
	case "scope":
		updated.OIDC.Scopes = []string{"openid", "email"}
		update(updated, 200)
	case "redirect":
		updated.OIDC.RedirectURL = "https://pulse.example.test/api/oidc/changed/callback"
		update(updated, 200)
	case "policy-revert":
		updated.AllowedEmails = []string{"different@example.test"}
		update(updated, 200)
		update(p, 200)
	case "unknown":
		r.ssoAuthLoadFailed.Store(true)
	default:
		t.Fatalf("unhandled action %s", action)
	}
	unblock()
	awaitOIDCInitialization(t, done)
	result := oidcCallbackPolicyResult{Action: action, Barrier: barrier, CallbackTarget: callback.Header().Get("Location"), ExistingSessionPreserved: reflect.DeepEqual(oldSnapshot, GetSessionStore().GetSession(oldSession)), RoleWrites: recorder.roleWrites, IdentityWrites: recorder.identityWrites, TokenCalls: tokenCalls.Load()}
	for _, cookie := range callback.Result().Cookies() {
		if cookie.Name != sessionCookieName(true) {
			continue
		}
		session := GetSessionStore().GetSession(cookie.Value)
		principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, p.ID, "synthetic-subject")
		if err != nil {
			t.Fatal(err)
		}
		result.SessionVerified = cookie.HttpOnly && cookie.Secure && session != nil && session.Username == principal && session.DisplayUsername == "Synthetic operator" && session.OIDCIssuer == idp.URL && session.OIDCClientID == p.OIDC.ClientID && GetSessionStore().ValidateSession(cookie.Value)
	}
	result.ReplayTarget = doOIDCCallback(t, r, target).Header().Get("Location")
	if tokenCalls.Load() != result.TokenCalls {
		t.Error("replay performed another token exchange")
	}
	return result
}

func TestOIDCCallbackPolicyWithdrawal(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, barrier := range []string{"token", "jwks"} {
		for _, action := range []string{"unchanged", "presentation", "other-provider", "failed-save", "disable", "delete", "disable-reenable", "delete-recreate", "email", "domain", "group", "role", "claim", "client", "secret", "scope", "redirect", "policy-revert", "unknown"} {
			t.Run(barrier+"/"+action, func(t *testing.T) {
				result := runOIDCCallbackPolicy(t, key, barrier, action)
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("PULSE_CALLBACK_POLICY %s", data)
				allowed := action == "unchanged" || action == "presentation" || action == "other-provider" || action == "failed-save"
				if allowed {
					if result.CallbackTarget != "/alerts?oidc=success" || !result.SessionVerified || result.ExistingSessionPreserved || result.RoleWrites != 1 || result.IdentityWrites != 1 {
						t.Errorf("unchanged saved policy lost native login: %+v", result)
					}
				} else {
					if result.CallbackTarget != "/alerts?oidc=error&oidc_error=invalid_state" || result.SessionVerified || !result.ExistingSessionPreserved || result.RoleWrites != 0 || result.IdentityWrites != 0 {
						t.Errorf("obsolete callback wrote roles, identity or a session after withdrawal: %+v", result)
					}
				}
				if result.TokenCalls != 1 {
					t.Errorf("unexpected token exchanges: %+v", result)
				}
				if parsed, err := url.Parse(result.ReplayTarget); err != nil || parsed.Query().Get("oidc") == "success" {
					t.Errorf("replay succeeded: %+v", result)
				}
			})
		}
	}
}
