package monitoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// InstallSnapshotCounts holds install-wide resource and alert counts aggregated
// across tenant monitors.
type InstallSnapshotCounts struct {
	PVENodes           int
	PBSInstances       int
	PMGInstances       int
	VMs                int
	Containers         int
	DockerHosts        int
	KubernetesClusters int
	ActiveAlerts       int
}

// ReloadableMonitor wraps a Monitor with reload capability
type ReloadableMonitor struct {
	mu          sync.RWMutex
	mtMonitor   *MultiTenantMonitor
	persistence *config.MultiTenantPersistence
	config      *config.Config
	wsHub       *websocket.Hub
	ctx         context.Context
	cancel      context.CancelFunc
	parentCtx   context.Context
	reloadChan  chan chan error
}

// NewReloadableMonitor creates a new reloadable monitor
func NewReloadableMonitor(cfg *config.Config, persistence *config.MultiTenantPersistence, wsHub *websocket.Hub) (*ReloadableMonitor, error) {
	mtMonitor := NewMultiTenantMonitor(cfg, persistence, wsHub)
	// No error check needed for NewMultiTenantMonitor as it doesn't return error

	rm := &ReloadableMonitor{
		mtMonitor:   mtMonitor,
		config:      cfg,
		persistence: persistence,
		wsHub:       wsHub,
		reloadChan:  make(chan chan error, 1),
	}

	return rm, nil
}

// Start starts the monitor with reload capability
func (rm *ReloadableMonitor) Start(ctx context.Context) {
	rm.mu.Lock()
	rm.parentCtx = ctx
	rm.ctx, rm.cancel = context.WithCancel(ctx)
	rm.mu.Unlock()

	// Start the multi-tenant monitor manager
	// Note: It doesn't start individual monitors until requested via GetMonitor()
	// But we might want to start "default" monitor if it exists?
	// For now, lazy loading handles it.

	// Watch for reload signals
	go rm.watchReload(ctx)
}

// Reload triggers a monitor reload
func (rm *ReloadableMonitor) Reload() error {
	done := make(chan error, 1)
	rm.reloadChan <- done
	return <-done
}

// watchReload watches for reload signals
func (rm *ReloadableMonitor) watchReload(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case done := <-rm.reloadChan:
			log.Info().Msg("reloading monitor configuration")
			if err := rm.doReload(); err != nil {
				log.Error().Err(err).Msg("failed to reload monitor")
				done <- err
			} else {
				log.Info().Msg("monitor reloaded successfully")
				done <- nil
			}
		}
	}
}

// doReload performs the actual reload
func (rm *ReloadableMonitor) doReload() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Load fresh configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("reloadable monitor reload config: %w", err)
	}

	// Polling interval changes and other settings require a full reload
	log.Info().Msg("performing full monitor reload")

	// Cancel current monitor
	if rm.cancel != nil {
		rm.cancel()
	}

	// Stop the underlying multi-tenant monitor to ensure its global context is canceled
	// and all child monitors are stopped.
	if rm.mtMonitor != nil {
		rm.mtMonitor.Stop()
	}

	// Wait a moment for cleanup
	time.Sleep(1 * time.Second)

	// Create new multi-tenant monitor
	// Note: We lose existing instances state here, which is expected on full reload.
	newMTMonitor := NewMultiTenantMonitor(cfg, rm.persistence, rm.wsHub)

	// Replace monitor
	rm.mtMonitor = newMTMonitor
	rm.config = cfg

	// Start new monitor context (individual monitors are lazy loaded/started)
	rm.ctx, rm.cancel = context.WithCancel(rm.parentCtx)

	return nil
}

// GetMultiTenantMonitor returns the current multi-tenant monitor instance
func (rm *ReloadableMonitor) GetMultiTenantMonitor() *MultiTenantMonitor {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.mtMonitor
}

// GetMonitor returns the default monitor instance (compatibility shim)
// It ensures the "default" tenant is initialized.
func (rm *ReloadableMonitor) GetMonitor() *Monitor {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if rm.mtMonitor == nil {
		return nil
	}
	monitor, err := rm.mtMonitor.GetMonitor("default")
	if err != nil {
		log.Debug().Err(err).Msg("Default monitor unavailable")
		return nil
	}
	return monitor
}

// GetConfig returns the current configuration used by the monitor.
func (rm *ReloadableMonitor) GetConfig() *config.Config {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if rm.config == nil {
		return nil
	}
	return rm.config
}

// ReadSnapshot returns the current state for a specific tenant.
func (rm *ReloadableMonitor) ReadSnapshot(orgID string) interface{} {
	if orgID == "" {
		orgID = "default"
	}
	mtMonitor := rm.GetMultiTenantMonitor()
	if mtMonitor == nil {
		log.Debug().Str("orgID", orgID).Msg("ReadSnapshot requested with no active multi-tenant monitor")
		return nil
	}

	monitor, err := mtMonitor.GetMonitor(orgID)
	if err != nil {
		log.Debug().Err(err).Str("orgID", orgID).Msg("ReadSnapshot monitor unavailable")
		return nil
	}
	return monitor.ReadSnapshot()
}

// AggregateInstallSnapshotCounts returns install-wide resource and alert counts
// across all provisioned organizations.
func (rm *ReloadableMonitor) AggregateInstallSnapshotCounts() InstallSnapshotCounts {
	rm.mu.RLock()
	mtMonitor := rm.mtMonitor
	persistence := rm.persistence
	rm.mu.RUnlock()

	if mtMonitor == nil {
		return InstallSnapshotCounts{}
	}

	orgIDs := []string{"default"}
	if persistence != nil {
		orgs, err := persistence.ListOrganizations()
		if err != nil {
			log.Warn().Err(err).Msg("Telemetry snapshot falling back to default organization after tenant listing failed")
		} else {
			seen := make(map[string]struct{}, len(orgs))
			orgIDs = orgIDs[:0]
			for _, org := range orgs {
				if org == nil {
					continue
				}
				orgID := strings.TrimSpace(org.ID)
				if orgID == "" {
					continue
				}
				if _, exists := seen[orgID]; exists {
					continue
				}
				seen[orgID] = struct{}{}
				orgIDs = append(orgIDs, orgID)
			}
			if len(orgIDs) == 0 {
				orgIDs = []string{"default"}
			}
		}
	}

	var counts InstallSnapshotCounts
	for _, orgID := range orgIDs {
		monitor, err := mtMonitor.GetMonitor(orgID)
		if err != nil || monitor == nil {
			log.Debug().Err(err).Str("org_id", orgID).Msg("Telemetry snapshot could not load tenant monitor")
			continue
		}
		accumulateInstallSnapshotCounts(&counts, monitor)
	}
	return counts
}

func accumulateInstallSnapshotCounts(counts *InstallSnapshotCounts, monitor *Monitor) {
	if counts == nil || monitor == nil {
		return
	}

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState != nil {
		counts.PVENodes += len(readState.Nodes())
		counts.PBSInstances += len(readState.PBSInstances())
		counts.PMGInstances += len(readState.PMGInstances())
		counts.VMs += len(readState.VMs())
		counts.Containers += len(readState.Containers())
		counts.DockerHosts += len(readState.DockerHosts())
		counts.KubernetesClusters += len(readState.K8sClusters())
	}
	counts.ActiveAlerts += len(monitor.ActiveAlertsSnapshot())
}

// Stop stops the monitor
func (rm *ReloadableMonitor) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.cancel != nil {
		rm.cancel()
	}

	if rm.mtMonitor != nil {
		rm.mtMonitor.Stop()
	}
}
