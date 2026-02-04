package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestTenantMiddlewareRejectsOrgCookieForNonMember(t *testing.T) {
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

	orgA := &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	orgB := &models.Organization{
		ID:          "org-b",
		DisplayName: "Org B",
		OwnerUserID: "charlie",
		Members: []models.OrganizationMember{
			{UserID: "charlie", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	mtp := config.NewMultiTenantPersistence(dataDir)
	if err := mtp.SaveOrganization(orgA); err != nil {
		t.Fatalf("save organization A: %v", err)
	}
	if err := mtp.SaveOrganization(orgB); err != nil {
		t.Fatalf("save organization B: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	sessionToken := "org-cookie-session-token"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: "pulse_org_id", Value: "org-b"})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member org cookie access, got %d", rec.Code)
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
