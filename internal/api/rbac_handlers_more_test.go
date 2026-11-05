package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// These controls use the ordinary file-backed manager, so rejecting a body must
// preserve both the live policy and the bytes that the next process will read.
func rbacJSONFixture(t *testing.T) (*RBACHandlers, *auth.FileManager, string) {
	t.Helper()
	dir := t.TempDir()
	manager, err := auth.NewFileManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveRole(auth.Role{ID: "limited", Name: "Saved reader", Description: "Keep this policy", Permissions: []auth.Permission{{Action: "read", Resource: "nodes", Effect: "deny", Conditions: map[string]string{"tag": "production"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateUserRoles("alice", []string{auth.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	original := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(original) })
	auth.SetManager(manager)
	return NewRBACHandlers(nil), manager, dir
}

func rbacJSONSavedBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for _, name := range []string{"rbac_roles.json", "rbac_assignments.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		result[name] = string(data)
	}
	return result
}

func TestRBACMutationJSONRejectsAmbiguousRoles(t *testing.T) {
	valid := `{"id":"limited","name":"Changed","permissions":[{"action":"admin","resource":"*"}]}`
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, tc := range []struct{ name, body string }{
			{"null", `null`},
			{"array", `[]`},
			{"empty", ``},
			{"incomplete", `{"id":"limited"`},
			{"duplicate id", `{"id":"other","id":"limited"}`},
			{"escaped id", `{"id":"other","i\u0064":"limited"}`},
			{"folded name", `{"id":"limited","name":"Keep","NAME":"Changed"}`},
			{"duplicate identical", `{"id":"limited","name":"Changed","name":"Changed"}`},
			{"duplicate permissions", `{"id":"limited","permissions":[],"PERMISSIONS":[{"action":"admin","resource":"*"}]}`},
			{"duplicate action", `{"id":"limited","permissions":[{"action":"read","ACTION":"admin","resource":"*"}]}`},
			{"duplicate resource", `{"id":"limited","permissions":[{"action":"read","resource":"nodes","resourc\u0065":"*"}]}`},
			{"duplicate effect", `{"id":"limited","permissions":[{"action":"read","resource":"nodes","effect":"deny","Effect":"allow"}]}`},
			{"duplicate condition", `{"id":"limited","permissions":[{"action":"read","resource":"nodes","conditions":{"tag":"production","t\u0061g":"private-fragment-not-public"}}]}`},
			{"second object", valid + `{}`},
			{"trailing junk", valid + `private-fragment-not-public`},
			{"oversized suffix", valid + strings.Repeat(" ", 64*1024)},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				h, manager, dir := rbacJSONFixture(t)
				before := rbacJSONSavedBytes(t, dir)
				policy, _ := manager.GetRole("limited")
				count := len(manager.GetRoles())
				target := "/api/admin/roles"
				if method == http.MethodPut {
					target += "/limited"
				}
				rec := httptest.NewRecorder()
				h.HandleRoles(rec, httptest.NewRequest(method, target, strings.NewReader(tc.body)))
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_json"`) {
					t.Fatalf("rejection = %d %s", rec.Code, rec.Body.String())
				}
				if strings.Contains(rec.Body.String(), "private-fragment-not-public") {
					t.Fatal("rejection echoed request data")
				}
				after, _ := manager.GetRole("limited")
				if !reflect.DeepEqual(policy, after) || len(manager.GetRoles()) != count {
					t.Fatal("rejected body changed live roles")
				}
				if !reflect.DeepEqual(before, rbacJSONSavedBytes(t, dir)) {
					t.Fatal("rejected body changed persisted access")
				}
			})
		}
	}
}

func TestRBACMutationJSONRejectsAmbiguousAssignments(t *testing.T) {
	valid := `{"roleIds":["admin"]}`
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, tc := range []struct{ name, body string }{
			{"null", `null`},
			{"array", `[]`},
			{"empty", ``},
			{"wrong type", `{"roleIds":"admin"}`},
			{"duplicate", `{"roleIds":["viewer"],"roleIds":["admin"]}`},
			{"escaped duplicate", `{"roleIds":["viewer"],"roleId\u0073":["admin"]}`},
			{"folded duplicate", `{"roleIds":["viewer"],"ROLEIDS":["admin"]}`},
			{"unicode duplicate", `{"roleIds":["viewer"],"roleIdſ":[]}`},
			{"identical duplicate", `{"roleIds":["viewer"],"roleIds":["viewer"]}`},
			{"second object", valid + `{}`},
			{"trailing junk", valid + `private-fragment-not-public`},
			{"oversized suffix", valid + strings.Repeat(" ", 64*1024)},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				h, manager, dir := rbacJSONFixture(t)
				before := rbacJSONSavedBytes(t, dir)
				assignment, _ := manager.GetUserAssignment("alice")
				rec := httptest.NewRecorder()
				h.HandleUserRoleActions(rec, httptest.NewRequest(method, "/api/admin/users/alice/roles", strings.NewReader(tc.body)))
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_json"`) {
					t.Fatalf("rejection = %d %s", rec.Code, rec.Body.String())
				}
				if strings.Contains(rec.Body.String(), "private-fragment-not-public") {
					t.Fatal("rejection echoed request data")
				}
				after, _ := manager.GetUserAssignment("alice")
				if !reflect.DeepEqual(assignment, after) {
					t.Fatal("rejected body changed live assignment")
				}
				if !reflect.DeepEqual(before, rbacJSONSavedBytes(t, dir)) {
					t.Fatal("rejected body changed persisted access")
				}
			})
		}
	}
}

type rbacJSONBrokenBody struct{ body string }

func (b *rbacJSONBrokenBody) Read(p []byte) (int, error) {
	if b.body == "" {
		return 0, errors.New("synthetic body transport failure")
	}
	n := copy(p, b.body)
	b.body = b.body[n:]
	return n, nil
}
func (*rbacJSONBrokenBody) Close() error { return nil }

func TestRBACMutationJSONRejectsUnreadRemainder(t *testing.T) {
	for _, role := range []bool{false, true} {
		h, _, dir := rbacJSONFixture(t)
		before := rbacJSONSavedBytes(t, dir)
		target, body := "/api/admin/users/alice/roles", `{"roleIds":["admin"]}`
		handler := h.HandleUserRoleActions
		if role {
			target, body, handler = "/api/admin/roles/limited", `{"id":"limited","permissions":[]}`, h.HandleRoles
		}
		req := httptest.NewRequest(http.MethodPut, target, nil)
		req.Body = &rbacJSONBrokenBody{body: body}
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusBadRequest || !reflect.DeepEqual(before, rbacJSONSavedBytes(t, dir)) {
			t.Fatalf("incomplete body changed access: status %d", rec.Code)
		}
	}
}

func TestRBACMutationJSONKeepsSupportedObjects(t *testing.T) {
	t.Run("role aliases and complete policy", func(t *testing.T) {
		h, manager, _ := rbacJSONFixture(t)
		body := `{"ID":"limited","NAME":"Deliberate policy","parentId":"viewer","priority":17,"permissions":[{"ACTION":"read","resource":"nodes","effect":"deny","conditions":{"Ops":"one","ops":"two"}}],"future":{"name":"ignored","NAME":"also ignored"}}`
		// A complete body exactly at the existing 64 KiB bound is still valid.
		body += strings.Repeat(" ", 64*1024-len(body))
		rec := httptest.NewRecorder()
		h.HandleRoles(rec, httptest.NewRequest(http.MethodPut, "/api/admin/roles/limited", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("supported role = %d %s", rec.Code, rec.Body.String())
		}
		role, _ := manager.GetRole("limited")
		if role.Name != "Deliberate policy" || role.ParentID != "viewer" || role.Priority != 17 || len(role.Permissions) != 1 || role.Permissions[0].Effect != "deny" || len(role.Permissions[0].Conditions) != 2 {
			t.Fatalf("policy lost fields: %+v", role)
		}
	})
	for _, body := range []string{`{"ROLEIDS":["operator"],"future":{}}`, `{"roleIdſ":["operator"]}`, `{"roleIds":[]}`, `{"roleIds":null}`, `{}`} {
		t.Run("assignment "+body, func(t *testing.T) {
			h, manager, _ := rbacJSONFixture(t)
			rec := httptest.NewRecorder()
			h.HandleUserRoleActions(rec, httptest.NewRequest(http.MethodPut, "/api/admin/users/alice/roles", strings.NewReader(body)))
			if rec.Code != http.StatusNoContent {
				t.Fatalf("supported assignment = %d %s", rec.Code, rec.Body.String())
			}
			got, _ := manager.GetUserAssignment("alice")
			want := []string(nil)
			if strings.Contains(body, "operator") {
				want = []string{auth.RoleOperator}
			} else if strings.Contains(body, "[]") {
				want = []string{}
			}
			if !reflect.DeepEqual(want, got.RoleIDs) {
				t.Fatalf("roles = %#v, want %#v", got.RoleIDs, want)
			}
		})
	}
}
