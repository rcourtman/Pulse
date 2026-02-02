package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func testSAMLProvider(id string, enabled bool) config.SSOProvider {
	return config.SSOProvider{
		ID:      id,
		Name:    "Test SAML",
		Type:    config.SSOProviderTypeSAML,
		Enabled: enabled,
		SAML: &config.SAMLProviderConfig{
			IDPSSOURL:   "https://idp.example.com/sso",
			IDPEntityID: "https://idp.example.com/metadata",
		},
	}
}

func newSAMLRouter(t *testing.T, provider config.SSOProvider) *Router {
	t.Helper()
	return &Router{
		config:      &config.Config{PublicURL: "https://pulse.example.com"},
		samlManager: NewSAMLServiceManager("https://pulse.example.com"),
		ssoConfig: &config.SSOConfig{
			Providers: []config.SSOProvider{provider},
		},
	}
}

func TestHandleSAMLMetadata_Success(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/metadata", nil)
	rr := httptest.NewRecorder()

	router.handleSAMLMetadata(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("expected application/xml, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "EntityDescriptor") {
		t.Fatalf("expected metadata XML, got %q", rr.Body.String())
	}
}

func TestHandleSAMLLogin_SuccessGetAndPost(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))

	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/login", nil)
	rr := httptest.NewRecorder()
	router.handleSAMLLogin(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rr.Code)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "idp.example.com") {
		t.Fatalf("expected redirect to idp, got %q", loc)
	}

	body := bytes.NewBufferString(`{"returnTo":"/dashboard"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/saml/okta/login", body)
	rr = httptest.NewRecorder()
	router.handleSAMLLogin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if authURL := resp["authorizationUrl"]; !strings.Contains(authURL, "idp.example.com") {
		t.Fatalf("expected authorizationUrl, got %q", authURL)
	}
}

func TestHandleSAMLLogin_InitFailure(t *testing.T) {
	router := newSAMLRouter(t, config.SSOProvider{
		ID:      "broken",
		Name:    "Broken",
		Type:    config.SSOProviderTypeSAML,
		Enabled: true,
		SAML:    &config.SAMLProviderConfig{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/saml/broken/login", nil)
	rr := httptest.NewRecorder()
	router.handleSAMLLogin(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["code"] != "saml_init_failed" {
		t.Fatalf("expected saml_init_failed, got %#v", resp["code"])
	}
}

func TestHandleSAMLMetadata_NotInitialized(t *testing.T) {
	router := newSAMLRouter(t, config.SSOProvider{
		ID:      "no-config",
		Name:    "No Config",
		Type:    config.SSOProviderTypeSAML,
		Enabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/saml/no-config/metadata", nil)
	rr := httptest.NewRecorder()
	router.handleSAMLMetadata(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestRefreshSAMLProvider(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))

	// No service yet - should be no-op.
	if err := router.RefreshSAMLProvider(reqContext(t), "okta"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	service, err := NewSAMLService(reqContext(t), "okta", &config.SAMLProviderConfig{
		IDPSSOURL:   "https://idp.example.com/sso",
		IDPEntityID: "https://idp.example.com/metadata",
	}, "https://pulse.example.com")
	if err != nil {
		t.Fatalf("NewSAMLService: %v", err)
	}
	router.samlManager.services["okta"] = service

	if err := router.RefreshSAMLProvider(reqContext(t), "okta"); err == nil {
		t.Fatalf("expected refresh error without metadata url")
	}
}

func reqContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
