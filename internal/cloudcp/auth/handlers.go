package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tokenStr := strings.TrimSpace(r.URL.Query().Get("token"))
		if tokenStr == "" {
			writeError(w, http.StatusBadRequest, "missing_token", "Token parameter is required")
			return
		}

		token, err := svc.ValidateToken(tokenStr)
		if err != nil {
			log.Warn().Err(err).Msg("Magic link verification failed")
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
			log.Error().Err(err).Str("tenant_id", token.TenantID).Msg("Tenant not found for magic link")
			writeError(w, http.StatusNotFound, "tenant_not_found", "Tenant not found")
			return
		}

		// Read the per-tenant handoff key.
		tenantDataDir := filepath.Join(tenantsDir, tenant.ID)
		handoffKeyPath := filepath.Join(tenantDataDir, cloudauth.HandoffKeyFile)
		handoffKey, err := os.ReadFile(handoffKeyPath)
		if err != nil {
			log.Error().Err(err).Str("tenant_id", tenant.ID).Msg("Failed to read handoff key")
			writeError(w, http.StatusInternalServerError, "handoff_error", "Unable to generate handoff")
			return
		}

		// Sign a short-lived handoff token.
		handoffToken, err := cloudauth.Sign(handoffKey, token.Email, tenant.ID, handoffTTL)
		if err != nil {
			log.Error().Err(err).Str("tenant_id", tenant.ID).Msg("Failed to sign handoff token")
			writeError(w, http.StatusInternalServerError, "handoff_error", "Unable to generate handoff")
			return
		}

		// Build redirect URL: https://<tenant-id>.<baseDomain>/auth/cloud-handoff?token=Y
		redirectURL := fmt.Sprintf("https://%s.%s/auth/cloud-handoff?token=%s",
			tenant.ID, baseDomain, handoffToken)

		log.Info().
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req adminGenerateMagicLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
			return
		}
		if req.Email == "" || req.TenantID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "email and tenant_id are required")
			return
		}

		token, err := svc.GenerateToken(req.Email, req.TenantID)
		if err != nil {
			log.Error().Err(err).Str("email", req.Email).Str("tenant_id", req.TenantID).Msg("Failed to generate magic link")
			writeError(w, http.StatusInternalServerError, "generate_error", "Failed to generate magic link")
			return
		}

		magicURL := BuildVerifyURL(baseURL, token)
		emailSent := false

		if req.SendEmail && emailSender != nil && emailFrom != "" {
			html, text, renderErr := email.RenderMagicLinkEmail(email.MagicLinkData{MagicLinkURL: magicURL})
			if renderErr != nil {
				log.Error().Err(renderErr).Msg("Failed to render magic link email")
			} else if sendErr := emailSender.Send(r.Context(), email.Message{
				From:    emailFrom,
				To:      req.Email,
				Subject: "Sign in to Pulse",
				HTML:    html,
				Text:    text,
			}); sendErr != nil {
				log.Error().Err(sendErr).Str("to", req.Email).Msg("Failed to send magic link email")
			} else {
				emailSent = true
			}
		}

		log.Info().
			Str("email", req.Email).
			Str("tenant_id", req.TenantID).
			Bool("email_sent", emailSent).
			Msg("Admin generated magic link")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(adminGenerateMagicLinkResponse{
			URL:       magicURL,
			Email:     req.Email,
			TenantID:  req.TenantID,
			EmailSent: emailSent,
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error:   code,
		Message: message,
	})
}
