package api

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	_ "modernc.org/sqlite"
)

func TestRBACIntegrity_HealthyOrg(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "acme"
	mkdirAllOrFatal(t, filepath.Join(baseDir, "orgs", orgID), "create org dir")

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	if _, err := provider.GetManager(orgID); err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}

	result := VerifyRBACIntegrity(provider, orgID)

	if !result.Healthy {
		t.Fatalf("expected healthy org, got unhealthy result: %+v", result)
	}
	if !result.DBAccessible {
		t.Fatalf("expected DBAccessible=true, got false")
	}
	if !result.TablesPresent {
		t.Fatalf("expected TablesPresent=true, got false")
	}
	if result.BuiltInRoleCount < 4 {
		t.Fatalf("expected >=4 built-in roles, got %d", result.BuiltInRoleCount)
	}
	if result.TotalRoles < 4 {
		t.Fatalf("expected >=4 total roles, got %d", result.TotalRoles)
	}
	if result.Error != "" {
		t.Fatalf("expected empty error, got %q", result.Error)
	}
}

func TestRBACIntegrity_NonExistentOrg(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	result := VerifyRBACIntegrity(provider, "missing-org")

	if result.Healthy {
		t.Fatalf("expected unhealthy result for non-existent org, got healthy")
	}
	if result.DBAccessible {
		t.Fatalf("expected DBAccessible=false for non-existent org")
	}
	if result.TablesPresent {
		t.Fatalf("expected TablesPresent=false for non-existent org")
	}
	if result.Error == "" {
		t.Fatalf("expected error for non-existent org")
	}
	if !strings.Contains(result.Error, "failed to get manager") {
		t.Fatalf("expected manager error, got %q", result.Error)
	}
}

func TestRBACIntegrity_DefaultOrg(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	result := VerifyRBACIntegrity(provider, "default")

	if !result.Healthy {
		t.Fatalf("expected default org to be healthy, got %+v", result)
	}
	if !result.DBAccessible {
		t.Fatalf("expected DBAccessible=true, got false")
	}
	if !result.TablesPresent {
		t.Fatalf("expected TablesPresent=true, got false")
	}
	if result.BuiltInRoleCount < 4 {
		t.Fatalf("expected >=4 built-in roles, got %d", result.BuiltInRoleCount)
	}
}

func TestResetAdminRole_RestoresAccess(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "ops"
	username := "breakglass-user"
	mkdirAllOrFatal(t, filepath.Join(baseDir, "orgs", orgID), "create org dir")

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

	if _, ok := manager.GetRole(auth.RoleAdmin); !ok {
		t.Fatalf("expected admin role to exist before reset")
	}

	if err := ResetAdminRole(provider, orgID, username); err != nil {
		t.Fatalf("ResetAdminRole failed: %v", err)
	}

	assignment, ok := manager.GetUserAssignment(username)
	if !ok {
		t.Fatalf("expected user assignment for %q", username)
	}
	if !containsRoleIDRecovery(assignment.RoleIDs, auth.RoleAdmin) {
		t.Fatalf("expected %q to have %q role, got %v", username, auth.RoleAdmin, assignment.RoleIDs)
	}
}

func TestResetAdminRole_RecreatesDeletedAdminRole(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "recoverable"
	username := "recovery-user"
	mkdirAllOrFatal(t, filepath.Join(baseDir, "orgs", orgID), "create org dir")

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	if _, err := provider.GetManager(orgID); err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}

	dbPath := filepath.Join(baseDir, "orgs", orgID, "rbac", "rbac.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed opening sqlite db at %s: %v", dbPath, err)
	}
	if _, err := db.Exec("DELETE FROM rbac_roles WHERE id = ?", auth.RoleAdmin); err != nil {
		db.Close()
		t.Fatalf("failed deleting built-in admin role directly from db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed closing sqlite db: %v", err)
	}

	if err := ResetAdminRole(provider, orgID, username); err != nil {
		t.Fatalf("ResetAdminRole after admin deletion failed: %v", err)
	}

	managerAfter, err := provider.GetManager(orgID)
	if err != nil {
		t.Fatalf("GetManager(%s) after recovery failed: %v", orgID, err)
	}

	adminRole, ok := managerAfter.GetRole(auth.RoleAdmin)
	if !ok {
		t.Fatalf("expected admin role to be recreated")
	}
	if !adminRole.IsBuiltIn {
		t.Fatalf("expected recreated admin role to be built-in")
	}

	assignment, ok := managerAfter.GetUserAssignment(username)
	if !ok {
		t.Fatalf("expected user assignment for %q", username)
	}
	if !containsRoleIDRecovery(assignment.RoleIDs, auth.RoleAdmin) {
		t.Fatalf("expected %q to have %q role, got %v", username, auth.RoleAdmin, assignment.RoleIDs)
	}
}

func TestBackupRBACData_CreatesBackupFile(t *testing.T) {
	baseDir := t.TempDir()
	orgID := "backup-org"
	mkdirAllOrFatal(t, filepath.Join(baseDir, "orgs", orgID), "create org dir")

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	if _, err := provider.GetManager(orgID); err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgID, err)
	}

	destDir := filepath.Join(baseDir, "backups")
	backupPath, err := BackupRBACData(provider, orgID, destDir)
	if err != nil {
		t.Fatalf("BackupRBACData failed: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("expected backup file at %s: %v", backupPath, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty backup file, got size %d", info.Size())
	}
}

func TestBackupRBACData_NonExistentOrg(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("cleanup close failed: %v", err)
		}
	})

	destDir := filepath.Join(baseDir, "backups")
	if _, err := BackupRBACData(provider, "missing-org", destDir); err == nil {
		t.Fatalf("expected error backing up non-existent org")
	}
}

func containsRoleIDRecovery(roleIDs []string, roleID string) bool {
	for _, id := range roleIDs {
		if id == roleID {
			return true
		}
	}
	return false
}

func mkdirAllOrFatal(t *testing.T, path string, action string) {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatalf("failed to %s at %s: %v", action, path, err)
	}
}
