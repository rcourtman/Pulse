package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type mockAuthorizer struct {
	allowed bool
	err     error
}

func (m *mockAuthorizer) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	return m.allowed, m.err
}

type mockAuthorizerFn struct {
	fn func(ctx context.Context, action string, resource string) (bool, error)
}

func (m *mockAuthorizerFn) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	return m.fn(ctx, action, resource)
}

func TestRequirePermission(t *testing.T) {
	cfg := &config.Config{} // Basic empty config

	t.Run("Allowed", func(t *testing.T) {
		var capturedSubject string
		authMock := &mockAuthorizer{
			allowed: true,
			err:     nil,
		}
		// Custom mock to capture subject
		customAuth := func(ctx context.Context, action string, resource string) (bool, error) {
			capturedSubject = auth.GetUser(ctx)
			return authMock.Authorize(ctx, action, resource)
		}

		handler := RequirePermission(cfg, &mockAuthorizerFn{fn: customAuth}, "read", "logs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/api/test", nil)
		// Mock authentication bypass or setup
		// Since CheckAuth is internal, we might need a way to mock it or set up what it expects.
		// For this test, let's assume CheckAuth passes if no auth is configured.

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", rr.Code)
		}

		if capturedSubject != "anonymous" {
			t.Errorf("Expected subject 'anonymous', got %q", capturedSubject)
		}
	})

	t.Run("Denied", func(t *testing.T) {
		auth := &mockAuthorizer{allowed: false}
		handler := RequirePermission(cfg, auth, "write", "settings", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("POST", "/api/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected status Forbidden, got %d", rr.Code)
		}
	})

	t.Run("EnterpriseAnonymousRefusal", func(t *testing.T) {
		// Simulate Enterprise logic: deny "anonymous"
		enterpriseLogic := func(ctx context.Context, action string, resource string) (bool, error) {
			username := auth.GetUser(ctx)
			if username == "anonymous" {
				return false, nil
			}
			return true, nil
		}

		handler := RequirePermission(cfg, &mockAuthorizerFn{fn: enterpriseLogic}, "read", "nodes", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// req with no auth -> CheckAuth sets "anonymous"
		req := httptest.NewRequest("GET", "/api/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected status Forbidden for anonymous Enterprise access, got %d", rr.Code)
		}
	})

	t.Run("APITokenPrincipal", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.AuthUser = "admin"
		cfg.AuthPass = "password"

		// Setup a token
		rawToken := "valid-token"
		record, _ := config.NewAPITokenRecord(rawToken, "My Token", nil)
		cfg.APITokens = append(cfg.APITokens, *record)

		var capturedSubject string
		customAuth := func(ctx context.Context, action string, resource string) (bool, error) {
			capturedSubject = auth.GetUser(ctx)
			return true, nil
		}

		handler := RequirePermission(cfg, &mockAuthorizerFn{fn: customAuth}, "read", "logs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("X-API-Token", rawToken)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", rr.Code)
		}

		expectedPrincipal := "token:" + record.ID
		if capturedSubject != expectedPrincipal {
			t.Errorf("Expected principal %q, got %q", expectedPrincipal, capturedSubject)
		}
	})

	t.Run("APITokenMissingIDFallback", func(t *testing.T) {
		cfg := &config.Config{}
		// Setup a token without an ID
		rawToken := "legacy-token"
		record, _ := config.NewAPITokenRecord(rawToken, "Legacy", nil)
		record.ID = "" // Explicitly clear ID
		cfg.APITokens = append(cfg.APITokens, *record)

		var capturedSubject string
		customAuth := func(ctx context.Context, action string, resource string) (bool, error) {
			capturedSubject = auth.GetUser(ctx)
			return true, nil
		}

		handler := RequirePermission(cfg, &mockAuthorizerFn{fn: customAuth}, "read", "logs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("X-API-Token", rawToken)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if !strings.HasPrefix(capturedSubject, "token:legacy-") {
			t.Errorf("Expected fallback legacy principal, got %q", capturedSubject)
		}
	})

	t.Run("APITokenAdminAccess", func(t *testing.T) {
		cfg := &config.Config{}
		// Setup a valid token in config
		rawToken := "admin-token"
		record, _ := config.NewAPITokenRecord(rawToken, "Admin Token", nil)
		cfg.APITokens = append(cfg.APITokens, *record)

		// Mock Enterprise authorizer that grants tokens admin access
		eAuth := func(ctx context.Context, action string, resource string) (bool, error) {
			user := auth.GetUser(ctx)
			// Match our new stable principal format
			if strings.HasPrefix(user, "token:") {
				return true, nil
			}
			return false, nil
		}

		handler := RequirePermission(cfg, &mockAuthorizerFn{fn: eAuth}, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("POST", "/api/security/tokens", nil)
		req.Header.Set("X-API-Token", rawToken)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected token to have admin access to users, got %d", rr.Code)
		}
	})

	t.Run("AdminSync", func(t *testing.T) {
		// 1. Verify sync on non-empty
		e := &enterpriseMock{adminUser: "initial"}
		mock := &enterpriseAuthorizerWithSync{e}
		auth.SetAuthorizer(mock)
		defer auth.SetAuthorizer(&auth.DefaultAuthorizer{})

		auth.SetAdminUser("superadmin")
		if e.adminUser != "superadmin" {
			t.Errorf("Expected admin sub-system synced to 'superadmin', got %q", e.adminUser)
		}

		// 2. Verify skip on empty
		auth.SetAdminUser("")
		if e.adminUser != "superadmin" {
			t.Error("Expected empty admin sync to be skipped to preserve configuration")
		}
	})
}

// Support types for TestRequirePermission
type enterpriseMock struct {
	adminUser string
}

type enterpriseAuthorizerWithSync struct {
	*enterpriseMock
}

func (e *enterpriseAuthorizerWithSync) SetAdminUser(u string) { e.adminUser = u }
func (e *enterpriseAuthorizerWithSync) Authorize(ctx context.Context, action string, resource string) (bool, error) {
	return auth.GetUser(ctx) == e.adminUser, nil
}
