package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHandlePublicMagicLinkVerifyRejectsInvalidOrgIDInToken(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	store := NewInMemoryMagicLinkStore()
	svc := NewMagicLinkServiceWithKey(key, store, nil, nil)
	t.Cleanup(func() { svc.Stop() })

	token, err := randomOpaqueTokenID()
	if err != nil {
		t.Fatalf("randomOpaqueTokenID: %v", err)
	}
	tokenHash := signHMACSHA256(key, token)
	if err := store.Put(tokenHash, &MagicLinkToken{
		Email:     "alice@example.com",
		OrgID:     "../evil",
		ExpiresAt: time.Now().Add(5 * time.Minute).UTC(),
	}); err != nil {
		t.Fatalf("store.Put: %v", err)
	}

	h := NewMagicLinkHandlers(nil, svc, true, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/public/magic-link/verify?token="+token, nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.HandlePublicMagicLinkVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid org ID in token, got %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_session" {
			t.Fatalf("did not expect pulse_session cookie on invalid token org")
		}
	}
}

func TestHandlePublicMagicLinkVerifyUsesStableOrganizationPrincipal(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	sessionsMu.Lock()
	allSessions = make(map[string][]string)
	sessionsMu.Unlock()

	dataDir := t.TempDir()
	InitSessionStore(dataDir)
	InitCSRFStore(dataDir)
	persistence := config.NewMultiTenantPersistence(dataDir)

	now := time.Now().UTC()
	org := &models.Organization{
		ID:          "org_magic_stable",
		DisplayName: "Magic Stable",
		CreatedAt:   now,
		OwnerUserID: "u_owner_magic",
		OwnerEmail:  "owner@example.com",
		Members: []models.OrganizationMember{
			{
				UserID:  "u_owner_magic",
				Email:   "owner@example.com",
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: "u_owner_magic",
			},
		},
	}
	if err := persistence.SaveOrganization(org); err != nil {
		t.Fatalf("SaveOrganization: %v", err)
	}

	key := []byte("0123456789abcdef0123456789abcdef")
	store := NewInMemoryMagicLinkStore()
	svc := NewMagicLinkServiceWithKey(key, store, nil, nil)
	t.Cleanup(func() { svc.Stop() })
	token, err := svc.GenerateToken("OWNER@example.com", org.ID)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	h := NewMagicLinkHandlers(persistence, svc, true, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/public/magic-link/verify?format=json&token="+token, nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.HandlePublicMagicLinkVerify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Success bool   `json:"success"`
		OrgID   string `json:"org_id"`
		UserID  string `json:"user_id"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Success || payload.OrgID != org.ID || payload.UserID != "u_owner_magic" || payload.Email != "owner@example.com" {
		t.Fatalf("payload = %+v, want stable user id and contact email", payload)
	}

	var sessionToken string
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSession {
			sessionToken = cookie.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("expected session cookie")
	}
	session := GetSessionStore().GetSession(sessionToken)
	if session == nil {
		t.Fatal("expected stored session")
	}
	if session.Username != "u_owner_magic" {
		t.Fatalf("session username = %q, want stable user id", session.Username)
	}
	sessionsMu.RLock()
	tracked := append([]string(nil), allSessions["u_owner_magic"]...)
	emailTracked := append([]string(nil), allSessions["owner@example.com"]...)
	sessionsMu.RUnlock()
	if len(tracked) != 1 || tracked[0] != sessionToken {
		t.Fatalf("tracked stable sessions = %v, want [%s]", tracked, sessionToken)
	}
	if len(emailTracked) != 0 {
		t.Fatalf("email-keyed sessions = %v, want none", emailTracked)
	}
}

func TestHandlePublicMagicLinkVerifyRejectsRemovedOrganizationMember(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	dataDir := t.TempDir()
	InitSessionStore(dataDir)
	InitCSRFStore(dataDir)
	persistence := config.NewMultiTenantPersistence(dataDir)

	now := time.Now().UTC()
	if err := persistence.SaveOrganization(&models.Organization{
		ID:          "org_magic_removed",
		DisplayName: "Magic Removed",
		CreatedAt:   now,
		OwnerUserID: "u_owner_magic",
		OwnerEmail:  "owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "u_owner_magic", Email: "owner@example.com", Role: models.OrgRoleOwner, AddedAt: now},
		},
	}); err != nil {
		t.Fatalf("SaveOrganization initial: %v", err)
	}

	key := []byte("0123456789abcdef0123456789abcdef")
	store := NewInMemoryMagicLinkStore()
	svc := NewMagicLinkServiceWithKey(key, store, nil, nil)
	t.Cleanup(func() { svc.Stop() })
	token, err := svc.GenerateToken("owner@example.com", "org_magic_removed")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if err := persistence.SaveOrganization(&models.Organization{
		ID:          "org_magic_removed",
		DisplayName: "Magic Removed",
		CreatedAt:   now,
		OwnerUserID: "u_other_owner",
		OwnerEmail:  "other@example.com",
		Members: []models.OrganizationMember{
			{UserID: "u_other_owner", Email: "other@example.com", Role: models.OrgRoleOwner, AddedAt: now},
		},
	}); err != nil {
		t.Fatalf("SaveOrganization removed: %v", err)
	}

	h := NewMagicLinkHandlers(persistence, svc, true, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/public/magic-link/verify?format=json&token="+token, nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.HandlePublicMagicLinkVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSession {
			t.Fatalf("did not expect session cookie for removed member")
		}
	}
}
