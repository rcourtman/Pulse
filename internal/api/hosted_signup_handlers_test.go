package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestHostedSignupSuccess(t *testing.T) {
	router, persistence, rbacProvider, emailer, _ := newHostedSignupTestRouter(t, true)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		OrgID   string `json:"org_id"`
		UserID  string `json:"user_id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.OrgID == "" {
		t.Fatal("expected org_id to be set")
	}
	if response.UserID != "owner@example.com" {
		t.Fatalf("expected user_id owner@example.com, got %q", response.UserID)
	}
	if response.Message != "Check your email for a magic link to finish signing in." {
		t.Fatalf("unexpected message: %q", response.Message)
	}

	org, err := persistence.LoadOrganization(response.OrgID)
	if err != nil {
		t.Fatalf("load org: %v", err)
	}
	if org.DisplayName != "My Organization" {
		t.Fatalf("expected org display name My Organization, got %q", org.DisplayName)
	}
	if org.OwnerUserID != "owner@example.com" {
		t.Fatalf("expected owner owner@example.com, got %q", org.OwnerUserID)
	}

	manager, err := rbacProvider.GetManager(response.OrgID)
	if err != nil {
		t.Fatalf("get rbac manager: %v", err)
	}
	assignment, ok := manager.GetUserAssignment("owner@example.com")
	if !ok {
		t.Fatal("expected user role assignment to exist")
	}
	if !containsRoleID(assignment.RoleIDs, auth.RoleAdmin) {
		t.Fatalf("expected admin role assignment, got roles: %v", assignment.RoleIDs)
	}

	// Verify a magic link was "sent".
	emailer.mu.Lock()
	defer emailer.mu.Unlock()
	if emailer.calls != 1 {
		t.Fatalf("expected 1 magic link email, got %d", emailer.calls)
	}
	if emailer.to != "owner@example.com" {
		t.Fatalf("magic link to = %q, want owner@example.com", emailer.to)
	}
	if !strings.Contains(emailer.magicLinkURL, "/api/public/magic-link/verify?token=") {
		t.Fatalf("magic link url = %q, expected verify endpoint with token", emailer.magicLinkURL)
	}
}

func TestHostedSignupValidationFailures(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{
			name: "missing email",
			body: `{"org_name":"My Organization"}`,
		},
		{
			name: "invalid email",
			body: `{"email":"userexample.com","org_name":"My Organization"}`,
		},
		{
			name: "invalid org_name",
			body: `{"email":"user@example.com","org_name":"../evil"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, _, _, _ := newHostedSignupTestRouter(t, true)
			rec := doHostedSignupRequest(router, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHostedSignupHostedModeGate(t *testing.T) {
	router, _, _, _, _ := newHostedSignupTestRouter(t, false)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when hosted mode is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedSignupRateLimit(t *testing.T) {
	router, _, _, _, _ := newHostedSignupTestRouter(t, true)

	for i := 1; i <= 6; i++ {
		body := fmt.Sprintf(
			`{"email":"user%d@example.com","org_name":"Org %d"}`,
			i,
			i,
		)
		rec := doHostedSignupRequest(router, body)
		if i <= 5 && rec.Code != http.StatusCreated {
			t.Fatalf("request %d expected 201, got %d: %s", i, rec.Code, rec.Body.String())
		}
		if i == 6 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d expected 429, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}
}

func TestHostedSignupCleanupOnRBACFailure(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	realRBAC := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = realRBAC.Close()
	})

	wrapped := &failingRBACProvider{
		inner:      realRBAC,
		failUpdate: true,
	}

	router, _ := newHostedSignupTestRouterWithDeps(t, true, persistence, wrapped)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if wrapped.removeCalls == 0 {
		t.Fatalf("expected RBAC provider RemoveTenant to be called during cleanup")
	}

	// Cleanup should remove any partially provisioned org directories.
	orgsDir := baseDir + "/orgs"
	entries, err := os.ReadDir(orgsDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read orgs dir: %v", err)
	}
	if err == nil && len(entries) != 0 {
		t.Fatalf("expected no org directories after cleanup; found %d", len(entries))
	}
}

func newHostedSignupTestRouter(t *testing.T, hostedMode bool) (*Router, *config.MultiTenantPersistence, *TenantRBACProvider, *captureMagicLinkEmailer, string) {
	t.Helper()

	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	rbacProvider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = rbacProvider.Close()
	})

	router, emailer := newHostedSignupTestRouterWithDeps(t, hostedMode, persistence, rbacProvider)
	return router, persistence, rbacProvider, emailer, baseDir
}

func newHostedSignupTestRouterWithDeps(t *testing.T, hostedMode bool, persistence *config.MultiTenantPersistence, rbacProvider HostedRBACProvider) (*Router, *captureMagicLinkEmailer) {
	t.Helper()

	emailer := &captureMagicLinkEmailer{}
	// Use a stable key to avoid flakiness.
	key := []byte("0123456789abcdef0123456789abcdef")
	magicLinks := NewMagicLinkServiceWithKey(key, nil, emailer, NewRateLimiter(1000, 1*time.Hour))
	t.Cleanup(func() { magicLinks.Stop() })

	router := &Router{
		mux:               http.NewServeMux(),
		hostedMode:        hostedMode,
		signupRateLimiter: NewRateLimiter(5, 1*time.Hour),
	}
	t.Cleanup(func() {
		router.signupRateLimiter.Stop()
	})

	hostedSignupHandlers := NewHostedSignupHandlers(persistence, rbacProvider, magicLinks, nil, hostedMode)
	router.registerHostedRoutes(hostedSignupHandlers, nil, nil)

	return router, emailer
}

func doHostedSignupRequest(router *Router, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/public/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)
	return rec
}

func containsRoleID(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type failingRBACProvider struct {
	inner      *TenantRBACProvider
	failUpdate bool

	removeCalls int
}

func (p *failingRBACProvider) GetManager(orgID string) (auth.ExtendedManager, error) {
	manager, err := p.inner.GetManager(orgID)
	if err != nil {
		return nil, err
	}
	if !p.failUpdate {
		return manager, nil
	}
	return failingManager{ExtendedManager: manager}, nil
}

func (p *failingRBACProvider) RemoveTenant(orgID string) error {
	p.removeCalls++
	return p.inner.RemoveTenant(orgID)
}

type failingManager struct {
	auth.ExtendedManager
}

func (m failingManager) UpdateUserRoles(username string, roleIDs []string) error {
	return fmt.Errorf("forced UpdateUserRoles failure")
}

type captureMagicLinkEmailer struct {
	mu           sync.Mutex
	calls        int
	to           string
	magicLinkURL string
}

func (c *captureMagicLinkEmailer) SendMagicLink(to, magicLinkURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.to = to
	c.magicLinkURL = magicLinkURL
	return nil
}
