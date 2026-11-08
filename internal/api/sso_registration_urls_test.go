package api

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Exercise discovery, the actual authorization URL and callback code exchange,
// rather than comparing two display helpers which could agree on the wrong URL.
func TestSSORegistrationOIDCMatchesAuthorizationAndExchange(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	resetTrustedProxyCIDRsForTests()
	t.Cleanup(resetTrustedProxyCIDRsForTests)
	for _, tc := range []struct {
		name, public, redirect, peer, want string
		forwarded                          bool
	}{
		{name: "explicit legacy redirect", public: "https://configured.example.test", redirect: "https://pulse.example.test/api/oidc/callback?workspace=Ops", want: "https://pulse.example.test/api/oidc/callback?workspace=Ops"},
		{name: "configured public base", public: " https://pulse.example.test/base/ ", want: "https://pulse.example.test/base/api/oidc/setup/callback"},
		{name: "configured query boundary", public: "https://pulse.example.test/base/?source=setup#section", want: "https://pulse.example.test/base/api/oidc/setup/callback"},
		{name: "request TLS", want: "https://pulse.lan:8443/api/oidc/setup/callback"},
		{name: "trusted proxy", peer: "192.0.2.1:1234", forwarded: true, want: "https://proxy.example.test:9443/api/oidc/setup/callback"},
		{name: "untrusted forwarded headers", peer: "192.0.2.2:1234", forwarded: true, want: "https://pulse.lan:8443/api/oidc/setup/callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exchanged := make(chan string, 2)
			idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				base := "http://" + req.Host
				switch req.URL.Path {
				case "/.well-known/openid-configuration":
					_ = json.NewEncoder(w).Encode(map[string]string{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "jwks_uri": base + "/jwks"})
				case "/token":
					if err := req.ParseForm(); err != nil {
						t.Error(err)
					}
					exchanged <- req.Form.Get("redirect_uri")
					fmt.Fprint(w, `{"access_token":"synthetic-access","token_type":"Bearer","expires_in":60}`)
				default:
					http.NotFound(w, req)
				}
			}))
			defer idp.Close()
			p := config.SSOProvider{ID: "setup", Name: "Operations", Type: config.SSOProviderTypeOIDC, Enabled: true, OIDC: &config.OIDCProviderConfig{IssuerURL: idp.URL, ClientID: "pulse", ClientSecret: "synthetic-secret", RedirectURL: tc.redirect, Scopes: []string{"openid"}}}
			r := &Router{config: &config.Config{PublicURL: tc.public}, ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{p}}, oidcManager: NewOIDCServiceManager()}
			request := func(path string) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://pulse.lan:8443"+path, nil)
				if tc.peer != "" {
					req.RemoteAddr = tc.peer
				}
				if tc.forwarded {
					req.Header.Set("X-Forwarded-Host", "proxy.example.test:9443")
					req.Header.Set("X-Forwarded-Proto", "https")
				}
				return req
			}
			list := httptest.NewRecorder()
			r.handleListSSOProviders(list, request("/api/security/sso/providers"))
			var collection SSOProvidersListResponse
			if err := json.Unmarshal(list.Body.Bytes(), &collection); err != nil {
				t.Fatal(err)
			}
			if list.Code != 200 || len(collection.Providers) != 1 {
				t.Fatalf("collection: %d %s", list.Code, list.Body.String())
			}
			if got := collection.Providers[0].OIDCCallbackURL; got != tc.want {
				t.Errorf("setup callback = %q, want %q", got, tc.want)
			}
			detail := httptest.NewRecorder()
			r.handleGetSSOProvider(detail, request("/api/security/sso/providers/setup"), p.ID)
			var full SSOProviderResponse
			if err := json.Unmarshal(detail.Body.Bytes(), &full); err != nil {
				t.Fatal(err)
			}
			if full.OIDCCallbackURL != tc.want {
				t.Errorf("detail callback = %q, want %q", full.OIDCCallbackURL, tc.want)
			}
			login := httptest.NewRecorder()
			r.handleSSOOIDCLogin(login, request("/api/oidc/setup/login?returnTo=/alerts"))
			service := r.oidcManager.GetService(p.ID)
			if service != nil {
				defer service.Stop()
			}
			if login.Code != 302 {
				t.Fatalf("login: %d %s", login.Code, login.Body.String())
			}
			authorization, err := url.Parse(login.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			if got := authorization.Query().Get("redirect_uri"); got != tc.want {
				t.Errorf("authorization redirect_uri = %q, want %q", got, tc.want)
			}
			state := authorization.Query().Get("state")
			if state == "" {
				t.Fatal("login did not retain state")
			}
			callback := httptest.NewRecorder()
			r.handleSSOOIDCCallback(callback, request("/api/oidc/setup/callback?code=synthetic&state="+url.QueryEscape(state)))
			if got := callback.Header().Get("Location"); got != "/alerts?oidc=error&oidc_error=missing_id_token" {
				t.Fatalf("callback did not reach token exchange: %q", got)
			}
			select {
			case got := <-exchanged:
				if got != tc.want {
					t.Errorf("exchange redirect_uri = %q, want %q", got, tc.want)
				}
			default:
				t.Fatal("no code exchange")
			}
			again := httptest.NewRecorder()
			r.handleSSOOIDCCallback(again, request("/api/oidc/setup/callback?code=synthetic&state="+url.QueryEscape(state)))
			select {
			case <-exchanged:
				t.Fatal("replayed callback reached token exchange")
			default:
			}
		})
	}
}

func TestSSORegistrationOIDCUnknownOriginDoesNotStartFlow(t *testing.T) {
	for _, public := range []string{"", "https://user:password@configured.example.test"} {
		t.Run(public, func(t *testing.T) {
			p := newRecoveryTestProvider("setup", true, "")
			r := newRecoveryTestRouter(t, []config.SSOProvider{p}, nil)
			r.config.PublicURL = public
			req := httptest.NewRequest(http.MethodPost, "http://pulse.lan/api/oidc/setup/login", nil)
			req.Host = "bad host"
			response := providerToResponse(&p, public, req)
			if response.OIDCCallbackURL != "" || response.OIDCLoginURL != "" {
				t.Fatalf("unknown authority advertised URLs: %+v", response)
			}
			rec := httptest.NewRecorder()
			r.handleSSOOIDCLogin(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"invalid_config"`) {
				t.Fatalf("unknown callback admitted: %d %s", rec.Code, rec.Body.String())
			}
			if r.oidcManager.GetService(p.ID) != nil {
				t.Fatal("initialized a service without a callback")
			}
		})
	}
	p := newRecoveryTestProvider("setup", true, "https://pulse.example.test/api/oidc/callback")
	if got := providerToResponse(&p, "", nil).OIDCCallbackURL; got != p.OIDC.RedirectURL {
		t.Fatalf("explicit callback needs no request origin: %q", got)
	}
}

func registrationAuthnRequest(t *testing.T, redirect string) saml.AuthnRequest {
	t.Helper()
	target, err := url.Parse(redirect)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := base64.StdEncoding.DecodeString(target.Query().Get("SAMLRequest"))
	if err != nil {
		t.Fatal(err)
	}
	rd := flate.NewReader(bytes.NewReader(compressed))
	defer rd.Close()
	raw, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	var authn saml.AuthnRequest
	if err := xml.Unmarshal(raw, &authn); err != nil {
		t.Fatal(err)
	}
	return authn
}

func TestSSORegistrationSAMLMatchesMetadataAndAuthnRequest(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	resetTrustedProxyCIDRsForTests()
	t.Cleanup(resetTrustedProxyCIDRsForTests)
	for _, tc := range []struct {
		name, public, peer, base string
		forwarded, custom        bool
	}{
		{name: "uncaptured request origin", base: "https://pulse.lan:8443"},
		{name: "configured base", public: "https://pulse.example.test/base/", base: "https://pulse.example.test/base"},
		{name: "configured query boundary", public: "https://pulse.example.test/base/?source=setup#section", base: "https://pulse.example.test/base"},
		{name: "advanced endpoint paths", public: "https://pulse.example.test", base: "https://pulse.example.test", custom: true},
		{name: "trusted proxy", peer: "192.0.2.1:1234", forwarded: true, base: "https://proxy.example.test:9443"},
		{name: "untrusted forwarded headers", peer: "192.0.2.2:1234", forwarded: true, base: "https://pulse.lan:8443"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testSAMLProvider("setup", true)
			if tc.custom {
				p.SAML.SPEntityID = "urn:pulse:operations"
				p.SAML.SPACSPath = "/api/saml/setup/acs/"
				p.SAML.SPMetadataPath = "/api/saml/setup/metadata/"
			}
			r := &Router{config: &config.Config{PublicURL: tc.public}, ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{p}}, samlManager: NewSAMLServiceManager("")}
			request := func(path string) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://pulse.lan:8443"+path, nil)
				if tc.peer != "" {
					req.RemoteAddr = tc.peer
				}
				if tc.forwarded {
					req.Header.Set("X-Forwarded-Host", "proxy.example.test:9443")
					req.Header.Set("X-Forwarded-Proto", "https")
				}
				return req
			}
			list := httptest.NewRecorder()
			r.handleListSSOProviders(list, request("/api/security/sso/providers"))
			var collection SSOProvidersListResponse
			if err := json.Unmarshal(list.Body.Bytes(), &collection); err != nil {
				t.Fatal(err)
			}
			if list.Code != 200 || len(collection.Providers) != 1 {
				t.Fatalf("collection: %d %s", list.Code, list.Body.String())
			}
			display := collection.Providers[0]
			wantEntity := tc.base + "/saml/setup"
			wantACS := tc.base + "/api/saml/setup/acs"
			wantMetadata := tc.base + "/api/saml/setup/metadata"
			if tc.custom {
				wantEntity = p.SAML.SPEntityID
				wantACS += "/"
				wantMetadata += "/"
			}
			if display.SAMLSPEntityID != wantEntity || display.SAMLACSURL != wantACS || display.SAMLMetadataURL != wantMetadata {
				t.Errorf("setup URLs = %q %q %q, want %q %q %q", display.SAMLSPEntityID, display.SAMLACSURL, display.SAMLMetadataURL, wantEntity, wantACS, wantMetadata)
			}
			meta := httptest.NewRecorder()
			r.handleSAMLMetadata(meta, request("/api/saml/setup/metadata"))
			var descriptor saml.EntityDescriptor
			if err := xml.Unmarshal(meta.Body.Bytes(), &descriptor); err != nil {
				t.Fatal(err)
			}
			if meta.Code != 200 || descriptor.EntityID != wantEntity || len(descriptor.SPSSODescriptors) != 1 {
				t.Fatalf("metadata: %d %s", meta.Code, meta.Body.String())
			}
			for _, acs := range descriptor.SPSSODescriptors[0].AssertionConsumerServices {
				if acs.Location != wantACS {
					t.Errorf("metadata ACS=%q, want %q", acs.Location, wantACS)
				}
			}
			login := httptest.NewRecorder()
			r.handleSAMLLogin(login, request("/api/saml/setup/login"))
			if login.Code != 302 {
				t.Fatalf("login: %d %s", login.Code, login.Body.String())
			}
			authn := registrationAuthnRequest(t, login.Header().Get("Location"))
			if authn.AssertionConsumerServiceURL != wantACS || authn.Issuer == nil || authn.Issuer.Value != wantEntity {
				t.Errorf("AuthnRequest: ACS=%q issuer=%+v", authn.AssertionConsumerServiceURL, authn.Issuer)
			}
			if tc.public == "" {
				if r.config.PublicURL != "" || r.samlManager.GetPublicURL() != "" || r.samlManager.GetService(p.ID).GetSPEntityID() != "/saml/setup" {
					t.Fatal("request-derived origin rebound shared/configured authority")
				}
			}
		})
	}
}

func TestSSORegistrationSAMLConcurrentOriginsStayRequestLocal(t *testing.T) {
	p := testSAMLProvider("setup", true)
	r := &Router{config: &config.Config{}, ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{p}}, samlManager: NewSAMLServiceManager("")}
	if err := r.samlManager.InitializeProvider(context.Background(), p.ID, p.SAML); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, host := range []string{"one.example.test", "two.example.test"} {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(host string) {
				defer wg.Done()
				rec := httptest.NewRecorder()
				r.handleSAMLMetadata(rec, httptest.NewRequest(http.MethodGet, "https://"+host+"/api/saml/setup/metadata", nil))
				if rec.Code != 200 || !strings.Contains(rec.Body.String(), `entityID="https://`+host+`/saml/setup"`) {
					t.Errorf("%s metadata=%d %s", host, rec.Code, rec.Body.String())
				}
			}(host)
		}
	}
	wg.Wait()
	service := r.samlManager.GetService(p.ID)
	if service.GetSPEntityID() != "/saml/setup" || r.samlManager.GetPublicURL() != "" || r.config.PublicURL != "" {
		t.Fatal("concurrent requests changed shared authority")
	}
	req := httptest.NewRequest(http.MethodGet, "https://one.example.test/api/saml/setup/metadata", nil)
	req.Host = "bad host"
	rec := httptest.NewRecorder()
	r.handleSAMLMetadata(rec, req)
	if rec.Code == 200 {
		t.Fatal("invalid origin produced relative/guessed metadata")
	}
}

func TestSSORegistrationSAMLNoBaseKeepsSignedResponseBinding(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	cfg := &config.SAMLProviderConfig{IDPMetadataXML: idp.metadataXML()}
	service, err := NewSAMLService(context.Background(), "setup", cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	signingService, err := NewSAMLService(context.Background(), "setup", cfg, "https://pulse.example.com")
	if err != nil {
		t.Fatal(err)
	}
	r := &Router{config: &config.Config{}, samlManager: NewSAMLServiceManager(""), ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{{ID: "setup", Name: "Operations", Type: config.SSOProviderTypeSAML, Enabled: true, SAML: cfg}}}}
	r.samlManager.services["setup"] = service
	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://pulse.example.com/api/saml/setup/login?returnTo=/alerts", nil)
	r.handleSAMLLogin(login, req)
	if login.Code != 302 {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	authn := registrationAuthnRequest(t, login.Header().Get("Location"))
	if authn.AssertionConsumerServiceURL != "https://pulse.example.com/api/saml/setup/acs" {
		t.Fatalf("relative/wrong ACS: %q", authn.AssertionConsumerServiceURL)
	}
	var token string
	for _, c := range login.Result().Cookies() {
		if strings.Contains(c.Name, "saml_login") {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("missing browser login binding")
	}
	response := idp.responseXML(t, signingService, authn.ID)
	post := func(origin string) *http.Request {
		form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(response)}, "RelayState": {"/untrusted"}}
		req := httptest.NewRequest(http.MethodPost, origin+"/api/saml/setup/acs", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req
	}
	if _, _, err := service.ProcessResponse(post("https://pulse.example.com"), sessionHash("another-browser")); err == nil {
		t.Fatal("another browser accepted")
	}
	if _, _, err := service.ProcessResponse(post("https://other.example.com"), sessionHash(token)); err == nil {
		t.Fatal("another origin accepted")
	}
	result, returnTo, err := service.ProcessResponse(post("https://pulse.example.com"), sessionHash(token))
	if err != nil {
		t.Fatal(err)
	}
	if result.NameID != "alice" || returnTo != "/alerts" {
		t.Fatalf("result=%+v returnTo=%q", result, returnTo)
	}
	if _, _, err := service.ProcessResponse(post("https://pulse.example.com"), sessionHash(token)); err == nil {
		t.Fatal("replayed signed response accepted")
	}
	if service.GetSPEntityID() != "/saml/setup" {
		t.Fatal("response processing changed shared SP")
	}
}

func TestSSORegistrationSAMLNoBaseKeepsSignedLogoutBinding(t *testing.T) {
	for _, binding := range []string{"redirect", "post"} {
		t.Run(binding, func(t *testing.T) {
			r, idp := newSAMLSLOTestRouter(t)
			service := r.samlManager.GetService("okta")
			if err := service.SetBaseURL(""); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "https://pulse.example.com/api/saml/okta/logout", nil)
			redirect, err := service.makeLogoutRequest(req, "alice", "session-index", "bound-session")
			if err != nil {
				t.Fatal(err)
			}
			target, err := url.Parse(redirect)
			if err != nil {
				t.Fatal(err)
			}
			var logout saml.LogoutRequest
			if err := xml.Unmarshal(inflateSAMLTestPayload(t, target.Query().Get("SAMLRequest")), &logout); err != nil {
				t.Fatal(err)
			}
			if logout.Issuer == nil || logout.Issuer.Value != "https://pulse.example.com/saml/okta" {
				t.Fatalf("logout issuer=%+v", logout.Issuer)
			}
			query := idp.redirectQuery(t, logout.ID)
			form := idp.postForm(t, logout.ID)
			response := func(origin string) *http.Request {
				if binding == "redirect" {
					return httptest.NewRequest(http.MethodGet, origin+"/api/saml/okta/slo?"+query, nil)
				}
				req := httptest.NewRequest(http.MethodPost, origin+"/api/saml/okta/slo", strings.NewReader(form))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			}
			if err := service.ValidateLogoutResponse(response("https://other.example.com"), "bound-session"); err == nil {
				t.Fatal("another origin accepted a signed logout")
			}
			if err := service.ValidateLogoutResponse(response("https://pulse.example.com"), "bound-session"); err != nil {
				t.Fatal(err)
			}
			if err := service.ValidateLogoutResponse(response("https://pulse.example.com"), "bound-session"); err == nil {
				t.Fatal("replayed signed logout accepted")
			}
			if service.GetSPEntityID() != "/saml/okta" {
				t.Fatal("logout validation rebound shared authority")
			}
		})
	}
}
