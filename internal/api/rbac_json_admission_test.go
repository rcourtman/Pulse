package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

func TestRBACAdminJSONRejectsBeforeTokenConsumption(t *testing.T) {
	for _, tt := range []struct{ name, body string }{
		{"username", `{"username":"intended","username":"other","recovery_token":%q}`},
		{"identical username", `{"username":"intended","username":"intended","recovery_token":%q}`},
		{"escaped username", `{"username":"intended","usernam\u0065":"other","recovery_token":%q}`},
		{"folded username", `{"username":"intended","USERNAME":"other","recovery_token":%q}`},
		{"unicode folded username", `{"username":"intended","uſername":"other","recovery_token":%q}`},
		{"org scope", `{"username":"intended","org_id":"other","org_id":"default","recovery_token":%q}`},
		{"escaped org scope", `{"username":"intended","org_id":"other","org_\u0069d":"default","recovery_token":%q}`},
		{"folded org scope", `{"username":"intended","org_id":"other","ORG_ID":"default","recovery_token":%q}`},
		{"token", `{"username":"intended","recovery_token":"not-valid","recovery_token":%q}`},
		{"folded token", `{"username":"intended","recovery_token":"not-valid","RECOVERY_TOKEN":%q}`},
		{"trailing document", `{"username":"intended","recovery_token":%q}{}`},
		{"trailing junk", `{"username":"intended","recovery_token":%q}x`},
		{"oversized suffix", `{"username":"intended","recovery_token":%q}` + strings.Repeat(" ", 4*1024)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			provider := NewTenantRBACProvider(baseDir)
			t.Cleanup(func() { _ = provider.Close() })
			manager, err := provider.GetManager("default")
			require.NoError(t, err)
			resetRecoveryStore()
			t.Cleanup(resetRecoveryStore)
			InitRecoveryTokenStore(baseDir)
			token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5*time.Minute, "127.0.0.1")
			require.NoError(t, err)
			handlers := NewRBACHandlers(nil, provider)
			serve := func(body string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", strings.NewReader(body))
				req.RemoteAddr = "127.0.0.1:12345"
				rec := httptest.NewRecorder()
				handlers.HandleRBACAdminReset(rec, req)
				return rec
			}
			rec := serve(fmt.Sprintf(tt.body, token))
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), `"invalid_json"`)
			require.NotContains(t, rec.Body.String(), token)
			for _, user := range []string{"intended", "other"} {
				_, assigned := manager.GetUserAssignment(user)
				require.False(t, assigned, "rejected request assigned a role")
			}
			// The same token remains available for one deliberate, scoped reset.
			valid := fmt.Sprintf(`{"USERNAME":"intended","ORG_ID":"default","recovery_\u0074oken":%q,"future":{}}`, token)
			require.Equal(t, http.StatusOK, serve(valid).Code, "rejected input consumed the one-use token")
			assignment, ok := manager.GetUserAssignment("intended")
			require.True(t, ok)
			require.Contains(t, assignment.RoleIDs, auth.RoleAdmin)
			_, assigned := manager.GetUserAssignment("other")
			require.False(t, assigned)
			require.Equal(t, http.StatusForbidden, serve(valid).Code, "successful reset must consume the token")
		})
	}
}

func TestRBACAdminJSONKeepsTenantBoundary(t *testing.T) {
	provider := NewTenantRBACProvider(t.TempDir())
	t.Cleanup(func() { _ = provider.Close() })
	handlers := NewRBACHandlers(nil, provider)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", strings.NewReader(`{"ORG_ID":"org-b","USERNAME":"intended","RECOVERY_TOKEN":"not-used"}`))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	rec := httptest.NewRecorder()
	handlers.HandleRBACAdminReset(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"access_denied"`)
}
