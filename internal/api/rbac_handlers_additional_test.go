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

type mockExtendedRBACManager struct {
	mockRBACManager
	logs           []auth.RBACChangeLog
	role           auth.Role
	effectivePerms []auth.Permission
	rolesInherited []auth.Role
}

func (m *mockExtendedRBACManager) GetRoleWithInheritance(id string) (auth.Role, []auth.Permission, bool) {
	if m.role.ID == id {
		return m.role, m.effectivePerms, true
	}
	return auth.Role{}, nil, false
}

func (m *mockExtendedRBACManager) GetRolesWithInheritance(username string) []auth.Role {
	return m.rolesInherited
}

func (m *mockExtendedRBACManager) GetChangeLogs(limit int, offset int) []auth.RBACChangeLog {
	return m.logs
}

func (m *mockExtendedRBACManager) GetChangeLogsForEntity(entityType, entityID string) []auth.RBACChangeLog {
	return m.logs
}

func (m *mockExtendedRBACManager) SaveRoleWithContext(role auth.Role, username string) error {
	return m.SaveRole(role)
}

func (m *mockExtendedRBACManager) DeleteRoleWithContext(id string, username string) error {
	return m.DeleteRole(id)
}

func (m *mockExtendedRBACManager) UpdateUserRolesWithContext(username string, roleIDs []string, byUser string) error {
	return m.UpdateUserRoles(username, roleIDs)
}

func TestRBACHandlers_AdditionalBranches(t *testing.T) {
	orig := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(orig) })

	cfg := &config.Config{}
	handler := NewRBACHandlers(cfg)

	auth.SetManager(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/roles", nil)
	rr := httptest.NewRecorder()
	handler.HandleRoles(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rr.Code)
	}

	manager := &mockRBACManager{}
	auth.SetManager(manager)

	req = httptest.NewRequest(http.MethodGet, "/api/admin/roles/invalid$id", nil)
	rr = httptest.NewRecorder()
	handler.HandleRoles(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/roles", bytes.NewReader([]byte("{bad")))
	rr = httptest.NewRecorder()
	handler.HandleRoles(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/admin/roles", nil)
	rr = httptest.NewRecorder()
	handler.HandleRoles(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rr = httptest.NewRecorder()
	handler.HandleGetUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/users/", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/users/invalid$user/roles", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/admin/users/test/roles", bytes.NewReader([]byte("{bad")))
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	body, _ := json.Marshal(map[string]any{"roleIds": []string{"bad role"}})
	req = httptest.NewRequest(http.MethodPut, "/api/admin/users/test/roles", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	auth.SetManager(nil)
	req = httptest.NewRequest(http.MethodGet, "/api/admin/rbac/changelog", nil)
	rr = httptest.NewRecorder()
	handler.HandleRBACChangelog(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rr.Code)
	}

	extended := &mockExtendedRBACManager{
		logs: []auth.RBACChangeLog{{Action: "role_created"}},
		role: auth.Role{ID: "custom"},
		effectivePerms: []auth.Permission{
			{Action: "read", Resource: "nodes"},
		},
		rolesInherited: []auth.Role{{ID: "custom"}},
	}
	auth.SetManager(extended)

	req = httptest.NewRequest(http.MethodGet, "/api/admin/rbac/changelog?limit=1&offset=0", nil)
	rr = httptest.NewRecorder()
	handler.HandleRBACChangelog(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/roles/custom/effective", nil)
	rr = httptest.NewRecorder()
	handler.HandleRoleEffective(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/users/alice/effective-permissions", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserEffectivePermissions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
