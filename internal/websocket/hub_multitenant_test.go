package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeMultiTenantChecker struct {
	result MultiTenantCheckResult
	called *bool
}

func (f fakeMultiTenantChecker) CheckMultiTenant(ctx context.Context, orgID string) MultiTenantCheckResult {
	if f.called != nil {
		*f.called = true
	}
	return f.result
}

func TestHandleWebSocket_MultiTenantDisabled(t *testing.T) {
	hub := NewHub(nil)
	hub.SetMultiTenantChecker(fakeMultiTenantChecker{
		result: MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: false,
			Licensed:       false,
			Reason:         "disabled",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}

func TestHandleWebSocket_MultiTenantUnlicensed(t *testing.T) {
	hub := NewHub(nil)
	hub.SetMultiTenantChecker(fakeMultiTenantChecker{
		result: MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: true,
			Licensed:       false,
			Reason:         "unlicensed",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rec.Code)
	}
}

func TestHandleWebSocket_SingleTenantModeIgnoresOrgID(t *testing.T) {
	hub := NewHub(nil)
	hub.SetSingleTenantMode(true)

	called := false
	hub.SetMultiTenantChecker(fakeMultiTenantChecker{
		called: &called,
		result: MultiTenantCheckResult{
			Allowed:        false,
			FeatureEnabled: false,
			Licensed:       false,
			Reason:         "disabled",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-a")
	rec := httptest.NewRecorder()

	hub.HandleWebSocket(rec, req)

	if called {
		t.Fatal("expected single-tenant mode to bypass multi-tenant checker")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected websocket upgrade failure after collapsing org to default, got %d", rec.Code)
	}
}
