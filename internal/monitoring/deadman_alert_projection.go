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
// the condition. A newer update can also revoke the one standing raise that
// has left the queue but has not yet been admitted by the manager.
type deadManAlertProjection struct {
	mu       sync.Mutex
	pending  []deadManAlertUpdate
	inFlight *deadManStandingRaise
	wake     chan struct{}
	closed   bool
}

type deadManStandingRaise struct {
	alertType  string
	superseded chan struct{}
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
	p.revokeInFlightLocked(update.alertType)
	p.pending = slices.DeleteFunc(p.pending, func(pending deadManAlertUpdate) bool {
		return pending.alertType == update.alertType && (!pending.history || update.history)
	})
	p.pending = append(p.pending, update)
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *deadManAlertProjection) revokeInFlightLocked(alertType string) {
	if p.inFlight != nil && p.inFlight.alertType == alertType {
		close(p.inFlight.superseded)
		p.inFlight = nil
	}
}

// discardStandingRaises invalidates old destination-dependent raises on a
// saved configuration change, without waiting for the next watchdog cycle.
// Clears and actual historical observations still belong to the worker.
func (p *deadManAlertProjection) discardStandingRaises(alertTypes ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, alertType := range alertTypes {
		p.revokeInFlightLocked(alertType)
	}
	p.pending = slices.DeleteFunc(p.pending, func(update deadManAlertUpdate) bool {
		return update.raise != nil && !update.history && slices.Contains(alertTypes, update.alertType)
	})
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
		p.inFlight = nil // the sole worker has finished the preceding manager call
		if len(p.pending) == 0 {
			p.mu.Unlock()
			return
		}
		update := p.pending[0]
		p.pending = slices.Delete(p.pending, 0, 1)
		var superseded <-chan struct{}
		if update.raise != nil && !update.history {
			p.inFlight = &deadManStandingRaise{alertType: update.alertType, superseded: make(chan struct{})}
			superseded = p.inFlight.superseded
		}
		p.mu.Unlock()
		if update.raise != nil {
			manager.RaiseSystemAlertUnlessSuperseded(*update.raise, superseded)
		} else {
			manager.ClearSystemAlert(update.alertType)
		}
	}
}
