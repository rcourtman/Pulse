package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Instance administration is the same boundary for API tokens as for
// sessions. A token bound to a tenant organization acts for that organization,
// so on instance routes (RequireAdmin, ensureAdminSession and the settings
// scope helpers) it is refused whatever scopes it holds, the same as that
// organization's owner. Routes that serve only the request organization
// (RequireOrgAdmin, ensureOrgAdminSession, ensureOrgSettingsWriteScope) keep
// judging tokens by scope alone. Before this, an org-bound token holding
// settings:write reached system settings, SSO providers, token management,
// updates, relay, log level and the other instance surfaces.

type orgBoundTokenBinding struct {
	name     string
	orgID    string
	orgIDs   []string
	instance bool // holds instance-wide authority
}

var orgBoundTokenBindings = []orgBoundTokenBinding{
	{name: "unbound legacy token", instance: true},
	{name: "bound to the default org", orgID: "default", instance: true},
	{name: "OrgIDs naming only the default org", orgIDs: []string{"default"}, instance: true},
	{name: "bound to a tenant org", orgID: "org-a", instance: false},
	{name: "OrgIDs naming a tenant org", orgIDs: []string{"org-a"}, instance: false},
	{name: "OrgIDs naming the default and a tenant org", orgIDs: []string{"default", "org-a"}, instance: false},
	{name: "OrgIDs taking precedence over a default OrgID", orgID: "default", orgIDs: []string{"org-a"}, instance: false},
}

func orgBoundTokenFixture(t *testing.T, binding orgBoundTokenBinding, scopes []string) (*config.Config, string) {
	t.Helper()
	raw := "org-bound-authority-token-" + strings.ReplaceAll(binding.name, " ", "-") + ".12345678"
	record := newTokenRecord(t, raw, scopes, nil)
	record.OrgID = binding.orgID
	record.OrgIDs = append([]string(nil), binding.orgIDs...)
	return newTestConfigWithTokens(t, record), raw
}

func orgBoundTokenRequest(t *testing.T, cfg *config.Config, method, target, raw string) *http.Request {
	t.Helper()
	req := newLoopbackRequest(method, target, nil)
	record, ok := cfg.ValidateAPIToken(raw)
	if !ok || record == nil {
		t.Fatalf("fixture token did not validate")
	}
	attachAPITokenRecord(req, record)
	return req
}

func TestInstanceSettingsScopeRequiresInstanceWideToken(t *testing.T) {
	for _, binding := range orgBoundTokenBindings {
		for _, scopes := range [][]string{
			{config.ScopeSettingsRead, config.ScopeSettingsWrite},
			{config.ScopeWildcard},
			{config.ScopeMonitoringRead},
		} {
			hasSettings := scopes[0] != config.ScopeMonitoringRead
			t.Run(binding.name+"/"+strings.Join(scopes, ","), func(t *testing.T) {
				cfg, raw := orgBoundTokenFixture(t, binding, scopes)
				checks := []struct {
					name string
					fn   func(*config.Config, http.ResponseWriter, *http.Request) bool
					want bool
				}{
					{"ensureSettingsReadScope", ensureSettingsReadScope, binding.instance && hasSettings},
					{"ensureSettingsWriteScope", ensureSettingsWriteScope, binding.instance && hasSettings},
					{"ensureOrgSettingsWriteScope", ensureOrgSettingsWriteScope, hasSettings},
					{"ensureAdminSession", ensureAdminSession, binding.instance},
					{"ensureOrgAdminSession", ensureOrgAdminSession, true},
				}
				for _, check := range checks {
					rec := httptest.NewRecorder()
					got := check.fn(cfg, rec, orgBoundTokenRequest(t, cfg, http.MethodPost, "/api/x", raw))
					if got != check.want {
						t.Fatalf("%s = %v, want %v", check.name, got, check.want)
					}
					if !got && rec.Code != http.StatusForbidden {
						t.Fatalf("%s refused with %d, want 403", check.name, rec.Code)
					}
				}
			})
		}
	}
}

func TestRequireAdminRequiresInstanceWideTokenAndRequireOrgAdminDoesNot(t *testing.T) {
	for _, binding := range orgBoundTokenBindings {
		t.Run(binding.name, func(t *testing.T) {
			cfg, raw := orgBoundTokenFixture(t, binding, []string{config.ScopeSettingsWrite})
			reached := func(wrap func(*config.Config, http.HandlerFunc) http.HandlerFunc) (bool, int) {
				hit := false
				rec := httptest.NewRecorder()
				req := newLoopbackRequest(http.MethodPost, "/api/x", nil)
				req.Header.Set("X-API-Token", raw)
				wrap(cfg, RequireScope(config.ScopeSettingsWrite, func(http.ResponseWriter, *http.Request) { hit = true }))(rec, req)
				return hit, rec.Code
			}
			if hit, code := reached(RequireAdmin); hit != binding.instance {
				t.Fatalf("RequireAdmin reached handler = %v (status %d), want %v", hit, code, binding.instance)
			} else if !hit && code != http.StatusForbidden {
				t.Fatalf("RequireAdmin refused with %d, want 403", code)
			}
			if hit, code := reached(RequireOrgAdmin); !hit {
				t.Fatalf("RequireOrgAdmin refused a settings:write token with %d", code)
			}
		})
	}
}

// The full router: an org-bound token resolves to its own organization
// without naming one, passes the tenant middleware, and must still be refused
// by every instance route while reaching the org-bound routes its scope covers.
func TestOrgBoundSettingsTokenRouterMatrix(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	scopes := []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}
	instanceRaw := "instance-settings-token.12345678"
	orgRaw := "org-a-settings-token.12345678"
	orgReadOnlyRaw := "org-a-monitoring-token.12345678"
	instance := newTokenRecord(t, instanceRaw, scopes, nil)
	orgBound := newTokenRecord(t, orgRaw, scopes, nil)
	orgBound.OrgID = "org-a"
	orgReadOnly := newTokenRecord(t, orgReadOnlyRaw, []string{config.ScopeMonitoringRead}, nil)
	orgReadOnly.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, instance, orgBound, orgReadOnly)
	mtp := config.NewMultiTenantPersistence(cfg.DataPath)
	if err := mtp.SaveOrganization(&models.Organization{ID: "org-a", DisplayName: "Org A", OwnerUserID: "owner"}); err != nil {
		t.Fatalf("save organization: %v", err)
	}
	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	router := NewRouter(cfg, nil, mtm, nil, nil, "1.0.0")
	cleanupTestRouter(t, router)

	serve := func(method, path, raw string) *httptest.ResponseRecorder {
		body := ""
		switch {
		case method == http.MethodGet:
		case path == "/api/security/tokens":
			// Without scopes the new token would ask for full access, which a
			// settings token may not grant.
			body = `{"name":"matrix","scopes":["settings:read"]}`
		default:
			body = "{}"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Token", raw)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	instanceRoutes := []struct{ method, path string }{
		{http.MethodPost, "/api/system/settings/update"},
		{http.MethodGet, "/api/system/settings"},
		{http.MethodPost, "/api/security/sso/providers"},
		{http.MethodGet, "/api/security/sso/providers"},
		{http.MethodGet, "/api/security/tokens"},
		{http.MethodPost, "/api/security/tokens"},
		{http.MethodPost, "/api/security/reset-lockout"},
		{http.MethodGet, "/api/logs/level"},
		{http.MethodPost, "/api/logs/level"},
		{http.MethodGet, "/api/updates/status"},
		{http.MethodGet, "/api/settings/relay"},
	}
	for _, route := range instanceRoutes {
		t.Run("instance "+route.method+" "+route.path, func(t *testing.T) {
			if rec := serve(route.method, route.path, orgRaw); rec.Code != http.StatusForbidden {
				t.Fatalf("org-bound settings token answered %d, want 403: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			if rec := serve(route.method, route.path, instanceRaw); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
				t.Fatalf("instance-wide settings token answered %d: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		})
	}

	orgRoutes := []struct{ method, path string }{
		// The handler answers 405 to anything but POST, so reaching it proves
		// ensureOrgSettingsWriteScope admitted the token without minting one.
		{http.MethodGet, "/api/security/tokens/relay-mobile"},
		{http.MethodGet, "/api/agents/diagnostics"},
	}
	for _, route := range orgRoutes {
		t.Run("org "+route.method+" "+route.path, func(t *testing.T) {
			if rec := serve(route.method, route.path, orgRaw); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
				t.Fatalf("org-bound settings token answered %d on an org route: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
			}
			if rec := serve(route.method, route.path, orgReadOnlyRaw); rec.Code != http.StatusForbidden {
				t.Fatalf("org-bound token without settings scope answered %d, want 403", rec.Code)
			}
		})
	}
}
