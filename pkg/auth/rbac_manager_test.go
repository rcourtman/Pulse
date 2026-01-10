package auth

import (
	"os"
	"testing"
)

func TestFileManager(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "rbac-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m, err := NewFileManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}

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

	t.Run("Create custom role", func(t *testing.T) {
		customRole := Role{
			ID:          "custom",
			Name:        "Custom Role",
			Description: "A custom test role",
			Permissions: []Permission{{Action: "read", Resource: "nodes"}},
		}
		if err := m.SaveRole(customRole); err != nil {
			t.Errorf("Failed to save custom role: %v", err)
		}

		retrieved, ok := m.GetRole("custom")
		if !ok {
			t.Error("Custom role not found after save")
		}
		if retrieved.Name != "Custom Role" {
			t.Errorf("Expected name 'Custom Role', got '%s'", retrieved.Name)
		}
	})

	t.Run("Delete custom role", func(t *testing.T) {
		if err := m.DeleteRole("custom"); err != nil {
			t.Errorf("Failed to delete custom role: %v", err)
		}

		_, ok := m.GetRole("custom")
		if ok {
			t.Error("Custom role should not exist after delete")
		}
	})

	t.Run("Assign role to user", func(t *testing.T) {
		if err := m.AssignRole("testuser", RoleViewer); err != nil {
			t.Errorf("Failed to assign role: %v", err)
		}

		assignment, ok := m.GetUserAssignment("testuser")
		if !ok {
			t.Error("User assignment not found")
		}
		if len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != RoleViewer {
			t.Errorf("Expected viewer role, got %v", assignment.RoleIDs)
		}
	})

	t.Run("Get user permissions", func(t *testing.T) {
		perms := m.GetUserPermissions("testuser")
		if len(perms) == 0 {
			t.Error("Expected permissions for user with viewer role")
		}

		hasRead := false
		for _, p := range perms {
			if p.Action == "read" {
				hasRead = true
				break
			}
		}
		if !hasRead {
			t.Error("Viewer role should have read permissions")
		}
	})

	t.Run("Update user roles", func(t *testing.T) {
		if err := m.UpdateUserRoles("testuser", []string{RoleAdmin, RoleOperator}); err != nil {
			t.Errorf("Failed to update user roles: %v", err)
		}

		assignment, _ := m.GetUserAssignment("testuser")
		if len(assignment.RoleIDs) != 2 {
			t.Errorf("Expected 2 roles, got %d", len(assignment.RoleIDs))
		}
	})

	t.Run("Remove role from user", func(t *testing.T) {
		if err := m.RemoveRole("testuser", RoleOperator); err != nil {
			t.Errorf("Failed to remove role: %v", err)
		}

		assignment, _ := m.GetUserAssignment("testuser")
		if len(assignment.RoleIDs) != 1 {
			t.Errorf("Expected 1 role after removal, got %d", len(assignment.RoleIDs))
		}
	})

	t.Run("Persistence across reload", func(t *testing.T) {
		// Save a custom role
		customRole := Role{
			ID:          "persistent",
			Name:        "Persistent Role",
			Description: "Should survive reload",
			Permissions: []Permission{{Action: "write", Resource: "alerts"}},
		}
		if err := m.SaveRole(customRole); err != nil {
			t.Fatalf("Failed to save role: %v", err)
		}

		// Create new manager with same data dir
		m2, err := NewFileManager(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create second FileManager: %v", err)
		}

		// Check custom role persisted
		retrieved, ok := m2.GetRole("persistent")
		if !ok {
			t.Error("Custom role should persist across reload")
		}
		if retrieved.Name != "Persistent Role" {
			t.Errorf("Expected 'Persistent Role', got '%s'", retrieved.Name)
		}

		// Check user assignment persisted
		assignment, ok := m2.GetUserAssignment("testuser")
		if !ok {
			t.Error("User assignment should persist across reload")
		}
		if len(assignment.RoleIDs) == 0 {
			t.Error("User should still have roles after reload")
		}
	})
}
