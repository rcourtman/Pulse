package api

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type OrganizationContextKey string

const (
	OrgIDContextKey OrganizationContextKey = "org_id"
	OrgContextKey   OrganizationContextKey = "org_object"
)

// TenantMiddleware extracts the organization ID from the request and
// sets up the context for multi-tenant isolation.
type TenantMiddleware struct {
	persistence *config.MultiTenantPersistence
}

func NewTenantMiddleware(p *config.MultiTenantPersistence) *TenantMiddleware {
	return &TenantMiddleware{persistence: p}
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

		// 2. Validate/Load Organization
		// In a real implementation, we would check if the user has access to this org.
		// For Phase 1 (Persistence), we just ensure the org is valid in the persistence layer.

		// Ensure the organization persistence is initialized
		// This creates the directory if it doesn't exist for valid IDs
		_, err := m.persistence.GetPersistence(orgID)
		if err != nil {
			http.Error(w, "Invalid Organization ID", http.StatusBadRequest)
			return
		}

		// 3. Inject into Context
		ctx := context.WithValue(r.Context(), OrgIDContextKey, orgID)

		// Also store a mock organization object for now
		org := &models.Organization{ID: orgID, DisplayName: orgID}
		ctx = context.WithValue(ctx, OrgContextKey, org)

		next.ServeHTTP(w, r.WithContext(ctx))
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
