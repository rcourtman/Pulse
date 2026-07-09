package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const magicLinkRequestBodyLimit = 16 * 1024

type MagicLinkHandlers struct {
	persistence      *config.MultiTenantPersistence
	service          *MagicLinkService
	hostedMode       bool
	resolvePublicURL func(*http.Request) string
}

func NewMagicLinkHandlers(
	persistence *config.MultiTenantPersistence,
	service *MagicLinkService,
	hostedMode bool,
	resolvePublicURL func(*http.Request) string,
) *MagicLinkHandlers {
	return &MagicLinkHandlers{
		persistence:      persistence,
		service:          service,
		hostedMode:       hostedMode,
		resolvePublicURL: resolvePublicURL,
	}
}

type magicLinkPrincipal struct {
	UserID string
	Email  string
	Role   models.OrganizationRole
}

func (h *MagicLinkHandlers) HandlePublicMagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.persistence == nil || h.service == nil {
		// Don't pretend success if the system is misconfigured.
		writeErrorResponse(w, http.StatusServiceUnavailable, "magic_links_unavailable", "Magic link service is not configured", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, magicLinkRequestBodyLimit)
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	email := strings.TrimSpace(req.Email)
	if !isValidSignupEmail(email) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_email", "Invalid email format", nil)
		return
	}

	// Always return 200 to avoid leaking whether the email exists.
	// Rate limiting is still enforced by silently not sending additional links.
	if !h.service.AllowRequest(email) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "If that email is registered, you'll receive a magic link shortly.",
		})
		return
	}

	orgID, ok, err := h.findOrgForEmail(email)
	if err != nil {
		log.Warn().Err(err).Str("email", email).Msg("Magic link request: failed to resolve org")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "If that email is registered, you'll receive a magic link shortly.",
		})
		return
	}
	if !ok {
		// Unknown email: don't send mail, but still return 200.
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "If that email is registered, you'll receive a magic link shortly.",
		})
		return
	}

	token, err := h.service.GenerateToken(email, orgID)
	if err != nil {
		log.Warn().Err(err).Str("email", email).Str("org_id", orgID).Msg("Magic link request: failed to generate token")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "If that email is registered, you'll receive a magic link shortly.",
		})
		return
	}

	baseURL := ""
	if h.resolvePublicURL != nil {
		baseURL = h.resolvePublicURL(r)
	}
	// Hosted mode must never fall back to request Host header (host header injection risk).
	// If public URL isn't configured, fail closed by not sending any email.
	if baseURL == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "If that email is registered, you'll receive a magic link shortly.",
		})
		return
	}

	if err := h.service.SendMagicLink(email, orgID, token, baseURL); err != nil {
		log.Warn().Err(err).Str("email", email).Str("org_id", orgID).Msg("Magic link request: failed to send email")
		// Still 200 to avoid revealing existence.
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "If that email is registered, you'll receive a magic link shortly.",
	})
}

func (h *MagicLinkHandlers) HandlePublicMagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.service == nil {
		http.Error(w, "Magic link service not configured", http.StatusServiceUnavailable)
		return
	}

	tokenStr := strings.TrimSpace(r.URL.Query().Get("token"))
	token, err := h.service.ValidateToken(tokenStr)
	if err != nil {
		// For browser flows, redirect back to login instead of showing a raw error.
		if !strings.Contains(r.Header.Get("Accept"), "application/json") {
			http.Redirect(w, r, "/login?error=magic_link_invalid", http.StatusTemporaryRedirect)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "invalid_magic_link", "Invalid or expired magic link", nil)
		return
	}
	if !isValidOrganizationID(token.OrgID) {
		// Reject malformed org IDs from storage to avoid cookie/context poisoning.
		writeErrorResponse(w, http.StatusBadRequest, "invalid_magic_link", "Invalid or expired magic link", nil)
		return
	}
	principal, err := h.resolveMagicLinkPrincipal(token.OrgID, token.Email)
	if err != nil {
		log.Warn().Err(err).Str("org_id", token.OrgID).Str("email", token.Email).Msg("Magic link verify: failed to resolve stable principal")
		if !strings.Contains(r.Header.Get("Accept"), "application/json") {
			http.Redirect(w, r, "/login?error=magic_link_invalid", http.StatusTemporaryRedirect)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "invalid_magic_link", "Invalid or expired magic link", nil)
		return
	}

	// Invalidate any pre-existing session to prevent session fixation attacks.
	InvalidateOldSessionFromRequest(r)

	// Create a session (reusing the existing session infrastructure).
	sessionToken := generateSessionToken()
	if sessionToken == "" {
		writeErrorResponse(w, http.StatusInternalServerError, "session_error", "Failed to create session", nil)
		return
	}

	userAgent := r.Header.Get("User-Agent")
	clientIP := GetClientIP(r)
	sessionDuration := 24 * time.Hour
	GetSessionStore().CreateSession(sessionToken, sessionDuration, userAgent, clientIP, principal.UserID)
	TrackUserSession(principal.UserID, sessionToken)

	csrfToken := generateCSRFToken(sessionToken)
	cookiePolicy := getBrowserCookiePolicy(r)
	cookieMaxAge := int(sessionDuration.Seconds())

	cookiePolicy.set(w, &http.Cookie{
		Name:     sessionCookieName(cookiePolicy.secure),
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   cookieMaxAge,
	})
	cookiePolicy.set(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  csrfToken,
		Path:   "/",
		MaxAge: cookieMaxAge,
	})
	// Org cookie is intentionally NOT HttpOnly — the frontend reads/writes it to
	// synchronize org context for WebSocket connections (which cannot send custom headers).
	cookiePolicy.set(w, &http.Cookie{
		Name:   CookieNameOrgID,
		Value:  token.OrgID,
		Path:   "/",
		MaxAge: cookieMaxAge,
	})

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"org_id":  token.OrgID,
			"user_id": principal.UserID,
			"email":   principal.Email,
		})
		return
	}

	http.Redirect(w, r, "/infrastructure", http.StatusTemporaryRedirect)
}

func (h *MagicLinkHandlers) findOrgForEmail(email string) (string, bool, error) {
	orgs, err := h.persistence.ListOrganizations()
	if err != nil {
		return "", false, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", false, nil
	}

	for _, org := range orgs {
		if org == nil {
			continue
		}
		userID, role, ok := org.ResolvePrincipalByEmail(email)
		if ok && strings.TrimSpace(userID) != "" && models.IsValidOrganizationRole(role) {
			return org.ID, true, nil
		}
	}
	return "", false, nil
}

func (h *MagicLinkHandlers) resolveMagicLinkPrincipal(orgID, email string) (*magicLinkPrincipal, error) {
	if h == nil || h.persistence == nil {
		return nil, fmt.Errorf("organization persistence is not configured")
	}
	orgID = strings.TrimSpace(orgID)
	email = strings.ToLower(strings.TrimSpace(email))
	if !isValidOrganizationID(orgID) || email == "" {
		return nil, fmt.Errorf("invalid magic link principal input")
	}

	org, err := h.persistence.LoadOrganizationStrict(orgID)
	if err != nil {
		return nil, fmt.Errorf("load organization %s: %w", orgID, err)
	}
	userID, role, ok := org.ResolvePrincipalByEmail(email)
	if !ok {
		return nil, fmt.Errorf("no organization principal for magic link email")
	}
	if userID == "" || !models.IsValidOrganizationRole(role) {
		return nil, fmt.Errorf("invalid organization principal for magic link email")
	}
	return &magicLinkPrincipal{
		UserID: userID,
		Email:  email,
		Role:   role,
	}, nil
}
