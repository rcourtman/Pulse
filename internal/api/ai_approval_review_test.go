package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type approvalReviewAuthorizer struct {
	grant string
	err   bool
	calls []string
}

func (a *approvalReviewAuthorizer) Authorize(_ context.Context, action, resource string) (bool, error) {
	a.calls = append(a.calls, action+":"+resource)
	if a.err {
		return false, errors.New("fixture-private-policy-read-failure")
	}
	return action+":"+resource == a.grant, nil
}

func approvalReviewFixture(t *testing.T, az auth.Authorizer, records ...config.APITokenRecord) (*Router, *approval.Store) {
	t.Helper()
	oldAuth, oldStore := auth.GetAuthorizer(), approval.GetStore()
	auth.SetAuthorizer(az)
	t.Cleanup(func() { auth.SetAuthorizer(oldAuth); approval.SetStore(oldStore) })
	cfg := newTestConfigWithTokens(t, records...)
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	cfg.AuthUser, cfg.AuthPass = "admin", "fixture-unused-hash"
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "fixture")
	store, err := approval.NewStore(approval.StoreConfig{DataDir: cfg.DataPath, DisablePersistence: true})
	if err != nil {
		t.Fatal(err)
	}
	approval.SetStore(store)
	return router, store
}

func seedApprovalReview(t *testing.T, store *approval.Store, id, org, tool string) *approval.ApprovalRequest {
	t.Helper()
	item := &approval.ApprovalRequest{ID: id, OrgID: org, ToolID: tool, Command: "printf fixture", TargetType: "agent", TargetID: "host-web-01", TargetName: "web-01", Context: "Fixture review of web-01 maintenance", RequestedBy: approval.RequesterPulsePatrol, RiskLevel: approval.RiskMedium, ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.CreateApproval(item); err != nil {
		t.Fatal(err)
	}
	return item
}

func approvalReviewRequest(t *testing.T, router *Router, user, token, method, path, org string, csrf bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{"reason":"fixture decision"}`))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		session := "fixture-review-session-" + user
		GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", user)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
		if csrf {
			req.Header.Set("X-CSRF-Token", generateCSRFToken(session))
		}
	}
	if token != "" {
		req.Header.Set("X-API-Token", token)
	}
	if org != "" {
		req.Header.Set("X-Pulse-Org-ID", org)
	}
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	return rec
}

func TestAIApprovalReviewSessionRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, user, grant string
		az                auth.Authorizer
		allowed           bool
	}{
		{name: "default viewer", user: "viewer", az: &auth.DefaultAuthorizer{}},
		{name: "configured administrator", user: "admin", az: &auth.DefaultAuthorizer{}, allowed: true},
		{name: "explicit review grant", user: "reviewer", grant: auth.ActionApprove + ":" + auth.ResourceActions, allowed: true},
		{name: "legacy AI administration", user: "reviewer", grant: auth.ActionAdmin + ":" + auth.ResourceAI, allowed: true},
		{name: "execute is not review", user: "reviewer", grant: auth.ActionExecute + ":" + auth.ResourceActions},
		{name: "monitoring is not review", user: "reviewer", grant: auth.ActionRead + ":" + auth.ResourceNodes},
		{name: "explicit denial", user: "reviewer"},
		{name: "unreadable policy", user: "reviewer", az: &approvalReviewAuthorizer{err: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			az := tc.az
			if az == nil {
				az = &approvalReviewAuthorizer{grant: tc.grant}
			}
			router, store := approvalReviewFixture(t, az)
			seedApprovalReview(t, store, "fixture-own", "default", "shell")
			seedApprovalReview(t, store, "fixture-approve", "default", "shell")
			seedApprovalReview(t, store, "fixture-deny", "default", "shell")
			seedApprovalReview(t, store, "fixture-investigation", "default", "investigation_fix")
			seedApprovalReview(t, store, "fixture-other", "org-other", "shell")
			for _, route := range []struct {
				name, method, path, id string
				want                   int
			}{
				{"list", http.MethodGet, "/api/ai/approvals", "", http.StatusOK},
				{"detail", http.MethodGet, "/api/ai/approvals/fixture-own", "fixture-own", http.StatusOK},
				{"approve", http.MethodPost, "/api/ai/approvals/fixture-approve/approve", "fixture-approve", http.StatusOK},
				{"deny", http.MethodPost, "/api/ai/approvals/fixture-deny/deny", "fixture-deny", http.StatusOK},
				{"investigation dispatch", http.MethodPost, "/api/ai/approvals/fixture-investigation/approve", "fixture-investigation", http.StatusPaymentRequired},
			} {
				t.Run(route.name, func(t *testing.T) {
					before := store.GetPendingApprovalsForOrg("default")
					snapshot, _ := json.Marshal(before)
					rec := approvalReviewRequest(t, router, tc.user, "", route.method, route.path, "", true)
					want := http.StatusForbidden
					if tc.allowed {
						want = route.want
					}
					if rec.Code != want {
						t.Fatalf("status=%d want=%d: %s", rec.Code, want, rec.Body.String())
					}
					if !tc.allowed {
						for _, private := range []string{"fixture-own", "printf fixture", "web-01", "fixture-private-policy-read-failure"} {
							if strings.Contains(rec.Body.String(), private) {
								t.Fatalf("denial disclosed %q: %s", private, rec.Body.String())
							}
						}
						after, _ := json.Marshal(store.GetPendingApprovalsForOrg("default"))
						if string(after) != string(snapshot) {
							t.Fatal("denied route changed saved approvals")
						}
					} else {
						if strings.Contains(rec.Body.String(), "fixture-other") {
							t.Fatal("cross-org record escaped")
						}
						if route.name == "list" && !strings.Contains(rec.Body.String(), "fixture-own") {
							t.Fatal("authorized list lost pending item")
						}
						if route.id != "" {
							item, _ := store.GetApproval(route.id)
							wantStatus := approval.StatusPending
							if route.name == "approve" {
								wantStatus = approval.StatusApproved
							}
							if route.name == "deny" {
								wantStatus = approval.StatusDenied
							}
							if item.Status != wantStatus || item.Consumed {
								t.Fatalf("decision/dispatch changed unexpectedly: %+v", item)
							}
							if wantStatus != approval.StatusPending && item.DecidedBy != tc.user {
								t.Fatalf("decision attribution=%q want %q", item.DecidedBy, tc.user)
							}
						}
					}
				})
			}
		})
	}
}

func TestAIApprovalReviewTokenCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, scope     string
		allowed, detail bool
	}{
		{"monitoring", config.ScopeMonitoringRead, false, false},
		{"chat", config.ScopeAIChat, false, false},
		{"canonical approve only", config.ScopeActionsApprove, false, false},
		{"legacy ai execute", config.ScopeAIExecute, true, true},
		{"paired mobile", config.ScopeRelayMobileAccess, true, false},
		{"legacy wildcard", config.ScopeWildcard, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "fixture-approval-token.12345678"
			token := newTokenRecord(t, raw, []string{tc.scope}, nil)
			// No owner binding: paired legacy clients must keep their existing scope contract.
			az := &approvalReviewAuthorizer{err: true}
			router, store := approvalReviewFixture(t, az, token)
			savedTokens, _ := json.Marshal(router.config.APITokens)
			seedApprovalReview(t, store, "fixture-own", "default", "shell")
			seedApprovalReview(t, store, "fixture-approve", "default", "shell")
			seedApprovalReview(t, store, "fixture-deny", "default", "shell")
			seedApprovalReview(t, store, "fixture-investigation", "default", "investigation_fix")
			for _, route := range []struct {
				method, path, id string
				want             int
				detail           bool
			}{
				{http.MethodGet, "/api/ai/approvals", "", 200, false},
				{http.MethodGet, "/api/ai/approvals/fixture-own", "", 200, true},
				{http.MethodPost, "/api/ai/approvals/fixture-approve/approve", "fixture-approve", 200, false},
				{http.MethodPost, "/api/ai/approvals/fixture-deny/deny", "fixture-deny", 200, false},
				{http.MethodPost, "/api/ai/approvals/fixture-investigation/approve", "fixture-investigation", 402, false},
			} {
				rec := approvalReviewRequest(t, router, "", raw, route.method, route.path, "", false)
				allowed := tc.allowed && (!route.detail || tc.detail)
				want := http.StatusForbidden
				if allowed {
					want = route.want
				}
				if rec.Code != want {
					t.Fatalf("%s %s: status=%d want=%d: %s", route.method, route.path, rec.Code, want, rec.Body.String())
				}
				if !allowed && strings.Contains(rec.Body.String(), "printf fixture") {
					t.Fatal("denied token received command")
				}
				if route.id != "" {
					item, _ := store.GetApproval(route.id)
					expected := approval.StatusPending
					if allowed && strings.HasSuffix(route.path, "/deny") {
						expected = approval.StatusDenied
					}
					if allowed && route.want == 200 && strings.HasSuffix(route.path, "/approve") {
						expected = approval.StatusApproved
					}
					if item.Status != expected || item.Consumed {
						t.Fatalf("unexpected saved state: %+v", item)
					}
				}
			}
			if len(az.calls) != 0 {
				t.Fatalf("legacy token gained a new RBAC dependency: %v", az.calls)
			}
			after, _ := json.Marshal(router.config.APITokens)
			// LastUsed is allowed to advance; scopes, owner, org, expiry and credential stay exact.
			var beforeRecords, afterRecords []map[string]any
			_ = json.Unmarshal(savedTokens, &beforeRecords)
			_ = json.Unmarshal(after, &afterRecords)
			for _, records := range [][]map[string]any{beforeRecords, afterRecords} {
				for _, r := range records {
					delete(r, "lastUsedAt")
				}
			}
			if !reflect.DeepEqual(beforeRecords, afterRecords) {
				t.Fatalf("token authority changed: before=%s after=%s", savedTokens, after)
			}
		})
	}
}

func TestAIApprovalReviewTenantIsolation(t *testing.T) {
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(false) })
	t.Setenv("PULSE_DEV", "true")
	raw := "fixture-tenant-review-token.12345678"
	token := newTokenRecord(t, raw, []string{config.ScopeAIExecute}, nil)
	token.OrgID = "org-a"
	router, store := approvalReviewFixture(t, &approvalReviewAuthorizer{grant: auth.ActionApprove + ":" + auth.ResourceActions}, token)
	persistence := config.NewMultiTenantPersistence(router.config.DataPath)
	for _, org := range []string{"org-a", "org-b"} {
		owner := "someone-else"
		if org == "org-a" {
			owner = "reviewer"
		}
		if err := persistence.SaveOrganization(&models.Organization{ID: org, DisplayName: org, OwnerUserID: owner, Members: []models.OrganizationMember{{UserID: owner, Role: models.OrgRoleOwner}}}); err != nil {
			t.Fatal(err)
		}
	}
	seedApprovalReview(t, store, "fixture-a", "org-a", "shell")
	seedApprovalReview(t, store, "fixture-b", "org-b", "shell")
	for _, caller := range []struct{ name, user, token string }{
		{"session", "reviewer", ""}, {"token", "", raw},
	} {
		t.Run(caller.name, func(t *testing.T) {
			rec := approvalReviewRequest(t, router, caller.user, caller.token, http.MethodGet, "/api/ai/approvals", "org-a", true)
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), "fixture-a") || strings.Contains(rec.Body.String(), "fixture-b") {
				t.Fatalf("own list %d: %s", rec.Code, rec.Body.String())
			}
			for _, route := range []struct{ method, suffix string }{
				{http.MethodGet, ""}, {http.MethodPost, "/approve"}, {http.MethodPost, "/deny"},
			} {
				rec := approvalReviewRequest(t, router, caller.user, caller.token, route.method, "/api/ai/approvals/fixture-b"+route.suffix, "org-a", true)
				if rec.Code != 404 {
					t.Fatalf("cross-org item %d: %s", rec.Code, rec.Body.String())
				}
				rec = approvalReviewRequest(t, router, caller.user, caller.token, route.method, "/api/ai/approvals/fixture-b"+route.suffix, "org-b", true)
				if rec.Code != 403 {
					t.Fatalf("forged organization %d: %s", rec.Code, rec.Body.String())
				}
			}
		})
	}
	for _, id := range []string{"fixture-a", "fixture-b"} {
		item, _ := store.GetApproval(id)
		if item.Status != approval.StatusPending || item.Consumed {
			t.Fatal("cross-org request changed state")
		}
	}
}

func TestAIApprovalReviewSSOAdministrator(t *testing.T) {
	old := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	t.Cleanup(func() { auth.SetAuthorizer(old) })
	router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
	manager := installTestRBACManager(t)
	if err := manager.UpdateUserRoles("sso:review-admin", []string{auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	oldStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(oldStore) })
	store, err := approval.NewStore(approval.StoreConfig{DataDir: t.TempDir(), DisablePersistence: true})
	if err != nil {
		t.Fatal(err)
	}
	approval.SetStore(store)
	seedApprovalReview(t, store, "fixture-own", "default", "shell")
	for _, user := range []string{"sso:viewer", "sso:review-admin"} {
		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/api/ai/approvals"}, {http.MethodGet, "/api/ai/approvals/fixture-own"}, {http.MethodPost, "/api/ai/approvals/fixture-own/deny"},
		} {
			rec := approvalReviewRequest(t, router, user, "", route.method, route.path, "", true)
			want := http.StatusForbidden
			if user == "sso:review-admin" {
				want = http.StatusOK
			}
			if rec.Code != want {
				t.Fatalf("SSO %s %s: %d want %d: %s", user, route.path, rec.Code, want, rec.Body.String())
			}
		}
	}
}

func TestAIApprovalReviewProxyAdministrator(t *testing.T) {
	router, store := approvalReviewFixture(t, &auth.DefaultAuthorizer{})
	router.config.ProxyAuthSecret = "fixture-proxy-secret"
	router.config.ProxyAuthUserHeader = "X-Remote-User"
	router.config.ProxyAuthRoleHeader = "X-Remote-Roles"
	router.config.ProxyAuthAdminRole = "admin"
	seedApprovalReview(t, store, "fixture-own", "default", "shell")
	for _, role := range []string{"viewer", "admin"} {
		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/api/ai/approvals"}, {http.MethodGet, "/api/ai/approvals/fixture-own"}, {http.MethodPost, "/api/ai/approvals/fixture-own/deny"},
		} {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
			req.Header.Set("X-Proxy-Secret", router.config.ProxyAuthSecret)
			req.Header.Set("X-Remote-User", "fixture-proxy-user")
			req.Header.Set("X-Remote-Roles", role)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			want := http.StatusForbidden
			if role == "admin" {
				want = http.StatusOK
			}
			if rec.Code != want {
				t.Fatalf("proxy %s %s: %d want %d: %s", role, route.path, rec.Code, want, rec.Body.String())
			}
		}
	}
}

type approvalReviewInheritedManager struct {
	mockExtendedRBACManager
	fail bool
}

func (m *approvalReviewInheritedManager) GetRolesWithInheritanceWithError(user string) ([]auth.Role, error) {
	roles := m.GetRolesWithInheritance(user)
	if m.fail {
		return roles, errors.New("fixture-private-inherited-policy-failure")
	}
	return roles, nil
}

func TestAIApprovalReviewInheritedPolicyAndWithdrawal(t *testing.T) {
	manager := &approvalReviewInheritedManager{}
	manager.rolesInherited = []auth.Role{{ID: "child", ParentID: "parent", Permissions: []auth.Permission{{Action: auth.ActionApprove, Resource: auth.ResourceActions, Effect: auth.EffectAllow}}}, {ID: "parent", Permissions: []auth.Permission{{Action: auth.ActionApprove, Resource: auth.ResourceActions, Effect: auth.EffectDeny}, {Action: auth.ActionAdmin, Resource: auth.ResourceAI, Effect: auth.EffectDeny}}}}
	router, store := approvalReviewFixture(t, auth.NewRBACAuthorizer(manager))
	seedApprovalReview(t, store, "fixture-own", "default", "shell")
	for _, phase := range []string{"inherited-deny", "unreadable-inheritance", "granted", "withdrawn"} {
		manager.fail = phase == "unreadable-inheritance"
		if phase == "granted" {
			manager.rolesInherited = manager.rolesInherited[:1]
		}
		if phase == "withdrawn" {
			manager.rolesInherited = nil
		}
		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/api/ai/approvals"}, {http.MethodGet, "/api/ai/approvals/fixture-own"}, {http.MethodPost, "/api/ai/approvals/fixture-own/approve"}, {http.MethodPost, "/api/ai/approvals/fixture-own/deny"},
		} {
			// Read grants only in the permitted phase; a decision is withheld until
			// the same live policy has been withdrawn.
			if phase == "granted" && route.method != http.MethodGet {
				continue
			}
			rec := approvalReviewRequest(t, router, "reviewer", "", route.method, route.path, "", true)
			want := http.StatusForbidden
			if phase == "granted" {
				want = 200
			}
			if rec.Code != want {
				t.Fatalf("%s %s: %d want %d: %s", phase, route.path, rec.Code, want, rec.Body.String())
			}
			if phase != "granted" && strings.Contains(rec.Body.String(), "fixture-private") {
				t.Fatal("private policy error escaped")
			}
		}
	}
	item, _ := store.GetApproval("fixture-own")
	if item.Status != approval.StatusPending || item.Consumed {
		t.Fatal("withdrawal changed saved intent")
	}
}
