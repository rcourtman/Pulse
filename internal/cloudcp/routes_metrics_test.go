package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestRegisterRoutes_ExposesMetrics(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	// Seed at least one sample so the exposition is deterministic.
	cpmetrics.HealthCheckResults.WithLabelValues("healthy").Inc()
	cpmetrics.TenantsByState.WithLabelValues(string(registry.TenantStateActive)).Set(0)

	mux := http.NewServeMux()
	deps := &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry: reg,
		Version:  "test",
	}
	RegisterRoutes(mux, deps)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "pulse_cp_health_check_results_total") {
		t.Fatalf("expected health check metric in /metrics output")
	}
	if !strings.Contains(body, "pulse_cp_tenants_by_state") {
		t.Fatalf("expected tenant state metric in /metrics output")
	}
}
