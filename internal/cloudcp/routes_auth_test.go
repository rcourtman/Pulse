package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestRegisterRoutes_StatusAuthModes(t *testing.T) {
	t.Run("default requires admin key", func(t *testing.T) {
		dir := t.TempDir()
		reg, err := registry.NewTenantRegistry(dir)
		if err != nil {
			t.Fatalf("NewTenantRegistry: %v", err)
		}
		t.Cleanup(func() { _ = reg.Close() })

		mux := http.NewServeMux()
		RegisterRoutes(mux, &Deps{
			Config: &CPConfig{
				DataDir:             dir,
				AdminKey:            "test-admin-key",
				BaseURL:             "https://cloud.example.com",
				StripeWebhookSecret: "whsec_test",
			},
			Registry: reg,
			Version:  "test",
		})

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
	})

	t.Run("public status enabled", func(t *testing.T) {
		dir := t.TempDir()
		reg, err := registry.NewTenantRegistry(dir)
		if err != nil {
			t.Fatalf("NewTenantRegistry: %v", err)
		}
		t.Cleanup(func() { _ = reg.Close() })

		mux := http.NewServeMux()
		RegisterRoutes(mux, &Deps{
			Config: &CPConfig{
				DataDir:             dir,
				AdminKey:            "test-admin-key",
				BaseURL:             "https://cloud.example.com",
				PublicStatus:        true,
				StripeWebhookSecret: "whsec_test",
			},
			Registry: reg,
			Version:  "test",
		})

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}

func TestRegisterRoutes_AccountRoutesRequireSession(t *testing.T) {
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
		Kind:        registry.AccountKindMSP,
		DisplayName: "Acme MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatalf("GenerateUserID: %v", err)
	}
	if err := reg.CreateUser(&registry.User{
		ID:    userID,
		Email: "owner@example.com",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    userID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatalf("CreateMembership: %v", err)
	}

	magicSvc, err := cpauth.NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(magicSvc.Close)

	token, err := magicSvc.GenerateSessionToken(userID, "owner@example.com", cpauth.SessionTTL)
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:   reg,
		MagicLinks: magicSvc,
		Version:    "test",
	})

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/tenants", nil)
	unauthRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want %d (body=%q)", unauthRec.Code, http.StatusUnauthorized, unauthRec.Body.String())
	}

	authReq := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/tenants", nil)
	authReq.Header.Set("Authorization", "Bearer "+token)
	authRec := httptest.NewRecorder()
	mux.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want %d (body=%q)", authRec.Code, http.StatusOK, authRec.Body.String())
	}

	// Revoke sessions and verify the previously issued token is no longer valid.
	if _, err := reg.RevokeUserSessions(userID); err != nil {
		t.Fatalf("RevokeUserSessions: %v", err)
	}

	revokedReq := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/tenants", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+token)
	revokedRec := httptest.NewRecorder()
	mux.ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked status = %d, want %d (body=%q)", revokedRec.Code, http.StatusUnauthorized, revokedRec.Body.String())
	}
}

func TestRegisterRoutes_LogoutRevokesSession(t *testing.T) {
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
		Kind:        registry.AccountKindMSP,
		DisplayName: "Acme MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatalf("GenerateUserID: %v", err)
	}
	if err := reg.CreateUser(&registry.User{
		ID:    userID,
		Email: "owner@example.com",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    userID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatalf("CreateMembership: %v", err)
	}

	magicSvc, err := cpauth.NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(magicSvc.Close)

	sessionVersion, err := reg.GetUserSessionVersion(userID)
	if err != nil {
		t.Fatalf("GetUserSessionVersion: %v", err)
	}
	token, err := magicSvc.GenerateSessionTokenWithVersion(userID, "owner@example.com", sessionVersion, cpauth.SessionTTL)
	if err != nil {
		t.Fatalf("GenerateSessionTokenWithVersion: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:   reg,
		MagicLinks: magicSvc,
		Version:    "test",
	})

	logoutReq := httptest.NewRequest(http.MethodPost, portal.PortalLogoutPath, nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRec := httptest.NewRecorder()
	mux.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d (body=%q)", logoutRec.Code, http.StatusOK, logoutRec.Body.String())
	}

	afterReq := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/tenants", nil)
	afterReq.Header.Set("Authorization", "Bearer "+token)
	afterRec := httptest.NewRecorder()
	mux.ServeHTTP(afterRec, afterReq)
	if afterRec.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d, want %d (body=%q)", afterRec.Code, http.StatusUnauthorized, afterRec.Body.String())
	}
}

func TestRegisterRoutes_PortalBootstrapRequiresSession(t *testing.T) {
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

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatalf("GenerateUserID: %v", err)
	}
	if err := reg.CreateUser(&registry.User{
		ID:    userID,
		Email: "owner@example.com",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    userID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatalf("CreateMembership: %v", err)
	}

	magicSvc, err := cpauth.NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(magicSvc.Close)

	token, err := magicSvc.GenerateSessionToken(userID, "owner@example.com", cpauth.SessionTTL)
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:   reg,
		MagicLinks: magicSvc,
		Version:    "test",
	})

	unauthReq := httptest.NewRequest(http.MethodGet, portal.PortalBootstrapPath, nil)
	unauthRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want %d (body=%q)", unauthRec.Code, http.StatusUnauthorized, unauthRec.Body.String())
	}

	authReq := httptest.NewRequest(http.MethodGet, portal.PortalBootstrapPath, nil)
	authReq.Header.Set("Authorization", "Bearer "+token)
	authRec := httptest.NewRecorder()
	mux.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want %d (body=%q)", authRec.Code, http.StatusOK, authRec.Body.String())
	}
	if !strings.Contains(authRec.Body.String(), "\"email\":\"owner@example.com\"") {
		t.Fatalf("expected bootstrap payload to include owner email, body=%q", authRec.Body.String())
	}
	if !strings.Contains(authRec.Body.String(), "\"id\":\""+accountID+"\"") {
		t.Fatalf("expected bootstrap payload to include account id, body=%q", authRec.Body.String())
	}

	if _, err := reg.RevokeUserSessions(userID); err != nil {
		t.Fatalf("RevokeUserSessions: %v", err)
	}

	revokedReq := httptest.NewRequest(http.MethodGet, portal.PortalBootstrapPath, nil)
	revokedReq.Header.Set("Authorization", "Bearer "+token)
	revokedRec := httptest.NewRecorder()
	mux.ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked status = %d, want %d (body=%q)", revokedRec.Code, http.StatusUnauthorized, revokedRec.Body.String())
	}
}

func TestRegisterRoutes_PortalPageSessionModes(t *testing.T) {
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
		Kind:        registry.AccountKindMSP,
		DisplayName: "Acme MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatalf("GenerateUserID: %v", err)
	}
	if err := reg.CreateUser(&registry.User{
		ID:    userID,
		Email: "owner@example.com",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    userID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatalf("CreateMembership: %v", err)
	}

	magicSvc, err := cpauth.NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(magicSvc.Close)

	token, err := magicSvc.GenerateSessionToken(userID, "owner@example.com", cpauth.SessionTTL)
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:   reg,
		MagicLinks: magicSvc,
		Version:    "test",
	})

	unauthReq := httptest.NewRequest(http.MethodGet, portal.PortalPagePath, nil)
	unauthRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusOK {
		t.Fatalf("unauth status = %d, want %d (body=%q)", unauthRec.Code, http.StatusOK, unauthRec.Body.String())
	}
	for _, needle := range []string{
		`id="portal-app-root"`,
		`"authenticated":false`,
		`"magic_link_request_path":"` + portal.PortalMagicLinkRequestPath + `"`,
		`"signup_path":"` + portal.PortalSignupPath + `"`,
		"Enter the commercial email address for your Pulse account.",
	} {
		if !strings.Contains(unauthRec.Body.String(), needle) {
			t.Fatalf("expected unauthenticated portal page to contain %q, body=%q", needle, unauthRec.Body.String())
		}
	}

	authReq := httptest.NewRequest(http.MethodGet, portal.PortalPagePath, nil)
	authReq.Header.Set("Authorization", "Bearer "+token)
	authRec := httptest.NewRecorder()
	mux.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want %d (body=%q)", authRec.Code, http.StatusOK, authRec.Body.String())
	}
	for _, needle := range []string{
		"Pulse Account",
		"Acme MSP",
		`id="pulse-account-bootstrap"`,
		`id="portal-app-root"`,
		`"authenticated":true`,
		"Hosted operations",
		"Account services",
		"Self-hosted licenses and billing",
	} {
		if !strings.Contains(authRec.Body.String(), needle) {
			t.Fatalf("expected authenticated portal page to contain %q, body=%q", needle, authRec.Body.String())
		}
	}
}
