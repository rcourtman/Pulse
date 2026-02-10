package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// RequireOrgOwnerOrPlatformAdmin restricts access to routes scoped by a path org ID (`{id}`).
//
// Allowed callers:
// - Platform admin:
//   - Basic auth (configured AuthUser/AuthPass)
//   - Proxy auth with the configured admin role
//   - Dev bypass (ALLOW_ADMIN_BYPASS in dev)
//
// - Org owner:
//   - Session/OIDC/proxy user matching org.OwnerUserID
//
// - Org-bound API token:
//   - Token that is authorized for the target org via token.CanAccessOrg(orgID)
//
// This is intentionally stricter than TenantMiddleware membership checks; "owner" is required.
func RequireOrgOwnerOrPlatformAdmin(cfg *config.Config, orgs OrgPersistenceProvider, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Dev bypass (disabled by default).
		if adminBypassEnabled() {
			handler(w, r)
			return
		}

		// Authenticate first. We intentionally do not reuse RequireAdmin because
		// RequireAdmin treats *all* authenticated non-proxy users as "admin".
		if !CheckAuth(cfg, w, r) {
			// Match RequireAdmin's behavior for API routes.
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		orgID := strings.TrimSpace(r.PathValue("id"))
		if !isValidOrganizationID(orgID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID", nil)
			return
		}

		// Promote auth info into request context for downstream checks.
		r = extractAndStoreAuthContext(cfg, nil, r)

		// Platform admin checks.
		authMethod := strings.TrimSpace(w.Header().Get("X-Auth-Method"))
		switch authMethod {
		case "basic":
			handler(w, r)
			return
		case "proxy":
			if cfg != nil && cfg.ProxyAuthSecret != "" {
				if valid, username, isAdmin := CheckProxyAuth(cfg, r); valid && isAdmin {
					log.Debug().Str("user", username).Str("org_id", orgID).Msg("Allowing platform admin via proxy auth")
					handler(w, r)
					return
				}
			}
		case "bypass":
			handler(w, r)
			return
		}

		// Org-bound API token checks.
		if authMethod == "api_token" {
			if token := internalauth.GetAPIToken(r.Context()); token != nil {
				if record, ok := token.(*config.APITokenRecord); ok && record != nil && record.CanAccessOrg(orgID) {
					handler(w, r)
					return
				}
			}
			writeErrorResponse(w, http.StatusForbidden, "access_denied", "Token is not authorized for this organization", nil)
			return
		}

		// Owner check for session/OIDC/proxy users.
		userID := internalauth.GetUser(r.Context())
		if strings.TrimSpace(userID) == "" {
			writeErrorResponse(w, http.StatusForbidden, "access_denied", "Organization owner or platform admin required", nil)
			return
		}

		if orgs == nil {
			// Fail closed: don't allow cross-tenant admin actions if we can't verify ownership.
			writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
			return
		}

		// Avoid leaking existence: invalid org IDs are already handled above; if the org doesn't exist
		// or can't be loaded, treat it as not found (consistent with handler-side behavior elsewhere).
		if orgID != "default" && !orgs.OrgExists(orgID) {
			writeErrorResponse(w, http.StatusNotFound, "org_not_found", "Organization not found", nil)
			return
		}

		org, err := orgs.LoadOrganization(orgID)
		if err != nil || org == nil {
			writeErrorResponse(w, http.StatusNotFound, "org_not_found", "Organization not found", nil)
			return
		}

		if !strings.EqualFold(strings.TrimSpace(org.OwnerUserID), strings.TrimSpace(userID)) {
			writeErrorResponse(w, http.StatusForbidden, "access_denied", "Organization owner or platform admin required", nil)
			return
		}

		handler(w, r)
	}
}
