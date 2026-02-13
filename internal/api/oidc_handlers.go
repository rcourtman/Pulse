package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

func (r *Router) handleOIDCLogin(w http.ResponseWriter, req *http.Request) {
	// Support both GET (direct redirect) and POST (JSON response)
	// GET is preferred for browsers as it guarantees same-window navigation
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET or POST is allowed", nil)
		return
	}

	cfg := r.ensureOIDCConfig()
	if cfg == nil || !cfg.Enabled {
		if req.Method == http.MethodGet {
			// Redirect back to login with error instead of plain text
			r.redirectOIDCError(w, req, "/", "oidc_disabled")
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "oidc_disabled", "OIDC authentication is not enabled", nil)
		return
	}

	// Build redirect URL from request (respects X-Forwarded-* headers)
	redirectURL := buildRedirectURL(req, cfg.RedirectURL)

	service, err := r.getOIDCService(req.Context(), redirectURL)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.IssuerURL).Msg("Failed to initialise OIDC service")
		if req.Method == http.MethodGet {
			// Redirect back to login with error instead of plain text
			r.redirectOIDCError(w, req, "/", "oidc_init_failed")
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "oidc_init_failed", "OIDC provider is unavailable", nil)
		return
	}

	log.Debug().Str("issuer", cfg.IssuerURL).Str("client_id", cfg.ClientID).Msg("Starting OIDC login flow")

	var returnTo string
	if req.Method == http.MethodPost {
		var payload struct {
			ReturnTo string `json:"returnTo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil && err != io.EOF {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request payload", nil)
			return
		}
		returnTo = sanitizeOIDCReturnTo(payload.ReturnTo)
	} else {
		// GET: read returnTo from query param
		returnTo = sanitizeOIDCReturnTo(req.URL.Query().Get("returnTo"))
	}

	state, entry, err := service.newStateEntry("", returnTo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create OIDC state entry")
		if req.Method == http.MethodGet {
			r.redirectOIDCError(w, req, "/", "oidc_state_error")
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "oidc_state_error", "Unable to start OIDC login", nil)
		return
	}

	authURL := service.authCodeURL(state, entry)

	// GET: direct HTTP redirect (guarantees same-window navigation in all browsers)
	// POST: return JSON (for API clients/backwards compatibility)
	if req.Method == http.MethodGet {
		http.Redirect(w, req, authURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authorizationUrl": authURL,
	})
}

func (r *Router) handleOIDCCallback(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	cfg := r.ensureOIDCConfig()
	if cfg == nil || !cfg.Enabled {
		http.Error(w, "OIDC is not enabled", http.StatusNotFound)
		return
	}

	// Build redirect URL from request (respects X-Forwarded-* headers)
	redirectURL := buildRedirectURL(req, cfg.RedirectURL)

	service, err := r.getOIDCService(req.Context(), redirectURL)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.IssuerURL).Msg("Failed to initialise OIDC service for callback")
		r.redirectOIDCError(w, req, "", "oidc_init_failed")
		return
	}

	log.Debug().Str("issuer", cfg.IssuerURL).Msg("Processing OIDC callback")

	query := req.URL.Query()
	if errParam := query.Get("error"); errParam != "" {
		log.Warn().Str("error", errParam).Msg("OIDC provider returned error")
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Provider error: "+errParam)
		r.redirectOIDCError(w, req, "", errParam)
		return
	}

	state := query.Get("state")
	if state == "" {
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing state parameter")
		r.redirectOIDCError(w, req, "", "missing_state")
		return
	}

	entry, ok := service.consumeState(state)
	if !ok {
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Invalid or expired state")
		r.redirectOIDCError(w, req, "", "invalid_state")
		return
	}

	code := query.Get("code")
	if code == "" {
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing authorization code")
		r.redirectOIDCError(w, req, entry.ReturnTo, "missing_code")
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	ctx = service.contextWithHTTPClient(ctx)

	token, err := service.exchangeCode(ctx, code, entry)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.IssuerURL).Msg("OIDC code exchange failed")
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Code exchange failed: "+err.Error())
		r.redirectOIDCError(w, req, entry.ReturnTo, "exchange_failed")
		return
	}

	log.Debug().Msg("OIDC code exchange successful")

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing ID token")
		r.redirectOIDCError(w, req, entry.ReturnTo, "missing_id_token")
		return
	}

	// Verify the ID token
	idToken, err := service.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		errorCode := "invalid_id_token"
		logMessage := "Failed to verify ID token - check issuer URL matches token issuer claim"
		if strings.Contains(err.Error(), "unexpected signature algorithm") {
			errorCode = "invalid_signature_alg"
			logMessage = "Failed to verify ID token - provider is issuing HS256 tokens, Pulse requires RS256"
		}
		log.Error().
			Err(err).
			Str("issuer", cfg.IssuerURL).
			Str("client_id", cfg.ClientID).
			Str("redirect_url", cfg.RedirectURL).
			Msg(logMessage)
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "ID token verification failed: "+err.Error())
		r.redirectOIDCError(w, req, entry.ReturnTo, errorCode)
		return
	}

	log.Debug().Str("subject", idToken.Subject).Msg("ID token verified successfully")

	claims := make(map[string]any)
	if err := idToken.Claims(&claims); err != nil {
		log.Error().Err(err).Msg("Failed to parse ID token claims")
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", "", GetClientIP(req), req.URL.Path, false, "Invalid token claims")
		r.redirectOIDCError(w, req, entry.ReturnTo, "invalid_claims")
		return
	}

	username := extractStringClaim(claims, cfg.UsernameClaim)
	email := extractStringClaim(claims, cfg.EmailClaim)
	if username == "" {
		username = email
	}
	if username == "" {
		username = extractStringClaim(claims, "name")
	}
	if username == "" {
		username = idToken.Subject
	}

	log.Debug().
		Str("username", username).
		Str("email", email).
		Str("subject", idToken.Subject).
		Str("username_claim", cfg.UsernameClaim).
		Str("email_claim", cfg.EmailClaim).
		Msg("Extracted user identity from claims")

	if len(cfg.AllowedEmails) > 0 && !matchesValue(email, cfg.AllowedEmails) {
		log.Debug().Str("email", email).Strs("allowed_emails", cfg.AllowedEmails).Msg("Email not in allowed list")
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", email, GetClientIP(req), req.URL.Path, false, "Email not permitted")
		r.redirectOIDCError(w, req, entry.ReturnTo, "email_restricted")
		return
	}

	if len(cfg.AllowedDomains) > 0 && !matchesDomain(email, cfg.AllowedDomains) {
		log.Debug().Str("email", email).Strs("allowed_domains", cfg.AllowedDomains).Msg("Email domain not in allowed list")
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", email, GetClientIP(req), req.URL.Path, false, "Email domain restricted")
		r.redirectOIDCError(w, req, entry.ReturnTo, "domain_restricted")
		return
	}

	if len(cfg.AllowedGroups) > 0 {
		groups := extractStringSliceClaim(claims, cfg.GroupsClaim)
		log.Debug().
			Strs("user_groups", groups).
			Strs("allowed_groups", cfg.AllowedGroups).
			Str("groups_claim", cfg.GroupsClaim).
			Msg("Checking group membership")
		if !intersects(groups, cfg.AllowedGroups) {
			log.Debug().Msg("User not in any allowed groups")
			LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", username, GetClientIP(req), req.URL.Path, false, "Group restriction failed")
			r.redirectOIDCError(w, req, entry.ReturnTo, "group_restricted")
			return
		}
		log.Debug().Msg("User group membership verified")
	}

	// RBAC Integration: Map OIDC groups to Pulse roles
	if authManager := internalauth.GetManager(); authManager != nil {
		groups := extractStringSliceClaim(claims, cfg.GroupsClaim)
		var rolesToAssign []string
		seenRoles := make(map[string]bool)

		for _, group := range groups {
			if roleID, ok := cfg.GroupRoleMappings[group]; ok {
				if !seenRoles[roleID] {
					rolesToAssign = append(rolesToAssign, roleID)
					seenRoles[roleID] = true
				}
			}
		}

		if len(rolesToAssign) > 0 {
			log.Info().
				Str("user", username).
				Strs("mapped_roles", rolesToAssign).
				Msg("Auto-assigning roles based on OIDC group mapping")
			if err := authManager.UpdateUserRoles(username, rolesToAssign); err != nil {
				log.Error().Err(err).Str("user", username).Msg("Failed to auto-assign OIDC roles")
				LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_role_assignment", username, GetClientIP(req), req.URL.Path, false, "Failed to auto-assign roles: "+strings.Join(rolesToAssign, ", "))
				// We don't fail the login here, but log the error
			} else {
				LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_role_assignment", username, GetClientIP(req), req.URL.Path, true, "Auto-assigned roles: "+strings.Join(rolesToAssign, ", "))
			}
		}
	}

	// Prepare OIDC token info for session storage (enables refresh token support)
	var oidcTokens *OIDCTokenInfo
	if token.RefreshToken != "" {
		oidcTokens = &OIDCTokenInfo{
			RefreshToken:   token.RefreshToken,
			AccessTokenExp: token.Expiry,
			Issuer:         cfg.IssuerURL,
			ClientID:       cfg.ClientID,
		}
		log.Debug().
			Time("access_token_expiry", token.Expiry).
			Bool("has_refresh_token", true).
			Msg("OIDC tokens will be stored for session refresh")
	}

	if err := r.establishOIDCSession(w, req, username, oidcTokens); err != nil {
		log.Error().Err(err).Msg("Failed to establish session after OIDC login")
		LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", username, GetClientIP(req), req.URL.Path, false, "Session creation failed")
		r.redirectOIDCError(w, req, entry.ReturnTo, "session_failed")
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "oidc_login", username, GetClientIP(req), req.URL.Path, true, "OIDC login success")

	target := entry.ReturnTo
	if target == "" {
		target = "/"
	}
	target = addQueryParam(target, "oidc", "success")
	http.Redirect(w, req, target, http.StatusFound)
}

func (r *Router) getOIDCService(ctx context.Context, redirectURL string) (*OIDCService, error) {
	cfg := r.ensureOIDCConfig()
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("oidc disabled")
	}

	r.oidcMu.Lock()
	defer r.oidcMu.Unlock()

	// Create a config clone with the dynamic redirect URL
	cfgWithRedirect := cfg.Clone()
	cfgWithRedirect.RedirectURL = redirectURL

	if r.oidcService != nil && r.oidcService.Matches(cfgWithRedirect) {
		return r.oidcService, nil
	}

	service, err := NewOIDCService(ctx, cfgWithRedirect)
	if err != nil {
		return nil, err
	}

	previous := r.oidcService
	r.oidcService = service
	if previous != nil {
		previous.Stop()
	}
	return service, nil
}

func sanitizeOIDCReturnTo(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return ""
	}
	return trimmed
}

func (r *Router) redirectOIDCError(w http.ResponseWriter, req *http.Request, returnTo string, code string) {
	target := returnTo
	if target == "" {
		target = "/"
	}
	target = addQueryParam(target, "oidc", "error")
	if code != "" {
		target = addQueryParam(target, "oidc_error", code)
	}

	http.Redirect(w, req, target, http.StatusFound)
}

func addQueryParam(path, key, value string) string {
	if path == "" {
		path = "/"
	}
	u, err := url.Parse(path)
	if err != nil {
		return path
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func extractStringClaim(claims map[string]any, key string) string {
	if key == "" {
		return ""
	}
	value, ok := claims[key]
	if !ok {
		return ""
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		if len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				return strings.TrimSpace(str)
			}
		}
	}

	return ""
}

func extractStringSliceClaim(claims map[string]any, key string) []string {
	if key == "" {
		return nil
	}
	value, ok := claims[key]
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case string:
		// Split on commas or spaces
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ' '
		})
		return parts
	default:
		return nil
	}
}

func matchesValue(candidate string, allowed []string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return false
	}
	for _, item := range allowed {
		if strings.ToLower(strings.TrimSpace(item)) == candidate {
			return true
		}
	}
	return false
}

func matchesDomain(email string, allowed []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at == -1 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	for _, item := range allowed {
		normalized := strings.ToLower(strings.Trim(strings.TrimSpace(item), "@"))
		if normalized != "" && domain == normalized {
			return true
		}
	}
	return false
}

func intersects(values []string, allowed []string) bool {
	if len(values) == 0 || len(allowed) == 0 {
		return false
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}
	for _, val := range values {
		if _, ok := allowedSet[strings.ToLower(strings.TrimSpace(val))]; ok {
			return true
		}
	}
	return false
}

func (r *Router) ensureOIDCConfig() *config.OIDCConfig {
	if r.config.OIDC == nil {
		r.config.OIDC = config.NewOIDCConfig()
		r.config.OIDC.ApplyDefaults(r.config.PublicURL)
	}
	return r.config.OIDC
}

// InitializeOIDCProviders initializes all enabled SSO OIDC providers at startup.
func (r *Router) InitializeOIDCProviders(ctx context.Context) error {
	if r.ssoConfig == nil {
		return nil
	}
	for _, provider := range r.ssoConfig.Providers {
		if provider.Type == config.SSOProviderTypeOIDC && provider.Enabled && provider.OIDC != nil {
			p := provider // capture loop variable
			if err := r.oidcManager.InitializeProvider(ctx, p.ID, &p, ""); err != nil {
				log.Error().
					Err(err).
					Str("provider_id", p.ID).
					Msg("Failed to initialize SSO OIDC provider")
				// Continue initializing other providers
			}
		}
	}
	return nil
}

// ssoProviderToOIDCConfig converts an SSO multi-provider config into the legacy
// OIDCConfig that NewOIDCService expects.
func ssoProviderToOIDCConfig(provider *config.SSOProvider, redirectURL string) *config.OIDCConfig {
	oidc := provider.OIDC
	scopes := oidc.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	return &config.OIDCConfig{
		Enabled:           true,
		IssuerURL:         oidc.IssuerURL,
		ClientID:          oidc.ClientID,
		ClientSecret:      oidc.ClientSecret,
		RedirectURL:       redirectURL,
		LogoutURL:         oidc.LogoutURL,
		Scopes:            scopes,
		UsernameClaim:     oidc.UsernameClaim,
		EmailClaim:        oidc.EmailClaim,
		GroupsClaim:       provider.GroupsClaim,
		AllowedGroups:     provider.AllowedGroups,
		AllowedDomains:    provider.AllowedDomains,
		AllowedEmails:     provider.AllowedEmails,
		GroupRoleMappings: provider.GroupRoleMappings,
		CABundle:          oidc.CABundle,
	}
}

// extractOIDCProviderID extracts the provider ID from an OIDC endpoint path.
// Expected paths: /api/oidc/{providerID}/login, /api/oidc/{providerID}/callback
func extractOIDCProviderID(urlPath, endpoint string) string {
	parts := strings.Split(strings.TrimPrefix(urlPath, "/"), "/")
	// parts: ["api", "oidc", "{providerID}", "{endpoint}"]
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "oidc" && parts[3] == endpoint {
		return parts[2]
	}
	return ""
}

// buildSSOOIDCCallbackURL constructs the callback URL for a multi-provider OIDC flow.
// The path includes the provider ID: /api/oidc/{providerID}/callback
func buildSSOOIDCCallbackURL(req *http.Request, providerID string, configuredURL string) string {
	if configured := strings.TrimSpace(configuredURL); configured != "" {
		return configured
	}

	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	peerIP := extractRemoteIP(req.RemoteAddr)
	trustedProxy := isTrustedProxyIP(peerIP)

	if trustedProxy {
		if proto := firstForwardedValue(req.Header.Get("X-Forwarded-Proto")); proto != "" {
			scheme = proto
		} else if proto := firstForwardedValue(req.Header.Get("X-Forwarded-Scheme")); proto != "" {
			scheme = proto
		}
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	switch scheme {
	case "https", "http":
	default:
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	rawHost := ""
	if trustedProxy {
		rawHost = firstForwardedValue(req.Header.Get("X-Forwarded-Host"))
	}
	if rawHost == "" {
		rawHost = req.Host
	}
	host, _ := sanitizeForwardedHost(rawHost)
	if host == "" {
		host = req.Host
	}

	return fmt.Sprintf("%s://%s/api/oidc/%s/callback", scheme, host, providerID)
}

// handleSSOOIDCLogin handles login for a multi-provider SSO OIDC provider.
// Path: /api/oidc/{providerID}/login
func (r *Router) handleSSOOIDCLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET or POST is allowed", nil)
		return
	}

	providerID := extractOIDCProviderID(req.URL.Path, "login")
	if providerID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Provider ID is required", nil)
		return
	}
	if !validateProviderID(providerID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Invalid provider ID format", nil)
		return
	}

	provider := r.getSSOProvider(providerID)
	if provider == nil || provider.Type != config.SSOProviderTypeOIDC || !provider.Enabled {
		writeErrorResponse(w, http.StatusNotFound, "provider_not_found", "OIDC provider not found or not enabled", nil)
		return
	}
	if provider.OIDC == nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_config", "OIDC configuration is missing", nil)
		return
	}

	redirectURL := buildSSOOIDCCallbackURL(req, providerID, provider.OIDC.RedirectURL)

	service := r.oidcManager.GetService(providerID)
	if service != nil {
		// Check if cached service config still matches (especially redirect URL which
		// is unknown at startup and may change behind different reverse proxies).
		expectedCfg := ssoProviderToOIDCConfig(provider, redirectURL)
		if !service.Matches(expectedCfg) {
			service = nil // force re-initialization with correct config
		}
	}
	if service == nil {
		if err := r.oidcManager.InitializeProvider(req.Context(), providerID, provider, redirectURL); err != nil {
			log.Error().Err(err).Str("provider_id", providerID).Str("issuer", provider.OIDC.IssuerURL).Msg("Failed to initialize SSO OIDC provider")
			if req.Method == http.MethodGet {
				r.redirectOIDCError(w, req, "/", "oidc_init_failed")
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, "oidc_init_failed", "OIDC provider is unavailable", nil)
			return
		}
		service = r.oidcManager.GetService(providerID)
	}
	if service == nil {
		log.Error().Str("provider_id", providerID).Msg("SSO OIDC service unavailable after initialization")
		if req.Method == http.MethodGet {
			r.redirectOIDCError(w, req, "/", "oidc_init_failed")
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "oidc_init_failed", "OIDC provider is unavailable", nil)
		return
	}

	var returnTo string
	if req.Method == http.MethodPost {
		var payload struct {
			ReturnTo string `json:"returnTo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil && err != io.EOF {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request payload", nil)
			return
		}
		returnTo = sanitizeOIDCReturnTo(payload.ReturnTo)
	} else {
		returnTo = sanitizeOIDCReturnTo(req.URL.Query().Get("returnTo"))
	}

	state, entry, err := service.newStateEntry(providerID, returnTo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create OIDC state entry")
		if req.Method == http.MethodGet {
			r.redirectOIDCError(w, req, "/", "oidc_state_error")
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "oidc_state_error", "Unable to start OIDC login", nil)
		return
	}

	authURL := service.authCodeURL(state, entry)

	LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login_initiated", "", GetClientIP(req), req.URL.Path, true, "Provider: "+providerID)

	if req.Method == http.MethodGet {
		http.Redirect(w, req, authURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authorizationUrl": authURL,
	})
}

// handleSSOOIDCCallback handles the callback for a multi-provider SSO OIDC provider.
// Path: /api/oidc/{providerID}/callback
func (r *Router) handleSSOOIDCCallback(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	providerID := extractOIDCProviderID(req.URL.Path, "callback")
	if providerID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Provider ID is required", nil)
		return
	}
	if !validateProviderID(providerID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_provider", "Invalid provider ID format", nil)
		return
	}

	provider := r.getSSOProvider(providerID)
	if provider == nil || provider.Type != config.SSOProviderTypeOIDC || !provider.Enabled {
		r.redirectOIDCError(w, req, "/", "provider_not_found")
		return
	}
	if provider.OIDC == nil {
		r.redirectOIDCError(w, req, "/", "invalid_config")
		return
	}

	redirectURL := buildSSOOIDCCallbackURL(req, providerID, provider.OIDC.RedirectURL)

	service := r.oidcManager.GetService(providerID)
	if service != nil {
		expectedCfg := ssoProviderToOIDCConfig(provider, redirectURL)
		if !service.Matches(expectedCfg) {
			service = nil
		}
	}
	if service == nil {
		if err := r.oidcManager.InitializeProvider(req.Context(), providerID, provider, redirectURL); err != nil {
			log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to initialize SSO OIDC provider for callback")
			r.redirectOIDCError(w, req, "/", "oidc_init_failed")
			return
		}
		service = r.oidcManager.GetService(providerID)
	}
	if service == nil {
		log.Error().Str("provider_id", providerID).Msg("SSO OIDC service unavailable after initialization for callback")
		r.redirectOIDCError(w, req, "/", "oidc_init_failed")
		return
	}

	query := req.URL.Query()
	if errParam := query.Get("error"); errParam != "" {
		log.Warn().Str("error", errParam).Str("provider_id", providerID).Msg("OIDC provider returned error")
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Provider error: "+errParam)
		r.redirectOIDCError(w, req, "", errParam)
		return
	}

	state := query.Get("state")
	if state == "" {
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing state parameter")
		r.redirectOIDCError(w, req, "", "missing_state")
		return
	}

	entry, ok := service.consumeState(state)
	if !ok {
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Invalid or expired state")
		r.redirectOIDCError(w, req, "", "invalid_state")
		return
	}

	// Safety check: verify the provider ID in the state matches the callback path.
	// SSO flow always stores providerID in state; reject any mismatch including empty.
	if entry.ProviderID != providerID {
		log.Warn().Str("state_provider", entry.ProviderID).Str("path_provider", providerID).Msg("OIDC provider ID mismatch between state and callback path")
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Provider ID mismatch")
		r.redirectOIDCError(w, req, entry.ReturnTo, "provider_mismatch")
		return
	}

	code := query.Get("code")
	if code == "" {
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing authorization code")
		r.redirectOIDCError(w, req, entry.ReturnTo, "missing_code")
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	ctx = service.contextWithHTTPClient(ctx)

	token, err := service.exchangeCode(ctx, code, entry)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Msg("OIDC code exchange failed")
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Code exchange failed: "+err.Error())
		r.redirectOIDCError(w, req, entry.ReturnTo, "exchange_failed")
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing ID token")
		r.redirectOIDCError(w, req, entry.ReturnTo, "missing_id_token")
		return
	}

	idToken, err := service.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		errorCode := "invalid_id_token"
		if strings.Contains(err.Error(), "unexpected signature algorithm") {
			errorCode = "invalid_signature_alg"
		}
		log.Error().Err(err).Str("provider_id", providerID).Msg("Failed to verify ID token")
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "ID token verification failed: "+err.Error())
		r.redirectOIDCError(w, req, entry.ReturnTo, errorCode)
		return
	}

	// Verify nonce matches what we sent in the authorization request
	if idToken.Nonce != entry.Nonce {
		log.Warn().Str("provider_id", providerID).Msg("OIDC nonce mismatch — possible token replay")
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", "", GetClientIP(req), req.URL.Path, false, "Nonce mismatch")
		r.redirectOIDCError(w, req, entry.ReturnTo, "nonce_mismatch")
		return
	}

	claims := make(map[string]any)
	if err := idToken.Claims(&claims); err != nil {
		log.Error().Err(err).Msg("Failed to parse ID token claims")
		r.redirectOIDCError(w, req, entry.ReturnTo, "invalid_claims")
		return
	}

	// Use SSO provider config for claim names
	usernameClaim := provider.OIDC.UsernameClaim
	if usernameClaim == "" {
		usernameClaim = "preferred_username"
	}
	emailClaim := provider.OIDC.EmailClaim
	if emailClaim == "" {
		emailClaim = "email"
	}

	username := extractStringClaim(claims, usernameClaim)
	email := extractStringClaim(claims, emailClaim)
	if username == "" {
		username = email
	}
	if username == "" {
		username = extractStringClaim(claims, "name")
	}
	if username == "" {
		username = idToken.Subject
	}

	// Apply access restrictions from SSO provider config
	if len(provider.AllowedEmails) > 0 && !matchesValue(email, provider.AllowedEmails) {
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", email, GetClientIP(req), req.URL.Path, false, "Email not permitted")
		r.redirectOIDCError(w, req, entry.ReturnTo, "email_restricted")
		return
	}
	if len(provider.AllowedDomains) > 0 && !matchesDomain(email, provider.AllowedDomains) {
		LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", email, GetClientIP(req), req.URL.Path, false, "Email domain restricted")
		r.redirectOIDCError(w, req, entry.ReturnTo, "domain_restricted")
		return
	}
	if len(provider.AllowedGroups) > 0 {
		groups := extractStringSliceClaim(claims, provider.GroupsClaim)
		if !intersects(groups, provider.AllowedGroups) {
			LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", username, GetClientIP(req), req.URL.Path, false, "Group restriction failed")
			r.redirectOIDCError(w, req, entry.ReturnTo, "group_restricted")
			return
		}
	}

	// RBAC: Always call UpdateUserRoles so user appears in Users list
	if authManager := internalauth.GetManager(); authManager != nil {
		groups := extractStringSliceClaim(claims, provider.GroupsClaim)
		var rolesToAssign []string
		seenRoles := make(map[string]bool)
		for _, group := range groups {
			if roleID, ok := provider.GroupRoleMappings[group]; ok {
				if !seenRoles[roleID] {
					rolesToAssign = append(rolesToAssign, roleID)
					seenRoles[roleID] = true
				}
			}
		}
		if err := authManager.UpdateUserRoles(username, rolesToAssign); err != nil {
			log.Error().Err(err).Str("user", username).Msg("Failed to update SSO OIDC user roles")
		} else if len(rolesToAssign) > 0 {
			LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_role_assignment", username, GetClientIP(req), req.URL.Path, true, "Auto-assigned roles: "+strings.Join(rolesToAssign, ", "))
		}
	}

	// Store OIDC tokens for session refresh
	var oidcTokens *OIDCTokenInfo
	if token.RefreshToken != "" {
		oidcTokens = &OIDCTokenInfo{
			RefreshToken:   token.RefreshToken,
			AccessTokenExp: token.Expiry,
			Issuer:         provider.OIDC.IssuerURL,
			ClientID:       provider.OIDC.ClientID,
		}
	}

	if err := r.establishOIDCSession(w, req, username, oidcTokens); err != nil {
		log.Error().Err(err).Msg("Failed to establish session after SSO OIDC login")
		r.redirectOIDCError(w, req, entry.ReturnTo, "session_failed")
		return
	}

	LogAuditEventForTenant(GetOrgID(req.Context()), "sso_oidc_login", username, GetClientIP(req), req.URL.Path, true, "SSO OIDC login success via provider: "+providerID)

	target := entry.ReturnTo
	if target == "" {
		target = "/"
	}
	target = addQueryParam(target, "oidc", "success")
	http.Redirect(w, req, target, http.StatusFound)
}

// buildRedirectURL constructs the OIDC redirect URL from the incoming request,
// respecting X-Forwarded-* headers when behind a reverse proxy
func buildRedirectURL(req *http.Request, configuredURL string) string {
	// If explicitly configured, use that
	if configured := strings.TrimSpace(configuredURL); configured != "" {
		return configured
	}

	// Build from request headers (respects reverse proxy headers)
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	peerIP := extractRemoteIP(req.RemoteAddr)
	trustedProxy := isTrustedProxyIP(peerIP)

	if trustedProxy {
		if proto := firstForwardedValue(req.Header.Get("X-Forwarded-Proto")); proto != "" {
			scheme = proto
		} else if proto := firstForwardedValue(req.Header.Get("X-Forwarded-Scheme")); proto != "" {
			scheme = proto
		}
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	switch scheme {
	case "https", "http":
	default:
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	rawHost := ""
	if trustedProxy {
		rawHost = firstForwardedValue(req.Header.Get("X-Forwarded-Host"))
	}
	if rawHost == "" {
		rawHost = req.Host
	}
	host, _ := sanitizeForwardedHost(rawHost)
	if host == "" {
		host = req.Host
	}

	redirectURL := fmt.Sprintf("%s://%s%s", scheme, host, config.DefaultOIDCCallbackPath)

	log.Debug().
		Str("scheme", scheme).
		Str("host", host).
		Str("x_forwarded_proto", req.Header.Get("X-Forwarded-Proto")).
		Str("x_forwarded_host", req.Header.Get("X-Forwarded-Host")).
		Bool("trusted_proxy", trustedProxy).
		Str("peer_ip", peerIP).
		Str("redirect_url", redirectURL).
		Bool("has_tls", req.TLS != nil).
		Msg("Built OIDC redirect URL from request")

	return redirectURL
}
