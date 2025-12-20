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
	"github.com/rs/zerolog/log"
)

func (r *Router) handleOIDCLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	cfg := r.ensureOIDCConfig()
	if cfg == nil || !cfg.Enabled {
		writeErrorResponse(w, http.StatusBadRequest, "oidc_disabled", "OIDC authentication is not enabled", nil)
		return
	}

	// Build redirect URL from request (respects X-Forwarded-* headers)
	redirectURL := buildRedirectURL(req, cfg.RedirectURL)

	service, err := r.getOIDCService(req.Context(), redirectURL)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.IssuerURL).Msg("Failed to initialise OIDC service")
		writeErrorResponse(w, http.StatusInternalServerError, "oidc_init_failed", "OIDC provider is unavailable", nil)
		return
	}

	log.Debug().Str("issuer", cfg.IssuerURL).Str("client_id", cfg.ClientID).Msg("Starting OIDC login flow")

	var payload struct {
		ReturnTo string `json:"returnTo"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil && err != io.EOF {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request payload", nil)
		return
	}

	returnTo := sanitizeOIDCReturnTo(payload.ReturnTo)

	state, entry, err := service.newStateEntry(returnTo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create OIDC state entry")
		writeErrorResponse(w, http.StatusInternalServerError, "oidc_state_error", "Unable to start OIDC login", nil)
		return
	}

	authURL := service.authCodeURL(state, entry)

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
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Provider error: "+errParam)
		r.redirectOIDCError(w, req, "", errParam)
		return
	}

	state := query.Get("state")
	if state == "" {
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing state parameter")
		r.redirectOIDCError(w, req, "", "missing_state")
		return
	}

	entry, ok := service.consumeState(state)
	if !ok {
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Invalid or expired state")
		r.redirectOIDCError(w, req, "", "invalid_state")
		return
	}

	code := query.Get("code")
	if code == "" {
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing authorization code")
		r.redirectOIDCError(w, req, entry.ReturnTo, "missing_code")
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	ctx = service.contextWithHTTPClient(ctx)

	token, err := service.exchangeCode(ctx, code, entry)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.IssuerURL).Msg("OIDC code exchange failed")
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Code exchange failed: "+err.Error())
		r.redirectOIDCError(w, req, entry.ReturnTo, "exchange_failed")
		return
	}

	log.Debug().Msg("OIDC code exchange successful")

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Missing ID token")
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
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "ID token verification failed: "+err.Error())
		r.redirectOIDCError(w, req, entry.ReturnTo, errorCode)
		return
	}

	log.Debug().Str("subject", idToken.Subject).Msg("ID token verified successfully")

	claims := make(map[string]any)
	if err := idToken.Claims(&claims); err != nil {
		log.Error().Err(err).Msg("Failed to parse ID token claims")
		LogAuditEvent("oidc_login", "", GetClientIP(req), req.URL.Path, false, "Invalid token claims")
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
		LogAuditEvent("oidc_login", email, GetClientIP(req), req.URL.Path, false, "Email not permitted")
		r.redirectOIDCError(w, req, entry.ReturnTo, "email_restricted")
		return
	}

	if len(cfg.AllowedDomains) > 0 && !matchesDomain(email, cfg.AllowedDomains) {
		log.Debug().Str("email", email).Strs("allowed_domains", cfg.AllowedDomains).Msg("Email domain not in allowed list")
		LogAuditEvent("oidc_login", email, GetClientIP(req), req.URL.Path, false, "Email domain restricted")
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
			LogAuditEvent("oidc_login", username, GetClientIP(req), req.URL.Path, false, "Group restriction failed")
			r.redirectOIDCError(w, req, entry.ReturnTo, "group_restricted")
			return
		}
		log.Debug().Msg("User group membership verified")
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
		LogAuditEvent("oidc_login", username, GetClientIP(req), req.URL.Path, false, "Session creation failed")
		r.redirectOIDCError(w, req, entry.ReturnTo, "session_failed")
		return
	}

	LogAuditEvent("oidc_login", username, GetClientIP(req), req.URL.Path, true, "OIDC login success")

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

	r.oidcService = service
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

// buildRedirectURL constructs the OIDC redirect URL from the incoming request,
// respecting X-Forwarded-* headers when behind a reverse proxy
func buildRedirectURL(req *http.Request, configuredURL string) string {
	// If explicitly configured, use that
	if strings.TrimSpace(configuredURL) != "" {
		return configuredURL
	}

	// Build from request headers (respects reverse proxy headers)
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	// Check X-Forwarded-Proto header (set by reverse proxies)
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := req.Host
	// Check X-Forwarded-Host header (set by reverse proxies)
	if fwdHost := req.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	redirectURL := fmt.Sprintf("%s://%s%s", scheme, host, config.DefaultOIDCCallbackPath)

	log.Debug().
		Str("scheme", scheme).
		Str("host", host).
		Str("x_forwarded_proto", req.Header.Get("X-Forwarded-Proto")).
		Str("x_forwarded_host", req.Header.Get("X-Forwarded-Host")).
		Str("redirect_url", redirectURL).
		Bool("has_tls", req.TLS != nil).
		Msg("Built OIDC redirect URL from request")

	return redirectURL
}
