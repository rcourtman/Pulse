package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

type testAuditAdminEndpoints struct {
	listCalls int
}

func (t *testAuditAdminEndpoints) HandleListEvents(http.ResponseWriter, *http.Request)     { t.listCalls++ }
func (t *testAuditAdminEndpoints) HandleVerifyEvent(http.ResponseWriter, *http.Request)    {}
func (t *testAuditAdminEndpoints) HandleExportEvents(http.ResponseWriter, *http.Request)   {}
func (t *testAuditAdminEndpoints) HandleSummary(http.ResponseWriter, *http.Request)        {}
func (t *testAuditAdminEndpoints) HandleGetWebhooks(http.ResponseWriter, *http.Request)    {}
func (t *testAuditAdminEndpoints) HandleUpdateWebhooks(http.ResponseWriter, *http.Request) {}

func TestResolveAuditAdminEndpoints_DefaultWhenBinderNil(t *testing.T) {
	SetAuditAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetAuditAdminEndpointsBinder(nil)
	})

	defaults := &testAuditAdminEndpoints{}
	resolved := resolveAuditAdminEndpoints(defaults, extensions.AuditAdminRuntime{})

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	resolved.HandleListEvents(rec, req)
	if defaults.listCalls != 1 {
		t.Fatalf("expected default list handler call, got %d", defaults.listCalls)
	}
}

func TestResolveAuditAdminEndpoints_UsesBinderOverride(t *testing.T) {
	SetAuditAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetAuditAdminEndpointsBinder(nil)
	})

	defaults := &testAuditAdminEndpoints{}
	override := &testAuditAdminEndpoints{}
	SetAuditAdminEndpointsBinder(func(_ extensions.AuditAdminEndpoints, _ extensions.AuditAdminRuntime) extensions.AuditAdminEndpoints {
		return override
	})

	resolved := resolveAuditAdminEndpoints(defaults, extensions.AuditAdminRuntime{})
	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	resolved.HandleListEvents(rec, req)

	if defaults.listCalls != 0 {
		t.Fatalf("expected default list handler to be bypassed, got %d calls", defaults.listCalls)
	}
	if override.listCalls != 1 {
		t.Fatalf("expected override list handler call, got %d", override.listCalls)
	}
}
