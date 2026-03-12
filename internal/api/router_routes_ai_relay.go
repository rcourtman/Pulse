package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/rs/zerolog/log"
)

const (
	featureKubernetesAIKey = "kubernetes_ai"
	featureAIAlertsKey     = "ai_alerts"
	featureRelayKey        = "relay"
	featureAIAutoFixKey    = "ai_autofix"
)

func (r *Router) registerAIRelayRoutesGroup() {
	// Resolve AI extension endpoints. In the open-source build (no enterprise binder),
	// the free adapters return 402 for all premium operations. Enterprise binders
	// replace these with real handler implementations.
	r.aiAutoFixEndpoints = resolveAIAutoFixEndpoints(
		aiAutoFixFreeAdapter{},
		newAIAutoFixRuntime(r),
	)
	r.aiAlertAnalysisEndpoints = resolveAIAlertAnalysisEndpoints(
		aiAlertAnalysisFreeAdapter{},
		newAIAlertAnalysisRuntime(r),
	)
	r.mux.HandleFunc("/api/settings/ai", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceSettings, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleGetAISettings)))
	r.mux.HandleFunc("/api/settings/ai/update", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleUpdateAISettings)))
	r.mux.HandleFunc("/api/ai/test", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestAIConnection)))
	r.mux.HandleFunc("/api/ai/test/{provider}", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestProvider)))
	// AI models list - require ai:chat scope (needed to select a model for chat)
	r.mux.HandleFunc("/api/ai/models", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleListModels)))
	r.mux.HandleFunc("/api/ai/execute", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecute)))
	r.mux.HandleFunc("/api/ai/execute/stream", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecuteStream)))
	r.mux.HandleFunc("/api/ai/kubernetes/analyze", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiAlertAnalysisEndpoints.HandleAnalyzeKubernetesCluster)))
	r.mux.HandleFunc("/api/ai/investigate-alert", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiAlertAnalysisEndpoints.HandleInvestigateAlert)))

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
	r.mux.HandleFunc("/api/ai/patrol/runs/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatrolRun)))
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

	// Patrol Autonomy - GET stays in core (always returns current level); PUT is gated via extension
	r.mux.HandleFunc("/api/ai/patrol/autonomy", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetPatrolAutonomy(w, req)
		case http.MethodPut:
			r.aiAutoFixEndpoints.HandleUpdatePatrolAutonomy(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Investigation endpoints - viewing is free; reinvestigation and fix execution gated via extension
	// SECURITY: Require ai:execute scope to prevent low-privilege tokens from reading investigation details
	r.mux.HandleFunc("/api/ai/findings/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		switch {
		case strings.HasSuffix(path, "/investigation/messages"):
			r.aiSettingsHandler.HandleGetInvestigationMessages(w, req)
		case strings.HasSuffix(path, "/investigation"):
			r.aiSettingsHandler.HandleGetInvestigation(w, req)
		case strings.HasSuffix(path, "/reinvestigate"):
			r.aiAutoFixEndpoints.HandleReinvestigateFinding(w, req)
		case strings.HasSuffix(path, "/reapprove"):
			r.aiAutoFixEndpoints.HandleReapproveInvestigationFix(w, req)
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
	// SECURITY: Remediation endpoints require ai:execute scope; license gating via extension
	r.mux.HandleFunc("/api/ai/remediation/plans", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiAutoFixEndpoints.HandleGetRemediationPlans(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/ai/remediation/plan", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiAutoFixEndpoints.HandleGetRemediationPlan)))
	r.mux.HandleFunc("/api/ai/remediation/approve", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiAutoFixEndpoints.HandleApproveRemediationPlan)))
	r.mux.HandleFunc("/api/ai/remediation/execute", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiAutoFixEndpoints.HandleExecuteRemediationPlan)))
	r.mux.HandleFunc("/api/ai/remediation/rollback", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiAutoFixEndpoints.HandleRollbackRemediationPlan)))
	// SECURITY: Circuit breaker status could reveal operational state - require ai:execute scope
	r.mux.HandleFunc("/api/ai/circuit/status", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetCircuitBreakerStatus)))

	// Phase 7: Incident Recording API - require ai:execute scope to protect incident data
	r.mux.HandleFunc("/api/ai/incidents", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRecentIncidents)))
	r.mux.HandleFunc("/api/ai/incidents/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetIncidentData)))

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
	r.mux.HandleFunc("/api/ai/approvals", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiAutoFixEndpoints.HandleListApprovals)))
	r.mux.HandleFunc("/api/ai/approvals/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.routeApprovals)))

	// AI question endpoints - require ai:chat scope for interactive AI features
	r.mux.HandleFunc("/api/ai/question/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.routeQuestions)))

	// Provide extension endpoints to the approval handler for investigation fix gating
	r.aiSettingsHandler.SetAIAutoFixEndpoints(r.aiAutoFixEndpoints)
}

// --- AI Auto-Fix free-tier adapter ---
// All methods return 402 "requires Pulse Pro". Enterprise binders replace this
// with real handler implementations.

type aiAutoFixFreeAdapter struct{}

var _ extensions.AIAutoFixEndpoints = aiAutoFixFreeAdapter{}

func (aiAutoFixFreeAdapter) HandleReinvestigateFinding(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Investigation requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleReapproveInvestigationFix(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Fix execution requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleUpdatePatrolAutonomy(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Patrol autonomy settings require Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleGetRemediationPlans(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Remediation requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleGetRemediationPlan(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Remediation requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleApproveRemediationPlan(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Remediation requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleExecuteRemediationPlan(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Remediation requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleRollbackRemediationPlan(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Remediation requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleApproveInvestigationFix(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Auto-Fix requires Pulse Pro")
}

func (aiAutoFixFreeAdapter) HandleListApprovals(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAutoFixKey, "Approval management requires Pulse Pro")
}

func newAIAutoFixRuntime(r *Router) extensions.AIAutoFixRuntime {
	hasLicenseFeature := func(req *http.Request, feature string) bool {
		if r.licenseHandlers == nil {
			return false
		}
		svc := r.licenseHandlers.FeatureService(req.Context())
		if svc == nil {
			return false
		}
		return svc.RequireFeature(feature) == nil
	}
	return extensions.AIAutoFixRuntime{
		HasLicenseFeature:    hasLicenseFeature,
		WriteLicenseRequired: WriteLicenseRequired,
		WriteError:           writeErrorResponse,
		HandlerDeps:          newAIAutoFixHandlerDeps(r),
	}
}

func newAIAutoFixHandlerDeps(r *Router) extensions.AIAutoFixHandlerDeps {
	h := r.aiSettingsHandler
	return extensions.AIAutoFixHandlerDeps{
		GetInvestigationStore: func(orgID string) aicontracts.InvestigationStore {
			h.investigationMu.RLock()
			defer h.investigationMu.RUnlock()
			return h.investigationStores[orgID]
		},
		Approvals: func() aicontracts.ApprovalStoreAccessor {
			if approval.GetStore() == nil {
				return nil
			}
			return &approvalStoreAdapter{}
		},
		MCPExecutor:    &mcpToolAdapter{handler: h},
		AgentExecutor:  &agentCommandAdapter{handler: h},
		FindingUpdater: &findingOutcomeAdapter{handler: h},
		FixVerifier:    &fixVerificationAdapter{handler: h},
		PatrolConfig: func(req *http.Request) aicontracts.PatrolConfigAccessor {
			svc := h.GetAIService(req.Context())
			if svc == nil {
				return nil
			}
			return &patrolConfigAdapter{svc: svc}
		},
		PatrolConfigUpdate: func(req *http.Request) aicontracts.PatrolConfigUpdater {
			svc := h.GetAIService(req.Context())
			if svc == nil {
				return nil
			}
			return &patrolConfigUpdateAdapter{handler: h, ctx: req.Context()}
		},
		GetRemediationEngine: func(orgID string) aicontracts.RemediationEngine {
			return h.GetRemediationEngineForOrg(orgID)
		},
		LaunchRemediationVerification: func(ctx context.Context, findingID, executionID string, engine aicontracts.RemediationEngine) {
			aiSvc := h.GetAIService(ctx)
			if aiSvc == nil {
				return
			}
			go func() {
				time.Sleep(30 * time.Second)

				patrol := aiSvc.GetPatrolService()
				if patrol == nil {
					log.Warn().Str("findingID", findingID).Msg("[Remediation] Post-fix verification skipped: no patrol service")
					return
				}

				finding := patrol.GetFindings().Get(findingID)
				if finding == nil {
					log.Warn().Str("findingID", findingID).Msg("[Remediation] Post-fix verification skipped: finding not found")
					return
				}

				bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				verified, verifyErr := patrol.VerifyFixResolved(bgCtx, finding.ResourceID, finding.ResourceType, finding.Key, finding.ID)
				if verifyErr != nil {
					log.Error().Err(verifyErr).Str("findingID", findingID).Msg("[Remediation] Post-fix verification failed with error")
				} else if !verified {
					log.Warn().Str("findingID", findingID).Msg("[Remediation] Post-fix verification: issue persists")
				} else {
					log.Info().Str("findingID", findingID).Msg("[Remediation] Post-fix verification: issue resolved")
				}

				// Update execution status based on verification result
				if verifyErr != nil {
					engine.SetExecutionVerification(executionID, false, fmt.Sprintf("Verification error: %v", verifyErr))
				} else if !verified {
					engine.SetExecutionVerification(executionID, false, "Issue persists after fix")
				} else {
					engine.SetExecutionVerification(executionID, true, "Issue resolved")
				}
			}()
		},
		GetOrchestrator: func(req *http.Request) aicontracts.InvestigationOrchestrator {
			svc := h.GetAIService(req.Context())
			if svc == nil {
				return nil
			}
			patrol := svc.GetPatrolService()
			if patrol == nil {
				return nil
			}
			return patrol.GetInvestigationOrchestrator()
		},
		SetupOrchestrator: func(orgID string) {
			h.setupInvestigationOrchestrator(orgID, h.GetAIService(context.Background()))
		},
		IsInvestigationEnabled: isAIInvestigationEnabled,
		GetOrgID:               GetOrgID,
		NormalizeOrgID:         approval.NormalizeOrgID,
		GetUsername: func(req *http.Request) string {
			return getAuthUsername(h.getConfig(req.Context()), req)
		},
		EnsureScope: func(w http.ResponseWriter, req *http.Request, scope string) bool {
			return ensureScope(w, req, scope)
		},
		AuditLog:    LogAuditEvent,
		GetClientIP: GetClientIP,
	}
}

// --- AI Alert Analysis free-tier adapter ---

type aiAlertAnalysisFreeAdapter struct{}

var _ extensions.AIAlertAnalysisEndpoints = aiAlertAnalysisFreeAdapter{}

func (aiAlertAnalysisFreeAdapter) HandleInvestigateAlert(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureAIAlertsKey, "Alert investigation requires Pulse Pro")
}

func (aiAlertAnalysisFreeAdapter) HandleAnalyzeKubernetesCluster(w http.ResponseWriter, _ *http.Request) {
	WriteLicenseRequired(w, featureKubernetesAIKey, "Kubernetes AI analysis requires Pulse Pro")
}

// ===========================================================================
// Adapter implementations for AIAutoFixHandlerDeps
// ===========================================================================

// approvalStoreAdapter implements aicontracts.ApprovalStoreAccessor by
// wrapping the global approval.Store singleton.
type approvalStoreAdapter struct{}

var _ aicontracts.ApprovalStoreAccessor = (*approvalStoreAdapter)(nil)

func (a *approvalStoreAdapter) GetApproval(id string) (*aicontracts.ApprovalInfo, bool) {
	store := approval.GetStore()
	if store == nil {
		return nil, false
	}
	req, ok := store.GetApproval(id)
	if !ok {
		return nil, false
	}
	return approvalRequestToInfo(req), true
}

func (a *approvalStoreAdapter) Approve(id, username string) (*aicontracts.ApprovalInfo, error) {
	store := approval.GetStore()
	if store == nil {
		return nil, fmt.Errorf("approval store not initialized")
	}
	req, err := store.Approve(id, username)
	if err != nil {
		return nil, err
	}
	return approvalRequestToInfo(req), nil
}

func (a *approvalStoreAdapter) CreateApproval(info *aicontracts.ApprovalInfo) error {
	store := approval.GetStore()
	if store == nil {
		return fmt.Errorf("approval store not initialized")
	}
	req := &approval.ApprovalRequest{
		OrgID:      info.OrgID,
		ToolID:     info.ToolID,
		Command:    info.Command,
		TargetType: info.TargetType,
		TargetID:   info.TargetID,
		TargetName: info.TargetName,
		Context:    info.Context,
		RiskLevel:  approval.RiskLevel(info.RiskLevel),
	}
	if err := store.CreateApproval(req); err != nil {
		return err
	}
	// Backfill the generated ID
	info.ID = req.ID
	return nil
}

func (a *approvalStoreAdapter) GetPendingForOrg(orgID string) ([]*aicontracts.ApprovalInfo, map[string]int) {
	store := approval.GetStore()
	if store == nil {
		return nil, nil
	}
	pending := store.GetPendingApprovalsForOrg(orgID)
	infos := make([]*aicontracts.ApprovalInfo, len(pending))
	for i, req := range pending {
		infos[i] = approvalRequestToInfo(req)
	}
	return infos, store.GetStatsForOrg(orgID)
}

func (a *approvalStoreAdapter) BelongsToOrg(info *aicontracts.ApprovalInfo, orgID string) bool {
	if info == nil {
		return false
	}
	// Reuse the canonical approval.BelongsToOrg logic
	req := &approval.ApprovalRequest{OrgID: info.OrgID}
	return approval.BelongsToOrg(req, orgID)
}

func (a *approvalStoreAdapter) AssessRiskLevel(command, targetType string) string {
	return string(approval.AssessRiskLevel(command, targetType))
}

func approvalRequestToInfo(req *approval.ApprovalRequest) *aicontracts.ApprovalInfo {
	if req == nil {
		return nil
	}
	return &aicontracts.ApprovalInfo{
		ID:          req.ID,
		OrgID:       req.OrgID,
		ExecutionID: req.ExecutionID,
		ToolID:      req.ToolID,
		Command:     req.Command,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		TargetName:  req.TargetName,
		Context:     req.Context,
		RiskLevel:   string(req.RiskLevel),
		Status:      string(req.Status),
		RequestedAt: req.RequestedAt,
		ExpiresAt:   req.ExpiresAt,
		DecidedAt:   req.DecidedAt,
		DecidedBy:   req.DecidedBy,
		DenyReason:  req.DenyReason,
		CommandHash: req.CommandHash,
		Consumed:    req.Consumed,
	}
}

// mcpToolAdapter implements aicontracts.MCPToolExecutor by wrapping the chat service.
type mcpToolAdapter struct {
	handler *AISettingsHandler
}

var _ aicontracts.MCPToolExecutor = (*mcpToolAdapter)(nil)

func (m *mcpToolAdapter) ExecuteMCPTool(ctx context.Context, command, approvalID string) (string, int, error) {
	if m.handler.chatHandler == nil {
		return "", -1, fmt.Errorf("chat handler not available")
	}
	chatSvc := m.handler.chatHandler.GetService(ctx)
	if chatSvc == nil {
		return "", -1, fmt.Errorf("chat service not available")
	}
	chatService, ok := chatSvc.(*chat.Service)
	if !ok {
		return "", -1, fmt.Errorf("chat service type mismatch")
	}
	toolName, args, err := parseMCPToolCall(command)
	if err != nil {
		return "", -1, fmt.Errorf("failed to parse tool call: %w", err)
	}
	if approvalID != "" {
		args["_approval_id"] = approvalID
	}
	log.Info().Str("tool", toolName).Str("approvalID", approvalID).Interface("args", args).Msg("Executing MCP tool fix with pre-approval")
	result, toolErr := chatService.ExecuteMCPTool(ctx, toolName, args)
	if toolErr != nil {
		return result, 1, toolErr
	}
	return result, 0, nil
}

// agentCommandAdapter implements aicontracts.AgentCommandExecutor.
type agentCommandAdapter struct {
	handler *AISettingsHandler
}

var _ aicontracts.AgentCommandExecutor = (*agentCommandAdapter)(nil)

func (a *agentCommandAdapter) ExecuteCommand(ctx context.Context, agentID, command string) (stdout, stderr string, exitCode int, err error) {
	if a.handler.agentServer == nil {
		return "", "", -1, fmt.Errorf("agent server not available")
	}
	result, execErr := a.handler.agentServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
		Command:    command,
		TargetType: "agent",
	})
	if execErr != nil {
		return "", "", -1, execErr
	}
	return result.Stdout, result.Stderr, result.ExitCode, nil
}

func (a *agentCommandAdapter) FindAgentForTarget(targetHost string) string {
	if a.handler.agentServer == nil {
		return ""
	}
	agents := a.handler.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return ""
	}
	if targetHost != "" {
		for _, agent := range agents {
			if agent.Hostname == targetHost || agent.AgentID == targetHost {
				return agent.AgentID
			}
		}
	}
	if len(agents) == 1 {
		return agents[0].AgentID
	}
	return ""
}

// findingOutcomeAdapter implements aicontracts.FindingOutcomeUpdater.
type findingOutcomeAdapter struct {
	handler *AISettingsHandler
}

var _ aicontracts.FindingOutcomeUpdater = (*findingOutcomeAdapter)(nil)

func (f *findingOutcomeAdapter) UpdateFindingOutcome(ctx context.Context, orgID, findingID, outcome string) {
	f.handler.updateFindingOutcome(ctx, orgID, findingID, outcome)
}

// fixVerificationAdapter implements aicontracts.FixVerificationLauncher.
type fixVerificationAdapter struct {
	handler *AISettingsHandler
}

var _ aicontracts.FixVerificationLauncher = (*fixVerificationAdapter)(nil)

func (v *fixVerificationAdapter) LaunchVerification(ctx context.Context, orgID, findingID, sessionID string, proposedFix *aicontracts.Fix, store aicontracts.InvestigationStore) {
	aiSvc := v.handler.GetAIService(ctx)
	go func() {
		time.Sleep(30 * time.Second)

		patrol := aiSvc.GetPatrolService()
		if patrol == nil {
			log.Warn().Str("findingID", findingID).Msg("Post-fix verification skipped: no patrol service")
			return
		}
		finding := patrol.GetFindings().Get(findingID)
		if finding == nil {
			log.Warn().Str("findingID", findingID).Msg("Post-fix verification skipped: finding not found")
			return
		}
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		verified, verifyErr := patrol.VerifyFixResolved(bgCtx, finding.ResourceID, finding.ResourceType, finding.Key, finding.ID)
		if verifyErr != nil {
			log.Error().Err(verifyErr).Str("findingID", findingID).Msg("Post-fix verification failed with error")
			store.Complete(sessionID, aicontracts.OutcomeFixVerificationFailed, fmt.Sprintf("Fix executed but verification error: %v", verifyErr), proposedFix)
		} else if !verified {
			log.Warn().Str("findingID", findingID).Msg("Post-fix verification: issue persists")
			store.Complete(sessionID, aicontracts.OutcomeFixVerificationFailed, "Fix executed but issue persists after verification.", proposedFix)
		} else {
			log.Info().Str("findingID", findingID).Msg("Post-fix verification: issue resolved")
			store.Complete(sessionID, aicontracts.OutcomeFixVerified, "Fix executed and verified - issue resolved.", proposedFix)
		}
		latestSession := store.GetLatestByFinding(findingID)
		if latestSession != nil {
			v.handler.updateFindingOutcome(bgCtx, orgID, findingID, string(latestSession.Outcome))
		}
	}()
}

// patrolConfigAdapter implements aicontracts.PatrolConfigAccessor by wrapping an AI service.
type patrolConfigAdapter struct {
	svc *ai.Service
}

var _ aicontracts.PatrolConfigAccessor = (*patrolConfigAdapter)(nil)

func (p *patrolConfigAdapter) GetEffectiveAutonomyLevel() string {
	if p.svc == nil {
		return ""
	}
	return p.svc.GetEffectivePatrolAutonomyLevel()
}

func (p *patrolConfigAdapter) HasLicenseFeature(feature string) bool {
	if p.svc == nil {
		return false
	}
	return p.svc.HasLicenseFeature(feature)
}

func (p *patrolConfigAdapter) GetPatrolInvestigationBudget() int {
	if p.svc == nil {
		return 0
	}
	cfg := p.svc.GetConfig()
	if cfg == nil {
		return 0
	}
	return cfg.GetPatrolInvestigationBudget()
}

func (p *patrolConfigAdapter) GetPatrolInvestigationTimeout() time.Duration {
	if p.svc == nil {
		return 0
	}
	cfg := p.svc.GetConfig()
	if cfg == nil {
		return 0
	}
	return cfg.GetPatrolInvestigationTimeout()
}

func (p *patrolConfigAdapter) GetPatrolFullModeUnlocked() bool {
	if p.svc == nil {
		return false
	}
	cfg := p.svc.GetConfig()
	if cfg == nil {
		return false
	}
	return cfg.PatrolFullModeUnlocked
}

func (p *patrolConfigAdapter) GetPatrolAutonomyLevel() string {
	if p.svc == nil {
		return ""
	}
	cfg := p.svc.GetConfig()
	if cfg == nil {
		return ""
	}
	return cfg.GetPatrolAutonomyLevel()
}

func (p *patrolConfigAdapter) IsValidPatrolAutonomyLevel(level string) bool {
	return config.IsValidPatrolAutonomyLevel(level)
}

// patrolConfigUpdateAdapter implements aicontracts.PatrolConfigUpdater.
type patrolConfigUpdateAdapter struct {
	handler *AISettingsHandler
	ctx     context.Context
}

var _ aicontracts.PatrolConfigUpdater = (*patrolConfigUpdateAdapter)(nil)

func (u *patrolConfigUpdateAdapter) SaveAutonomySettings(ctx context.Context, level string, unlocked bool, budget, timeoutSec int) error {
	svc := u.handler.GetAIService(ctx)
	if svc == nil {
		return fmt.Errorf("AI service not available")
	}
	cfg := svc.GetConfig()
	if cfg == nil {
		return fmt.Errorf("AI config not available")
	}
	cfg.PatrolFullModeUnlocked = unlocked
	cfg.PatrolAutonomyLevel = level
	cfg.PatrolInvestigationBudget = budget
	cfg.PatrolInvestigationTimeoutSec = timeoutSec
	return u.handler.getPersistence(ctx).SaveAIConfig(*cfg)
}

func (u *patrolConfigUpdateAdapter) ReloadConfig(ctx context.Context) error {
	svc := u.handler.GetAIService(ctx)
	if svc == nil {
		return fmt.Errorf("AI service not available")
	}
	return svc.LoadConfig()
}

// ---------------------------------------------------------------------------
// Pure helper functions (used by adapters)
// ---------------------------------------------------------------------------

// isMCPToolCall checks if a command is an MCP tool call (vs a shell command).
func isMCPToolCall(command string) bool {
	return strings.HasPrefix(command, "pulse_") ||
		strings.HasPrefix(command, "default_api:") ||
		strings.Contains(command, "pulse_control_guest") ||
		strings.Contains(command, "pulse_run_command") ||
		strings.Contains(command, "pulse_get_resource")
}

// cleanTargetHost extracts just the hostname from a target host string.
// Handles cases like "pve-node (The container's host is 'pve-node')" → "pve-node".
func cleanTargetHost(targetHost string) string {
	if targetHost == "" {
		return ""
	}
	if idx := strings.Index(targetHost, " ("); idx > 0 {
		return strings.TrimSpace(targetHost[:idx])
	}
	if idx := strings.Index(targetHost, " "); idx > 0 {
		return strings.TrimSpace(targetHost[:idx])
	}
	return strings.TrimSpace(targetHost)
}

// parseMCPToolCall parses an MCP tool call string into tool name and arguments.
func parseMCPToolCall(command string) (string, map[string]interface{}, error) {
	command = strings.TrimPrefix(command, "default_api:")
	openParen := strings.Index(command, "(")
	if openParen == -1 {
		return "", nil, fmt.Errorf("no opening parenthesis in tool call")
	}
	toolName := strings.TrimSpace(command[:openParen])
	closeParen := strings.LastIndex(command, ")")
	if closeParen == -1 || closeParen <= openParen {
		return "", nil, fmt.Errorf("no closing parenthesis in tool call")
	}
	argsStr := command[openParen+1 : closeParen]
	args := make(map[string]interface{})
	if strings.TrimSpace(argsStr) == "" {
		return toolName, args, nil
	}
	pairs := splitToolArgs(argsStr)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "'\"")
		args[key] = value
	}
	return toolName, args, nil
}

// splitToolArgs splits tool arguments respecting quoted strings.
func splitToolArgs(argsStr string) []string {
	var result []string
	var current strings.Builder
	var inQuote rune
	var escaped bool
	for _, r := range argsStr {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			current.WriteRune(r)
			continue
		}
		if inQuote != 0 {
			current.WriteRune(r)
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			current.WriteRune(r)
			continue
		}
		if r == ',' {
			if s := strings.TrimSpace(current.String()); s != "" {
				result = append(result, s)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		result = append(result, s)
	}
	return result
}

func newAIAlertAnalysisRuntime(r *Router) extensions.AIAlertAnalysisRuntime {
	return extensions.AIAlertAnalysisRuntime{
		HasLicenseFeature: func(req *http.Request, feature string) bool {
			if r.licenseHandlers == nil {
				return false
			}
			svc := r.licenseHandlers.FeatureService(req.Context())
			if svc == nil {
				return false
			}
			return svc.RequireFeature(feature) == nil
		},
		WriteLicenseRequired: WriteLicenseRequired,
		WriteError:           writeErrorResponse,
		CoreHandlers: extensions.AIAlertAnalysisCoreHandlers{
			HandleInvestigateAlert:         r.aiSettingsHandler.HandleInvestigateAlert,
			HandleAnalyzeKubernetesCluster: r.aiSettingsHandler.HandleAnalyzeKubernetesCluster,
		},
	}
}
