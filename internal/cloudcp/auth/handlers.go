package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	"github.com/rs/zerolog/log"
)

const handoffTTL = 60 * time.Second

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

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}
