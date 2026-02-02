package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSAMLServiceManager_RemoveProvider(t *testing.T) {
	manager := NewSAMLServiceManager("https://pulse.example.com")
	manager.services["okta"] = &SAMLService{}

	manager.RemoveProvider("okta")

	if svc := manager.GetService("okta"); svc != nil {
		t.Fatalf("expected provider to be removed")
	}
}

func TestHandleSAMLACS_MethodNotAllowed(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	if router.samlManager == nil {
		router.samlManager = NewSAMLServiceManager("https://pulse.example.com")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/acs", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLACS(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleSAMLACS_InvalidProviderID(t *testing.T) {
	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	req := httptest.NewRequest(http.MethodPost, "/api/saml/invalid$id/acs", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLACS(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "saml_error=invalid_provider") {
		t.Fatalf("expected invalid_provider redirect, got %q", loc)
	}
}

func TestHandleSAMLACS_ProviderNotFound(t *testing.T) {
	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/acs", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLACS(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "saml_error=provider_not_found") {
		t.Fatalf("expected provider_not_found redirect, got %q", loc)
	}
}

func TestHandleSAMLACS_ServiceNotInitialized(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/acs", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLACS(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "saml_error=provider_not_initialized") {
		t.Fatalf("expected provider_not_initialized redirect, got %q", loc)
	}
}

func TestHandleSAMLLogout_Fallback(t *testing.T) {
	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/logout", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLLogout(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestEstablishSAMLSession(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	samlInfo := &SAMLSessionInfo{ProviderID: "okta", NameID: "user", SessionIndex: "sess-1"}
	if err := router.establishSAMLSession(rec, req, "admin", samlInfo); err != nil {
		t.Fatalf("establishSAMLSession error: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected session and csrf cookies, got %d", len(cookies))
	}
}

func TestRedirectSAMLError(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.redirectSAMLError(rec, req, "/dashboard", "session_failed")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "saml=error") || !strings.Contains(loc, "saml_error=session_failed") {
		t.Fatalf("unexpected redirect location %q", loc)
	}
}

func TestInitializeSAMLProviders(t *testing.T) {
	provider := testSAMLProvider("okta", true)
	provider.SAML = &config.SAMLProviderConfig{
		IDPSSOURL:   "https://idp.example.com/sso",
		IDPEntityID: "https://idp.example.com/metadata",
	}

	router := newSAMLRouter(t, provider)
	if router.samlManager == nil {
		router.samlManager = NewSAMLServiceManager("https://pulse.example.com")
	}

	if err := router.InitializeSAMLProviders(reqContext(t)); err != nil {
		t.Fatalf("InitializeSAMLProviders error: %v", err)
	}
	if svc := router.samlManager.GetService("okta"); svc == nil {
		t.Fatalf("expected SAML service to be initialized")
	}
}
