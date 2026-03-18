package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestMultiTenantOrganizationLoader_NoPersistence(t *testing.T) {
	loader := NewMultiTenantOrganizationLoader(nil)
	if _, err := loader.GetOrganization("org"); err == nil {
		t.Fatalf("expected error when persistence is nil")
	}
}

func TestDefaultAuthorizationChecker_CanAccessOrg_Default(t *testing.T) {
	// Default org is always accessible to any authenticated user,
	// even without an organization loader configured.
	checker := NewAuthorizationChecker(nil)
	if !checker.CanAccessOrg("user", nil, "default") {
		t.Fatalf("expected default org to be accessible to any authenticated user")
	}
}

type staticOrgLoader struct {
	org *models.Organization
	err error
}

func (s staticOrgLoader) GetOrganization(string) (*models.Organization, error) {
	return s.org, s.err
}

func TestDefaultAuthorizationChecker_CanAccessOrg_DefaultWithMembershipConfigured(t *testing.T) {
	// Default org is always accessible to any authenticated user,
	// regardless of membership configuration.
	checker := NewAuthorizationChecker(staticOrgLoader{
		org: &models.Organization{
			ID: "default",
			Members: []models.OrganizationMember{
				{UserID: "owner", Role: models.OrgRoleOwner},
			},
		},
	})

	// Both member and non-member should be able to access the default org.
	if !checker.CanAccessOrg("user", nil, "default") {
		t.Fatalf("expected default org to be accessible to non-member user")
	}
	if !checker.CanAccessOrg("owner", nil, "default") {
		t.Fatalf("expected default org to be accessible to owner")
	}
}
