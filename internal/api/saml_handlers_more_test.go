package api

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
