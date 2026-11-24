package monitoring

import (
	"slices"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// deadManAlertProjection keeps alert-manager locks and history writes off the
// watchdog's signalling path. Only the projection worker calls the manager.
// There is at most one pending standing update per watchdog alert type, plus
// a history observation for restart gaps and unreadable restart state. Those
// observations must be recorded even if a later success has already cleared
// the condition. An update already in the manager finishes before its newer
// replacement, so delayed work cannot leave an old condition standing.
type deadManAlertProjection struct {
	mu      sync.Mutex
	pending []deadManAlertUpdate
	wake    chan struct{}
	closed  bool
}

type deadManAlertUpdate struct {
	alertType string
	raise     *alerts.SystemAlertInput // nil clears the standing condition
	history   bool
}

func (p *deadManAlertProjection) raise(input alerts.SystemAlertInput) {
	p.submit(deadManAlertUpdate{alertType: input.Type, raise: &input})
}

func (p *deadManAlertProjection) record(input alerts.SystemAlertInput) {
	p.submit(deadManAlertUpdate{alertType: input.Type, raise: &input, history: true})
}

func (p *deadManAlertProjection) clear(alertType string) {
	p.submit(deadManAlertUpdate{alertType: alertType})
}

func (p *deadManAlertProjection) submit(update deadManAlertUpdate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.pending = slices.DeleteFunc(p.pending, func(pending deadManAlertUpdate) bool {
		return pending.alertType == update.alertType && (!pending.history || update.history)
	})
	p.pending = append(p.pending, update)
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// close stops admission and lets the worker finish the already accepted
// projection before Monitor.Start releases ownership of the alert store.
func (p *deadManAlertProjection) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		close(p.wake)
	}
}

func (p *deadManAlertProjection) run(manager *alerts.Manager) {
	for range p.wake {
		p.drain(manager)
	}
}

func (p *deadManAlertProjection) drain(manager *alerts.Manager) {
	for {
		p.mu.Lock()
		if len(p.pending) == 0 {
			p.mu.Unlock()
			return
		}
		update := p.pending[0]
		p.pending = slices.Delete(p.pending, 0, 1)
		p.mu.Unlock()
		if update.raise != nil {
			manager.RaiseSystemAlert(*update.raise)
		} else {
			manager.ClearSystemAlert(update.alertType)
		}
	}
}
