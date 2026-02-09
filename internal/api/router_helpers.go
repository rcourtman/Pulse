package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
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

// MultiTenantStateProvider wraps a MultiTenantMonitor to provide state for specific tenants.
type MultiTenantStateProvider struct {
	mtMonitor      *monitoring.MultiTenantMonitor
	defaultMonitor *monitoring.Monitor
}

// NewMultiTenantStateProvider creates a new tenant state provider.
func NewMultiTenantStateProvider(mtm *monitoring.MultiTenantMonitor, defaultM *monitoring.Monitor) *MultiTenantStateProvider {
	return &MultiTenantStateProvider{
		mtMonitor:      mtm,
		defaultMonitor: defaultM,
	}
}

// GetStateForTenant returns the state for a specific tenant.
func (p *MultiTenantStateProvider) GetStateForTenant(orgID string) models.StateSnapshot {
	// Default org uses the default monitor
	if orgID == "" || orgID == "default" {
		if p.defaultMonitor != nil {
			return p.defaultMonitor.GetState()
		}
		return models.StateSnapshot{}
	}

	// Try to get tenant-specific monitor
	if p.mtMonitor != nil {
		monitor, err := p.mtMonitor.GetMonitor(orgID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to get tenant monitor for state")
			// Fall back to default monitor
			if p.defaultMonitor != nil {
				return p.defaultMonitor.GetState()
			}
			return models.StateSnapshot{}
		}
		if monitor != nil {
			return monitor.GetState()
		}
	}

	// Fall back to default monitor
	if p.defaultMonitor != nil {
		return p.defaultMonitor.GetState()
	}
	return models.StateSnapshot{}
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

	// Wire tenant state provider to resource handlers
	if r.resourceHandlers != nil {
		r.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, r.monitor))
	}
}
