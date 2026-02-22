package extensions

import "net/http"

// SSOAdminEndpoints defines the enterprise SSO admin endpoint surface.
type SSOAdminEndpoints interface {
	HandleProvidersCollection(http.ResponseWriter, *http.Request)
	HandleProviderItem(http.ResponseWriter, *http.Request)
	HandleProviderTest(http.ResponseWriter, *http.Request)
	HandleMetadataPreview(http.ResponseWriter, *http.Request)
}

// SSOAdminRuntime exposes runtime capabilities for SSO admin endpoints.
// Reserved for private implementations that need additional host services.
type SSOAdminRuntime struct{}

// BindSSOAdminEndpointsFunc allows enterprise modules to bind replacement
// SSO admin endpoints while retaining access to default handlers.
type BindSSOAdminEndpointsFunc func(defaults SSOAdminEndpoints, runtime SSOAdminRuntime) SSOAdminEndpoints
