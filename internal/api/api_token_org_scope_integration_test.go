package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestMultiTenantAPITokenRemainsScopedToIssuingOrg(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	const rawToken = "org-bound-router-token-123.12345678"

	dataDir := t.TempDir()
	hashedPass, err := internalauth.HashPassword("super-secure-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	record, err := config.NewAPITokenRecord(rawToken, "org-a-token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("create api token record: %v", err)
	}
	record.OrgID = "org-a"

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashedPass,
		APITokens:  []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

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
	sessionClient := &http.Client{Jar: jar}

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
	loginResp, err := sessionClient.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}

	assertConfigAccess := func(orgID string, wantStatus int) string {
		t.Helper()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/config", nil)
		if err != nil {
			t.Fatalf("create config request: %v", err)
		}
		req.Header.Set("X-Pulse-Org-ID", orgID)
		req.Header.Set("Authorization", "Bearer "+rawToken)

		res, err := sessionClient.Do(req)
		if err != nil {
			t.Fatalf("config request failed: %v", err)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read config response: %v", err)
		}
		if res.StatusCode != wantStatus {
			t.Fatalf("expected %d for org %s, got %d: %s", wantStatus, orgID, res.StatusCode, string(body))
		}
		return string(body)
	}

	assertConfigAccess("org-a", http.StatusOK)
	body := assertConfigAccess("org-b", http.StatusForbidden)
	if !strings.Contains(body, "access_denied") || !strings.Contains(body, "Token") {
		t.Fatalf("expected cross-org denial payload to mention access_denied and token binding, got %q", body)
	}
}
