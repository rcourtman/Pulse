package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// The registered router must not advertise or admit instance administration
// from a policy whose saved parent restriction cannot be read.
func TestRBACInheritedPolicyFailureProtectsSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	manager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	for _, role := range []auth.Role{
		{ID: "fixture-deny-parent", Name: "Saved parent restriction", Permissions: []auth.Permission{{Action: "admin", Resource: "*", Effect: auth.EffectDeny}}},
		{ID: "fixture-child", Name: "Scoped inherited role", ParentID: "fixture-deny-parent", Permissions: []auth.Permission{{Action: "admin", Resource: "*", Effect: auth.EffectAllow}}},
	} {
		if err := manager.SaveRole(role); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.UpdateUserRoles("scoped-session", []string{"fixture-child"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateUserRoles("legitimate-admin", []string{auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	oldManager, oldAuthorizer := auth.GetManager(), auth.GetAuthorizer()
	auth.SetManager(manager)
	auth.SetAuthorizer(auth.NewRBACAuthorizer(manager))
	t.Cleanup(func() { auth.SetManager(oldManager); auth.SetAuthorizer(oldAuthorizer) })
	cfg := newTestConfigWithTokens(t, config.APITokenRecord{ID: "fixture-monitor", Name: "Monitoring token", Scopes: []string{config.ScopeMonitoringRead}, CreatedAt: time.Unix(1728000000, 0)})
	cfg.DataPath, cfg.ConfigPath = dir, dir
	cfg.AuthUser, cfg.AuthPass = "local-admin", "fixture-unused-hash"
	savedTokens := append([]config.APITokenRecord(nil), cfg.APITokens...)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "fixture")
	db, err := sql.Open("sqlite", filepath.Join(dir, "rbac", "rbac.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, state := range []string{"healthy-deny", "unreadable-deny", "recovered-deny", "healthy-allow", "local-admin"} {
		var conditions any
		if state == "unreadable-deny" || state == "local-admin" {
			conditions = "{"
		}
		if _, err := db.Exec(`UPDATE rbac_permissions SET conditions = ? WHERE role_id = 'fixture-deny-parent'`, conditions); err != nil {
			t.Fatal(err)
		}
		user := "scoped-session"
		if state == "healthy-allow" {
			user = "legitimate-admin"
		}
		if state == "local-admin" {
			user = "local-admin"
		}
		session := "fixture-" + state
		GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", user)
		t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
		for _, path := range []string{"/api/security/status", "/api/security/tokens"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			allowed := state == "healthy-allow" || state == "local-admin"
			if path == "/api/security/status" {
				if rec.Code != 200 {
					t.Fatalf("security status %s: %d", state, rec.Code)
				}
				var status struct {
					CurrentUsername      string                             `json:"currentUsername"`
					SettingsCapabilities securityStatusSettingsCapabilities `json:"settingsCapabilities"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
					t.Fatal(err)
				}
				if status.CurrentUsername != user || status.SettingsCapabilities.APIAccessRead != allowed || status.SettingsCapabilities.APIAccessWrite != allowed {
					t.Fatalf("capability snapshot for %s: %s", state, rec.Body.String())
				}
			} else {
				want := http.StatusForbidden
				if allowed {
					want = http.StatusOK
				} else if state == "unreadable-deny" {
					want = http.StatusInternalServerError
				}
				if rec.Code != want {
					t.Fatalf("token read %s: status=%d want=%d body=%s", state, rec.Code, want, rec.Body.String())
				}
				if !allowed {
					for _, private := range []string{dir, "fixture-monitor", "Monitoring token", "fixture-deny-parent", "parse conditions"} {
						if strings.Contains(rec.Body.String(), private) {
							t.Fatalf("refusal disclosed %q", private)
						}
					}
					if state == "unreadable-deny" && !strings.Contains(rec.Body.String(), `"error":"internal_error"`) {
						t.Fatalf("read failure was not explicit: %s", rec.Body.String())
					}
				} else if !strings.Contains(rec.Body.String(), "Monitoring token") {
					t.Fatalf("permitted inventory lost: %s", rec.Body.String())
				}
			}
		}
		if state == "healthy-deny" || state == "unreadable-deny" || state == "recovered-deny" {
			req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"must-not-be-created","scopes":["*"]}`))
			req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			want := http.StatusForbidden
			if state == "unreadable-deny" {
				want = http.StatusInternalServerError
			}
			if rec.Code != want {
				t.Fatalf("token creation %s: %d, want %d: %s", state, rec.Code, want, rec.Body.String())
			}
		}

	}
	if !reflect.DeepEqual(cfg.APITokens, savedTokens) {
		t.Fatal("permission reads changed saved tokens")
	}
}
