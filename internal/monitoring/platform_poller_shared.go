package monitoring

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// startPollerLoop runs the scheduling scaffold shared by the periodic
// platform pollers (TrueNAS, VMware): guard against double-start under mu,
// then sync + poll immediately and again on every nextWaitDuration tick until
// the context is cancelled. cancelSlot and stoppedSlot point at the poller's
// lifecycle fields so Stop keeps working against the same state.
func startPollerLoop(
	ctx context.Context,
	mu *sync.Mutex,
	cancelSlot *context.CancelFunc,
	stoppedSlot *chan struct{},
	syncConnections func(),
	pollAll func(context.Context),
	nextWaitDuration func(time.Time) time.Duration,
) {
	if ctx == nil {
		ctx = context.Background()
	}

	mu.Lock()
	if *cancelSlot != nil {
		mu.Unlock()
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	*cancelSlot = cancel
	*stoppedSlot = make(chan struct{})
	stopped := *stoppedSlot
	mu.Unlock()

	go func() {
		defer close(stopped)
		defer func() {
			mu.Lock()
			if *stoppedSlot == stopped {
				*cancelSlot = nil
			}
			mu.Unlock()
		}()

		syncConnections()
		pollAll(runCtx)
		for {
			wait := nextWaitDuration(time.Now())
			timer := time.NewTimer(wait)
			select {
			case <-runCtx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
				syncConnections()
				pollAll(runCtx)
			}
		}
	}()
}

// loadActiveInstanceConfigs resolves the active platform connection configs
// for an org: prefer the poller's cached map, otherwise load from per-org
// persistence and keep enabled instances keyed by trimmed connection ID with
// defaults applied. Shared by the TrueNAS and VMware pollers so the "what
// counts as an active connection" policy stays single-sourced.
func loadActiveInstanceConfigs[T any](
	cached map[string]T,
	multiTenant *config.MultiTenantPersistence,
	orgID string,
	load func(*config.ConfigPersistence) ([]T, error),
	applyDefaults func(*T),
	enabled func(T) bool,
	connID func(T) string,
) map[string]T {
	if len(cached) > 0 || multiTenant == nil {
		return cached
	}

	persistence, err := multiTenant.GetPersistence(orgID)
	if err != nil || persistence == nil {
		return cached
	}
	instances, err := load(persistence)
	if err != nil {
		return cached
	}

	active := make(map[string]T)
	for i := range instances {
		instance := instances[i]
		applyDefaults(&instance)
		if !enabled(instance) {
			continue
		}
		id := strings.TrimSpace(connID(instance))
		if id == "" {
			continue
		}
		active[id] = instance
	}
	return active
}
