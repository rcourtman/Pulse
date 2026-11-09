package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type samlCallbackPolicyResult struct {
	Action                   string `json:"action"`
	CallbackTarget           string `json:"callbackTarget"`
	ReplayTarget             string `json:"replayTarget"`
	SessionVerified          bool   `json:"sessionVerified"`
	ExistingSessionPreserved bool   `json:"existingSessionPreserved"`
	CookieWrites             int    `json:"cookieWrites"`
	RoleWrites               int    `json:"roleWrites"`
	IdentityWrites           int    `json:"identityWrites"`
}

// Exercise actual signed SAML, request/browser binding, management publication,
// role/identity persistence and session creation. The body barrier is in HTTP,
// not a production hook or timing guess, and publication must finish before it
// opens. No external IdP, real identities or credentials are involved.
func runSAMLCallbackPolicy(t *testing.T, idp *samlManualCertTestIdP, action string) samlCallbackPolicyResult {
	t.Helper()
	r, _ := setupTestRouter(t)
	r.config.PublicURL = "https://pulse.example.com"
	useSAMLTestAuthStores(t)
	previousManager := internalauth.GetManager()
	manager, err := internalauth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	recorder := &oidcCallbackPolicyRecorder{Manager: manager}
	internalauth.SetManager(recorder)
	t.Cleanup(func() { internalauth.SetManager(previousManager) })
	p := config.SSOProvider{
		ID: samlACSTestProvider, Name: "Synthetic SAML", Type: config.SSOProviderTypeSAML, Enabled: true,
		SAML:              &config.SAMLProviderConfig{IDPMetadataXML: idp.metadataXML(), UsernameAttr: "name", EmailAttr: "mail", GroupsAttr: "groups"},
		GroupRoleMappings: map[string]string{"ops": internalauth.RoleViewer},
	}
	if err := r.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := r.saveSSOConfig(); err != nil {
		t.Fatal(err)
	}
	if err := r.InitializeSAMLProviders(t.Context()); err != nil {
		t.Fatal(err)
	}
	service := r.samlManager.GetService(p.ID)
	if service == nil {
		t.Fatal("SAML service missing")
	}
	login := httptest.NewRecorder()
	r.handleSAMLLogin(login, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login?returnTo=/alerts", nil))
	if login.Code != http.StatusFound {
		t.Fatalf("native login: %d %s", login.Code, login.Body.String())
	}
	requestID, loginToken := samlTestAuthnRequestID(t, login.Header().Get("Location")), ""
	for _, cookie := range login.Result().Cookies() {
		if cookie.Name == cookieNameSAMLLoginSecure && cookie.Secure && cookie.HttpOnly {
			loginToken = cookie.Value
		}
	}
	if loginToken == "" {
		t.Fatal("native HTTPS login wrote no secure browser-binding cookie")
	}
	response := idp.signedResponseXML(t, service, requestID, "synthetic-subject", func(a *saml.Assertion) {
		a.AttributeStatements = []saml.AttributeStatement{{Attributes: []saml.Attribute{
			{Name: "name", Values: []saml.AttributeValue{{Value: "Synthetic operator"}}},
			{Name: "mail", Values: []saml.AttributeValue{{Value: "operator@example.test"}}},
			{Name: "groups", Values: []saml.AttributeValue{{Value: "ops"}}},
		}}}
	})
	form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(response)}, "RelayState": {"/settings"}}
	const oldSession = "synthetic-existing-local-session"
	GetSessionStore().CreateSession(oldSession, time.Hour, "fixture", "127.0.0.1", "existing-local-user")
	oldSnapshot := GetSessionStore().GetSession(oldSession)
	body := &ssoUpdateHeldBody{Reader: strings.NewReader(form.Encode()), entered: make(chan struct{}), release: make(chan struct{})}
	req := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/acs", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: cookieNameSAMLLoginSecure, Value: loginToken})
	req.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: oldSession})
	callback, done := httptest.NewRecorder(), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(body.release) }) }
	t.Cleanup(unblock)
	go func() { defer close(done); r.handleSAMLACS(callback, req) }()
	awaitOIDCInitialization(t, body.entered)
	update := func(provider config.SSOProvider, expected int) {
		t.Helper()
		data, err := json.Marshal(provider)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		r.handleSSOProvider(rec, httptest.NewRequest("PUT", "/api/security/sso/providers/"+p.ID, bytes.NewReader(data)))
		if rec.Code != expected {
			t.Fatalf("management update: %d expected %d: %s", rec.Code, expected, rec.Body.String())
		}
	}
	remove := func() {
		t.Helper()
		rec := httptest.NewRecorder()
		r.handleSSOProvider(rec, httptest.NewRequest("DELETE", "/api/security/sso/providers/"+p.ID, nil))
		if rec.Code != 204 {
			t.Fatalf("management delete: %d %s", rec.Code, rec.Body.String())
		}
	}
	updated := (&config.SSOConfig{Providers: []config.SSOProvider{p}}).Clone().Providers[0]
	switch action {
	case "unchanged":
	case "presentation":
		updated.Name, updated.DisplayName, updated.Priority = "Renamed SAML", "Renamed sign-in", 4
		update(updated, 200)
	case "other-provider":
		updated.ID = "independent"
		if apply, save := r.commitSSOConfig(func(c *config.SSOConfig) error { return c.AddProvider(updated) }); apply != nil || save != nil {
			t.Fatalf("independent save: %v %v", apply, save)
		}
	case "failed-save":
		updated.Enabled = false
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
		data, _ := json.Marshal(p)
		rec := httptest.NewRecorder()
		r.handleCreateSSOProvider(rec, httptest.NewRequest("POST", "/api/security/sso/providers", bytes.NewReader(data)))
		if rec.Code != 201 {
			t.Fatalf("management recreate: %d %s", rec.Code, rec.Body.String())
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
		updated.SAML.UsernameAttr = "different"
		update(updated, 200)
	case "trust":
		// A different valid IdP, not an invalid certificate which save rejects.
		updated.SAML.IDPMetadataXML = newSAMLManualCertTestIdP(t).metadataXML()
		update(updated, 200)
	case "idp-initiated":
		updated.SAML.AllowIDPInitiated = true
		update(updated, 200)
	case "acs-path":
		updated.SAML.SPACSPath = "/api/saml/okta/acs/"
		update(updated, 200)
	case "type":
		updated.Type, updated.SAML = config.SSOProviderTypeOIDC, nil
		updated.OIDC = &config.OIDCProviderConfig{IssuerURL: "https://idp.example.test", ClientID: "synthetic"}
		// Avoid discovery for this snapshot-owner path. Persistence/publication
		// is the same fence used by private snapshot writers.
		if apply, save := r.commitSSOConfig(func(c *config.SSOConfig) error { return c.UpdateProvider(updated) }); apply != nil || save != nil {
			t.Fatalf("type save: %v %v", apply, save)
		}
	case "policy-revert":
		updated.AllowedEmails = []string{"different@example.test"}
		update(updated, 200)
		update(p, 200)
	case "snapshot":
		updated.Enabled = false
		if apply, save := r.commitSSOConfig(func(c *config.SSOConfig) error { return c.UpdateProvider(updated) }); apply != nil || save != nil {
			t.Fatalf("snapshot save: %v %v", apply, save)
		}
	case "unknown":
		r.ssoAuthLoadFailed.Store(true)
	default:
		t.Fatalf("unhandled action %s", action)
	}
	unblock()
	awaitOIDCInitialization(t, done)
	result := samlCallbackPolicyResult{Action: action, CallbackTarget: callback.Header().Get("Location"), ExistingSessionPreserved: reflect.DeepEqual(oldSnapshot, GetSessionStore().GetSession(oldSession)), CookieWrites: len(callback.Result().Cookies()), RoleWrites: recorder.roleWrites, IdentityWrites: recorder.identityWrites}
	for _, cookie := range callback.Result().Cookies() {
		if cookie.Name != sessionCookieName(true) {
			continue
		}
		session := GetSessionStore().GetSession(cookie.Value)
		principal, err := stableSSOPrincipal(config.SSOProviderTypeSAML, p.ID, "synthetic-subject")
		if err != nil {
			t.Fatal(err)
		}
		result.SessionVerified = cookie.HttpOnly && cookie.Secure && session != nil && session.Username == principal && session.DisplayUsername == "Synthetic operator" && session.SAMLProviderID == p.ID && GetSessionStore().ValidateSession(cookie.Value)
	}
	replayReq := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/acs", strings.NewReader(form.Encode()))
	replayReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	replayReq.AddCookie(&http.Cookie{Name: cookieNameSAMLLoginSecure, Value: loginToken})
	replay := httptest.NewRecorder()
	r.handleSAMLACS(replay, replayReq)
	result.ReplayTarget = replay.Header().Get("Location")
	for _, c := range replay.Result().Cookies() {
		if action != "idp-initiated" && (c.Name == cookieNameSession || c.Name == cookieNameSessionSecure) {
			t.Error("replay wrote a session cookie")
		}
	}
	return result
}

func TestSAMLCallbackPolicyWithdrawal(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	for _, action := range []string{"unchanged", "presentation", "other-provider", "failed-save", "disable", "delete", "disable-reenable", "delete-recreate", "email", "domain", "group", "role", "claim", "trust", "idp-initiated", "acs-path", "type", "policy-revert", "snapshot", "unknown"} {
		t.Run(action, func(t *testing.T) {
			result := runSAMLCallbackPolicy(t, idp, action)
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("PULSE_SAML_CALLBACK_POLICY %s", data)
			allowed := action == "unchanged" || action == "presentation" || action == "other-provider" || action == "failed-save"
			if allowed {
				if result.CallbackTarget != "/alerts?saml=success" || !result.SessionVerified || result.ExistingSessionPreserved || result.RoleWrites != 1 || result.IdentityWrites != 1 {
					t.Errorf("valid saved policy lost native SAML login: %+v", result)
				}
			} else {
				if result.CallbackTarget != "/alerts?saml=error&saml_error=invalid_state" || result.SessionVerified || !result.ExistingSessionPreserved || result.CookieWrites != 0 || result.RoleWrites != 0 || result.IdentityWrites != 0 {
					t.Errorf("superseded SAML callback changed roles, identity or session: %+v", result)
				}
			}
			// The opt-in change now explicitly permits unsolicited signed responses.
			// Preserve that current policy, rather than inventing a replay gate
			// on the deliberately opt-in IdP-initiated path.
			if parsed, err := url.Parse(result.ReplayTarget); err != nil || (parsed.Query().Get("saml") == "success") != (action == "idp-initiated") {
				t.Errorf("replay succeeded: %+v", result)
			}
		})
	}
}
