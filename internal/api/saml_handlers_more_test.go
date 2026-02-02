package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newTestSAMLService(t *testing.T, providerID string, metadataXML string) *SAMLService {
	t.Helper()
	service, err := NewSAMLService(context.Background(), providerID, &config.SAMLProviderConfig{
		IDPMetadataXML: metadataXML,
	}, "https://pulse.example.com")
	if err != nil {
		t.Fatalf("NewSAMLService: %v", err)
	}
	return service
}

func TestHandleSAMLACS_ProcessResponseError(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	router.samlManager.services["okta"] = &SAMLService{}

	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/acs", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLACS(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "saml_error=saml_validation_failed") {
		t.Fatalf("expected validation failed redirect, got %q", loc)
	}
}

func TestHandleSAMLMetadata_InvalidMethod(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/metadata", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLMetadata(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleSAMLMetadata_InvalidProviderID(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	req := httptest.NewRequest(http.MethodGet, "/api/saml/invalid$id/metadata", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLMetadata(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetSAMLSessionInfo_NoCookie(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if info := router.getSAMLSessionInfo(req); info != nil {
		t.Fatalf("expected nil session info without cookie")
	}
}

func TestGetSAMLSessionInfo_ReturnsInfo(t *testing.T) {
	InitSessionStore(t.TempDir())

	token := generateSessionToken()
	GetSessionStore().CreateSAMLSession(token, time.Hour, "agent", "127.0.0.1", "user", &SAMLTokenInfo{
		ProviderID:   "okta",
		NameID:       "name-id",
		SessionIndex: "sess-1",
	})

	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})

	info := router.getSAMLSessionInfo(req)
	if info == nil {
		t.Fatalf("expected session info")
	}
	if info.ProviderID != "okta" || info.NameID != "name-id" || info.SessionIndex != "sess-1" {
		t.Fatalf("unexpected session info: %#v", info)
	}
}

func TestClearSession(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.clearSession(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "session" {
		t.Fatalf("expected session cookie name, got %q", cookie.Name)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
	if !cookie.HttpOnly {
		t.Fatalf("expected HttpOnly cookie")
	}
}

func TestHandleSAMLSLO_Redirects(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/slo", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLSLO(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/?logout=success" {
		t.Fatalf("unexpected redirect location %q", loc)
	}
}

func TestHandleSAMLLogout_SLOUnavailable(t *testing.T) {
	InitSessionStore(t.TempDir())

	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	metadataXML := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`
	router.samlManager.services["okta"] = newTestSAMLService(t, "okta", metadataXML)

	token := generateSessionToken()
	GetSessionStore().CreateSAMLSession(token, time.Hour, "agent", "127.0.0.1", "user", &SAMLTokenInfo{
		ProviderID:   "okta",
		NameID:       "name-id",
		SessionIndex: "sess-1",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/logout", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	rec := httptest.NewRecorder()

	router.handleSAMLLogout(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/?logout=success" {
		t.Fatalf("unexpected redirect location %q", loc)
	}
}

func TestHandleSAMLLogout_SLOSuccess(t *testing.T) {
	InitSessionStore(t.TempDir())

	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	metadataXML := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
    <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/slo"/>
  </IDPSSODescriptor>
</EntityDescriptor>`
	router.samlManager.services["okta"] = newTestSAMLService(t, "okta", metadataXML)

	token := generateSessionToken()
	GetSessionStore().CreateSAMLSession(token, time.Hour, "agent", "127.0.0.1", "user", &SAMLTokenInfo{
		ProviderID:   "okta",
		NameID:       "name-id",
		SessionIndex: "sess-1",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/logout", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	rec := httptest.NewRecorder()

	router.handleSAMLLogout(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "https://idp.example.com/slo") || !strings.Contains(loc, "SAMLRequest=") {
		t.Fatalf("unexpected SLO redirect location %q", loc)
	}
}

func TestExtractSAMLProviderID(t *testing.T) {
	if got := extractSAMLProviderID("/api/saml/okta/login", "login"); got != "okta" {
		t.Fatalf("expected okta, got %q", got)
	}
	if got := extractSAMLProviderID("/api/saml/okta/logout", "login"); got != "" {
		t.Fatalf("expected empty provider, got %q", got)
	}
	if got := extractSAMLProviderID("/api/saml/okta/login/extra", "login"); got != "okta" {
		t.Fatalf("expected okta for extra path, got %q", got)
	}
	if got := extractSAMLProviderID("/api/other/okta/login", "login"); got != "" {
		t.Fatalf("expected empty provider for non-saml path, got %q", got)
	}
}
