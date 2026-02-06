package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenantMiddleware_ContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test default if missing
	assert.Equal(t, "default", GetOrgID(ctx))
	assert.Equal(t, "default", GetOrganization(ctx).ID)

	// Test extraction
	orgID := "acme-corp"
	ctx = context.WithValue(ctx, OrgIDContextKey, orgID)
	assert.Equal(t, orgID, GetOrgID(ctx))
}

func TestTenantMiddleware_FullChain(t *testing.T) {
	defer func() {
		SetMultiTenantEnabled(false)
		SetLicenseServiceProvider(nil)
	}()

	tests := []struct {
		name           string
		orgID          string
		flagEnabled    bool
		hasLicense     bool
		expectedStatus int
	}{
		{
			name:           "Tenant - Enabled - Licensed - OK",
			orgID:          "acme",
			flagEnabled:    true,
			hasLicense:     true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Tenant - Enabled - Unlicensed - 402",
			orgID:          "acme",
			flagEnabled:    true,
			hasLicense:     false,
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name:           "Tenant - Disabled - 501",
			orgID:          "acme",
			flagEnabled:    false,
			hasLicense:     true,
			expectedStatus: http.StatusNotImplemented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMultiTenantEnabled(tt.flagEnabled)

			if tt.hasLicense {
				t.Setenv("PULSE_DEV", "true")
			} else {
				t.Setenv("PULSE_DEV", "false")
			}

			mw := NewTenantMiddleware(nil)
			handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Pulse-Org-ID", tt.orgID)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRequireMultiTenant_Middleware(t *testing.T) {
	defer SetMultiTenantEnabled(false)

	mw := RequireMultiTenant(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Default Org - OK even if disabled", func(t *testing.T) {
		SetMultiTenantEnabled(false)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Tenant - Disabled - 501", func(t *testing.T) {
		SetMultiTenantEnabled(false)
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), OrgIDContextKey, "acme")
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req.WithContext(ctx))
		assert.Equal(t, http.StatusNotImplemented, w.Code)
	})

	t.Run("Cookie Extraction", func(t *testing.T) {
		mw := NewTenantMiddleware(nil)
		handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "cookie-org", GetOrgID(r.Context()))
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "pulse_org_id", Value: "cookie-org"})
		handler.ServeHTTP(httptest.NewRecorder(), req)
	})
}

func TestCheckMultiTenantLicense_Variations(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	ctx := context.Background()

	assert.True(t, CheckMultiTenantLicense(ctx, "default"))
	assert.True(t, CheckMultiTenantLicense(ctx, ""))

	SetMultiTenantEnabled(false)
	assert.False(t, CheckMultiTenantLicense(ctx, "acme"))

	SetMultiTenantEnabled(true)
	// Without provider or real license, this falls back to a new service (false)
	assert.False(t, CheckMultiTenantLicense(ctx, "acme"))
}
