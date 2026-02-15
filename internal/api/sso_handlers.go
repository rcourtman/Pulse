package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rs/zerolog/log"
)

// Security constants for SSO
const (
	maxProviderIDLength   = 64
	maxProviderNameLength = 128
	maxURLLength          = 2048
	maxRequestBodySize    = 1 << 20 // 1MB
)

// providerIDRegex validates provider IDs (alphanumeric, hyphens, underscores)
var providerIDRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}[a-zA-Z0-9]?$`)

// validateProviderID checks if a provider ID is safe and valid
func validateProviderID(id string) bool {
	if id == "" || len(id) > maxProviderIDLength {
		return false
	}
	return providerIDRegex.MatchString(id)
}

// sanitizeProviderName sanitizes a provider name
func sanitizeProviderName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > maxProviderNameLength {
		name = name[:maxProviderNameLength]
	}
	// Remove control characters
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	return name
}

// validateURL checks if a URL is valid and uses an allowed scheme
func validateURL(urlStr string, allowedSchemes []string) bool {
	if urlStr == "" || len(urlStr) > maxURLLength {
		return false
	}
	parsed, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false
	}
	for _, scheme := range allowedSchemes {
		if strings.EqualFold(parsed.Scheme, scheme) {
			return true
		}
	}
	return false
}

// SSOProviderResponse represents an SSO provider for API responses
type SSOProviderResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Enabled     bool   `json:"enabled"`
	DisplayName string `json:"displayName,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	Priority    int    `json:"priority"`

	// OIDC-specific (only present for OIDC providers)
	OIDCIssuerURL       string `json:"oidcIssuerUrl,omitempty"`
	OIDCClientID        string `json:"oidcClientId,omitempty"`
	OIDCClientSecretSet bool   `json:"oidcClientSecretSet,omitempty"`
	OIDCLoginURL        string `json:"oidcLoginUrl,omitempty"`
	OIDCCallbackURL     string `json:"oidcCallbackUrl,omitempty"`

	// SAML-specific (only present for SAML providers)
	SAMLIDPEntityID string `json:"samlIdpEntityId,omitempty"`
	SAMLSPEntityID  string `json:"samlSpEntityId,omitempty"`
	SAMLMetadataURL string `json:"samlMetadataUrl,omitempty"`
	SAMLACSURL      string `json:"samlAcsUrl,omitempty"`

	// Common restrictions
	AllowedGroups  []string `json:"allowedGroups,omitempty"`
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	AllowedEmails  []string `json:"allowedEmails,omitempty"`
}

// SSOProvidersListResponse represents the list of SSO providers
type SSOProvidersListResponse struct {
	Providers              []SSOProviderResponse `json:"providers"`
	DefaultProviderID      string                `json:"defaultProviderId,omitempty"`
	AllowMultipleProviders bool                  `json:"allowMultipleProviders"`
}

// handleSSOProviders handles listing and creating SSO providers
func (r *Router) handleSSOProviders(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.handleListSSOProviders(w, req)
	case http.MethodPost:
		r.handleCreateSSOProvider(w, req)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
	}
}

// handleSSOProvider handles getting, updating, and deleting a specific SSO provider
func (r *Router) handleSSOProvider(w http.ResponseWriter, req *http.Request) {
	// Extract provider ID from path: /api/security/sso/providers/{id}
	providerID := strings.TrimPrefix(req.URL.Path, "/api/security/sso/providers/")
	providerID = strings.TrimSuffix(providerID, "/")

	if providerID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Provider ID is required", nil)
		return
	}

	// Security: Validate provider ID format to prevent injection attacks
	if !validateProviderID(providerID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid provider ID format", nil)
		return
	}

	switch req.Method {
	case http.MethodGet:
		r.handleGetSSOProvider(w, req, providerID)
	case http.MethodPut:
		r.handleUpdateSSOProvider(w, req, providerID)
	case http.MethodDelete:
		r.handleDeleteSSOProvider(w, req, providerID)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
	}
}

// ensureSSOConfig loads the SSO configuration from persistence on first access,
// falling back to an empty config if nothing is persisted.
func (r *Router) ensureSSOConfig() *config.SSOConfig {
	if r.ssoConfig == nil {
		if r.persistence != nil {
			cfg, err := r.persistence.LoadSSOConfig()
			if err != nil {
				log.Error().Err(err).Msg("Failed to load SSO config from persistence")
			}
			if cfg != nil {
				r.ssoConfig = cfg
				return r.ssoConfig
			}
		}
		r.ssoConfig = config.NewSSOConfig()
	}
	return r.ssoConfig
}

func (r *Router) handleListSSOProviders(w http.ResponseWriter, req *http.Request) {
	r.ensureSSOConfig()

	response := SSOProvidersListResponse{
		Providers:              make([]SSOProviderResponse, 0),
		DefaultProviderID:      r.ssoConfig.DefaultProviderID,
		AllowMultipleProviders: r.ssoConfig.AllowMultipleProviders,
	}

	for _, p := range r.ssoConfig.Providers {
		response.Providers = append(response.Providers, providerToResponse(&p, r.config.PublicURL))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (r *Router) handleGetSSOProvider(w http.ResponseWriter, req *http.Request, providerID string) {
	r.ensureSSOConfig()

	provider := r.ssoConfig.GetProvider(providerID)
	if provider == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Provider not found", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerToResponse(provider, r.config.PublicURL))
}

func (r *Router) handleCreateSSOProvider(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(io.LimitReader(req.Body, maxRequestBodySize))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "read_error", "Failed to read request body", nil)
		return
	}

	var provider config.SSOProvider
	if err := json.Unmarshal(body, &provider); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload", nil)
		return
	}

	// Generate ID if not provided
	if provider.ID == "" {
		provider.ID = uuid.NewString()
	}

	// Security: Validate provider ID format
	if !validateProviderID(provider.ID) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid provider ID format", nil)
		return
	}

	// Sanitize provider name
	provider.Name = sanitizeProviderName(provider.Name)
	provider.DisplayName = sanitizeProviderName(provider.DisplayName)

	// Validate provider
	if provider.Name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Provider name is required", nil)
		return
	}

	if provider.Type != config.SSOProviderTypeOIDC && provider.Type != config.SSOProviderTypeSAML {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Provider type must be 'oidc' or 'saml'", nil)
		return
	}

	// SAML requires Advanced SSO license (OIDC is free)
	if provider.Type == config.SSOProviderTypeSAML {
		svc := r.licenseHandlers.Service(req.Context())
		if err := svc.RequireFeature(license.FeatureAdvancedSSO); err != nil {
			WriteLicenseRequired(w, license.FeatureAdvancedSSO, "SAML SSO requires a Pro license. Basic OIDC SSO is available on all tiers.")
			return
		}
	}

	// Security: Validate OIDC configuration
	if provider.Type == config.SSOProviderTypeOIDC {
		if provider.OIDC == nil {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "OIDC configuration is required", nil)
			return
		}
		if provider.OIDC.IssuerURL != "" && !validateURL(provider.OIDC.IssuerURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid OIDC issuer URL", nil)
			return
		}
		if provider.OIDC.RedirectURL != "" && !validateURL(provider.OIDC.RedirectURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid OIDC redirect URL", nil)
			return
		}
	}

	// Security: Validate SAML configuration
	if provider.Type == config.SSOProviderTypeSAML {
		if provider.SAML == nil {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "SAML configuration is required", nil)
			return
		}
		if provider.SAML.IDPMetadataURL != "" && !validateURL(provider.SAML.IDPMetadataURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid SAML metadata URL", nil)
			return
		}
		if provider.SAML.IDPSSOURL != "" && !validateURL(provider.SAML.IDPSSOURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid SAML SSO URL", nil)
			return
		}
	}

	// Security: Validate icon URL if provided
	if provider.IconURL != "" && !validateURL(provider.IconURL, []string{"https", "http", "data"}) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid icon URL", nil)
		return
	}

	r.ensureSSOConfig()

	// Check for duplicate ID
	if r.ssoConfig.GetProvider(provider.ID) != nil {
		writeErrorResponse(w, http.StatusConflict, "duplicate_id", "Provider with this ID already exists", nil)
		return
	}

	// Add provider
	if err := r.ssoConfig.AddProvider(provider); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "add_error", err.Error(), nil)
		return
	}

	// Persist configuration
	if err := r.saveSSOConfig(); err != nil {
		log.Error().Err(err).Msg("Failed to persist SSO configuration")
		// Remove the provider we just added since persistence failed
		if err := r.ssoConfig.RemoveProvider(provider.ID); err != nil {
			log.Warn().Err(err).Str("provider_id", provider.ID).Msg("Failed to remove provider during rollback")
		}
		writeErrorResponse(w, http.StatusInternalServerError, "save_error", "Failed to save configuration", nil)
		return
	}

	// Initialize SAML provider if applicable
	if provider.Type == config.SSOProviderTypeSAML && provider.Enabled && provider.SAML != nil {
		if err := r.samlManager.InitializeProvider(req.Context(), provider.ID, provider.SAML); err != nil {
			log.Warn().Err(err).Str("provider_id", provider.ID).Msg("Failed to initialize SAML provider (will retry on first use)")
		}
	}

	// Initialize OIDC provider if applicable
	if provider.Type == config.SSOProviderTypeOIDC && provider.Enabled && provider.OIDC != nil {
		if err := r.oidcManager.InitializeProvider(req.Context(), provider.ID, &provider, ""); err != nil {
			log.Warn().Err(err).Str("provider_id", provider.ID).Msg("Failed to initialize OIDC provider (will retry on first use)")
		}
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "sso_provider_created", "", GetClientIP(req), req.URL.Path, true, "Created provider: "+provider.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(providerToResponse(&provider, r.config.PublicURL))
}

func (r *Router) handleUpdateSSOProvider(w http.ResponseWriter, req *http.Request, providerID string) {
	r.ensureSSOConfig()

	existing := r.ssoConfig.GetProvider(providerID)
	if existing == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Provider not found", nil)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxRequestBodySize))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "read_error", "Failed to read request body", nil)
		return
	}

	var updated config.SSOProvider
	if err := json.Unmarshal(body, &updated); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload", nil)
		return
	}

	// Ensure ID matches
	updated.ID = providerID

	// Sanitize inputs
	updated.Name = sanitizeProviderName(updated.Name)
	updated.DisplayName = sanitizeProviderName(updated.DisplayName)

	if updated.Name == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Provider name is required", nil)
		return
	}

	// SAML requires Advanced SSO license (OIDC is free)
	if updated.Type == config.SSOProviderTypeSAML {
		svc := r.licenseHandlers.Service(req.Context())
		if err := svc.RequireFeature(license.FeatureAdvancedSSO); err != nil {
			WriteLicenseRequired(w, license.FeatureAdvancedSSO, "SAML SSO requires a Pro license. Basic OIDC SSO is available on all tiers.")
			return
		}
	}

	// Security: Validate URLs for OIDC
	if updated.Type == config.SSOProviderTypeOIDC && updated.OIDC != nil {
		if updated.OIDC.IssuerURL != "" && !validateURL(updated.OIDC.IssuerURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid OIDC issuer URL", nil)
			return
		}
		if updated.OIDC.RedirectURL != "" && !validateURL(updated.OIDC.RedirectURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid OIDC redirect URL", nil)
			return
		}
	}

	// Security: Validate URLs for SAML
	if updated.Type == config.SSOProviderTypeSAML && updated.SAML != nil {
		if updated.SAML.IDPMetadataURL != "" && !validateURL(updated.SAML.IDPMetadataURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid SAML metadata URL", nil)
			return
		}
		if updated.SAML.IDPSSOURL != "" && !validateURL(updated.SAML.IDPSSOURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid SAML SSO URL", nil)
			return
		}
	}

	// Security: Validate icon URL if provided
	if updated.IconURL != "" && !validateURL(updated.IconURL, []string{"https", "http", "data"}) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid icon URL", nil)
		return
	}

	// Preserve existing config when not provided in update (e.g., toggle enabled/disabled
	// sends the flat list response format, which doesn't include nested OIDC/SAML objects)
	if updated.Type == config.SSOProviderTypeOIDC && updated.OIDC == nil && existing.OIDC != nil {
		updated.OIDC = existing.OIDC
	}
	if updated.Type == config.SSOProviderTypeSAML && updated.SAML == nil && existing.SAML != nil {
		updated.SAML = existing.SAML
	}
	if updated.GroupsClaim == "" && existing.GroupsClaim != "" {
		updated.GroupsClaim = existing.GroupsClaim
	}
	if len(updated.GroupRoleMappings) == 0 && len(existing.GroupRoleMappings) > 0 {
		updated.GroupRoleMappings = existing.GroupRoleMappings
	}
	if len(updated.AllowedGroups) == 0 && len(existing.AllowedGroups) > 0 {
		updated.AllowedGroups = existing.AllowedGroups
	}
	if len(updated.AllowedDomains) == 0 && len(existing.AllowedDomains) > 0 {
		updated.AllowedDomains = existing.AllowedDomains
	}
	if len(updated.AllowedEmails) == 0 && len(existing.AllowedEmails) > 0 {
		updated.AllowedEmails = existing.AllowedEmails
	}

	// Preserve secrets if not provided in update
	if updated.Type == config.SSOProviderTypeOIDC && updated.OIDC != nil && existing.OIDC != nil {
		if updated.OIDC.ClientSecret == "" && existing.OIDC.ClientSecretSet {
			updated.OIDC.ClientSecret = existing.OIDC.ClientSecret
			updated.OIDC.ClientSecretSet = true
		}
	}

	// Update provider
	if err := r.ssoConfig.UpdateProvider(updated); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "update_error", err.Error(), nil)
		return
	}

	// Persist configuration
	if err := r.saveSSOConfig(); err != nil {
		log.Error().Err(err).Msg("Failed to persist SSO configuration")
		// Revert to existing provider
		if err := r.ssoConfig.UpdateProvider(*existing); err != nil {
			log.Warn().Err(err).Str("provider_id", existing.ID).Msg("Failed to revert provider during rollback")
		}
		writeErrorResponse(w, http.StatusInternalServerError, "save_error", "Failed to save configuration", nil)
		return
	}

	// Clean up stale services when the provider type changed
	if existing.Type != updated.Type {
		if existing.Type == config.SSOProviderTypeSAML {
			r.samlManager.RemoveProvider(updated.ID)
		}
		if existing.Type == config.SSOProviderTypeOIDC {
			r.oidcManager.RemoveService(updated.ID)
		}
	}

	// Re-initialize SAML provider if applicable
	if updated.Type == config.SSOProviderTypeSAML && updated.SAML != nil {
		if updated.Enabled {
			if err := r.samlManager.InitializeProvider(req.Context(), updated.ID, updated.SAML); err != nil {
				log.Warn().Err(err).Str("provider_id", updated.ID).Msg("Failed to re-initialize SAML provider")
			}
		} else {
			r.samlManager.RemoveProvider(updated.ID)
		}
	}

	// Re-initialize OIDC provider if applicable
	if updated.Type == config.SSOProviderTypeOIDC {
		if updated.Enabled && updated.OIDC != nil {
			if err := r.oidcManager.InitializeProvider(req.Context(), updated.ID, &updated, ""); err != nil {
				log.Warn().Err(err).Str("provider_id", updated.ID).Msg("Failed to re-initialize OIDC provider")
			}
		} else {
			r.oidcManager.RemoveService(updated.ID)
		}
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "sso_provider_updated", "", GetClientIP(req), req.URL.Path, true, "Updated provider: "+updated.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerToResponse(&updated, r.config.PublicURL))
}

func (r *Router) handleDeleteSSOProvider(w http.ResponseWriter, req *http.Request, providerID string) {
	r.ensureSSOConfig()

	existing := r.ssoConfig.GetProvider(providerID)
	if existing == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Provider not found", nil)
		return
	}

	// Remove provider
	if err := r.ssoConfig.RemoveProvider(providerID); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "remove_error", err.Error(), nil)
		return
	}

	// Persist configuration
	if err := r.saveSSOConfig(); err != nil {
		log.Error().Err(err).Msg("Failed to persist SSO configuration")
		// Re-add the provider since persistence failed
		if err := r.ssoConfig.AddProvider(*existing); err != nil {
			log.Warn().Err(err).Str("provider_id", existing.ID).Msg("Failed to re-add provider during rollback")
		}
		writeErrorResponse(w, http.StatusInternalServerError, "save_error", "Failed to save configuration", nil)
		return
	}

	// Remove SAML service if applicable
	if existing.Type == config.SSOProviderTypeSAML {
		r.samlManager.RemoveProvider(providerID)
	}

	// Remove OIDC service if applicable
	if existing.Type == config.SSOProviderTypeOIDC {
		r.oidcManager.RemoveService(providerID)
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "sso_provider_deleted", "", GetClientIP(req), req.URL.Path, true, "Deleted provider: "+existing.Name)

	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) saveSSOConfig() error {
	if r.persistence == nil {
		return nil
	}
	return r.persistence.SaveSSOConfig(r.ssoConfig)
}

func providerToResponse(p *config.SSOProvider, publicURL string) SSOProviderResponse {
	resp := SSOProviderResponse{
		ID:             p.ID,
		Name:           p.Name,
		Type:           string(p.Type),
		Enabled:        p.Enabled,
		DisplayName:    p.DisplayName,
		IconURL:        p.IconURL,
		Priority:       p.Priority,
		AllowedGroups:  p.AllowedGroups,
		AllowedDomains: p.AllowedDomains,
		AllowedEmails:  p.AllowedEmails,
	}

	if resp.DisplayName == "" {
		resp.DisplayName = p.Name
	}

	baseURL := publicURL
	if baseURL == "" {
		baseURL = "http://localhost:7655"
	}

	if p.Type == config.SSOProviderTypeOIDC && p.OIDC != nil {
		resp.OIDCIssuerURL = p.OIDC.IssuerURL
		resp.OIDCClientID = p.OIDC.ClientID
		resp.OIDCClientSecretSet = p.OIDC.ClientSecretSet || p.OIDC.ClientSecret != ""
		resp.OIDCLoginURL = baseURL + "/api/oidc/" + p.ID + "/login"
		resp.OIDCCallbackURL = baseURL + "/api/oidc/" + p.ID + "/callback"
	}

	if p.Type == config.SSOProviderTypeSAML && p.SAML != nil {
		resp.SAMLIDPEntityID = p.SAML.IDPEntityID
		if resp.SAMLIDPEntityID == "" {
			resp.SAMLIDPEntityID = p.SAML.IDPIssuer
		}
		resp.SAMLSPEntityID = p.SAML.SPEntityID
		if resp.SAMLSPEntityID == "" {
			resp.SAMLSPEntityID = baseURL + "/saml/" + p.ID
		}
		resp.SAMLMetadataURL = baseURL + "/api/saml/" + p.ID + "/metadata"
		resp.SAMLACSURL = baseURL + "/api/saml/" + p.ID + "/acs"
	}

	return resp
}

// ============================================================================
// SSO Provider Connection Testing
// ============================================================================

const (
	maxTestRequestBodySize = 32 * 1024 // 32KB
	testConnectionTimeout  = 30 * time.Second
)

// SSOTestRequest represents a request to test SSO provider configuration
type SSOTestRequest struct {
	Type string          `json:"type"` // "saml" or "oidc"
	SAML *SAMLTestConfig `json:"saml,omitempty"`
	OIDC *OIDCTestConfig `json:"oidc,omitempty"`
}

// SAMLTestConfig contains SAML configuration to test
type SAMLTestConfig struct {
	IDPMetadataURL string `json:"idpMetadataUrl,omitempty"`
	IDPMetadataXML string `json:"idpMetadataXml,omitempty"`
	IDPSSOURL      string `json:"idpSsoUrl,omitempty"`
	IDPCertificate string `json:"idpCertificate,omitempty"`
}

// OIDCTestConfig contains OIDC configuration to test
type OIDCTestConfig struct {
	IssuerURL string `json:"issuerUrl"`
	ClientID  string `json:"clientId,omitempty"`
}

// SSOTestResponse represents the result of a connection test
type SSOTestResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   string          `json:"error,omitempty"`
	Details *SSOTestDetails `json:"details,omitempty"`
}

// SSOTestDetails contains detailed information about the tested provider
type SSOTestDetails struct {
	Type         string            `json:"type"`
	EntityID     string            `json:"entityId,omitempty"`
	SSOURL       string            `json:"ssoUrl,omitempty"`
	SLOURL       string            `json:"sloUrl,omitempty"`
	Certificates []CertificateInfo `json:"certificates,omitempty"`
	// OIDC-specific
	TokenEndpoint    string   `json:"tokenEndpoint,omitempty"`
	UserinfoEndpoint string   `json:"userinfoEndpoint,omitempty"`
	JWKSURI          string   `json:"jwksUri,omitempty"`
	SupportedScopes  []string `json:"supportedScopes,omitempty"`
}

// CertificateInfo contains certificate details
type CertificateInfo struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"notBefore"`
	NotAfter  time.Time `json:"notAfter"`
	IsExpired bool      `json:"isExpired"`
}

// handleTestSSOProvider tests an SSO provider configuration
func (r *Router) handleTestSSOProvider(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	// Rate limiting
	clientIP := GetClientIP(req)
	if !authLimiter.Allow(clientIP) {
		writeErrorResponse(w, http.StatusTooManyRequests, "rate_limited", "Too many requests", nil)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxTestRequestBodySize))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "read_error", "Failed to read request body", nil)
		return
	}

	var testReq SSOTestRequest
	if err := json.Unmarshal(body, &testReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload", nil)
		return
	}

	// Validate request
	if testReq.Type != "saml" && testReq.Type != "oidc" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Type must be 'saml' or 'oidc'", nil)
		return
	}

	var response SSOTestResponse

	ctx, cancel := context.WithTimeout(req.Context(), testConnectionTimeout)
	defer cancel()

	switch testReq.Type {
	case "saml":
		response = r.testSAMLConnection(ctx, testReq.SAML)
	case "oidc":
		response = r.testOIDCConnection(ctx, testReq.OIDC)
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "sso_provider_test", "", clientIP, req.URL.Path, response.Success,
		"Tested "+testReq.Type+" provider connection")

	w.Header().Set("Content-Type", "application/json")
	if response.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}

func (r *Router) testSAMLConnection(ctx context.Context, cfg *SAMLTestConfig) SSOTestResponse {
	if cfg == nil {
		return SSOTestResponse{
			Success: false,
			Message: "SAML configuration is required",
			Error:   "missing_config",
		}
	}

	// Need at least one source of metadata
	if cfg.IDPMetadataURL == "" && cfg.IDPMetadataXML == "" && cfg.IDPSSOURL == "" {
		return SSOTestResponse{
			Success: false,
			Message: "Provide IdP Metadata URL, XML, or SSO URL",
			Error:   "missing_metadata",
		}
	}

	// Validate URLs
	if cfg.IDPMetadataURL != "" && !validateURL(cfg.IDPMetadataURL, []string{"https", "http"}) {
		return SSOTestResponse{
			Success: false,
			Message: "Invalid metadata URL format",
			Error:   "invalid_url",
		}
	}

	var metadata *saml.EntityDescriptor
	var rawXML []byte
	var err error

	httpClient := newTestHTTPClient()

	if cfg.IDPMetadataURL != "" {
		rawXML, metadata, err = fetchSAMLMetadataFromURL(ctx, httpClient, cfg.IDPMetadataURL)
		if err != nil {
			return SSOTestResponse{
				Success: false,
				Message: "Failed to fetch metadata from URL",
				Error:   err.Error(),
			}
		}
	} else if cfg.IDPMetadataXML != "" {
		rawXML = []byte(cfg.IDPMetadataXML)
		metadata, err = parseSAMLMetadataXML(rawXML)
		if err != nil {
			return SSOTestResponse{
				Success: false,
				Message: "Failed to parse metadata XML",
				Error:   err.Error(),
			}
		}
	} else {
		// Manual configuration - just validate the SSO URL
		if !validateURL(cfg.IDPSSOURL, []string{"https", "http"}) {
			return SSOTestResponse{
				Success: false,
				Message: "Invalid SSO URL format",
				Error:   "invalid_url",
			}
		}
		return SSOTestResponse{
			Success: true,
			Message: "SSO URL is valid (manual configuration)",
			Details: &SSOTestDetails{
				Type:   "saml",
				SSOURL: cfg.IDPSSOURL,
			},
		}
	}

	// Extract details from metadata
	details := &SSOTestDetails{
		Type:     "saml",
		EntityID: metadata.EntityID,
	}

	// Extract SSO URL
	if len(metadata.IDPSSODescriptors) > 0 {
		idpDesc := metadata.IDPSSODescriptors[0]
		for _, sso := range idpDesc.SingleSignOnServices {
			if sso.Binding == saml.HTTPPostBinding || sso.Binding == saml.HTTPRedirectBinding {
				details.SSOURL = sso.Location
				break
			}
		}
		// Extract SLO URL
		for _, slo := range idpDesc.SingleLogoutServices {
			details.SLOURL = slo.Location
			break
		}
		// Extract certificates
		for _, kd := range idpDesc.KeyDescriptors {
			if kd.Use == "signing" || kd.Use == "" {
				for _, x509Cert := range kd.KeyInfo.X509Data.X509Certificates {
					certInfo := extractCertificateInfo(x509Cert.Data)
					if certInfo != nil {
						details.Certificates = append(details.Certificates, *certInfo)
					}
				}
			}
		}
	}

	_ = rawXML // Used for metadata preview endpoint

	return SSOTestResponse{
		Success: true,
		Message: "SAML metadata validated successfully",
		Details: details,
	}
}

func (r *Router) testOIDCConnection(ctx context.Context, cfg *OIDCTestConfig) SSOTestResponse {
	if cfg == nil {
		return SSOTestResponse{
			Success: false,
			Message: "OIDC configuration is required",
			Error:   "missing_config",
		}
	}

	if cfg.IssuerURL == "" {
		return SSOTestResponse{
			Success: false,
			Message: "Issuer URL is required",
			Error:   "missing_issuer",
		}
	}

	if !validateURL(cfg.IssuerURL, []string{"https", "http"}) {
		return SSOTestResponse{
			Success: false,
			Message: "Invalid issuer URL format",
			Error:   "invalid_url",
		}
	}

	// Fetch OIDC discovery document
	discoveryURL := strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/openid-configuration"

	httpClient := newTestHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return SSOTestResponse{
			Success: false,
			Message: "Failed to create discovery request",
			Error:   err.Error(),
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return SSOTestResponse{
			Success: false,
			Message: "Failed to fetch OIDC discovery document",
			Error:   err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SSOTestResponse{
			Success: false,
			Message: "OIDC discovery returned non-200 status",
			Error:   resp.Status,
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return SSOTestResponse{
			Success: false,
			Message: "Failed to read discovery response",
			Error:   err.Error(),
		}
	}

	var discovery struct {
		Issuer                string   `json:"issuer"`
		AuthorizationEndpoint string   `json:"authorization_endpoint"`
		TokenEndpoint         string   `json:"token_endpoint"`
		UserinfoEndpoint      string   `json:"userinfo_endpoint"`
		JWKSURI               string   `json:"jwks_uri"`
		ScopesSupported       []string `json:"scopes_supported"`
	}

	if err := json.Unmarshal(body, &discovery); err != nil {
		return SSOTestResponse{
			Success: false,
			Message: "Failed to parse discovery document",
			Error:   err.Error(),
		}
	}

	// Validate issuer matches
	if discovery.Issuer != cfg.IssuerURL && discovery.Issuer != strings.TrimRight(cfg.IssuerURL, "/") {
		log.Warn().
			Str("expected", cfg.IssuerURL).
			Str("actual", discovery.Issuer).
			Msg("OIDC issuer mismatch - this may cause token validation issues")
	}

	details := &SSOTestDetails{
		Type:             "oidc",
		EntityID:         discovery.Issuer,
		TokenEndpoint:    discovery.TokenEndpoint,
		UserinfoEndpoint: discovery.UserinfoEndpoint,
		JWKSURI:          discovery.JWKSURI,
		SupportedScopes:  discovery.ScopesSupported,
	}

	return SSOTestResponse{
		Success: true,
		Message: "OIDC discovery successful",
		Details: details,
	}
}

// ============================================================================
// SSO Metadata Preview
// ============================================================================

// MetadataPreviewRequest represents a request to preview IdP metadata
type MetadataPreviewRequest struct {
	Type        string `json:"type"` // "saml"
	MetadataURL string `json:"metadataUrl,omitempty"`
	MetadataXML string `json:"metadataXml,omitempty"`
}

// MetadataPreviewResponse contains the metadata preview
type MetadataPreviewResponse struct {
	XML    string              `json:"xml"`
	Parsed *ParsedMetadataInfo `json:"parsed"`
}

// ParsedMetadataInfo contains parsed metadata information
type ParsedMetadataInfo struct {
	EntityID      string            `json:"entityId"`
	SSOURL        string            `json:"ssoUrl,omitempty"`
	SLOURL        string            `json:"sloUrl,omitempty"`
	Certificates  []CertificateInfo `json:"certificates,omitempty"`
	NameIDFormats []string          `json:"nameIdFormats,omitempty"`
}

// handleMetadataPreview fetches and displays IdP metadata
func (r *Router) handleMetadataPreview(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	// Rate limiting
	clientIP := GetClientIP(req)
	if !authLimiter.Allow(clientIP) {
		writeErrorResponse(w, http.StatusTooManyRequests, "rate_limited", "Too many requests", nil)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxTestRequestBodySize))
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "read_error", "Failed to read request body", nil)
		return
	}

	var previewReq MetadataPreviewRequest
	if err := json.Unmarshal(body, &previewReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload", nil)
		return
	}

	if previewReq.Type != "saml" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Only SAML metadata preview is supported", nil)
		return
	}

	if previewReq.MetadataURL == "" && previewReq.MetadataXML == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Provide either metadataUrl or metadataXml", nil)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), testConnectionTimeout)
	defer cancel()

	var rawXML []byte
	var metadata *saml.EntityDescriptor

	httpClient := newTestHTTPClient()

	if previewReq.MetadataURL != "" {
		if !validateURL(previewReq.MetadataURL, []string{"https", "http"}) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid metadata URL", nil)
			return
		}
		rawXML, metadata, err = fetchSAMLMetadataFromURL(ctx, httpClient, previewReq.MetadataURL)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "fetch_error", "Failed to fetch metadata: "+err.Error(), nil)
			return
		}
	} else {
		rawXML = []byte(previewReq.MetadataXML)
		metadata, err = parseSAMLMetadataXML(rawXML)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "parse_error", "Failed to parse metadata: "+err.Error(), nil)
			return
		}
	}

	// Build parsed info
	parsed := &ParsedMetadataInfo{
		EntityID: metadata.EntityID,
	}

	if len(metadata.IDPSSODescriptors) > 0 {
		idpDesc := metadata.IDPSSODescriptors[0]

		// Extract SSO URL
		for _, sso := range idpDesc.SingleSignOnServices {
			if sso.Binding == saml.HTTPPostBinding || sso.Binding == saml.HTTPRedirectBinding {
				parsed.SSOURL = sso.Location
				break
			}
		}

		// Extract SLO URL
		for _, slo := range idpDesc.SingleLogoutServices {
			parsed.SLOURL = slo.Location
			break
		}

		// Extract NameID formats
		for _, nid := range idpDesc.NameIDFormats {
			parsed.NameIDFormats = append(parsed.NameIDFormats, string(nid))
		}

		// Extract certificates
		for _, kd := range idpDesc.KeyDescriptors {
			if kd.Use == "signing" || kd.Use == "" {
				for _, x509Cert := range kd.KeyInfo.X509Data.X509Certificates {
					certInfo := extractCertificateInfo(x509Cert.Data)
					if certInfo != nil {
						parsed.Certificates = append(parsed.Certificates, *certInfo)
					}
				}
			}
		}
	}

	// Format XML for display
	formattedXML := formatXML(rawXML)

	response := MetadataPreviewResponse{
		XML:    formattedXML,
		Parsed: parsed,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ============================================================================
// Helper Functions
// ============================================================================

func newTestHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   testConnectionTimeout,
	}
}

func fetchSAMLMetadataFromURL(ctx context.Context, client *http.Client, metadataURL string) ([]byte, *saml.EntityDescriptor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("metadata request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, nil, err
	}

	metadata, err := parseSAMLMetadataXML(body)
	if err != nil {
		return nil, nil, err
	}

	return body, metadata, nil
}

func parseSAMLMetadataXML(data []byte) (*saml.EntityDescriptor, error) {
	var metadata saml.EntityDescriptor
	if err := xml.Unmarshal(data, &metadata); err != nil {
		// Try parsing as EntitiesDescriptor
		var entities saml.EntitiesDescriptor
		if err2 := xml.Unmarshal(data, &entities); err2 != nil {
			return nil, fmt.Errorf("failed to parse metadata: %v", err)
		}
		if len(entities.EntityDescriptors) == 0 {
			return nil, fmt.Errorf("no entity descriptors found in metadata")
		}
		metadata = entities.EntityDescriptors[0]
	}
	return &metadata, nil
}

func extractCertificateInfo(certData string) *CertificateInfo {
	// Remove whitespace and decode base64
	certData = strings.ReplaceAll(certData, "\n", "")
	certData = strings.ReplaceAll(certData, "\r", "")
	certData = strings.ReplaceAll(certData, " ", "")

	// Try to decode as PEM first
	var derBytes []byte
	block, _ := pem.Decode([]byte(certData))
	if block != nil {
		derBytes = block.Bytes
	} else {
		// Assume it's base64 encoded DER
		var err error
		derBytes, err = base64Decode(certData)
		if err != nil {
			return nil
		}
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil
	}

	return &CertificateInfo{
		Subject:   cert.Subject.String(),
		Issuer:    cert.Issuer.String(),
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		IsExpired: time.Now().After(cert.NotAfter),
	}
}

func base64Decode(s string) ([]byte, error) {
	// Try standard base64 first
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(s)
	}
	return decoded, err
}

func formatXML(data []byte) string {
	// Try to pretty-print the XML
	var buf strings.Builder
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if err := encoder.EncodeToken(token); err != nil {
			// Fall back to original
			return string(data)
		}
	}
	encoder.Flush()

	if buf.Len() > 0 {
		return buf.String()
	}
	return string(data)
}
