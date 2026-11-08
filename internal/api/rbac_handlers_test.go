package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
func (m *mockRBACManager) DeleteUser(username string) (bool, error) {
	for i, assignment := range m.assignments {
		if assignment.Username == username {
			m.assignments = append(m.assignments[:i], m.assignments[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
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

	t.Run("Block self deprovision", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/admin", nil)
		req = req.WithContext(auth.WithUser(req.Context(), "AdMiN"))
		rr := httptest.NewRecorder()

		h.HandleUserRoleActions(rr, req)

		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "self_deprovision_denied") {
			t.Fatalf("self deprovision response = %d %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Deprovision user and revoke sessions", func(t *testing.T) {
		resetSessionTracking()
		resetSessionStoreForTests()
		resetCSRFStoreForTests()
		t.Cleanup(resetSessionStoreForTests)
		t.Cleanup(resetCSRFStoreForTests)
		dir := t.TempDir()
		InitSessionStore(dir)
		InitCSRFStore(dir)
		mock.assignments = append(mock.assignments, auth.UserRoleAssignment{Username: "retired-user", RoleIDs: []string{"viewer"}})
		token := generateSessionToken()
		GetSessionStore().CreateSession(token, time.Hour, "agent", "127.0.0.1", "retired-user")

		req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/retired-user", nil)
		req = req.WithContext(auth.WithUser(req.Context(), "admin"))
		rr := httptest.NewRecorder()
		h.HandleUserRoleActions(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("deprovision response = %d %s", rr.Code, rr.Body.String())
		}
		if GetSessionStore().GetSession(token) != nil {
			t.Fatal("deprovision left persisted session active")
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

func TestRBACHandlersReturnEmptyAssignmentsAsArray(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })
	handler := NewRBACHandlers(&config.Config{DataPath: baseDir}, provider)

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	response := httptest.NewRecorder()
	handler.HandleGetUsers(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if response.Body.String() != "[]\n" {
		t.Fatalf("body = %q, want an empty JSON array", response.Body.String())
	}
}

func TestRBACHandlersReportLegacyMigrationFailure(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "rbac_roles.json"), []byte(`[{"id":`), 0600); err != nil {
		t.Fatalf("write corrupt legacy roles: %v", err)
	}

	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })
	handler := NewRBACHandlers(&config.Config{DataPath: baseDir}, provider)
	request := httptest.NewRequest(http.MethodGet, "/api/admin/roles", nil)
	response := httptest.NewRecorder()
	handler.HandleRoles(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"code":"rbac_store_unavailable"`)) {
		t.Fatalf("missing explicit RBAC store error: %s", response.Body.String())
	}

	// Management routes fail closed, but the store itself stays live so admin
	// recovery remains reachable. Failing the manager outright used to take
	// ResetAdminRole down with the rest of the surface, leaving no way back
	// from a rejected legacy import.
	if err := ResetAdminRole(provider, "default", "operator"); err != nil {
		t.Fatalf("admin recovery must stay reachable while RBAC routes fail closed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "rbac_roles.json")); err != nil {
		t.Fatalf("corrupt source was not preserved: %v", err)
	}
}

func TestRBACHandlersReportSQLiteReadFailure(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	manager, err := provider.GetManager("default")
	if err != nil {
		t.Fatalf("GetManager(default): %v", err)
	}
	sqliteManager, ok := manager.(*auth.SQLiteManager)
	if !ok {
		t.Fatalf("manager type = %T, want *auth.SQLiteManager", manager)
	}
	if err := sqliteManager.Close(); err != nil {
		t.Fatalf("close manager: %v", err)
	}

	handler := NewRBACHandlers(&config.Config{DataPath: baseDir}, provider)
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	response := httptest.NewRecorder()
	handler.HandleGetUsers(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"code":"rbac_store_unavailable"`)) {
		t.Fatalf("missing explicit RBAC store error: %s", response.Body.String())
	}
}

func TestRBACHandlersAllowStableSSOPrincipalAssignments(t *testing.T) {
	baseDir := t.TempDir()
	provider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() { _ = provider.Close() })
	handler := NewRBACHandlers(&config.Config{DataPath: baseDir}, provider)
	principal := "sso:oidc:okta:opaque-subject"

	body := bytes.NewBufferString(`{"roleIds":["viewer"]}`)
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/users/"+principal+"/roles",
		body,
	)
	response := httptest.NewRecorder()
	handler.HandleUserRoleActions(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", response.Code, response.Body.String())
	}
	manager, err := provider.GetManager("default")
	if err != nil {
		t.Fatalf("GetManager(default): %v", err)
	}
	assignment, ok := manager.GetUserAssignment(principal)
	if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != auth.RoleViewer {
		t.Fatalf("stable SSO assignment not saved: %#v, exists=%v", assignment, ok)
	}
}

func TestNewRouterSharesCanonicalRBACManagerWithSSOAndSettings(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	legacy, err := auth.NewFileManager(cfg.DataPath)
	if err != nil {
		t.Fatalf("NewFileManager: %v", err)
	}
	if err := legacy.SaveRole(auth.Role{
		ID:          "legacy-operator",
		Name:        "Legacy operator",
		Permissions: []auth.Permission{{Action: "read", Resource: "nodes"}},
	}); err != nil {
		t.Fatalf("save legacy role: %v", err)
	}
	if err := legacy.UpdateUserRoles("legacy-user", []string{"legacy-operator"}); err != nil {
		t.Fatalf("save legacy assignment: %v", err)
	}

	originalManager := auth.GetManager()
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	t.Cleanup(func() {
		router.shutdownBackgroundWorkers()
		router.ShutdownRBAC()
		auth.SetManager(originalManager)
	})

	settingsManager, err := router.rbacProvider.GetManager("default")
	if err != nil {
		t.Fatalf("GetManager(default): %v", err)
	}
	if auth.GetManager() != settingsManager {
		t.Fatal("SSO/authorization global manager differs from the settings manager")
	}
	assignment, ok := auth.GetManager().GetUserAssignment("legacy-user")
	if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != "legacy-operator" {
		t.Fatalf("upgraded legacy assignment unavailable to canonical manager: %#v, exists=%v", assignment, ok)
	}
}

// These same fixtures witness 404/partial-200 failures on the unmodified parent.
func TestRBACRoleDetailReadFailure(t *testing.T) {
	for _, orgID := range []string{"default", "org-a"} {
		for _, failure := range []string{"closed store", "invalid permission conditions"} {
			t.Run(orgID+"/"+failure, func(t *testing.T) {
				dir := t.TempDir()
				dataDir := dir
				if orgID != "default" {
					dataDir = filepath.Join(dir, "orgs", orgID)
					if err := os.MkdirAll(dataDir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				provider := NewTenantRBACProvider(dir)
				t.Cleanup(func() { _ = provider.Close() })
				manager, err := provider.GetManager(orgID)
				if err != nil {
					t.Fatal(err)
				}
				role := auth.Role{
					ID: "saved-reader", Name: "Private fixture role",
					Permissions: []auth.Permission{
						{Action: "read", Resource: "nodes", Conditions: map[string]string{"tag": "private-fixture"}},
					},
				}
				if err := manager.SaveRole(role); err != nil {
					t.Fatal(err)
				}
				if failure == "closed store" {
					if err := manager.(*auth.SQLiteManager).Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					db, err := sql.Open("sqlite", filepath.Join(dataDir, "rbac", "rbac.db"))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = db.Close() })
					// This is malformed application JSON in a disposable role,
					// not a damaged SQLite file or a driver/crash investigation.
					if _, err := db.Exec(`UPDATE rbac_permissions SET conditions = '{' WHERE role_id = ?`, role.ID); err != nil {
						t.Fatal(err)
					}
				}
				req := httptest.NewRequest(http.MethodGet, "/api/admin/roles/"+role.ID, nil)
				req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
				rec := httptest.NewRecorder()
				NewRBACHandlers(nil, provider).HandleRoles(rec, req)
				t.Logf("role detail response: status=%d body=%s", rec.Code, rec.Body.String())
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want 503", rec.Code)
				}
				var body APIError
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Code != "rbac_store_unavailable" {
					t.Fatalf("error code = %q", body.Code)
				}
				for _, private := range []string{dir, role.ID, role.Name, "private-fixture", "database is closed", "parse conditions"} {
					if strings.Contains(rec.Body.String(), private) {
						t.Fatalf("error response disclosed fixture or store detail %q", private)
					}
				}
			})
		}
	}
}

func TestRBACRoleDetailReadPreservesSavedAndMissingRoles(t *testing.T) {
	dir := t.TempDir()
	provider := NewTenantRBACProvider(dir)
	t.Cleanup(func() { _ = provider.Close() })
	handler := NewRBACHandlers(nil, provider)
	for i, orgID := range []string{"default", "org-a", "org-b"} {
		if orgID != "default" {
			if err := os.MkdirAll(filepath.Join(dir, "orgs", orgID), 0700); err != nil {
				t.Fatal(err)
			}
		}
		manager, err := provider.GetManager(orgID)
		if err != nil {
			t.Fatal(err)
		}
		// Identical role IDs in three stores still select only the admitted org.
		role := auth.Role{ID: "saved-reader", Name: orgID, ParentID: auth.RoleViewer, Priority: i,
			Permissions: []auth.Permission{{Action: "read", Resource: "nodes:" + orgID,
				Effect: auth.EffectDeny, Conditions: map[string]string{"tag": orgID}}}}
		if err := manager.SaveRole(role); err != nil {
			t.Fatal(err)
		}
		if err := manager.SaveRole(auth.Role{ID: "no-permissions", Name: "No permissions"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, orgID := range []string{"default", "org-a", "org-b"} {
		for _, roleID := range []string{"saved-reader", "no-permissions", "missing"} {
			t.Run(orgID+"/"+roleID, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/api/admin/roles/"+roleID, nil)
				req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
				rec := httptest.NewRecorder()
				handler.HandleRoles(rec, req)
				manager, err := provider.GetManager(orgID)
				if err != nil {
					t.Fatal(err)
				}
				want, exists := manager.GetRole(roleID)
				if !exists {
					var body APIError
					if rec.Code != http.StatusNotFound || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Code != "not_found" {
						t.Fatalf("missing role = %d %s", rec.Code, rec.Body.String())
					}
					return
				}
				var role auth.Role
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &role) != nil || strings.TrimSpace(rec.Body.String()) != string(wantJSON) {
					t.Fatalf("saved role = %d %s; want %+v", rec.Code, rec.Body.String(), want)
				}
				if rec.Header().Get("Content-Type") != "application/json" || role.Permissions == nil {
					t.Fatal("role response lost JSON or its (possibly empty) permissions array")
				}
			})
		}
	}
}

type roleDetailErrorManager struct {
	mockRBACManager
	err error
}

func (m *roleDetailErrorManager) GetRole(string) (auth.Role, bool) {
	panic("error-aware role read must not fall back to the legacy reader")
}

func (m *roleDetailErrorManager) GetRoleWithError(string) (auth.Role, bool, error) {
	// Even a different store returning partial data with an error cannot have
	// that data published as success or translated into a missing role.
	return auth.Role{ID: "partial-private-role"}, true, m.err
}

func TestRBACRoleDetailReadErrorAwareAndLegacyManagers(t *testing.T) {
	original := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(original) })
	handler := NewRBACHandlers(nil)
	auth.SetManager(&roleDetailErrorManager{err: errors.New("private fixture failure")})
	rec := httptest.NewRecorder()
	handler.HandleRoles(rec, httptest.NewRequest(http.MethodGet, "/api/admin/roles/reader", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":"rbac_store_unavailable"`) || strings.Contains(rec.Body.String(), "private") {
		t.Fatalf("error-aware read = %d %s", rec.Code, rec.Body.String())
	}
	legacy := &mockRBACManager{roles: []auth.Role{{ID: "reader", Name: "Legacy reader", Permissions: []auth.Permission{}}}}
	auth.SetManager(legacy)
	rec = httptest.NewRecorder()
	handler.HandleRoles(rec, httptest.NewRequest(http.MethodGet, "/api/admin/roles/reader", nil))
	var role auth.Role
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &role) != nil || !reflect.DeepEqual(role, legacy.roles[0]) {
		t.Fatalf("legacy read = %d %s", rec.Code, rec.Body.String())
	}
}
