package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantMiddleware(t *testing.T) {
	prevMultiTenant := IsMultiTenantEnabled()
	t.Cleanup(func() {
		SetMultiTenantEnabled(prevMultiTenant)
		SetLicenseServiceProvider(nil)
	})

	// Setup temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "pulse-tenant-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create MultiTenantPersistence
	mtp := config.NewMultiTenantPersistence(tmpDir)

	// Create middleware
	middleware := NewTenantMiddleware(mtp)

	// Test handler that checks the context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := GetOrgID(r.Context())
		if orgID == "" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("OrgID missing"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OrgID: " + orgID))
	})

	// Wrap handler
	handler := middleware.Middleware(testHandler)

	t.Run("Default Org (No Header)", func(t *testing.T) {
		SetMultiTenantEnabled(false)
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OrgID: default", rec.Body.String())

		// Default org no longer initializes tenant persistence; no directory expectation.
	})

	t.Run("Custom Org (Feature Disabled)", func(t *testing.T) {
		SetMultiTenantEnabled(false)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Pulse-Org-ID", "customer-a")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotImplemented, rec.Code)
		assert.Contains(t, rec.Body.String(), "Multi-tenant functionality is not enabled")
	})

	t.Run("Custom Org (Feature Enabled, Unlicensed)", func(t *testing.T) {
		SetMultiTenantEnabled(true)
		SetLicenseServiceProvider(nil)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Pulse-Org-ID", "customer-a")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusPaymentRequired, rec.Code)
		assert.Contains(t, rec.Body.String(), "Multi-tenant access requires an Enterprise license")
	})

	t.Run("Custom Org (Feature Enabled, Licensed)", func(t *testing.T) {
		SetMultiTenantEnabled(true)
		// Enable license dev mode for test keys
		prevDevMode := os.Getenv("PULSE_LICENSE_DEV_MODE")
		os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
		t.Cleanup(func() {
			if prevDevMode == "" {
				os.Unsetenv("PULSE_LICENSE_DEV_MODE")
			} else {
				os.Setenv("PULSE_LICENSE_DEV_MODE", prevDevMode)
			}
		})
		license.SetPublicKey(nil)

		licenseKey, err := license.GenerateLicenseForTesting("test@example.com", license.TierEnterprise, 24*time.Hour)
		require.NoError(t, err)

		service := license.NewService()
		_, err = service.Activate(licenseKey)
		require.NoError(t, err)

		SetLicenseServiceProvider(staticLicenseProvider{svc: service})

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Pulse-Org-ID", "customer-a")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OrgID: customer-a", rec.Body.String())

		// Verify custom directory was created
		_, err = os.Stat(filepath.Join(tmpDir, "orgs", "customer-a"))
		assert.NoError(t, err)
	})

	t.Run("Invalid Org ID (Directory Traversal Attempt)", func(t *testing.T) {
		SetMultiTenantEnabled(false)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Pulse-Org-ID", "../../../etc/passwd")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

type staticLicenseProvider struct {
	svc *license.Service
}

func (p staticLicenseProvider) Service(ctx context.Context) *license.Service {
	return p.svc
}
