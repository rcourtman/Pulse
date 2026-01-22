package monitoring

import (
	"context"
	"fmt"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// MultiTenantMonitor manages a dedicated Monitor instance for each organization.
type MultiTenantMonitor struct {
	mu           sync.RWMutex
	monitors     map[string]*Monitor
	persistence  *config.MultiTenantPersistence
	baseConfig   *config.Config
	wsHub        *websocket.Hub
	globalCtx    context.Context
	globalCancel context.CancelFunc
}

// NewMultiTenantMonitor creates a new multi-tenant monitor manager.
func NewMultiTenantMonitor(baseCfg *config.Config, persistence *config.MultiTenantPersistence, wsHub *websocket.Hub) *MultiTenantMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &MultiTenantMonitor{
		monitors:     make(map[string]*Monitor),
		persistence:  persistence,
		baseConfig:   baseCfg, // Used as a template or for global settings
		wsHub:        wsHub,
		globalCtx:    ctx,
		globalCancel: cancel,
	}
}

// GetMonitor returns the monitor instance for a specific organization.
// It lazily initializes the monitor if it doesn't exist.
func (mtm *MultiTenantMonitor) GetMonitor(orgID string) (*Monitor, error) {
	mtm.mu.RLock()
	monitor, exists := mtm.monitors[orgID]
	mtm.mu.RUnlock()

	if exists {
		return monitor, nil
	}

	mtm.mu.Lock()
	defer mtm.mu.Unlock()

	// Double-check locking pattern
	if monitor, exists = mtm.monitors[orgID]; exists {
		return monitor, nil
	}

	// Initialize new monitor for this tenant
	log.Info().Str("org_id", orgID).Msg("Initializing tenant monitor")

	// 1. Load Tenant Config
	// We need a specific config for this tenant.
	// For now, we clone the base config (assuming shared defaults)
	// In the future, we'll load overrides from persistence.GetPersistence(orgID)
	tenantConfig := *mtm.baseConfig // Shallow copy

	// Ensure the DataPath is correct for this tenant to isolate storage (sqlite, etc)
	tenantPersistence, err := mtm.persistence.GetPersistence(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get persistence for org %s: %w", orgID, err)
	}
	tenantConfig.DataPath = tenantPersistence.GetConfigDir()

	// 2. Create Monitor
	// Usage of internal New constructor
	monitor, err = New(&tenantConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor for org %s: %w", orgID, err)
	}

	// 3. Start Monitor
	// We pass the global context, but maybe we should give it a derived one?
	// Using globalCtx ensures all monitors stop when MultiTenantMonitor stops.
	// NOTE: Monitor.Start is async
	go monitor.Start(mtm.globalCtx, mtm.wsHub)

	mtm.monitors[orgID] = monitor
	return monitor, nil
}

// Stop stops all tenant monitors.
func (mtm *MultiTenantMonitor) Stop() {
	mtm.mu.Lock()
	defer mtm.mu.Unlock()

	log.Info().Msg("Stopping MultiTenantMonitor and all tenant instances")
	mtm.globalCancel()

	for _, monitor := range mtm.monitors {
		monitor.Stop()
	}
	// Clear map
	mtm.monitors = make(map[string]*Monitor)
}

// RemoveTenant stops and removes a specific tenant's monitor.
// Useful for offboarding or manual reloading.
func (mtm *MultiTenantMonitor) RemoveTenant(orgID string) {
	mtm.mu.Lock()
	defer mtm.mu.Unlock()

	if monitor, exists := mtm.monitors[orgID]; exists {
		log.Info().Str("org_id", orgID).Msg("Stopping and removing tenant monitor")
		monitor.Stop()
		delete(mtm.monitors, orgID)
	}
}
