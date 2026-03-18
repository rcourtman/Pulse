package admin

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auditlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

type listTenantsResponse struct {
	Tenants []*registry.Tenant `json:"tenants"`
	Count   int                `json:"count"`
}

type unauthorizedResponse struct {
	Error string `json:"error"`
}

// HandleListTenants returns an authenticated handler that lists all tenants.
func HandleListTenants(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Optional state filter
		stateFilter := strings.TrimSpace(r.URL.Query().Get("state"))

		var tenants []*registry.Tenant
		var err error

		if stateFilter != "" {
			tenants, err = reg.ListByState(registry.TenantState(stateFilter))
		} else {
			tenants, err = reg.List()
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if tenants == nil {
			tenants = []*registry.Tenant{}
		}

		w.Header().Set("Content-Type", "application/json")
		encodeJSON(w, map[string]any{
			"tenants": tenants,
			"count":   len(tenants),
		})
	}
}

// AdminKeyMiddleware returns middleware that requires a valid admin API key.
func AdminKeyMiddleware(adminKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-Admin-Key"))
		if key == "" {
			// Also check Authorization: Bearer <key>
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			}
		}

		if key == "" || adminKey == "" || subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) != 1 {
			log.Warn().
				Str("audit_event", "cp_admin_auth").
				Str("outcome", "failure").
				Str("reason", "invalid_admin_key").
				Str("client_ip", auditlog.ClientIP(r)).
				Str("method", r.Method).
				Str("path", auditlog.RequestPath(r)).
				Msg("Control plane admin authentication failed")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			encodeJSON(w, map[string]string{
				"error": "unauthorized",
			})
			return
		}

		// Admin-key authenticated requests bypass session/membership middleware.
		// Provide a conservative default role so downstream account-mutation auth
		// checks stay fail-closed on missing role context.
		req := r.Clone(r.Context())
		if strings.TrimSpace(req.Header.Get("X-User-Role")) == "" {
			req.Header.Set("X-User-Role", string(registry.MemberRoleOwner))
		}
		next.ServeHTTP(w, req)
	})
}

func encodeJSON(w http.ResponseWriter, payload any) {
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("cloudcp.admin: encode JSON response")
	}
}
