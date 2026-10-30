package monitoring

import (
	"context"
	"strings"
	"time"
)

// One owner covers tasks, storage and snapshots. Their existing independent
// budgets remain children of this runtime-scoped context, not a core cycle.
// Cancellation is not completion: ordinary cycles cannot stack another scan
// behind a client that has not returned yet.
type pveBackupPoll struct {
	ctx    context.Context
	cancel context.CancelFunc
	client PVEClientInterface
	done   chan struct{}
}

type pveBackupPollContextKey struct{}

func (m *Monitor) beginPVEBackupPoll(instance string, client PVEClientInterface, last, next time.Time) *pveBackupPoll {
	m.pveGuestPublicationMu.RLock()
	defer m.pveGuestPublicationMu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if client == nil || m.pveClients[instance] != client || !m.lastPVEBackupPoll[instance].Equal(last) {
		return nil
	}
	parent := m.runtimeCtx
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil
	}
	if previous := m.pveBackupPolls[instance]; previous != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	owner := &pveBackupPoll{ctx: ctx, cancel: cancel, client: client, done: make(chan struct{})}
	if m.pveBackupPolls == nil {
		m.pveBackupPolls = make(map[string]*pveBackupPoll)
	}
	if m.lastPVEBackupPoll == nil {
		m.lastPVEBackupPoll = make(map[string]time.Time)
	}
	m.pveBackupPolls[instance] = owner
	m.lastPVEBackupPoll[instance] = next
	return owner
}

func (m *Monitor) finishPVEBackupPoll(instance string, owner *pveBackupPoll) {
	m.mu.Lock()
	if m.pveBackupPolls[instance] == owner {
		delete(m.pveBackupPolls, instance)
		if owner.ctx.Err() == nil && m.pveClients[instance] == owner.client {
			m.lastPVEBackupPoll[instance] = time.Now()
		}
	}
	m.mu.Unlock()
	owner.cancel()
	close(owner.done)
}

// Called with m.mu and the connection-publication writer held. A replacement
// may start immediately; the old owner cannot publish or release the new claim.
func (m *Monitor) cancelPVEBackupPollLocked(instance string) {
	if owner := m.pveBackupPolls[instance]; owner != nil {
		owner.cancel()
		delete(m.pveBackupPolls, instance)
	}
	// A newly bound client needs its own first scan even when the old one
	// completed before replacement and its configured interval has not elapsed.
	delete(m.lastPVEBackupPoll, instance)
}

// Hold the connection publication reader across local acceptance, never across
// a provider read. Retirement/replacement uses the writer, so checking the
// owner and publishing its effects is atomic with respect to either transition.
// Already accepted/enqueued historical evidence remains valid; late reads do not.
func (m *Monitor) beginPVEBackupPublication(ctx context.Context, instance string) (func(), bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	owner, _ := ctx.Value(pveBackupPollContextKey{}).(*pveBackupPoll)
	if owner == nil {
		return func() {}, true // focused collectors retain their normal behaviour
	}
	m.pveGuestPublicationMu.RLock()
	m.mu.RLock()
	current := ctx.Err() == nil && owner.ctx.Err() == nil && m.pveBackupPolls[instance] == owner && m.pveClients[instance] == owner.client
	m.mu.RUnlock()
	if !current {
		m.pveGuestPublicationMu.RUnlock()
		return nil, false
	}
	return m.pveGuestPublicationMu.RUnlock, true
}

// Connection identity, not just a transport pointer, bounds cached task logs.
// Keep accepted live state until an ordinary new read replaces it, but never
// let a reused name/UPID borrow another connection's immutable guest results.
func (m *Monitor) discardPVEBackupTaskCacheLocked(instance string) {
	delete(m.vzdumpJobLogHistoryFirst, instance)
	for key := range m.vzdumpJobTaskCache {
		if strings.HasPrefix(key, instance+"|") {
			delete(m.vzdumpJobTaskCache, key)
		}
	}
}
