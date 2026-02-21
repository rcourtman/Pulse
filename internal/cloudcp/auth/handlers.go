package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auditlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const handoffTTL = 60 * time.Second

type adminGenerateMagicLinkRequest struct {
	Email     string `json:"email"`
	TenantID  string `json:"tenant_id"`
	SendEmail bool   `json:"send_email"`
}

type adminGenerateMagicLinkResponse struct {
	URL       string `json:"url"`
	Email     string `json:"email"`
	TenantID  string `json:"tenant_id"`
	EmailSent bool   `json:"email_sent"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HandleMagicLinkVerify returns an http.HandlerFunc that validates a control-plane
// magic link token, generates a short-lived handoff token, and redirects the user
// to the tenant container.
func HandleMagicLinkVerify(svc *Service, reg *registry.TenantRegistry, tenantsDir, baseDomain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}

		tokenStr := strings.TrimSpace(r.URL.Query().Get("token"))
		if tokenStr == "" {
			auditEvent(r, "cp_magic_link_verify", "failure").
				Str("reason", "missing_token").
				Msg("Magic link verification failed")
			writeError(w, http.StatusBadRequest, "missing_token", "Token parameter is required")
			return
		}

		token, err := svc.ValidateToken(tokenStr)
		if err != nil {
			auditEvent(r, "cp_magic_link_verify", "failure").
				Err(err).
				Str("reason", "invalid_or_expired_token").
				Msg("Magic link verification failed")
			// Browser redirect on failure.
			if !strings.Contains(r.Header.Get("Accept"), "application/json") {
				http.Redirect(w, r, "/login?error=magic_link_invalid", http.StatusTemporaryRedirect)
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired magic link")
			return
		}

		// Look up tenant to confirm it exists and is active.
		tenant, err := reg.Get(token.TenantID)
		if err != nil || tenant == nil {
			auditEvent(r, "cp_magic_link_verify", "failure").
				Err(err).
				Str("tenant_id", token.TenantID).
				Str("reason", "tenant_not_found").
				Msg("Magic link verification failed")
			writeError(w, http.StatusNotFound, "tenant_not_found", "Tenant not found")
			return
		}

		// Read the per-tenant handoff key.
		tenantDataDir := filepath.Join(tenantsDir, tenant.ID)
		handoffKeyPath := filepath.Join(tenantDataDir, cloudauth.HandoffKeyFile)
		handoffKey, err := os.ReadFile(handoffKeyPath)
		if err != nil {
			auditEvent(r, "cp_magic_link_verify", "failure").
				Err(err).
				Str("tenant_id", tenant.ID).
				Str("reason", "handoff_key_read_failed").
				Msg("Magic link verification failed")
			writeError(w, http.StatusInternalServerError, "handoff_error", "Unable to generate handoff")
			return
		}

		// Sign a short-lived handoff token.
		handoffToken, err := cloudauth.Sign(handoffKey, token.Email, tenant.ID, handoffTTL)
		if err != nil {
			auditEvent(r, "cp_magic_link_verify", "failure").
				Err(err).
				Str("tenant_id", tenant.ID).
				Str("reason", "handoff_sign_failed").
				Msg("Magic link verification failed")
			writeError(w, http.StatusInternalServerError, "handoff_error", "Unable to generate handoff")
			return
		}

		// Build redirect URL: https://<tenant-id>.<baseDomain>/auth/cloud-handoff?token=Y
		redirectURL := fmt.Sprintf("https://%s.%s/auth/cloud-handoff?token=%s",
			tenant.ID, baseDomain, handoffToken)

		if userID, err := ensureAccountUserAndMembership(reg, tenant, token.Email); err != nil {
			log.Warn().
				Err(err).
				Str("tenant_id", tenant.ID).
				Str("email", token.Email).
				Msg("Failed to establish control-plane session identity")
		} else if sessionToken, err := svc.GenerateSessionToken(userID, token.Email, SessionTTL); err != nil {
			log.Warn().
				Err(err).
				Str("tenant_id", tenant.ID).
				Str("email", token.Email).
				Msg("Failed to issue control-plane session")
		} else {
			http.SetCookie(w, &http.Cookie{
				Name:     SessionCookieName,
				Value:    sessionToken,
				Path:     "/",
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(SessionTTL.Seconds()),
				Expires:  time.Now().UTC().Add(SessionTTL),
			})
		}

		auditEvent(r, "cp_magic_link_verify", "success").
			Str("tenant_id", tenant.ID).
			Str("email", token.Email).
			Msg("Magic link verified, redirecting to tenant handoff")

		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

// HandleAdminGenerateMagicLink returns an admin-only handler that generates a magic
// link for a given email + tenant_id. The caller is responsible for wrapping this
// with AdminKeyMiddleware.
//
// If send_email is true in the request, the magic link is also emailed to the user.
func HandleAdminGenerateMagicLink(svc *Service, baseURL string, emailSender email.Sender, emailFrom string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}

		var req adminGenerateMagicLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			auditEvent(r, "cp_magic_link_admin_generate", "failure").
				Str("reason", "bad_request").
				Msg("Admin magic link generation failed")
			writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
			return
		}
		if req.Email == "" || req.TenantID == "" {
			auditEvent(r, "cp_magic_link_admin_generate", "failure").
				Str("reason", "missing_email_or_tenant_id").
				Msg("Admin magic link generation failed")
			writeError(w, http.StatusBadRequest, "bad_request", "email and tenant_id are required")
			return
		}

		token, err := svc.GenerateToken(req.Email, req.TenantID)
		if err != nil {
			auditEvent(r, "cp_magic_link_admin_generate", "failure").
				Err(err).
				Str("email", req.Email).
				Str("tenant_id", req.TenantID).
				Str("reason", "token_generation_failed").
				Msg("Admin magic link generation failed")
			writeError(w, http.StatusInternalServerError, "generate_error", "Failed to generate magic link")
			return
		}

		magicURL := BuildVerifyURL(baseURL, token)
		emailSent := false

		if req.SendEmail && emailSender != nil && emailFrom != "" {
			html, text, renderErr := email.RenderMagicLinkEmail(email.MagicLinkData{MagicLinkURL: magicURL})
			if renderErr != nil {
				log.Error().
					Err(renderErr).
					Str("tenant_id", req.TenantID).
					Str("email", req.Email).
					Msg("Failed to render magic link email")
			} else if sendErr := emailSender.Send(r.Context(), email.Message{
				From:    emailFrom,
				To:      req.Email,
				Subject: "Sign in to Pulse",
				HTML:    html,
				Text:    text,
			}); sendErr != nil {
				log.Error().
					Err(sendErr).
					Str("tenant_id", req.TenantID).
					Str("email", req.Email).
					Msg("Failed to send magic link email")
			} else {
				emailSent = true
			}
		}

		auditEvent(r, "cp_magic_link_admin_generate", "success").
			Str("email", req.Email).
			Str("tenant_id", req.TenantID).
			Bool("email_sent", emailSent).
			Msg("Admin generated magic link")

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"url":        magicURL,
			"email":      req.Email,
			"tenant_id":  req.TenantID,
			"email_sent": emailSent,
		}); err != nil {
			log.Error().Err(err).Msg("cloudcp.auth: encode admin magic link response")
		}
	}
}

func auditEvent(r *http.Request, eventName, outcome string) *zerolog.Event {
	e := log.Info()
	if outcome != "success" {
		e = log.Warn()
	}

	actorID := auditlog.ActorID(r)
	if actorID == "" {
		actorID = "admin_key"
	}

	return e.
		Str("audit_event", eventName).
		Str("outcome", outcome).
		Str("actor_id", actorID).
		Str("client_ip", auditlog.ClientIP(r)).
		Str("method", r.Method).
		Str("path", auditlog.RequestPath(r))
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	}); err != nil {
		log.Error().Err(err).Msg("cloudcp.auth: encode error response")
	}
}

func ensureAccountUserAndMembership(reg *registry.TenantRegistry, tenant *registry.Tenant, email string) (string, error) {
	if reg == nil {
		return "", fmt.Errorf("registry unavailable")
	}
	if tenant == nil {
		return "", fmt.Errorf("tenant is required")
	}
	accountID := strings.TrimSpace(tenant.AccountID)
	if accountID == "" {
		return "", fmt.Errorf("tenant has no account id")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	user, err := reg.GetUserByEmail(email)
	if err != nil {
		return "", fmt.Errorf("lookup user by email: %w", err)
	}
	if user == nil {
		userID, genErr := registry.GenerateUserID()
		if genErr != nil {
			return "", fmt.Errorf("generate user id: %w", genErr)
		}
		candidate := &registry.User{
			ID:    userID,
			Email: email,
		}
		if createErr := reg.CreateUser(candidate); createErr != nil {
			reloaded, reloadErr := reg.GetUserByEmail(email)
			if reloadErr != nil || reloaded == nil {
				return "", fmt.Errorf("create user: %w", createErr)
			}
			user = reloaded
		} else {
			user = candidate
		}
	}
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return "", fmt.Errorf("user resolution failed")
	}

	m, err := reg.GetMembership(accountID, user.ID)
	if err != nil {
		return "", fmt.Errorf("lookup membership: %w", err)
	}
	if m == nil {
		newMembership := &registry.AccountMembership{
			AccountID: accountID,
			UserID:    user.ID,
			Role:      registry.MemberRoleOwner,
		}
		if createErr := reg.CreateMembership(newMembership); createErr != nil {
			reloaded, reloadErr := reg.GetMembership(accountID, user.ID)
			if reloadErr != nil || reloaded == nil {
				return "", fmt.Errorf("create membership: %w", createErr)
			}
		}
	}

	_ = reg.UpdateUserLastLogin(user.ID)
	return user.ID, nil
}
