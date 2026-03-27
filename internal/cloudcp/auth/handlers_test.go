package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestHandleMagicLinkVerifyPortalTargetCreatesSessionAndRedirectsToPortal(t *testing.T) {
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

	token, err := svc.GeneratePortalToken("buyer@example.com", "")
	if err != nil {
		t.Fatalf("GeneratePortalToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token="+token, nil)
	rec := httptest.NewRecorder()

	HandleMagicLinkVerify(svc, reg, dir, "cloud.example.com", "/portal")(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/portal" {
		t.Fatalf("location=%q, want /portal", got)
	}
	foundSessionCookie := false
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == SessionCookieName && cookie.Value != "" {
			foundSessionCookie = true
			break
		}
	}
	if !foundSessionCookie {
		t.Fatalf("expected %s cookie to be set", SessionCookieName)
	}

	user, err := reg.GetUserByEmail("buyer@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user == nil {
		t.Fatal("expected control-plane user to be created")
	}
}

func TestHandleMagicLinkVerifyInvalidBrowserRedirectsToPortal(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/verify?token=ml1_invalid", nil)
	rec := httptest.NewRecorder()

	HandleMagicLinkVerify(svc, reg, dir, "cloud.example.com", "/portal")(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/portal" {
		t.Fatalf("location=%q, want /portal", got)
	}
}
