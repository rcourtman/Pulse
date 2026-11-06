package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func notificationAuthorityRouter(t *testing.T) *Router {
	t.Helper()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(false) })
	t.Setenv("PULSE_DEV", "true")

	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, AuthUser: "platform-admin", AuthPass: hashed}
	mtp := config.NewMultiTenantPersistence(dataDir)
	for _, id := range []string{"org-a", "org-b"} {
		if err := mtp.SaveOrganization(&models.Organization{
			ID:          id,
			DisplayName: id,
			OwnerUserID: id + "-owner",
			Members: []models.OrganizationMember{
				{UserID: id + "-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
				{UserID: id + "-viewer", Role: models.OrgRoleViewer, AddedAt: time.Now()},
			},
		}); err != nil {
			t.Fatalf("save organization: %v", err)
		}
	}
	hub := websocket.NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	monitors := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(monitors.Stop)
	return newTestRouter(t, cfg, nil, monitors, hub, nil, "1.0.0")
}

func notificationAuthorityRequest(t *testing.T, router *Router, method, path, body, user, org string) *httptest.ResponseRecorder {
	t.Helper()
	session := "notification-authority-" + user + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	GetSessionStore().CreateSession(session, time.Hour, "agent", "127.0.0.1", user)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", generateCSRFToken(session))
	req.Header.Set("X-Pulse-Org-ID", org)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	return rec
}

// A provider opening a client workspace arrives as that organization's owner.
// Alert destinations are the organization's own, read and written through its
// own monitor, so its managers configure them. Viewers may not, and neither
// may a manager of a different organization.
func TestNotificationDestinationsAreOrganizationBound(t *testing.T) {
	router := notificationAuthorityRouter(t)

	for _, path := range []string{
		"/api/notifications/email",
		"/api/notifications/apprise",
		"/api/notifications/webhooks",
		"/api/notifications/webhook-templates",
		"/api/notifications/email-providers",
		"/api/notifications/health",
		"/api/notifications/delivery-log",
	} {
		for _, tc := range []struct {
			user   string
			org    string
			status int
		}{
			{"org-a-owner", "org-a", http.StatusOK},
			{"org-a-viewer", "org-a", http.StatusForbidden},
			{"org-b-owner", "org-a", http.StatusForbidden},
		} {
			rec := notificationAuthorityRequest(t, router, http.MethodGet, path, "", tc.user, tc.org)
			if rec.Code != tc.status {
				t.Fatalf("%s as %s in %s = %d, want %d: %s", path, tc.user, tc.org, rec.Code, tc.status, rec.Body.String())
			}
		}
	}
}

// One organization's saved destination stays out of every other organization.
func TestNotificationDestinationSavedByOrganizationOwnerStaysInThatOrganization(t *testing.T) {
	router := notificationAuthorityRouter(t)

	body, err := json.Marshal(notifications.WebhookConfig{
		ID:      "org-a-hook",
		Name:    "Client A pager",
		URL:     "https://1.1.1.1/client-a", // a public literal, so saving needs no DNS
		Method:  http.MethodPost,
		Enabled: true,
		Service: "generic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/notifications/webhooks", string(body), "org-a-owner", "org-a"); rec.Code != http.StatusOK {
		t.Fatalf("org owner saving a webhook = %d: %s", rec.Code, rec.Body.String())
	}

	read := func(user, org string) string {
		t.Helper()
		rec := notificationAuthorityRequest(t, router, http.MethodGet, "/api/notifications/webhooks", "", user, org)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s reading webhooks in %s = %d: %s", user, org, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if got := read("org-a-owner", "org-a"); !strings.Contains(got, "Client A pager") {
		t.Fatalf("org-a lost its own webhook: %s", got)
	}
	if got := read("org-b-owner", "org-b"); strings.Contains(got, "Client A pager") {
		t.Fatalf("org-b can read org-a's webhook: %s", got)
	}
	if got := read("platform-admin", "default"); strings.Contains(got, "Client A pager") {
		t.Fatalf("default organization received org-a's webhook: %s", got)
	}
}

// The queue and dead-letter routes read one shared queue, not the request
// organization's, so they stay instance administration.
func TestNotificationQueueRoutesStayInstanceAdministration(t *testing.T) {
	router := notificationAuthorityRouter(t)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/notifications/dlq"},
		{http.MethodGet, "/api/notifications/queue/stats"},
		{http.MethodPost, "/api/notifications/dlq/retry"},
		{http.MethodPost, "/api/notifications/dlq/delete"},
		{http.MethodPost, "/api/notifications/terminal-failures/retry"},
		{http.MethodPost, "/api/notifications/terminal-failures/dismiss"},
	} {
		rec := notificationAuthorityRequest(t, router, tc.method, tc.path, "{}", "org-a-owner", "org-a")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s as org owner = %d, want 403: %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

// Apprise CLI targets run as this server, so an organization manager reaches
// Apprise through an API server. Disabled or empty CLI settings carry nothing,
// so saving them is fine, and an instance administrator keeps the command line.
func TestAppriseCommandLineStaysWithInstanceAdministrators(t *testing.T) {
	router := notificationAuthorityRouter(t)
	const cli = `{"enabled":true,"mode":"cli","targets":["discord://id/token"]}`

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		user   string
		org    string
		status int
	}{
		{"owner saves CLI targets", http.MethodPut, "/api/notifications/apprise", cli, "org-a-owner", "org-a", http.StatusForbidden},
		{"owner saves targets with no mode, which means CLI", http.MethodPut, "/api/notifications/apprise", `{"enabled":true,"targets":["discord://id/token"]}`, "org-a-owner", "org-a", http.StatusForbidden},
		{"owner tests CLI targets", http.MethodPost, "/api/notifications/test", `{"method":"apprise","config":` + cli + `}`, "org-a-owner", "org-a", http.StatusForbidden},
		{"owner saves empty defaults", http.MethodPut, "/api/notifications/apprise", `{"enabled":false,"mode":"cli","targets":[]}`, "org-a-owner", "org-a", http.StatusOK},
		{"owner saves an API server", http.MethodPut, "/api/notifications/apprise", `{"enabled":true,"mode":"http","serverUrl":"https://1.1.1.1"}`, "org-a-owner", "org-a", http.StatusOK},
		{"administrator saves CLI targets", http.MethodPut, "/api/notifications/apprise", cli, "platform-admin", "default", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := notificationAuthorityRequest(t, router, tc.method, tc.path, tc.body, tc.user, tc.org)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}

	// Targets an administrator already saved add nothing when a manager saves
	// the same settings again. An instance administrator is not a member of a
	// client organization, so the saved targets are seeded with authority granted.
	router.notificationHandlers.SetAppriseCLIAuthority(func(*http.Request) bool { return true })
	if rec := notificationAuthorityRequest(t, router, http.MethodPut, "/api/notifications/apprise", cli, "org-b-owner", "org-b"); rec.Code != http.StatusOK {
		t.Fatalf("seeding saved targets = %d: %s", rec.Code, rec.Body.String())
	}
	router.notificationHandlers.SetAppriseCLIAuthority(func(req *http.Request) bool {
		return requestCarriesInstanceAuthority(router.config, req)
	})
	if rec := notificationAuthorityRequest(t, router, http.MethodPut, "/api/notifications/apprise", cli, "org-b-owner", "org-b"); rec.Code != http.StatusOK {
		t.Fatalf("owner re-saving the administrator's targets = %d: %s", rec.Code, rec.Body.String())
	}
	// Targets saved for an API server were never allowed on the command line,
	// however they are carried across.
	smuggled := `{"enabled":true,"mode":"http","serverUrl":"https://1.1.1.1","targets":["json://10.0.0.1"]}`
	if rec := notificationAuthorityRequest(t, router, http.MethodPut, "/api/notifications/apprise", smuggled, "org-a-owner", "org-a"); rec.Code != http.StatusOK {
		t.Fatalf("owner saving API-mode targets = %d: %s", rec.Code, rec.Body.String())
	}
	asCLI := `{"enabled":true,"mode":"cli","targets":["json://10.0.0.1"]}`
	if rec := notificationAuthorityRequest(t, router, http.MethodPost, "/api/notifications/test", `{"method":"apprise","config":`+asCLI+`}`, "org-a-owner", "org-a"); rec.Code != http.StatusForbidden {
		t.Fatalf("owner testing API-mode targets as CLI = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if rec := notificationAuthorityRequest(t, router, http.MethodPut, "/api/notifications/apprise", asCLI, "org-a-owner", "org-a"); rec.Code != http.StatusForbidden {
		t.Fatalf("owner flipping API-mode targets to CLI = %d, want 403: %s", rec.Code, rec.Body.String())
	}

	changed := `{"enabled":true,"mode":"cli","targets":["discord://id/token","json://10.0.0.1"]}`
	if rec := notificationAuthorityRequest(t, router, http.MethodPut, "/api/notifications/apprise", changed, "org-b-owner", "org-b"); rec.Code != http.StatusForbidden {
		t.Fatalf("owner adding a target = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

// Destinations hold reusable webhook URLs and bot tokens, so admitting the
// organization's managers does not admit tokens bound to that organization.
func TestNotificationDestinationsRefuseOrganizationBoundTokens(t *testing.T) {
	router := notificationAuthorityRouter(t)
	const raw = "org-bound-notification-token-123.12345678"
	record, err := config.NewAPITokenRecord(raw, "client token", []string{config.ScopeSettingsRead, config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	record.OrgID = "org-a"
	router.config.APITokens = append(router.config.APITokens, *record)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/webhooks", nil)
	req.Header.Set("X-API-Token", raw)
	req.Header.Set("X-Pulse-Org-ID", "org-a")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("org-bound token reading destinations = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}
