package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
)

func TestHandleMagicLinkVerifyPortalTargetCreatesPortalSession(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	token, err := svc.GeneratePortalToken("portal@example.com")
	if err != nil {
		t.Fatalf("GeneratePortalToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token="+token, nil)
	rec := httptest.NewRecorder()

	HandleMagicLinkVerify(svc, reg, filepath.Join(dir, "tenants"), "cloud.example.com")(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/portal" {
		t.Fatalf("location=%q, want %q", got, "/portal")
	}
	cookie := rec.Result().Cookies()
	if len(cookie) == 0 || cookie[0].Name != SessionCookieName || strings.TrimSpace(cookie[0].Value) == "" {
		t.Fatalf("expected %s session cookie, got %#v", SessionCookieName, cookie)
	}

	user, err := reg.GetUserByEmail("portal@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user == nil {
		t.Fatal("expected portal user to be created")
	}
	accountIDs, err := reg.ListAccountsByUser(user.ID)
	if err != nil {
		t.Fatalf("ListAccountsByUser: %v", err)
	}
	if len(accountIDs) != 0 {
		t.Fatalf("accountIDs=%v, want no memberships for portal-only login", accountIDs)
	}
}

func TestHandleMagicLinkVerifyTenantTargetStillRedirectsToTenantHandoff(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatalf("GenerateAccountID: %v", err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          accountID,
		Kind:        registry.AccountKindIndividual,
		DisplayName: "Hosted Account",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := reg.Create(&registry.Tenant{
		ID:          "t-tenant-1",
		AccountID:   accountID,
		Email:       "tenant@example.com",
		DisplayName: "Hosted Workspace",
		State:       registry.TenantStateActive,
	}); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	tenantsDir := filepath.Join(dir, "tenants")
	tenantDir := filepath.Join(tenantsDir, "t-tenant-1")
	if err := os.MkdirAll(tenantDir, 0o700); err != nil {
		t.Fatalf("MkdirAll tenant dir: %v", err)
	}
	handoffKey, err := cloudauth.GenerateHandoffKey()
	if err != nil {
		t.Fatalf("GenerateHandoffKey: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tenantDir, cloudauth.HandoffKeyFile), handoffKey, 0o600); err != nil {
		t.Fatalf("WriteFile handoff key: %v", err)
	}

	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	token, err := svc.GenerateToken("tenant@example.com", "t-tenant-1")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token="+token, nil)
	rec := httptest.NewRecorder()

	HandleMagicLinkVerify(svc, reg, tenantsDir, "cloud.example.com")(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "https://t-tenant-1.cloud.example.com/auth/cloud-handoff?token=") {
		t.Fatalf("location=%q, want tenant handoff redirect", location)
	}
	user, err := reg.GetUserByEmail("tenant@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user == nil {
		t.Fatal("expected tenant user to exist")
	}
	membership, err := reg.GetMembership(accountID, user.ID)
	if err != nil {
		t.Fatalf("GetMembership: %v", err)
	}
	if membership == nil {
		t.Fatal("expected tenant owner membership to be created")
	}

	handoffToken := strings.TrimPrefix(location, "https://t-tenant-1.cloud.example.com/auth/cloud-handoff?token=")
	handoffToken, err = url.QueryUnescape(handoffToken)
	if err != nil {
		t.Fatalf("QueryUnescape handoff token: %v", err)
	}
	claims, err := cloudauth.VerifyClaimsWithExpiry(handoffKey, handoffToken)
	if err != nil {
		t.Fatalf("VerifyClaimsWithExpiry: %v", err)
	}
	if claims.Email != "tenant@example.com" {
		t.Fatalf("claims.Email = %q, want %q", claims.Email, "tenant@example.com")
	}
	if claims.TenantID != "t-tenant-1" {
		t.Fatalf("claims.TenantID = %q, want %q", claims.TenantID, "t-tenant-1")
	}
	if claims.AccountID != accountID {
		t.Fatalf("claims.AccountID = %q, want %q", claims.AccountID, accountID)
	}
	if claims.Role != string(registry.MemberRoleOwner) {
		t.Fatalf("claims.Role = %q, want %q", claims.Role, registry.MemberRoleOwner)
	}
	if strings.TrimSpace(claims.UserID) == "" {
		t.Fatal("expected claims.UserID to be populated")
	}
}
