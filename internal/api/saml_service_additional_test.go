package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestParseIDPMetadataXML_EmptyEntities(t *testing.T) {
	wrapped := `<?xml version="1.0"?>
<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">
</EntitiesDescriptor>`
	if _, err := parseIDPMetadataXML([]byte(wrapped)); err == nil {
		t.Fatal("expected error for empty entities descriptor")
	}
}

func TestExtractAttribute(t *testing.T) {
	service := &SAMLService{}
	attrs := map[string][]string{
		"email": {"user@example.com"},
	}

	if got := service.extractAttribute(attrs, "", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}
	if got := service.extractAttribute(attrs, "email", ""); got != "user@example.com" {
		t.Fatalf("unexpected attribute value: %q", got)
	}
	if got := service.extractAttribute(attrs, "missing", "default"); got != "default" {
		t.Fatalf("unexpected missing attribute value: %q", got)
	}
}

func TestProcessResponse_InvalidResponse(t *testing.T) {
	service := &SAMLService{
		sp: &saml.ServiceProvider{},
	}

	body := strings.NewReader("RelayState=/dashboard&SAMLResponse=")
	req := httptest.NewRequest(http.MethodPost, "/acs", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, relay, err := service.ProcessResponse(req)
	if err == nil {
		t.Fatal("expected error for invalid response")
	}
	if relay != "/dashboard" {
		t.Fatalf("unexpected relay state: %q", relay)
	}
}

func TestSAMLServiceIdentifiers(t *testing.T) {
	service := &SAMLService{
		providerID: "provider-1",
		sp:         &saml.ServiceProvider{EntityID: "sp-entity"},
		idpMetadata: &saml.EntityDescriptor{
			EntityID: "idp-entity",
		},
	}

	if service.ProviderID() != "provider-1" {
		t.Fatalf("unexpected provider id")
	}
	if service.GetSPEntityID() != "sp-entity" {
		t.Fatalf("unexpected sp entity id")
	}
	if service.GetIDPEntityID() != "idp-entity" {
		t.Fatalf("unexpected idp entity id")
	}

	service.sp = nil
	if service.GetSPEntityID() != "" {
		t.Fatalf("expected empty sp entity id when nil")
	}
	service.idpMetadata = nil
	if service.GetIDPEntityID() != "" {
		t.Fatalf("expected empty idp entity id when nil")
	}
}

func TestRefreshMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="idp-refresh">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"></IDPSSODescriptor>
</EntityDescriptor>`))
	}))
	defer server.Close()

	cfg := &config.SAMLProviderConfig{IDPMetadataURL: server.URL}
	service := &SAMLService{
		providerID: "idp",
		config:     cfg,
		baseURL:    "http://localhost",
		httpClient: server.Client(),
	}

	if err := service.RefreshMetadata(context.Background()); err != nil {
		t.Fatalf("refresh metadata: %v", err)
	}
	if service.sp == nil {
		t.Fatal("expected service provider to be initialized")
	}
}

func TestFetchIDPMetadataFromURL_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	service := &SAMLService{httpClient: server.Client()}
	if _, err := service.fetchIDPMetadataFromURL(context.Background(), server.URL); err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestAddIDPCertificate_InvalidPEM(t *testing.T) {
	service := &SAMLService{config: &config.SAMLProviderConfig{IDPCertificate: "not-pem"}}
	metadata := &saml.EntityDescriptor{
		IDPSSODescriptors: []saml.IDPSSODescriptor{{}},
	}
	if err := service.addIDPCertificate(metadata); err == nil {
		t.Fatal("expected error for invalid certificate pem")
	}
}
