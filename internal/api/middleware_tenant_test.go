package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func setupTenantMiddlewareStatusTest(t *testing.T, org *models.Organization) *TenantMiddleware {
	t.Helper()

	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })
	t.Setenv("PULSE_DEV", "true")

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if org != nil {
		if err := mtp.SaveOrganization(org); err != nil {
			t.Fatalf("save organization: %v", err)
		}
	}

	return NewTenantMiddleware(mtp)
}

func decodeErrorPayload(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response payload: %v", err)
	}
	return payload
}

func TestTenantMiddleware_SuspendGateSuspendedOrgBlocked(t *testing.T) {
	mw := setupTenantMiddlewareStatusTest(t, &models.Organization{
		ID:          "acme-suspended",
		DisplayName: "Acme Suspended",
		Status:      models.OrgStatusSuspended,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme-suspended")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
	payload := decodeErrorPayload(t, rec)
	if payload["error"] != "org_suspended" {
		t.Fatalf("expected org_suspended error code, got %q", payload["error"])
	}
	if payload["message"] != "Organization is suspended" {
		t.Fatalf("unexpected message: %q", payload["message"])
	}
}

func TestTenantMiddleware_SuspendGatePendingDeletionOrgBlocked(t *testing.T) {
	mw := setupTenantMiddlewareStatusTest(t, &models.Organization{
		ID:          "acme-pending-delete",
		DisplayName: "Acme Pending Delete",
		Status:      models.OrgStatusPendingDeletion,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme-pending-delete")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
	payload := decodeErrorPayload(t, rec)
	if payload["error"] != "org_suspended" {
		t.Fatalf("expected org_suspended error code, got %q", payload["error"])
	}
	if payload["message"] != "Organization is suspended" {
		t.Fatalf("unexpected message: %q", payload["message"])
	}
}

func TestTenantMiddleware_SuspendGateActiveOrgAllowed(t *testing.T) {
	mw := setupTenantMiddlewareStatusTest(t, &models.Organization{
		ID:          "acme-active",
		DisplayName: "Acme Active Org",
		Status:      models.OrgStatusActive,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme-active")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org := GetOrganization(r.Context())
		if org == nil {
			t.Fatalf("expected organization in context")
		}
		if org.DisplayName != "Acme Active Org" {
			t.Fatalf("expected loaded organization display name, got %q", org.DisplayName)
		}
		if models.NormalizeOrgStatus(org.Status) != models.OrgStatusActive {
			t.Fatalf("expected active status in context, got %q", org.Status)
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTenantMiddleware_SuspendGateDefaultOrgExempt(t *testing.T) {
	mw := setupTenantMiddlewareStatusTest(t, &models.Organization{
		ID:          "default",
		DisplayName: "Default Suspended",
		Status:      models.OrgStatusSuspended,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := GetOrgID(r.Context()); got != "default" {
			t.Fatalf("expected default org id, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestTenantMiddleware_SuspendGateEmptyStatusTreatedAsActive(t *testing.T) {
	mw := setupTenantMiddlewareStatusTest(t, &models.Organization{
		ID:          "acme-legacy",
		DisplayName: "Acme Legacy Org",
		Status:      "",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme-legacy")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org := GetOrganization(r.Context())
		if org == nil {
			t.Fatalf("expected organization in context")
		}
		if org.DisplayName != "Acme Legacy Org" {
			t.Fatalf("expected loaded organization display name, got %q", org.DisplayName)
		}
		if models.NormalizeOrgStatus(org.Status) != models.OrgStatusActive {
			t.Fatalf("expected normalized active status, got %q", org.Status)
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}
