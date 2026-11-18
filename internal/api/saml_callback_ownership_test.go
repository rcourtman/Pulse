package api

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A real SOAP artifact resolution is held at the HTTP boundary. The inner
// Response and Assertion are signed by the fixture IdP, not a verifier hook.
type samlHeldArtifact struct {
	router   *Router
	service  *SAMLService
	idp      *samlManualCertTestIdP
	entered  chan struct{}
	release  func()
	response []byte
	calls    atomic.Int32
	metadata atomic.Value
}

func newSAMLHeldArtifact(t *testing.T, allowIDPInitiated bool) *samlHeldArtifact {
	t.Helper()
	f := &samlHeldArtifact{idp: newSAMLManualCertTestIdP(t), entered: make(chan struct{})}
	release := make(chan struct{})
	var once sync.Once
	f.release = func() { once.Do(func() { close(release) }) }
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/metadata" {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = io.WriteString(w, f.metadata.Load().(string))
			return
		}
		if req.URL.Path != "/artifact" || req.Method != http.MethodPost {
			http.NotFound(w, req)
			return
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "fixture body read failed", 400)
			return
		}
		var envelope struct {
			Body struct {
				Resolve struct {
					ID string `xml:"ID,attr"`
				} `xml:"ArtifactResolve"`
			} `xml:"Body"`
		}
		if xml.Unmarshal(body, &envelope) != nil || envelope.Body.Resolve.ID == "" {
			http.Error(w, "fixture requires an actual ArtifactResolve", 400)
			return
		}
		if f.calls.Add(1) == 1 {
			close(f.entered)
			select {
			case <-release:
			case <-req.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "text/xml")
		// crewjam permits an unsigned outer ArtifactResponse only when its
		// embedded Response/Assertion passes the ordinary signature checks.
		_, _ = fmt.Fprintf(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><samlp:ArtifactResponse xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="fixture-artifact-response" InResponseTo="%s" Version="2.0" IssueInstant="%s"><samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>%s</samlp:ArtifactResponse></soap:Body></soap:Envelope>`, envelope.Body.Resolve.ID, time.Now().UTC().Format(time.RFC3339Nano), f.response)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(f.release)
	metadata := strings.Replace(f.idp.metadataXML(), "</IDPSSODescriptor>", `<ArtifactResolutionService Binding="urn:oasis:names:tc:SAML:2.0:bindings:SOAP" Location="`+server.URL+`/artifact" index="0"/></IDPSSODescriptor>`, 1)
	f.metadata.Store(metadata)
	useSAMLTestAuthStores(t)
	p := testSAMLProvider(samlACSTestProvider, true)
	p.Name, p.DisplayName = "Synthetic SAML", "Synthetic SAML"
	p.SAML.IDPMetadataURL = server.URL + "/metadata"
	p.SAML.AllowIDPInitiated = allowIDPInitiated
	f.router = newSAMLRouter(t, p)
	if err := f.router.InitializeSAMLProviders(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.service = f.router.samlManager.GetService(p.ID)
	return f
}

func (f *samlHeldArtifact) start(t *testing.T, requestID, token string) (*httptest.ResponseRecorder, <-chan struct{}) {
	t.Helper()
	f.response = f.idp.responseXML(t, f.service, requestID)
	form := url.Values{"SAMLart": {"synthetic-artifact"}, "RelayState": {"/untrusted-return"}}
	req := httptest.NewRequest("POST", "/api/saml/okta/acs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: cookieNameSAMLLogin, Value: token})
	}
	// Real browser bindings precede the synthetic IdP round trip. Direct
	// handler delivery models the existing same-origin repost, not a CSRF bypass.
	rec, done := httptest.NewRecorder(), make(chan struct{})
	go func() { defer close(done); f.router.handleSAMLACS(rec, req) }()
	t.Cleanup(func() { f.release(); awaitOIDCInitialization(t, done) })
	awaitOIDCInitialization(t, f.entered)
	return rec, done
}

func TestSAMLCallbackArtifactKeepsManagementAvailable(t *testing.T) {
	f := newSAMLHeldArtifact(t, false)
	id, token := startSAMLTestLogin(t, f.router, "", "/alerts?tab=history")
	rec, done := f.start(t, id, token)
	before := samlLoginRequests(f.service)
	management := make(chan struct{})
	var updateErr error
	go func() {
		defer close(management)
		updateErr = f.router.samlManager.SetPublicURL("https://current.example.test/pulse")
	}()
	awaitOIDCInitialization(t, management)
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	// A pending rebind must not monopolize the manager and prevent an
	// independent browser/provider or an explicit withdrawal from proceeding.
	other := testSAMLProvider("independent", true)
	if err := f.router.samlManager.InitializeProvider(t.Context(), other.ID, other.SAML); err != nil {
		t.Fatal(err)
	}
	if f.router.samlManager.GetService(other.ID) == nil {
		t.Fatal("independent service unavailable during artifact resolution")
	}
	f.router.samlManager.RemoveProvider(other.ID)
	if f.router.samlManager.GetService(other.ID) != nil {
		t.Fatal("withdrawal did not finish while artifact resolution was held")
	}
	f.release()
	awaitOIDCInitialization(t, done)
	assertSAMLLoginRefused(t, rec, "verifier rebound during artifact resolution")
	if !reflect.DeepEqual(before, samlLoginRequests(f.service)) || f.calls.Load() != 1 {
		t.Fatal("stale verification spent a browser request or repeated resolution")
	}
	if f.service.GetSPEntityID() != "https://current.example.test/pulse/saml/okta" {
		t.Fatal("stale completion replaced the current public URL")
	}
}

func TestSAMLCallbackArtifactCurrentVerifierStillCompletesOnce(t *testing.T) {
	f := newSAMLHeldArtifact(t, false)
	id, token := startSAMLTestLogin(t, f.router, "", "/alerts?tab=history")
	rec, done := f.start(t, id, token)
	f.release()
	awaitOIDCInitialization(t, done)
	assertSAMLLoginSucceeded(t, rec, "/alerts?saml=success&tab=history", "current signed artifact")
	if len(samlLoginRequests(f.service)) != 0 || f.calls.Load() != 1 {
		t.Fatal("verified callback did not spend exactly its original request")
	}
	form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(f.response)}}
	assertSAMLLoginRefused(t, postSAMLTestACS(f.router, form, token), "spent artifact response replay")
}

func TestSAMLCallbackArtifactTrustRefreshFencesVerification(t *testing.T) {
	for _, allowIDPInitiated := range []bool{false, true} {
		t.Run(fmt.Sprintf("idp-initiated=%v", allowIDPInitiated), func(t *testing.T) {
			f := newSAMLHeldArtifact(t, allowIDPInitiated)
			id, token := startSAMLTestLogin(t, f.router, "", "/alerts")
			if allowIDPInitiated {
				id = ""
			}
			rec, done := f.start(t, id, token)
			before := samlLoginRequests(f.service)
			currentIDP := newSAMLManualCertTestIdP(t)
			f.metadata.Store(currentIDP.metadataXML())
			refresh := make(chan struct{})
			var refreshErr error
			go func() { defer close(refresh); refreshErr = f.service.RefreshMetadata(t.Context()) }()
			awaitOIDCInitialization(t, refresh)
			if refreshErr != nil {
				t.Fatal(refreshErr)
			}
			f.release()
			awaitOIDCInitialization(t, done)
			assertSAMLLoginRefused(t, rec, "trust replaced during artifact resolution")
			if !reflect.DeepEqual(before, samlLoginRequests(f.service)) {
				t.Fatal("trust replacement spent a request from the old verification")
			}
			// Current trust still works. SP-initiated binding and IDP-initiated
			// opt-in retain their existing separate rules.
			freshID, freshToken := startSAMLTestLogin(t, f.router, token, "/alerts")
			if allowIDPInitiated {
				freshID = ""
			}
			form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(currentIDP.responseXML(t, f.service, freshID))}, "RelayState": {"/alerts"}}
			assertSAMLLoginSucceeded(t, postSAMLTestACS(f.router, form, freshToken), "/alerts?saml=success", "current trust after rejected old completion")
		})
	}
}

func TestSAMLCallbackArtifactSignedFailuresKeepBindings(t *testing.T) {
	for _, failure := range []string{"signature", "expired", "wrong-browser"} {
		t.Run(failure, func(t *testing.T) {
			f := newSAMLHeldArtifact(t, false)
			id, token := startSAMLTestLogin(t, f.router, "", "/alerts")
			before := samlLoginRequests(f.service)
			if failure == "wrong-browser" {
				_, otherToken := startSAMLTestLogin(t, f.router, "", "/infrastructure")
				token = otherToken
				before = samlLoginRequests(f.service)
			}
			rec, done := f.start(t, id, token)
			if failure == "signature" {
				// Replace only after HTTP is held; release synchronizes the read.
				f.response = newSAMLManualCertTestIdP(t).responseXML(t, f.service, id)
			}
			if failure == "expired" {
				f.service.authnRequests.mu.Lock()
				entry := f.service.authnRequests.entries[id]
				entry.expiresAt = time.Now().Add(-time.Minute)
				f.service.authnRequests.entries[id] = entry
				f.service.authnRequests.mu.Unlock()
			}
			f.release()
			awaitOIDCInitialization(t, done)
			assertSAMLLoginRefused(t, rec, failure)
			if failure == "expired" {
				delete(before, id) // Expired own requests are spent, never revived.
			}
			if !reflect.DeepEqual(before, samlLoginRequests(f.service)) {
				t.Fatal("failed signature/browser changed another pending binding")
			}
		})
	}
}
