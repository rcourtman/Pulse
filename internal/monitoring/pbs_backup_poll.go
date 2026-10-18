package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rs/zerolog/log"
)

// Ownership, cadence and cancellation are independent. A cancelled reader
// retains its slot until actual return, but cannot publish into a new runtime.
type pbsBackupPoll struct {
	ctx     context.Context
	cancel  context.CancelFunc
	client  *pbs.Client
	started time.Time
}

// Namespace discovery is collection evidence, not part of the public datastore
// model. Carry its result with the exact datastore input into the admitted scan.
type pbsBackupDatastore struct {
	models.PBSDatastore
	namespaceDiscoveryErr error
}

// A complete empty datastore list can confirm absence. A failed list cannot
// select cached paths to probe, certify protection, or delete recovery facts.
type pbsBackupInventory struct {
	datastores            []pbsBackupDatastore
	datastoreDiscoveryErr error
}

type pbsBackupPollContextKey struct{}

func (m *Monitor) beginPBSBackupPoll(instance string, now time.Time, client *pbs.Client) (*pbsBackupPoll, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client == nil || m.pbsClients[instance] != client {
		return nil, "connection changed"
	}
	parent := m.runtimeCtx
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil, "runtime ended"
	}
	if m.pbsBackupPollers[instance] != nil {
		return nil, "previous backup scan is still running"
	}
	run, reason, next := m.shouldRunBackupPoll(m.lastPBSBackupPoll[instance], now)
	if !run {
		return nil, reason
	}
	if m.lastPBSBackupPoll == nil {
		m.lastPBSBackupPoll = make(map[string]time.Time)
	}
	if m.pbsBackupPollers == nil {
		m.pbsBackupPollers = make(map[string]*pbsBackupPoll)
	}
	ctx, cancel := context.WithCancel(parent)
	owner := &pbsBackupPoll{ctx: ctx, cancel: cancel, client: client, started: next}
	m.lastPBSBackupPoll[instance] = next
	m.pbsBackupPollers[instance] = owner
	return owner, ""
}

func (m *Monitor) finishPBSBackupPoll(instance string, owner *pbsBackupPoll) {
	defer owner.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pbsBackupPollers[instance] != owner {
		return
	}
	// Do not erase a requested cadence reset or delay resumption of a withdrawn
	// runtime by treating its eventual return as a fresh successful scan.
	if m.lastPBSBackupPoll[instance].Equal(owner.started) {
		if owner.ctx.Err() == nil {
			m.lastPBSBackupPoll[instance] = time.Now()
		} else {
			delete(m.lastPBSBackupPoll, instance)
		}
	}
	delete(m.pbsBackupPollers, instance)
}

// Called with mu and the publication writer held. Cancellation is not return.
func (m *Monitor) cancelPBSBackupPollLocked(instance string) {
	if owner := m.pbsBackupPollers[instance]; owner != nil {
		owner.cancel()
		delete(m.lastPBSBackupPoll, instance)
	}
}

func (m *Monitor) replacePBSClient(instance string, client *pbs.Client) {
	m.pbsBackupPublicationMu.Lock()
	defer m.pbsBackupPublicationMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pbsClients == nil {
		m.pbsClients = make(map[string]*pbs.Client)
	}
	if m.pbsClients[instance] != client {
		m.cancelPBSBackupPollLocked(instance)
		delete(m.lastPBSBackupPoll, instance)
		// Historical rows can remain last known, but an identical count/time on
		// a new connection cannot reuse the previous connection's verification.
		delete(m.pbsBackupCacheTime, instance)
	}
	m.pbsClients[instance] = client
}

func (m *Monitor) applyPBSBackupEffect(ctx context.Context, instance string, effect func()) {
	if ctx.Err() != nil {
		return
	}
	owner, _ := ctx.Value(pbsBackupPollContextKey{}).(*pbsBackupPoll)
	if owner == nil {
		// Focused collectors keep their existing caller-owned behaviour.
		effect()
		return
	}
	m.pbsBackupPublicationMu.RLock()
	defer m.pbsBackupPublicationMu.RUnlock()
	m.mu.RLock()
	current := ctx.Err() == nil && owner.ctx.Err() == nil && m.pbsBackupPollers[instance] == owner && m.pbsClients[instance] == owner.client
	m.mu.RUnlock()
	if current {
		// State, cache, recovery and alert admission share one boundary. Network
		// reads occur outside this lock; replacement/retirement takes its writer.
		effect()
	}
}

func (m *Monitor) pollPBSBackupsAsync(instance string, client *pbs.Client, inventory pbsBackupInventory) {
	owner, reason := m.beginPBSBackupPoll(instance, time.Now(), client)
	if owner == nil {
		log.Debug().Str("instance", instance).Str("reason", reason).Msg("Skipping PBS backup polling this cycle")
		return
	}
	// Namespace slices also belong to this admitted scan, not a later poll.
	inventory.datastores = append([]pbsBackupDatastore(nil), inventory.datastores...)
	for i := range inventory.datastores {
		inventory.datastores[i].Namespaces = append([]models.PBSNamespace(nil), inventory.datastores[i].Namespaces...)
	}
	go func() {
		defer recoverFromPanic(fmt.Sprintf("pollPBSBackups-%s", instance))
		defer m.finishPBSBackupPoll(instance, owner)
		parent := context.WithValue(owner.ctx, pbsBackupPollContextKey{}, owner)
		ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
		defer cancel()
		log.Info().Str("instance", instance).Int("datastores", len(inventory.datastores)).Msg("Starting background PBS backup polling")
		m.pollPBSBackupInventory(ctx, instance, client, inventory)
		log.Info().Str("instance", instance).Dur("duration", time.Since(owner.started)).Msg("Completed background PBS backup polling")
	}()
}

func (m *Monitor) publishPBSBackupDiscoveryFailure(ctx context.Context, instance string, discoveryErr error) {
	retain := shouldReuseCachedPBSBackups(discoveryErr)
	rejected := 0
	if !retain {
		rejected = 1
	}
	// The failed root listing is one unavailable observation, not zero
	// configured datastores successfully read. No raw point is re-observed.
	observation, err := buildPBSProtectionProviderObservation(instance, 1, 0, 1, rejected, time.Now().UTC())
	if err != nil {
		log.Warn().Err(err).Str("instance", instance).Msg("Failed to build PBS discovery observation")
		return
	}
	m.applyPBSBackupEffect(ctx, instance, func() {
		if !retain {
			m.state.UpdatePBSBackups(instance, nil)
			m.prunePBSBackupCacheTimes(instance, nil)
			m.syncGuestBackupTimesAndResourceStore()
		}
		// Historical recovery facts remain qualified by unavailable/denied
		// evidence. Only an ordinary complete inventory can reconcile them.
		m.ingestProtectionProviderObservationsAsync([]recovery.ProtectionProviderObservation{observation})
		m.checkBackupAlerts(ctx)
		m.broadcastStateUpdate()
	})
}
