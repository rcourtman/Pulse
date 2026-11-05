package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	basicTenantAdmin    = "instance-admin"
	basicTenantPassword = "SyntheticPassword!1"
)

type basicTenantFixture struct {
	router *Router
	cfg    *config.Config
}

// newBasicTenantFixture builds a shared-process multi-organization runtime
// whose configured local administrator is a viewer of tenant-viewer, an admin
// of tenant-managed, and not a member of tenant-foreign.
func newBasicTenantFixture(t *testing.T) *basicTenantFixture {
	t.Helper()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(false) })
	t.Setenv("PULSE_DEV", "true")
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	hash, err := internalauth.HashPassword(basicTenantPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, AuthUser: basicTenantAdmin, AuthPass: hash}
	mtp := config.NewMultiTenantPersistence(dataDir)
	now := time.Now()
	orgs := []*models.Organization{
		{
			ID: "tenant-viewer", DisplayName: "Viewer Tenant", OwnerUserID: "viewer-owner",
			Members: []models.OrganizationMember{
				{UserID: "viewer-owner", Role: models.OrgRoleOwner, AddedAt: now},
				{UserID: basicTenantAdmin, Role: models.OrgRoleViewer, AddedAt: now},
			},
		},
		{
			ID: "tenant-managed", DisplayName: "Managed Tenant", OwnerUserID: "managed-owner",
			Members: []models.OrganizationMember{
				{UserID: "managed-owner", Role: models.OrgRoleOwner, AddedAt: now},
				{UserID: basicTenantAdmin, Role: models.OrgRoleAdmin, AddedAt: now},
			},
		},
		{
			ID: "tenant-foreign", DisplayName: "Foreign Tenant", OwnerUserID: "foreign-owner",
			Members: []models.OrganizationMember{
				{UserID: "foreign-owner", Role: models.OrgRoleOwner, AddedAt: now},
			},
		},
	}
	for _, org := range orgs {
		if err := mtp.SaveOrganization(org); err != nil {
			t.Fatalf("save organization %s: %v", org.ID, err)
		}
	}
	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	defaultMonitor, err := mtm.GetMonitor("default")
	if err != nil {
		t.Fatalf("default monitor: %v", err)
	}
	router := newTestRouter(t, cfg, defaultMonitor, mtm, nil, nil, "test")
	cleanupTestRouter(t, router)
	return &basicTenantFixture{router: router, cfg: cfg}
}

func (f *basicTenantFixture) serve(t *testing.T, method, path, orgID, password, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if orgID != "" {
		req.Header.Set("X-Pulse-Org-ID", orgID)
	}
	credentials := base64.StdEncoding.EncodeToString([]byte(basicTenantAdmin + ":" + password))
	req.Header.Set("Authorization", "Basic "+credentials)
	// Pulse's web client is the one Basic caller that gets a session, so these
	// requests cover whether an organization denial still issues one.
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	f.router.Handler().ServeHTTP(rec, req)
	return rec
}

func basicResponseIssuedSession(rec *httptest.ResponseRecorder) bool {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSession || cookie.Name == cookieNameSessionSecure {
			return true
		}
	}
	return false
}

func TestBasicAuthObeysOrganizationMembership(t *testing.T) {
	f := newBasicTenantFixture(t)

	reads := []string{"/api/config/nodes", "/api/state", "/api/state/summary"}
	for _, path := range reads {
		t.Run("default organization "+path, func(t *testing.T) {
			rec := f.serve(t, http.MethodGet, path, "", basicTenantPassword, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("default organization read = %d: %s", rec.Code, rec.Body.String())
			}
		})
		t.Run("managed tenant "+path, func(t *testing.T) {
			rec := f.serve(t, http.MethodGet, path, "tenant-managed", basicTenantPassword, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("managed tenant read = %d: %s", rec.Code, rec.Body.String())
			}
		})
		t.Run("foreign tenant "+path, func(t *testing.T) {
			rec := f.serve(t, http.MethodGet, path, "tenant-foreign", basicTenantPassword, "")
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "access_denied") {
				t.Fatalf("foreign tenant read = %d: %s", rec.Code, rec.Body.String())
			}
			if basicResponseIssuedSession(rec) {
				t.Fatal("foreign tenant denial issued a session")
			}
		})
	}

	t.Run("tenant membership admits reads", func(t *testing.T) {
		rec := f.serve(t, http.MethodGet, "/api/state", "tenant-viewer", basicTenantPassword, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("member tenant state = %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestBasicAuthCannotMintTokensForForeignOrganization(t *testing.T) {
	f := newBasicTenantFixture(t)
	body := `{"name":"synthetic","scopes":["monitoring:read"]}`

	rec := f.serve(t, http.MethodPost, "/api/security/tokens", "tenant-foreign", basicTenantPassword, body)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "access_denied") {
		t.Fatalf("foreign tenant token mint = %d: %s", rec.Code, rec.Body.String())
	}
	if len(f.cfg.APITokens) != 0 {
		t.Fatalf("denied mint persisted %d tokens", len(f.cfg.APITokens))
	}
	if basicResponseIssuedSession(rec) {
		t.Fatal("denied mint issued a session")
	}

	rec = f.serve(t, http.MethodPost, "/api/security/tokens", "", basicTenantPassword, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("default organization token mint = %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Record.ID == "" {
		t.Fatalf("decode default mint: %v %s", err, rec.Body.String())
	}
	if len(f.cfg.APITokens) != 1 || f.cfg.APITokens[0].OrgID != "default" {
		t.Fatalf("default mint persisted %+v", f.cfg.APITokens)
	}
}

func TestBasicAuthOrganizationDenialKeepsCredentialSemantics(t *testing.T) {
	f := newBasicTenantFixture(t)

	t.Run("wrong password still fails authentication and counts", func(t *testing.T) {
		before, _, _ := GetLockoutInfo(basicTenantAdmin)
		rec := f.serve(t, http.MethodGet, "/api/state", "tenant-foreign", "wrong-password", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password = %d: %s", rec.Code, rec.Body.String())
		}
		after, _, _ := GetLockoutInfo(basicTenantAdmin)
		if after != before+1 {
			t.Fatalf("failed attempts %d -> %d, want one more", before, after)
		}
		ClearFailedLogins(basicTenantAdmin)
		ClearFailedLogins("192.0.2.1")
	})

	t.Run("verified password refused by organization is not a failed login", func(t *testing.T) {
		rec := f.serve(t, http.MethodGet, "/api/state", "tenant-foreign", basicTenantPassword, "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("foreign tenant = %d: %s", rec.Code, rec.Body.String())
		}
		if attempts, _, _ := GetLockoutInfo(basicTenantAdmin); attempts != 0 {
			t.Fatalf("organization denial recorded %d failed logins", attempts)
		}
	})

	t.Run("successful default request issues the Basic session", func(t *testing.T) {
		rec := f.serve(t, http.MethodGet, "/api/state", "", basicTenantPassword, "")
		if rec.Code != http.StatusOK || !basicResponseIssuedSession(rec) {
			t.Fatalf("default Basic request = %d, session issued %v", rec.Code, basicResponseIssuedSession(rec))
		}
		if got := rec.Header().Get("X-Auth-Method"); got != "basic" {
			t.Fatalf("auth method header = %q", got)
		}
	})
}
