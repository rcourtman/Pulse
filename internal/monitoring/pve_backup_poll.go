package monitoring

import (
	"context"
	"time"
)

// The owner outlives cancellation until the entire scan returns. Cadence,
// cancellation and permission to publish are separate facts.
type pveBackupPoll struct {
	ctx     context.Context
	cancel  context.CancelFunc
	client  PVEClientInterface
	started time.Time
}

type pveBackupPollContextKey struct{}

func (m *Monitor) beginPVEBackupPoll(instance string, now time.Time, client PVEClientInterface) (*pveBackupPoll, string) {
	if m == nil {
		return nil, "configuration unavailable"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if client == nil || m.pveClients[instance] != client {
		return nil, "connection changed"
	}
	parent := m.runtimeCtx
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil, "runtime ended"
	}
	if m.pveBackupPolls[instance] != nil {
		return nil, "previous backup scan is still running"
	}
	run, reason, next := m.shouldRunBackupPoll(m.lastPVEBackupPoll[instance], now)
	if !run {
		return nil, reason
	}
	if m.lastPVEBackupPoll == nil {
		m.lastPVEBackupPoll = make(map[string]time.Time)
	}
	if m.pveBackupPolls == nil {
		m.pveBackupPolls = make(map[string]*pveBackupPoll)
	}
	ctx, cancel := context.WithCancel(parent)
	owner := &pveBackupPoll{ctx: ctx, cancel: cancel, client: client, started: next}
	m.lastPVEBackupPoll[instance] = next
	m.pveBackupPolls[instance] = owner
	return owner, ""
}

func (m *Monitor) finishPVEBackupPoll(instance string, owner *pveBackupPoll) {
	defer owner.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pveBackupPolls[instance] != owner {
		return
	}
	// Never turn a withdrawn scan or a requested cadence reset into a fresh
	// completion time that delays the next ordinary scan.
	if m.lastPVEBackupPoll[instance].Equal(owner.started) {
		if owner.ctx.Err() == nil {
			m.lastPVEBackupPoll[instance] = time.Now()
		} else {
			delete(m.lastPVEBackupPoll, instance)
		}
	}
	delete(m.pveBackupPolls, instance)
}

// Called with m.mu and the publication writer held. Cancellation is not
// completion: a replacement waits for the old reader to return, without
// granting that reader permission to publish or renew its caches.
func (m *Monitor) cancelPVEBackupPollLocked(instance string) {
	if owner := m.pveBackupPolls[instance]; owner != nil {
		owner.cancel()
		delete(m.lastPVEBackupPoll, instance)
	}
}

// A child collection deadline is not connection withdrawal. Unread nodes and
// stores must still retain their earlier evidence when a live scan runs out of
// budget; an invalidated owner cannot admit any of that scan's effects.
func pveBackupPollWithdrawn(ctx context.Context) bool {
	owner, _ := ctx.Value(pveBackupPollContextKey{}).(*pveBackupPoll)
	return owner != nil && owner.ctx.Err() != nil
}

func (m *Monitor) applyPVEBackupEffect(ctx context.Context, instance string, effect func()) {
	owner, _ := ctx.Value(pveBackupPollContextKey{}).(*pveBackupPoll)
	if owner == nil {
		// Focused collectors retain their existing partial-budget semantics.
		effect()
		return
	}
	m.pveGuestPublicationMu.RLock()
	defer m.pveGuestPublicationMu.RUnlock()
	m.mu.RLock()
	current := owner.ctx.Err() == nil && m.pveBackupPolls[instance] == owner && m.pveClients[instance] == owner.client
	m.mu.RUnlock()
	if current {
		// Collection is outside this lock. State, cache, alert and recovery
		// admission cannot race connection retirement/replacement once accepted.
		effect()
	}
}
