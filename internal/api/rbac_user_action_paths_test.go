package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestRBACUserActionPathsRejectMisaddressedRequests(t *testing.T) {
	for _, tc := range []struct {
		path, method, allow string
		status              int
	}{
		{"alice", http.MethodPut, "GET, DELETE", http.StatusMethodNotAllowed},
		{"alice", http.MethodPost, "GET, DELETE", http.StatusMethodNotAllowed},
		{"alice/permissions", http.MethodPut, "GET", http.StatusMethodNotAllowed},
		{"alice/permissions", http.MethodPost, "GET", http.StatusMethodNotAllowed},
		{"alice/permissions", http.MethodDelete, "GET", http.StatusMethodNotAllowed},
		{"alice/permissions", http.MethodPatch, "GET", http.StatusMethodNotAllowed},
		{"alice/roles", http.MethodDelete, "GET, PUT, POST", http.StatusMethodNotAllowed},
		{"alice/roles", http.MethodPatch, "GET, PUT, POST", http.StatusMethodNotAllowed},
		{"alice/roles", http.MethodHead, "GET, PUT, POST", http.StatusMethodNotAllowed},
		{"alice/roles", http.MethodOptions, "GET, PUT, POST", http.StatusMethodNotAllowed},
		{"alice/role", http.MethodPut, "", http.StatusNotFound},
		{"alice/role", http.MethodPost, "", http.StatusNotFound},
		{"alice/role", http.MethodGet, "", http.StatusNotFound},
		{"alice/role", http.MethodDelete, "", http.StatusNotFound},
		{"alice/roles/private-path-fragment", http.MethodPut, "", http.StatusNotFound},
		{"alice/roles/private-path-fragment", http.MethodPost, "", http.StatusNotFound},
		{"alice/permissions/private-path-fragment", http.MethodGet, "", http.StatusNotFound},
		{"alice/permissions/private-path-fragment", http.MethodPut, "", http.StatusNotFound},
		{"alice/effective-permissions", http.MethodGet, "", http.StatusNotFound},
		{"alice/effective-permissions", http.MethodPut, "", http.StatusNotFound},
	} {
		for _, suffix := range []string{"", "/"} {
			for _, body := range []string{`{"roleIds":["admin"]}`, `{}`} {
				t.Run(tc.method+"/"+tc.path+suffix+"/"+body, func(t *testing.T) {
					h, manager, dir := rbacJSONFixture(t)
					if err := manager.UpdateUserRoles("neighbor", []string{auth.RoleOperator}); err != nil {
						t.Fatal(err)
					}
					before := rbacJSONSavedBytes(t, dir)
					assignment, _ := manager.GetUserAssignment("alice")
					neighbor, _ := manager.GetUserAssignment("neighbor")
					request := httptest.NewRequest(tc.method, "/api/admin/users/"+tc.path+suffix, strings.NewReader(body))
					request = request.WithContext(auth.WithUser(request.Context(), "maintainer"))
					response := httptest.NewRecorder()
					h.HandleUserRoleActions(response, request)
					if response.Code != tc.status || response.Header().Get("Allow") != tc.allow {
						t.Fatalf("route response = %d Allow=%q body=%q; want %d Allow=%q", response.Code, response.Header().Get("Allow"), response.Body.String(), tc.status, tc.allow)
					}
					if strings.Contains(response.Body.String(), "private-path-fragment") {
						t.Fatal("rejection echoed the rejected path")
					}
					current, _ := manager.GetUserAssignment("alice")
					adjacent, _ := manager.GetUserAssignment("neighbor")
					if !reflect.DeepEqual(assignment, current) || !reflect.DeepEqual(neighbor, adjacent) || !reflect.DeepEqual(before, rbacJSONSavedBytes(t, dir)) {
						t.Fatal("misaddressed request changed saved access")
					}
					reopened, err := auth.NewFileManager(dir)
					if err != nil {
						t.Fatal(err)
					}
					saved, ok := reopened.GetUserAssignment("alice")
					wantJSON, _ := json.Marshal(assignment)
					savedJSON, _ := json.Marshal(saved)
					if !ok || string(wantJSON) != string(savedJSON) {
						t.Fatal("misaddressed request changed access on reopen")
					}
				})
			}
		}
	}
}

func TestRBACUserActionPathsKeepSupportedRequests(t *testing.T) {
	// Keep the legacy bare-identity GET and one optional trailing slash as well
	// as the canonical /roles, /permissions and bare-identity DELETE contracts.
	for _, suffix := range []string{"", "/"} {
		t.Run("suffix="+suffix, func(t *testing.T) {
			h, manager, _ := rbacJSONFixture(t)
			serve := func(method, path, body string) *httptest.ResponseRecorder {
				request := httptest.NewRequest(method, "/api/admin/users/"+path+suffix, strings.NewReader(body))
				request = request.WithContext(auth.WithUser(request.Context(), "maintainer"))
				response := httptest.NewRecorder()
				h.HandleUserRoleActions(response, request)
				return response
			}
			for _, path := range []string{"alice", "alice/roles"} {
				response := serve(http.MethodGet, path, "")
				var assignment auth.UserRoleAssignment
				if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &assignment) != nil || assignment.Username != "alice" || !reflect.DeepEqual(assignment.RoleIDs, []string{auth.RoleViewer}) {
					t.Fatalf("assignment read = %d %s", response.Code, response.Body.String())
				}
			}
			response := serve(http.MethodGet, "alice/permissions", "")
			var permissions []auth.Permission
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &permissions) != nil || !reflect.DeepEqual(permissions, manager.GetUserPermissions("alice")) {
				t.Fatalf("permissions read = %d %s", response.Code, response.Body.String())
			}
			for _, method := range []string{http.MethodPut, http.MethodPost} {
				for _, body := range []string{`{"roleIds":["operator"]}`, `{"roleIds":[]}`} {
					response := serve(method, "alice/roles", body)
					if response.Code != http.StatusNoContent {
						t.Fatalf("supported assignment = %d %s", response.Code, response.Body.String())
					}
					assignment, _ := manager.GetUserAssignment("alice")
					if strings.Contains(body, "operator") && !reflect.DeepEqual(assignment.RoleIDs, []string{auth.RoleOperator}) {
						t.Fatal("normal write did not set the selected role")
					}
					if strings.Contains(body, "[]") && len(assignment.RoleIDs) != 0 {
						t.Fatal("explicit empty selection no longer clears roles")
					}
				}
			}
			response = serve(http.MethodDelete, "alice", "")
			if response.Code != http.StatusNoContent {
				t.Fatalf("supported removal = %d %s", response.Code, response.Body.String())
			}
			if _, ok := manager.GetUserAssignment("alice"); ok {
				t.Fatal("canonical removal left identity behind")
			}
		})
	}
}

// Rejection must happen before lazy store provisioning or consuming the body,
// even when storage is not available. No database or failure reproduction runs.
func TestRBACUserActionPathsRejectBeforeStoreAndBody(t *testing.T) {
	provider := NewTenantRBACProvider(t.TempDir())
	t.Cleanup(func() { _ = provider.Close() })
	h := NewRBACHandlers(nil, provider)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPut, "/api/admin/users/alice/permissions", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/admin/users/alice/role", http.StatusNotFound},
		{http.MethodGet, "/api/admin/users/alice/permissions/extra", http.StatusNotFound},
		{http.MethodPut, "/elsewhere/alice/roles", http.StatusNotFound},
		{http.MethodPut, "/api/admin/users/", http.StatusBadRequest},
		{http.MethodPut, "/api/admin/users/invalid$user/roles", http.StatusBadRequest},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			body := &rbacUnadmittedBody{t: t}
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Body = body
			response := httptest.NewRecorder()
			h.HandleUserRoleActions(response, request)
			if response.Code != tc.status {
				t.Fatalf("route status = %d; want %d", response.Code, tc.status)
			}
			if provider.ManagerCount() != 0 {
				t.Fatal("unmatched request provisioned a store")
			}
		})
	}
}

type rbacUnadmittedBody struct{ t *testing.T }

func (b *rbacUnadmittedBody) Read([]byte) (int, error) {
	b.t.Fatal("read unadmitted request body")
	return 0, nil
}
func (*rbacUnadmittedBody) Close() error { return nil }

func TestRBACUserActionPathsNativeHTTP(t *testing.T) {
	h, manager, dir := rbacJSONFixture(t)
	const principal = "oidc:provider:reader-1"
	if err := manager.UpdateUserRoles(principal, []string{auth.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/users/", h.HandleUserRoleActions)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"alice/permissions", http.StatusMethodNotAllowed},
		{"alice/role", http.StatusNotFound},
		{"alice/roles/extra", http.StatusNotFound},
		{"alice%2Froles", http.StatusBadRequest},
		{"alice%2fpermissions", http.StatusBadRequest},
		{"alice%252Froles", http.StatusBadRequest},
		{"alice/roles%2Fextra", http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			before := rbacJSONSavedBytes(t, dir)
			request, err := http.NewRequest(http.MethodPut, server.URL+"/api/admin/users/"+tc.path, strings.NewReader(`{"roleIds":["admin"]}`))
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("native route status=%d; want %d", response.StatusCode, tc.status)
			}
			if !reflect.DeepEqual(before, rbacJSONSavedBytes(t, dir)) {
				t.Fatal("escaped or unmatched path changed saved access")
			}
		})
	}
	// Normal client encoding of colons in an opaque SSO identity still targets
	// precisely that principal, not a prefix-matching local account.
	request, err := http.NewRequest(http.MethodPut, server.URL+"/api/admin/users/oidc%3Aprovider%3Areader-1/r%6Fles", strings.NewReader(`{"roleIds":["operator"]}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("encoded SSO assignment=%d", response.StatusCode)
	}
	assignment, ok := manager.GetUserAssignment(principal)
	if !ok || !reflect.DeepEqual(assignment.RoleIDs, []string{auth.RoleOperator}) {
		t.Fatal("encoded SSO write missed its exact principal")
	}
	alice, _ := manager.GetUserAssignment("alice")
	if !reflect.DeepEqual(alice.RoleIDs, []string{auth.RoleViewer}) {
		t.Fatal("encoded SSO write changed neighboring local principal")
	}
}
