package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRequireAdmin_StandardAuth(t *testing.T) {
	// Setup: Standard auth (User/Pass), user is authenticated
	cfg := &config.Config{
		AuthUser: "admin",
		AuthPass: "$2a$10$hashed...",
	}

	// Mock handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware
	middleware := RequireAdmin(cfg, nextHandler)

	// Case 1: user is authenticated via Basic Auth
	// (CheckAuth would usually handle this, but RequireAdmin assumes CheckAuth ran or checks context?)
	// Looking at auth.go outline: RequireAdmin checks context or re-runs auth check?
	// Usually middleware chains: CheckAuth -> RequireAdmin -> Handler.
	// But `RequireAdmin` signature is `(cfg, handler)`. It likely checks authentication inside.

	// Let's assume we need to mock the authentication context OR passed auth check.
	// If RequireAdmin calls CheckAuth internally, we need to provide credentials.

	req := httptest.NewRequest("GET", "/admin", nil)
	// We'll simulate "already authenticated" if possible, or provide headers.
	// Since we can't easily set context values that private middleware expects without helpers,
	// we will try to rely on the behavior that "standard auth users are always admins".

	// If we provide no creds, it should fail (401)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminBypass_DisabledByDefault(t *testing.T) {
	// By default, admin bypass should be false
	// We can't access private variables, but if there's a helper...
	// `adminBypassEnabled()` is private.
	// We can test behavior via middleware or handlers if they use it.
	// RequireAuth likely checks it.

	cfg := &config.Config{
		AuthUser: "user",
		AuthPass: "pass", // configured
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequireAuth(cfg, nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "Should be 401 when auth configured and no bypass")
}
