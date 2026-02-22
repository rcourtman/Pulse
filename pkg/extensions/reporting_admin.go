package extensions

import "net/http"

// ReportingAdminEndpoints defines the enterprise reporting admin endpoint surface.
type ReportingAdminEndpoints interface {
	HandleGenerateReport(http.ResponseWriter, *http.Request)
	HandleGenerateMultiReport(http.ResponseWriter, *http.Request)
}

// ReportingAdminRuntime exposes runtime capabilities needed by reporting admin endpoints.
// Reserved for private implementations that need additional host services.
type ReportingAdminRuntime struct{}

// BindReportingAdminEndpointsFunc allows enterprise modules to bind replacement
// reporting admin endpoints while retaining access to default handlers.
type BindReportingAdminEndpointsFunc func(defaults ReportingAdminEndpoints, runtime ReportingAdminRuntime) ReportingAdminEndpoints
