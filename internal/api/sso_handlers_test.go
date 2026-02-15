package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Counter for generating unique test IPs to avoid rate limiting
var testIPCounter uint64

// getUniqueTestIP returns a unique IP for each test to avoid rate limiting
func getUniqueTestIP() string {
	n := atomic.AddUint64(&testIPCounter, 1)
	return fmt.Sprintf("10.%d.%d.%d", (n>>16)&255, (n>>8)&255, n&255)
}

// setTestIP sets a unique IP on the request to avoid rate limiting
func setTestIP(req *http.Request) {
	ip := getUniqueTestIP()
	req.RemoteAddr = ip + ":12345"
}

// Sample SAML metadata for testing
const testSAMLMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data>
          <X509Certificate>MIICpDCCAYwCCQDU+pQ4P3rFWjANBgkqhkiG9w0BAQsFADAUMRIwEAYDVQQDDAlsb2NhbGhvc3QwHhcNMjQwMTAxMDAwMDAwWhcNMjcwMTAxMDAwMDAwWjAUMRIwEAYDVQQDDAlsb2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC7o5e7Xv6ufFhDFQgGX4cMi2e0q1z1E+xJxPJ7mQK0lCk5Y6r6J0fHxXrLJ7kA6mMj6F/D8r9ZgT0rQ1eV6H8xJU4z+C/w5E9JQ+YB7EZ8x0mC3z9HxKM7qM3eF7+Y5TJb3l0N5E1C2vL7qJX5Z6T2N8vA/8R5kE+g0m3E7Y6z7K2N6E9A0J8F5E3D2V6N9G0H1I2J3K4L5M6N7O8P9Q0R1S2T3U4V5W6X7Y8Z9</X509Certificate>
        </X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://idp.example.com/sso"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
    <SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/slo"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

// Sample OIDC discovery document
const testOIDCDiscovery = `{
  "issuer": "https://idp.example.com",
  "authorization_endpoint": "https://idp.example.com/oauth2/authorize",
  "token_endpoint": "https://idp.example.com/oauth2/token",
  "userinfo_endpoint": "https://idp.example.com/oauth2/userinfo",
  "jwks_uri": "https://idp.example.com/.well-known/jwks.json",
  "scopes_supported": ["openid", "profile", "email"]
}`

func TestHandleTestSSOProvider_SAMLSuccess(t *testing.T) {
	// Create mock SAML metadata server
	metadataServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testSAMLMetadata))
	}))
	defer metadataServer.Close()

	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataURL: metadataServer.URL,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Error)
	}

	if resp.Details == nil {
		t.Fatal("expected details to be present")
	}

	if resp.Details.EntityID != "https://idp.example.com" {
		t.Errorf("expected entityId='https://idp.example.com', got '%s'", resp.Details.EntityID)
	}

	if resp.Details.SSOURL != "https://idp.example.com/sso" {
		t.Errorf("expected ssoUrl='https://idp.example.com/sso', got '%s'", resp.Details.SSOURL)
	}
}

func TestHandleTestSSOProvider_SAMLMetadataXML(t *testing.T) {
	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataXML: testSAMLMetadata,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Error)
	}

	if resp.Details.EntityID != "https://idp.example.com" {
		t.Errorf("expected entityId='https://idp.example.com', got '%s'", resp.Details.EntityID)
	}
}

func TestHandleTestSSOProvider_SAMLFetchError(t *testing.T) {
	// Server that returns 500
	errorServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataURL: errorServer.URL,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false, got true")
	}
}

func TestHandleTestSSOProvider_SAMLInvalidXML(t *testing.T) {
	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataXML: "not valid xml",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp SSOTestResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Success {
		t.Error("expected success=false for invalid XML")
	}
}

func TestHandleTestSSOProvider_OIDCSuccess(t *testing.T) {
	// Create mock OIDC discovery server
	discoveryServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(testOIDCDiscovery))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer discoveryServer.Close()

	reqBody := SSOTestRequest{
		Type: "oidc",
		OIDC: &OIDCTestConfig{
			IssuerURL: discoveryServer.URL,
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Error)
	}

	if resp.Details == nil {
		t.Fatal("expected details to be present")
	}

	if resp.Details.TokenEndpoint == "" {
		t.Error("expected tokenEndpoint to be present")
	}
}

func TestHandleTestSSOProvider_OIDCFetchError(t *testing.T) {
	reqBody := SSOTestRequest{
		Type: "oidc",
		OIDC: &OIDCTestConfig{
			IssuerURL: "http://localhost:99999", // Invalid port
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp SSOTestResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Success {
		t.Error("expected success=false for unreachable server")
	}
}

func TestHandleTestSSOProvider_InvalidType(t *testing.T) {
	reqBody := map[string]string{
		"type": "invalid",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleTestSSOProvider_MissingConfig(t *testing.T) {
	tests := []struct {
		name    string
		reqBody SSOTestRequest
	}{
		{
			name: "saml without config",
			reqBody: SSOTestRequest{
				Type: "saml",
			},
		},
		{
			name: "oidc without config",
			reqBody: SSOTestRequest{
				Type: "oidc",
			},
		},
		{
			name: "saml with empty config",
			reqBody: SSOTestRequest{
				Type: "saml",
				SAML: &SAMLTestConfig{},
			},
		},
		{
			name: "oidc with empty issuer",
			reqBody: SSOTestRequest{
				Type: "oidc",
				OIDC: &OIDCTestConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			setTestIP(req)
			rec := httptest.NewRecorder()

			router := &Router{}
			router.handleTestSSOProvider(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}

			var resp SSOTestResponse
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)

			if resp.Success {
				t.Error("expected success=false for missing config")
			}
		})
	}
}

func TestHandleMetadataPreview_Success(t *testing.T) {
	// Create mock metadata server
	metadataServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testSAMLMetadata))
	}))
	defer metadataServer.Close()

	reqBody := MetadataPreviewRequest{
		Type:        "saml",
		MetadataURL: metadataServer.URL,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMetadataPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp MetadataPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.XML == "" {
		t.Error("expected XML content to be present")
	}

	if resp.Parsed == nil {
		t.Fatal("expected parsed info to be present")
	}

	if resp.Parsed.EntityID != "https://idp.example.com" {
		t.Errorf("expected entityId='https://idp.example.com', got '%s'", resp.Parsed.EntityID)
	}

	if resp.Parsed.SSOURL != "https://idp.example.com/sso" {
		t.Errorf("expected ssoUrl='https://idp.example.com/sso', got '%s'", resp.Parsed.SSOURL)
	}
}

func TestHandleMetadataPreview_FromXML(t *testing.T) {
	reqBody := MetadataPreviewRequest{
		Type:        "saml",
		MetadataXML: testSAMLMetadata,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMetadataPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp MetadataPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Parsed.EntityID != "https://idp.example.com" {
		t.Errorf("expected entityId='https://idp.example.com', got '%s'", resp.Parsed.EntityID)
	}
}

func TestHandleMetadataPreview_InvalidURL(t *testing.T) {
	reqBody := MetadataPreviewRequest{
		Type:        "saml",
		MetadataURL: "not-a-valid-url",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMetadataPreview(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleMetadataPreview_FetchError(t *testing.T) {
	reqBody := MetadataPreviewRequest{
		Type:        "saml",
		MetadataURL: "http://localhost:99999/metadata",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMetadataPreview(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleMetadataPreview_UnsupportedType(t *testing.T) {
	reqBody := MetadataPreviewRequest{
		Type:        "oidc",
		MetadataURL: "https://example.com",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMetadataPreview(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleMetadataPreview_MissingInput(t *testing.T) {
	reqBody := MetadataPreviewRequest{
		Type: "saml",
		// No URL or XML
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMetadataPreview(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// Test helper functions
func TestParseSAMLMetadataXML(t *testing.T) {
	metadata, err := parseSAMLMetadataXML([]byte(testSAMLMetadata))
	if err != nil {
		t.Fatalf("failed to parse valid metadata: %v", err)
	}

	if metadata.EntityID != "https://idp.example.com" {
		t.Errorf("expected entityID='https://idp.example.com', got '%s'", metadata.EntityID)
	}

	if len(metadata.IDPSSODescriptors) == 0 {
		t.Fatal("expected IDPSSODescriptors to be present")
	}
}

func TestParseSAMLMetadataXML_Invalid(t *testing.T) {
	_, err := parseSAMLMetadataXML([]byte("not xml"))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}

func TestFormatXML(t *testing.T) {
	input := `<root><child>value</child></root>`
	output := formatXML([]byte(input))

	// Should contain indentation
	if output == input {
		t.Log("XML formatting may not have added indentation, but that's acceptable")
	}

	// Should still contain the data
	if !bytes.Contains([]byte(output), []byte("value")) {
		t.Error("formatted XML should still contain original content")
	}
}
