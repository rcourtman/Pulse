package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

type testRBACAdminEndpoints struct {
	integrityCalls int
	resetCalls     int
}

func (t *testRBACAdminEndpoints) HandleIntegrityCheck(http.ResponseWriter, *http.Request) {
	t.integrityCalls++
}

func (t *testRBACAdminEndpoints) HandleAdminReset(http.ResponseWriter, *http.Request) {
	t.resetCalls++
}

func TestResolveRBACAdminEndpoints_DefaultWhenBinderNil(t *testing.T) {
	SetRBACAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetRBACAdminEndpointsBinder(nil)
	})

	defaults := &testRBACAdminEndpoints{}
	resolved := resolveRBACAdminEndpoints(defaults)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/rbac/integrity", nil)
	rec := httptest.NewRecorder()
	resolved.HandleIntegrityCheck(rec, req)
	if defaults.integrityCalls != 1 {
		t.Fatalf("expected default integrity handler call, got %d", defaults.integrityCalls)
	}
}

func TestResolveRBACAdminEndpoints_UsesBinderOverride(t *testing.T) {
	SetRBACAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetRBACAdminEndpointsBinder(nil)
	})

	defaults := &testRBACAdminEndpoints{}
	override := &testRBACAdminEndpoints{}

	SetRBACAdminEndpointsBinder(func(_ extensions.RBACAdminEndpoints) extensions.RBACAdminEndpoints {
		return override
	})

	resolved := resolveRBACAdminEndpoints(defaults)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", nil)
	rec := httptest.NewRecorder()
	resolved.HandleAdminReset(rec, req)

	if defaults.resetCalls != 0 {
		t.Fatalf("expected default reset handler to be bypassed, got %d calls", defaults.resetCalls)
	}
	if override.resetCalls != 1 {
		t.Fatalf("expected override reset handler call, got %d", override.resetCalls)
	}
}
