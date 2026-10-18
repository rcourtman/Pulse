package api

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
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
	// 3 cookies: pulse_session (unprefixed), __Host-pulse_session (prefixed), pulse_csrf
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies (pulse_session + __Host-pulse_session + pulse_csrf), got %d", len(cookies))
	}
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "pulse_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected pulse_session cookie to be cleared")
	}
	if sessionCookie.MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", sessionCookie.MaxAge)
	}
	if !sessionCookie.HttpOnly {
		t.Fatalf("expected HttpOnly cookie")
	}
}

// TestClearSession_DeletesServerSideCSRFToken regresses the missing CSRF-
// cleanup-on-logout symmetry: clearSession invalidated the session-store
// record and zeroed the cookies, but never deleted the CSRF entry bound to
// the session. The entry stuck around in the in-memory + on-disk CSRF store
// for up to 4 hours after logout — not exploitable (the session was gone),
// but the asymmetry was a real bug that accumulated dead records, and
// password-change / re-login paths already delete CSRF state on cleanup.
func TestClearSession_DeletesServerSideCSRFToken(t *testing.T) {
	dataDir := t.TempDir()
	InitSessionStore(dataDir)
	InitCSRFStore(dataDir)

	sessionToken := "test-clear-session-csrf-token-1234567890"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "test-ua", "127.0.0.1", "testuser")

	csrfToken := GetCSRFStore().GenerateCSRFToken(sessionToken)
	if csrfToken == "" {
		t.Fatal("expected non-empty CSRF token from GenerateCSRFToken")
	}
	if !GetCSRFStore().ValidateCSRFToken(sessionToken, csrfToken) {
		t.Fatal("CSRF token should validate immediately after issuance")
	}

	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()

	router.clearSession(rec, req)

	if GetCSRFStore().ValidateCSRFToken(sessionToken, csrfToken) {
		t.Fatal("CSRF token must be invalidated when clearSession runs against the bound session")
	}
}

// TestHandleSAMLSLO_RejectsEmptyPayload regresses the unauthenticated force-
// logout DoS that previously existed on /api/saml/{id}/slo. A bare GET or
// POST with no SAMLResponse/SAMLRequest payload must not redirect to the
// "logout=success" page and must not touch session state.
func TestHandleSAMLSLO_RejectsEmptyPayload(t *testing.T) {
	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	metadataXML := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`
	router.samlManager.services["okta"] = newTestSAMLService(t, "okta", metadataXML)

	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/slo", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLSLO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty SLO payload, got %d body=%q", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("must not redirect on empty SLO payload, got %q", loc)
	}
}

// TestHandleSAMLSLO_RejectsInvalidLogoutResponse confirms a SAMLResponse
// parameter with a value the IdP cert cannot validate (gibberish) is
// rejected with 403, again without clearing session.
func TestHandleSAMLSLO_RejectsInvalidLogoutResponse(t *testing.T) {
	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	metadataXML := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`
	router.samlManager.services["okta"] = newTestSAMLService(t, "okta", metadataXML)

	// A garbage SAMLResponse value: not real base64-encoded XML, no signature.
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/saml/okta/slo?SAMLResponse=not-a-real-response",
		nil,
	)
	rec := httptest.NewRecorder()

	router.handleSAMLSLO(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on invalid LogoutResponse, got %d body=%q", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("must not redirect on invalid LogoutResponse, got %q", loc)
	}
}

// TestHandleSAMLSLO_RejectsIDPInitiatedLogoutRequest documents the explicit
// rejection of IdP-initiated SLO until proper request-binding is wired up.
// Today an unvalidated LogoutRequest must not be treated as a force-logout
// signal.
func TestHandleSAMLSLO_RejectsIDPInitiatedLogoutRequest(t *testing.T) {
	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	metadataXML := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`
	router.samlManager.services["okta"] = newTestSAMLService(t, "okta", metadataXML)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/saml/okta/slo?SAMLRequest=anything",
		nil,
	)
	rec := httptest.NewRecorder()

	router.handleSAMLSLO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on IdP-initiated SLO, got %d body=%q", rec.Code, rec.Body.String())
	}
}

const samlSLOTestDestination = "https://pulse.example.com/api/saml/okta/slo"

// samlSLOTestIdP signs LogoutResponses with a key the SP under test trusts
// through its IdP metadata, the way a real IdP answers a LogoutRequest.
type samlSLOTestIdP struct {
	signer *saml.ServiceProvider
}

// newSAMLSLOTestRouter returns a router whose "okta" provider supports SLO and
// trusts the returned IdP's signing key, with fresh session and CSRF stores.
func newSAMLSLOTestRouter(t *testing.T) (*Router, *samlSLOTestIdP) {
	t.Helper()
	dataPath := t.TempDir()
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

	certPEM, _, key := generateTestCert(t)
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("decode IdP certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse IdP certificate: %v", err)
	}
	metadataXML := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data><X509Certificate>` + base64.StdEncoding.EncodeToString(block.Bytes) + `</X509Certificate></X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
    <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/slo"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

	router := &Router{samlManager: NewSAMLServiceManager("https://pulse.example.com")}
	router.samlManager.services["okta"] = newTestSAMLService(t, "okta", metadataXML)
	return router, &samlSLOTestIdP{signer: &saml.ServiceProvider{
		EntityID:        "idp",
		Key:             key,
		Certificate:     cert,
		SignatureMethod: "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256",
	}}
}

// redirectQuery returns the HTTP-Redirect binding query carrying a signed,
// successful LogoutResponse that answers requestID.
func (idp *samlSLOTestIdP) redirectQuery(t *testing.T, requestID string) string {
	t.Helper()
	resp, err := idp.signer.MakeLogoutResponse(samlSLOTestDestination, requestID)
	if err != nil {
		t.Fatalf("sign LogoutResponse: %v", err)
	}
	return resp.Redirect("").RawQuery
}

// postForm returns the HTTP-POST binding form body carrying the same signed
// LogoutResponse as redirectQuery.
func (idp *samlSLOTestIdP) postForm(t *testing.T, requestID string) string {
	t.Helper()
	query, err := url.ParseQuery(idp.redirectQuery(t, requestID))
	if err != nil {
		t.Fatalf("parse redirect query: %v", err)
	}
	signedXML := inflateSAMLTestPayload(t, query.Get("SAMLResponse"))
	return url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(signedXML)}}.Encode()
}

func inflateSAMLTestPayload(t *testing.T, payload string) []byte {
	t.Helper()
	compressed, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode SAML payload: %v", err)
	}
	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(compressed)))
	if err != nil {
		t.Fatalf("inflate SAML payload: %v", err)
	}
	return raw
}

// newSAMLTestSession creates a live SAML session with the okta provider.
func newSAMLTestSession(t *testing.T, nameID string) string {
	t.Helper()
	token := generateSessionToken()
	GetSessionStore().CreateSAMLSession(token, time.Hour, "agent", "127.0.0.1", nameID, &SAMLTokenInfo{
		ProviderID:   "okta",
		NameID:       nameID,
		SessionIndex: "sess-" + nameID,
	})
	return token
}

// startSAMLTestLogout runs SP-initiated logout for the session and returns the
// ID of the LogoutRequest sent to the IdP.
func startSAMLTestLogout(t *testing.T, router *Router, token string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/logout", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	rec := httptest.NewRecorder()
	router.handleSAMLLogout(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("logout: expected status %d, got %d", http.StatusFound, rec.Code)
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || location.Host != "idp.example.com" {
		t.Fatalf("logout: unexpected SLO redirect %q (%v)", rec.Header().Get("Location"), err)
	}
	var logoutRequest struct {
		ID string `xml:"ID,attr"`
	}
	if err := xml.Unmarshal(inflateSAMLTestPayload(t, location.Query().Get("SAMLRequest")), &logoutRequest); err != nil {
		t.Fatalf("parse LogoutRequest: %v", err)
	}
	if logoutRequest.ID == "" {
		t.Fatal("LogoutRequest has no ID")
	}
	return logoutRequest.ID
}

// deliverSAMLTestLogoutResponse sends an HTTP-Redirect binding LogoutResponse
// to /slo from a browser carrying the given session cookie ("" for none).
func deliverSAMLTestLogoutResponse(router *Router, query, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/saml/okta/slo?"+query, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	}
	rec := httptest.NewRecorder()
	router.handleSAMLSLO(rec, req)
	return rec
}

// postSAMLTestLogoutResponse sends an HTTP-POST binding form body to /slo as a
// cross-site form POST does: without the SameSite=Lax session cookie.
func postSAMLTestLogoutResponse(router *Router, form string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/slo", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.handleSAMLSLO(rec, req)
	return rec
}

func assertSAMLSLORefused(t *testing.T, rec *httptest.ResponseRecorder, label string) {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("%s: expected status %d, got %d body=%q", label, http.StatusForbidden, rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("%s: refused SLO must not redirect, got %q", label, loc)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("%s: refused SLO must not touch cookies, got %v", label, cookies)
	}
}

// The browser that started SP-initiated logout comes back from the IdP without
// a session cookie, because handleSAMLLogout cleared it before redirecting.
// The IdP's signed answer to that logout completes it, by either binding.
func TestHandleSAMLSLO_AcceptsAnswerToOutstandingLogout(t *testing.T) {
	router, idp := newSAMLSLOTestRouter(t)

	token := newSAMLTestSession(t, "alice")
	requestID := startSAMLTestLogout(t, router, token)
	if ValidateSession(token) {
		t.Fatal("SP-initiated logout must clear the local session before redirecting")
	}

	rec := deliverSAMLTestLogoutResponse(router, idp.redirectQuery(t, requestID), "")
	if rec.Code != http.StatusFound {
		t.Fatalf("redirect binding: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/?logout=success" {
		t.Fatalf("redirect binding: unexpected redirect %q", loc)
	}

	token = newSAMLTestSession(t, "alice")
	requestID = startSAMLTestLogout(t, router, token)
	rec = postSAMLTestLogoutResponse(router, idp.postForm(t, requestID))
	if rec.Code != http.StatusFound {
		t.Fatalf("POST binding: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/?logout=success" {
		t.Fatalf("POST binding: unexpected redirect %q", loc)
	}
}

// Each LogoutRequest is answered once. Replaying the IdP's response, with or
// without a session cookie, is refused and leaves the presenting session alone.
func TestHandleSAMLSLO_RefusesReplayedLogoutResponse(t *testing.T) {
	router, idp := newSAMLSLOTestRouter(t)

	requestID := startSAMLTestLogout(t, router, newSAMLTestSession(t, "alice"))
	query := idp.redirectQuery(t, requestID)
	if rec := deliverSAMLTestLogoutResponse(router, query, ""); rec.Code != http.StatusFound {
		t.Fatalf("first delivery: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}

	assertSAMLSLORefused(t, deliverSAMLTestLogoutResponse(router, query, ""), "replay without session")

	victim := newSAMLTestSession(t, "bob")
	assertSAMLSLORefused(t, deliverSAMLTestLogoutResponse(router, query, victim), "replay into another session")
	if !ValidateSession(victim) {
		t.Fatal("replayed LogoutResponse cleared another user's session")
	}
}

// A response the IdP signed for one user's logout must not clear a different
// user's live session, even on its first delivery inside the issue window: a
// cross-site GET carries the victim's SameSite=Lax cookie and skips CSRF. The
// misdelivered response spends the request, so it cannot be retried either.
func TestHandleSAMLSLO_RefusesAnotherSessionsLogoutResponse(t *testing.T) {
	router, idp := newSAMLSLOTestRouter(t)

	requestID := startSAMLTestLogout(t, router, newSAMLTestSession(t, "attacker"))
	query := idp.redirectQuery(t, requestID)

	victim := newSAMLTestSession(t, "victim")
	assertSAMLSLORefused(t, deliverSAMLTestLogoutResponse(router, query, victim), "another session's response")
	if !ValidateSession(victim) {
		t.Fatal("another user's LogoutResponse cleared the victim's session")
	}
	assertSAMLSLORefused(t, deliverSAMLTestLogoutResponse(router, query, ""), "retry after misdelivery")
}

// A correctly signed LogoutResponse that answers no LogoutRequest this SP
// issued (an unsolicited one, or one answering a request the IdP was handed
// by someone else) is refused without touching the session.
func TestHandleSAMLSLO_RefusesUnsolicitedLogoutResponse(t *testing.T) {
	router, idp := newSAMLSLOTestRouter(t)
	victim := newSAMLTestSession(t, "victim")

	for _, requestID := range []string{"", "id-never-issued-by-this-sp"} {
		label := "InResponseTo=" + requestID
		assertSAMLSLORefused(t, deliverSAMLTestLogoutResponse(router, idp.redirectQuery(t, requestID), victim), label)
		if !ValidateSession(victim) {
			t.Fatalf("%s: unsolicited LogoutResponse cleared the session", label)
		}
	}
}

// encoding/xml matches an un-namespaced attribute field to any attribute with
// that local name, a namespace declaration such as xmlns:InResponseTo included,
// and exclusive canonicalization can leave an unused declaration out of the
// signed form. Appending one to a signed response must not rebind it to another
// outstanding LogoutRequest.
func TestHandleSAMLSLO_InResponseToIgnoresNamespaceDeclarations(t *testing.T) {
	router, idp := newSAMLSLOTestRouter(t)
	ownRequestID := startSAMLTestLogout(t, router, newSAMLTestSession(t, "alice"))
	otherRequestID := startSAMLTestLogout(t, router, newSAMLTestSession(t, "bob"))

	query, err := url.ParseQuery(idp.redirectQuery(t, ownRequestID))
	if err != nil {
		t.Fatalf("parse redirect query: %v", err)
	}
	signedXML := string(inflateSAMLTestPayload(t, query.Get("SAMLResponse")))
	genuine := `InResponseTo="` + ownRequestID + `"`
	if !strings.Contains(signedXML, genuine) {
		t.Fatalf("signed response has no %s: %s", genuine, signedXML)
	}
	spoofed := strings.Replace(signedXML, genuine, genuine+` xmlns:InResponseTo="`+otherRequestID+`"`, 1)
	form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString([]byte(spoofed))}}.Encode()

	// The signature still verifies, so the response completes the request it
	// was signed for and nothing else.
	if rec := postSAMLTestLogoutResponse(router, form); rec.Code != http.StatusFound {
		t.Fatalf("spoofed delivery: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}
	if rec := deliverSAMLTestLogoutResponse(router, idp.redirectQuery(t, otherRequestID), ""); rec.Code != http.StatusFound {
		t.Fatalf("a namespace declaration spent another LogoutRequest: genuine answer got %d body=%q", rec.Code, rec.Body.String())
	}
	assertSAMLSLORefused(t, deliverSAMLTestLogoutResponse(router, idp.redirectQuery(t, ownRequestID), ""), "signed request after spoofed delivery")
}

// A cross-site HTTP-POST delivery withholds the SameSite=Lax session cookie,
// so /slo cannot see which session the browser holds. Completing a logout
// must therefore write no cookie: a deletion cookie would log out the
// browser's live session even though the request never presented it.
func TestHandleSAMLSLO_CompletionWritesNoCookies(t *testing.T) {
	router, idp := newSAMLSLOTestRouter(t)

	requestID := startSAMLTestLogout(t, router, newSAMLTestSession(t, "attacker"))
	victim := newSAMLTestSession(t, "victim")
	rec := postSAMLTestLogoutResponse(router, idp.postForm(t, requestID))
	if rec.Code != http.StatusFound {
		t.Fatalf("cookieless POST binding: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("cookieless POST binding: SLO completion wrote cookies %v", cookies)
	}
	if !ValidateSession(victim) {
		t.Fatal("SLO completion invalidated an unrelated session")
	}

	token := newSAMLTestSession(t, "alice")
	requestID = startSAMLTestLogout(t, router, token)
	rec = deliverSAMLTestLogoutResponse(router, idp.redirectQuery(t, requestID), token)
	if rec.Code != http.StatusFound {
		t.Fatalf("own session: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("own session: SLO completion wrote cookies %v", cookies)
	}
}

// The SAML routes skip CSRF, and a cross-site POST withholds the
// SameSite=Lax session cookie, so SP-initiated logout must not answer a
// request that carries no session with the deletion cookies of a logout.
func TestHandleSAMLLogout_RefusesRequestsWithoutSession(t *testing.T) {
	router, _ := newSAMLSLOTestRouter(t)

	for _, path := range []string{"/api/saml/okta/logout", "/api/saml/unknown/logout"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		router.handleSAMLLogout(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected status %d, got %d body=%q", path, http.StatusUnauthorized, rec.Code, rec.Body.String())
		}
		if cookies := rec.Result().Cookies(); len(cookies) != 0 {
			t.Fatalf("%s: cookieless logout wrote cookies %v", path, cookies)
		}
	}
}

func TestHandleSAMLLogout_SLOUnavailable(t *testing.T) {
	dataPath := t.TempDir()
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

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

	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/logout", nil)
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
	dataPath := t.TempDir()
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

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

	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/logout", nil)
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

// A cross-site top-level GET carries the SameSite=Lax session cookie and skips
// the CSRF check, so SP-initiated logout must not clear the session on GET or
// HEAD.
func TestHandleSAMLLogout_RefusesSafeMethods(t *testing.T) {
	dataPath := t.TempDir()
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

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

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/api/saml/okta/logout", nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
		rec := httptest.NewRecorder()

		router.handleSAMLLogout(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: expected status %d, got %d", method, http.StatusMethodNotAllowed, rec.Code)
		}
		if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
			t.Fatalf("%s: expected Allow %q, got %q", method, http.MethodPost, allow)
		}
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Fatalf("%s: expected no SLO redirect, got %q", method, loc)
		}
		if !ValidateSession(token) {
			t.Fatalf("%s: SAML logout cleared the session", method)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/logout", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	rec := httptest.NewRecorder()
	router.handleSAMLLogout(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("POST: expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if ValidateSession(token) {
		t.Fatal("POST: expected SAML logout to clear the session")
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

const samlManualCertTestIdPEntityID = "https://idp.example.com/metadata"

// samlManualCertTestIdP holds an IdP signing key and its PEM certificate, the
// form an administrator pastes or points IDPCertFile at for a provider
// configured without IdP metadata. It signs ACS Responses and LogoutResponses
// the way the real IdP would.
type samlManualCertTestIdP struct {
	certPEM []byte
	idp     *saml.IdentityProvider
	slo     *samlSLOTestIdP
}

func newSAMLManualCertTestIdP(t *testing.T) *samlManualCertTestIdP {
	t.Helper()
	certPEM, _, key := generateTestCert(t)
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("decode IdP certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse IdP certificate: %v", err)
	}
	entityID, err := url.Parse(samlManualCertTestIdPEntityID)
	if err != nil {
		t.Fatalf("parse IdP entity ID: %v", err)
	}
	const signatureMethod = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	return &samlManualCertTestIdP{
		certPEM: certPEM,
		idp: &saml.IdentityProvider{
			Key:             key,
			Certificate:     cert,
			MetadataURL:     *entityID,
			SignatureMethod: signatureMethod,
		},
		slo: &samlSLOTestIdP{signer: &saml.ServiceProvider{
			EntityID:        samlManualCertTestIdPEntityID,
			Key:             key,
			Certificate:     cert,
			SignatureMethod: signatureMethod,
		}},
	}
}

// metadataXML publishes the same signing certificate as IdP metadata does,
// base64 DER inside <X509Certificate>.
func (idp *samlManualCertTestIdP) metadataXML() string {
	return `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="` + samlManualCertTestIdPEntityID + `">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data><X509Certificate>` + base64.StdEncoding.EncodeToString(idp.idp.Certificate.Raw) + `</X509Certificate></X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
    <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/slo"/>
  </IDPSSODescriptor>
</EntityDescriptor>`
}

// acsPost returns the HTTP-POST binding request to the service's ACS carrying
// a signed, unsolicited (no InResponseTo) Response that identifies alice.
func (idp *samlManualCertTestIdP) acsPost(t *testing.T, service *SAMLService) *http.Request {
	t.Helper()
	return samlTestACSRequest(service, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, ""))}})
}

// acsPostAnswering returns the same request for an SP-initiated login: the
// Response and its SubjectConfirmationData answer the AuthnRequest requestID.
func (idp *samlManualCertTestIdP) acsPostAnswering(t *testing.T, service *SAMLService, requestID string) *http.Request {
	t.Helper()
	return samlTestACSRequest(service, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, requestID))}})
}

// responseXML returns the Response the IdP posts to the service's ACS after
// authenticating alice, answering requestID ("" for an unsolicited one). Both
// the Response and its Assertion are signed.
func (idp *samlManualCertTestIdP) responseXML(t *testing.T, service *SAMLService, requestID string) []byte {
	t.Helper()
	return idp.signedResponseXML(t, service, requestID, "alice", nil)
}

// signedResponseXML is responseXML for the user nameID, with edit applied to
// the Assertion before the IdP signs it.
func (idp *samlManualCertTestIdP) signedResponseXML(t *testing.T, service *SAMLService, requestID, nameID string, edit func(*saml.Assertion)) []byte {
	t.Helper()
	spMetadata := service.sp.Metadata()
	authn := &saml.IdpAuthnRequest{
		IDP:                     idp.idp,
		HTTPRequest:             httptest.NewRequest(http.MethodPost, "https://idp.example.com/sso", nil),
		Request:                 saml.AuthnRequest{ID: requestID},
		ServiceProviderMetadata: spMetadata,
		SPSSODescriptor:         &spMetadata.SPSSODescriptors[0],
		ACSEndpoint:             &saml.IndexedEndpoint{Binding: saml.HTTPPostBinding, Location: service.sp.AcsURL.String()},
		Now:                     time.Now(),
	}
	if err := (saml.DefaultAssertionMaker{}).MakeAssertion(authn, &saml.Session{
		ID:         "idp-session-" + nameID,
		CreateTime: time.Now(),
		ExpireTime: time.Now().Add(time.Hour),
		Index:      "idx-" + nameID,
		NameID:     nameID,
	}); err != nil {
		t.Fatalf("make assertion: %v", err)
	}
	if edit != nil {
		edit(authn.Assertion)
	}
	form, err := authn.PostBinding()
	if err != nil {
		t.Fatalf("sign ACS Response: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(form.SAMLResponse)
	if err != nil {
		t.Fatalf("decode ACS Response: %v", err)
	}
	return raw
}

func samlTestACSRequest(service *SAMLService, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, service.sp.AcsURL.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// samlTestAuthnRequestID returns the ID of the AuthnRequest carried by an
// HTTP-Redirect binding URL to the IdP.
func samlTestAuthnRequestID(t *testing.T, redirectURL string) string {
	t.Helper()
	location, err := url.Parse(redirectURL)
	if err != nil || location.Host != "idp.example.com" {
		t.Fatalf("unexpected AuthnRequest redirect %q (%v)", redirectURL, err)
	}
	var authnRequest struct {
		ID string `xml:"ID,attr"`
	}
	if err := xml.Unmarshal(inflateSAMLTestPayload(t, location.Query().Get("SAMLRequest")), &authnRequest); err != nil {
		t.Fatalf("parse AuthnRequest: %v", err)
	}
	if authnRequest.ID == "" {
		t.Fatal("AuthnRequest has no ID")
	}
	return authnRequest.ID
}

// samlTestXMLAttr appends attr="value" to the first start tag named tag (any
// prefix) in doc, the way an attacker edits a signed Response in transit.
func samlTestXMLAttr(t *testing.T, doc []byte, tag, attr, value string) []byte {
	t.Helper()
	re := regexp.MustCompile(`<([A-Za-z0-9]+:)?` + tag + `[\s>/]`)
	loc := re.FindIndex(doc)
	if loc == nil {
		t.Fatalf("no <%s> in %s", tag, doc)
	}
	end := bytes.IndexByte(doc[loc[0]:], '>') + loc[0]
	if doc[end-1] == '/' {
		end--
	}
	out := append([]byte{}, doc[:end]...)
	out = append(out, []byte(` `+attr+`="`+value+`"`)...)
	return append(out, doc[end:]...)
}

// useSAMLTestAuthStores gives the test fresh session and CSRF stores and puts
// the previous ones back afterwards, so tests that run later still find the
// stores they were using.
func useSAMLTestAuthStores(t *testing.T) {
	t.Helper()
	dataPath := t.TempDir()

	sessionStoreMu.Lock()
	previousSessionStore, previousSessionPath := sessionStore, sessionStoreDataPath
	sessionStore, sessionStoreDataPath = nil, ""
	sessionStoreMu.Unlock()
	csrfStoreMu.Lock()
	previousCSRFStore, previousCSRFPath := csrfStore, csrfStoreDataPath
	csrfStore, csrfStoreDataPath = nil, ""
	csrfStoreMu.Unlock()
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

	t.Cleanup(func() {
		resetSessionStoreForTests()
		resetCSRFStoreForTests()
		sessionStoreMu.Lock()
		sessionStore, sessionStoreDataPath = previousSessionStore, previousSessionPath
		sessionStoreMu.Unlock()
		csrfStoreMu.Lock()
		csrfStore, csrfStoreDataPath = previousCSRFStore, previousCSRFPath
		csrfStoreMu.Unlock()
	})
}

// samlTestUnsignedResponse removes the Response's own signature, leaving only
// its Assertion signed, as many IdPs send it.
func samlTestUnsignedResponse(t *testing.T, signed []byte) []byte {
	t.Helper()
	unsigned := regexp.MustCompile(`(?s)(<samlp:Response[^>]*>.*?</saml:Issuer>)<ds:Signature.*?</ds:Signature>`).ReplaceAll(signed, []byte("$1"))
	if bytes.Equal(unsigned, signed) {
		t.Fatalf("could not strip the Response signature from %s", signed)
	}
	return unsigned
}

const samlACSTestProvider = "okta"

// newSAMLACSTestRouter returns a router whose "okta" SAML provider trusts idp
// through its metadata, at default settings unless allowIDPInitiated is set,
// with fresh session and CSRF stores.
func newSAMLACSTestRouter(t *testing.T, idp *samlManualCertTestIdP, allowIDPInitiated bool) (*Router, *SAMLService) {
	t.Helper()
	useSAMLTestAuthStores(t)

	cfg := &config.SAMLProviderConfig{IDPMetadataXML: idp.metadataXML(), AllowIDPInitiated: allowIDPInitiated}
	service, err := NewSAMLService(context.Background(), samlACSTestProvider, cfg, "https://pulse.example.com")
	if err != nil {
		t.Fatalf("NewSAMLService: %v", err)
	}
	router := &Router{
		samlManager: NewSAMLServiceManager("https://pulse.example.com"),
		ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{{
			ID: samlACSTestProvider, Name: "Test SAML", Type: config.SSOProviderTypeSAML, Enabled: true, SAML: cfg,
		}}},
	}
	router.samlManager.services[samlACSTestProvider] = service
	return router, service
}

// startSAMLTestLogin starts SP-initiated login in a browser holding loginToken
// ("" for a browser that has none yet) and returns the ID of the AuthnRequest
// sent to the IdP and the login token the browser holds afterwards.
func startSAMLTestLogin(t *testing.T, router *Router, loginToken, returnTo string) (requestID, heldToken string) {
	t.Helper()
	target := "/api/saml/okta/login"
	if returnTo != "" {
		target += "?returnTo=" + url.QueryEscape(returnTo)
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if loginToken != "" {
		req.AddCookie(&http.Cookie{Name: cookieNameSAMLLogin, Value: loginToken})
	}
	rec := httptest.NewRecorder()
	router.handleSAMLLogin(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("login: expected status %d, got %d body=%q", http.StatusFound, rec.Code, rec.Body.String())
	}
	heldToken = loginToken
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSAMLLogin {
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 || cookie.Path != "/" {
				t.Fatalf("login cookie must be an HttpOnly SameSite=Lax cookie that expires, got %+v", cookie)
			}
			heldToken = cookie.Value
		}
	}
	if heldToken == "" {
		t.Fatal("login set no login cookie")
	}
	return samlTestAuthnRequestID(t, rec.Header().Get("Location")), heldToken
}

// postSAMLTestACS delivers an ACS form from a browser holding loginToken (""
// for none). A cross-site HTTP-POST from the IdP carries no SameSite=Lax
// cookie, so pass "" to model the IdP's own delivery.
func postSAMLTestACS(router *Router, form url.Values, loginToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/acs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if loginToken != "" {
		req.AddCookie(&http.Cookie{Name: cookieNameSAMLLogin, Value: loginToken})
	}
	rec := httptest.NewRecorder()
	router.handleSAMLACS(rec, req)
	return rec
}

// samlTestRepostForm returns the hidden fields of the ACS repost page.
func samlTestRepostForm(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected the ACS repost page, got status %d type %q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	form := url.Values{}
	for _, match := range regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(rec.Body.String(), -1) {
		form.Add(match[1], html.UnescapeString(match[2]))
	}
	return form
}

// deliverSAMLTestResponse delivers a Response the way the browser that holds
// loginToken receives it: the IdP's cross-site POST, which carries no login
// cookie and gets the repost page, then the page's same-origin repost.
func deliverSAMLTestResponse(t *testing.T, router *Router, form url.Values, loginToken string) *httptest.ResponseRecorder {
	t.Helper()
	return postSAMLTestACS(router, samlTestRepostForm(t, postSAMLTestACS(router, form, "")), loginToken)
}

func assertSAMLLoginSucceeded(t *testing.T, rec *httptest.ResponseRecorder, wantLocation, label string) {
	t.Helper()
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != wantLocation {
		t.Fatalf("%s: expected redirect to %q, got status %d location %q body=%q", label, wantLocation, rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSession && cookie.Value != "" && ValidateSession(cookie.Value) {
			return
		}
	}
	t.Fatalf("%s: login established no session, cookies %v", label, rec.Result().Cookies())
}

func assertSAMLLoginRefused(t *testing.T, rec *httptest.ResponseRecorder, label string) {
	t.Helper()
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "saml_error=saml_validation_failed") {
		t.Fatalf("%s: expected the validation-failed redirect, got status %d location %q body=%q", label, rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSession || cookie.Name == cookieNameSessionSecure {
			t.Fatalf("%s: refused login wrote session cookie %+v", label, cookie)
		}
	}
}

// At default settings, with IdP-initiated logins off, SP-initiated login
// completes: the IdP's cross-site POST of its signed answer gets the repost
// page, whose same-origin repost carries the login cookie, and the login
// returns to the recorded returnTo whatever RelayState the IdP posts.
func TestHandleSAMLACS_CompletesLoginStartedInThisBrowser(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	router, service := newSAMLACSTestRouter(t, idp, false)

	requestID, token := startSAMLTestLogin(t, router, "", "/alerts?tab=history&range=7d")
	form := url.Values{
		"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, requestID))},
		"RelayState":   {"/settings"},
	}
	repost := postSAMLTestACS(router, form, "")
	fields := samlTestRepostForm(t, repost)
	if fields.Get("SAMLResponse") != form.Get("SAMLResponse") || fields.Get("RelayState") != "/settings" || fields.Get(samlACSRepostField) != "1" {
		t.Fatalf("repost page must carry the posted fields and the repost marker, got %v", fields)
	}
	if cookies := repost.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("repost page must not write cookies, got %v", cookies)
	}
	if body := repost.Body.String(); strings.Contains(body, " action=") || !strings.Contains(body, "document.forms[0].submit()") {
		t.Fatalf("repost page must submit itself back to the URL the IdP posted to, got %q", body)
	}

	assertSAMLLoginSucceeded(t, postSAMLTestACS(router, fields, token), "/alerts?range=7d&saml=success&tab=history", "repost with login cookie")
}

// Each AuthnRequest is answered once, and only in the browser that started it.
// A Response answering another browser's request, an unknown request or none
// is refused, and refusing another browser leaves that browser's login intact.
func TestHandleSAMLACS_RefusesResponsesNotAnsweringThisBrowsersLogin(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	router, service := newSAMLACSTestRouter(t, idp, false)

	victimRequestID, victimToken := startSAMLTestLogin(t, router, "", "/")
	attackerRequestID, attackerToken := startSAMLTestLogin(t, router, "", "/")
	attackerForm := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, attackerRequestID))}}

	assertSAMLLoginRefused(t, deliverSAMLTestResponse(t, router, attackerForm, victimToken), "another browser's Response")
	for _, requestID := range []string{"", "id-never-issued-by-this-sp"} {
		form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, requestID))}}
		assertSAMLLoginRefused(t, deliverSAMLTestResponse(t, router, form, victimToken), "InResponseTo="+requestID)
	}
	assertSAMLLoginRefused(t, postSAMLTestACS(router, url.Values{
		"SAMLResponse":     attackerForm["SAMLResponse"],
		samlACSRepostField: {"1"},
	}, ""), "repost without a login cookie")

	assertSAMLLoginSucceeded(t, deliverSAMLTestResponse(t, router, attackerForm, attackerToken), "/?saml=success", "the Response's own browser")
	assertSAMLLoginRefused(t, deliverSAMLTestResponse(t, router, attackerForm, attackerToken), "replay in the same browser")

	victimForm := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, victimRequestID))}}
	assertSAMLLoginSucceeded(t, deliverSAMLTestResponse(t, router, victimForm, victimToken), "/?saml=success", "the victim's own login after the refusals")
}

// encoding/xml fills an un-namespaced attribute field from a namespace
// declaration with the same local name, and exclusive canonicalization leaves
// an unused declaration out of the signed form. Declaring xmlns:InResponseTo
// on a signed Response or Assertion must not rebind the attacker's answer to
// the victim's outstanding AuthnRequest, whether the Response itself is signed
// or only its Assertion is.
func TestHandleSAMLACS_InResponseToIgnoresNamespaceDeclarations(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	router, service := newSAMLACSTestRouter(t, idp, false)

	victimRequestID, victimToken := startSAMLTestLogin(t, router, "", "/")
	attackerRequestID, _ := startSAMLTestLogin(t, router, "", "/")
	signed := idp.responseXML(t, service, attackerRequestID)

	spoofed := samlTestXMLAttr(t, signed, "Response", "xmlns:InResponseTo", victimRequestID)
	spoofed = samlTestXMLAttr(t, spoofed, "SubjectConfirmationData", "xmlns:InResponseTo", victimRequestID)
	assertSAMLLoginRefused(t, deliverSAMLTestResponse(t, router, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(spoofed)}}, victimToken), "signed Response")

	// Without its own signature the Response's attributes are the attacker's
	// to write; only the signed Assertion says which request it answers.
	assertionOnly := bytes.Replace(samlTestUnsignedResponse(t, signed), []byte(`InResponseTo="`+attackerRequestID+`"`), []byte(`InResponseTo="`+victimRequestID+`"`), 1)
	assertionOnly = samlTestXMLAttr(t, assertionOnly, "SubjectConfirmationData", "xmlns:InResponseTo", victimRequestID)
	assertSAMLLoginRefused(t, deliverSAMLTestResponse(t, router, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(assertionOnly)}}, victimToken), "assertion-only signature")

	victimForm := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, victimRequestID))}}
	assertSAMLLoginSucceeded(t, deliverSAMLTestResponse(t, router, victimForm, victimToken), "/?saml=success", "the victim's own login after the spoofs")
}

// Many IdPs sign only the Assertion. Its signed SubjectConfirmationData then
// names the request, and the login completes. A signed assertion that confirms
// no request cannot ride along with one that does: an IdP-signed assertion for
// another user without a SubjectConfirmation, placed ahead of the victim's own
// in an unsigned Response, is the assertion crewjam would return, and the
// victim's request must not vouch for it.
func TestHandleSAMLACS_BindsTheAssertionItReturns(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	router, service := newSAMLACSTestRouter(t, idp, false)

	victimRequestID, victimToken := startSAMLTestLogin(t, router, "", "/")
	victimResponse := samlTestUnsignedResponse(t, idp.responseXML(t, service, victimRequestID))

	unconfirmed := idp.signedResponseXML(t, service, "", "mallory", func(assertion *saml.Assertion) {
		assertion.Subject.SubjectConfirmations = nil
	})
	unconfirmedAssertion := regexp.MustCompile(`(?s)<saml:Assertion .*?</saml:Assertion>`).Find(unconfirmed)
	at := bytes.Index(victimResponse, []byte("<saml:Assertion "))
	if unconfirmedAssertion == nil || at < 0 {
		t.Fatalf("could not splice assertions: %s / %s", unconfirmed, victimResponse)
	}
	spliced := append(append(append([]byte{}, victimResponse[:at]...), unconfirmedAssertion...), victimResponse[at:]...)
	assertSAMLLoginRefused(t, deliverSAMLTestResponse(t, router, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(spliced)}}, victimToken), "unconfirmed assertion ahead of the victim's")

	assertSAMLLoginSucceeded(t, deliverSAMLTestResponse(t, router, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(victimResponse)}}, victimToken), "/?saml=success", "assertion-only signature")
}

// Logins started in several tabs of one browser share its login cookie, and
// each completes with its own Response, in any order, returning to its own
// path.
func TestHandleSAMLACS_CompletesLoginsFromSeveralTabs(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	router, service := newSAMLACSTestRouter(t, idp, false)

	firstRequestID, token := startSAMLTestLogin(t, router, "", "/infrastructure")
	secondRequestID, secondToken := startSAMLTestLogin(t, router, token, "/alerts")
	if secondToken != token {
		t.Fatalf("a second login must keep the browser's login token, got %q then %q", token, secondToken)
	}

	second := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, secondRequestID))}}
	assertSAMLLoginSucceeded(t, deliverSAMLTestResponse(t, router, second, token), "/alerts?saml=success", "second tab")
	first := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(idp.responseXML(t, service, firstRequestID))}}
	assertSAMLLoginSucceeded(t, deliverSAMLTestResponse(t, router, first, token), "/infrastructure?saml=success", "first tab")
}

// With IdP-initiated logins allowed, an unsolicited Response still completes
// without a login cookie or a repost, as before request binding. But a stale
// signed Response stays stale: namespace declarations named like the
// attributes crewjam checks for age cannot override them.
func TestHandleSAMLACS_IDPInitiatedResponsesCannotOverrideTheirAge(t *testing.T) {
	idp := newSAMLManualCertTestIdP(t)
	router, service := newSAMLACSTestRouter(t, idp, true)

	signed := idp.responseXML(t, service, "")
	form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(signed)}, "RelayState": {"/settings"}}
	assertSAMLLoginSucceeded(t, postSAMLTestACS(router, form, ""), "/settings?saml=success", "fresh unsolicited Response")

	realNow := saml.TimeNow
	t.Cleanup(func() { saml.TimeNow = realNow })
	saml.TimeNow = func() time.Time { return realNow().Add(2 * time.Hour) }
	assertSAMLLoginRefused(t, postSAMLTestACS(router, form, ""), "stale unsolicited Response")

	future := realNow().Add(3 * time.Hour).UTC().Format(time.RFC3339)
	spoofed := samlTestXMLAttr(t, signed, "Response", "xmlns:IssueInstant", future)
	spoofed = samlTestXMLAttr(t, spoofed, "Assertion", "xmlns:IssueInstant", future)
	spoofed = samlTestXMLAttr(t, spoofed, "SubjectConfirmationData", "xmlns:NotOnOrAfter", future)
	spoofed = samlTestXMLAttr(t, spoofed, "Conditions", "xmlns:NotOnOrAfter", future)
	assertSAMLLoginRefused(t, postSAMLTestACS(router, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(spoofed)}}, ""), "stale Response with spoofed age")
}

// The repost page is the IdP's form replayed from Pulse's origin: values are
// HTML-escaped and its script carries the request's CSP nonce.
func TestWriteSAMLACSRepost_EscapesFieldsAndCarriesCSPNonce(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/saml/okta/acs", strings.NewReader(url.Values{
		"SAMLResponse": {`"><script>alert(1)</script>`},
		"RelayState":   {"/"},
		"Unrelated":    {"dropped"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), cspNonceKey{}, "test-nonce"))
	if err := req.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}
	rec := httptest.NewRecorder()
	writeSAMLACSRepost(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)") || strings.Contains(body, "Unrelated") {
		t.Fatalf("repost page must escape values and carry only SAML fields, got %q", body)
	}
	if !strings.Contains(body, `<script nonce="test-nonce">`) {
		t.Fatalf("repost script must carry the CSP nonce, got %q", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("repost page must not be cached, got Cache-Control %q", got)
	}
	if fields := samlTestRepostForm(t, rec); fields.Get("SAMLResponse") != `"><script>alert(1)</script>` {
		t.Fatalf("repost must carry the exact posted value, got %v", fields)
	}
}

// samlRejectionDetail adds crewjam/saml's private reason to a rejected
// response, which its public error ("Authentication failed") hides.
func samlRejectionDetail(err error) string {
	var invalid *saml.InvalidResponseError
	if errors.As(err, &invalid) && invalid.PrivateErr != nil {
		return err.Error() + ": " + invalid.PrivateErr.Error()
	}
	return fmt.Sprint(err)
}
