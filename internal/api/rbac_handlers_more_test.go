package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type errorRBACManager struct {
	mockRBACManager
	saveErr    error
	deleteErr  error
	updateErr  error
	perms      []auth.Permission
	rolesByID  map[string]auth.Role
	assignByID map[string]auth.UserRoleAssignment
}

func (m *errorRBACManager) GetRole(id string) (auth.Role, bool) {
	if m.rolesByID != nil {
		role, ok := m.rolesByID[id]
		return role, ok
	}
	return m.mockRBACManager.GetRole(id)
}

func (m *errorRBACManager) SaveRole(role auth.Role) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	return m.mockRBACManager.SaveRole(role)
}

func (m *errorRBACManager) DeleteRole(id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return m.mockRBACManager.DeleteRole(id)
}

func (m *errorRBACManager) UpdateUserRoles(username string, roleIDs []string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return m.mockRBACManager.UpdateUserRoles(username, roleIDs)
}

func (m *errorRBACManager) GetUserAssignment(username string) (auth.UserRoleAssignment, bool) {
	if m.assignByID != nil {
		assignment, ok := m.assignByID[username]
		return assignment, ok
	}
	return m.mockRBACManager.GetUserAssignment(username)
}

func (m *errorRBACManager) GetUserPermissions(username string) []auth.Permission {
	if m.perms != nil {
		return m.perms
	}
	return nil
}

func TestRBACHandlers_HandleRoles_MoreBranches(t *testing.T) {
	orig := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(orig) })

	cfg := &config.Config{}
	handler := NewRBACHandlers(cfg)

	t.Run("MethodNotAllowed", func(t *testing.T) {
		auth.SetManager(&mockRBACManager{})
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/roles", nil)
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
		}
	})

	t.Run("GetRoleNotFound", func(t *testing.T) {
		auth.SetManager(&mockRBACManager{})
		req := httptest.NewRequest(http.MethodGet, "/api/admin/roles/missing", nil)
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("InvalidRoleIDInBody", func(t *testing.T) {
		auth.SetManager(&mockRBACManager{})
		role := auth.Role{ID: "bad role", Name: "Bad"}
		body, _ := json.Marshal(role)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/roles", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}

		req = httptest.NewRequest(http.MethodPut, "/api/admin/roles", bytes.NewReader(body))
		rr = httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("SaveRoleErrorOnCreate", func(t *testing.T) {
		auth.SetManager(&errorRBACManager{saveErr: errors.New("boom")})
		role := auth.Role{ID: "viewer", Name: "Viewer"}
		body, _ := json.Marshal(role)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/roles", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("SaveRoleErrorOnUpdate", func(t *testing.T) {
		auth.SetManager(&errorRBACManager{saveErr: errors.New("boom")})
		role := auth.Role{ID: "admin", Name: "Admin"}
		body, _ := json.Marshal(role)
		req := httptest.NewRequest(http.MethodPut, "/api/admin/roles/admin", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("DeleteRoleError", func(t *testing.T) {
		auth.SetManager(&errorRBACManager{deleteErr: errors.New("boom")})
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/roles/admin", nil)
		rr := httptest.NewRecorder()
		handler.HandleRoles(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestRBACHandlers_HandleGetUsers_MoreBranches(t *testing.T) {
	orig := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(orig) })

	cfg := &config.Config{}
	handler := NewRBACHandlers(cfg)

	auth.SetManager(&mockRBACManager{})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", nil)
	rr := httptest.NewRecorder()
	handler.HandleGetUsers(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	auth.SetManager(nil)
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rr = httptest.NewRecorder()
	handler.HandleGetUsers(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rr.Code)
	}
}

func TestRBACHandlers_HandleUserRoleActions_MoreBranches(t *testing.T) {
	orig := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(orig) })

	cfg := &config.Config{}
	handler := NewRBACHandlers(cfg)

	auth.SetManager(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users/alice/roles", nil)
	rr := httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rr.Code)
	}

	auth.SetManager(&errorRBACManager{assignByID: map[string]auth.UserRoleAssignment{}})
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users/alice/roles", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	auth.SetManager(&errorRBACManager{updateErr: errors.New("boom")})
	body, _ := json.Marshal(map[string]any{"roleIds": []string{"admin"}})
	req = httptest.NewRequest(http.MethodPut, "/api/admin/users/alice/roles", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	auth.SetManager(&mockRBACManager{})
	req = httptest.NewRequest(http.MethodDelete, "/api/admin/users/alice/roles", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserRoleActions(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestRBACHandlers_ChangelogAndEffectiveBranches(t *testing.T) {
	orig := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(orig) })

	cfg := &config.Config{}
	handler := NewRBACHandlers(cfg)

	auth.SetManager(&mockExtendedRBACManager{})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/changelog", nil)
	rr := httptest.NewRecorder()
	handler.HandleRBACChangelog(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	em := &mockExtendedRBACManager{}
	auth.SetManager(em)
	req = httptest.NewRequest(http.MethodGet, "/api/admin/rbac/changelog?entity_type=role&entity_id=custom", nil)
	rr = httptest.NewRecorder()
	handler.HandleRBACChangelog(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/admin/roles/invalid$id/effective", nil)
	rr = httptest.NewRecorder()
	handler.HandleRoleEffective(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	em = &mockExtendedRBACManager{role: auth.Role{ID: "existing"}}
	auth.SetManager(em)
	req = httptest.NewRequest(http.MethodGet, "/api/admin/roles/missing/effective", nil)
	rr = httptest.NewRecorder()
	handler.HandleRoleEffective(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/users/alice/effective-permissions", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserEffectivePermissions(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	auth.SetManager(&errorRBACManager{})
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users/invalid$user/effective-permissions", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserEffectivePermissions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	auth.SetManager(&errorRBACManager{perms: nil})
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users/alice/effective-permissions", nil)
	rr = httptest.NewRecorder()
	handler.HandleUserEffectivePermissions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
