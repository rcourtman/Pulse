package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
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

func (m *SAMLServiceManager) GetPublicURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseURL
}

func (m *SAMLServiceManager) SetPublicURL(baseURL string) error {
	normalizedBaseURL, err := normalizeSAMLBaseURL(baseURL)
	if err != nil {
		return fmt.Errorf("normalize SAML public URL: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if normalizedBaseURL == m.baseURL {
		return nil
	}

	for providerID, service := range m.services {
		if err := service.SetBaseURL(normalizedBaseURL); err != nil {
			return fmt.Errorf("rebind SAML provider %q public URL: %w", providerID, err)
		}
	}

	m.baseURL = normalizedBaseURL
	return nil
}

// GetService returns a SAML service for the given provider ID
func (m *SAMLServiceManager) GetService(providerID string) *SAMLService {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.services[providerID]
}

// InitializeProvider creates or updates a SAML service for a provider
func (m *SAMLServiceManager) InitializeProvider(ctx context.Context, providerID string, cfg *config.SAMLProviderConfig) error {
	m.mu.RLock()
	baseURL := m.baseURL
	m.mu.RUnlock()

	service, err := NewSAMLService(ctx, providerID, cfg, baseURL)
	if err != nil {
		return fmt.Errorf("initialize SAML provider %q: %w", providerID, err)
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

func (r *Router) syncSAMLPublicURL() error {
	if r == nil || r.samlManager == nil || r.config == nil {
		return nil
	}
	return r.samlManager.SetPublicURL(r.config.PublicURL)
}

// handleSAMLLogin initiates a SAML authentication flow
func (r *Router) handleSAMLLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET or POST is allowed", nil)
		return
	}

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

	if err := r.syncSAMLPublicURL(); err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to synchronize SAML public URL")
		writeErrorResponse(w, http.StatusInternalServerError, "saml_init_failed", "Failed to synchronize SAML provider", nil)
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

	// Create SAML AuthnRequest, bound to this browser's login cookie
	loginToken, err := samlLoginToken(req)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to create SAML login binding")
		writeErrorResponse(w, http.StatusInternalServerError, "saml_auth_failed", "Failed to create authentication request", nil)
		return
	}
	redirectURL, err := service.MakeAuthRequest(returnTo, sessionHash(loginToken))
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to create SAML auth request")
		writeErrorResponse(w, http.StatusInternalServerError, "saml_auth_failed", "Failed to create authentication request", nil)
		return
	}
	cookiePolicy := getBrowserCookiePolicy(req)
	cookiePolicy.setHTTPOnly(w, &http.Cookie{
		Name:   samlLoginCookieName(cookiePolicy.secure),
		Value:  loginToken,
		Path:   "/",
		MaxAge: int(samlRequestTTL / time.Second),
	})

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

	if err := req.ParseForm(); err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to parse SAML response form")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", "", GetClientIP(req), req.URL.Path, false, "SAML response validation failed: failed to parse form: "+err.Error())
		r.redirectSAMLError(w, req, "", "saml_validation_failed")
		return
	}
	browserKey := samlLoginBrowserKey(req)
	if browserKey == "" && !service.AllowsIDPInitiated() && req.PostForm.Get(samlACSRepostField) == "" &&
		(req.PostForm.Get("SAMLResponse") != "" || req.PostForm.Get("SAMLart") != "") {
		writeSAMLACSRepost(w, req)
		return
	}

	// Process SAML response
	result, relayState, err := service.ProcessResponse(req, browserKey)
	if err != nil {
		if errors.Is(err, errSAMLResponseUnbound) {
			log.Warn().Err(err).Str("provider_id", providerID).Str("client_ip", GetClientIP(req)).Msg("SAML Response does not answer a login started in this browser")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", "", GetClientIP(req), req.URL.Path, false, "SAML response does not answer a login started in this browser")
		} else {
			log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to process SAML response")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", "", GetClientIP(req), req.URL.Path, false, "SAML response validation failed: "+err.Error())
		}
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
	if len(provider.AllowedDomains) > 0 {
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
	if len(provider.AllowedEmails) > 0 {
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

	principal, err := stableSSOPrincipal(config.SSOProviderTypeSAML, providerID, result.NameID)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("SAML NameID is not usable as a stable SSO principal")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", result.Username, GetClientIP(req), req.URL.Path, false, "Missing stable SAML subject")
		r.redirectSAMLError(w, req, relayState, "missing_subject")
		return
	}

	// RBAC Integration: Map SAML groups to Pulse roles under the stable SSO
	// principal. When mappings are configured they are authoritative, including
	// the case where the current assertion maps to no roles.
	if authManager := internalauth.GetManager(); authManager != nil {
		rolesToAssign := resolveGroupRoles(result.Groups, provider.GroupRoleMappings)

		legacyCandidates := ssoLegacyPrincipalCandidates(result.Username, result.Email, result.NameID)
		roleErr := applySSORoleAssignments(authManager, principal, legacyCandidates, rolesToAssign, len(provider.GroupRoleMappings) > 0, false)
		if roleErr != nil {
			log.Error().Err(roleErr).Str("user", principal).Str("display_user", result.Username).Msg("Failed to update SAML user roles")
			if len(rolesToAssign) > 0 {
				LogAuditEventForTenant(GetOrgID(req.Context()), "saml_role_assignment", principal, GetClientIP(req), req.URL.Path, false, "Failed to auto-assign roles: "+strings.Join(rolesToAssign, ", "))
			}
		} else if len(rolesToAssign) > 0 {
			log.Info().
				Str("user", principal).
				Str("display_user", result.Username).
				Strs("mapped_roles", rolesToAssign).
				Msg("Auto-assigning roles based on SAML group mapping")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_role_assignment", principal, GetClientIP(req), req.URL.Path, true, "Auto-assigned roles: "+strings.Join(rolesToAssign, ", "))
		}
		if err := recordSSOIdentity(authManager, principal, internalauth.UserIdentityMetadata{
			DisplayName:  result.Username,
			Email:        result.Email,
			ProviderType: string(config.SSOProviderTypeSAML),
			ProviderID:   providerID,
			LastLoginAt:  time.Now(),
		}); err != nil {
			log.Error().Err(err).Str("user", principal).Str("display_user", result.Username).Msg("Failed to record SAML identity metadata")
		}
	}

	// Store SAML session info for potential SLO
	samlSession := &SAMLSessionInfo{
		ProviderID:   providerID,
		NameID:       result.NameID,
		SessionIndex: result.SessionIdx,
	}

	if err := r.establishSAMLSession(w, req, principal, result.Username, samlSession); err != nil {
		log.Error().Err(err).Msg("Failed to establish session after SAML login")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", principal, GetClientIP(req), req.URL.Path, false, "Session creation failed")
		r.redirectSAMLError(w, req, relayState, "session_failed")
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_login", principal, GetClientIP(req), req.URL.Path, true, "SAML login success via "+providerID)

	http.Redirect(w, req, buildLocalRedirectTarget(relayState, map[string]string{
		"saml": "success",
	}), http.StatusFound)
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

	if err := r.syncSAMLPublicURL(); err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to synchronize SAML public URL for metadata")
		writeErrorResponse(w, http.StatusInternalServerError, "saml_init_failed", "Failed to synchronize SAML provider", nil)
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

// handleSAMLLogout handles SP-initiated SAML Single Logout. It is POST-only,
// like the /api/logout fallback: the IdP's LogoutResponse returns to /slo, not
// here, so a GET to this path could only be a forced logout.
func (r *Router) handleSAMLLogout(w http.ResponseWriter, req *http.Request) {
	if !requireRequestMethod(w, req, http.MethodPost) {
		return
	}
	// The SAML routes are public and skip CSRF so the IdP flows can reach
	// them, and a cross-site POST withholds the SameSite=Lax session cookie.
	// A request carrying no session has nothing to log out, yet the deletion
	// cookies every logout path below writes would log out whichever session
	// that browser holds. Refuse it, as a deployment with authentication
	// configured refuses an unauthenticated /api/logout. SP-initiated SAML
	// logout always needs the browser session it ends.
	if samlSessionBindingKey(req) == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
		return
	}
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

	// Bind the LogoutRequest to the session it logs out before clearing it, so
	// /slo can tell the IdP's answer to this logout from a replayed one.
	sessionKey := samlSessionBindingKey(req)

	// Clear local session first
	r.clearSession(w, req)

	// Attempt SAML SLO
	logoutURL, err := service.MakeLogoutRequest(session.NameID, session.SessionIndex, sessionKey)
	if err != nil {
		log.Warn().Err(err).Str("provider_id", providerID).Msg("SAML SLO not available, local logout only")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_logout", "", GetClientIP(req), req.URL.Path, true, "Local logout only (SLO not available)")
		http.Redirect(w, req, "/?logout=success", http.StatusFound)
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_logout", "", GetClientIP(req), req.URL.Path, true, "Initiating SAML SLO")
	http.Redirect(w, req, logoutURL, http.StatusFound)
}

// handleSAMLSLO handles the IdP's signed LogoutResponse following an
// SP-initiated SLO (the redirect target our MakeLogoutRequest pointed at).
//
// The previous implementation cleared the user's session unconditionally on
// any POST or GET to this endpoint — an unauthenticated cross-origin force-
// logout DoS against any user with a SAML session. Verify the IdP's
// XML-DSig on the LogoutResponse before clearing anything; on validation
// failure log the audit event and refuse to mutate session state.
//
// A valid signature alone is not enough: the IdP signs its answer to every
// user's logout, and this route must take the HTTP-Redirect binding's GET,
// which carries the SameSite=Lax session cookie on a cross-site navigation and
// skips the CSRF check. The response must also answer an outstanding
// LogoutRequest from handleSAMLLogout, once, in a browser carrying either no
// session (handleSAMLLogout already cleared its cookie) or the session that
// request logged out. Anything else is a replay or a forced logout and is
// refused the same way. Completion writes no session state or cookie:
// handleSAMLLogout already did that before redirecting to the IdP.
func (r *Router) handleSAMLSLO(w http.ResponseWriter, req *http.Request) {
	providerID := extractSAMLProviderID(req.URL.Path, "slo")
	if providerID == "" || !validateProviderID(providerID) {
		http.NotFound(w, req)
		return
	}

	service := r.samlManager.GetService(providerID)
	if service == nil {
		http.NotFound(w, req)
		return
	}

	// Require either a SAMLResponse (response to our SLO request) or a
	// SAMLRequest (IdP-initiated SLO). Requests with neither are not real
	// SAML traffic and should not touch session state.
	if err := req.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	hasResponse := req.URL.Query().Get("SAMLResponse") != "" || req.PostForm.Get("SAMLResponse") != ""
	hasRequest := req.URL.Query().Get("SAMLRequest") != "" || req.PostForm.Get("SAMLRequest") != ""
	if !hasResponse && !hasRequest {
		log.Warn().
			Str("provider_id", providerID).
			Str("client_ip", GetClientIP(req)).
			Msg("SAML SLO endpoint hit with no SAMLResponse/SAMLRequest payload — refusing to clear session")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_slo_callback", "", GetClientIP(req), req.URL.Path, false, "Missing SAMLResponse/SAMLRequest payload")
		http.Error(w, "missing SAML payload", http.StatusBadRequest)
		return
	}

	// IdP-initiated LogoutRequest validation isn't wired up yet (no stored
	// in-flight request IDs to bind it to). Reject explicitly rather than
	// silently treating it as a force-logout signal.
	if hasRequest && !hasResponse {
		log.Warn().
			Str("provider_id", providerID).
			Str("client_ip", GetClientIP(req)).
			Msg("IdP-initiated SAML LogoutRequest received but not supported on this SP")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_slo_callback", "", GetClientIP(req), req.URL.Path, false, "IdP-initiated LogoutRequest not supported")
		http.Error(w, "IdP-initiated logout not supported", http.StatusBadRequest)
		return
	}

	if err := service.ValidateLogoutResponse(req, samlSessionBindingKey(req)); err != nil {
		if errors.Is(err, errSAMLLogoutResponseUnbound) {
			log.Warn().
				Err(err).
				Str("provider_id", providerID).
				Str("client_ip", GetClientIP(req)).
				Msg("SAML LogoutResponse does not answer an outstanding logout for this session — refusing to clear session")
			LogAuditEventForTenant(GetOrgID(req.Context()), "saml_slo_callback", "", GetClientIP(req), req.URL.Path, false, "LogoutResponse does not answer an outstanding logout request for this session")
			http.Error(w, "invalid LogoutResponse", http.StatusForbidden)
			return
		}
		log.Warn().
			Err(err).
			Str("provider_id", providerID).
			Str("client_ip", GetClientIP(req)).
			Msg("SAML LogoutResponse failed signature/validation — refusing to clear session")
		LogAuditEventForTenant(GetOrgID(req.Context()), "saml_slo_callback", "", GetClientIP(req), req.URL.Path, false, "LogoutResponse validation failed")
		http.Error(w, "invalid LogoutResponse", http.StatusForbidden)
		return
	}

	// handleSAMLLogout already invalidated the session this response answers
	// and expired its cookies. Write nothing here: a cross-site HTTP-POST
	// delivery withholds the SameSite=Lax session cookie, so this request
	// cannot see which session the browser holds, and deletion cookies would
	// log out whichever one that is.
	LogAuditEventForTenant(GetOrgID(req.Context()), "saml_slo_callback", "", GetClientIP(req), req.URL.Path, true, "SAML SLO complete")
	http.Redirect(w, req, "/?logout=success", http.StatusFound)
}

// SAMLSessionInfo stores SAML-specific session information for SLO
type SAMLSessionInfo struct {
	ProviderID   string `json:"providerId"`
	NameID       string `json:"nameId"`
	SessionIndex string `json:"sessionIndex"`
}

// establishSAMLSession creates a session for a SAML-authenticated user.
func (r *Router) establishSAMLSession(w http.ResponseWriter, req *http.Request, username, displayUsername string, samlInfo *SAMLSessionInfo) error {
	// Invalidate any pre-existing session to prevent session fixation attacks.
	InvalidateOldSessionFromRequest(req)

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
	GetSessionStore().CreateSAMLSessionWithDisplayName(token, 24*time.Hour, userAgent, clientIP, username, displayUsername, samlTokens)

	if username != "" {
		TrackUserSession(username, token)
	}

	csrfToken := generateCSRFToken(token)
	cookiePolicy := getBrowserCookiePolicy(req)

	cookiePolicy.setHTTPOnly(w, &http.Cookie{
		Name:     sessionCookieName(cookiePolicy.secure),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	cookiePolicy.setClientReadable(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  csrfToken,
		Path:   "/",
		MaxAge: 86400,
	})

	return nil
}

// getSAMLSessionInfo retrieves SAML session info from the current session
func (r *Router) getSAMLSessionInfo(req *http.Request) *SAMLSessionInfo {
	cookie, err := readSessionCookie(req)
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

// samlSessionBindingKey identifies the session the request's cookie selects,
// as the session store's token hash, or returns "" when the request carries
// none. SLO binds each LogoutRequest to it and checks it again when the IdP's
// LogoutResponse arrives.
func samlSessionBindingKey(req *http.Request) string {
	cookie, err := readSessionCookie(req)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return sessionHash(cookie.Value)
}

// The SAML login cookie binds each SP-initiated login to the browser that
// started it. handleSAMLLogin sets it to a random token and records the
// AuthnRequest against the token's hash; the ACS accepts the IdP's Response
// only from a browser presenting that token. Like the session cookie it uses
// the __Host- prefix over HTTPS, so a related subdomain cannot plant a token
// of its own.
const (
	cookieNameSAMLLogin       = "pulse_saml_login"
	cookieNameSAMLLoginSecure = "__Host-pulse_saml_login"
	samlLoginTokenBytes       = 32
	// samlACSRepostField marks the ACS form writeSAMLACSRepost re-posts.
	samlACSRepostField = "PulseSAMLRepost"
)

func samlLoginCookieName(secure bool) string {
	if secure {
		return cookieNameSAMLLoginSecure
	}
	return cookieNameSAMLLogin
}

// readSAMLLoginCookie reads the login cookie the way readSessionCookie reads
// the session cookie.
func readSAMLLoginCookie(req *http.Request) (*http.Cookie, error) {
	if isConnectionSecure(req) {
		return req.Cookie(cookieNameSAMLLoginSecure)
	}
	if cookie, err := req.Cookie(cookieNameSAMLLogin); err == nil {
		return cookie, nil
	}
	return req.Cookie(cookieNameSAMLLoginSecure)
}

// samlLoginToken returns the login token the browser already holds, so that
// logins started in several tabs all stay bound to it, or a new one.
func samlLoginToken(req *http.Request) (string, error) {
	if cookie, err := readSAMLLoginCookie(req); err == nil && validSAMLLoginToken(cookie.Value) {
		return cookie.Value, nil
	}
	return generateRandomURLString(samlLoginTokenBytes)
}

func validSAMLLoginToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(samlLoginTokenBytes) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil
}

// samlLoginBrowserKey returns the binding key of the login token the request
// carries, or "" when it carries none.
func samlLoginBrowserKey(req *http.Request) string {
	cookie, err := readSAMLLoginCookie(req)
	if err != nil || !validSAMLLoginToken(cookie.Value) {
		return ""
	}
	return sessionHash(cookie.Value)
}

var samlACSRepostPage = template.Must(template.New("saml-acs-repost").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Signing in to Pulse</title></head>
<body><form method="post">{{range .Fields}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">{{end}}<p>Finishing sign-in…</p><noscript><button type="submit">Continue</button></noscript></form>
<script nonce="{{.Nonce}}">document.forms[0].submit();</script></body></html>
`))

// writeSAMLACSRepost answers the IdP's HTTP-POST delivery of a Response that
// arrived without the login cookie. The IdP's page posts to the ACS from
// another site, and browsers withhold SameSite=Lax cookies from cross-site
// POSTs, so the cookie that binds the login to this browser is missing on
// arrival in every browser. The page posts the same fields once more from
// Pulse's own origin, which carries the cookie. The form has no action, so it
// returns to the URL the IdP posted to, path prefix included, and the marker
// field stops a second repost: a browser that still has no cookie started no
// login here, and the ACS refuses its Response.
func writeSAMLACSRepost(w http.ResponseWriter, req *http.Request) {
	type field struct{ Name, Value string }
	page := struct {
		Fields []field
		Nonce  string
	}{Nonce: CSPNonceFromContext(req.Context())}
	for _, name := range []string{"SAMLResponse", "SAMLart", "RelayState"} {
		if value := req.PostForm.Get(name); value != "" {
			page.Fields = append(page.Fields, field{Name: name, Value: value})
		}
	}
	page.Fields = append(page.Fields, field{Name: samlACSRepostField, Value: "1"})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := samlACSRepostPage.Execute(w, page); err != nil {
		log.Error().Err(err).Msg("Failed to write SAML ACS repost page")
	}
}

// clearSession clears the current session - properly invalidates server-side session
// and clears both pulse_session and pulse_csrf cookies
func (r *Router) clearSession(w http.ResponseWriter, req *http.Request) {
	cookiePolicy := getBrowserCookiePolicy(req)

	// Invalidate server-side session first
	if cookie, err := readSessionCookie(req); err == nil && cookie.Value != "" {
		// Get username before deleting session for untracking
		if username := GetSessionUsername(cookie.Value); username != "" {
			UntrackUserSession(username, cookie.Value)
		}
		GetSessionStore().InvalidateSession(cookie.Value)
		// Also clear the server-side CSRF token bound to this session.
		// InvalidateUserSessions and InvalidateOldSessionFromRequest already
		// delete CSRF state for their cases (password change, re-login);
		// logout was the missing-symmetry path — sessions were destroyed
		// but the matching CSRF entries lingered until their 4-hour TTL,
		// accumulating dead records on disk/memory.
		GetCSRFStore().DeleteCSRFToken(cookie.Value)
	}

	// Clear both session cookie variants (prefixed and unprefixed)
	for _, name := range []string{cookieNameSession, cookieNameSessionSecure} {
		cookiePolicy.setHTTPOnly(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
	}

	// Clear pulse_csrf cookie
	cookiePolicy.setClientReadable(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func (r *Router) redirectSAMLError(w http.ResponseWriter, req *http.Request, returnTo string, code string) {
	http.Redirect(w, req, buildLocalRedirectTarget(returnTo, map[string]string{
		"saml":       "error",
		"saml_error": code,
	}), http.StatusFound)
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
	if err := r.syncSAMLPublicURL(); err != nil {
		return fmt.Errorf("sync saml public url: %w", err)
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
	if err := r.syncSAMLPublicURL(); err != nil {
		return fmt.Errorf("sync saml public url: %w", err)
	}
	service := r.samlManager.GetService(providerID)
	if service == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return service.RefreshMetadata(ctx)
}
