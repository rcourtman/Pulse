package api

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// baseAgentHandlers provides the shared monitor-management and broadcast logic
// for Docker, Kubernetes, and Host agent handler types.
type baseAgentHandlers struct {
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
	b.legacyMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor manager and
// refreshes the legacy monitor from the "default" organization.
func (b *baseAgentHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	b.mtMonitor = mtm
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			b.legacyMonitor = m
		}
	}
}

// getMonitor returns the monitor for the organization in the request context,
// falling back to the legacy single-tenant monitor.
func (b *baseAgentHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	orgID := GetOrgID(ctx)
	if b.mtMonitor != nil {
		if m, err := b.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m
		}
	}
	return b.legacyMonitor
}

// broadcastState pushes monitor state to tenant-scoped WebSocket clients when
// context includes an org ID, falling back to global broadcast for legacy paths.
func (b *baseAgentHandlers) broadcastState(ctx context.Context) {
	if b.wsHub == nil {
		return
	}

	state := b.getMonitor(ctx).GetState().ToFrontend()
	orgID := GetOrgID(ctx)
	if orgID != "" {
		go b.wsHub.BroadcastStateToTenant(orgID, state)
		return
	}
	go b.wsHub.BroadcastState(state)
}
