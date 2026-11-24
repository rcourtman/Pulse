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

type importKeepsAccessCredentialsKey struct{}

// WithImportKeepingAccessCredentials marks a configuration import admitted for
// a caller who may not issue API tokens or write SSO configuration, so the
// import must leave both as the destination has them.
func WithImportKeepingAccessCredentials(ctx context.Context) context.Context {
	return context.WithValue(ctx, importKeepsAccessCredentialsKey{}, true)
}

// ImportKeepsAccessCredentials reports whether the transfer boundary marked
// this import with WithImportKeepingAccessCredentials.
func ImportKeepsAccessCredentials(ctx context.Context) bool {
	kept, _ := ctx.Value(importKeepsAccessCredentialsKey{}).(bool)
	return kept
}
