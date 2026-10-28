package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/stretchr/testify/require"
)

func resetSelectionFixture(t *testing.T, contextOrg string) (*TenantRBACProvider, string, func(string, string) *httptest.ResponseRecorder) {
	t.Helper()
	dir := t.TempDir()
	if contextOrg != "" && strings.TrimSpace(contextOrg) != "default" {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "orgs", strings.TrimSpace(contextOrg)), 0700))
	}
	provider := NewTenantRBACProvider(dir)
	t.Cleanup(func() { _ = provider.Close() })
	resetRecoveryStore()
	t.Cleanup(resetRecoveryStore)
	InitRecoveryTokenStore(dir)
	token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5*time.Minute, "127.0.0.1")
	require.NoError(t, err)
	handlers := NewRBACHandlers(nil, provider)
	return provider, token, func(body, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", strings.NewReader(body))
		req.RemoteAddr = ip + ":12345"
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, contextOrg))
		rec := httptest.NewRecorder()
		handlers.HandleRBACAdminReset(rec, req)
		return rec
	}
}

func TestRBACResetSelectionRejectsBeforeSpendingToken(t *testing.T) {
	for _, contextOrg := range []string{"", "org-a"} {
		for _, tt := range []struct {
			name, field, code string
			status            int
		}{
			{"empty", `"org_id":""`, "invalid_org_id", 400},
			{"spaces", `"org_id":"  "`, "invalid_org_id", 400},
			{"escaped whitespace", `"org_id":"\t\r\n"`, "invalid_org_id", 400},
			{"unicode whitespace", `"org_id":"\u2003"`, "invalid_org_id", 400},
			{"null", `"org_id":null`, "invalid_org_id", 400},
			{"folded null", `"ORG_ID":null`, "invalid_org_id", 400},
			{"escaped key", `"org_\u0069d":""`, "invalid_org_id", 400},
			{"number", `"org_id":1`, "invalid_json", 400},
			{"boolean", `"org_id":true`, "invalid_json", 400},
			{"object", `"org_id":{}`, "invalid_json", 400},
			{"array", `"org_id":[]`, "invalid_json", 400},
			{"duplicate", `"org_id":null,"ORG_ID":"default"`, "invalid_json", 400},
			{"traversal", `"org_id":"../other"`, "invalid_org_id", 400},
			{"long", `"org_id":"` + strings.Repeat("a", 65) + `"`, "invalid_org_id", 400},
			{"foreign", `"org_id":"org-b"`, "access_denied", 403},
		} {
			t.Run(contextOrg+"/"+tt.name, func(t *testing.T) {
				provider, token, serve := resetSelectionFixture(t, contextOrg)
				body := fmt.Sprintf(`{%s,"username":"recovery-user","recovery_token":%q}`, tt.field, token)
				rec := serve(body, "127.0.0.1")
				require.Equal(t, tt.status, rec.Code, rec.Body.String())
				var failure APIError
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &failure))
				require.Equal(t, tt.code, failure.Code)
				require.NotContains(t, rec.Body.String(), token)
				require.Zero(t, provider.ManagerCount(), "rejection initialized or changed roles")
				org := contextOrg
				if org == "" {
					org = "default"
				}
				valid := fmt.Sprintf(`{"org_id":%q,"username":"recovery-user","recovery_token":%q}`, org, token)
				require.Equal(t, http.StatusForbidden, serve(valid, "192.0.2.2").Code, "token lost IP binding")
				require.Equal(t, http.StatusOK, serve(valid, "127.0.0.1").Code, "rejected selection consumed the original token")
				manager, err := provider.GetManager(org)
				require.NoError(t, err)
				assignment, ok := manager.GetUserAssignment("recovery-user")
				require.True(t, ok)
				require.Contains(t, assignment.RoleIDs, auth.RoleAdmin)
				require.Equal(t, http.StatusForbidden, serve(valid, "127.0.0.1").Code, "reset token became reusable")
			})
		}
	}
}

func TestRBACResetSelectionPreservesSupportedTargetsAndNames(t *testing.T) {
	for _, tt := range []struct{ name, contextOrg, field, username, wantOrg string }{
		{"omitted legacy", "", "", "recovery", "default"},
		{"omitted blank context", " \t ", "", "recovery", "default"},
		{"omitted scoped", "org-a", "", "recovery", "org-a"},
		{"explicit legacy", "", `"org_id":"default",`, "recovery", "default"},
		{"trimmed scoped", " org-a ", `"ORG_ID":" org-a ",`, "recovery", "org-a"},
		{"escaped key", "org-a", `"org_\u0069d":"org-a",`, "recovery", "org-a"},
		{"opaque exact name", "org-a", `"org_id":"org-a",`, "  sso:oidc:fixture:subject  ", "org-a"},
		{"international name", "org-a", "", "恢复管理员", "org-a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider, token, serve := resetSelectionFixture(t, tt.contextOrg)
			body := fmt.Sprintf(`{%s"username":%q,"recovery_token":%q,"future":{}}`, tt.field, tt.username, token)
			rec := serve(body, "127.0.0.1")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var result map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
			require.Equal(t, tt.wantOrg, result["org_id"])
			require.Equal(t, tt.username, result["username"])
			manager, err := provider.GetManager(tt.wantOrg)
			require.NoError(t, err)
			assignment, ok := manager.GetUserAssignment(tt.username)
			require.True(t, ok)
			require.Equal(t, []string{auth.RoleAdmin}, assignment.RoleIDs)
			require.Equal(t, http.StatusForbidden, serve(body, "127.0.0.1").Code)
		})
	}
}

func TestRBACResetSelectionFallbackAndEarlyChecks(t *testing.T) {
	for _, nilResult := range []bool{false, true} {
		t.Run(fmt.Sprint("nil-binder-result=", nilResult), func(t *testing.T) {
			SetRBACAdminEndpointsBinder(nil)
			t.Cleanup(func() { SetRBACAdminEndpointsBinder(nil) })
			if nilResult {
				SetRBACAdminEndpointsBinder(func(extensions.RBACAdminEndpoints, extensions.RBACAdminRuntime) extensions.RBACAdminEndpoints {
					return nil
				})
			}
			provider := NewTenantRBACProvider(t.TempDir())
			t.Cleanup(func() { _ = provider.Close() })
			h := NewRBACHandlers(nil, provider)
			endpoints := resolveRBACAdminEndpoints(rbacAdminEndpointAdapter{handlers: h}, newRBACAdminRuntime(h))
			rec := httptest.NewRecorder()
			endpoints.HandleAdminReset(rec, httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", strings.NewReader(`{"org_id":null,"username":"recovery-user","recovery_token":"unused"}`)))
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), `"invalid_org_id"`)
			require.Zero(t, provider.ManagerCount())
		})
	}
	for _, tt := range []struct {
		method string
		status int
	}{{http.MethodGet, 405}, {http.MethodPost, 501}} {
		rec := httptest.NewRecorder()
		NewRBACHandlers(nil, nil).HandleRBACAdminReset(rec, httptest.NewRequest(tt.method, "/api/admin/rbac/reset-admin", strings.NewReader(`{"org_id":null}`)))
		require.Equal(t, tt.status, rec.Code)
	}
}
