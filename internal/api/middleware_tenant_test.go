package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantMiddleware(t *testing.T) {
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
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OrgID: default", rec.Body.String())

		// Verify default directory was created
		_, err := os.Stat(filepath.Join(tmpDir, "orgs", "default"))
		assert.NoError(t, err)
	})

	t.Run("Custom Org (Header)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Pulse-Org-ID", "customer-a")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OrgID: customer-a", rec.Body.String())

		// Verify custom directory was created
		_, err := os.Stat(filepath.Join(tmpDir, "orgs", "customer-a"))
		assert.NoError(t, err)
	})

	t.Run("Invalid Org ID (Directory Traversal Attempt)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Pulse-Org-ID", "../../../etc/passwd")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
