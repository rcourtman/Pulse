package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestRBACSavedPermissionsPreserveRules(t *testing.T) {
	original := auth.GetManager()
	t.Cleanup(func() { auth.SetManager(original) })
	for _, kind := range []string{"file", "sqlite"} {
		t.Run(kind, func(t *testing.T) {
			var manager auth.Manager
			if kind == "file" {
				m, err := auth.NewFileManager(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				manager = m
			} else {
				m, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: t.TempDir()})
				if err != nil {
					t.Fatal(err)
				}
				defer m.Close()
				manager = m
			}
			parent := auth.Role{ID: "inherited-auditor", Name: "Inherited auditor", Permissions: []auth.Permission{{Action: "read", Resource: "backups", Effect: auth.EffectAllow}, {Action: "delete", Resource: "nodes", Effect: auth.EffectDeny}}}
			child := auth.Role{ID: "constrained-reader", Name: "Constrained reader", ParentID: parent.ID, Permissions: []auth.Permission{
				{Action: "read", Resource: "nodes", Effect: auth.EffectAllow},
				{Action: "read", Resource: "nodes", Effect: auth.EffectDeny, Conditions: map[string]string{"env": "production"}},
				{Action: "read", Resource: "nodes", Effect: auth.EffectAllow, Conditions: map[string]string{"env": "test"}},
				{Action: "read", Resource: "nodes", Effect: auth.EffectAllow, Conditions: map[string]string{"env": "development"}},
			}}
			for _, role := range []auth.Role{parent, child} {
				if err := manager.SaveRole(role); err != nil {
					t.Fatal(err)
				}
			}
			if err := manager.UpdateUserRoles("saved-reader", []string{child.ID}); err != nil {
				t.Fatal(err)
			}
			auth.SetManager(manager)
			handler := NewRBACHandlers(&config.Config{})
			rr := httptest.NewRecorder()
			handler.HandleUserRoleActions(rr, httptest.NewRequest(http.MethodGet, "/api/admin/users/saved-reader/permissions", nil))
			t.Logf("SAVED_POLICY_FIXTURE_%s=%s", kind, rr.Body.String())
			if rr.Code != http.StatusOK {
				t.Fatalf("status=%d: %s", rr.Code, rr.Body.String())
			}
			var got []auth.Permission
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := append(append([]auth.Permission{}, parent.Permissions...), child.Permissions...)
			// Compare exact fields without relying on a map's output order.
			if len(got) != len(want) {
				t.Errorf("saved rule count=%d, want %d", len(got), len(want))
			}
			for _, permission := range want {
				raw, _ := json.Marshal(permission)
				found := false
				for _, actual := range got {
					value, _ := json.Marshal(actual)
					if string(value) == string(raw) {
						found = true
					}
				}
				if !found {
					t.Errorf("lost rule %s", raw)
				}
			}
			if kind == "file" {
				if err := manager.DeleteRole(parent.ID); err != nil {
					t.Fatal(err)
				}
				rr = httptest.NewRecorder()
				handler.HandleUserRoleActions(rr, httptest.NewRequest(http.MethodGet, "/api/admin/users/saved-reader/permissions", nil))
				t.Logf("SAVED_POLICY_FAILURE_FIXTURE=%s", rr.Body.String())
				if rr.Code != http.StatusServiceUnavailable {
					t.Errorf("missing inherited policy status=%d, want503", rr.Code)
				}
			}
		})
	}
}
