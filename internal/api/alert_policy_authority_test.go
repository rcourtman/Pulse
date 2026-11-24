package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Token scopes do not bind sessions, so the requests that change alert policy
// or Patrol's standing suppression need a manager as well. A proxy user outside
// the admin role keeps the reads and the per-alert and per-finding actions.
func TestAlertAndPatrolPolicyRoutesRefuseProxyNonAdmin(t *testing.T) {
	router := newProxyAuthRouter(t)

	request := func(method, path, body, roles string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Proxy-User", "someone")
		req.Header.Set("X-Proxy-Roles", roles)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	for _, tc := range []struct {
		method, path, body string
		policy             bool
	}{
		{http.MethodPut, "/api/alerts/config", `{"enabled":false}`, true},
		{http.MethodPut, "/api/alerts/deadman/config", `{"pingUrl":""}`, true},
		{http.MethodPut, "/api/alerts/intent-policies", `{}`, true},
		{http.MethodPost, "/api/alerts/activate", `{}`, true},
		{http.MethodDelete, "/api/alerts/history", ``, true},
		{http.MethodPost, "/api/ai/patrol/suppressions", `{"description":"quiet","allow_broad_scope":true}`, true},
		{http.MethodDelete, "/api/ai/patrol/suppressions/rule_any__1", ``, true},
		{http.MethodPost, "/api/ai/patrol/suppress", `{"finding_id":"f-1"}`, true},
		{http.MethodDelete, "/api/ai/patrol/findings?confirm=true", ``, true},

		{http.MethodGet, "/api/alerts/config", ``, false},
		{http.MethodGet, "/api/alerts/deadman/config", ``, false},
		{http.MethodGet, "/api/alerts/intent-policies", ``, false},
		{http.MethodGet, "/api/alerts/history", ``, false},
		{http.MethodPost, "/api/alerts/acknowledge", `{}`, false},
		{http.MethodPost, "/api/alerts/snooze", `{}`, false},
		{http.MethodPost, "/api/alerts/clear", `{}`, false},
		{http.MethodPost, "/api/alerts/incidents/note", `{}`, false},
		{http.MethodGet, "/api/ai/patrol/suppressions", ``, false},
		{http.MethodDelete, "/api/ai/patrol/suppressions/finding_f-1", ``, false},
	} {
		viewer := request(tc.method, tc.path, tc.body, "viewer|user")
		if refused := viewer.Code == http.StatusForbidden; refused != tc.policy {
			t.Errorf("non-admin %s %s = %d, refused = %v, want %v: %s", tc.method, tc.path, viewer.Code, refused, tc.policy, viewer.Body.String())
		}
		if admin := request(tc.method, tc.path, tc.body, "viewer|admin"); admin.Code == http.StatusForbidden || admin.Code == http.StatusUnauthorized {
			t.Errorf("admin %s %s = %d, want the handler to answer: %s", tc.method, tc.path, admin.Code, admin.Body.String())
		}
	}
}

// Alert policy is the request organization's own, so its managers change it in
// a session and its viewers do not; a token bound to it is still judged by the
// scope alone.
func TestAlertPolicyRoutesAdmitRequestOrganizationManagers(t *testing.T) {
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(false) })
	t.Setenv("PULSE_DEV", "true")

	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	const writeToken = "alert-policy-write.12345678"
	writer := newTokenRecord(t, writeToken, []string{config.ScopeMonitoringWrite}, nil)
	writer.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, writer)
	cfg.AuthUser, cfg.AuthPass = "platform-admin", hashed
	mtp := config.NewMultiTenantPersistence(cfg.DataPath)
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "org-owner",
		Members: []models.OrganizationMember{
			{UserID: "org-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "org-viewer", Role: models.OrgRoleViewer, AddedAt: time.Now()},
		},
	}); err != nil {
		t.Fatalf("save organization: %v", err)
	}
	monitors := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(monitors.Stop)
	router := newTestRouter(t, cfg, nil, monitors, nil, nil, "1.0.0")

	saveConfig := func(user, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/alerts/config", strings.NewReader(`{"enabled":true}`))
		req.Header.Set("X-Pulse-Org-ID", "org-a")
		if token != "" {
			req.Header.Set("X-API-Token", token)
		} else {
			session := "alert-policy-" + user + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
			GetSessionStore().CreateSession(session, time.Hour, "agent", "127.0.0.1", user)
			req.Header.Set("X-CSRF-Token", generateCSRFToken(session))
			req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
		}
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	for _, tc := range []struct {
		user, token string
		status      int
	}{
		{"org-owner", "", http.StatusOK},
		{"org-viewer", "", http.StatusForbidden},
		{"", writeToken, http.StatusOK},
	} {
		if rec := saveConfig(tc.user, tc.token); rec.Code != tc.status {
			t.Fatalf("user %q token %q alert config save = %d, want %d: %s", tc.user, tc.token, rec.Code, tc.status, rec.Body.String())
		}
	}
}

type alertOperatorAuthorizer struct{}

func (alertOperatorAuthorizer) Authorize(ctx context.Context, action, resource string) (bool, error) {
	return internalauth.GetUser(ctx) == "operator" && action == internalauth.ActionWrite && resource == internalauth.ResourceAlerts, nil
}

// With a real RBAC authorizer, the grant the built-in operator role carries
// (write on alerts) admits a user who is not an administrator to alert policy.
// It does not reach Patrol's suppression rules, and a viewer stays refused.
func TestAlertPolicyRoutesAdmitRBACAlertWriteGrant(t *testing.T) {
	previous := internalauth.GetAuthorizer()
	internalauth.SetAuthorizer(alertOperatorAuthorizer{})
	t.Cleanup(func() { internalauth.SetAuthorizer(previous) })
	router := newProxyAuthRouter(t)

	for _, tc := range []struct {
		user, method, path, body string
		refused                  bool
	}{
		{"operator", http.MethodPut, "/api/alerts/config", `{"enabled":false}`, false},
		{"operator", http.MethodDelete, "/api/alerts/history", ``, false},
		{"operator", http.MethodPost, "/api/ai/patrol/suppressions", `{"description":"quiet","allow_broad_scope":true}`, true},
		{"viewer", http.MethodPut, "/api/alerts/config", `{"enabled":false}`, true},
		{"viewer", http.MethodDelete, "/api/alerts/history", ``, true},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Proxy-User", tc.user)
		req.Header.Set("X-Proxy-Roles", "user")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if refused := rec.Code == http.StatusForbidden; refused != tc.refused {
			t.Errorf("%s %s %s = %d, refused = %v, want %v: %s", tc.user, tc.method, tc.path, rec.Code, refused, tc.refused, rec.Body.String())
		}
	}
}
