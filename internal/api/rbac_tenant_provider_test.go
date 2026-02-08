package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestTenantRBACProvider_DefaultOrg(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	manager, err := provider.GetManager("default")
	if err != nil {
		t.Fatalf("GetManager(default) failed: %v", err)
	}

	assertBuiltInRolesPresent(t, manager)

	dbPath := filepath.Join(baseDir, "rbac", "rbac.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected default rbac db at %s: %v", dbPath, err)
	}
}

func TestTenantRBACProvider_NonDefaultOrg(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "acme"
	createOrgDir(t, baseDir, orgID)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	manager, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}

	assertBuiltInRolesPresent(t, manager)

	dbPath := filepath.Join(baseDir, "orgs", orgID, "rbac", "rbac.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected tenant rbac db at %s: %v", dbPath, err)
	}
}

func TestTenantRBACProvider_Isolation(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"
	createOrgDir(t, baseDir, orgA)
	createOrgDir(t, baseDir, orgB)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
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

	roleID := "custom-org-a"
	if err := managerA.SaveRole(auth.Role{
		ID:          roleID,
		Name:        "Custom Org A",
		Description: "Role for org A only",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
	}); err != nil {
		t.Fatalf("SaveRole in %s failed: %v", orgA, err)
	}

	if _, ok := managerA.GetRole(roleID); !ok {
		t.Fatalf("expected role %s in %s", roleID, orgA)
	}
	if _, ok := managerB.GetRole(roleID); ok {
		t.Fatalf("role %s leaked into %s", roleID, orgB)
	}
}

func TestTenantRBACProvider_CachesManager(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "cached-org"
	createOrgDir(t, baseDir, orgID)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	manager1, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("first GetManager(%s) failed: %v", orgID, err)
	}
	manager2, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("second GetManager(%s) failed: %v", orgID, err)
	}

	if sqliteManagerPointer(t, manager1) != sqliteManagerPointer(t, manager2) {
		t.Fatalf("expected cached manager instance for org %s", orgID)
	}
}

func TestTenantRBACProvider_RemoveTenant(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "remove-org"
	createOrgDir(t, baseDir, orgID)

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	manager1, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("initial GetManager(%s) failed: %v", orgID, err)
	}
	firstPtr := sqliteManagerPointer(t, manager1)

	if err := provider.RemoveTenant(orgID); err != nil {
		t.Fatalf("RemoveTenant(%s) failed: %v", orgID, err)
	}

	manager2, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("GetManager(%s) after remove failed: %v", orgID, err)
	}
	secondPtr := sqliteManagerPointer(t, manager2)

	if firstPtr == secondPtr {
		t.Fatalf("expected a new manager instance after RemoveTenant(%s)", orgID)
	}
}

func TestTenantRBACProvider_Close(t *testing.T) {
	baseDir := t.TempDir()
	createOrgDir(t, baseDir, "org-a")
	createOrgDir(t, baseDir, "org-b")

	provider := NewTenantRBACProvider(baseDir)

	if _, err := provider.GetManager("default"); err != nil {
		t.Fatalf("GetManager(default) failed: %v", err)
	}
	if _, err := provider.GetManager("org-a"); err != nil {
		t.Fatalf("GetManager(org-a) failed: %v", err)
	}
	if _, err := provider.GetManager("org-b"); err != nil {
		t.Fatalf("GetManager(org-b) failed: %v", err)
	}

	if err := provider.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("second Close() failed: %v", err)
	}
}

func assertBuiltInRolesPresent(t *testing.T, manager auth.ExtendedManager) {
	t.Helper()

	roles := manager.GetRoles()
	if len(roles) < 4 {
		t.Fatalf("expected at least 4 roles, got %d", len(roles))
	}

	roleSet := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		roleSet[role.ID] = struct{}{}
	}

	required := []string{
		auth.RoleAdmin,
		auth.RoleOperator,
		auth.RoleViewer,
		auth.RoleAuditor,
	}
	for _, roleID := range required {
		if _, ok := roleSet[roleID]; !ok {
			t.Fatalf("missing built-in role %q", roleID)
		}
	}
}

func createOrgDir(t *testing.T, baseDir, orgID string) {
	t.Helper()
	orgDir := filepath.Join(baseDir, "orgs", orgID)
	if err := os.MkdirAll(orgDir, 0700); err != nil {
		t.Fatalf("failed to create org dir %s: %v", orgDir, err)
	}
}

func sqliteManagerPointer(t *testing.T, manager auth.ExtendedManager) *auth.SQLiteManager {
	t.Helper()
	sqliteManager, ok := manager.(*auth.SQLiteManager)
	if !ok {
		t.Fatalf("expected *auth.SQLiteManager, got %T", manager)
	}
	return sqliteManager
}
