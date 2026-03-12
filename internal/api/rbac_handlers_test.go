package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		_ = json.Unmarshal(rr.Body.Bytes(), &roles)
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
		_ = json.Unmarshal(rr.Body.Bytes(), &role)
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
		_ = json.Unmarshal(rr.Body.Bytes(), &perms)
		if len(perms) != 1 || perms[0].Action != "read" {
			t.Errorf("Unexpected permissions: %+v", perms)
		}
	})

	t.Run("Block self role mutation", func(t *testing.T) {
		beforeAssignments := len(mock.assignments)
		reqData := struct {
			RoleIDs []string `json:"roleIds"`
		}{RoleIDs: []string{"viewer"}}
		body, _ := json.Marshal(reqData)
		req := httptest.NewRequest("PUT", "/api/admin/users/admin/roles", bytes.NewReader(body))
		req = req.WithContext(auth.WithUser(req.Context(), "AdMiN"))
		rr := httptest.NewRecorder()

		h.HandleUserRoleActions(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("Expected status Forbidden, got %d", rr.Code)
		}

		var payload map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode error response: %v", err)
		}
		if payload["code"] != "self_modification_denied" {
			t.Fatalf("expected self_modification_denied error, got %+v", payload)
		}
		if len(mock.assignments) != beforeAssignments {
			t.Fatalf("self mutation should not write assignments, got %+v", mock.assignments)
		}
	})
}

func TestRBACHandlers_TenantScoped(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"

	for _, orgID := range []string{orgA, orgB} {
		orgDir := filepath.Join(baseDir, "orgs", orgID)
		if err := os.MkdirAll(orgDir, 0700); err != nil {
			t.Fatalf("failed to create org dir %s: %v", orgDir, err)
		}
	}

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	handler := NewRBACHandlers(&config.Config{}, provider)
	roleID := "tenant-a-custom-role"

	role := auth.Role{
		ID:          roleID,
		Name:        "Tenant A Custom",
		Description: "Role scoped to org A",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
	}
	body, err := json.Marshal(role)
	if err != nil {
		t.Fatalf("failed to marshal role: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/roles", bytes.NewReader(body))
	createReq = createReq.WithContext(context.WithValue(createReq.Context(), OrgIDContextKey, orgA))
	createRR := httptest.NewRecorder()
	handler.HandleRoles(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusOK, createRR.Code, createRR.Body.String())
	}

	managerA, err := provider.GetManager(orgA)
	if err != nil {
		t.Fatalf("GetManager(%s) failed: %v", orgA, err)
	}
	if _, ok := managerA.GetRole(roleID); !ok {
		t.Fatalf("expected role %q to exist in %s", roleID, orgA)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/roles", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), OrgIDContextKey, orgB))
	listRR := httptest.NewRecorder()
	handler.HandleRoles(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, listRR.Code, listRR.Body.String())
	}

	var rolesInOrgB []auth.Role
	if err := json.Unmarshal(listRR.Body.Bytes(), &rolesInOrgB); err != nil {
		t.Fatalf("failed to decode org B roles: %v", err)
	}

	for _, r := range rolesInOrgB {
		if r.ID == roleID {
			t.Fatalf("role %q from %s leaked into %s", roleID, orgA, orgB)
		}
	}
}

func TestMultiTenantRBACRoleUpdateChangesPermissions(t *testing.T) {
	baseDir := t.TempDir()
	orgA := "org-a"
	orgB := "org-b"

	for _, orgID := range []string{orgA, orgB} {
		orgDir := filepath.Join(baseDir, "orgs", orgID)
		if err := os.MkdirAll(orgDir, 0o700); err != nil {
			t.Fatalf("failed to create org dir %s: %v", orgDir, err)
		}
	}

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("provider close failed: %v", err)
		}
	})

	handler := NewRBACHandlers(&config.Config{}, provider)

	createRole := func(orgID string, role auth.Role) {
		t.Helper()

		body, err := json.Marshal(role)
		if err != nil {
			t.Fatalf("marshal role %s: %v", role.ID, err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/admin/roles", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("create role %s in %s failed: %d %s", role.ID, orgID, rr.Code, rr.Body.String())
		}
	}

	readerRole := auth.Role{
		ID:          "tenant-reader",
		Name:        "Tenant Reader",
		Description: "Read-only access in org A",
		Permissions: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
	}
	writerRole := auth.Role{
		ID:          "tenant-writer",
		Name:        "Tenant Writer",
		Description: "Write access in org A",
		Permissions: []auth.Permission{
			{Action: "write", Resource: "nodes"},
		},
	}

	createRole(orgA, readerRole)
	createRole(orgA, writerRole)

	updateRoles := func(orgID string, username string, roleIDs []string) {
		t.Helper()

		payload, err := json.Marshal(struct {
			RoleIDs []string `json:"roleIds"`
		}{RoleIDs: roleIDs})
		if err != nil {
			t.Fatalf("marshal role update: %v", err)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+username+"/roles", bytes.NewReader(payload))
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
		rr := httptest.NewRecorder()
		handler.HandleUserRoleActions(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("update roles for %s in %s failed: %d %s", username, orgID, rr.Code, rr.Body.String())
		}
	}

	getPermissions := func(orgID string, username string) []auth.Permission {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, "/api/admin/users/"+username+"/permissions", nil)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
		rr := httptest.NewRecorder()
		handler.HandleUserRoleActions(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("get permissions for %s in %s failed: %d %s", username, orgID, rr.Code, rr.Body.String())
		}

		var perms []auth.Permission
		if err := json.Unmarshal(rr.Body.Bytes(), &perms); err != nil {
			t.Fatalf("decode permissions for %s in %s: %v", username, orgID, err)
		}
		return perms
	}

	updateRoles(orgA, "alice", []string{readerRole.ID})
	permsAfterReader := getPermissions(orgA, "alice")
	if len(permsAfterReader) != 1 || permsAfterReader[0].Action != "read" || permsAfterReader[0].Resource != "nodes" {
		t.Fatalf("unexpected reader permissions in %s: %+v", orgA, permsAfterReader)
	}

	updateRoles(orgA, "alice", []string{writerRole.ID})
	permsAfterWriter := getPermissions(orgA, "alice")
	if len(permsAfterWriter) != 1 || permsAfterWriter[0].Action != "write" || permsAfterWriter[0].Resource != "nodes" {
		t.Fatalf("unexpected writer permissions in %s: %+v", orgA, permsAfterWriter)
	}

	permsInOrgB := getPermissions(orgB, "alice")
	if len(permsInOrgB) != 0 {
		t.Fatalf("permissions leaked into %s: %+v", orgB, permsInOrgB)
	}
}
