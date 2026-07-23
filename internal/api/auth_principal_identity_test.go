package api

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type ssoIdentityRBACManager struct {
	auth.Manager
	assignments  map[string]auth.UserRoleAssignment
	updatedUser  string
	updatedRoles []string
}

func (m *ssoIdentityRBACManager) GetUserAssignment(username string) (auth.UserRoleAssignment, bool) {
	if m.assignments == nil {
		return auth.UserRoleAssignment{}, false
	}
	assignment, ok := m.assignments[username]
	return assignment, ok
}

func (m *ssoIdentityRBACManager) UpdateUserRoles(username string, roleIDs []string) error {
	m.updatedUser = username
	m.updatedRoles = append([]string{}, roleIDs...)
	if m.assignments == nil {
		m.assignments = make(map[string]auth.UserRoleAssignment)
	}
	m.assignments[username] = auth.UserRoleAssignment{Username: username, RoleIDs: append([]string{}, roleIDs...)}
	return nil
}

func TestStableSSOPrincipalUsesProviderScopedOpaqueSubject(t *testing.T) {
	principal, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "okta", "alice@example.com")
	if err != nil {
		t.Fatalf("stableSSOPrincipal error: %v", err)
	}
	again, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "okta", "alice@example.com")
	if err != nil {
		t.Fatalf("stableSSOPrincipal repeat error: %v", err)
	}
	otherProvider, err := stableSSOPrincipal(config.SSOProviderTypeOIDC, "entra", "alice@example.com")
	if err != nil {
		t.Fatalf("stableSSOPrincipal other provider error: %v", err)
	}

	if principal != again {
		t.Fatalf("stable principal is not deterministic: %q != %q", principal, again)
	}
	if principal == otherProvider {
		t.Fatal("stable principal must be scoped by provider")
	}
	if !strings.HasPrefix(principal, "sso:oidc:okta:") {
		t.Fatalf("principal %q missing expected provider scope", principal)
	}
	if strings.Contains(principal, "alice") || strings.Contains(principal, "example.com") {
		t.Fatalf("principal %q must not expose the raw email subject", principal)
	}
}

func TestStableSSOPrincipalRequiresSubject(t *testing.T) {
	if _, err := stableSSOPrincipal(config.SSOProviderTypeSAML, "okta", ""); err == nil {
		t.Fatal("expected missing subject to fail")
	}
}

func TestApplySSORoleAssignmentsMigratesLegacyRoles(t *testing.T) {
	manager := &ssoIdentityRBACManager{
		assignments: map[string]auth.UserRoleAssignment{
			"alice@example.com": {Username: "alice@example.com", RoleIDs: []string{"admin", "viewer"}},
		},
	}
	principal := "sso:oidc:okta:stable"

	err := applySSORoleAssignments(manager, principal, ssoLegacyPrincipalCandidates("Alice@Example.com"), nil, false, true)
	if err != nil {
		t.Fatalf("applySSORoleAssignments error: %v", err)
	}

	if manager.updatedUser != principal {
		t.Fatalf("updated user = %q, want %q", manager.updatedUser, principal)
	}
	if got := strings.Join(manager.updatedRoles, ","); got != "admin,viewer" {
		t.Fatalf("updated roles = %q, want admin,viewer", got)
	}
}

func TestApplySSORoleAssignmentsUsesAuthoritativeMapping(t *testing.T) {
	manager := &ssoIdentityRBACManager{
		assignments: map[string]auth.UserRoleAssignment{
			"alice@example.com": {Username: "alice@example.com", RoleIDs: []string{"admin"}},
		},
	}
	principal := "sso:saml:okta:stable"

	err := applySSORoleAssignments(manager, principal, []string{"alice@example.com"}, []string{"viewer"}, true, true)
	if err != nil {
		t.Fatalf("applySSORoleAssignments error: %v", err)
	}

	if manager.updatedUser != principal {
		t.Fatalf("updated user = %q, want %q", manager.updatedUser, principal)
	}
	if got := strings.Join(manager.updatedRoles, ","); got != "viewer" {
		t.Fatalf("updated roles = %q, want viewer", got)
	}
}

func TestApplySSORoleAssignmentsMovesLegacySQLiteAlias(t *testing.T) {
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()
	if err := manager.UpdateUserRoles("alice@example.com", []string{auth.RoleViewer}); err != nil {
		t.Fatalf("seed legacy assignment: %v", err)
	}
	principal := "sso:oidc:okta:stable"

	if err := applySSORoleAssignments(
		manager,
		principal,
		[]string{"alice@example.com"},
		nil,
		false,
		true,
	); err != nil {
		t.Fatalf("applySSORoleAssignments: %v", err)
	}

	assignment, ok := manager.GetUserAssignment(principal)
	if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != auth.RoleViewer {
		t.Fatalf("canonical assignment = %#v, exists=%v", assignment, ok)
	}
	if stale, ok := manager.GetUserAssignment("alice@example.com"); ok {
		t.Fatalf("legacy alias retained a reusable grant: %#v", stale)
	}
}

func TestApplySSORoleAssignmentsRejectsConflictingLegacySQLiteAlias(t *testing.T) {
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()
	if err := manager.UpdateUserRoles("alice@example.com", []string{auth.RoleAdmin}); err != nil {
		t.Fatalf("seed legacy assignment: %v", err)
	}
	principal := "sso:oidc:okta:stable"
	if err := manager.UpdateUserRoles(principal, []string{auth.RoleViewer}); err != nil {
		t.Fatalf("seed canonical assignment: %v", err)
	}

	err = applySSORoleAssignments(
		manager,
		principal,
		[]string{"alice@example.com"},
		nil,
		false,
		true,
	)
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error = %v, want a conflicting-grant failure", err)
	}
	canonical, ok := manager.GetUserAssignment(principal)
	if !ok || len(canonical.RoleIDs) != 1 || canonical.RoleIDs[0] != auth.RoleViewer {
		t.Fatalf("canonical assignment changed: %#v, exists=%v", canonical, ok)
	}
	legacy, ok := manager.GetUserAssignment("alice@example.com")
	if !ok || len(legacy.RoleIDs) != 1 || legacy.RoleIDs[0] != auth.RoleAdmin {
		t.Fatalf("legacy assignment changed before conflict resolution: %#v, exists=%v", legacy, ok)
	}
}
