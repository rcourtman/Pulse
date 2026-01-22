package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

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
			log.Info().Msg("Reloading monitor configuration")
			if err := rm.doReload(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor")
				done <- err
			} else {
				log.Info().Msg("Monitor reloaded successfully")
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
		return err
	}

	// Polling interval changes and other settings require a full reload
	log.Info().Msg("Performing full monitor reload")

	// Cancel current monitor
	if rm.cancel != nil {
		rm.cancel()
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
	m, _ := rm.mtMonitor.GetMonitor("default")
	return m
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

// GetState returns the current state
func (rm *ReloadableMonitor) GetState() interface{} {
	// For backward compatibility / frontend simplicity, return default org state
	// TODO: Make WebSocket state getter tenant-aware
	monitor, err := rm.GetMultiTenantMonitor().GetMonitor("default")
	if err != nil {
		return nil
	}
	return monitor.GetState()
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
