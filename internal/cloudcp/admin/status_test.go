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

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["version"] != "test-version" {
		t.Errorf("version = %v, want test-version", resp["version"])
	}
	if resp["total_tenants"] != float64(1) {
		t.Errorf("total_tenants = %v, want 1", resp["total_tenants"])
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

	t.Run("correct Bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/tenants", nil)
		req.Header.Set("Authorization", "Bearer secret-key")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}
