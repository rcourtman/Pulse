package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"
)

func (r *Router) registerOrgLicenseRoutesGroup(orgHandlers *OrgHandlers, rbacHandlers *RBACHandlers, auditHandlers *AuditHandlers) {
	conversionConfig := pkglicensing.NewCollectionConfig()
	conversionHandlers := NewConversionHandlers(
		pkglicensing.NewRecorderFromWindowedAggregator(metering.NewWindowedAggregator(), r.conversionStore),
		pkglicensing.NewPipelineHealth(),
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

	// Audit log routes (Enterprise feature)
	r.mux.HandleFunc("GET /api/audit", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/{id}/verify", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleVerifyAuditEvent))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/export", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleExportAuditEvents))(w, req)
	}))
	r.mux.HandleFunc("GET /api/audit/summary", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleAuditSummary))(w, req)
	}))

	// RBAC routes (Phase 2 - Enterprise feature)
	r.mux.HandleFunc("/api/admin/roles", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureRBAC, rbacHandlers.HandleRoles)(w, req)
	}))
	r.mux.HandleFunc("/api/admin/roles/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureRBAC, rbacHandlers.HandleRoles)(w, req)
	}))
	r.mux.HandleFunc("/api/admin/users", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureRBAC, rbacHandlers.HandleGetUsers)(w, req)
	}))
	r.mux.HandleFunc("/api/admin/users/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureRBAC, rbacHandlers.HandleUserRoleActions)(w, req)
	}))
	// RBAC admin operations (Enterprise feature)
	r.mux.HandleFunc("GET /api/admin/rbac/integrity", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureRBAC, rbacHandlers.HandleRBACIntegrityCheck)(w, req)
	}))
	r.mux.HandleFunc("POST /api/admin/rbac/reset-admin", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureRBAC, rbacHandlers.HandleRBACAdminReset)(w, req)
	}))

	// Advanced Reporting routes
	r.mux.HandleFunc("/api/admin/reports/generate", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateReport))(w, req)
	}))
	r.mux.HandleFunc("/api/admin/reports/generate-multi", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateMultiReport))(w, req)
	}))

	// Audit Webhook routes
	r.mux.HandleFunc("/api/admin/webhooks/audit", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceAuditLogs, func(w http.ResponseWriter, req *http.Request) {
		if !ensureAdminSession(r.config, w, req) {
			return
		}
		RequireLicenseFeature(r.licenseHandlers, pkglicensing.FeatureAuditLogging, func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				RequireScope(config.ScopeSettingsRead, auditHandlers.HandleGetWebhooks)(w, req)
			} else {
				RequireScope(config.ScopeSettingsWrite, auditHandlers.HandleUpdateWebhooks)(w, req)
			}
		})(w, req)
	}))
}
