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

func TestTenantMiddlewareTokenTakesPrecedenceOverUser(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	rawToken := "token-precedence-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   hashed,
		APITokens:  []config.APITokenRecord{record},
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
		OwnerUserID: "bob",
		Members: []models.OrganizationMember{
			{UserID: "bob", Role: models.OrgRoleOwner, AddedAt: time.Now()},
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

	sessionToken := "user-session-token"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "bob")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-b")
	req.Header.Set("X-API-Token", rawToken)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for token-bound org mismatch, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
	if msg := payload["message"]; msg == "" || !strings.Contains(msg, "Token") {
		t.Fatalf("expected token access denied message, got %q", msg)
	}
}
