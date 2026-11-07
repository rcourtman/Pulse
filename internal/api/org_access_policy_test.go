package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const orgAccessTestAdmin = "admin"

func enableSharedMultiOrgForTest(t *testing.T) {
	t.Helper()
	previous := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(previous) })
}

// newOrgAccessFixture builds persistence with one tenant, "acme": alice owns
// it, bob is a member, and carol only holds a pending invitation.
func newOrgAccessFixture(t *testing.T, hosted bool) (*config.MultiTenantPersistence, *DefaultAuthorizationChecker) {
	t.Helper()
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	acme := &models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner},
			{UserID: "bob", Role: models.OrgRoleViewer},
		},
		PendingInvitations: []models.OrganizationInvitation{
			{UserID: "carol", Role: models.OrgRoleViewer},
		},
	}
	if err := persistence.SaveOrganization(acme); err != nil {
		t.Fatalf("save acme: %v", err)
	}
	return persistence, newOrgAccessChecker(&config.Config{AuthUser: orgAccessTestAdmin}, persistence, hosted)
}

func saveDefaultOrgMembers(t *testing.T, persistence *config.MultiTenantPersistence, userIDs ...string) {
	t.Helper()
	current, err := persistence.LoadOrganization("default")
	if err != nil {
		t.Fatalf("load default org: %v", err)
	}
	org := &models.Organization{ID: "default", DisplayName: "default", PersistenceVersion: current.PersistenceVersion}
	for _, userID := range userIDs {
		org.Members = append(org.Members, models.OrganizationMember{UserID: userID, Role: models.OrgRoleViewer})
	}
	if err := persistence.SaveOrganization(org); err != nil {
		t.Fatalf("save default org: %v", err)
	}
}

func TestOrgAccessPolicy_DefaultOrgNeedsAdminOrMembershipInSharedMultiOrgMode(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)

	for _, user := range []string{"bob", "carol", "nobody"} {
		res := checker.CheckAccess(nil, user, "default")
		if res.Allowed {
			t.Fatalf("%s reached the default organization without membership", user)
		}
		if res.Reason != "User is not a member of the default organization" {
			t.Fatalf("unexpected reason for %s: %q", user, res.Reason)
		}
		if checker.UserCanAccessOrg(user, "default") || checker.CanAccessOrg(user, nil, "default") {
			t.Fatalf("%s reached the default organization through another entry point", user)
		}
	}
	if !checker.CheckAccess(nil, orgAccessTestAdmin, "default").Allowed {
		t.Fatal("configured instance administrator must reach the default organization")
	}
	if !checker.CheckAccess(nil, "bob", "acme").Allowed {
		t.Fatal("tenant membership must still grant the tenant")
	}

	saveDefaultOrgMembers(t, persistence, "dave")
	if !checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("explicit default-organization member must reach it")
	}
	if checker.CheckAccess(nil, "bob", "default").Allowed {
		t.Fatal("another user's default membership must not grant bob")
	}

	// Token rules are unchanged: unbound tokens keep default access and
	// org-bound tokens need an explicit default binding.
	if !checker.CheckAccess(&config.APITokenRecord{}, "", "default").Allowed {
		t.Fatal("legacy unbound token must keep default-organization access")
	}
	if checker.CheckAccess(&config.APITokenRecord{OrgID: "acme"}, "", "default").Allowed {
		t.Fatal("acme-bound token must not reach the default organization")
	}
}

func TestOrgAccessPolicy_DefaultOrgStaysOpenOutsideSharedMultiOrgMode(t *testing.T) {
	installTestRBACManager(t)

	t.Run("single tenant", func(t *testing.T) {
		previous := IsMultiTenantEnabled()
		SetMultiTenantEnabled(false)
		t.Cleanup(func() { SetMultiTenantEnabled(previous) })
		_, checker := newOrgAccessFixture(t, false)
		if !checker.CheckAccess(nil, "bob", "default").Allowed {
			t.Fatal("single-tenant default organization must stay open to signed-in users")
		}
	})

	t.Run("hosted", func(t *testing.T) {
		enableSharedMultiOrgForTest(t)
		_, checker := newOrgAccessFixture(t, true)
		if !checker.CheckAccess(nil, "bob", "default").Allowed {
			t.Fatal("hosted runtimes do not share the default organization with other tenants")
		}
	})
}

func TestOrgAccessPolicy_DefaultOrgMetadataIsReadStrictly(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)

	// Missing org.json is no membership, not a synthesized open organization.
	if checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("missing default metadata must not grant access")
	}

	saveDefaultOrgMembers(t, persistence, "dave")
	orgFile := filepath.Join(persistence.BaseDataDir(), "org.json")
	if err := os.WriteFile(orgFile, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt default org metadata: %v", err)
	}
	if checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("unreadable default metadata must deny members")
	}
	if !checker.CheckAccess(nil, orgAccessTestAdmin, "default").Allowed {
		t.Fatal("instance administrators do not depend on default metadata")
	}
}

func TestOrgAccessPolicy_ProxyAdminAuthorityIsBoundToItsPrincipal(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	_, checker := newOrgAccessFixture(t, false)

	proxyCtx := func(username string, isAdmin bool) context.Context {
		req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
		return attachProxyPrincipal(req, username, isAdmin).Context()
	}

	if !checker.CheckAccessContext(proxyCtx("bob", true), nil, "bob", "default").Allowed {
		t.Fatal("proxy administrator must reach the default organization")
	}
	if checker.CheckAccessContext(proxyCtx("bob", false), nil, "bob", "default").Allowed {
		t.Fatal("proxy non-administrator must not reach the default organization")
	}
	if checker.CheckAccessContext(proxyCtx("mallory", true), nil, "bob", "default").Allowed {
		t.Fatal("proxy admin authority must not transfer to another username")
	}
	// A proxy user named like the local administrator is judged by the
	// proxy's role, not by the local administrator's name.
	if checker.CheckAccessContext(proxyCtx(orgAccessTestAdmin, false), nil, orgAccessTestAdmin, "default").Allowed {
		t.Fatal("proxy non-administrator must not inherit the local administrator's authority")
	}
	// Without a request context the proxy role is unknown, so it never grants.
	if checker.CheckAccess(nil, "bob", "default").Allowed {
		t.Fatal("proxy admin authority must not apply without its request context")
	}
}

func TestOrgAccessPolicy_LandingOrgForUsersWithoutDefaultAccess(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)
	saveDefaultOrgMembers(t, persistence, "dave")
	if err := persistence.SaveOrganization(&models.Organization{
		ID: "zeta", DisplayName: "Zeta", OwnerUserID: "bob",
		Members: []models.OrganizationMember{{UserID: "bob", Role: models.OrgRoleOwner}},
	}); err != nil {
		t.Fatalf("save zeta: %v", err)
	}

	ctx := context.Background()
	if got := checker.LandingOrgID(ctx, "bob"); got != "acme" {
		t.Fatalf("bob landing org = %q, want first membership acme", got)
	}
	for _, user := range []string{"carol", "nobody", orgAccessTestAdmin, "dave"} {
		if got := checker.LandingOrgID(ctx, user); got != "" {
			t.Fatalf("%s landing org = %q, want none", user, got)
		}
	}
}

func TestMigrateDefaultOrgAccessSeedsExistingDefaultUsersOnce(t *testing.T) {
	manager := installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)
	cfg := &config.Config{AuthUser: orgAccessTestAdmin}

	assign := func(user string, roleIDs ...string) {
		t.Helper()
		if err := manager.UpdateUserRoles(user, roleIDs); err != nil {
			t.Fatalf("assign %s: %v", user, err)
		}
	}
	assign("dave", internalauth.RoleViewer)      // default-only user: seeded
	assign("bob", internalauth.RoleViewer)       // acme member: not seeded
	assign("carol", internalauth.RoleViewer)     // acme invitee: not seeded
	assign("rbac-admin", internalauth.RoleAdmin) // reaches default by authority
	assign("token:abc", internalauth.RoleViewer) // never a person
	assign("empty")                              // no roles

	if err := migrateDefaultOrgAccess(persistence, manager, cfg, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	defaultOrg, err := persistence.LoadOrganizationStrict("default")
	if err != nil {
		t.Fatalf("load default org: %v", err)
	}
	if len(defaultOrg.Members) != 1 || defaultOrg.Members[0].UserID != "dave" ||
		defaultOrg.Members[0].Role != models.OrgRoleViewer ||
		defaultOrg.Members[0].AddedBy != defaultOrgAccessMigrationActor {
		t.Fatalf("unexpected seeded members: %+v", defaultOrg.Members)
	}
	if !checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("seeded user must keep default-organization access")
	}

	raw, err := os.ReadFile(filepath.Join(persistence.BaseDataDir(), defaultOrgAccessMigrationFile))
	if err != nil {
		t.Fatalf("read migration marker: %v", err)
	}
	var record defaultOrgAccessMigrationRecord
	if err := json.Unmarshal(raw, &record); err != nil || len(record.Seeded) != 1 || record.Seeded[0] != "dave" {
		t.Fatalf("unexpected migration record %s (%v)", raw, err)
	}

	// The marker stops a second run from re-granting a revoked user.
	saveDefaultOrgMembers(t, persistence)
	if err := migrateDefaultOrgAccess(persistence, manager, cfg, false); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("migration must not run twice")
	}
}

func TestMigrateDefaultOrgAccessSkipsOutsideSharedMultiOrgMode(t *testing.T) {
	manager := installTestRBACManager(t)
	previous := IsMultiTenantEnabled()
	SetMultiTenantEnabled(false)
	t.Cleanup(func() { SetMultiTenantEnabled(previous) })
	persistence := config.NewMultiTenantPersistence(t.TempDir())

	if err := migrateDefaultOrgAccess(persistence, manager, &config.Config{}, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(persistence.BaseDataDir(), defaultOrgAccessMigrationFile)); !os.IsNotExist(err) {
		t.Fatalf("single-tenant start must not record the migration: %v", err)
	}
}

func TestMigrateDefaultOrgAccessRetriesWhenTenantMetadataIsUnreadable(t *testing.T) {
	manager := installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, _ := newOrgAccessFixture(t, false)
	if err := os.WriteFile(filepath.Join(persistence.BaseDataDir(), "orgs", "acme", "org.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("corrupt acme metadata: %v", err)
	}
	if err := migrateDefaultOrgAccess(persistence, manager, &config.Config{}, false); err == nil {
		t.Fatal("migration must fail on unreadable tenant metadata")
	}
	// A tenant directory without metadata hides its members just the same.
	if err := os.Remove(filepath.Join(persistence.BaseDataDir(), "orgs", "acme", "org.json")); err != nil {
		t.Fatalf("remove acme metadata: %v", err)
	}
	if err := migrateDefaultOrgAccess(persistence, manager, &config.Config{}, false); err == nil {
		t.Fatal("migration must fail when a tenant's members are unknown")
	}
	if _, err := os.Stat(filepath.Join(persistence.BaseDataDir(), defaultOrgAccessMigrationFile)); !os.IsNotExist(err) {
		t.Fatalf("failed migration must not record completion: %v", err)
	}
}

func orgAccessTestRequest(t *testing.T, method, path, user string, configure func(*http.Request)) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if user != "" {
		req = req.WithContext(internalauth.WithUser(req.Context(), user))
	}
	if configure != nil {
		configure(req)
	}
	return req
}

func TestTenantMiddleware_SharedMultiOrgDefaultAccess(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	t.Setenv("PULSE_DEV", "true")
	persistence, checker := newOrgAccessFixture(t, false)
	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{Persistence: persistence, AuthChecker: checker})

	serve := func(req *http.Request) (int, string) {
		t.Helper()
		servedOrg := ""
		handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			servedOrg = GetOrgID(r.Context())
			if resolved := resolveTenantOrgID(r); resolved != servedOrg {
				t.Fatalf("handler org %q disagrees with the authorized org %q", resolved, servedOrg)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, servedOrg
	}

	// The reported proof of concept: an acme-only user selecting default.
	code, _ := serve(orgAccessTestRequest(t, http.MethodGet, "/api/state", "bob", func(r *http.Request) {
		r.Header.Set("X-Pulse-Org-ID", "default")
	}))
	if code != http.StatusForbidden {
		t.Fatalf("explicit default header: status %d, want 403", code)
	}
	code, _ = serve(orgAccessTestRequest(t, http.MethodGet, "/api/state", "bob", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: CookieNameOrgID, Value: "default"})
	}))
	if code != http.StatusForbidden {
		t.Fatalf("default org cookie: status %d, want 403", code)
	}

	// Without a selection, bob lands in his own organization.
	for _, path := range []string{"/", "/api/security/status", "/api/orgs", "/api/state"} {
		code, org := serve(orgAccessTestRequest(t, http.MethodGet, path, "bob", nil))
		if code != http.StatusNoContent || org != "acme" {
			t.Fatalf("bob %s: status %d org %q, want acme", path, code, org)
		}
	}

	// A user with no organization keeps only the organization-neutral routes.
	for _, path := range []string{"/", "/settings", "/api/health", "/api/security/status", "/api/logout", "/api/orgs", "/api/org-invitations"} {
		code, org := serve(orgAccessTestRequest(t, http.MethodGet, path, "carol", nil))
		if code != http.StatusNoContent || org != "default" {
			t.Fatalf("carol %s: status %d org %q, want neutral admission", path, code, org)
		}
	}
	for _, path := range []string{"/api/state", "/api/resources", "/ws", "/simple-stats"} {
		if code, _ := serve(orgAccessTestRequest(t, http.MethodGet, path, "carol", nil)); code != http.StatusForbidden {
			t.Fatalf("carol %s: status %d, want 403", path, code)
		}
	}
	if code, _ := serve(orgAccessTestRequest(t, http.MethodPost, "/api/orgs", "carol", nil)); code != http.StatusForbidden {
		t.Fatalf("carol POST /api/orgs: status %d, want 403", code)
	}
	// A stale default selection still loads the shell and the bootstrap
	// routes, and data routes say why they refused so the client can recover.
	for _, path := range []string{"/", "/api/security/status", "/api/orgs"} {
		code, org := serve(orgAccessTestRequest(t, http.MethodGet, path, "bob", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: CookieNameOrgID, Value: "default"})
		}))
		if code != http.StatusNoContent || org != "default" {
			t.Fatalf("bob %s with a default cookie: status %d org %q", path, code, org)
		}
	}
	rec := httptest.NewRecorder()
	mw.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("data route must not be served from a refused default selection")
	})).ServeHTTP(rec, orgAccessTestRequest(t, http.MethodGet, "/api/state/summary", "bob", func(r *http.Request) {
		r.Header.Set("X-Pulse-Org-ID", "default")
	}))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"default_org_access_denied"`) {
		t.Fatalf("refused default selection: %d %s", rec.Code, rec.Body.String())
	}

	// Instance administrators keep the default organization.
	code, org := serve(orgAccessTestRequest(t, http.MethodGet, "/api/state", orgAccessTestAdmin, nil))
	if code != http.StatusNoContent || org != "default" {
		t.Fatalf("admin: status %d org %q, want default", code, org)
	}
}

func TestOrgAccessRevalidatorEndsStreamsWhenAccessIsRevoked(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)
	saveDefaultOrgMembers(t, persistence, "dave")

	previousInterval := orgAccessRecheckInterval
	orgAccessRecheckInterval = 0
	t.Cleanup(func() { orgAccessRecheckInterval = previousInterval })

	req := orgAccessTestRequest(t, http.MethodGet, "/ws", "dave", nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "default")
	req = req.WithContext(withOrgAccessChecker(ctx, checker))

	allowed := orgAccessRevalidator(req)
	if allowed == nil || !allowed() {
		t.Fatal("member stream must start allowed")
	}
	saveDefaultOrgMembers(t, persistence)
	if allowed() {
		t.Fatal("stream must end once default membership is revoked")
	}
	saveDefaultOrgMembers(t, persistence, "dave")
	if allowed() {
		t.Fatal("an ended stream must stay ended")
	}

	// Token streams keep their own credential revalidation.
	tokenReq := req.WithContext(internalauth.WithAPIToken(req.Context(), &config.APITokenRecord{ID: "t"}))
	if orgAccessRevalidator(tokenReq) != nil {
		t.Fatal("token streams must not get the user revalidator")
	}
}

func TestOrgHandlers_DefaultOrgMembersInSharedMultiOrgMode(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	t.Setenv("PULSE_DEV", "true")
	persistence, checker := newOrgAccessFixture(t, false)
	h := NewOrgHandlers(persistence, nil)
	h.SetAccessPolicy(checker)

	call := func(handler http.HandlerFunc, method, path, user, body string, pathValues map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(internalauth.WithUser(req.Context(), user))
		for key, value := range pathValues {
			req.SetPathValue(key, value)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	listOrgIDs := func(user string) []string {
		t.Helper()
		rec := call(h.HandleListOrgs, http.MethodGet, "/api/orgs", user, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("list orgs for %s: %d %s", user, rec.Code, rec.Body.String())
		}
		var orgs []models.Organization
		if err := json.Unmarshal(rec.Body.Bytes(), &orgs); err != nil {
			t.Fatalf("decode orgs: %v", err)
		}
		ids := make([]string, 0, len(orgs))
		for _, org := range orgs {
			ids = append(ids, org.ID)
		}
		return ids
	}

	if got := strings.Join(listOrgIDs("bob"), ","); got != "acme" {
		t.Fatalf("bob sees %q, want only acme", got)
	}
	if got := strings.Join(listOrgIDs(orgAccessTestAdmin), ","); got != "default" {
		t.Fatalf("admin sees %q, want default", got)
	}
	defaultPath := map[string]string{"id": "default"}
	if rec := call(h.HandleGetOrg, http.MethodGet, "/api/orgs/default", "bob", "", defaultPath); rec.Code != http.StatusForbidden {
		t.Fatalf("bob GET default: %d, want 403", rec.Code)
	}
	rec := call(h.HandleGetOrg, http.MethodGet, "/api/orgs/default", orgAccessTestAdmin, "", defaultPath)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"canManageMembers":true`) {
		t.Fatalf("admin GET default: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.HandleListInvitations, http.MethodGet, "/api/orgs/default/invitations", orgAccessTestAdmin, "", defaultPath)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("admin listing default invitations: %d %q, want an empty list", rec.Code, rec.Body.String())
	}

	// Only instance administrators manage default members, and only as viewers.
	if rec := call(h.HandleInviteMember, http.MethodPost, "/api/orgs/default/members", "alice", `{"userId":"bob"}`, defaultPath); rec.Code != http.StatusForbidden {
		t.Fatalf("tenant owner adding a default member: %d, want 403", rec.Code)
	}
	if rec := call(h.HandleInviteMember, http.MethodPost, "/api/orgs/default/members", orgAccessTestAdmin, `{"userId":"bob","role":"admin"}`, defaultPath); rec.Code != http.StatusBadRequest {
		t.Fatalf("admin role on default: %d, want 400", rec.Code)
	}
	rec = call(h.HandleInviteMember, http.MethodPost, "/api/orgs/default/members", orgAccessTestAdmin, `{"userId":"bob"}`, defaultPath)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"kind":"member"`) {
		t.Fatalf("admin adding bob: %d %s", rec.Code, rec.Body.String())
	}
	if !checker.CheckAccess(nil, "bob", "default").Allowed {
		t.Fatal("added member must reach the default organization")
	}
	if got := strings.Join(listOrgIDs("bob"), ","); got != "acme,default" {
		t.Fatalf("bob sees %q after the grant", got)
	}
	rec = call(h.HandleGetOrg, http.MethodGet, "/api/orgs/default", "bob", "", defaultPath)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "canManageMembers") {
		t.Fatalf("member GET default: %d %s", rec.Code, rec.Body.String())
	}

	removePath := map[string]string{"id": "default", "userId": "bob"}
	if rec := call(h.HandleRemoveMember, http.MethodDelete, "/api/orgs/default/members/bob", "bob", "", removePath); rec.Code != http.StatusForbidden {
		t.Fatalf("member removing themself: %d, want 403", rec.Code)
	}
	if rec := call(h.HandleRemoveMember, http.MethodDelete, "/api/orgs/default/members/bob", orgAccessTestAdmin, "", removePath); rec.Code != http.StatusNoContent {
		t.Fatalf("admin removing bob: %d %s", rec.Code, rec.Body.String())
	}
	if checker.CheckAccess(nil, "bob", "default").Allowed {
		t.Fatal("removed member must lose the default organization")
	}
}

func TestOrgHandlers_DefaultOrgOwnerIsRevocableAndStaleSavesConflict(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	t.Setenv("PULSE_DEV", "true")
	persistence, checker := newOrgAccessFixture(t, false)
	h := NewOrgHandlers(persistence, nil)
	h.SetAccessPolicy(checker)

	// An owner field alone grants nothing; membership does.
	if err := persistence.SaveOrganization(&models.Organization{ID: "default", DisplayName: "default", OwnerUserID: "olivia"}); err != nil {
		t.Fatalf("save default org: %v", err)
	}
	if checker.CheckAccess(nil, "olivia", "default").Allowed {
		t.Fatal("default ownership without membership must not grant access")
	}

	// Removing a member who also owns the default organization revokes them.
	saveDefaultOrgMembers(t, persistence, "olivia", "dave")
	org, err := persistence.LoadOrganizationStrict("default")
	if err != nil {
		t.Fatalf("load default org: %v", err)
	}
	org.OwnerUserID = "olivia"
	if err := persistence.SaveOrganization(org); err != nil {
		t.Fatalf("save default owner: %v", err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/default/members/olivia", nil)
	req = req.WithContext(internalauth.WithUser(req.Context(), orgAccessTestAdmin))
	req.SetPathValue("id", "default")
	req.SetPathValue("userId", "olivia")
	rec := httptest.NewRecorder()
	h.HandleRemoveMember(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove owner member: %d %s", rec.Code, rec.Body.String())
	}
	if checker.CheckAccess(nil, "olivia", "default").Allowed {
		t.Fatal("removed default owner kept access")
	}

	// A save from a snapshot taken before dave's removal must not restore him.
	stale, err := persistence.LoadOrganizationStrict("default")
	if err != nil {
		t.Fatalf("load stale snapshot: %v", err)
	}
	saveDefaultOrgMembers(t, persistence)
	stale.SharedResources = append(stale.SharedResources, models.OrganizationShare{ID: "share-1", TargetOrgID: "acme", ResourceType: "vm", ResourceID: "vm-1"})
	if err := h.saveOrganization(stale); !errors.Is(err, config.ErrOrganizationChanged) {
		t.Fatalf("save stale snapshot = %v, want conflict", err)
	}
	if checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("a stale default snapshot restored a removed member")
	}
	saved, err := persistence.LoadOrganizationStrict("default")
	if err != nil || len(saved.SharedResources) != 0 {
		t.Fatalf("conflicting share change was published: %+v (%v)", saved, err)
	}
	// A fresh intentional share edit still succeeds, without restoring dave.
	saved.SharedResources = stale.SharedResources
	if err := h.saveOrganization(saved); err != nil {
		t.Fatalf("save reviewed share change: %v", err)
	}
	if checker.CheckAccess(nil, "dave", "default").Allowed {
		t.Fatal("fresh share edit restored a removed member")
	}
}
