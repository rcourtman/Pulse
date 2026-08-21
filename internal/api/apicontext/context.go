package apicontext

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// OrganizationContextKey is shared by tenant middleware and domain handlers.
type OrganizationContextKey string

const (
	OrgIDContextKey OrganizationContextKey = "org_id"
	OrgContextKey   OrganizationContextKey = "org_object"
)

// OrgID returns the selected tenant or the historical default tenant.
func OrgID(ctx context.Context) string {
	if id, ok := ctx.Value(OrgIDContextKey).(string); ok {
		return id
	}
	return "default"
}

// Organization returns the selected organization or the default organization.
func Organization(ctx context.Context) *models.Organization {
	if org, ok := ctx.Value(OrgContextKey).(*models.Organization); ok {
		return org
	}
	return &models.Organization{ID: "default", DisplayName: "Default Organization"}
}
