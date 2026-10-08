package monitoring

import (
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const backupIntentEvidenceMaxAge = 5 * time.Minute

type resourceOperatorIntentReader interface {
	GetResourceOperatorState(canonicalID string) (unifiedresources.ResourceOperatorState, bool, error)
}

type resourceIntentIdentityReader interface {
	ResolveCanonicalResourceID(ref string) (string, bool)
}

type resourceIntentAncestorReader interface {
	ResolveCanonicalResourceAncestors(ref string) []string
}

func (m *Monitor) installOperatorIntentResolver(store ResourceStoreInterface) {
	if m == nil || m.alertManager == nil {
		return
	}
	identity := m.newOperatorIntentIdentity(store)
	m.operatorIntentIdentity.Store(identity)
	identity.refresh()
	identityStore := func() any {
		if identity == nil {
			return store
		}
		return identity.current()
	}
	identityReader, hasIdentityReader := store.(resourceIntentIdentityReader)
	if hasIdentityReader && identityReader != nil {
		m.alertManager.SetResourceIntentIdentityResolver(func(ref string) (string, bool) {
			return identityStore().(resourceIntentIdentityReader).ResolveCanonicalResourceID(ref)
		})
	} else {
		m.alertManager.SetResourceIntentIdentityResolver(nil)
	}
	reader, ok := store.(resourceOperatorIntentReader)
	if !ok || reader == nil {
		m.alertManager.SetOperatorIntentContextResolver(nil)
		return
	}
	_, hasAncestorReader := store.(resourceIntentAncestorReader)
	m.alertManager.SetOperatorIntentContextResolver(func(resourceID string, now time.Time) (alerts.OperatorIntentContext, bool) {
		identity := identityStore()
		if hasIdentityReader {
			if canonicalID, found := identity.(resourceIntentIdentityReader).ResolveCanonicalResourceID(resourceID); found {
				resourceID = canonicalID
			}
		}
		state, found, err := reader.GetResourceOperatorState(resourceID)
		if err != nil {
			log.Warn().Err(err).Str("resourceID", resourceID).Msg("Failed to read operator state for alert intent")
			return alerts.OperatorIntentContext{}, false
		}
		context := alerts.OperatorIntentContext{}
		if found {
			context = alerts.OperatorIntentContext{
				IntentionallyOffline: state.IntentionallyOffline,
				MonitoringMode:       string(state.MonitoringMode),
				LifecycleState:       string(state.LifecycleState),
			}
		}

		applyActiveMaintenance := func(candidate unifiedresources.ResourceOperatorState, sourceID string, inherited bool) {
			occurrence, active := candidate.ActiveMaintenanceOccurrenceAt(now)
			if !active {
				return
			}
			// Overlapping exact/inherited schedules remain suppressed until the
			// latest active end. This avoids promising an early delivery resume.
			if context.MaintenanceEndAt != nil && !occurrence.EndAt.After(*context.MaintenanceEndAt) {
				return
			}
			startAt, endAt := occurrence.StartAt, occurrence.EndAt
			context.MaintenanceStartAt = &startAt
			context.MaintenanceEndAt = &endAt
			context.MaintenanceReason = candidate.MaintenanceReason
			context.MaintenanceSourceID = sourceID
			context.MaintenanceInherited = inherited
			context.MaintenanceScope = string(candidate.MaintenanceScope)
		}
		if found {
			applyActiveMaintenance(state, resourceID, false)
		}
		if hasAncestorReader {
			for _, ancestorID := range identity.(resourceIntentAncestorReader).ResolveCanonicalResourceAncestors(resourceID) {
				ancestorState, ancestorFound, ancestorErr := reader.GetResourceOperatorState(ancestorID)
				if ancestorErr != nil {
					log.Warn().Err(ancestorErr).Str("resourceID", resourceID).Str("ancestorID", ancestorID).Msg("Failed to read inherited operator maintenance state")
					continue
				}
				if !ancestorFound || ancestorState.MaintenanceScope != unifiedresources.MaintenanceScopeResourceAndDescendants {
					continue
				}
				applyActiveMaintenance(ancestorState, ancestorID, true)
			}
		}
		return context, found || context.MaintenanceEndAt != nil
	})
	// Alerts restore before the resource store is attached during startup.
	// Reconcile that restored set as soon as persisted policy is available.
	if m.alertManager.ReconcileOperatorIntentState() > 0 && m.state != nil {
		m.SyncAlertState()
	}
}

// operatorIntentIdentity is the store alert intent resolves resource
// references and ancestors through: the published registry with saved hosts
// that have not reported since a restart overlaid, the read state Patrol
// resolves through and the resources API seeds from. A saved host exists only
// there, and an operator link holds it under the link's primary, so the
// published registry alone left the saved host's alerts reading their literal
// reference and missed intent set on that host or on the primary.
//
// The alert manager resolves under its own lock, once per metric check, so
// the overlay is kept for the published generation it was built from.
// Publication builds it before evaluating alerts (refresh, outside that
// lock); a lookup that races ahead of publication builds it itself. A
// saved-host change between generations shows on the next one. Mock mode
// carries no saved hosts: the published registry answers and any real-mode
// overlay is dropped.
type operatorIntentIdentity struct {
	monitor     *Monitor
	store       ResourceStoreInterface
	published   unifiedresources.ReadState
	generations interface {
		Generation() unifiedresources.RegistryGeneration
	}

	mu        sync.Mutex
	built     bool
	builtFor  unifiedresources.RegistryGeneration
	readState any
}

// newOperatorIntentIdentity returns nil for a store without generations or
// the reference and ancestor readers, which alert intent then uses as is.
func (m *Monitor) newOperatorIntentIdentity(store ResourceStoreInterface) *operatorIntentIdentity {
	published, isReadState := store.(unifiedresources.ReadState)
	generations, versioned := store.(interface {
		Generation() unifiedresources.RegistryGeneration
	})
	_, resolves := store.(resourceIntentIdentityReader)
	_, walksAncestors := store.(resourceIntentAncestorReader)
	if !isReadState || !versioned || !resolves || !walksAncestors {
		return nil
	}
	return &operatorIntentIdentity{monitor: m, store: store, published: published, generations: generations}
}

// current returns the read state for the current published generation.
func (i *operatorIntentIdentity) current() any {
	if mock.IsMockEnabled() {
		i.mu.Lock()
		i.built, i.readState = false, nil
		i.mu.Unlock()
		return i.store
	}
	generation := i.generations.Generation()
	i.mu.Lock()
	if i.built && i.builtFor == generation {
		readState := i.readState
		i.mu.Unlock()
		return readState
	}
	i.mu.Unlock()
	return i.build(generation)
}

// refresh builds the read state for the current published generation unless
// it is already built.
func (i *operatorIntentIdentity) refresh() {
	if i == nil || mock.IsMockEnabled() {
		return
	}
	generation := i.generations.Generation()
	i.mu.Lock()
	fresh := i.built && i.builtFor == generation
	i.mu.Unlock()
	if !fresh {
		i.build(generation)
	}
}

// build overlays saved hosts on the published registry outside i.mu, so a
// concurrent lookup waits on neither the overlay nor the store reads it makes.
// It keeps the result only while the mock-mode epoch it started in and the
// generation it read are still current. The check and the store run inside
// the fence, so a mode switch, which advances the fence after flipping the
// mode, either waits for them or makes them refuse: a build that read no
// saved hosts because mock mode was on never stands in for real mode.
func (i *operatorIntentIdentity) build(generation unifiedresources.RegistryGeneration) any {
	scope := i.monitor.mockModeFence.begin()
	if mock.IsMockEnabled() {
		return i.store
	}
	readState := any(i.store)
	overlay := i.monitor.readStateWithStandaloneHostContinuity(i.published)
	if _, ok := overlay.(resourceIntentIdentityReader); ok {
		if _, ok := overlay.(resourceIntentAncestorReader); ok {
			readState = overlay
		}
	}
	scope.run(func() {
		if mock.IsMockEnabled() || i.generations.Generation() != generation {
			return
		}
		i.mu.Lock()
		i.built, i.builtFor, i.readState = true, generation, readState
		i.mu.Unlock()
	})
	return readState
}

func (m *Monitor) resolveBackupIntentContext(_ string, instance, node string, vmid int, now time.Time) (alerts.BackupIntentContext, bool) {
	if m == nil || m.state == nil || vmid <= 0 {
		return alerts.BackupIntentContext{}, false
	}
	instance = strings.TrimSpace(instance)
	node = strings.TrimSpace(node)
	if now.IsZero() {
		now = time.Now().UTC()
	}

	for _, task := range m.state.GetSnapshot().PVEBackups.BackupTasks {
		if task.VMID != vmid || (instance != "" && task.Instance != instance) || (node != "" && task.Node != "" && task.Node != node) {
			continue
		}
		if task.ObservedAt.IsZero() || now.Sub(task.ObservedAt) > backupIntentEvidenceMaxAge || task.ObservedAt.After(now.Add(time.Minute)) {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if !task.EndTime.IsZero() || status == "ok" || status == "stopped" || status == "error" || status == "warning" {
			continue
		}
		return alerts.BackupIntentContext{
			Active:     true,
			ObservedAt: task.ObservedAt,
			Evidence:   "pve_vzdump_task:" + task.ID,
		}, true
	}
	return alerts.BackupIntentContext{}, false
}
