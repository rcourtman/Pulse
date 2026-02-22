package extensions

import (
	"context"
	"net/http"
	"time"
)

// SSOAdminEndpoints defines the enterprise SSO admin endpoint surface.
type SSOAdminEndpoints interface {
	HandleProvidersCollection(http.ResponseWriter, *http.Request)
	HandleProviderItem(http.ResponseWriter, *http.Request)
	HandleProviderTest(http.ResponseWriter, *http.Request)
	HandleMetadataPreview(http.ResponseWriter, *http.Request)
}

// SSOTestRequest represents a request to test SSO provider configuration.
type SSOTestRequest struct {
	Type string          `json:"type"` // "saml" or "oidc"
	SAML *SAMLTestConfig `json:"saml,omitempty"`
	OIDC *OIDCTestConfig `json:"oidc,omitempty"`
}

// SAMLTestConfig contains SAML configuration to test.
type SAMLTestConfig struct {
	IDPMetadataURL string `json:"idpMetadataUrl,omitempty"`
	IDPMetadataXML string `json:"idpMetadataXml,omitempty"`
	IDPSSOURL      string `json:"idpSsoUrl,omitempty"`
	IDPCertificate string `json:"idpCertificate,omitempty"`
}

// OIDCTestConfig contains OIDC configuration to test.
type OIDCTestConfig struct {
	IssuerURL string `json:"issuerUrl"`
	ClientID  string `json:"clientId,omitempty"`
}

// SSOTestResponse represents the result of a connection test.
type SSOTestResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   string          `json:"error,omitempty"`
	Details *SSOTestDetails `json:"details,omitempty"`
}

// MetadataPreviewRequest represents a request to preview IdP metadata.
type MetadataPreviewRequest struct {
	Type        string `json:"type"` // "saml"
	MetadataURL string `json:"metadataUrl,omitempty"`
	MetadataXML string `json:"metadataXml,omitempty"`
}

// MetadataPreviewResponse contains metadata preview information.
type MetadataPreviewResponse struct {
	XML    string              `json:"xml"`
	Parsed *ParsedMetadataInfo `json:"parsed"`
}

// ParsedMetadataInfo contains parsed metadata details.
type ParsedMetadataInfo struct {
	EntityID      string            `json:"entityId"`
	SSOURL        string            `json:"ssoUrl,omitempty"`
	SLOURL        string            `json:"sloUrl,omitempty"`
	Certificates  []CertificateInfo `json:"certificates,omitempty"`
	NameIDFormats []string          `json:"nameIdFormats,omitempty"`
}

// SSOTestDetails contains detailed information about the tested provider.
type SSOTestDetails struct {
	Type             string            `json:"type"`
	EntityID         string            `json:"entityId,omitempty"`
	SSOURL           string            `json:"ssoUrl,omitempty"`
	SLOURL           string            `json:"sloUrl,omitempty"`
	Certificates     []CertificateInfo `json:"certificates,omitempty"`
	TokenEndpoint    string            `json:"tokenEndpoint,omitempty"`
	UserinfoEndpoint string            `json:"userinfoEndpoint,omitempty"`
	JWKSURI          string            `json:"jwksUri,omitempty"`
	SupportedScopes  []string          `json:"supportedScopes,omitempty"`
}

// CertificateInfo contains certificate details returned from SAML/OIDC tests.
type CertificateInfo struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"notBefore"`
	NotAfter  time.Time `json:"notAfter"`
	IsExpired bool      `json:"isExpired"`
}

// WriteSSOErrorFunc writes a structured SSO error response.
type WriteSSOErrorFunc func(http.ResponseWriter, int, string, string, map[string]string)

// LogSSOAuditEventFunc records an SSO admin audit event.
type LogSSOAuditEventFunc func(ctx context.Context, event, path string, success bool, message, clientIP string)

// SSOAdminRuntime exposes runtime capabilities needed by SSO admin endpoints.
type SSOAdminRuntime struct {
	GetClientIP          func(*http.Request) string
	AllowAuthRequest     func(clientIP string) bool
	TestSAMLConnection   func(context.Context, *SAMLTestConfig) SSOTestResponse
	TestOIDCConnection   func(context.Context, *OIDCTestConfig) SSOTestResponse
	PreviewSAMLMetadata  func(context.Context, MetadataPreviewRequest) (MetadataPreviewResponse, error)
	IsValidProviderID    func(string) bool
	HandleListProviders  func(http.ResponseWriter, *http.Request)
	HandleCreateProvider func(http.ResponseWriter, *http.Request)
	HandleGetProvider    func(http.ResponseWriter, *http.Request, string)
	HandleUpdateProvider func(http.ResponseWriter, *http.Request, string)
	HandleDeleteProvider func(http.ResponseWriter, *http.Request, string)
	LogAuditEvent        LogSSOAuditEventFunc
	WriteError           WriteSSOErrorFunc
}

// MetadataPreviewError provides a structured error for metadata preview operations.
type MetadataPreviewError struct {
	Code    string
	Message string
}

func (e *MetadataPreviewError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// BindSSOAdminEndpointsFunc allows enterprise modules to bind replacement
// SSO admin endpoints while retaining access to default handlers.
type BindSSOAdminEndpointsFunc func(defaults SSOAdminEndpoints, runtime SSOAdminRuntime) SSOAdminEndpoints
