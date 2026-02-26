package extensions

import "net/http"

// AIAlertAnalysisEndpoints defines the enterprise alert analysis endpoint surface.
// Covers: alert-triggered AI investigation and Kubernetes AI analysis.
// Implementations can replace or decorate the default handlers.
type AIAlertAnalysisEndpoints interface {
	HandleInvestigateAlert(http.ResponseWriter, *http.Request)
	HandleAnalyzeKubernetesCluster(http.ResponseWriter, *http.Request)
}

// AIAlertAnalysisRuntime exposes API/runtime capabilities needed by alert analysis endpoints.
// Implementations are provided by the public server and consumed by enterprise binders.
type AIAlertAnalysisRuntime struct {
	// HasLicenseFeature checks whether a license feature is available for the given request.
	// The *http.Request is used to derive the tenant/org context for multi-tenant license checks.
	HasLicenseFeature    func(*http.Request, string) bool
	WriteLicenseRequired func(http.ResponseWriter, string, string)
	WriteError           func(http.ResponseWriter, int, string, string, map[string]string)

	// CoreHandlers provides access to the core handler implementations.
	CoreHandlers AIAlertAnalysisCoreHandlers
}

// AIAlertAnalysisCoreHandlers contains function references to the core handler implementations.
type AIAlertAnalysisCoreHandlers struct {
	HandleInvestigateAlert         http.HandlerFunc
	HandleAnalyzeKubernetesCluster http.HandlerFunc
}

// BindAIAlertAnalysisEndpointsFunc allows enterprise modules to bind replacement
// alert analysis endpoints while retaining access to default handlers.
type BindAIAlertAnalysisEndpointsFunc func(defaults AIAlertAnalysisEndpoints, runtime AIAlertAnalysisRuntime) AIAlertAnalysisEndpoints
