package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
)

func TestHandleCloudHandoffRejectsReplay(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:    "alice@example.com",
		TenantID: "tenant-1",
		UserID:   "user-alice",
		Role:     "owner",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}
	saveHandoffTestOrganization(t, dataPath, &models.Organization{
		ID:          "tenant-1",
		DisplayName: "Tenant One",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "alice@example.com",
	})

	handler := HandleCloudHandoff(dataPath)

	firstReq := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	firstRec := httptest.NewRecorder()
	handler(firstRec, firstReq)

	if firstRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("first use status = %d, want %d", firstRec.Code, http.StatusTemporaryRedirect)
	}
	if got := firstRec.Header().Get("Location"); got != "/" {
		t.Fatalf("first use redirect = %q, want %q", got, "/")
	}
	if cookieHeaders := firstRec.Header().Values("Set-Cookie"); len(cookieHeaders) == 0 || !strings.Contains(strings.Join(cookieHeaders, ";"), "pulse_session=") {
		t.Fatalf("first use should set pulse_session cookie, got headers: %v", cookieHeaders)
	}

	replayReq := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	replayRec := httptest.NewRecorder()
	handler(replayRec, replayReq)

	if replayRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("replay status = %d, want %d", replayRec.Code, http.StatusTemporaryRedirect)
	}
	if got := replayRec.Header().Get("Location"); got != "/login?error=handoff_replayed" {
		t.Fatalf("replay redirect = %q, want %q", got, "/login?error=handoff_replayed")
	}
}

func TestHandleCloudHandoffSetsTenantOrgCookie(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:    "alice@example.com",
		TenantID: "tenant-1",
		UserID:   "user-alice",
		Role:     "owner",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}
	saveHandoffTestOrganization(t, dataPath, &models.Organization{
		ID:          "tenant-1",
		DisplayName: "Tenant One",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "alice@example.com",
	})

	handler := HandleCloudHandoff(dataPath)

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	var orgCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_org_id" {
			orgCookie = c
			break
		}
	}
	if orgCookie == nil {
		t.Fatal("expected pulse_org_id cookie to be set")
	}
	if orgCookie.Value != "tenant-1" {
		t.Fatalf("pulse_org_id cookie = %q, want %q", orgCookie.Value, "tenant-1")
	}
}

func TestHandleCloudHandoffRejectsInvalidTenantID(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:    "alice@example.com",
		TenantID: "../tenant-1",
		UserID:   "user-alice",
		Role:     "owner",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}

	handler := HandleCloudHandoff(dataPath)

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if got := rec.Header().Get("Location"); got != "/login?error=handoff_invalid" {
		t.Fatalf("redirect = %q, want %q", got, "/login?error=handoff_invalid")
	}
}

func TestHandleCloudHandoffRejectsBlankUserID(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	token, err := cloudauth.Sign(key, "alice@example.com", "tenant-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}
	saveHandoffTestOrganization(t, dataPath, &models.Organization{
		ID:          "tenant-1",
		DisplayName: "Tenant One",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "alice@example.com",
	})

	handler := HandleCloudHandoff(dataPath)

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if got := rec.Header().Get("Location"); got != "/login?error=handoff_invalid" {
		t.Fatalf("redirect = %q, want %q", got, "/login?error=handoff_invalid")
	}
}

func TestHandleCloudHandoffLowercasesSessionEmailIdentity(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	sessionsMu.Lock()
	allSessions = make(map[string][]string)
	sessionsMu.Unlock()
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:     "Operator.Owner+Mixed@PulseRelay.Pro",
		TenantID:  "tenant-1",
		AccountID: "acct-mixed-email",
		UserID:    "user-mixed-email",
		Role:      "owner",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}
	saveHandoffTestOrganization(t, dataPath, &models.Organization{
		ID:          "tenant-1",
		DisplayName: "Tenant One",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "operator.owner+mixed@pulserelay.pro",
		Members: []models.OrganizationMember{
			{UserID: "operator.owner+mixed@pulserelay.pro", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	})

	handler := HandleCloudHandoff(dataPath)

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("redirect = %q, want %q", got, "/")
	}

	sessionsMu.RLock()
	tracked := append([]string(nil), allSessions["user-mixed-email"]...)
	sessionsMu.RUnlock()
	if len(tracked) == 0 {
		t.Fatal("expected stable user session to be tracked")
	}

	var session *SessionData
	for i := len(tracked) - 1; i >= 0; i-- {
		session = GetSessionStore().GetSession(tracked[i])
		if session != nil {
			break
		}
	}
	if session == nil {
		t.Fatal("expected tracked session to exist in the session store")
	}
	if session.Username != "user-mixed-email" {
		t.Fatalf("session username = %q, want stable user id %q", session.Username, "user-mixed-email")
	}
}

func TestHandleCloudHandoffUsesExistingTenantOrganizationMembership(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	tenantID := "tenant-claims-membership"
	mtp := saveHandoffTestOrganization(t, dataPath, &models.Organization{
		ID:          tenantID,
		DisplayName: "Claims Membership",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "legacy-owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
			{UserID: "courtmanr@gmail.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	})

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:     "courtmanr@gmail.com",
		TenantID:  tenantID,
		AccountID: "acct-claims-membership",
		UserID:    "user-claims-membership",
		Role:      "owner",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}

	handler := HandleCloudHandoff(dataPath)

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("redirect = %q, want %q", got, "/")
	}

	org, err := mtp.LoadOrganization(tenantID)
	if err != nil {
		t.Fatalf("load organization: %v", err)
	}
	if org.OwnerUserID != "legacy-owner@example.com" {
		t.Fatalf("ownerUserID = %q, want %q", org.OwnerUserID, "legacy-owner@example.com")
	}
	if got := org.GetMemberRole("user-claims-membership"); got != models.OrgRoleOwner {
		t.Fatalf("member role = %q, want %q", got, models.OrgRoleOwner)
	}
	if got := org.Members[1].Email; got != "courtmanr@gmail.com" {
		t.Fatalf("canonicalized member email = %q, want %q", got, "courtmanr@gmail.com")
	}
}

func TestHandleCloudHandoffRejectsMissingTenantOrganizationMembership(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)
	dataPath := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dataPath, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	tenantID := "tenant-missing-membership"
	saveHandoffTestOrganization(t, dataPath, &models.Organization{
		ID:          tenantID,
		DisplayName: "Missing Membership",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "legacy-owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	})

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:     "courtmanr@gmail.com",
		TenantID:  tenantID,
		AccountID: "acct-missing-membership",
		UserID:    "user-missing-membership",
		Role:      "owner",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("sign handoff token: %v", err)
	}

	handler := HandleCloudHandoff(dataPath)

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/login?error=handoff_invalid" {
		t.Fatalf("redirect = %q, want %q", got, "/login?error=handoff_invalid")
	}
}
