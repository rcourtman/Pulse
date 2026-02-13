package monitoring

import (
	"context"
	"fmt"
	"strings"
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
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, fmt.Errorf("organization ID is required")
	}

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
	log.Info().Str("org_id", orgID).Msg("initializing tenant monitor")

	// 1. Load Tenant Config
	// Deep copy the base config to ensure tenant isolation.
	// Each tenant gets its own independent config that won't share
	// credential slices or other mutable state with other tenants.
	tenantConfig := mtm.baseConfig.DeepCopy()

	// Clear inherited credentials - tenants must load their own
	// This prevents credential leakage between tenants
	tenantConfig.PVEInstances = nil
	tenantConfig.PBSInstances = nil
	tenantConfig.PMGInstances = nil

	// Ensure the DataPath is correct for this tenant to isolate storage (sqlite, etc)
	tenantPersistence, err := mtm.persistence.GetPersistence(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get persistence for org %s: %w", orgID, err)
	}
	tenantConfig.DataPath = tenantPersistence.GetConfigDir()

	// Load tenant-specific nodes from <orgDir>/nodes.enc
	nodesConfig, err := tenantPersistence.LoadNodesConfig()
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("failed to load tenant nodes config, starting with empty config")
		// Not a fatal error - tenant may not have configured any nodes yet
	} else if nodesConfig != nil {
		tenantConfig.PVEInstances = nodesConfig.PVEInstances
		tenantConfig.PBSInstances = nodesConfig.PBSInstances
		tenantConfig.PMGInstances = nodesConfig.PMGInstances
		log.Info().
			Str("org_id", orgID).
			Int("pve_count", len(nodesConfig.PVEInstances)).
			Int("pbs_count", len(nodesConfig.PBSInstances)).
			Int("pmg_count", len(nodesConfig.PMGInstances)).
			Msg("Loaded tenant nodes config")
	}

	// 2. Create Monitor
	// Usage of internal New constructor
	monitor, err = New(tenantConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor for org %s: %w", orgID, err)
	}

	// Set org ID for tenant isolation
	// This enables tenant-scoped WebSocket broadcasts
	monitor.SetOrgID(orgID)

	// 3. Start Monitor
	// We pass the global context, but maybe we should give it a derived one?
	// Using globalCtx ensures all monitors stop when MultiTenantMonitor stops.
	// NOTE: Monitor.Start is async
	go monitor.Start(mtm.globalCtx, mtm.wsHub)

	mtm.monitors[orgID] = monitor
	return monitor, nil
}

// PeekMonitor returns the tenant monitor instance if it is already initialized.
// It does not create a new monitor.
func (mtm *MultiTenantMonitor) PeekMonitor(orgID string) (*Monitor, bool) {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return nil, false
	}

	mtm.mu.RLock()
	defer mtm.mu.RUnlock()
	monitor, exists := mtm.monitors[orgID]
	return monitor, exists
}

// Stop stops all tenant monitors.
func (mtm *MultiTenantMonitor) Stop() {
	mtm.mu.Lock()
	defer mtm.mu.Unlock()

	log.Info().Msg("stopping MultiTenantMonitor and all tenant instances")
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
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return
	}

	mtm.mu.Lock()
	defer mtm.mu.Unlock()

	if monitor, exists := mtm.monitors[orgID]; exists {
		log.Info().Str("org_id", orgID).Msg("stopping and removing tenant monitor")
		monitor.Stop()
		delete(mtm.monitors, orgID)
	}
}

// OrgExists checks if an organization exists (directory exists) without creating it.
func (mtm *MultiTenantMonitor) OrgExists(orgID string) bool {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return false
	}

	if mtm.persistence == nil {
		return false
	}
	return mtm.persistence.OrgExists(orgID)
}
