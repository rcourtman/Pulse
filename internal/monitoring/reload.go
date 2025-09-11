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
	monitor     *Monitor
	config      *config.Config
	wsHub       *websocket.Hub
	ctx         context.Context
	cancel      context.CancelFunc
	parentCtx   context.Context
	reloadChan  chan struct{}
}

// NewReloadableMonitor creates a new reloadable monitor
func NewReloadableMonitor(cfg *config.Config, wsHub *websocket.Hub) (*ReloadableMonitor, error) {
	monitor, err := New(cfg)
	if err != nil {
		return nil, err
	}

	rm := &ReloadableMonitor{
		monitor:    monitor,
		config:     cfg,
		wsHub:      wsHub,
		reloadChan: make(chan struct{}, 1),
	}

	return rm, nil
}

// Start starts the monitor with reload capability
func (rm *ReloadableMonitor) Start(ctx context.Context) {
	rm.mu.Lock()
	rm.parentCtx = ctx
	rm.ctx, rm.cancel = context.WithCancel(ctx)
	rm.mu.Unlock()

	// Start the monitor
	go rm.monitor.Start(rm.ctx, rm.wsHub)

	// Watch for reload signals
	go rm.watchReload(ctx)
}

// Reload triggers a monitor reload
func (rm *ReloadableMonitor) Reload() error {
	select {
	case rm.reloadChan <- struct{}{}:
		return nil
	default:
		// Channel is full, reload already pending
		return nil
	}
}

// watchReload watches for reload signals
func (rm *ReloadableMonitor) watchReload(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.reloadChan:
			log.Info().Msg("Reloading monitor configuration")
			if err := rm.doReload(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor")
			} else {
				log.Info().Msg("Monitor reloaded successfully")
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

	// Note: Polling interval is now hardcoded to 10s, no special handling needed

	// For other changes, do a full reload
	log.Info().Msg("Performing full monitor reload")
	
	// Cancel current monitor
	if rm.cancel != nil {
		rm.cancel()
	}

	// Wait a moment for cleanup
	time.Sleep(1 * time.Second)

	// Create new monitor
	newMonitor, err := New(cfg)
	if err != nil {
		// Restart old monitor if new one fails
		rm.ctx, rm.cancel = context.WithCancel(rm.parentCtx)
		go rm.monitor.Start(rm.ctx, rm.wsHub)
		return err
	}

	// Replace monitor
	rm.monitor = newMonitor
	rm.config = cfg

	// Start new monitor
	rm.ctx, rm.cancel = context.WithCancel(rm.parentCtx)
	go rm.monitor.Start(rm.ctx, rm.wsHub)

	return nil
}

// Note: Polling interval change detection removed - now hardcoded to 10s

// GetMonitor returns the current monitor instance
func (rm *ReloadableMonitor) GetMonitor() *Monitor {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.monitor
}

// GetState returns the current state
func (rm *ReloadableMonitor) GetState() interface{} {
	return rm.GetMonitor().GetState()
}

// Stop stops the monitor
func (rm *ReloadableMonitor) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if rm.cancel != nil {
		rm.cancel()
	}
	
	if rm.monitor != nil {
		rm.monitor.Stop()
	}
}

// Note: UpdatePollingInterval removed - polling interval is now hardcoded to 10s