package api

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const legacyGrandfatherReconcileInterval = 5 * time.Second

type legacyGrandfatherReconcileLoop struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

func (l *legacyGrandfatherReconcileLoop) isRunning() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (h *LicenseHandlers) legacyGrandfatherReconcileLoop(orgID string) *legacyGrandfatherReconcileLoop {
	if h == nil {
		return nil
	}
	if loop, ok := h.legacyGrandfatherReconcile.Load(orgID); ok {
		if typed, ok := loop.(*legacyGrandfatherReconcileLoop); ok {
			return typed
		}
	}
	loop := &legacyGrandfatherReconcileLoop{}
	actual, _ := h.legacyGrandfatherReconcile.LoadOrStore(orgID, loop)
	if typed, ok := actual.(*legacyGrandfatherReconcileLoop); ok {
		return typed
	}
	return loop
}

func (h *LicenseHandlers) ensureLegacyGrandfatherReconcileLoop(orgID string, service *licenseService) {
	if h == nil || service == nil {
		return
	}
	orgID = normalizeHostedEntitlementOrgID(orgID)
	if !service.NeedsLegacyMonitoredSystemCapture() {
		h.stopLegacyGrandfatherReconcileLoop(orgID)
		return
	}

	loop := h.legacyGrandfatherReconcileLoop(orgID)
	if loop == nil {
		return
	}
	if loop.isRunning() {
		return
	}

	loop.mu.Lock()
	defer loop.mu.Unlock()
	if loop.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	loop.cancel = cancel
	loop.running = true
	loop.wg.Add(1)
	go func() {
		defer func() {
			loop.mu.Lock()
			loop.running = false
			loop.mu.Unlock()
			h.legacyGrandfatherReconcile.Delete(orgID)
			loop.wg.Done()
		}()
		h.runLegacyGrandfatherReconcileLoop(ctx, orgID, service)
	}()
}

func (h *LicenseHandlers) stopLegacyGrandfatherReconcileLoop(orgID string) {
	if h == nil {
		return
	}
	orgID = normalizeHostedEntitlementOrgID(orgID)
	value, ok := h.legacyGrandfatherReconcile.Load(orgID)
	if !ok {
		return
	}
	loop, ok := value.(*legacyGrandfatherReconcileLoop)
	if !ok || loop == nil {
		h.legacyGrandfatherReconcile.Delete(orgID)
		return
	}

	loop.mu.Lock()
	if !loop.running {
		loop.mu.Unlock()
		h.legacyGrandfatherReconcile.Delete(orgID)
		return
	}
	cancel := loop.cancel
	loop.running = false
	loop.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	loop.wg.Wait()
	h.legacyGrandfatherReconcile.Delete(orgID)
}

func (h *LicenseHandlers) runLegacyGrandfatherReconcileLoop(
	ctx context.Context,
	orgID string,
	service *licenseService,
) {
	ticker := time.NewTicker(legacyGrandfatherReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcileCtx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
			h.reconcileLegacyMigrationGrandfatherFloor(reconcileCtx, orgID, service)
			if !service.NeedsLegacyMonitoredSystemCapture() {
				return
			}
		}
	}
}

func (h *LicenseHandlers) reconcileLegacyMigrationGrandfatherFloor(
	ctx context.Context,
	orgID string,
	service *licenseService,
) {
	if h == nil || service == nil {
		return
	}

	orgID = normalizeHostedEntitlementOrgID(orgID)
	if !service.NeedsLegacyMonitoredSystemCapture() {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if resolved := GetOrgID(ctx); resolved == "" {
		ctx = context.WithValue(ctx, OrgIDContextKey, orgID)
	}

	count, ok := h.canonicalMonitoredSystemGrandfatherFloor(ctx)
	if !ok {
		h.ensureLegacyGrandfatherReconcileLoop(orgID, service)
		return
	}

	if err := service.CaptureLegacyMonitoredSystemGrandfatherFloor(count); err != nil {
		log.Warn().
			Str("org_id", orgID).
			Int("monitored_systems", count).
			Err(err).
			Msg("Failed to persist migrated monitored-system grandfather floor")
		h.ensureLegacyGrandfatherReconcileLoop(orgID, service)
		return
	}

	if service.NeedsLegacyMonitoredSystemCapture() {
		h.ensureLegacyGrandfatherReconcileLoop(orgID, service)
	}
}
