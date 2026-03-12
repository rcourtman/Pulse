package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestTenantMiddlewareAllowsMemberSession(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	// Dev mode enables license checks for multi-tenant features.
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashed,
	}

	org := &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	mtp := config.NewMultiTenantPersistence(dataDir)
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)

	sessionToken := "member-session-token"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-a")
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for org member, got %d", rec.Code)
	}
}

func TestTenantMiddlewareRejectsNonMemberSession(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	// Dev mode enables license checks for multi-tenant features.
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashed,
	}

	org := &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	mtp := config.NewMultiTenantPersistence(dataDir)
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	sessionToken := "nonmember-session-token"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "bob")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-a")
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
	if msg := payload["message"]; msg == "" || !strings.Contains(msg, "member") {
		t.Fatalf("expected member access denied message, got %q", msg)
	}
}

func TestMultiTenantListOrgsShowsOnlyMemberOrganizations(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashed,
	}

	mtp := config.NewMultiTenantPersistence(dataDir)
	saveOrg := func(org *models.Organization) {
		t.Helper()
		if err := mtp.SaveOrganization(org); err != nil {
			t.Fatalf("save organization %s: %v", org.ID, err)
		}
	}

	saveOrg(&models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	})
	saveOrg(&models.Organization{
		ID:          "org-b",
		DisplayName: "Org B",
		OwnerUserID: "bob",
		Members: []models.OrganizationMember{
			{UserID: "bob", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	})
	saveOrg(&models.Organization{
		ID:          "org-shared",
		DisplayName: "Org Shared",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "bob", Role: models.OrgRoleViewer, AddedAt: time.Now()},
		},
	})

	router := newMultiTenantRouter(t, cfg)

	listVisibleOrgIDs := func(username string) []string {
		t.Helper()

		sessionToken := "session-" + username + "-" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
		GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", username)

		req := httptest.NewRequest(http.MethodGet, "/api/orgs", nil)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s org list, got %d: %s", username, rec.Code, rec.Body.String())
		}

		var orgs []models.Organization
		if err := json.Unmarshal(rec.Body.Bytes(), &orgs); err != nil {
			t.Fatalf("decode org list for %s: %v", username, err)
		}

		ids := make([]string, 0, len(orgs))
		for _, org := range orgs {
			ids = append(ids, org.ID)
		}
		slices.Sort(ids)
		return ids
	}

	if got := listVisibleOrgIDs("alice"); !slices.Equal(got, []string{"default", "org-a", "org-shared"}) {
		t.Fatalf("visible orgs for alice = %v, want %v", got, []string{"default", "org-a", "org-shared"})
	}
	if got := listVisibleOrgIDs("bob"); !slices.Equal(got, []string{"default", "org-b", "org-shared"}) {
		t.Fatalf("visible orgs for bob = %v, want %v", got, []string{"default", "org-b", "org-shared"})
	}
	if got := listVisibleOrgIDs("charlie"); !slices.Equal(got, []string{"default"}) {
		t.Fatalf("visible orgs for charlie = %v, want %v", got, []string{"default"})
	}
}

func TestMultiTenantMemberRoleChangesUpdateOrgManagementAccess(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashed,
	}

	mtp := config.NewMultiTenantPersistence(dataDir)
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "bob", Role: models.OrgRoleViewer, AddedAt: time.Now()},
		},
	}); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)

	sessionTokenFor := func(username string) string {
		t.Helper()
		token := "session-" + username + "-" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
		GetSessionStore().CreateSession(token, time.Hour, "agent", "127.0.0.1", username)
		return token
	}

	doRequest := func(sessionToken, method, path string, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
			req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
		}
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	aliceSession := sessionTokenFor("alice")
	bobSession := sessionTokenFor("bob")

	deniedUpdate := doRequest(
		bobSession,
		http.MethodPut,
		"/api/orgs/acme",
		`{"displayName":"Viewer Should Not Update"}`,
	)
	if deniedUpdate.Code != http.StatusForbidden {
		t.Fatalf("expected viewer update to be denied, got %d: %s", deniedUpdate.Code, deniedUpdate.Body.String())
	}

	promoteBob := doRequest(
		aliceSession,
		http.MethodPost,
		"/api/orgs/acme/members",
		`{"userId":"bob","role":"admin"}`,
	)
	if promoteBob.Code != http.StatusOK {
		t.Fatalf("expected owner to promote bob, got %d: %s", promoteBob.Code, promoteBob.Body.String())
	}

	allowedUpdate := doRequest(
		bobSession,
		http.MethodPut,
		"/api/orgs/acme",
		`{"displayName":"Bob Admin Updated"}`,
	)
	if allowedUpdate.Code != http.StatusOK {
		t.Fatalf("expected admin update to succeed, got %d: %s", allowedUpdate.Code, allowedUpdate.Body.String())
	}

	orgAfterPromotion, err := mtp.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load organization after promotion: %v", err)
	}
	if orgAfterPromotion.DisplayName != "Bob Admin Updated" {
		t.Fatalf("displayName after admin update = %q, want %q", orgAfterPromotion.DisplayName, "Bob Admin Updated")
	}
	if orgAfterPromotion.GetMemberRole("bob") != models.OrgRoleAdmin {
		t.Fatalf("bob role after promotion = %q, want %q", orgAfterPromotion.GetMemberRole("bob"), models.OrgRoleAdmin)
	}

	demoteBob := doRequest(
		aliceSession,
		http.MethodPost,
		"/api/orgs/acme/members",
		`{"userId":"bob","role":"viewer"}`,
	)
	if demoteBob.Code != http.StatusOK {
		t.Fatalf("expected owner to demote bob, got %d: %s", demoteBob.Code, demoteBob.Body.String())
	}

	deniedAgain := doRequest(
		bobSession,
		http.MethodPut,
		"/api/orgs/acme",
		`{"displayName":"Viewer Denied Again"}`,
	)
	if deniedAgain.Code != http.StatusForbidden {
		t.Fatalf("expected demoted viewer update to be denied, got %d: %s", deniedAgain.Code, deniedAgain.Body.String())
	}

	orgAfterDemotion, err := mtp.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load organization after demotion: %v", err)
	}
	if orgAfterDemotion.DisplayName != "Bob Admin Updated" {
		t.Fatalf("displayName changed after denied viewer update = %q, want %q", orgAfterDemotion.DisplayName, "Bob Admin Updated")
	}
	if orgAfterDemotion.GetMemberRole("bob") != models.OrgRoleViewer {
		t.Fatalf("bob role after demotion = %q, want %q", orgAfterDemotion.GetMemberRole("bob"), models.OrgRoleViewer)
	}
}
