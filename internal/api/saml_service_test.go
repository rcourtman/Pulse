package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

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

	url, err := service.MakeAuthRequest("")
	if err != nil || !strings.Contains(url, "SAMLRequest") {
		t.Fatalf("unexpected auth url: %v %s", err, url)
	}

	if _, err := service.GetMetadata(); err != nil {
		t.Fatalf("metadata error: %v", err)
	}

	logoutURL, err := service.MakeLogoutRequest("user", "sess")
	if err != nil || !strings.Contains(logoutURL, "SAMLRequest") {
		t.Fatalf("unexpected logout url: %v %s", err, logoutURL)
	}

	service = &SAMLService{config: &config.SAMLProviderConfig{}}
	if _, err := service.MakeAuthRequest(""); err == nil {
		t.Fatal("expected error when sp missing")
	}
	if _, err := service.GetMetadata(); err == nil {
		t.Fatal("expected error when sp missing")
	}
	if _, err := service.MakeLogoutRequest("user", "sess"); err == nil {
		t.Fatal("expected error when sp missing")
	}
	if err := service.RefreshMetadata(context.Background()); err == nil {
		t.Fatal("expected refresh error without url")
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
