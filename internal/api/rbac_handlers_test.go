package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type mockRBACManager struct {
	roles       []auth.Role
	assignments []auth.UserRoleAssignment
}

func (m *mockRBACManager) GetRoles() []auth.Role { return m.roles }
func (m *mockRBACManager) GetRole(id string) (auth.Role, bool) {
	for _, r := range m.roles {
		if r.ID == id {
			return r, true
		}
	}
	return auth.Role{}, false
}
func (m *mockRBACManager) SaveRole(role auth.Role) error {
	m.roles = append(m.roles, role)
	return nil
}
func (m *mockRBACManager) DeleteRole(id string) error {
	for i, r := range m.roles {
		if r.ID == id {
			m.roles = append(m.roles[:i], m.roles[i+1:]...)
			return nil
		}
	}
	return nil
}
func (m *mockRBACManager) GetUserAssignments() []auth.UserRoleAssignment { return m.assignments }
func (m *mockRBACManager) GetUserAssignment(username string) (auth.UserRoleAssignment, bool) {
	for _, a := range m.assignments {
		if a.Username == username {
			return a, true
		}
	}
	return auth.UserRoleAssignment{}, false
}
func (m *mockRBACManager) AssignRole(username string, roleID string) error { return nil }
func (m *mockRBACManager) UpdateUserRoles(username string, roleIDs []string) error {
	m.assignments = append(m.assignments, auth.UserRoleAssignment{Username: username, RoleIDs: roleIDs})
	return nil
}
func (m *mockRBACManager) RemoveRole(username string, roleID string) error { return nil }
func (m *mockRBACManager) GetUserPermissions(username string) []auth.Permission {
	return []auth.Permission{{Action: "read", Resource: "nodes"}}
}

func TestHandleRoles(t *testing.T) {
	cfg := &config.Config{}
	h := NewRBACHandlers(cfg)

	mock := &mockRBACManager{
		roles: []auth.Role{{ID: "admin", Name: "Admin"}},
	}
	auth.SetManager(mock)

	t.Run("List roles", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/roles", nil)
		rr := httptest.NewRecorder()
		h.HandleRoles(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", rr.Code)
		}
		var roles []auth.Role
		json.Unmarshal(rr.Body.Bytes(), &roles)
		if len(roles) != 1 || roles[0].ID != "admin" {
			t.Errorf("Unexpected roles: %+v", roles)
		}
	})

	t.Run("Create role", func(t *testing.T) {
		role := auth.Role{ID: "custom", Name: "Custom"}
		body, _ := json.Marshal(role)
		req := httptest.NewRequest("POST", "/api/admin/roles", bytes.NewReader(body))
		rr := httptest.NewRecorder()

		h.HandleRoles(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", rr.Code)
		}
		if len(mock.roles) != 2 || mock.roles[1].ID != "custom" {
			t.Errorf("Role not saved correctly")
		}
	})

	t.Run("POST with path ID (rejected)", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/roles/new-role", nil)
		rr := httptest.NewRecorder()
		h.HandleRoles(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %d", rr.Code)
		}
	})

	t.Run("Get role", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/roles/admin", nil)
		rr := httptest.NewRecorder()
		h.HandleRoles(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", rr.Code)
		}
		var role auth.Role
		json.Unmarshal(rr.Body.Bytes(), &role)
		if role.ID != "admin" {
			t.Errorf("Expected admin role, got %s", role.ID)
		}
	})

	t.Run("Delete role", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/admin/roles/custom", nil)
		rr := httptest.NewRecorder()

		h.HandleRoles(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status No Content, got %d", rr.Code)
		}
		if len(mock.roles) != 1 {
			t.Errorf("Role not deleted")
		}
	})
}

func TestHandleUserRoleActions(t *testing.T) {
	cfg := &config.Config{}
	h := NewRBACHandlers(cfg)
	mock := &mockRBACManager{}
	auth.SetManager(mock)

	t.Run("Update user roles", func(t *testing.T) {
		reqData := struct {
			RoleIDs []string `json:"roleIds"`
		}{RoleIDs: []string{"admin", "viewer"}}
		body, _ := json.Marshal(reqData)
		req := httptest.NewRequest("PUT", "/api/admin/users/testuser/roles", bytes.NewReader(body))
		rr := httptest.NewRecorder()

		h.HandleUserRoleActions(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status No Content, got %d", rr.Code)
		}
		if len(mock.assignments) != 1 || mock.assignments[0].Username != "testuser" {
			t.Errorf("Assignment not saved correctly")
		}
	})

	t.Run("Get effective permissions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/users/testuser/permissions", nil)
		rr := httptest.NewRecorder()

		h.HandleUserRoleActions(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", rr.Code)
		}
		var perms []auth.Permission
		json.Unmarshal(rr.Body.Bytes(), &perms)
		if len(perms) != 1 || perms[0].Action != "read" {
			t.Errorf("Unexpected permissions: %+v", perms)
		}
	})
}
