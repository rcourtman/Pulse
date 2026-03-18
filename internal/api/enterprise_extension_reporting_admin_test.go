package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

type testReportingAdminEndpoints struct {
	generateCalls int
}

func (t *testReportingAdminEndpoints) HandleGenerateReport(http.ResponseWriter, *http.Request) {
	t.generateCalls++
}

func (t *testReportingAdminEndpoints) HandleGenerateMultiReport(http.ResponseWriter, *http.Request) {}

func TestResolveReportingAdminEndpoints_DefaultWhenBinderNil(t *testing.T) {
	SetReportingAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetReportingAdminEndpointsBinder(nil)
	})

	defaults := &testReportingAdminEndpoints{}
	resolved := resolveReportingAdminEndpoints(defaults, extensions.ReportingAdminRuntime{})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/generate", nil)
	rec := httptest.NewRecorder()
	resolved.HandleGenerateReport(rec, req)
	if defaults.generateCalls != 1 {
		t.Fatalf("expected default reporting handler call, got %d", defaults.generateCalls)
	}
}

func TestResolveReportingAdminEndpoints_UsesBinderOverride(t *testing.T) {
	SetReportingAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetReportingAdminEndpointsBinder(nil)
	})

	defaults := &testReportingAdminEndpoints{}
	override := &testReportingAdminEndpoints{}
	SetReportingAdminEndpointsBinder(func(_ extensions.ReportingAdminEndpoints, _ extensions.ReportingAdminRuntime) extensions.ReportingAdminEndpoints {
		return override
	})

	resolved := resolveReportingAdminEndpoints(defaults, extensions.ReportingAdminRuntime{})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/generate", nil)
	rec := httptest.NewRecorder()
	resolved.HandleGenerateReport(rec, req)

	if defaults.generateCalls != 0 {
		t.Fatalf("expected default reporting handler to be bypassed, got %d calls", defaults.generateCalls)
	}
	if override.generateCalls != 1 {
		t.Fatalf("expected override reporting handler call, got %d", override.generateCalls)
	}
}
