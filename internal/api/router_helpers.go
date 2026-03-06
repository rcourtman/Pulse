package api

import (
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

func (p *MultiTenantStateProvider) monitorForTenant(orgID string) *monitoring.Monitor {
	if orgID == "" || orgID == "default" {
		return p.defaultMonitor
	}

	if p.mtMonitor != nil {
		monitor, err := p.mtMonitor.GetMonitor(orgID)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to get tenant monitor")
			return nil
		}
		return monitor
	}

	return nil
}

// UnifiedReadStateForTenant returns the canonical typed unified read-state for a
// specific tenant, falling back to a snapshot-backed adapter only when the
// monitor has not been wired with a resource store yet.
func (p *MultiTenantStateProvider) UnifiedReadStateForTenant(orgID string) unifiedresources.ReadState {
	monitor := p.monitorForTenant(orgID)
	if monitor == nil {
		return nil
	}
	return monitor.GetUnifiedReadStateOrSnapshot()
}

// GetStateForTenant returns the legacy state snapshot bridge for a specific
// tenant. New code should prefer UnifiedReadStateForTenant.
func (p *MultiTenantStateProvider) GetStateForTenant(orgID string) models.StateSnapshot {
	monitor := p.monitorForTenant(orgID)
	if monitor == nil {
		return models.StateSnapshot{}
	}
	return monitor.ReadSnapshot()
}

// UnifiedResourceSnapshotForTenant returns the canonical unified-resource seed
// for a specific tenant, along with its freshness marker.
func (p *MultiTenantStateProvider) UnifiedResourceSnapshotForTenant(orgID string) ([]unifiedresources.Resource, time.Time) {
	monitor := p.monitorForTenant(orgID)
	if monitor == nil {
		return nil, time.Time{}
	}

	return monitor.UnifiedResourceSnapshot()
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
		mtm.SetMonitorInitializer(r.configureMonitorDependencies)
	}

	// Wire tenant state provider to resource handlers
	if r.resourceHandlers != nil {
		r.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(mtm, r.monitor))
	}
}
