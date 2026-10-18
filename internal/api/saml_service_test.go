package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func generateTestCert(t *testing.T) (certPEM, keyPEM []byte, key *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM, priv
}

func TestParseIDPMetadataXML(t *testing.T) {
	xml := `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp-1">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

	metadata, err := parseIDPMetadataXML([]byte(xml))
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if metadata.EntityID != "idp-1" {
		t.Fatalf("unexpected entity id: %s", metadata.EntityID)
	}

	wrapped := `<?xml version="1.0"?>
<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">
  <EntityDescriptor entityID="idp-2">
    <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"></IDPSSODescriptor>
  </EntityDescriptor>
</EntitiesDescriptor>`
	metadata, err = parseIDPMetadataXML([]byte(wrapped))
	if err != nil {
		t.Fatalf("parse wrapped metadata: %v", err)
	}
	if metadata.EntityID != "idp-2" {
		t.Fatalf("unexpected entity id: %s", metadata.EntityID)
	}

	if _, err := parseIDPMetadataXML([]byte("<bad")); err == nil {
		t.Fatal("expected error for invalid xml")
	}
}

func TestBuildManualMetadataAndCertificate(t *testing.T) {
	cfg := &config.SAMLProviderConfig{}
	service := &SAMLService{config: cfg}
	if _, err := service.buildManualMetadata(); err == nil {
		t.Fatal("expected error for missing SSO URL")
	}

	cfg.IDPSSOURL = "http://idp/sso"
	cfg.IDPSLOURL = "http://idp/slo"
	cfg.IDPIssuer = "issuer"
	certPEM, _, _ := generateTestCert(t)
	cfg.IDPCertificate = string(certPEM)
	certBlock, _ := pem.Decode(certPEM)

	metadata, err := service.buildManualMetadata()
	if err != nil {
		t.Fatalf("build metadata: %v", err)
	}
	if metadata.EntityID != "issuer" {
		t.Fatalf("unexpected entity id: %s", metadata.EntityID)
	}
	if len(metadata.IDPSSODescriptors) == 0 || len(metadata.IDPSSODescriptors[0].SingleLogoutServices) == 0 {
		t.Fatal("expected SLO service in metadata")
	}
	if len(metadata.IDPSSODescriptors[0].KeyDescriptors) == 0 {
		t.Fatal("expected key descriptor with certificate")
	}
	// crewjam/saml reads <X509Certificate> content: base64 DER, never PEM.
	certs := metadata.IDPSSODescriptors[0].KeyDescriptors[0].KeyInfo.X509Data.X509Certificates
	if len(certs) != 1 {
		t.Fatalf("expected one signing certificate, got %d", len(certs))
	}
	if certs[0].Data != base64.StdEncoding.EncodeToString(certBlock.Bytes) {
		t.Fatalf("expected the signing certificate as base64 DER, got %.40q", certs[0].Data)
	}

	cfg.IDPSSOURL = "https://user:pass@idp.example.com/sso"
	if _, err := service.buildManualMetadata(); err == nil {
		t.Fatal("expected error for idp sso URL with embedded credentials")
	}

	cfg.IDPSSOURL = "https://idp.example.com/sso"
	cfg.IDPSLOURL = "https://user:pass@idp.example.com/slo"
	if _, err := service.buildManualMetadata(); err == nil {
		t.Fatal("expected error for idp slo URL with embedded credentials")
	}
}

func TestLoadSPCredentials(t *testing.T) {
	cfg := &config.SAMLProviderConfig{}
	service := &SAMLService{config: cfg}
	if _, _, err := service.loadSPCredentials(); err == nil {
		t.Fatal("expected error for missing cert/key")
	}

	certPEM, keyPEM, _ := generateTestCert(t)
	cfg.SPCertificate = string(certPEM)
	if _, _, err := service.loadSPCredentials(); err == nil {
		t.Fatal("expected error for missing key")
	}
	cfg.SPCertificate = "bad"
	cfg.SPPrivateKey = "bad"
	if _, _, err := service.loadSPCredentials(); err == nil {
		t.Fatal("expected error for invalid pem")
	}

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ec key: %v", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(ecKey)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	cfg.SPCertificate = string(certPEM)
	cfg.SPPrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	if _, _, err := service.loadSPCredentials(); err == nil {
		t.Fatal("expected error for non-rsa key")
	}

	cfg.SPPrivateKey = string(keyPEM)
	cert, key, err := service.loadSPCredentials()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if cert == nil || key == nil {
		t.Fatal("expected cert and key")
	}
}

func TestSAMLServiceBasicFlows(t *testing.T) {
	certPEM, _, _ := generateTestCert(t)
	cfg := &config.SAMLProviderConfig{
		IDPMetadataXML: `<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp/sso"/>
    <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp/slo"/>
  </IDPSSODescriptor>
</EntityDescriptor>`,
		IDPCertificate: string(certPEM),
	}

	service, err := NewSAMLService(context.Background(), "idp", cfg, "http://localhost:8080")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	url, err := service.MakeAuthRequest("", sessionHash("login-token"))
	if err != nil || !strings.Contains(url, "SAMLRequest") {
		t.Fatalf("unexpected auth url: %v %s", err, url)
	}

	if _, err := service.GetMetadata(); err != nil {
		t.Fatalf("metadata error: %v", err)
	}

	logoutURL, err := service.MakeLogoutRequest("user", "sess", sessionHash("session-token"))
	if err != nil || !strings.Contains(logoutURL, "SAMLRequest") {
		t.Fatalf("unexpected logout url: %v %s", err, logoutURL)
	}

	service = &SAMLService{config: &config.SAMLProviderConfig{}}
	if _, err := service.MakeAuthRequest("", sessionHash("login-token")); err == nil {
		t.Fatal("expected error when sp missing")
	}
	if _, err := service.GetMetadata(); err == nil {
		t.Fatal("expected error when sp missing")
	}
	if _, err := service.MakeLogoutRequest("user", "sess", sessionHash("session-token")); err == nil {
		t.Fatal("expected error when sp missing")
	}
	if err := service.RefreshMetadata(context.Background()); err == nil {
		t.Fatal("expected refresh error without url")
	}
}

// Outstanding SLO requests expire after samlRequestTTL, and the store keeps
// at most maxSAMLRequestEntries by dropping the oldest.
func TestSAMLLogoutRequestStoreExpiresAndEvictsOldest(t *testing.T) {
	var store samlRequestStore
	now := time.Now()
	key := sessionHash("session-token")
	record := samlRequestRecord{bindingKey: key}

	store.put("id-expired", record, now)
	if err := store.consume("id-expired", key, now.Add(samlRequestTTL)); !errors.Is(err, errSAMLLogoutResponseUnbound) {
		t.Fatalf("expected an expired request to be refused, got %v", err)
	}
	store.put("id-live", record, now)
	if err := store.consume("id-live", "", now.Add(samlRequestTTL-time.Second)); err != nil {
		t.Fatalf("expected a live request to be accepted, got %v", err)
	}

	for i := 0; i <= maxSAMLRequestEntries; i++ {
		store.put(fmt.Sprintf("id-%04d", i), record, now.Add(time.Duration(i)*time.Millisecond))
	}
	if len(store.entries) != maxSAMLRequestEntries {
		t.Fatalf("expected %d outstanding requests, got %d", maxSAMLRequestEntries, len(store.entries))
	}
	if err := store.consume("id-0000", key, now); !errors.Is(err, errSAMLLogoutResponseUnbound) {
		t.Fatalf("expected the oldest request to be evicted, got %v", err)
	}
	if err := store.consume(fmt.Sprintf("id-%04d", maxSAMLRequestEntries), key, now); err != nil {
		t.Fatalf("expected the newest request to be kept, got %v", err)
	}
}

// Outstanding AuthnRequests are spent only by the browser they are bound to. A
// request presented by another browser stays outstanding for its owner, and
// each request is spent once and only before it expires.
func TestSAMLAuthnRequestStoreSpendsOnlyForItsBrowser(t *testing.T) {
	var store samlRequestStore
	now := time.Now()
	alice, bob := sessionHash("alice-login"), sessionHash("bob-login")

	store.put("id-alice-2", samlRequestRecord{bindingKey: alice, returnTo: "/alerts"}, now)
	store.put("id-alice-1", samlRequestRecord{bindingKey: alice, returnTo: "/"}, now)
	store.put("id-bob", samlRequestRecord{bindingKey: bob, returnTo: "/"}, now)

	if got, bound := store.outstanding(alice, now); strings.Join(got, ",") != "id-alice-1,id-alice-2,id-bob" || !bound {
		t.Fatalf("outstanding(alice) = %v, %v; want every request, bound", got, bound)
	}
	if _, bound := store.outstanding("", now); bound {
		t.Fatal("a browser without a login token has no request bound to it")
	}
	if _, err := store.spend("id-bob", alice, now); !errors.Is(err, errSAMLResponseUnbound) {
		t.Fatalf("expected another browser's request to be refused, got %v", err)
	}
	if _, err := store.spend("id-bob", "", now); !errors.Is(err, errSAMLResponseUnbound) {
		t.Fatalf("expected a browser without a login token to be refused, got %v", err)
	}
	if record, err := store.spend("id-bob", bob, now); err != nil || record.returnTo != "/" {
		t.Fatalf("refusing other browsers must leave the request for its owner, got %+v %v", record, err)
	}
	record, err := store.spend("id-alice-2", alice, now)
	if err != nil || record.returnTo != "/alerts" {
		t.Fatalf("spend(id-alice-2) = %+v %v, want its recorded returnTo", record, err)
	}
	if _, err := store.spend("id-alice-2", alice, now); !errors.Is(err, errSAMLResponseUnbound) {
		t.Fatalf("expected a spent request to be refused, got %v", err)
	}
	if got, bound := store.outstanding(alice, now.Add(samlRequestTTL)); len(got) != 0 || bound {
		t.Fatalf("expired requests must not be outstanding, got %v, %v", got, bound)
	}
	if _, err := store.spend("id-alice-1", alice, now.Add(samlRequestTTL)); !errors.Is(err, errSAMLResponseUnbound) {
		t.Fatalf("expected an expired request to be refused, got %v", err)
	}
}

// The binding decision reads InResponseTo from the canonical content of the
// signature that covers the assertion crewjam returned, taking only the
// attribute written without a prefix: the signed Response's when the message
// is signed, otherwise the returned assertion's own subject confirmations.
func TestSAMLSignedContentInResponseTo(t *testing.T) {
	parse := func(doc string) *etree.Element {
		t.Helper()
		parsed := etree.NewDocument()
		if err := parsed.ReadFromString(doc); err != nil {
			t.Fatalf("parse %s: %v", doc, err)
		}
		return parsed.Root()
	}
	assertion := func(id, subject, advice string) string {
		return `<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="` + id + `"><saml:Subject>` + subject + `</saml:Subject>` + advice + `</saml:Assertion>`
	}
	confirmation := func(attrs string) string {
		return `<saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer"><saml:SubjectConfirmationData ` + attrs + `/></saml:SubjectConfirmation>`
	}
	response := func(attrs, body string) string {
		return `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ` + attrs + `>` + body + `</samlp:Response>`
	}
	own, other := `InResponseTo="id-own"`, `InResponseTo="id-other"`
	advice := `<saml:Advice>` + assertion("a9", confirmation(other), "") + `</saml:Advice>`

	for _, tc := range []struct {
		name   string
		signed []string
		want   string
	}{
		{name: "signed Response", signed: []string{response(`xmlns:InResponseTo="id-other" `+own, assertion("a1", confirmation(other), ""))}, want: "id-own"},
		{name: "signed Response with encrypted Assertion", signed: []string{response(own, `<saml:EncryptedAssertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion"/>`)}, want: "id-own"},
		{name: "signed ArtifactResponse", signed: []string{`<samlp:ArtifactResponse xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" InResponseTo="id-artifact">` + response(own, "") + `</samlp:ArtifactResponse>`}, want: "id-own"},
		{name: "signed Assertion", signed: []string{assertion("a1", confirmation(`xmlns:InResponseTo="id-other" `+own), advice)}, want: "id-own"},
		{name: "signed Response answering no request", signed: []string{response(`xmlns:InResponseTo="id-own"`, assertion("a1", confirmation(own), ""))}},
		{name: "subject confirmations disagree", signed: []string{assertion("a1", confirmation(own)+confirmation(other), "")}},
		{name: "subject confirmation naming no request", signed: []string{assertion("a1", confirmation(own)+confirmation(`Recipient="x"`), "")}},
		{name: "subject confirmation without data", signed: []string{assertion("a1", confirmation(own)+`<saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer"/>`, "")}},
		{name: "one subject confirmation with conflicting data", signed: []string{assertion("a1", `<saml:SubjectConfirmation><saml:SubjectConfirmationData `+own+`/><saml:SubjectConfirmationData `+other+`/></saml:SubjectConfirmation>`, "")}},
		{name: "assertion without subject confirmation", signed: []string{assertion("a1", "", advice)}},
		{name: "another signed assertion names the request", signed: []string{assertion("a1", "", ""), assertion("a2", confirmation(own), "")}},
		{name: "returned assertion not signed", signed: []string{assertion("a2", confirmation(own), "")}},
		{name: "two signed assertions share the returned ID", signed: []string{assertion("a1", confirmation(own), ""), assertion("a1", confirmation(own), "")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := &samlSignedContent{}
			for _, doc := range tc.signed {
				content.elements = append(content.elements, parse(doc))
			}
			got, err := content.inResponseTo("a1")
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected a refusal, got %q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("inResponseTo() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// Signed content may not carry a prefixed attribute or namespace declaration
// named like an un-namespaced attribute crewjam/saml reads; the declarations
// and namespaced attributes real IdPs write are untouched.
func TestSAMLShadowingAttribute(t *testing.T) {
	for _, name := range []string{"ID", "InResponseTo", "IssueInstant", "Destination", "NotBefore", "NotOnOrAfter", "Recipient", "SessionIndex"} {
		if !samlBoundAttributeNames[name] {
			t.Fatalf("expected %s among the attributes crewjam/saml reads, got %v", name, samlBoundAttributeNames)
		}
	}
	for _, prefix := range []string{"saml", "samlp", "saml2", "saml2p", "ds", "dsig", "xs", "xsd", "xsi", "xenc", "ec", "md", "type"} {
		if samlBoundAttributeNames[prefix] {
			t.Fatalf("%q, a namespace prefix or namespaced attribute real IdPs write, is refused", prefix)
		}
	}

	const ns = `xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:ext="urn:example"`
	for _, tc := range []struct {
		doc, want string
	}{
		{doc: `<saml:Assertion ` + ns + ` ID="a1"><saml:Conditions xmlns:NotOnOrAfter="2999-01-01T00:00:00Z"/></saml:Assertion>`, want: "xmlns:NotOnOrAfter"},
		{doc: `<saml:Assertion ` + ns + ` ID="a1" ext:ID="a2"/>`, want: "ext:ID"},
		{doc: `<saml:Assertion ` + ns + ` ID="a1"><saml:AttributeStatement><saml:Attribute Name="mail"><saml:AttributeValue xsi:type="xs:string">a@example.com</saml:AttributeValue></saml:Attribute></saml:AttributeStatement></saml:Assertion>`},
	} {
		doc := etree.NewDocument()
		if err := doc.ReadFromString(tc.doc); err != nil {
			t.Fatalf("parse %s: %v", tc.doc, err)
		}
		if got := samlShadowingAttribute(doc.Root()); got != tc.want {
			t.Fatalf("samlShadowingAttribute(%s) = %q, want %q", tc.doc, got, tc.want)
		}
	}
}

func TestValidateSAMLRedirectTarget(t *testing.T) {
	allowed := []saml.Endpoint{
		{Location: "https://idp.example.com/sso"},
	}

	got, err := validateSAMLRedirectTarget("https://idp.example.com/sso?SAMLRequest=test", allowed)
	if err != nil {
		t.Fatalf("validate redirect: %v", err)
	}
	if !strings.Contains(got, "SAMLRequest=test") {
		t.Fatalf("unexpected validated redirect: %s", got)
	}

	if _, err := validateSAMLRedirectTarget("https://evil.example.com/sso?SAMLRequest=test", allowed); err == nil {
		t.Fatal("expected error for unexpected redirect target")
	}
}

func TestFetchMetadataFromURL(t *testing.T) {
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp-url">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"></IDPSSODescriptor>
</EntityDescriptor>`))
	}))
	defer server.Close()

	cfg := &config.SAMLProviderConfig{IDPMetadataURL: server.URL}
	service := &SAMLService{config: cfg, httpClient: newSAMLHTTPClient()}
	metadata, err := service.fetchIDPMetadataFromURL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if metadata.EntityID != "idp-url" {
		t.Fatalf("unexpected entity id: %s", metadata.EntityID)
	}
}
