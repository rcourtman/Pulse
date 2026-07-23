package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteManager(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "rbac-sqlite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	t.Run("Built-in roles exist", func(t *testing.T) {
		roles := m.GetRoles()
		if len(roles) < 4 {
			t.Errorf("Expected at least 4 built-in roles, got %d", len(roles))
		}

		// Check admin role exists
		admin, ok := m.GetRole(RoleAdmin)
		if !ok {
			t.Error("Admin role not found")
		}
		if !admin.IsBuiltIn {
			t.Error("Admin role should be built-in")
		}
	})

	t.Run("Cannot delete built-in role", func(t *testing.T) {
		err := m.DeleteRole(RoleAdmin)
		if err == nil {
			t.Error("Expected error when deleting built-in role")
		}
	})

	t.Run("Cannot modify built-in role", func(t *testing.T) {
		admin, _ := m.GetRole(RoleAdmin)
		admin.Name = "Modified Admin"
		err := m.SaveRole(admin)
		if err == nil {
			t.Error("Expected error when modifying built-in role")
		}
	})

	t.Run("Create custom role with conditions", func(t *testing.T) {
		customRole := Role{
			ID:          "custom-abac",
			Name:        "Custom ABAC Role",
			Description: "A custom role with ABAC conditions",
			Permissions: []Permission{
				{Action: "read", Resource: "nodes", Effect: EffectAllow},
				{Action: "write", Resource: "nodes:production", Effect: EffectDeny},
				{
					Action:     "read",
					Resource:   "nodes:*",
					Effect:     EffectAllow,
					Conditions: map[string]string{"tag": "test", "owner": "${user}"},
				},
			},
		}
		if err := m.SaveRole(customRole); err != nil {
			t.Errorf("Failed to save custom role: %v", err)
		}

		retrieved, ok := m.GetRole("custom-abac")
		if !ok {
			t.Error("Custom role not found after save")
		}
		if retrieved.Name != "Custom ABAC Role" {
			t.Errorf("Expected name 'Custom ABAC Role', got '%s'", retrieved.Name)
		}
		if len(retrieved.Permissions) != 3 {
			t.Errorf("Expected 3 permissions, got %d", len(retrieved.Permissions))
		}

		// Check permission with conditions
		var foundCondPerm bool
		for _, p := range retrieved.Permissions {
			if p.Conditions != nil && p.Conditions["tag"] == "test" {
				foundCondPerm = true
				if p.Conditions["owner"] != "${user}" {
					t.Errorf("Expected owner condition '${user}', got '%s'", p.Conditions["owner"])
				}
			}
		}
		if !foundCondPerm {
			t.Error("Permission with conditions not found")
		}
	})

	t.Run("Create role with inheritance", func(t *testing.T) {
		// First create parent role
		parentRole := Role{
			ID:          "parent-role",
			Name:        "Parent Role",
			Description: "A parent role",
			Permissions: []Permission{
				{Action: "read", Resource: "settings"},
			},
		}
		if err := m.SaveRole(parentRole); err != nil {
			t.Errorf("Failed to save parent role: %v", err)
		}

		// Create child role
		childRole := Role{
			ID:          "child-role",
			Name:        "Child Role",
			Description: "A child role inheriting from parent",
			ParentID:    "parent-role",
			Permissions: []Permission{
				{Action: "write", Resource: "settings"},
			},
		}
		if err := m.SaveRole(childRole); err != nil {
			t.Errorf("Failed to save child role: %v", err)
		}

		// Get child with inheritance
		role, effectivePerms, ok := m.GetRoleWithInheritance("child-role")
		if !ok {
			t.Error("Child role not found")
		}
		if role.ParentID != "parent-role" {
			t.Errorf("Expected parent ID 'parent-role', got '%s'", role.ParentID)
		}

		// Should have both own permissions and inherited permissions
		if len(effectivePerms) < 2 {
			t.Errorf("Expected at least 2 effective permissions, got %d", len(effectivePerms))
		}

		var hasRead, hasWrite bool
		for _, p := range effectivePerms {
			if p.Action == "read" && p.Resource == "settings" {
				hasRead = true
			}
			if p.Action == "write" && p.Resource == "settings" {
				hasWrite = true
			}
		}
		if !hasRead {
			t.Error("Missing inherited read permission")
		}
		if !hasWrite {
			t.Error("Missing own write permission")
		}
	})

	t.Run("Assign roles to user", func(t *testing.T) {
		if err := m.AssignRole("testuser", RoleViewer); err != nil {
			t.Errorf("Failed to assign role: %v", err)
		}
		if err := m.AssignRole("testuser", "child-role"); err != nil {
			t.Errorf("Failed to assign child role: %v", err)
		}

		assignment, ok := m.GetUserAssignment("testuser")
		if !ok {
			t.Error("User assignment not found")
		}
		if len(assignment.RoleIDs) != 2 {
			t.Errorf("Expected 2 roles, got %d", len(assignment.RoleIDs))
		}
	})

	t.Run("Get roles with inheritance for user", func(t *testing.T) {
		roles := m.GetRolesWithInheritance("testuser")
		// Should include viewer, child-role, and parent-role (inherited)
		if len(roles) < 3 {
			t.Errorf("Expected at least 3 roles with inheritance, got %d", len(roles))
		}

		var hasParent bool
		for _, r := range roles {
			if r.ID == "parent-role" {
				hasParent = true
			}
		}
		if !hasParent {
			t.Error("Parent role should be included via inheritance")
		}
	})

	t.Run("Get user permissions", func(t *testing.T) {
		perms := m.GetUserPermissions("testuser")
		if len(perms) == 0 {
			t.Error("Expected permissions for user")
		}
	})

	t.Run("Delete custom role", func(t *testing.T) {
		if err := m.DeleteRole("custom-abac"); err != nil {
			t.Errorf("Failed to delete custom role: %v", err)
		}

		_, ok := m.GetRole("custom-abac")
		if ok {
			t.Error("Custom role should not exist after delete")
		}
	})

	t.Run("Changelog is recorded", func(t *testing.T) {
		logs := m.GetChangeLogs(100, 0)
		if len(logs) == 0 {
			t.Error("Expected changelog entries")
		}

		// Should have entries for role creation
		var hasRoleCreated bool
		for _, l := range logs {
			if l.Action == ActionRoleCreated {
				hasRoleCreated = true
			}
		}
		if !hasRoleCreated {
			t.Error("Missing role_created changelog entry")
		}
	})

	t.Run("Get changelog for entity", func(t *testing.T) {
		logs := m.GetChangeLogsForEntity("role", "parent-role")
		if len(logs) == 0 {
			t.Error("Expected changelog entries for parent-role")
		}
	})
}

func TestSQLiteManagerMigration(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "rbac-migration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// First create file-based manager with some data
	fileManager, err := NewFileManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}

	// Add custom role and assignment
	customRole := Role{
		ID:          "migrate-test",
		Name:        "Migration Test Role",
		Description: "Should be migrated",
		Permissions: []Permission{{Action: "read", Resource: "alerts"}},
	}
	if err := fileManager.SaveRole(customRole); err != nil {
		t.Fatalf("Failed to save role in FileManager: %v", err)
	}
	if err := fileManager.AssignRole("migrateuser", "migrate-test"); err != nil {
		t.Fatalf("Failed to assign role in FileManager: %v", err)
	}

	// Now create SQLite manager with migration enabled
	sqliteManager, err := NewSQLiteManager(SQLiteManagerConfig{
		DataDir:          tmpDir,
		MigrateFromFiles: true,
	})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer sqliteManager.Close()

	// Check custom role was migrated
	t.Run("Custom role migrated", func(t *testing.T) {
		role, ok := sqliteManager.GetRole("migrate-test")
		if !ok {
			t.Error("Custom role should be migrated to SQLite")
		}
		if role.Name != "Migration Test Role" {
			t.Errorf("Expected 'Migration Test Role', got '%s'", role.Name)
		}
	})

	// Check assignment was migrated
	t.Run("User assignment migrated", func(t *testing.T) {
		assignment, ok := sqliteManager.GetUserAssignment("migrateuser")
		if !ok {
			t.Error("User assignment should be migrated to SQLite")
		}
		hasRole := false
		for _, rid := range assignment.RoleIDs {
			if rid == "migrate-test" {
				hasRole = true
			}
		}
		if !hasRole {
			t.Error("User should have migrated role")
		}
	})

	// Check backup files created
	t.Run("Backup files created", func(t *testing.T) {
		rolesBackup := filepath.Join(tmpDir, "rbac_roles.json.bak")
		if _, err := os.Stat(rolesBackup); os.IsNotExist(err) {
			t.Error("Roles backup file should be created")
		}
		assignmentsBackup := filepath.Join(tmpDir, "rbac_assignments.json.bak")
		if _, err := os.Stat(assignmentsBackup); os.IsNotExist(err) {
			t.Error("Assignments backup file should be created")
		}
	})
}

func TestSQLiteManagerMigrationPreservesZeroRoleIdentities(t *testing.T) {
	tmpDir := t.TempDir()
	fileManager, err := NewFileManager(tmpDir)
	if err != nil {
		t.Fatalf("NewFileManager: %v", err)
	}
	if err := fileManager.UpdateUserRoles("local-user", nil); err != nil {
		t.Fatalf("create empty local assignment: %v", err)
	}
	if err := fileManager.UpdateUserRoles("sso:oidc:okta:stable", []string{RoleViewer}); err != nil {
		t.Fatalf("create SSO assignment: %v", err)
	}

	manager, err := NewSQLiteManager(SQLiteManagerConfig{
		DataDir:          tmpDir,
		MigrateFromFiles: true,
	})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()

	empty, ok := manager.GetUserAssignment("local-user")
	if !ok {
		t.Fatal("zero-role local identity was lost during migration")
	}
	if len(empty.RoleIDs) != 0 {
		t.Fatalf("zero-role identity has roles: %v", empty.RoleIDs)
	}
	sso, ok := manager.GetUserAssignment("sso:oidc:okta:stable")
	if !ok || len(sso.RoleIDs) != 1 || sso.RoleIDs[0] != RoleViewer {
		t.Fatalf("SSO assignment not migrated: %#v, exists=%v", sso, ok)
	}
}

func TestSQLiteManagerMigrationPreservesExistingBackups(t *testing.T) {
	tmpDir := t.TempDir()
	fileManager, err := NewFileManager(tmpDir)
	if err != nil {
		t.Fatalf("NewFileManager: %v", err)
	}
	if err := fileManager.SaveRole(Role{
		ID:          "legacy-role",
		Name:        "Legacy role",
		Permissions: []Permission{{Action: "read", Resource: "*"}},
	}); err != nil {
		t.Fatalf("create legacy role: %v", err)
	}
	if err := fileManager.UpdateUserRoles("legacy-user", []string{RoleViewer}); err != nil {
		t.Fatalf("create legacy assignment: %v", err)
	}
	for _, name := range []string{"rbac_roles.json.bak", "rbac_assignments.json.bak"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("existing backup"), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	manager, err := NewSQLiteManager(SQLiteManagerConfig{
		DataDir:          tmpDir,
		MigrateFromFiles: true,
	})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()

	for _, name := range []string{"rbac_roles.json.bak", "rbac_assignments.json.bak"} {
		original, err := os.ReadFile(filepath.Join(tmpDir, name))
		if err != nil {
			t.Fatalf("read original %s: %v", name, err)
		}
		if string(original) != "existing backup" {
			t.Fatalf("existing backup %s was replaced", name)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, name+".1")); err != nil {
			t.Fatalf("new migration backup %s.1 missing: %v", name, err)
		}
	}
}

func TestSQLiteManagerMigrationRejectsCorruptAndStaleData(t *testing.T) {
	tests := []struct {
		name        string
		roles       string
		assignments string
		wantError   string
	}{
		{
			name:        "corrupt roles",
			roles:       `[{"id":`,
			assignments: `[]`,
			wantError:   "decode legacy RBAC roles",
		},
		{
			name: "corrupt assignments rolls back roles",
			roles: mustJSON(t, []Role{{
				ID:          "legacy-viewer",
				Name:        "Legacy Viewer",
				Permissions: []Permission{{Action: "read", Resource: "*"}},
			}}),
			assignments: `[{"username":`,
			wantError:   "decode legacy RBAC assignments",
		},
		{
			name: "stale assignment",
			roles: mustJSON(t, []Role{{
				ID:          "legacy-viewer",
				Name:        "Legacy Viewer",
				Permissions: []Permission{{Action: "read", Resource: "*"}},
			}}),
			assignments: mustJSON(t, []UserRoleAssignment{{
				Username: "stale-user",
				RoleIDs:  []string{"deleted-role"},
			}}),
			wantError: "references missing role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			rolesPath := filepath.Join(tmpDir, "rbac_roles.json")
			assignmentsPath := filepath.Join(tmpDir, "rbac_assignments.json")
			if err := os.WriteFile(rolesPath, []byte(tt.roles), 0600); err != nil {
				t.Fatalf("write roles: %v", err)
			}
			if err := os.WriteFile(assignmentsPath, []byte(tt.assignments), 0600); err != nil {
				t.Fatalf("write assignments: %v", err)
			}

			manager, err := NewSQLiteManager(SQLiteManagerConfig{
				DataDir:          tmpDir,
				MigrateFromFiles: true,
			})
			if manager != nil {
				_ = manager.Close()
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantError)
			}
			if _, err := os.Stat(rolesPath); err != nil {
				t.Fatalf("legacy roles source was not preserved: %v", err)
			}
			if _, err := os.Stat(assignmentsPath); err != nil {
				t.Fatalf("legacy assignments source was not preserved: %v", err)
			}
			if _, err := os.Stat(rolesPath + ".bak"); !os.IsNotExist(err) {
				t.Fatalf("roles backup must not be created on failed migration: %v", err)
			}

			reopened, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
			if err != nil {
				t.Fatalf("reopen after failed migration: %v", err)
			}
			defer reopened.Close()
			if _, ok := reopened.GetRole("legacy-viewer"); ok {
				t.Fatal("failed migration committed a partial role")
			}
		})
	}
}

func TestSQLiteManagerMigrationRejectsCurrentStateConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	current, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("create current manager: %v", err)
	}
	if err := current.SaveRole(Role{
		ID:          "existing",
		Name:        "Current v6 role",
		Permissions: []Permission{{Action: "read", Resource: "nodes"}},
	}); err != nil {
		t.Fatalf("save current role: %v", err)
	}
	if err := current.Close(); err != nil {
		t.Fatalf("close current manager: %v", err)
	}

	legacyRoles := []Role{{
		ID:          "existing",
		Name:        "Legacy conflicting role",
		Permissions: []Permission{{Action: "admin", Resource: "*"}},
	}}
	if err := os.WriteFile(filepath.Join(tmpDir, "rbac_roles.json"), []byte(mustJSON(t, legacyRoles)), 0600); err != nil {
		t.Fatalf("write legacy roles: %v", err)
	}

	manager, err := NewSQLiteManager(SQLiteManagerConfig{
		DataDir:          tmpDir,
		MigrateFromFiles: true,
	})
	if manager != nil {
		_ = manager.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "conflicts with current v6 data") {
		t.Fatalf("error = %v, want current-state conflict", err)
	}

	reopened, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("reopen current manager: %v", err)
	}
	defer reopened.Close()
	role, ok := reopened.GetRole("existing")
	if !ok || role.Name != "Current v6 role" {
		t.Fatalf("current v6 role was changed: %#v, exists=%v", role, ok)
	}
}

func TestSQLiteManagerMigrationMergesDistinctLegacyAndV6State(t *testing.T) {
	tmpDir := t.TempDir()
	current, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("create current manager: %v", err)
	}
	if err := current.SaveRole(Role{
		ID:          "v6-role",
		Name:        "V6 role",
		Permissions: []Permission{{Action: "write", Resource: "nodes"}},
	}); err != nil {
		t.Fatalf("save v6 role: %v", err)
	}
	if err := current.UpdateUserRoles("v6-user", []string{"v6-role"}); err != nil {
		t.Fatalf("save v6 assignment: %v", err)
	}
	if err := current.Close(); err != nil {
		t.Fatalf("close v6 manager: %v", err)
	}

	legacyRoles := []Role{{
		ID:          "legacy-role",
		Name:        "Legacy role",
		Permissions: []Permission{{Action: "read", Resource: "nodes"}},
	}}
	legacyAssignments := []UserRoleAssignment{{
		Username: "legacy-user",
		RoleIDs:  []string{"legacy-role"},
	}}
	if err := os.WriteFile(filepath.Join(tmpDir, "rbac_roles.json"), []byte(mustJSON(t, legacyRoles)), 0600); err != nil {
		t.Fatalf("write legacy roles: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "rbac_assignments.json"), []byte(mustJSON(t, legacyAssignments)), 0600); err != nil {
		t.Fatalf("write legacy assignments: %v", err)
	}

	manager, err := NewSQLiteManager(SQLiteManagerConfig{
		DataDir:          tmpDir,
		MigrateFromFiles: true,
	})
	if err != nil {
		t.Fatalf("migrate distinct states: %v", err)
	}
	defer manager.Close()
	for _, roleID := range []string{"v6-role", "legacy-role"} {
		if _, ok := manager.GetRole(roleID); !ok {
			t.Fatalf("role %q missing after merge", roleID)
		}
	}
	for username, roleID := range map[string]string{
		"v6-user":     "v6-role",
		"legacy-user": "legacy-role",
	} {
		assignment, ok := manager.GetUserAssignment(username)
		if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != roleID {
			t.Fatalf("assignment for %q = %#v, exists=%v", username, assignment, ok)
		}
	}
}

func TestSQLiteManagerKeepsIdentityWhenRolesAreEmptyOrDeleted(t *testing.T) {
	manager, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()

	if err := manager.UpdateUserRoles("empty-user", nil); err != nil {
		t.Fatalf("UpdateUserRoles empty: %v", err)
	}
	if assignment, ok := manager.GetUserAssignment("empty-user"); !ok || len(assignment.RoleIDs) != 0 {
		t.Fatalf("empty identity not retained: %#v, exists=%v", assignment, ok)
	}

	role := Role{
		ID:          "temporary-role",
		Name:        "Temporary",
		Permissions: []Permission{{Action: "read", Resource: "nodes"}},
	}
	if err := manager.SaveRole(role); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}
	if err := manager.AssignRole("renamed-user", role.ID); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	if err := manager.DeleteRole(role.ID); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	assignment, ok := manager.GetUserAssignment("renamed-user")
	if !ok || len(assignment.RoleIDs) != 0 {
		t.Fatalf("role deletion left a stale grant or removed identity: %#v, exists=%v", assignment, ok)
	}
	if permissions := manager.GetUserPermissions("renamed-user"); len(permissions) != 0 {
		t.Fatalf("deleted role still grants permissions: %#v", permissions)
	}

	assignments, err := manager.GetUserAssignmentsWithError()
	if err != nil {
		t.Fatalf("GetUserAssignmentsWithError: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("known identities = %d, want 2: %#v", len(assignments), assignments)
	}

	if _, err := manager.db.Exec(`
		INSERT INTO rbac_user_assignments (username, role_id, updated_at)
		VALUES ('orphaned-user', ?, ?)
	`, RoleViewer, time.Now().Unix()); err == nil {
		t.Fatal("assignment schema accepted an identity missing from rbac_users")
	}
}

func TestSQLiteManagerIdentityMigrationRejectsConflictingGrant(t *testing.T) {
	manager, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewSQLiteManager: %v", err)
	}
	defer manager.Close()
	if err := manager.UpdateUserRoles("legacy@example.com", []string{RoleAdmin}); err != nil {
		t.Fatalf("seed legacy assignment: %v", err)
	}
	if err := manager.UpdateUserRoles("sso:oidc:okta:stable", []string{RoleViewer}); err != nil {
		t.Fatalf("seed canonical assignment: %v", err)
	}

	err = manager.MigrateUserAssignment("legacy@example.com", "sso:oidc:okta:stable")
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error = %v, want conflict", err)
	}
	legacy, legacyExists := manager.GetUserAssignment("legacy@example.com")
	canonical, canonicalExists := manager.GetUserAssignment("sso:oidc:okta:stable")
	if !legacyExists || len(legacy.RoleIDs) != 1 || legacy.RoleIDs[0] != RoleAdmin {
		t.Fatalf("legacy assignment changed after conflict: %#v, exists=%v", legacy, legacyExists)
	}
	if !canonicalExists || len(canonical.RoleIDs) != 1 || canonical.RoleIDs[0] != RoleViewer {
		t.Fatalf("canonical assignment changed after conflict: %#v, exists=%v", canonical, canonicalExists)
	}
}

func mustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test data: %v", err)
	}
	return string(data)
}

func TestSQLiteManagerCircularInheritance(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbac-circular-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	// Create role A
	roleA := Role{ID: "role-a", Name: "Role A", Permissions: []Permission{{Action: "read", Resource: "a"}}}
	if err := m.SaveRole(roleA); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}

	// Create role B with parent A
	roleB := Role{ID: "role-b", Name: "Role B", ParentID: "role-a", Permissions: []Permission{{Action: "read", Resource: "b"}}}
	if err := m.SaveRole(roleB); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}

	// Create role C with parent B
	roleC := Role{ID: "role-c", Name: "Role C", ParentID: "role-b", Permissions: []Permission{{Action: "read", Resource: "c"}}}
	if err := m.SaveRole(roleC); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}

	// Try to make A inherit from C (creating cycle)
	roleA.ParentID = "role-c"
	err = m.SaveRole(roleA)
	if err == nil {
		t.Error("Expected error when creating circular inheritance")
	}
}

func TestSQLiteManagerContextOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbac-context-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	t.Run("SaveRoleWithContext records user", func(t *testing.T) {
		role := Role{
			ID:          "context-role",
			Name:        "Context Test Role",
			Permissions: []Permission{{Action: "read", Resource: "test"}},
		}
		if err := m.SaveRoleWithContext(role, "admin-user"); err != nil {
			t.Errorf("Failed to save role with context: %v", err)
		}

		// Check changelog has user
		logs := m.GetChangeLogsForEntity("role", "context-role")
		if len(logs) == 0 {
			t.Fatal("Expected changelog entry")
		}
		if logs[0].User != "admin-user" {
			t.Errorf("Expected user 'admin-user', got '%s'", logs[0].User)
		}
	})

	t.Run("DeleteRoleWithContext records user", func(t *testing.T) {
		if err := m.DeleteRoleWithContext("context-role", "admin-user"); err != nil {
			t.Errorf("Failed to delete role with context: %v", err)
		}

		logs := m.GetChangeLogsForEntity("role", "context-role")
		var hasDelete bool
		for _, l := range logs {
			if l.Action == ActionRoleDeleted && l.User == "admin-user" {
				hasDelete = true
			}
		}
		if !hasDelete {
			t.Error("Missing delete changelog with user")
		}
	})

	t.Run("UpdateUserRolesWithContext records user", func(t *testing.T) {
		if err := m.UpdateUserRolesWithContext("context-user", []string{RoleViewer}, "admin-user"); err != nil {
			t.Errorf("Failed to update user roles with context: %v", err)
		}

		logs := m.GetChangeLogsForEntity("assignment", "context-user")
		if len(logs) == 0 {
			t.Fatal("Expected changelog entry")
		}
		if logs[0].User != "admin-user" {
			t.Errorf("Expected user 'admin-user', got '%s'", logs[0].User)
		}
	})
}

func TestSQLiteManagerPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbac-persist-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create manager and add data
	m1, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}

	role := Role{
		ID:          "persist-role",
		Name:        "Persist Test",
		ParentID:    RoleViewer,
		Permissions: []Permission{{Action: "write", Resource: "persist", Effect: EffectAllow}},
	}
	if err := m1.SaveRole(role); err != nil {
		t.Fatalf("SaveRole: %v", err)
	}
	if err := m1.AssignRole("persist-user", "persist-role"); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	m1.Close()

	// Reopen and verify
	m2, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to reopen SQLiteManager: %v", err)
	}
	defer m2.Close()

	t.Run("Role persisted", func(t *testing.T) {
		r, ok := m2.GetRole("persist-role")
		if !ok {
			t.Error("Role should persist")
		}
		if r.ParentID != RoleViewer {
			t.Errorf("Expected parent %s, got %s", RoleViewer, r.ParentID)
		}
	})

	t.Run("Assignment persisted", func(t *testing.T) {
		a, ok := m2.GetUserAssignment("persist-user")
		if !ok {
			t.Error("Assignment should persist")
		}
		if len(a.RoleIDs) != 1 {
			t.Errorf("Expected 1 role, got %d", len(a.RoleIDs))
		}
	})

	t.Run("Changelog persisted", func(t *testing.T) {
		logs := m2.GetChangeLogs(100, 0)
		if len(logs) == 0 {
			t.Error("Changelog should persist")
		}
	})
}

func TestSQLiteManagerDenyPermission(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbac-deny-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	// Create role with both allow and deny permissions
	role := Role{
		ID:   "deny-test",
		Name: "Deny Test Role",
		Permissions: []Permission{
			{Action: "read", Resource: "*", Effect: EffectAllow},              // Allow all reads
			{Action: "read", Resource: "secrets", Effect: EffectDeny},         // But deny reading secrets
			{Action: "write", Resource: "settings", Effect: EffectAllow},      // Allow writing settings
			{Action: "write", Resource: "settings:admin", Effect: EffectDeny}, // But deny admin settings
		},
	}
	if err := m.SaveRole(role); err != nil {
		t.Fatalf("Failed to save role: %v", err)
	}

	retrieved, ok := m.GetRole("deny-test")
	if !ok {
		t.Fatal("Role not found")
	}

	// Verify deny effects are preserved
	var denyCount, allowCount int
	for _, p := range retrieved.Permissions {
		if p.GetEffect() == EffectDeny {
			denyCount++
		} else {
			allowCount++
		}
	}
	if denyCount != 2 {
		t.Errorf("Expected 2 deny permissions, got %d", denyCount)
	}
	if allowCount != 2 {
		t.Errorf("Expected 2 allow permissions, got %d", allowCount)
	}
}

func TestSQLiteManagerChangeLogRetention(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbac-retention-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewSQLiteManager(SQLiteManagerConfig{DataDir: tmpDir})
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	defer m.Close()

	// Create several roles to generate changelog entries
	for i := 0; i < 5; i++ {
		role := Role{
			ID:          "retention-role-" + string(rune('a'+i)),
			Name:        "Retention Test " + string(rune('a'+i)),
			Permissions: []Permission{{Action: "read", Resource: "test"}},
		}
		if err := m.SaveRole(role); err != nil {
			t.Fatalf("SaveRole: %v", err)
		}
		// Add slight delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	t.Run("Pagination works", func(t *testing.T) {
		// Get first 2
		logs1 := m.GetChangeLogs(2, 0)
		if len(logs1) != 2 {
			t.Errorf("Expected 2 logs, got %d", len(logs1))
		}

		// Get next 2
		logs2 := m.GetChangeLogs(2, 2)
		if len(logs2) != 2 {
			t.Errorf("Expected 2 logs, got %d", len(logs2))
		}

		// Ensure they're different
		if len(logs1) > 0 && len(logs2) > 0 && logs1[0].ID == logs2[0].ID {
			t.Error("Pagination returned same entries")
		}
	})

	t.Run("Logs ordered by timestamp desc", func(t *testing.T) {
		logs := m.GetChangeLogs(100, 0)
		for i := 1; i < len(logs); i++ {
			if logs[i].Timestamp.After(logs[i-1].Timestamp) {
				t.Error("Logs should be ordered by timestamp descending")
			}
		}
	})
}
