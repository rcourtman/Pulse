package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestOrgScopedAPITokenCannotReadOtherOrgMembersWithSessionCookiePresent(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashedPass, err := internalauth.HashPassword("super-secure-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashedPass,
	}

	mtp := config.NewMultiTenantPersistence(dataDir)
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := mtp.SaveOrganization(&models.Organization{
			ID:          orgID,
			DisplayName: strings.ToUpper(orgID),
			OwnerUserID: "admin",
			Members: []models.OrganizationMember{
				{UserID: "admin", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
			},
		}); err != nil {
			t.Fatalf("save organization %s: %v", orgID, err)
		}
	}

	router := newMultiTenantRouter(t, cfg)
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	loginBody, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "super-secure-pass",
	})
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}

	loginReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/login", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	csrfToken := ""
	for _, cookie := range jar.Cookies(serverURL) {
		if cookie.Name == CookieNameCSRF {
			csrfToken = cookie.Value
			break
		}
	}
	if csrfToken == "" {
		t.Fatalf("expected CSRF cookie after login")
	}

	createReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/security/tokens", strings.NewReader(`{"name":"org-a-token","scopes":["settings:read"]}`))
	if err != nil {
		t.Fatalf("create token request: %v", err)
	}
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.Header.Set("X-Pulse-Org-ID", "org-a")
	createReq.Header.Set("X-Org-ID", "org-a")
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("token creation failed: %v", err)
	}
	defer createResp.Body.Close()
	createBody, err := io.ReadAll(createResp.Body)
	if err != nil {
		t.Fatalf("read token create response: %v", err)
	}
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 creating token, got %d: %s", createResp.StatusCode, string(createBody))
	}

	var created struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(createBody, &created); err != nil {
		t.Fatalf("decode token create response: %v", err)
	}
	if strings.TrimSpace(created.Token) == "" {
		t.Fatalf("expected raw token in create response")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one persisted token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].OrgID; got != "org-a" {
		t.Fatalf("expected created token to bind to org-a, got %q", got)
	}

	assertMembersStatus := func(path string, headers map[string]string, wantStatus int) string {
		t.Helper()

		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatalf("create members request: %v", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("members request failed: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read members response: %v", err)
		}
		if resp.StatusCode != wantStatus {
			t.Fatalf("expected %d for %s, got %d: %s", wantStatus, path, resp.StatusCode, string(body))
		}
		return string(body)
	}

	headers := map[string]string{
		"Authorization":  "Bearer " + created.Token,
		"X-Pulse-Org-ID": "org-a",
		"X-Org-ID":       "org-a",
	}
	assertMembersStatus("/api/orgs/org-a/members", headers, http.StatusOK)
	body := assertMembersStatus("/api/orgs/org-b/members", headers, http.StatusForbidden)
	if !strings.Contains(body, "access_denied") {
		t.Fatalf("expected cross-org denial payload, got %q", body)
	}
}
