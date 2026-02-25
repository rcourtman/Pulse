package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	featureKubernetesAIKey = "kubernetes_ai"
	featureAIAlertsKey     = "ai_alerts"
	featureRelayKey        = "relay"
	featureAIAutoFixKey    = "ai_autofix"
)

func (r *Router) registerAIRelayRoutesGroup() {
	r.mux.HandleFunc("/api/settings/ai", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceSettings, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleGetAISettings)))
	r.mux.HandleFunc("/api/settings/ai/update", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleUpdateAISettings)))
	r.mux.HandleFunc("/api/ai/test", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestAIConnection)))
	r.mux.HandleFunc("/api/ai/test/{provider}", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestProvider)))
	// AI models list - require ai:chat scope (needed to select a model for chat)
	r.mux.HandleFunc("/api/ai/models", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleListModels)))
	r.mux.HandleFunc("/api/ai/execute", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecute)))
	r.mux.HandleFunc("/api/ai/execute/stream", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecuteStream)))
	r.mux.HandleFunc("/api/ai/kubernetes/analyze", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureKubernetesAIKey, r.aiSettingsHandler.HandleAnalyzeKubernetesCluster))))
	r.mux.HandleFunc("/api/ai/investigate-alert", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureAIAlertsKey, r.aiSettingsHandler.HandleInvestigateAlert))))

	r.mux.HandleFunc("/api/ai/run-command", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleRunCommand)))
	// SECURITY: AI Knowledge endpoints require ai:chat scope to prevent arbitrary guest data access
	r.mux.HandleFunc("/api/ai/knowledge", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleGetGuestKnowledge)))
	r.mux.HandleFunc("/api/ai/knowledge/save", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleSaveGuestNote)))
	r.mux.HandleFunc("/api/ai/knowledge/delete", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleDeleteGuestNote)))
	r.mux.HandleFunc("/api/ai/knowledge/export", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleExportGuestKnowledge)))
	r.mux.HandleFunc("/api/ai/knowledge/import", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleImportGuestKnowledge)))
	r.mux.HandleFunc("/api/ai/knowledge/clear", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleClearGuestKnowledge)))
	// SECURITY: Debug context leaks system prompt and infra details - require settings:read scope
	r.mux.HandleFunc("/api/ai/debug/context", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleDebugContext)))
	// SECURITY: Connected agents list could reveal fleet topology - require ai:execute scope
	r.mux.HandleFunc("/api/ai/agents", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetConnectedAgents)))
	// SECURITY: Cost summary could reveal usage patterns - require settings:read scope
	r.mux.HandleFunc("/api/ai/cost/summary", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		r.aiSettingsHandler.HandleGetAICostSummary(w, req)
	})))
	r.mux.HandleFunc("/api/ai/cost/reset", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleResetAICostHistory)))
	r.mux.HandleFunc("/api/ai/cost/export", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleExportAICostHistory)))
	// OAuth endpoints for Claude Pro/Max subscription authentication
	// Require settings:write scope to prevent low-privilege tokens from modifying OAuth credentials
	r.mux.HandleFunc("/api/ai/oauth/start", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleOAuthStart)))
	r.mux.HandleFunc("/api/ai/oauth/exchange", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleOAuthExchange))) // Manual code input
	r.mux.HandleFunc("/api/ai/oauth/callback", r.aiSettingsHandler.HandleOAuthCallback)                                                                  // Public - receives redirect from Anthropic
	r.mux.HandleFunc("/api/ai/oauth/disconnect", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleOAuthDisconnect)))

	// Relay routes for mobile remote access
	r.mux.HandleFunc("GET /api/settings/relay", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleGetRelayConfig))))
	r.mux.HandleFunc("PUT /api/settings/relay", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleUpdateRelayConfig))))
	r.mux.HandleFunc("GET /api/settings/relay/status", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleGetRelayStatus))))
	r.mux.HandleFunc("GET /api/onboarding/qr", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleGetOnboardingQR)(w, req)
	})))
	r.mux.HandleFunc("POST /api/onboarding/validate", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleValidateOnboardingConnection)(w, req)
	})))
	r.mux.HandleFunc("GET /api/onboarding/deep-link", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleGetOnboardingDeepLink)(w, req)
	})))

	// AI Patrol routes for background monitoring
	// Note: Status remains accessible so UI can show license/upgrade state
	// Read endpoints (findings, history, runs) return redacted preview data when unlicensed
	// Mutation endpoints (run, acknowledge, dismiss, etc.) return 402 to prevent unauthorized actions
	// SECURITY: Patrol status and stream require ai:execute scope to access findings
	r.mux.HandleFunc("/api/ai/patrol/status", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatrolStatus)))
	r.mux.HandleFunc("/api/ai/patrol/stream", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandlePatrolStream)))
	r.mux.HandleFunc("/api/ai/patrol/findings", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetPatrolFindings(w, req)
		case http.MethodDelete:
			// Clear all findings - doesn't require Pro license so users can clean up accumulated findings
			r.aiSettingsHandler.HandleClearAllFindings(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	// SECURITY: AI Patrol read endpoints - require ai:execute scope
	r.mux.HandleFunc("/api/ai/patrol/history", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetFindingsHistory)))
	r.mux.HandleFunc("/api/ai/patrol/run", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleForcePatrol)))
	// SECURITY: AI Patrol mutation endpoints - require ai:execute scope to prevent low-privilege tokens from
	// dismissing, suppressing, or otherwise hiding findings. This prevents attackers from blinding AI Patrol.
	r.mux.HandleFunc("/api/ai/patrol/acknowledge", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleAcknowledgeFinding)))
	// Dismiss and resolve don't require Pro license - users should be able to clear findings they can see
	// This is especially important for users who accumulated findings before fixing the patrol-without-AI bug
	r.mux.HandleFunc("/api/ai/patrol/dismiss", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleDismissFinding)))
	r.mux.HandleFunc("/api/ai/patrol/findings/note", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleSetFindingNote)))
	r.mux.HandleFunc("/api/ai/patrol/suppress", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleSuppressFinding)))
	r.mux.HandleFunc("/api/ai/patrol/snooze", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleSnoozeFinding)))
	r.mux.HandleFunc("/api/ai/patrol/resolve", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleResolveFinding)))
	r.mux.HandleFunc("/api/ai/patrol/runs", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatrolRunHistory)))
	// Suppression rules management - require scope to prevent low-privilege tokens from creating suppression rules
	r.mux.HandleFunc("/api/ai/patrol/suppressions", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetSuppressionRules(w, req)
		case http.MethodPost:
			r.aiSettingsHandler.HandleAddSuppressionRule(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/ai/patrol/suppressions/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleDeleteSuppressionRule)))
	r.mux.HandleFunc("/api/ai/patrol/dismissed", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetDismissedFindings)))

	// Patrol Autonomy - Community is locked to "monitor"; approval/assisted/full require Pro (enforced in handlers)
	r.mux.HandleFunc("/api/ai/patrol/autonomy", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetPatrolAutonomy(w, req)
		case http.MethodPut:
			r.aiSettingsHandler.HandleUpdatePatrolAutonomy(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Investigation endpoints - viewing is free; reinvestigation and fix execution require Pro
	// SECURITY: Require ai:execute scope to prevent low-privilege tokens from reading investigation details
	r.mux.HandleFunc("/api/ai/findings/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		switch {
		case strings.HasSuffix(path, "/investigation/messages"):
			r.aiSettingsHandler.HandleGetInvestigationMessages(w, req)
		case strings.HasSuffix(path, "/investigation"):
			r.aiSettingsHandler.HandleGetInvestigation(w, req)
		case strings.HasSuffix(path, "/reinvestigate"):
			// Reinvestigation is investigation and requires Pro license
			RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, r.aiSettingsHandler.HandleReinvestigateFinding)(w, req)
		case strings.HasSuffix(path, "/reapprove"):
			// Fix execution requires Pro license
			RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, r.aiSettingsHandler.HandleReapproveInvestigationFix)(w, req)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})))

	// AI Intelligence endpoints - expose learned patterns, correlations, and predictions
	// SECURITY: Require ai:execute scope to prevent low-privilege tokens from reading sensitive intelligence
	// Unified intelligence endpoint - aggregates all AI subsystems into a single view
	r.mux.HandleFunc("/api/ai/intelligence", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetIntelligence)))
	// Individual sub-endpoints for specific intelligence layers
	r.mux.HandleFunc("/api/ai/intelligence/patterns", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatterns)))
	r.mux.HandleFunc("/api/ai/intelligence/predictions", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPredictions)))
	r.mux.HandleFunc("/api/ai/intelligence/correlations", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetCorrelations)))
	r.mux.HandleFunc("/api/ai/intelligence/changes", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRecentChanges)))
	r.mux.HandleFunc("/api/ai/intelligence/baselines", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetBaselines)))
	r.mux.HandleFunc("/api/ai/intelligence/remediations", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRemediations)))
	r.mux.HandleFunc("/api/ai/intelligence/anomalies", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetAnomalies)))
	r.mux.HandleFunc("/api/ai/intelligence/learning", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetLearningStatus)))
	// Unified findings endpoint (alerts + AI findings)
	r.mux.HandleFunc("/api/ai/unified/findings", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetUnifiedFindings)))

	// Phase 6: AI Intelligence Services
	r.mux.HandleFunc("/api/ai/forecast", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetForecast)))
	r.mux.HandleFunc("/api/ai/forecasts/overview", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetForecastOverview)))
	r.mux.HandleFunc("/api/ai/learning/preferences", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetLearningPreferences)))
	r.mux.HandleFunc("/api/ai/proxmox/events", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetProxmoxEvents)))
	r.mux.HandleFunc("/api/ai/proxmox/correlations", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetProxmoxCorrelations)))
	// SECURITY: Remediation endpoints require ai:execute scope and Pro license (ai_autofix)
	r.mux.HandleFunc("/api/ai/remediation/plans", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetRemediationPlans(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	r.mux.HandleFunc("/api/ai/remediation/plan", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, r.aiSettingsHandler.HandleGetRemediationPlan))))
	// Approving a remediation plan is a mutation - keep with ai:execute scope
	r.mux.HandleFunc("/api/ai/remediation/approve", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, r.aiSettingsHandler.HandleApproveRemediationPlan))))
	r.mux.HandleFunc("/api/ai/remediation/execute", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, r.aiSettingsHandler.HandleExecuteRemediationPlan))))
	r.mux.HandleFunc("/api/ai/remediation/rollback", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, featureAIAutoFixKey, r.aiSettingsHandler.HandleRollbackRemediationPlan))))
	// SECURITY: Circuit breaker status could reveal operational state - require ai:execute scope
	r.mux.HandleFunc("/api/ai/circuit/status", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetCircuitBreakerStatus)))

	// Phase 7: Incident Recording API - require ai:execute scope to protect incident data
	r.mux.HandleFunc("/api/ai/incidents", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRecentIncidents)))
	r.mux.HandleFunc("/api/ai/incidents/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetIncidentData)))

	// AI Chat Sessions - sync across devices (legacy endpoints)
	r.mux.HandleFunc("/api/ai/chat/sessions", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleListAIChatSessions)))
	r.mux.HandleFunc("/api/ai/chat/sessions/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetAIChatSession(w, req)
		case http.MethodPut:
			r.aiSettingsHandler.HandleSaveAIChatSession(w, req)
		case http.MethodDelete:
			r.aiSettingsHandler.HandleDeleteAIChatSession(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// AI chat endpoints
	// SECURITY: Status endpoint is part of chat UX and should require ai:chat scope for token clients.
	r.mux.HandleFunc("/api/ai/status", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiHandler.HandleStatus)))
	r.mux.HandleFunc("/api/ai/chat", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiHandler.HandleChat)))
	r.mux.HandleFunc("/api/ai/sessions", RequireAuth(r.config, RequireScope(config.ScopeAIChat, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiHandler.HandleSessions(w, req)
		case http.MethodPost:
			r.aiHandler.HandleCreateSession(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/ai/sessions/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.routeAISessions)))

	// AI approval endpoints - for command approval workflow
	// Require ai:execute scope to prevent low-privilege tokens from enumerating or denying approvals
	r.mux.HandleFunc("/api/ai/approvals", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleListApprovals)))
	r.mux.HandleFunc("/api/ai/approvals/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.routeApprovals)))

	// AI question endpoints - require ai:chat scope for interactive AI features
	r.mux.HandleFunc("/api/ai/question/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.routeQuestions)))
}
