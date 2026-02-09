package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestHostedSignupSuccess(t *testing.T) {
	router, persistence, rbacProvider := newHostedSignupTestRouter(t, true)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","password":"securepass123","org_name":"My Organization"}`)
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
	if response.Message != "Tenant provisioned successfully" {
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
}

func TestHostedSignupValidationFailures(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{
			name: "missing email",
			body: `{"password":"securepass123","org_name":"My Organization"}`,
		},
		{
			name: "invalid email",
			body: `{"email":"userexample.com","password":"securepass123","org_name":"My Organization"}`,
		},
		{
			name: "short password",
			body: `{"email":"user@example.com","password":"short","org_name":"My Organization"}`,
		},
		{
			name: "invalid org_name",
			body: `{"email":"user@example.com","password":"securepass123","org_name":"../evil"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, _ := newHostedSignupTestRouter(t, true)
			rec := doHostedSignupRequest(router, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHostedSignupHostedModeGate(t *testing.T) {
	router, _, _ := newHostedSignupTestRouter(t, false)

	rec := doHostedSignupRequest(router, `{"email":"owner@example.com","password":"securepass123","org_name":"My Organization"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when hosted mode is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedSignupRateLimit(t *testing.T) {
	router, _, _ := newHostedSignupTestRouter(t, true)

	for i := 1; i <= 6; i++ {
		body := fmt.Sprintf(
			`{"email":"user%d@example.com","password":"securepass123","org_name":"Org %d"}`,
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

func newHostedSignupTestRouter(t *testing.T, hostedMode bool) (*Router, *config.MultiTenantPersistence, *TenantRBACProvider) {
	t.Helper()

	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	rbacProvider := NewTenantRBACProvider(baseDir)
	t.Cleanup(func() {
		_ = rbacProvider.Close()
	})

	router := &Router{
		mux:               http.NewServeMux(),
		hostedMode:        hostedMode,
		signupRateLimiter: NewRateLimiter(5, 1*time.Hour),
	}
	t.Cleanup(func() {
		router.signupRateLimiter.Stop()
	})

	hostedSignupHandlers := NewHostedSignupHandlers(persistence, rbacProvider, hostedMode)
	router.registerHostedRoutes(hostedSignupHandlers)

	return router, persistence, rbacProvider
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
