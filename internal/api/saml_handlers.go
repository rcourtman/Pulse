package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// SAMLServiceManager manages multiple SAML services for different providers
type SAMLServiceManager struct {
	mu       sync.RWMutex
	services map[string]*SAMLService
	baseURL  string
}

// NewSAMLServiceManager creates a new SAML service manager
func NewSAMLServiceManager(baseURL string) *SAMLServiceManager {
	return &SAMLServiceManager{
		services: make(map[string]*SAMLService),
		baseURL:  baseURL,
	}
}

// GetService returns a SAML service for the given provider ID
func (m *SAMLServiceManager) GetService(providerID string) *SAMLService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.services[providerID]
}

// InitializeProvider creates or updates a SAML service for a provider
func (m *SAMLServiceManager) InitializeProvider(ctx context.Context, providerID string, cfg *config.SAMLProviderConfig) error {
	service, err := NewSAMLService(ctx, providerID, cfg, m.baseURL)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[providerID] = service

	log.Info().
		Str("provider_id", providerID).
		Msg("Initialized SAML provider")

	return nil
}

// RemoveProvider removes a SAML service
func (m *SAMLServiceManager) RemoveProvider(providerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.services, providerID)
}

// handleSAMLLogin initiates a SAML authentication flow
func (r *Router) handleSAMLLogin(w http.ResponseWriter, req *http.Request) {
	providerID := extractSAMLProviderID(req.URL.Path, "login")
	if providerID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Provider ID is required", nil)
		return
	}

	// Security: Validate provider ID format
	if !validateProviderID(providerID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Invalid provider ID format", nil)
		return
	}

	provider := r.getSSOProvider(providerID)
	if provider == nil || provider.Type != config.SSOProviderTypeSAML || !provider.Enabled {
		writeErrorResponse(w, http.StatusNotFound, "provider_not_found", "SAML provider not found or not enabled", nil)
		return
	}

	service := r.samlManager.GetService(providerID)
	if service == nil {
		// Try to initialize the provider
		if err := r.samlManager.InitializeProvider(req.Context(), providerID, provider.SAML); err != nil {
			log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to initialize SAML provider")
			writeErrorResponse(w, http.StatusInternalServerError, "saml_init_failed", "Failed to initialize SAML provider", nil)
			return
		}
		service = r.samlManager.GetService(providerID)
	}

	// Get return URL from query or form
	returnTo := sanitizeOIDCReturnTo(req.URL.Query().Get("returnTo"))
	if returnTo == "" && req.Method == http.MethodPost {
		var payload struct {
			ReturnTo string `json:"returnTo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err == nil {
			returnTo = sanitizeOIDCReturnTo(payload.ReturnTo)
		}
	}

	// Create SAML AuthnRequest
	redirectURL, err := service.MakeAuthRequest(returnTo)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to create SAML auth request")
		writeErrorResponse(w, http.StatusInternalServerError, "saml_auth_failed", "Failed to create authentication request", nil)
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login_initiated", "", GetClientIP(req), req.URL.Path, true, "Provider: "+providerID)

	// Redirect for GET, return JSON for POST
	if req.Method == http.MethodGet {
		http.Redirect(w, req, redirectURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"authorizationUrl": redirectURL,
	})
}

// handleSAMLACS handles the SAML Assertion Consumer Service (callback)
func (r *Router) handleSAMLACS(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	providerID := extractSAMLProviderID(req.URL.Path, "acs")
	if providerID == "" {
		r.redirectSAMLError(w, req, "", "invalid_provider")
		return
	}

	// Security: Validate provider ID format
	if !validateProviderID(providerID) {
		r.redirectSAMLError(w, req, "", "invalid_provider")
		return
	}

	provider := r.getSSOProvider(providerID)
	if provider == nil || provider.Type != config.SSOProviderTypeSAML || !provider.Enabled {
		r.redirectSAMLError(w, req, "", "provider_not_found")
		return
	}

	service := r.samlManager.GetService(providerID)
	if service == nil {
		r.redirectSAMLError(w, req, "", "provider_not_initialized")
		return
	}

	// Process SAML response
	result, relayState, err := service.ProcessResponse(req)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to process SAML response")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", "", GetClientIP(req), req.URL.Path, false, "SAML response validation failed: "+err.Error())
		r.redirectSAMLError(w, req, relayState, "saml_validation_failed")
		return
	}

	// Check group restrictions
	if len(provider.AllowedGroups) > 0 {
		if !intersects(result.Groups, provider.AllowedGroups) {
			log.Debug().
				Str("username", result.Username).
				Strs("user_groups", result.Groups).
				Strs("allowed_groups", provider.AllowedGroups).
				Msg("User not in allowed groups")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", result.Username, GetClientIP(req), req.URL.Path, false, "Group restriction failed")
			r.redirectSAMLError(w, req, relayState, "group_restricted")
			return
		}
	}

	// Check domain restrictions
	if len(provider.AllowedDomains) > 0 && result.Email != "" {
		if !matchesDomain(result.Email, provider.AllowedDomains) {
			log.Debug().
				Str("email", result.Email).
				Strs("allowed_domains", provider.AllowedDomains).
				Msg("Email domain not allowed")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", result.Username, GetClientIP(req), req.URL.Path, false, "Domain restriction failed")
			r.redirectSAMLError(w, req, relayState, "domain_restricted")
			return
		}
	}

	// Check email restrictions
	if len(provider.AllowedEmails) > 0 && result.Email != "" {
		if !matchesValue(result.Email, provider.AllowedEmails) {
			log.Debug().
				Str("email", result.Email).
				Strs("allowed_emails", provider.AllowedEmails).
				Msg("Email not in allowed list")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", result.Username, GetClientIP(req), req.URL.Path, false, "Email restriction failed")
			r.redirectSAMLError(w, req, relayState, "email_restricted")
			return
		}
	}

	// RBAC Integration: Map SAML groups to Pulse roles
	if authManager := internalauth.GetManager(); authManager != nil && len(provider.GroupRoleMappings) > 0 {
		var rolesToAssign []string
		seenRoles := make(map[string]bool)

		for _, group := range result.Groups {
			if roleID, ok := provider.GroupRoleMappings[group]; ok {
				if !seenRoles[roleID] {
					rolesToAssign = append(rolesToAssign, roleID)
					seenRoles[roleID] = true
				}
			}
		}

		if len(rolesToAssign) > 0 {
			log.Info().
				Str("user", result.Username).
				Strs("mapped_roles", rolesToAssign).
				Msg("Auto-assigning roles based on SAML group mapping")
			if err := authManager.UpdateUserRoles(result.Username, rolesToAssign); err != nil {
				log.Error().Err(err).Str("user", result.Username).Msg("Failed to auto-assign SAML roles")
				LogAuditEventForTenant(GetOrgID(req.Context()), "saml_role_assignment", result.Username, GetClientIP(req), req.URL.Path, false, "Failed to auto-assign roles: "+strings.Join(rolesToAssign, ", "))
			} else {
				LogAuditEventForTenant(GetOrgID(req.Context()), "saml_role_assignment", result.Username, GetClientIP(req), req.URL.Path, true, "Auto-assigned roles: "+strings.Join(rolesToAssign, ", "))
			}
		}
	}

	// Establish session
	username := result.Username
	if username == "" {
		username = result.Email
	}
	if username == "" {
		username = result.NameID
	}

	// Store SAML session info for potential SLO
	samlSession := &SAMLSessionInfo{
		ProviderID:   providerID,
		NameID:       result.NameID,
		SessionIndex: result.SessionIdx,
	}

	if err := r.establishSAMLSession(w, req, username, samlSession); err != nil {
		log.Error().Err(err).Msg("Failed to establish session after SAML login")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", username, GetClientIP(req), req.URL.Path, false, "Session creation failed")
		r.redirectSAMLError(w, req, relayState, "session_failed")
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", username, GetClientIP(req), req.URL.Path, true, "SAML login success via "+providerID)

	// Redirect to return URL - sanitize relayState to prevent open redirect
	target := sanitizeOIDCReturnTo(relayState)
	if target == "" {
		target = "/"
	}
	target = addQueryParam(target, "saml", "success")
	http.Redirect(w, req, target, http.StatusFound)
}

// handleSAMLMetadata returns the SP metadata XML
func (r *Router) handleSAMLMetadata(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	providerID := extractSAMLProviderID(req.URL.Path, "metadata")
	if providerID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Provider ID is required", nil)
		return
	}

	// Security: Validate provider ID format
	if !validateProviderID(providerID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Invalid provider ID format", nil)
		return
	}

	provider := r.getSSOProvider(providerID)
	if provider == nil || provider.Type != config.SSOProviderTypeSAML {
		writeErrorResponse(w, http.StatusNotFound, "provider_not_found", "SAML provider not found", nil)
		return
	}

	service := r.samlManager.GetService(providerID)
	if service == nil {
		// Try to initialize the provider
		if provider.SAML != nil {
			if err := r.samlManager.InitializeProvider(req.Context(), providerID, provider.SAML); err != nil {
				log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to initialize SAML provider for metadata")
				writeErrorResponse(w, http.StatusInternalServerError, "saml_init_failed", "Failed to initialize SAML provider", nil)
				return
			}
			service = r.samlManager.GetService(providerID)
		}
		if service == nil {
			writeErrorResponse(w, http.StatusNotFound, "provider_not_initialized", "SAML provider not initialized", nil)
			return
		}
	}

	metadata, err := service.GetMetadata()
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to generate SAML metadata")
		writeErrorResponse(w, http.StatusInternalServerError, "metadata_error", "Failed to generate metadata", nil)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "inline; filename=metadata.xml")
	w.Write(metadata)
}

// handleSAMLLogout handles SAML Single Logout
func (r *Router) handleSAMLLogout(w http.ResponseWriter, req *http.Request) {
	providerID := extractSAMLProviderID(req.URL.Path, "logout")
	if providerID == "" {
		// Fall back to regular logout
		r.handleLogout(w, req)
		return
	}

	// Security: Validate provider ID format
	if !validateProviderID(providerID) {
		// Invalid ID, fall back to regular logout
		r.handleLogout(w, req)
		return
	}

	service := r.samlManager.GetService(providerID)
	if service == nil {
		// Fall back to regular logout
		r.handleLogout(w, req)
		return
	}

	// Get session info for SLO
	session := r.getSAMLSessionInfo(req)
	if session == nil || session.NameID == "" {
		// No SAML session info, fall back to regular logout
		r.handleLogout(w, req)
		return
	}

	// Clear local session first
	r.clearSession(w, req)

	// Attempt SAML SLO
	logoutURL, err := service.MakeLogoutRequest(session.NameID, session.SessionIndex)
	if err != nil {
		log.Warn().Err(err).Str("provider_id", providerID).Msg("SAML SLO not available, local logout only")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_logout", "", GetClientIP(req), req.URL.Path, true, "Local logout only (SLO not available)")
		http.Redirect(w, req, "/?logout=success", http.StatusFound)
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_logout", "", GetClientIP(req), req.URL.Path, true, "Initiating SAML SLO")
	http.Redirect(w, req, logoutURL, http.StatusFound)
}

// handleSAMLSLO handles SAML Single Logout responses/requests
func (r *Router) handleSAMLSLO(w http.ResponseWriter, req *http.Request) {
	// For now, just redirect to home with logout success
	// A full implementation would validate the LogoutResponse
	r.clearSession(w, req)
	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_slo_callback", "", GetClientIP(req), req.URL.Path, true, "SAML SLO complete")
	http.Redirect(w, req, "/?logout=success", http.StatusFound)
}

// SAMLSessionInfo stores SAML-specific session information for SLO
type SAMLSessionInfo struct {
	ProviderID   string `json:"providerId"`
	NameID       string `json:"nameId"`
	SessionIndex string `json:"sessionIndex"`
}

// establishSAMLSession creates a session for a SAML-authenticated user
func (r *Router) establishSAMLSession(w http.ResponseWriter, req *http.Request, username string, samlInfo *SAMLSessionInfo) error {
	token := generateSessionToken()
	if token == "" {
		return fmt.Errorf("failed to generate session token")
	}

	userAgent := req.Header.Get("User-Agent")
	clientIP := GetClientIP(req)

	// Convert SAMLSessionInfo to SAMLTokenInfo for storage
	var samlTokens *SAMLTokenInfo
	if samlInfo != nil {
		samlTokens = &SAMLTokenInfo{
			ProviderID:   samlInfo.ProviderID,
			NameID:       samlInfo.NameID,
			SessionIndex: samlInfo.SessionIndex,
		}
	}

	// Create session with SAML info for SLO support
	GetSessionStore().CreateSAMLSession(token, 24*time.Hour, userAgent, clientIP, username, samlTokens)

	if username != "" {
		TrackUserSession(username, token)
	}

	csrfToken := generateCSRFToken(token)
	isSecure, sameSitePolicy := getCookieSettings(req)

	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sameSitePolicy,
		MaxAge:   86400,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_csrf",
		Value:    csrfToken,
		Path:     "/",
		Secure:   isSecure,
		SameSite: sameSitePolicy,
		MaxAge:   86400,
	})

	return nil
}

// getSAMLSessionInfo retrieves SAML session info from the current session
func (r *Router) getSAMLSessionInfo(req *http.Request) *SAMLSessionInfo {
	cookie, err := req.Cookie("pulse_session")
	if err != nil || cookie.Value == "" {
		return nil
	}

	samlInfo := GetSessionStore().GetSAMLSessionInfo(cookie.Value)
	if samlInfo == nil {
		return nil
	}

	return &SAMLSessionInfo{
		ProviderID:   samlInfo.ProviderID,
		NameID:       samlInfo.NameID,
		SessionIndex: samlInfo.SessionIndex,
	}
}

// clearSession clears the current session - properly invalidates server-side session
// and clears both pulse_session and pulse_csrf cookies
func (r *Router) clearSession(w http.ResponseWriter, req *http.Request) {
	isSecure, sameSitePolicy := getCookieSettings(req)

	// Invalidate server-side session first
	if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		// Get username before deleting session for untracking
		if username := GetSessionUsername(cookie.Value); username != "" {
			UntrackUserSession(username, cookie.Value)
		}
		GetSessionStore().InvalidateSession(cookie.Value)
	}

	// Clear pulse_session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sameSitePolicy,
	})

	// Clear pulse_csrf cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_csrf",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   isSecure,
		SameSite: sameSitePolicy,
	})
}

func (r *Router) redirectSAMLError(w http.ResponseWriter, req *http.Request, returnTo string, code string) {
	// Sanitize returnTo to prevent open redirect attacks
	target := sanitizeOIDCReturnTo(returnTo)
	if target == "" {
		target = "/"
	}
	target = addQueryParam(target, "saml", "error")
	if code != "" {
		target = addQueryParam(target, "saml_error", code)
	}
	http.Redirect(w, req, target, http.StatusFound)
}

// extractSAMLProviderID extracts the provider ID from a SAML endpoint path
// Expected paths: /api/saml/{providerID}/login, /api/saml/{providerID}/acs, etc.
func extractSAMLProviderID(path, endpoint string) string {
	// Path format: /api/saml/{id}/{endpoint}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "saml" {
		if len(parts) >= 4 && parts[3] == endpoint {
			return parts[2]
		}
		// Also handle /api/saml/{id}/{endpoint} without trailing parts
		if len(parts) == 4 && parts[3] == endpoint {
			return parts[2]
		}
	}
	return ""
}

// getSSOProvider retrieves an SSO provider by ID from the current configuration
func (r *Router) getSSOProvider(providerID string) *config.SSOProvider {
	if r.ssoConfig == nil {
		return nil
	}
	return r.ssoConfig.GetProvider(providerID)
}

// InitializeSAMLProviders initializes all enabled SAML providers
func (r *Router) InitializeSAMLProviders(ctx context.Context) error {
	if r.ssoConfig == nil {
		return nil
	}

	for _, provider := range r.ssoConfig.Providers {
		if provider.Type == config.SSOProviderTypeSAML && provider.Enabled && provider.SAML != nil {
			if err := r.samlManager.InitializeProvider(ctx, provider.ID, provider.SAML); err != nil {
				log.Error().
					Err(err).
					Str("provider_id", provider.ID).
					Msg("Failed to initialize SAML provider")
				// Continue initializing other providers
			}
		}
	}

	return nil
}

// RefreshSAMLProvider refreshes a SAML provider's IdP metadata
func (r *Router) RefreshSAMLProvider(ctx context.Context, providerID string) error {
	service := r.samlManager.GetService(providerID)
	if service == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return service.RefreshMetadata(ctx)
}
