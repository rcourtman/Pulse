package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func (r *Router) registerOrgLicenseRoutesGroup(orgHandlers *OrgHandlers, rbacHandlers *RBACHandlers, auditHandlers *AuditHandlers) {
	conversionConfig := newCollectionConfigFromLicensing()
	conversionHandlers := NewConversionHandlers(
		newConversionRecorderFromLicensing(r.conversionStore),
		newConversionPipelineHealthFromLicensing(),
		conversionConfig,
		r.conversionStore,
		func() bool { return r != nil && r.config != nil && r.config.DisableLocalUpgradeMetrics },
	)

	// License routes (Pulse Pro)
	r.mux.HandleFunc("/api/license/status", RequireAdmin(r.config, r.licenseHandlers.HandleLicenseStatus))
	r.mux.HandleFunc("/api/license/features", RequireAuth(r.config, r.licenseHandlers.HandleLicenseFeatures))
	r.mux.HandleFunc("/api/license/activate", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleActivateLicense)))
	r.mux.HandleFunc("/api/license/clear", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleClearLicense)))
	r.mux.HandleFunc("GET /api/license/entitlements", RequireAuth(r.config, r.licenseHandlers.HandleEntitlements))
	r.mux.HandleFunc("GET /api/license/host-ledger", RequireAuth(r.config, r.handleHostLedger))
	r.mux.HandleFunc("POST /api/license/trial/start", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleStartTrial)))

	// Local upgrade metrics (formerly "conversion" telemetry). Canonical routes:
	// These are local-only signals used to improve in-app upgrade flows; no external export.
	r.mux.HandleFunc("POST /api/upgrade-metrics/events", RequireAuth(r.config, conversionHandlers.HandleRecordEvent))
	r.mux.HandleFunc("GET /api/upgrade-metrics/stats", RequireAuth(r.config, conversionHandlers.HandleGetStats))
	r.mux.HandleFunc("GET /api/upgrade-metrics/health", RequireAuth(r.config, conversionHandlers.HandleGetHealth))
	r.mux.HandleFunc("GET /api/upgrade-metrics/config", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		conversionHandlers.HandleGetConfig(w, req)
	})))
	r.mux.HandleFunc("PUT /api/upgrade-metrics/config", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}
		conversionHandlers.HandleUpdateConfig(w, req)
	})))
	r.mux.HandleFunc("GET /api/admin/upgrade-metrics-funnel", RequireAdmin(r.config, conversionHandlers.HandleConversionFunnel))

	// Legacy compatibility aliases (deprecated).
	r.mux.HandleFunc("POST /api/conversion/events", RequireAuth(r.config, conversionHandlers.HandleRecordEvent))
	r.mux.HandleFunc("GET /api/conversion/stats", RequireAuth(r.config, conversionHandlers.HandleGetStats))
	r.mux.HandleFunc("GET /api/conversion/health", RequireAuth(r.config, conversionHandlers.HandleGetHealth))
	r.mux.HandleFunc("GET /api/conversion/config", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		conversionHandlers.HandleGetConfig(w, req)
	})))
	r.mux.HandleFunc("PUT /api/conversion/config", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}
		conversionHandlers.HandleUpdateConfig(w, req)
	})))
	r.mux.HandleFunc("GET /api/admin/conversion-funnel", RequireAdmin(r.config, conversionHandlers.HandleConversionFunnel))

	// Organization routes (multi-tenant foundation)
	r.mux.HandleFunc("GET /api/orgs", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListOrgs)))
	r.mux.HandleFunc("POST /api/orgs", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleCreateOrg)))
	r.mux.HandleFunc("GET /api/orgs/{id}", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleGetOrg)))
	r.mux.HandleFunc("PUT /api/orgs/{id}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleUpdateOrg)))
	r.mux.HandleFunc("DELETE /api/orgs/{id}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleDeleteOrg)))
	r.mux.HandleFunc("GET /api/orgs/{id}/members", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListMembers)))
	r.mux.HandleFunc("POST /api/orgs/{id}/members", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleInviteMember)))
	r.mux.HandleFunc("DELETE /api/orgs/{id}/members/{userId}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleRemoveMember)))
	r.mux.HandleFunc("GET /api/orgs/{id}/shares", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListShares)))
	r.mux.HandleFunc("GET /api/orgs/{id}/shares/incoming", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListIncomingShares)))
	r.mux.HandleFunc("POST /api/orgs/{id}/shares", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleCreateShare)))
	r.mux.HandleFunc("DELETE /api/orgs/{id}/shares/{shareId}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleDeleteShare)))

	auditAdminEndpoints := resolveAuditAdminEndpoints(
		auditAdminEndpointAdapter{handlers: auditHandlers},
		newAuditAdminRuntime(),
	)

	// Audit log routes (Enterprise feature)
	r.mux.HandleFunc("GET /api/audit", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAuditLoggingValue, RequireScope(config.ScopeSettingsRead, auditAdminEndpoints.HandleListEvents))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAuditLoggingValue, RequireScope(config.ScopeSettingsRead, auditAdminEndpoints.HandleListEvents))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/{id}/verify", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAuditLoggingValue, RequireScope(config.ScopeSettingsRead, auditAdminEndpoints.HandleVerifyEvent))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/export", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAuditLoggingValue, RequireScope(config.ScopeSettingsRead, auditAdminEndpoints.HandleExportEvents))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/summary", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAuditLoggingValue, RequireScope(config.ScopeSettingsRead, auditAdminEndpoints.HandleSummary))(w, req)
	}))

	// RBAC routes (Phase 2 - Enterprise feature)
	r.mux.HandleFunc("/api/admin/roles", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRBACValue, rbacHandlers.HandleRoles)(w, req)
	}))
	r.mux.HandleFunc("/api/admin/roles/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRBACValue, rbacHandlers.HandleRoles)(w, req)
	}))
	r.mux.HandleFunc("/api/admin/users", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRBACValue, rbacHandlers.HandleGetUsers)(w, req)
	}))
	r.mux.HandleFunc("/api/admin/users/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRBACValue, rbacHandlers.HandleUserRoleActions)(w, req)
	}))
	rbacAdminEndpoints := resolveRBACAdminEndpoints(
		rbacAdminEndpointAdapter{handlers: rbacHandlers},
		newRBACAdminRuntime(rbacHandlers),
	)
	reportingAdminEndpoints := resolveReportingAdminEndpoints(
		reportingAdminEndpointAdapter{handlers: r.reportingHandlers},
		newReportingAdminRuntime(r.reportingHandlers),
	)
	// RBAC admin operations (Enterprise feature)
	r.mux.HandleFunc("GET /api/admin/rbac/integrity", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRBACValue, rbacAdminEndpoints.HandleIntegrityCheck)(w, req)
	}))
	r.mux.HandleFunc("POST /api/admin/rbac/reset-admin", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRBACValue, rbacAdminEndpoints.HandleAdminReset)(w, req)
	}))

	// Advanced Reporting routes
	r.mux.HandleFunc("/api/admin/reports/generate", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAdvancedReportingValue, RequireScope(config.ScopeSettingsRead, reportingAdminEndpoints.HandleGenerateReport))(w, req)
	}))
	r.mux.HandleFunc("/api/admin/reports/generate-multi", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAdvancedReportingValue, RequireScope(config.ScopeSettingsRead, reportingAdminEndpoints.HandleGenerateMultiReport))(w, req)
	}))

	// Audit Webhook routes
	r.mux.HandleFunc("/api/admin/webhooks/audit", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureAuditLoggingValue, func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				RequireScope(config.ScopeSettingsRead, auditAdminEndpoints.HandleGetWebhooks)(w, req)
			} else {
				RequireScope(config.ScopeSettingsWrite, auditAdminEndpoints.HandleUpdateWebhooks)(w, req)
			}
		})(w, req)
	}))
}

type auditAdminEndpointAdapter struct {
	handlers *AuditHandlers
}

var _ extensions.AuditAdminEndpoints = auditAdminEndpointAdapter{}

func (a auditAdminEndpointAdapter) HandleListEvents(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "audit_unavailable", "Audit management is not available", nil)
		return
	}
	a.handlers.HandleListAuditEvents(w, req)
}

func (a auditAdminEndpointAdapter) HandleVerifyEvent(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "audit_unavailable", "Audit management is not available", nil)
		return
	}
	a.handlers.HandleVerifyAuditEvent(w, req)
}

func (a auditAdminEndpointAdapter) HandleExportEvents(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "audit_unavailable", "Audit management is not available", nil)
		return
	}
	a.handlers.HandleExportAuditEvents(w, req)
}

func (a auditAdminEndpointAdapter) HandleSummary(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "audit_unavailable", "Audit management is not available", nil)
		return
	}
	a.handlers.HandleAuditSummary(w, req)
}

func (a auditAdminEndpointAdapter) HandleGetWebhooks(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "audit_unavailable", "Audit management is not available", nil)
		return
	}
	a.handlers.HandleGetWebhooks(w, req)
}

func (a auditAdminEndpointAdapter) HandleUpdateWebhooks(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "audit_unavailable", "Audit management is not available", nil)
		return
	}
	a.handlers.HandleUpdateWebhooks(w, req)
}

type rbacAdminEndpointAdapter struct {
	handlers *RBACHandlers
}

var _ extensions.RBACAdminEndpoints = rbacAdminEndpointAdapter{}

func (a rbacAdminEndpointAdapter) HandleIntegrityCheck(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}
	a.handlers.HandleRBACIntegrityCheck(w, req)
}

func (a rbacAdminEndpointAdapter) HandleAdminReset(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}
	a.handlers.HandleRBACAdminReset(w, req)
}

type reportingAdminEndpointAdapter struct {
	handlers *ReportingHandlers
}

var _ extensions.ReportingAdminEndpoints = reportingAdminEndpointAdapter{}

func (a reportingAdminEndpointAdapter) HandleGenerateReport(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "reporting_unavailable", "Reporting is not available", nil)
		return
	}
	a.handlers.HandleGenerateReport(w, req)
}

func (a reportingAdminEndpointAdapter) HandleGenerateMultiReport(w http.ResponseWriter, req *http.Request) {
	if a.handlers == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "reporting_unavailable", "Reporting is not available", nil)
		return
	}
	a.handlers.HandleGenerateMultiReport(w, req)
}

func newRBACAdminRuntime(handlers *RBACHandlers) extensions.RBACAdminRuntime {
	return extensions.RBACAdminRuntime{
		GetRequestOrgID:       GetOrgID,
		IsValidOrganizationID: isValidOrganizationID,
		GetClientIP:           GetClientIP,
		ValidateRecoveryToken: func(token, clientIP string) bool {
			store := GetRecoveryTokenStore()
			if store == nil {
				return false
			}
			return store.ValidateRecoveryTokenConstantTime(token, clientIP)
		},
		VerifyIntegrity: func(orgID string) (extensions.RBACIntegrityResult, error) {
			if handlers == nil || handlers.rbacProvider == nil {
				return extensions.RBACIntegrityResult{}, extensions.ErrRBACUnavailable
			}
			result := VerifyRBACIntegrity(handlers.rbacProvider, orgID)
			return extensions.RBACIntegrityResult(result), nil
		},
		ResetAdminRole: func(orgID, username string) error {
			if handlers == nil || handlers.rbacProvider == nil {
				return extensions.ErrRBACUnavailable
			}
			return ResetAdminRole(handlers.rbacProvider, orgID, username)
		},
		WriteError: writeErrorResponse,
	}
}

func newAuditAdminRuntime() extensions.AuditAdminRuntime {
	return extensions.AuditAdminRuntime{
		GetRequestOrgID:    GetOrgID,
		ResolveLogger:      getLoggerForOrg,
		IsPersistentLogger: isPersistentLogger,
		ValidateWebhookURL: validateWebhookURL,
		WriteError:         writeErrorResponse,
	}
}

func newReportingAdminRuntime(handlers *ReportingHandlers) extensions.ReportingAdminRuntime {
	runtime := extensions.ReportingAdminRuntime{
		GetEngine:        reporting.GetEngine,
		GetRequestOrgID:  GetOrgID,
		SanitizeFilename: sanitizeFilename,
		WriteError:       writeErrorResponse,
	}

	if handlers == nil {
		return runtime
	}

	runtime.GetStateSnapshot = handlers.getRuntimeStateSnapshot
	runtime.ListBackupsForResource = handlers.listBackupsForReport

	return runtime
}
