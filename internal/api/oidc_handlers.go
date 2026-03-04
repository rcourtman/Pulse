package api

import (
	"context"
	"encoding/json"
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
