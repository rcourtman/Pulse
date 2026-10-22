package monitoring

import (
	"context"
	"errors"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A guest cycle collects outside publication locks. Its effects are staged
// until both guest kinds are ready and the registered connection still owns
// this cycle. Child enrichment deadlines do not invalidate an authoritative
// inventory. An expired caller budget also keeps completed inventory usable;
// explicit cancellation of the parent cycle does not.
type pveGuestPoll struct {
	ctx         context.Context
	cancel      context.CancelFunc
	stopRuntime func() bool
	client      PVEClientInterface
	mu          sync.Mutex
	effects     []func()
}

type pveGuestPollContextKey struct{}

func (m *Monitor) beginPVEGuestPoll(ctx context.Context, instance string, client PVEClientInterface) (context.Context, *pveGuestPoll) {
	m.pveGuestPublicationMu.RLock()
	defer m.pveGuestPublicationMu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil || client == nil || m.pveClients[instance] != client || (m.runtimeCtx != nil && m.runtimeCtx.Err() != nil) {
		return ctx, nil
	}
	if previous := m.pveGuestPolls[instance]; previous != nil {
		if previous.client == client {
			// Cancellation is not completion. Do not stack another inventory or
			// release consumed-command fences while its owner is still running.
			return ctx, nil
		}
		previous.cancel()
	}
	ownedCtx, cancel := context.WithCancel(ctx)
	owner := &pveGuestPoll{ctx: ownedCtx, cancel: cancel, client: client}
	if m.runtimeCtx != nil {
		owner.stopRuntime = context.AfterFunc(m.runtimeCtx, cancel)
	}
	if m.pveGuestPolls == nil {
		m.pveGuestPolls = make(map[string]*pveGuestPoll)
	}
	m.pveGuestPolls[instance] = owner
	return context.WithValue(ownedCtx, pveGuestPollContextKey{}, owner), owner
}

func (m *Monitor) finishPVEGuestPoll(instance string, owner *pveGuestPoll) {
	owner.cancel()
	if owner.stopRuntime != nil {
		owner.stopRuntime()
	}
	m.mu.Lock()
	if m.pveGuestPolls[instance] == owner {
		delete(m.pveGuestPolls, instance)
	}
	m.mu.Unlock()
}

// Focused builder/collector callers retain their existing immediate effects.
// Only the production connection cycle installs a batch, inherited by all its
// concurrent builders. A completed partial builder is not a published cycle.
func stagePVEGuestEffect(ctx context.Context, effect func()) {
	if owner, _ := ctx.Value(pveGuestPollContextKey{}).(*pveGuestPoll); owner != nil {
		owner.mu.Lock()
		owner.effects = append(owner.effects, effect)
		owner.mu.Unlock()
		return
	}
	effect()
}

func (m *Monitor) publishPVEGuests(ctx context.Context, instance string, vms []models.VM, containers []models.Container) {
	owner, _ := ctx.Value(pveGuestPollContextKey{}).(*pveGuestPoll)
	if owner == nil {
		m.migratePVEGuestMetadata(instance, vms, containers)
		m.state.UpdateGuestsForInstance(instance, vms, containers)
		return
	}
	// Retirement/client/runtime changes take the writer side and therefore
	// cannot clear or replace the connection midway through an admitted batch.
	// m.mu is released before effects (including alert callbacks) run.
	m.pveGuestPublicationMu.RLock()
	defer m.pveGuestPublicationMu.RUnlock()
	m.mu.RLock()
	cycleErr := owner.ctx.Err()
	// A caller deadline bounds remote enrichment, not the lifetime of the
	// configured connection. Keep the existing authoritative-inventory tail
	// even if local assembly outlasts that budget. Explicit cancellation, an
	// ended runtime or a changed binding still withdraws the entire cycle.
	current := (cycleErr == nil || errors.Is(cycleErr, context.DeadlineExceeded)) && m.pveGuestPolls[instance] == owner && m.pveClients[instance] == owner.client && (m.runtimeCtx == nil || m.runtimeCtx.Err() == nil)
	m.mu.RUnlock()
	if !current {
		return
	}
	// Readers can observe the new node as soon as state is published. Move
	// operator metadata first, under the same accepted connection ownership.
	m.migratePVEGuestMetadata(instance, vms, containers)
	m.state.UpdateGuestsForInstance(instance, vms, containers)
	owner.mu.Lock()
	effects := owner.effects
	owner.effects = nil
	owner.mu.Unlock()
	for _, effect := range effects {
		effect()
	}
}

func (m *Monitor) migratePVEGuestMetadata(instance string, vms []models.VM, containers []models.Container) {
	if m.guestMetadataStore == nil {
		return
	}
	for _, vm := range vms {
		if vm.ID != "" && vm.Node != "" && vm.VMID > 0 {
			m.guestMetadataStore.GetWithLegacyMigration(vm.ID, instance, vm.Node, vm.VMID)
		}
	}
	for _, container := range containers {
		if container.ID != "" && container.Node != "" && container.VMID > 0 {
			m.guestMetadataStore.GetWithLegacyMigration(container.ID, instance, container.Node, container.VMID)
		}
	}
}

// Called with m.mu and the publication writer held. Replacing a transport is
// not a guest reboot: accepted cache ages and OS-command safety stay intact.
func (m *Monitor) cancelPVEGuestPollLocked(instance string) {
	if owner := m.pveGuestPolls[instance]; owner != nil {
		owner.cancel()
		delete(m.pveGuestPolls, instance)
	}
}

func (m *Monitor) replacePVEClient(instance string, client PVEClientInterface) {
	m.pveGuestPublicationMu.Lock()
	defer m.pveGuestPublicationMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelPVEGuestPollLocked(instance)
	m.cancelPVEBackupPollLocked(instance)
	if m.pveClients == nil {
		m.pveClients = make(map[string]PVEClientInterface)
	}
	m.pveClients[instance] = client
}
