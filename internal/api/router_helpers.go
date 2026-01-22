package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// getMonitor returns the tenant-specific monitor instance for the request.
// It uses the OrgID from the context (injected by TenantMiddleware).
// If no tenant monitor is found, or if not in multi-tenant mode, it returns the default monitor.
func (r *Router) getMonitor(req *http.Request) (*monitoring.Monitor, error) {
	if r.mtMonitor == nil {
		return r.monitor, nil
	}

	orgID := GetOrgID(req.Context())
	if orgID == "" {
		return r.monitor, nil
	}

	return r.mtMonitor.GetMonitor(orgID)
}

// SetMultiTenantMonitor updates the multi-tenant monitor manager.
// Used during reload.
func (r *Router) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	r.mtMonitor = mtm
	if r.alertHandlers != nil {
		r.alertHandlers.SetMultiTenantMonitor(mtm)
	}
	if r.notificationHandlers != nil {
		r.notificationHandlers.SetMultiTenantMonitor(mtm)
	}
	if r.dockerAgentHandlers != nil {
		r.dockerAgentHandlers.SetMultiTenantMonitor(mtm)
	}
	if r.hostAgentHandlers != nil {
		r.hostAgentHandlers.SetMultiTenantMonitor(mtm)
	}
	if r.kubernetesAgentHandlers != nil {
		r.kubernetesAgentHandlers.SetMultiTenantMonitor(mtm)
	}
	if r.systemSettingsHandler != nil {
		r.systemSettingsHandler.SetMultiTenantMonitor(mtm)
	}
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			r.monitor = m
		}
	}
}
