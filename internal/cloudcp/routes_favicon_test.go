package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestRegisterRoutes_ControlPlaneFavicons(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	trialStore, err := NewTrialSignupStore(dir)
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { trialStore.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:         reg,
		TrialSignupStore: trialStore,
		Version:          "test",
	})

	svgReq := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	svgRec := httptest.NewRecorder()
	mux.ServeHTTP(svgRec, svgReq)
	if svgRec.Code != http.StatusOK {
		t.Fatalf("GET /favicon.svg status=%d, want %d", svgRec.Code, http.StatusOK)
	}
	if got := svgRec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("GET /favicon.svg content-type=%q, want %q", got, "image/svg+xml")
	}
	if !strings.Contains(svgRec.Body.String(), "<svg") {
		t.Fatalf("expected svg favicon payload")
	}

	icoReq := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	icoRec := httptest.NewRecorder()
	mux.ServeHTTP(icoRec, icoReq)
	if icoRec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /favicon.ico status=%d, want %d", icoRec.Code, http.StatusMovedPermanently)
	}
	if got := icoRec.Header().Get("Location"); got != "/favicon.svg" {
		t.Fatalf("GET /favicon.ico location=%q, want %q", got, "/favicon.svg")
	}
}
