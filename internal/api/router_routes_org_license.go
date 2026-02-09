package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/metering"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func (r *Router) registerOrgLicenseRoutesGroup(orgHandlers *OrgHandlers, rbacHandlers *RBACHandlers, auditHandlers *AuditHandlers) {
	conversionHandlers := NewConversionHandlers(conversion.NewRecorder(metering.NewWindowedAggregator()))

	// License routes (Pulse Pro)
	r.mux.HandleFunc("/api/license/status", RequireAdmin(r.config, r.licenseHandlers.HandleLicenseStatus))
	r.mux.HandleFunc("/api/license/features", RequireAuth(r.config, r.licenseHandlers.HandleLicenseFeatures))
	r.mux.HandleFunc("/api/license/activate", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleActivateLicense)))
	r.mux.HandleFunc("/api/license/clear", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleClearLicense)))
	r.mux.HandleFunc("GET /api/license/entitlements", RequireAuth(r.config, r.licenseHandlers.HandleEntitlements))
	r.mux.HandleFunc("POST /api/conversion/events", RequireAuth(r.config, conversionHandlers.HandleRecordEvent))
	r.mux.HandleFunc("GET /api/conversion/stats", RequireAuth(r.config, conversionHandlers.HandleGetStats))

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
	r.mux.HandleFunc("GET /api/audit", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))))
	r.mux.HandleFunc("GET /api/audit/", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))))
	r.mux.HandleFunc("GET /api/audit/{id}/verify", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleVerifyAuditEvent))))

	// RBAC routes (Phase 2 - Enterprise feature)
	r.mux.HandleFunc("/api/admin/roles", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleRoles)))
	r.mux.HandleFunc("/api/admin/roles/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleRoles)))
	r.mux.HandleFunc("/api/admin/users", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleGetUsers)))
	r.mux.HandleFunc("/api/admin/users/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleUserRoleActions)))

	// Advanced Reporting routes
	r.mux.HandleFunc("/api/admin/reports/generate", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, RequireLicenseFeature(r.licenseHandlers, license.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateReport))))
	r.mux.HandleFunc("/api/admin/reports/generate-multi", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, RequireLicenseFeature(r.licenseHandlers, license.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateMultiReport))))

	// Audit Webhook routes
	r.mux.HandleFunc("/api/admin/webhooks/audit", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			RequireScope(config.ScopeSettingsRead, auditHandlers.HandleGetWebhooks)(w, req)
		} else {
			RequireScope(config.ScopeSettingsWrite, auditHandlers.HandleUpdateWebhooks)(w, req)
		}
	})))
}
