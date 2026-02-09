package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestRBACIsolation_RoleNotVisibleAcrossOrgs(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"
	mustCreateIsolationOrgDir(t, baseDir, orgA)
	mustCreateIsolationOrgDir(t, baseDir, orgB)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	managerA, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}
	managerB, err := provider.GetManager(orgB)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgB, err)
	}

	roleID := "org-a-ops-role"
	role := auth.Role{
		ID:          roleID,
		Name:        "Org A Ops",
		Description: "Scoped to org A only",
		Permissions: []auth.Permission{
			{Action: "write", Resource: "nodes"},
		},
	}
	if err := managerA.SaveRole(role); err != nil {
		t.Fatalf("SaveRole(%s) in %s failed: %v", roleID, orgA, err)
	}

	if _, ok := managerA.GetRole(roleID); !ok {
		t.Fatalf("expected role %q to exist in %s", roleID, orgA)
	}
	if _, ok := managerB.GetRole(roleID); ok {
		t.Fatalf("role %q leaked from %s to %s", roleID, orgA, orgB)
	}

	for _, r := range managerB.GetRoles() {
		if r.ID == roleID {
			t.Fatalf("role %q found in %s role listing", roleID, orgB)
		}
	}
}

func TestRBACIsolation_UserAssignmentNotVisibleAcrossOrgs(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"
	mustCreateIsolationOrgDir(t, baseDir, orgA)
	mustCreateIsolationOrgDir(t, baseDir, orgB)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	managerA, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}
	managerB, err := provider.GetManager(orgB)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgB, err)
	}

	roleID := "org-a-custom-reader"
	if err := managerA.SaveRole(auth.Role{
		ID:          roleID,
		Name:        "Org A Reader",
		Description: "Read role for org A",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
	}); err != nil {
		t.Fatalf("SaveRole(%s) failed: %v", roleID, err)
	}

	if err := managerA.UpdateUserRoles("alice", []string{roleID}); err != nil {
		t.Fatalf("UpdateUserRoles(alice) in %s failed: %v", orgA, err)
	}

	assignmentA, ok := managerA.GetUserAssignment("alice")
	if !ok {
		t.Fatalf("expected alice assignment in %s", orgA)
	}
	if len(assignmentA.RoleIDs) != 1 || assignmentA.RoleIDs[0] != roleID {
		t.Fatalf("unexpected alice assignment in %s: %+v", orgA, assignmentA)
	}

	if _, ok := managerB.GetUserAssignment("alice"); ok {
		t.Fatalf("alice assignment leaked from %s to %s", orgA, orgB)
	}

	permsA := managerA.GetUserPermissions("alice")
	if !containsPermission(permsA, "read", "nodes") {
		t.Fatalf("expected read:nodes permission for alice in %s, got %+v", orgA, permsA)
	}

	permsB := managerB.GetUserPermissions("alice")
	if len(permsB) != 0 {
		t.Fatalf("expected no permissions for alice in %s, got %+v", orgB, permsB)
	}
}

func TestRBACIsolation_SameUsernameIndependentRoles(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"
	mustCreateIsolationOrgDir(t, baseDir, orgA)
	mustCreateIsolationOrgDir(t, baseDir, orgB)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	managerA, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}
	managerB, err := provider.GetManager(orgB)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgB, err)
	}

	if err := managerA.SaveRole(auth.Role{
		ID:          "ops",
		Name:        "Ops",
		Description: "Operations role",
		Permissions: []auth.Permission{
			{Action: "write", Resource: "nodes"},
		},
	}); err != nil {
		t.Fatalf("SaveRole(ops) in %s failed: %v", orgA, err)
	}

	if err := managerA.UpdateUserRoles("admin", []string{"ops"}); err != nil {
		t.Fatalf("UpdateUserRoles(admin, ops) in %s failed: %v", orgA, err)
	}
	if err := managerB.UpdateUserRoles("admin", []string{auth.RoleViewer}); err != nil {
		t.Fatalf("UpdateUserRoles(admin, viewer) in %s failed: %v", orgB, err)
	}

	permsA := managerA.GetUserPermissions("admin")
	if !containsPermission(permsA, "write", "nodes") {
		t.Fatalf("expected write:nodes in %s for admin, got %+v", orgA, permsA)
	}

	permsB := managerB.GetUserPermissions("admin")
	if containsPermission(permsB, "write", "nodes") {
		t.Fatalf("unexpected write:nodes in %s for admin, got %+v", orgB, permsB)
	}
}

func TestRBACIsolation_ChangelogScopedToOrg(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"
	mustCreateIsolationOrgDir(t, baseDir, orgA)
	mustCreateIsolationOrgDir(t, baseDir, orgB)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	managerA, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}
	managerB, err := provider.GetManager(orgB)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgB, err)
	}

	roleID := "org-a-audit-test"
	if err := managerA.SaveRoleWithContext(auth.Role{
		ID:          roleID,
		Name:        "Audit Test Role",
		Description: "Role for changelog isolation test",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
	}, "alice"); err != nil {
		t.Fatalf("SaveRoleWithContext(%s) in %s failed: %v", roleID, orgA, err)
	}

	logsA := managerA.GetChangeLogsForEntity("role", roleID)
	if len(logsA) == 0 {
		t.Fatalf("expected changelog entry in %s for role %q", orgA, roleID)
	}

	logsB := managerB.GetChangeLogs(100, 0)
	if len(logsB) != 0 {
		t.Fatalf("expected no changelog entries in %s, got %d", orgB, len(logsB))
	}
}

func TestRBACIsolation_DefaultOrgIsolatedFromNamedOrgs(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	mustCreateIsolationOrgDir(t, baseDir, orgA)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	defaultManager, err := provider.GetManager("default")
	if err != nil {
		t.Fatalf("GetManager(default) failed: %v", err)
	}
	orgAManager, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}

	defaultOnlyRoleID := "default-only-role"
	if err := defaultManager.SaveRole(auth.Role{
		ID:          defaultOnlyRoleID,
		Name:        "Default Only",
		Description: "Default org role",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "settings"},
		},
	}); err != nil {
		t.Fatalf("SaveRole(%s) in default failed: %v", defaultOnlyRoleID, err)
	}
	if _, ok := orgAManager.GetRole(defaultOnlyRoleID); ok {
		t.Fatalf("default role %q leaked into %s", defaultOnlyRoleID, orgA)
	}

	orgAOnlyRoleID := "org-a-only-role"
	if err := orgAManager.SaveRole(auth.Role{
		ID:          orgAOnlyRoleID,
		Name:        "Org A Only",
		Description: "Named org role",
		Permissions: []auth.Permission{
			{Action: "write", Resource: "alerts"},
		},
	}); err != nil {
		t.Fatalf("SaveRole(%s) in %s failed: %v", orgAOnlyRoleID, orgA, err)
	}
	if _, ok := defaultManager.GetRole(orgAOnlyRoleID); ok {
		t.Fatalf("named org role %q leaked into default", orgAOnlyRoleID)
	}
}

func TestRBACIsolation_OrgDeletionRemovesRBACData(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	mustCreateIsolationOrgDir(t, baseDir, orgA)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	managerA, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}

	roleID := "org-a-ephemeral-role"
	if err := managerA.SaveRole(auth.Role{
		ID:          roleID,
		Name:        "Ephemeral",
		Description: "Should be removed with org data",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
	}); err != nil {
		t.Fatalf("SaveRole(%s) failed: %v", roleID, err)
	}
	if _, ok := managerA.GetRole(roleID); !ok {
		t.Fatalf("expected role %q to exist before deletion", roleID)
	}

	if err := provider.RemoveTenant(orgA); err != nil {
		t.Fatalf("RemoveTenant(%s) failed: %v", orgA, err)
	}

	// Simulate org deletion/recreation lifecycle: org directory removed, then recreated empty.
	orgDir := filepath.Join(baseDir, "orgs", orgA)
	if err := os.RemoveAll(orgDir); err != nil {
		t.Fatalf("failed to remove org dir %s: %v", orgDir, err)
	}
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to recreate org dir %s: %v", orgDir, err)
	}

	managerARecreated, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) after delete/recreate failed: %v", orgA, err)
	}

	if _, ok := managerARecreated.GetRole(roleID); ok {
		t.Fatalf("expected role %q to be removed after org deletion", roleID)
	}
	assertBuiltInRolesPresent(t, managerARecreated)
}

func mustCreateIsolationOrgDir(t *testing.T, baseDir, orgID string) {
	t.Helper()
	orgDir := filepath.Join(baseDir, "orgs", orgID)
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to create org dir %s: %v", orgDir, err)
	}
}

func containsPermission(perms []auth.Permission, action, resource string) bool {
	for _, perm := range perms {
		if perm.Action == action && perm.Resource == resource {
			return true
		}
	}
	return false
}
