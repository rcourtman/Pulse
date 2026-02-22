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

// SSOProviderType identifies the SSO provider implementation type.
type SSOProviderType string

const (
	SSOProviderTypeOIDC SSOProviderType = "oidc"
	SSOProviderTypeSAML SSOProviderType = "saml"
)

// OIDCProviderConfig contains OIDC provider settings.
type OIDCProviderConfig struct {
	IssuerURL       string   `json:"issuerUrl"`
	ClientID        string   `json:"clientId"`
	ClientSecret    string   `json:"clientSecret,omitempty"`
	RedirectURL     string   `json:"redirectUrl,omitempty"`
	LogoutURL       string   `json:"logoutUrl,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	UsernameClaim   string   `json:"usernameClaim,omitempty"`
	EmailClaim      string   `json:"emailClaim,omitempty"`
	CABundle        string   `json:"caBundle,omitempty"`
	ClientSecretSet bool     `json:"clientSecretSet,omitempty"`
}

// SAMLProviderConfig contains SAML provider settings.
type SAMLProviderConfig struct {
	IDPMetadataURL       string `json:"idpMetadataUrl,omitempty"`
	IDPMetadataXML       string `json:"idpMetadataXml,omitempty"`
	IDPSSOURL            string `json:"idpSsoUrl,omitempty"`
	IDPSLOURL            string `json:"idpSloUrl,omitempty"`
	IDPCertificate       string `json:"idpCertificate,omitempty"`
	IDPCertFile          string `json:"idpCertFile,omitempty"`
	IDPEntityID          string `json:"idpEntityId,omitempty"`
	IDPIssuer            string `json:"idpIssuer,omitempty"`
	SPEntityID           string `json:"spEntityId,omitempty"`
	SPACSPath            string `json:"spAcsPath,omitempty"`
	SPMetadataPath       string `json:"spMetadataPath,omitempty"`
	SPCertificate        string `json:"spCertificate,omitempty"`
	SPPrivateKey         string `json:"spPrivateKey,omitempty"`
	SPCertFile           string `json:"spCertFile,omitempty"`
	SPKeyFile            string `json:"spKeyFile,omitempty"`
	SignRequests         bool   `json:"signRequests,omitempty"`
	WantAssertionsSigned bool   `json:"wantAssertionsSigned,omitempty"`
	AllowUnencrypted     bool   `json:"allowUnencrypted,omitempty"`
	UsernameAttr         string `json:"usernameAttr,omitempty"`
	EmailAttr            string `json:"emailAttr,omitempty"`
	GroupsAttr           string `json:"groupsAttr,omitempty"`
	FirstNameAttr        string `json:"firstNameAttr,omitempty"`
	LastNameAttr         string `json:"lastNameAttr,omitempty"`
	NameIDFormat         string `json:"nameIdFormat,omitempty"`
	ForceAuthn           bool   `json:"forceAuthn,omitempty"`
	AllowIDPInitiated    bool   `json:"allowIdpInitiated,omitempty"`
	RelayStateTemplate   string `json:"relayStateTemplate,omitempty"`
}

// SSOProvider describes a configured provider.
type SSOProvider struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Type              SSOProviderType     `json:"type"`
	Enabled           bool                `json:"enabled"`
	DisplayName       string              `json:"displayName,omitempty"`
	IconURL           string              `json:"iconUrl,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	AllowedGroups     []string            `json:"allowedGroups,omitempty"`
	AllowedDomains    []string            `json:"allowedDomains,omitempty"`
	AllowedEmails     []string            `json:"allowedEmails,omitempty"`
	GroupsClaim       string              `json:"groupsClaim,omitempty"`
	GroupRoleMappings map[string]string   `json:"groupRoleMappings,omitempty"`
	OIDC              *OIDCProviderConfig `json:"oidc,omitempty"`
	SAML              *SAMLProviderConfig `json:"saml,omitempty"`
}

// SSOConfigSnapshot is the runtime snapshot used by enterprise handlers.
type SSOConfigSnapshot struct {
	Providers              []SSOProvider `json:"providers,omitempty"`
	DefaultProviderID      string        `json:"defaultProviderId,omitempty"`
	AllowMultipleProviders bool          `json:"allowMultipleProviders,omitempty"`
}

// SSOProviderResponse is the API response shape for SSO providers.
type SSOProviderResponse struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Enabled             bool     `json:"enabled"`
	DisplayName         string   `json:"displayName,omitempty"`
	IconURL             string   `json:"iconUrl,omitempty"`
	Priority            int      `json:"priority"`
	OIDCIssuerURL       string   `json:"oidcIssuerUrl,omitempty"`
	OIDCClientID        string   `json:"oidcClientId,omitempty"`
	OIDCClientSecretSet bool     `json:"oidcClientSecretSet,omitempty"`
	OIDCLoginURL        string   `json:"oidcLoginUrl,omitempty"`
	OIDCCallbackURL     string   `json:"oidcCallbackUrl,omitempty"`
	SAMLIDPEntityID     string   `json:"samlIdpEntityId,omitempty"`
	SAMLSPEntityID      string   `json:"samlSpEntityId,omitempty"`
	SAMLMetadataURL     string   `json:"samlMetadataUrl,omitempty"`
	SAMLACSURL          string   `json:"samlAcsUrl,omitempty"`
	AllowedGroups       []string `json:"allowedGroups,omitempty"`
	AllowedDomains      []string `json:"allowedDomains,omitempty"`
	AllowedEmails       []string `json:"allowedEmails,omitempty"`
}

// SSOProvidersListResponse is the API response shape for list requests.
type SSOProvidersListResponse struct {
	Providers              []SSOProviderResponse `json:"providers"`
	DefaultProviderID      string                `json:"defaultProviderId,omitempty"`
	AllowMultipleProviders bool                  `json:"allowMultipleProviders"`
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
	GetClientIP            func(*http.Request) string
	AllowAuthRequest       func(clientIP string) bool
	TestSAMLConnection     func(context.Context, *SAMLTestConfig) SSOTestResponse
	TestOIDCConnection     func(context.Context, *OIDCTestConfig) SSOTestResponse
	PreviewSAMLMetadata    func(context.Context, MetadataPreviewRequest) (MetadataPreviewResponse, error)
	IsValidProviderID      func(string) bool
	GetSSOConfigSnapshot   func() SSOConfigSnapshot
	SaveSSOConfigSnapshot  func(SSOConfigSnapshot) error
	GetPublicURL           func() string
	RequireFeature         func(context.Context, string) error
	WriteLicenseRequired   func(http.ResponseWriter, string, string)
	InitializeSAMLProvider func(context.Context, string, *SAMLProviderConfig) error
	RemoveSAMLProvider     func(string)
	InitializeOIDCProvider func(context.Context, string, *SSOProvider) error
	RemoveOIDCProvider     func(string)
	LogAuditEvent          LogSSOAuditEventFunc
	WriteError             WriteSSOErrorFunc
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
