package api

import (
	"context"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// baseAgentHandlers provides the shared monitor-management and broadcast logic
// for Docker, Kubernetes, and Host agent handler types.
type baseAgentHandlers struct {
	stateMu       sync.RWMutex
	mtMonitor     *monitoring.MultiTenantMonitor
	legacyMonitor *monitoring.Monitor
	wsHub         *websocket.Hub
}

// newBaseAgentHandlers constructs the shared base, resolving the default org
// monitor from the multi-tenant manager when no explicit monitor is given.
func newBaseAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub) baseAgentHandlers {
	if m == nil && mtm != nil {
		if mon, err := mtm.GetMonitor("default"); err == nil {
			m = mon
		}
	}
	return baseAgentHandlers{mtMonitor: mtm, legacyMonitor: m, wsHub: hub}
}

// SetMonitor updates the single-tenant monitor reference.
func (b *baseAgentHandlers) SetMonitor(m *monitoring.Monitor) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.legacyMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor manager and
// refreshes the legacy monitor from the "default" organization.
func (b *baseAgentHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	var legacy *monitoring.Monitor
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			legacy = m
		}
	}

	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.mtMonitor = mtm
	if legacy != nil {
		b.legacyMonitor = legacy
	}
}

// getMonitor returns the monitor for the organization in the request context,
// falling back to the legacy single-tenant monitor.
func (b *baseAgentHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	b.stateMu.RLock()
	mtMonitor := b.mtMonitor
	legacyMonitor := b.legacyMonitor
	b.stateMu.RUnlock()

	orgID := GetOrgID(ctx)
	if mtMonitor != nil {
		if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m
		}
	}
	return legacyMonitor
}

// broadcastState pushes monitor state to tenant-scoped WebSocket clients when
// context includes an org ID, falling back to global broadcast for legacy paths.
func (b *baseAgentHandlers) broadcastState(ctx context.Context) {
	if b.wsHub == nil {
		return
	}

	monitor := b.getMonitor(ctx)
	if monitor == nil {
		return
	}

	state := monitor.GetState().ToFrontend()
	orgID := GetOrgID(ctx)
	if orgID != "" {
		go b.wsHub.BroadcastStateToTenant(orgID, state)
		return
	}
	go b.wsHub.BroadcastState(state)
}
