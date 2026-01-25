package api

import "testing"

func TestMultiTenantOrganizationLoader_NoPersistence(t *testing.T) {
	loader := NewMultiTenantOrganizationLoader(nil)
	if _, err := loader.GetOrganization("org"); err == nil {
		t.Fatalf("expected error when persistence is nil")
	}
}

func TestDefaultAuthorizationChecker_CanAccessOrg_Default(t *testing.T) {
	checker := NewAuthorizationChecker(nil)
	if !checker.CanAccessOrg("user", nil, "default") {
		t.Fatalf("expected default org access")
	}
}
