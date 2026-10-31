package monitoring

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// A collection budget belongs to a single runtime/client binding. Spending
// that budget may publish earlier accepted reads, but ending the binding may
// not. Cancellation retains the admission until the worker has really left.
type pbsBackupPoll struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *pbs.Client
	done   chan struct{}
}

type pbsBackupPollContextKey struct{}

func (m *Monitor) beginPBSBackupPoll(instance string, client *pbs.Client, last, next time.Time) *pbsBackupPoll {
	m.pbsBackupPublicationMu.RLock()
	defer m.pbsBackupPublicationMu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if client == nil || m.pbsClients[instance] != client || !m.lastPBSBackupPoll[instance].Equal(last) || m.pbsBackupPollers[instance] != nil {
		return nil
	}
	parent := m.runtimeCtx
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	owner := &pbsBackupPoll{ctx: ctx, cancel: cancel, client: client, done: make(chan struct{})}
	if m.pbsBackupPollers == nil {
		m.pbsBackupPollers = make(map[string]*pbsBackupPoll)
	}
	if m.lastPBSBackupPoll == nil {
		m.lastPBSBackupPoll = make(map[string]time.Time)
	}
	m.pbsBackupPollers[instance] = owner
	m.lastPBSBackupPoll[instance] = next
	return owner
}

func (m *Monitor) finishPBSBackupPoll(instance string, owner *pbsBackupPoll) {
	m.mu.Lock()
	if m.pbsBackupPollers[instance] == owner {
		delete(m.pbsBackupPollers, instance)
		if owner.ctx.Err() == nil && m.pbsClients[instance] == owner.client {
			m.lastPBSBackupPoll[instance] = time.Now()
		} else {
			delete(m.lastPBSBackupPoll, instance)
		}
	}
	m.mu.Unlock()
	owner.cancel()
	close(owner.done)
}

// Called with the publication writer and m.mu held. An old worker cannot
// publish into, or release, a later binding's claim even if its read returns late.
func (m *Monitor) cancelPBSBackupPollLocked(instance string) {
	if owner := m.pbsBackupPollers[instance]; owner != nil {
		owner.cancel()
		delete(m.pbsBackupPollers, instance)
	}
	delete(m.lastPBSBackupPoll, instance)
}

func (m *Monitor) replacePBSClient(instance string, client *pbs.Client) {
	m.pbsBackupPublicationMu.Lock()
	defer m.pbsBackupPublicationMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelPBSBackupPollLocked(instance)
	// A same-name server must independently read its own snapshots, never
	// borrow the former client's still-fresh count/time cache.
	delete(m.pbsBackupCacheTime, instance)
	if m.pbsClients == nil {
		m.pbsClients = make(map[string]*pbs.Client)
	}
	if client == nil {
		delete(m.pbsClients, instance)
	} else {
		m.pbsClients[instance] = client
	}
}

// Hold only across local publication, never a provider read. WithoutCancel
// retains the owner value, so an expired child cannot erase the runtime fence.
func (m *Monitor) beginPBSBackupPublication(ctx context.Context, instance string) (func(), bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	owner, _ := ctx.Value(pbsBackupPollContextKey{}).(*pbsBackupPoll)
	if owner == nil {
		return func() {}, true // focused collectors retain caller cancellation
	}
	m.pbsBackupPublicationMu.RLock()
	m.mu.RLock()
	current := ctx.Err() == nil && owner.ctx.Err() == nil && m.pbsBackupPollers[instance] == owner && m.pbsClients[instance] == owner.client
	m.mu.RUnlock()
	if !current {
		m.pbsBackupPublicationMu.RUnlock()
		return nil, false
	}
	return m.pbsBackupPublicationMu.RUnlock, true
}
