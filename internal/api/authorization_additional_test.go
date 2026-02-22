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
	checker := NewAuthorizationChecker(nil)
	if checker.CanAccessOrg("user", nil, "default") {
		t.Fatalf("expected default org access denial without organization loader")
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
	checker := NewAuthorizationChecker(staticOrgLoader{
		org: &models.Organization{
			ID: "default",
			Members: []models.OrganizationMember{
				{UserID: "owner", Role: models.OrgRoleOwner},
			},
		},
	})

	if checker.CanAccessOrg("user", nil, "default") {
		t.Fatalf("expected default org membership enforcement to deny non-member")
	}
	if !checker.CanAccessOrg("owner", nil, "default") {
		t.Fatalf("expected owner to access default org")
	}
}
