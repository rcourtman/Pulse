package api

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// baseAgentHandlers provides the shared monitor-management and broadcast logic
// for Docker, Kubernetes, and Unified Agent handler types.
type baseAgentHandlers struct {
	stateMu        sync.RWMutex
	mtMonitor      *monitoring.MultiTenantMonitor
	defaultMonitor *monitoring.Monitor
	wsHub          *websocket.Hub
}

// newBaseAgentHandlers constructs the shared base, resolving the default org
// monitor from the multi-tenant manager when no explicit monitor is given.
func newBaseAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub) baseAgentHandlers {
	if m == nil && mtm != nil {
		if mon, err := mtm.GetMonitor("default"); err == nil {
			m = mon
		}
	}
	return baseAgentHandlers{mtMonitor: mtm, defaultMonitor: m, wsHub: hub}
}

// SetMonitor updates the single-tenant monitor reference.
func (b *baseAgentHandlers) SetMonitor(m *monitoring.Monitor) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.defaultMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor manager and
// refreshes the default-org monitor fallback.
func (b *baseAgentHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	var defaultMonitor *monitoring.Monitor
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			defaultMonitor = m
		}
	}

	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.mtMonitor = mtm
	if defaultMonitor != nil {
		b.defaultMonitor = defaultMonitor
	}
}

// getMonitor returns the monitor for the organization in the request context.
// Only the default organization may fall back to the primary monitor. Any
// other organization whose tenant monitor cannot be resolved gets nil, so its
// requests never read or change the default organization's agents.
func (b *baseAgentHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	b.stateMu.RLock()
	mtMonitor := b.mtMonitor
	defaultMonitor := b.defaultMonitor
	b.stateMu.RUnlock()

	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" || orgID == "default" {
		if mtMonitor != nil {
			if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
				return m
			}
		}
		return defaultMonitor
	}

	if mtMonitor == nil {
		log.Warn().Str("org_id", orgID).Msg("Tenant monitor manager unavailable for agent request")
		return nil
	}
	m, err := mtMonitor.GetMonitor(orgID)
	if err != nil || m == nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to resolve tenant monitor for agent request")
		return nil
	}
	return m
}

// requireMonitor resolves the request organization's monitor, writing a 503
// and returning nil when it is unavailable.
func (b *baseAgentHandlers) requireMonitor(w http.ResponseWriter, r *http.Request) *monitoring.Monitor {
	monitor := b.getMonitor(r.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant monitor is not available", nil)
		return nil
	}
	return monitor
}

// broadcastState pushes monitor state to tenant-scoped WebSocket clients when
// context includes an org ID, falling back to global broadcast for default paths.
func (b *baseAgentHandlers) broadcastState(ctx context.Context) {
	if b.wsHub == nil {
		return
	}

	monitor := b.getMonitor(ctx)
	if monitor == nil {
		return
	}

	orgID := GetOrgID(ctx)
	if orgID != "" {
		if b.wsHub.GetTenantClientCount(orgID) == 0 {
			return
		}
	} else if b.wsHub.GetClientCount() == 0 {
		return
	}

	if orgID != "" {
		b.wsHub.BroadcastCurrentStateToTenant(orgID)
		return
	}
	b.wsHub.BroadcastCurrentState()
}
