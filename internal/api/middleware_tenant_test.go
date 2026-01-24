package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/stretchr/testify/assert"
)

// ensure TenantMiddleware satisfies the interface or logic we expect
// We test the http.Handler behavior

// mockLicenseProvider is a local mock for this test package to avoid collisions
type mockLicenseProvider struct {
	hasFeatures bool
}

func (p *mockLicenseProvider) Service(ctx context.Context) *license.Service {
	// In a real scenario, we'd return a mocked service control structure.
	// Since license.Service is concrete, we rely on its default state (no features)
	// or we'd need a way to inject state.
	// For now, testing the negative case (no license) is most important for security.
	return license.NewService()
}

func TestTenantMiddleware_Enforcement_Permanent(t *testing.T) {
	// Cleanup env after test
	defer func() {
		os.Unsetenv("PULSE_MULTI_TENANT_ENABLED")
		SetMultiTenantEnabled(false) // Reset global state
	}()

	tests := []struct {
		name           string
		orgID          string
		flagEnabled    bool
		expectedStatus int
	}{
		{
			name:           "Default Org - Always Allowed",
			orgID:          "default",
			flagEnabled:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Default Org - Flag Enabled - Allowed",
			orgID:          "default",
			flagEnabled:    true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Tenant - Flag Disabled - 501",
			orgID:          "acme-corp",
			flagEnabled:    false,
			expectedStatus: http.StatusNotImplemented,
		},
		{
			name:           "Tenant - Flag Enabled - No License - 402",
			orgID:          "acme-corp",
			flagEnabled:    true,
			expectedStatus: http.StatusPaymentRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMultiTenantEnabled(tt.flagEnabled)

			// Use nil persistence for this test as we aren't testing org existence check here,
			// or we assume it passes/skips if nil.
			// Looking at middleware_tenant.go: "if m.persistence != nil { ... }"
			// So nil makes it skip existence check, which is fine for testing flag/license logic.

			mw := NewTenantMiddleware(nil)

			// Create a handler that uses the middleware
			handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			if tt.orgID != "" {
				req.Header.Set("X-Pulse-Org-ID", tt.orgID)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
