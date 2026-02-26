package extensions

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

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

	// Approval listing
	HandleListApprovals(http.ResponseWriter, *http.Request)
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
	// Handlers that have been moved to enterprise are nil.
	CoreHandlers AIAutoFixCoreHandlers

	// HandlerDeps provides dependency callbacks for enterprise handler implementations.
	// Enterprise handlers call these instead of accessing internal packages directly.
	HandlerDeps AIAutoFixHandlerDeps
}

// AIAutoFixCoreHandlers contains function references to the core handler implementations
// in internal/api. These are populated at route registration time and allow enterprise
// binders to delegate to the real handlers without importing internal packages.
type AIAutoFixCoreHandlers struct {
	// All handlers moved to enterprise — these fields are nil.
	HandleReinvestigateFinding      http.HandlerFunc
	HandleReapproveInvestigationFix http.HandlerFunc
	HandleUpdatePatrolAutonomy      http.HandlerFunc
	HandleApproveInvestigationFix   http.HandlerFunc
	HandleListApprovals             http.HandlerFunc
	HandleGetRemediationPlans       http.HandlerFunc
	HandleGetRemediationPlan        http.HandlerFunc
	HandleApproveRemediationPlan    http.HandlerFunc
	HandleExecuteRemediationPlan    http.HandlerFunc
	HandleRollbackRemediationPlan   http.HandlerFunc
}

// AIAutoFixHandlerDeps provides dependency callbacks for enterprise handler
// implementations. These closures wrap OSS internals so enterprise code can
// operate without importing internal/ packages.
type AIAutoFixHandlerDeps struct {
	// Investigation store per org
	GetInvestigationStore func(orgID string) aicontracts.InvestigationStore

	// Approval operations (returns nil when store not initialized)
	Approvals func() aicontracts.ApprovalStoreAccessor

	// Command execution
	MCPExecutor   aicontracts.MCPToolExecutor
	AgentExecutor aicontracts.AgentCommandExecutor

	// Finding/patrol operations
	FindingUpdater     aicontracts.FindingOutcomeUpdater
	FixVerifier        aicontracts.FixVerificationLauncher
	PatrolConfig       func(r *http.Request) aicontracts.PatrolConfigAccessor
	PatrolConfigUpdate func(r *http.Request) aicontracts.PatrolConfigUpdater

	// Orchestrator access (for reinvestigate)
	GetOrchestrator        func(r *http.Request) aicontracts.InvestigationOrchestrator
	SetupOrchestrator      func(orgID string)
	IsInvestigationEnabled func() bool

	// Remediation engine per org
	GetRemediationEngine func(orgID string) aicontracts.RemediationEngine

	// Post-remediation fix verification (background goroutine)
	LaunchRemediationVerification func(ctx context.Context, findingID, executionID string, engine aicontracts.RemediationEngine)

	// Context helpers
	GetOrgID       func(ctx context.Context) string
	NormalizeOrgID func(orgID string) string
	GetUsername    func(r *http.Request) string
	EnsureScope    func(w http.ResponseWriter, r *http.Request, scope string) bool
	AuditLog       func(event, username, ip, path string, success bool, details string)
	GetClientIP    func(r *http.Request) string
}

// BindAIAutoFixEndpointsFunc allows enterprise modules to bind replacement
// AI auto-fix endpoints while retaining access to default handlers.
type BindAIAutoFixEndpointsFunc func(defaults AIAutoFixEndpoints, runtime AIAutoFixRuntime) AIAutoFixEndpoints
