package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestCPSecurityHeaders_NoStoreBeforeResponse(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusFound, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("Location", "/portal")
				http.SetCookie(w, &http.Cookie{Name: "fixture", Value: "synthetic", HttpOnly: true, Secure: true})
				w.WriteHeader(status)
				_, _ = w.Write([]byte("synthetic response"))
			})
			req := httptest.NewRequest(http.MethodGet, "/portal", nil)
			rec := httptest.NewRecorder()
			CPSecurityHeaders(inner).ServeHTTP(rec, req)
			resp := rec.Result()
			defer resp.Body.Close()
			// Result snapshots headers at the first write, not after the body.
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("committed Cache-Control = %q, want no-store", got)
			}
			if resp.StatusCode != status || rec.Body.String() != "synthetic response" || resp.Header.Get("Location") != "/portal" || resp.Header.Get("Content-Type") != "text/plain" {
				t.Fatal("cache policy changed downstream response")
			}
			cookies := resp.Cookies()
			if len(cookies) != 1 || cookies[0].Value != "synthetic" || !cookies[0].HttpOnly || !cookies[0].Secure {
				t.Fatal("cache policy changed downstream cookie")
			}
		})
	}
}

func TestCPSecurityHeaders_NoStorePortalRoutes(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Cache policy fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "owner@example.invalid"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}
	svc, err := cpauth.NewService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	token, err := svc.GenerateSessionToken(userID, "owner@example.invalid", cpauth.SessionTTL)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config:   &CPConfig{DataDir: dir, AdminKey: "synthetic-admin-key", BaseURL: "https://cloud.example.invalid", ControlPlaneMode: ControlPlaneModeProviderHostedMSP},
		Registry: reg, MagicLinks: svc, Version: "test",
	})
	handler := CPSecurityHeaders(mux)
	read := func(t *testing.T, path, credential string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if credential != "" {
			req.AddCookie(&http.Cookie{Name: cpauth.SessionCookieName, Value: credential})
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		resp := rec.Result()
		defer resp.Body.Close()
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET %s committed Cache-Control = %q, want no-store", path, got)
		}
		return rec
	}
	paths := []string{portal.PortalPagePath, portal.PortalBootstrapPath, portal.PortalDashboardPath + "?account_id=" + accountID, "/api/accounts/" + accountID + "/members"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := read(t, path, token)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "owner@example.invalid") && !strings.Contains(rec.Body.String(), "Cache policy fixture") {
				t.Fatal("fixture did not exercise a personal account response")
			}
		})
	}
	for _, path := range paths[1:] {
		t.Run("unauthenticated"+path, func(t *testing.T) {
			rec := read(t, path, "")
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "owner@example.invalid") {
				t.Fatal("unauthenticated response contains account data")
			}
		})
	}
	if _, err := reg.RevokeUserSessions(userID); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		t.Run("revoked"+path, func(t *testing.T) {
			rec := read(t, path, token)
			want := http.StatusUnauthorized
			if path == portal.PortalPagePath {
				want = http.StatusOK
			}
			if rec.Code != want {
				t.Fatalf("status = %d, want %d", rec.Code, want)
			}
			if strings.Contains(rec.Body.String(), "owner@example.invalid") || strings.Contains(rec.Body.String(), "Cache policy fixture") {
				t.Fatal("revoked response contains account data")
			}
		})
	}
}

func TestCPSecurityHeaders_PreservesPublicAssetCache(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/favicon.svg", nil)
			rec := httptest.NewRecorder()
			CPSecurityHeaders(http.HandlerFunc(handleControlPlaneFaviconSVG)).ServeHTTP(rec, req)
			resp := rec.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "public, max-age=86400" || resp.Header.Get("Content-Type") != "image/svg+xml" {
				t.Fatal("public favicon cache contract changed")
			}
			if method == http.MethodGet && rec.Body.String() != string(controlPlaneFaviconSVG) {
				t.Fatal("favicon bytes changed")
			}
			if method == http.MethodHead && rec.Body.Len() != 0 {
				t.Fatal("HEAD wrote favicon bytes")
			}
		})
	}
}
