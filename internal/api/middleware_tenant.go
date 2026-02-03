package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

type OrganizationContextKey string

const (
	OrgIDContextKey    OrganizationContextKey = "org_id"
	OrgContextKey      OrganizationContextKey = "org_object"
	APITokenContextKey OrganizationContextKey = "api_token_record"
)

// TenantMiddleware extracts the organization ID from the request and
// sets up the context for multi-tenant isolation.
type TenantMiddleware struct {
	persistence *config.MultiTenantPersistence
	authChecker AuthorizationChecker
}

// TenantMiddlewareConfig holds configuration for the tenant middleware.
type TenantMiddlewareConfig struct {
	Persistence *config.MultiTenantPersistence
	AuthChecker AuthorizationChecker
}

func NewTenantMiddleware(p *config.MultiTenantPersistence) *TenantMiddleware {
	return &TenantMiddleware{persistence: p}
}

// NewTenantMiddlewareWithConfig creates a new TenantMiddleware with full configuration.
func NewTenantMiddlewareWithConfig(cfg TenantMiddlewareConfig) *TenantMiddleware {
	return &TenantMiddleware{
		persistence: cfg.Persistence,
		authChecker: cfg.AuthChecker,
	}
}

// SetAuthChecker sets the authorization checker for the middleware.
func (m *TenantMiddleware) SetAuthChecker(checker AuthorizationChecker) {
	m.authChecker = checker
}

func (m *TenantMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract Org ID
		// Priority:
		// 1. Header: X-Pulse-Org-ID (for API clients/agents)
		// 2. Cookie: pulse_org_id (for browser session)
		// 3. Fallback: "default" (for backward compatibility)

		orgID := r.Header.Get("X-Pulse-Org-ID")
		if orgID == "" {
			// Check cookie
			if cookie, err := r.Cookie("pulse_org_id"); err == nil {
				orgID = cookie.Value
			}
		}

		// Fallback to default
		if orgID == "" {
			orgID = "default"
		}

		// 2. Validate Organization Exists (only for non-default orgs)
		// This must check existence WITHOUT creating directories to prevent DoS.
		// It also must run BEFORE feature checks to ensure invalid org IDs return 400 (Bad Request)
		// rather than 501/402 (feature disabled/unlicensed).
		if orgID != "default" && m.persistence != nil {
			if !m.persistence.OrgExists(orgID) {
				writeJSONError(w, http.StatusBadRequest, "invalid_org", "Invalid Organization ID")
				return
			}
		}

		// 3. Feature flag and License Check for multi-tenant access
		// Non-default orgs require:
		// 1. Feature flag enabled (PULSE_MULTI_TENANT_ENABLED=true) - returns 501 if disabled
		// 2. Enterprise license - returns 402 if unlicensed
		if orgID != "default" {
			// Check feature flag first - 501 Not Implemented if disabled
			if !IsMultiTenantEnabled() {
				writeMultiTenantDisabledError(w)
				return
			}
			// Feature is enabled, check license - 402 Payment Required if unlicensed
			checkCtx := context.WithValue(r.Context(), OrgIDContextKey, orgID)
			if !hasMultiTenantFeatureForContext(checkCtx) {
				writeMultiTenantRequiredError(w)
				return
			}
		}

		// 4. Authorization Check
		// Check if the authenticated user/token is allowed to access this organization
		// Note: This runs AFTER AuthContextMiddleware, so auth context is available
		if m.authChecker != nil && orgID != "default" {
			// Get API token from context (set by AuthContextMiddleware)
			var token *config.APITokenRecord
			if tokenVal := auth.GetAPIToken(r.Context()); tokenVal != nil {
				if t, ok := tokenVal.(*config.APITokenRecord); ok {
					token = t
				}
			}

			// Get user ID from context (set by AuthContextMiddleware)
			userID := auth.GetUser(r.Context())

			// Only perform authorization check if we have auth context
			// If no auth context, the route's RequireAuth will handle authentication errors
			if token != nil || userID != "" {
				// Perform authorization check using the interface method
				result := m.authChecker.CheckAccess(token, userID, orgID)
				if !result.Allowed {
					log.Warn().
						Str("org_id", orgID).
						Str("user_id", userID).
						Str("reason", result.Reason).
						Msg("Unauthorized access attempt to organization")
					writeJSONError(w, http.StatusForbidden, "access_denied", result.Reason)
					return
				}

				// Log warning for legacy tokens accessing non-default orgs
				if result.IsLegacyToken {
					log.Warn().
						Str("org_id", orgID).
						Msg("Legacy token with wildcard access used - consider binding to specific org")
				}
			}
		}

		// 5. Inject into Context
		ctx := context.WithValue(r.Context(), OrgIDContextKey, orgID)

		// Also store a mock organization object for now
		org := &models.Organization{ID: orgID, DisplayName: orgID}
		ctx = context.WithValue(ctx, OrgContextKey, org)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// Helper to get OrgID from context
func GetOrgID(ctx context.Context) string {
	if id, ok := ctx.Value(OrgIDContextKey).(string); ok {
		return id
	}
	return "default"
}

// Helper to get Organization from context
func GetOrganization(ctx context.Context) *models.Organization {
	if org, ok := ctx.Value(OrgContextKey).(*models.Organization); ok {
		return org
	}
	return &models.Organization{ID: "default", DisplayName: "Default Organization"}
}
