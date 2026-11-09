package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSAMLCallbackPolicyLocalCommitOrdering(t *testing.T) {
	r, _ := setupTestRouter(t)
	p := config.SSOProvider{ID: "saml-ordering", Type: config.SSOProviderTypeSAML, Enabled: true, SAML: &config.SAMLProviderConfig{}}
	if err := r.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	provider, revision := r.currentSSOCallbackPolicy(p.ID)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var completionErr error
	go func() {
		defer close(done)
		completionErr = r.commitSSOCallback(p.ID, revision, provider, func() error { close(entered); <-release; return nil })
	}()
	awaitOIDCInitialization(t, entered)
	if current := r.currentSSOConfig().GetProvider(p.ID); current == nil || !current.Enabled {
		t.Error("local effects block immutable reads")
	}
	if r.ssoMu.TryLock() {
		r.ssoMu.Unlock()
		t.Error("saved withdrawal can interleave with local effects")
	}
	unblock()
	awaitOIDCInitialization(t, done)
	if completionErr != nil {
		t.Fatal(completionErr)
	}
	apply, save := r.commitSSOConfig(func(c *config.SSOConfig) error { c.GetProvider(p.ID).Enabled = false; return nil })
	if apply != nil || save != nil {
		t.Fatalf("withdrawal: %v %v", apply, save)
	}
	called := false
	if err := r.commitSSOCallback(p.ID, revision, provider, func() error { called = true; return nil }); err != errSSOCallbackPolicyChanged || called {
		t.Fatalf("withdrawn local completion ran: called=%v err=%v", called, err)
	}
}

// The saved snapshot may precede a slow/failed service reinitialization. A
// still-installed verifier with old trust/claims must not stand in for it.
func TestSAMLCallbackRejectsMismatchedService(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	for _, field := range []string{"trust", "claim", "idp-initiated"} {
		t.Run(field, func(t *testing.T) {
			r, service := newSAMLACSTestRouter(t, idp, false)
			requestID, token := startSAMLTestLogin(t, r, "", "/alerts")
			form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, requestID))}}
			apply, save := r.commitSSOConfig(func(c *config.SSOConfig) error {
				p := c.GetProvider(samlACSTestProvider)
				switch field {
				case "trust":
					p.SAML.IDPMetadataXML = newSAMLManualCertTestIdP(t).metadataXML()
				case "claim":
					p.SAML.UsernameAttr = "changed"
				case "idp-initiated":
					p.SAML.AllowIDPInitiated = true
				}
				return nil
			})
			if apply != nil || save != nil {
				t.Fatalf("save: %v %v", apply, save)
			}
			rec := postSAMLTestACS(r, form, token)
			if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?saml=error&saml_error=invalid_state" || len(rec.Result().Cookies()) != 0 {
				t.Fatalf("stale initialized service authorized changed saved policy: %d %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Header().Get("Location"), "invalid_state") {
				t.Fatal("missing retry path")
			}
		})
	}
}
