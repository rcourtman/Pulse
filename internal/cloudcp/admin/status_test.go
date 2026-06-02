package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func TestHandleHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	HandleHealthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestHandleReadyz(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleReadyz(reg)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ready" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ready")
	}
}

func TestHandleReadyzNilRegistry(t *testing.T) {
	handler := HandleReadyz(nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Body.String() != "not ready" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "not ready")
	}
}

func TestHandleStatus(t *testing.T) {
	reg := newTestRegistry(t)

	// Seed data
	if err := reg.Create(&registry.Tenant{
		ID: "t-STATUS001", State: registry.TenantStateActive, HealthCheckOK: true,
	}); err != nil {
		t.Fatal(err)
	}

	handler := HandleStatus(reg, "test-version")
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Version != "test-version" {
		t.Errorf("version = %v, want test-version", resp.Version)
	}
	if resp.TotalTenants != 1 {
		t.Errorf("total_tenants = %v, want 1", resp.TotalTenants)
	}
}

func TestHandleStatusWithRuntimeIncludesProviderMSPMode(t *testing.T) {
	reg := newTestRegistry(t)

	handler := HandleStatusWithRuntime(reg, "test-version", RuntimeStatus{
		ControlPlaneMode:          "provider_hosted_msp",
		ProviderMSPPlanVersion:    "msp_growth",
		ProviderMSPPlanSource:     "license_file",
		ProviderMSPWorkspaceLimit: 15,
	})
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ControlPlaneMode != "provider_hosted_msp" {
		t.Fatalf("ControlPlaneMode = %q, want provider_hosted_msp", resp.ControlPlaneMode)
	}
	if resp.ProviderMSPPlanVersion != "msp_growth" {
		t.Fatalf("ProviderMSPPlanVersion = %q, want msp_growth", resp.ProviderMSPPlanVersion)
	}
	if resp.ProviderMSPPlanSource != "license_file" {
		t.Fatalf("ProviderMSPPlanSource = %q, want license_file", resp.ProviderMSPPlanSource)
	}
	if resp.ProviderMSPWorkspaceLimit != 15 {
		t.Fatalf("ProviderMSPWorkspaceLimit = %d, want 15", resp.ProviderMSPWorkspaceLimit)
	}
}

func TestHandleStatusNilRegistry(t *testing.T) {
	handler := HandleStatus(nil, "test-version")
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "service unavailable") {
		t.Fatalf("body = %q, want contains %q", rec.Body.String(), "service unavailable")
	}
}

func TestAdminKeyMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authorized"))
	})

	handler := AdminKeyMiddleware("secret-key", inner)

	t.Run("missing key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("X-Admin-Key", "wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("correct X-Admin-Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("X-Admin-Key", "secret-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("injects owner role when missing", func(t *testing.T) {
		roleSeen := ""
		inspect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleSeen = r.Header.Get("X-User-Role")
			w.WriteHeader(http.StatusOK)
		})
		h := AdminKeyMiddleware("secret-key", inspect)

		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("X-Admin-Key", "secret-key")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if roleSeen != string(registry.MemberRoleOwner) {
			t.Fatalf("X-User-Role = %q, want %q", roleSeen, registry.MemberRoleOwner)
		}
	})

	t.Run("overrides explicit caller role header", func(t *testing.T) {
		roleSeen := ""
		inspect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleSeen = r.Header.Get("X-User-Role")
			w.WriteHeader(http.StatusOK)
		})
		h := AdminKeyMiddleware("secret-key", inspect)

		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("X-Admin-Key", "secret-key")
		req.Header.Set("X-User-Role", string(registry.MemberRoleAdmin))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if roleSeen != string(registry.MemberRoleOwner) {
			t.Fatalf("X-User-Role = %q, want %q", roleSeen, registry.MemberRoleOwner)
		}
	})

	t.Run("correct Bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("Authorization", "Bearer secret-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("trimmed X-Admin-Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("X-Admin-Key", "  secret-key  ")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}
