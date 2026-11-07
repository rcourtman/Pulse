package api

import (
	"io"
	"net/http"
	"testing"
	"time"
)

func revokeMetricHistoryOrgMembership(t *testing.T, f *metricAuthorityFixture, user string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, f.server.URL+"/api/orgs/default/members/"+user, nil)
	if err != nil {
		t.Fatal(err)
	}
	const adminRaw = "synthetic-organization-history-admin"
	GetSessionStore().CreateSession(adminRaw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: adminRaw})
	req.Header.Set("X-CSRF-Token", generateCSRFToken(adminRaw))
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusNoContent {
		t.Fatalf("membership DELETE = %d %s %v", response.StatusCode, body, err)
	}
	persisted, err := f.router.multiTenant.LoadOrganizationStrict("default")
	if err != nil || persisted.HasMemberUserID(user) {
		t.Fatal("membership withdrawal not persisted")
	}
}

func TestMetricHistoryWithdrawnOrganizationCannotDisclose(t *testing.T) {
	for _, entry := range []string{"chart", "agent"} {
		for _, transport := range []string{"session", "proxy"} {
			t.Run(entry+"/"+transport, func(t *testing.T) {
				installTestRBACManager(t)
				enableSharedMultiOrgForTest(t)
				t.Setenv("PULSE_DEV", "true")
				f := newMetricAuthorityFixture(t, entry, nil)
				// With no role header configured proxy authentication deliberately
				// grants administration. This fixture instead needs a viewer whose
				// membership is the only default-organization entitlement.
				f.router.config.ProxyAuthRoleHeader = "X-Remote-Role"
				f.router.config.ProxyAuthAdminRole = "admin"
				const raw = "synthetic-org-history-session"
				const user = "organization-history-viewer"
				saveDefaultOrgMembers(t, f.router.multiTenant, user)
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", user)
				control := f.begin(t, metricAuthorityControl, "header")
				req := f.request(t, raw, transport)
				if transport == "proxy" {
					req.Header.Set("X-Remote-User", user)
					req.Header.Set("X-Remote-Role", "viewer")
				}
				primary := make(chan historyAuthorityResult, 1)
				go func() { response, err := f.server.Client().Do(req); primary <- historyAuthorityResult{response, err} }()
				select {
				case <-f.provider.entered:
				case got := <-primary:
					if got.response != nil {
						defer got.response.Body.Close()
					}
					t.Fatalf("history returned before its provider gate: %v", got)
				case <-time.After(5 * time.Second):
					t.Fatal("history never reached its provider gate")
				}
				// Authentication normally slides expiry. Snapshot after admission,
				// so only the withdrawal/result step is under comparison.
				before := GetSessionStore().GetSession(raw)
				revokeMetricHistoryOrgMembership(t, f, user)
				// The pending result was already admitted. A new request must also be
				// denied by TenantMiddleware, not be silently retargeted elsewhere.
				fresh := req.Clone(req.Context())
				fresh.Header.Set("X-Pulse-Org-ID", "default")
				response, err := f.server.Client().Do(fresh)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusForbidden {
					t.Fatalf("new withdrawn read = %d", response.StatusCode)
				}
				f.provider.unblock()
				readMetricAuthorityResult(t, primary, http.StatusForbidden)
				readMetricAuthorityResult(t, control, http.StatusOK)
				session := GetSessionStore().GetSession(raw)
				if session == nil || !session.ExpiresAt.Equal(before.ExpiresAt) || !session.CreatedAt.Equal(before.CreatedAt) || session.Username != before.Username {
					t.Fatal("organization removal altered saved session authority")
				}
				// A deliberate regrant admits a fresh read, not the already-denied
				// result. Neither history data nor credentials are recreated/reset.
				saveDefaultOrgMembers(t, f.router.multiTenant, user)
				admitted := make(chan historyAuthorityResult, 1)
				go func() { response, err := f.server.Client().Do(req); admitted <- historyAuthorityResult{response, err} }()
				readMetricAuthorityResult(t, admitted, http.StatusOK)
			})
		}
	}
}
