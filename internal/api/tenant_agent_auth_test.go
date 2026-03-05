package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestedOrgID_V5SingleTenantModeIgnoresHostAgentHeader(t *testing.T) {
	prevSingleTenant := isV5SingleTenantMode()
	setV5SingleTenantModeForTests(true)
	t.Cleanup(func() { setV5SingleTenantModeForTests(prevSingleTenant) })

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/lookup?hostname=missing-host", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme")

	if got := requestedOrgID(req); got != "default" {
		t.Fatalf("expected host agent path to collapse to default org in single-tenant mode, got %q", got)
	}
}
