package extensions

import "net/http"

// AIAutoFixEndpoints defines the enterprise AI auto-fix endpoint surface.
// Covers: investigation, remediation, autonomy control, and fix execution.
// Implementations can replace or decorate the default handlers.
type AIAutoFixEndpoints interface {
	// Investigation
	HandleReinvestigateFinding(http.ResponseWriter, *http.Request)
	HandleReapproveInvestigationFix(http.ResponseWriter, *http.Request)

	// Autonomy mutation (GET stays in core — always returns current level)
	HandleUpdatePatrolAutonomy(http.ResponseWriter, *http.Request)

	// Remediation
	HandleGetRemediationPlans(http.ResponseWriter, *http.Request)
	HandleGetRemediationPlan(http.ResponseWriter, *http.Request)
	HandleApproveRemediationPlan(http.ResponseWriter, *http.Request)
	HandleExecuteRemediationPlan(http.ResponseWriter, *http.Request)
	HandleRollbackRemediationPlan(http.ResponseWriter, *http.Request)

	// Fix execution (from approval flow)
	HandleApproveInvestigationFix(http.ResponseWriter, *http.Request)
}

// AIAutoFixRuntime exposes API/runtime capabilities needed by auto-fix endpoints.
// Implementations are provided by the public server and consumed by enterprise binders.
type AIAutoFixRuntime struct {
	// HasLicenseFeature checks whether a license feature is available for the given request.
	// The *http.Request is used to derive the tenant/org context for multi-tenant license checks.
	HasLicenseFeature    func(*http.Request, string) bool
	WriteLicenseRequired func(http.ResponseWriter, string, string)
	WriteError           func(http.ResponseWriter, int, string, string, map[string]string)

	// CoreHandlers provides access to the core handler implementations.
	// Enterprise binders use these to delegate to the real handlers after license checks.
	CoreHandlers AIAutoFixCoreHandlers
}

// AIAutoFixCoreHandlers contains function references to the core handler implementations
// in internal/api. These are populated at route registration time and allow enterprise
// binders to delegate to the real handlers without importing internal packages.
type AIAutoFixCoreHandlers struct {
	HandleReinvestigateFinding      http.HandlerFunc
	HandleReapproveInvestigationFix http.HandlerFunc
	HandleUpdatePatrolAutonomy      http.HandlerFunc
	HandleGetRemediationPlans       http.HandlerFunc
	HandleGetRemediationPlan        http.HandlerFunc
	HandleApproveRemediationPlan    http.HandlerFunc
	HandleExecuteRemediationPlan    http.HandlerFunc
	HandleRollbackRemediationPlan   http.HandlerFunc
	HandleApproveInvestigationFix   http.HandlerFunc
}

// BindAIAutoFixEndpointsFunc allows enterprise modules to bind replacement
// AI auto-fix endpoints while retaining access to default handlers.
type BindAIAutoFixEndpointsFunc func(defaults AIAutoFixEndpoints, runtime AIAutoFixRuntime) AIAutoFixEndpoints
