package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

type testSSOAdminEndpoints struct {
	providerCalls int
}

func (t *testSSOAdminEndpoints) HandleProvidersCollection(http.ResponseWriter, *http.Request) {
	t.providerCalls++
}
func (t *testSSOAdminEndpoints) HandleProviderItem(http.ResponseWriter, *http.Request)    {}
func (t *testSSOAdminEndpoints) HandleProviderTest(http.ResponseWriter, *http.Request)    {}
func (t *testSSOAdminEndpoints) HandleMetadataPreview(http.ResponseWriter, *http.Request) {}

func TestResolveSSOAdminEndpoints_DefaultWhenBinderNil(t *testing.T) {
	SetSSOAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetSSOAdminEndpointsBinder(nil)
	})

	defaults := &testSSOAdminEndpoints{}
	resolved := resolveSSOAdminEndpoints(defaults, extensions.SSOAdminRuntime{})
	req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers", nil)
	rec := httptest.NewRecorder()
	resolved.HandleProvidersCollection(rec, req)
	if defaults.providerCalls != 1 {
		t.Fatalf("expected default provider handler call, got %d", defaults.providerCalls)
	}
}

func TestResolveSSOAdminEndpoints_UsesBinderOverride(t *testing.T) {
	SetSSOAdminEndpointsBinder(nil)
	t.Cleanup(func() {
		SetSSOAdminEndpointsBinder(nil)
	})

	defaults := &testSSOAdminEndpoints{}
	override := &testSSOAdminEndpoints{}
	SetSSOAdminEndpointsBinder(func(_ extensions.SSOAdminEndpoints, _ extensions.SSOAdminRuntime) extensions.SSOAdminEndpoints {
		return override
	})

	resolved := resolveSSOAdminEndpoints(defaults, extensions.SSOAdminRuntime{})
	req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers", nil)
	rec := httptest.NewRecorder()
	resolved.HandleProvidersCollection(rec, req)

	if defaults.providerCalls != 0 {
		t.Fatalf("expected default provider handler to be bypassed, got %d calls", defaults.providerCalls)
	}
	if override.providerCalls != 1 {
		t.Fatalf("expected override provider handler call, got %d", override.providerCalls)
	}
}
