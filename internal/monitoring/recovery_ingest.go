package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const (
	// Periodic posture refresh cadence when a complete, healthy full-inventory
	// scope has zero changed points, so materialized postures still age cleanly
	// across freshness/verification windows without rewriting thousands of
	// unchanged recovery_points rows every minute.
	recoverySteadyPostureRefreshInterval = 15 * time.Minute
	// Periodic provider observation heartbeat when unchanged, keeping the latest
	// observation well inside recovery.db's 90-day retention window.
	recoverySteadyObservationRefreshInterval = 6 * time.Hour
)

type recoveryIngestSteadyKey struct {
	orgID    string
	provider string
	idPrefix string
	instance string
}

type recoveryIngestSteadyState struct {
	store              *recoverystore.Store
	pointDigests       map[string][16]byte
	observationDigest  [32]byte
	unsettledDigest    [32]byte
	subjectKeys        []string
	lastObservationAt  time.Time
	lastPostureRefresh time.Time
}

// recoveryReconcileScope identifies one class of points for one polled
// connection whose latest enumeration is authoritative: after the batch is
// upserted, points in scope that were not part of the batch are deleted.
type recoveryReconcileScope struct {
	provider string
	idPrefix string
	instance string

	// A snapshot poll only has removal authority for its exact read targets.
	// Nil leaves the existing instance-wide scope for full backup inventories.
	snapshotGuests map[recoverystore.PVESnapshotGuest]struct{}
}

// recoveryIngestBatch is one poll cycle's worth of recovery points, plus an
// optional reconcile scope when the points are a complete enumeration.
type recoveryIngestBatch struct {
	points       []recovery.RecoveryPoint
	observations []recovery.ProtectionProviderObservation
	reconcile    *recoveryReconcileScope
	// evaluatedSubjects names the guests a complete enumeration could answer for,
	// and unsettledSubjects those among them its unattributed backups could belong
	// to. A partial read sets neither: it says nothing about the snapshots it did
	// not reach. A guest that has not loaded is not evaluated and keeps its answer.
	evaluatedSubjects []string
	unsettledSubjects []string
}

type recoveryIngestCoalesceKey struct {
	provider string
	idPrefix string
	instance string
}

func (batch recoveryIngestBatch) scopeKey() (recoveryIngestCoalesceKey, bool) {
	if batch.reconcile == nil {
		return recoveryIngestCoalesceKey{}, false
	}
	return recoveryIngestCoalesceKey{
		provider: batch.reconcile.provider,
		idPrefix: batch.reconcile.idPrefix,
		instance: batch.reconcile.instance,
	}, true
}

func (batch recoveryIngestBatch) coalesceKey() (recoveryIngestCoalesceKey, bool) {
	// Guest-scoped batches can overlap without superseding one another. Keep
	// their observations and removals in source order, just like partial reads.
	if batch.reconcile == nil || batch.reconcile.snapshotGuests != nil {
		return recoveryIngestCoalesceKey{}, false
	}
	return batch.scopeKey()
}

func (m *Monitor) coalescePendingRecoveryIngest(batch recoveryIngestBatch) bool {
	key, ok := batch.coalesceKey()
	if !ok {
		return false
	}
	for i := len(m.recoveryIngestPending) - 1; i >= 0; i-- {
		pending := m.recoveryIngestPending[i]
		pendingKey, pendingOK := pending.coalesceKey()
		if pendingOK {
			if pendingKey == key {
				m.recoveryIngestPending[i] = batch
				return true
			}
			continue
		}
		// A scoped non-coalescable batch (e.g. a PVE guest-snapshot read)
		// touches only its own (provider, idPrefix, instance) scope; it never
		// blocks coalescing an unrelated instance-wide enumeration queued
		// ahead of it.
		if scopeKey, hasScope := pending.scopeKey(); hasScope && scopeKey != key {
			continue
		}
		// An incomplete poll/event or same-scope guest-scoped batch is an
		// ordered fact, not a replaceable full snapshot. Moving a later
		// complete (possibly empty) inventory before it could reinsert old
		// points after authoritative deletion.
		return false
	}
	return false
}

func (m *Monitor) ingestRecoveryPointsAsync(points []recovery.RecoveryPoint) {
	m.enqueueRecoveryIngest(recoveryIngestBatch{points: points})
}

// ingestAndReconcileRecoveryPointsAsync upserts a complete enumeration and
// removes points in scope that the source no longer reports. An empty points
// slice is meaningful here: it clears the scope entirely (#1580).
func (m *Monitor) ingestAndReconcileRecoveryPointsAsync(points []recovery.RecoveryPoint, scope recoveryReconcileScope) {
	m.enqueueRecoveryIngest(recoveryIngestBatch{points: points, reconcile: &scope})
}

func (m *Monitor) ingestAndReconcileRecoveryPointsWithObservationsAsync(
	points []recovery.RecoveryPoint,
	observations []recovery.ProtectionProviderObservation,
	scope recoveryReconcileScope,
) {
	m.enqueueRecoveryIngest(recoveryIngestBatch{
		points:       points,
		observations: observations,
		reconcile:    &scope,
	})
}

func (m *Monitor) ingestProtectionProviderObservationsAsync(
	observations []recovery.ProtectionProviderObservation,
) {
	m.enqueueRecoveryIngest(recoveryIngestBatch{observations: observations})
}

func (m *Monitor) enqueueRecoveryIngest(batch recoveryIngestBatch) {
	if m == nil ||
		(len(batch.points) == 0 &&
			len(batch.observations) == 0 &&
			batch.reconcile == nil) {
		return
	}

	m.recoveryIngestMu.Lock()
	if m.recoveryIngestRunning {
		// An instance-wide reconcile batch is a complete enumeration, so only
		// its newest pending value is authoritative. Distinct
		// scopes and non-reconciling event batches remain FIFO; this preserves
		// their facts without retaining every superseded full poll result.
		if m.coalescePendingRecoveryIngest(batch) {
			pending := len(m.recoveryIngestPending)
			m.recoveryIngestMu.Unlock()
			log.Debug().
				Int("points", len(batch.points)).
				Int("provider_observations", len(batch.observations)).
				Int("pending_batches", pending).
				Msg("Coalesced recovery point ingest behind active batch")
			return
		}
		m.recoveryIngestPending = append(m.recoveryIngestPending, batch)
		pending := len(m.recoveryIngestPending)
		m.recoveryIngestMu.Unlock()
		log.Debug().
			Int("points", len(batch.points)).
			Int("provider_observations", len(batch.observations)).
			Int("pending_batches", pending).
			Msg("Queued recovery point ingest behind active batch")
		return
	}
	m.recoveryIngestRunning = true
	m.recoveryIngestMu.Unlock()

	go m.runRecoveryPointIngestLoop([]recoveryIngestBatch{batch})
}

func (m *Monitor) runRecoveryPointIngestLoop(batches []recoveryIngestBatch) {
	for len(batches) > 0 {
		for i := range batches {
			batch := batches[i]
			// Release processed batch references immediately rather than pinning
			// every full backup inventory until the entire drained slice finishes.
			batches[i] = recoveryIngestBatch{}
			// SQLite upserts are usually quick, but allow a bit of time for large batches.
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			m.ingestRecoveryPointsBestEffort(ctx, batch)
			cancel()
		}

		m.recoveryIngestMu.Lock()
		if len(m.recoveryIngestPending) == 0 {
			m.recoveryIngestRunning = false
			m.recoveryIngestMu.Unlock()
			return
		}
		batches = m.recoveryIngestPending
		m.recoveryIngestPending = nil
		m.recoveryIngestMu.Unlock()
	}

	m.recoveryIngestMu.Lock()
	m.recoveryIngestRunning = false
	m.recoveryIngestMu.Unlock()
}

func (m *Monitor) ingestRecoveryPointsBestEffort(ctx context.Context, batch recoveryIngestBatch) {
	if m == nil ||
		(len(batch.points) == 0 &&
			len(batch.observations) == 0 &&
			batch.reconcile == nil) {
		return
	}

	m.mu.RLock()
	rm := m.recoveryManager
	orgID := strings.TrimSpace(m.orgID)
	m.mu.RUnlock()

	if rm == nil {
		return
	}
	if orgID == "" {
		orgID = "default"
	}

	store, err := rm.StoreForOrg(orgID)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to open recovery store for backup ingestion")
		return
	}
	m.retireUnsettledBackupScopes(ctx, store, orgID)

	if scopeKey, eligible := recoveryIngestBatchEligibleForSteadyState(batch); eligible {
		m.ingestCompleteRecoveryInventory(ctx, store, orgID, scopeKey, batch)
		return
	}
	// Any non-eligible batch (partial/degraded observation, guest-scoped
	// snapshot batch, or event-only upsert) invalidates matching steady-state
	// caches so the next complete enumeration writes and reconciles in full.
	m.invalidateRecoverySteadyStateForBatch(orgID, batch)

	// Collection-wide evidence is the authority for how much provider history
	// Pulse could actually see. Persist it before any point mutation so a large
	// point batch, refresh timeout, or reconciliation failure cannot leave
	// retained artifacts looking more trustworthy than the poll that produced
	// them.
	if err := store.UpsertProtectionProviderObservations(
		ctx,
		batch.observations,
	); err != nil {
		log.Warn().
			Err(err).
			Str("org_id", orgID).
			Int("provider_observations", len(batch.observations)).
			Msg("Failed to upsert provider protection observations")
		return
	}
	if err := store.UpsertPoints(ctx, batch.points); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Int("points", len(batch.points)).Msg("Failed to upsert recovery points from backup polling")
		// Do not reconcile against a batch that failed to land; deleting on
		// top of a failed upsert could drop points the source still reports.
		return
	}

	if batch.reconcile == nil {
		return
	}
	keepIDs := make([]string, 0, len(batch.points))
	for _, p := range batch.points {
		keepIDs = append(keepIDs, p.ID)
	}
	scope := batch.reconcile
	if scope.snapshotGuests != nil {
		_, err = store.ReconcilePVESnapshotPoints(ctx, scope.instance, scope.snapshotGuests, keepIDs)
	} else {
		_, err = store.ReconcileInstancePoints(ctx, scope.provider, scope.idPrefix, scope.instance, keepIDs)
	}
	if err != nil {
		log.Warn().
			Err(err).
			Str("org_id", orgID).
			Str("instance", batch.reconcile.instance).
			Str("idPrefix", batch.reconcile.idPrefix).
			Msg("Failed to reconcile recovery points against source enumeration")
	}
	if len(batch.evaluatedSubjects) == 0 {
		return
	}
	if err := store.UpdateUnsettledProtectionSubjects(ctx, batch.reconcile.instance, batch.evaluatedSubjects, batch.unsettledSubjects); err != nil {
		log.Warn().
			Err(err).
			Str("org_id", orgID).
			Str("instance", batch.reconcile.instance).
			Msg("Failed to update guests with an unsettled backup")
	}
}

func recoveryIngestBatchEligibleForSteadyState(batch recoveryIngestBatch) (recoveryIngestCoalesceKey, bool) {
	key, ok := batch.coalesceKey()
	if !ok {
		return recoveryIngestCoalesceKey{}, false
	}
	for _, obs := range batch.observations {
		if obs.HistoryCompleteness != recovery.ProtectionHistoryComplete ||
			obs.Permissions != operationaltrust.EvidencePermissionsSufficient {
			return recoveryIngestCoalesceKey{}, false
		}
	}
	return key, true
}

// ingestCompleteRecoveryInventory persists an authoritative full-inventory
// batch while skipping redundant SQLite rewrites of unchanged points on
// consecutive complete, healthy polls. Under complete history with sufficient
// permissions, supportedProtectionTimes does not rely on per-poll point
// Evidence.IngestedAt renewal; partial/degraded polls bypass this path and
// invalidate the cached steady state so pointConfirmedByObservation stays
// exact. Running points are never skipped so BackupTaskRunningMaxAge stays
// accurate.
func (m *Monitor) ingestCompleteRecoveryInventory(
	ctx context.Context,
	store *recoverystore.Store,
	orgID string,
	scopeKey recoveryIngestCoalesceKey,
	batch recoveryIngestBatch,
) {
	steadyKey := recoveryIngestSteadyKey{
		orgID:    orgID,
		provider: scopeKey.provider,
		idPrefix: scopeKey.idPrefix,
		instance: scopeKey.instance,
	}
	prev := m.loadRecoveryIngestSteadyState(steadyKey)
	if prev != nil && prev.store != store {
		prev = nil
	}

	now := time.Now().UTC()
	newDigests := make(map[string][16]byte, len(batch.points))
	subjectKeySet := make(map[string]struct{}, 64)
	var changedPoints []recovery.RecoveryPoint

	for _, p := range batch.points {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		digest := recoveryPointSemanticDigest(p)
		newDigests[id] = digest
		if sk := strings.TrimSpace(recovery.SubjectKeyForPoint(p)); sk != "" {
			subjectKeySet[sk] = struct{}{}
		}
		if prev == nil || p.Outcome == recovery.OutcomeRunning {
			changedPoints = append(changedPoints, p)
			continue
		}
		if oldDigest, exists := prev.pointDigests[id]; !exists || oldDigest != digest {
			changedPoints = append(changedPoints, p)
		}
	}

	hasRemovedPoints := prev == nil
	if !hasRemovedPoints {
		if len(prev.pointDigests) != len(newDigests) {
			hasRemovedPoints = true
		} else {
			for id := range prev.pointDigests {
				if _, stillPresent := newDigests[id]; !stillPresent {
					hasRemovedPoints = true
					break
				}
			}
		}
	}

	obsDigest := recoveryObservationsSemanticDigest(batch.observations)
	needObsWrite := len(batch.observations) > 0 && (prev == nil ||
		prev.observationDigest != obsDigest ||
		now.Sub(prev.lastObservationAt) >= recoverySteadyObservationRefreshInterval)

	unsettledDigest := recoveryUnsettledDigest(batch.evaluatedSubjects, batch.unsettledSubjects)
	needUnsettledWrite := len(batch.evaluatedSubjects) > 0 && (prev == nil ||
		prev.unsettledDigest != unsettledDigest)

	didMutateOrRefresh := false
	if needObsWrite {
		if err := store.UpsertProtectionProviderObservations(ctx, batch.observations); err != nil {
			m.deleteRecoveryIngestSteadyState(steadyKey)
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Int("provider_observations", len(batch.observations)).
				Msg("Failed to upsert provider protection observations")
			return
		}
		didMutateOrRefresh = true
	}

	if len(changedPoints) > 0 {
		if err := store.UpsertPoints(ctx, changedPoints); err != nil {
			m.deleteRecoveryIngestSteadyState(steadyKey)
			log.Warn().Err(err).Str("org_id", orgID).Int("points", len(changedPoints)).Msg("Failed to upsert recovery points from backup polling")
			return
		}
		didMutateOrRefresh = true
	}

	if hasRemovedPoints {
		keepIDs := make([]string, 0, len(batch.points))
		for _, p := range batch.points {
			keepIDs = append(keepIDs, p.ID)
		}
		if _, err := store.ReconcileInstancePoints(ctx, scopeKey.provider, scopeKey.idPrefix, scopeKey.instance, keepIDs); err != nil {
			m.deleteRecoveryIngestSteadyState(steadyKey)
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Str("instance", scopeKey.instance).
				Str("idPrefix", scopeKey.idPrefix).
				Msg("Failed to reconcile recovery points against source enumeration")
			return
		}
		didMutateOrRefresh = true
	}

	if needUnsettledWrite {
		if err := store.UpdateUnsettledProtectionSubjects(ctx, scopeKey.instance, batch.evaluatedSubjects, batch.unsettledSubjects); err != nil {
			m.deleteRecoveryIngestSteadyState(steadyKey)
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Str("instance", scopeKey.instance).
				Msg("Failed to update guests with an unsettled backup")
			return
		}
		didMutateOrRefresh = true
	}

	subjectKeys := make([]string, 0, len(subjectKeySet))
	for sk := range subjectKeySet {
		subjectKeys = append(subjectKeys, sk)
	}
	sort.Strings(subjectKeys)

	if !didMutateOrRefresh &&
		prev != nil &&
		len(subjectKeys) > 0 &&
		now.Sub(prev.lastPostureRefresh) >= recoverySteadyPostureRefreshInterval {
		if err := store.RefreshProtectionPostures(ctx, subjectKeys); err != nil {
			m.deleteRecoveryIngestSteadyState(steadyKey)
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Str("instance", scopeKey.instance).
				Msg("Failed to refresh steady-state protection postures")
			return
		}
		didMutateOrRefresh = true
	}

	next := &recoveryIngestSteadyState{
		store:             store,
		pointDigests:      newDigests,
		observationDigest: obsDigest,
		subjectKeys:       subjectKeys,
	}
	if prev != nil {
		next.lastObservationAt = prev.lastObservationAt
		next.lastPostureRefresh = prev.lastPostureRefresh
		next.unsettledDigest = prev.unsettledDigest
		if len(batch.observations) == 0 {
			next.observationDigest = prev.observationDigest
		}
	}
	if needObsWrite {
		next.lastObservationAt = now
	}
	if needUnsettledWrite {
		next.unsettledDigest = unsettledDigest
	}
	if didMutateOrRefresh {
		next.lastPostureRefresh = now
	}
	m.storeRecoveryIngestSteadyState(steadyKey, next)
}

func (m *Monitor) loadRecoveryIngestSteadyState(key recoveryIngestSteadyKey) *recoveryIngestSteadyState {
	m.recoveryIngestSteadyMu.Lock()
	defer m.recoveryIngestSteadyMu.Unlock()
	if m.recoveryIngestSteady == nil {
		return nil
	}
	return m.recoveryIngestSteady[key]
}

func (m *Monitor) storeRecoveryIngestSteadyState(key recoveryIngestSteadyKey, state *recoveryIngestSteadyState) {
	m.recoveryIngestSteadyMu.Lock()
	defer m.recoveryIngestSteadyMu.Unlock()
	if m.recoveryIngestSteady == nil {
		m.recoveryIngestSteady = make(map[recoveryIngestSteadyKey]*recoveryIngestSteadyState)
	}
	m.recoveryIngestSteady[key] = state
}

func (m *Monitor) deleteRecoveryIngestSteadyState(key recoveryIngestSteadyKey) {
	m.recoveryIngestSteadyMu.Lock()
	defer m.recoveryIngestSteadyMu.Unlock()
	delete(m.recoveryIngestSteady, key)
}

func (m *Monitor) invalidateRecoverySteadyStateForOrg(orgID string) {
	m.recoveryIngestSteadyMu.Lock()
	defer m.recoveryIngestSteadyMu.Unlock()
	for k := range m.recoveryIngestSteady {
		if k.orgID == orgID {
			delete(m.recoveryIngestSteady, k)
		}
	}
}

func (m *Monitor) invalidateRecoverySteadyStateForBatch(orgID string, batch recoveryIngestBatch) {
	m.recoveryIngestSteadyMu.Lock()
	defer m.recoveryIngestSteadyMu.Unlock()
	if len(m.recoveryIngestSteady) == 0 {
		return
	}
	if batch.reconcile != nil {
		target := recoveryIngestSteadyKey{
			orgID:    orgID,
			provider: batch.reconcile.provider,
			idPrefix: batch.reconcile.idPrefix,
			instance: batch.reconcile.instance,
		}
		delete(m.recoveryIngestSteady, target)
		return
	}
	affectedProviders := make(map[string]struct{}, 2)
	for _, obs := range batch.observations {
		if p := strings.TrimSpace(string(obs.Provider)); p != "" {
			affectedProviders[p] = struct{}{}
		}
	}
	for _, pt := range batch.points {
		if p := strings.TrimSpace(string(pt.Provider)); p != "" {
			affectedProviders[p] = struct{}{}
		}
	}
	for k := range m.recoveryIngestSteady {
		if k.orgID != orgID {
			continue
		}
		if len(affectedProviders) == 0 {
			delete(m.recoveryIngestSteady, k)
			continue
		}
		if _, match := affectedProviders[k.provider]; match {
			delete(m.recoveryIngestSteady, k)
		}
	}
}

func hashWriteString(h hash.Hash, s string) {
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(s)))
	_, _ = h.Write(lenBuf[:])
	_, _ = h.Write([]byte(s))
}

func hashWriteInt64(h hash.Hash, v int64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(v))
	_, _ = h.Write(buf[:])
}

func hashWriteBool(h hash.Hash, v bool) {
	if v {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
}

func hashWriteTimePtr(h hash.Hash, t *time.Time) {
	if t == nil || t.IsZero() {
		hashWriteBool(h, false)
		return
	}
	hashWriteBool(h, true)
	hashWriteInt64(h, t.UTC().UnixMilli())
}

func hashWriteBoolPtr(h hash.Hash, b *bool) {
	if b == nil {
		_, _ = h.Write([]byte{0})
		return
	}
	if *b {
		_, _ = h.Write([]byte{2})
	} else {
		_, _ = h.Write([]byte{1})
	}
}

func hashWriteInt64Ptr(h hash.Hash, v *int64) {
	if v == nil {
		hashWriteBool(h, false)
		return
	}
	hashWriteBool(h, true)
	hashWriteInt64(h, *v)
}

func hashWriteExternalRef(h hash.Hash, ref *recovery.ExternalRef) {
	if ref == nil {
		hashWriteBool(h, false)
		return
	}
	hashWriteBool(h, true)
	hashWriteString(h, ref.Type)
	hashWriteString(h, ref.ID)
	hashWriteString(h, ref.Name)
	hashWriteString(h, ref.Namespace)
}

func hashWriteAny(h hash.Hash, v any) {
	switch val := v.(type) {
	case nil:
		_, _ = h.Write([]byte{'0'})
	case string:
		_, _ = h.Write([]byte{'s'})
		hashWriteString(h, val)
	case bool:
		_, _ = h.Write([]byte{'b'})
		hashWriteBool(h, val)
	case int:
		_, _ = h.Write([]byte{'i'})
		hashWriteInt64(h, int64(val))
	case int64:
		_, _ = h.Write([]byte{'i'})
		hashWriteInt64(h, val)
	case uint64:
		_, _ = h.Write([]byte{'u'})
		hashWriteInt64(h, int64(val))
	case float64:
		_, _ = h.Write([]byte{'f'})
		hashWriteInt64(h, int64(math.Float64bits(val)))
	case []string:
		_, _ = h.Write([]byte{'S'})
		hashWriteInt64(h, int64(len(val)))
		for _, item := range val {
			hashWriteString(h, item)
		}
	case []any:
		_, _ = h.Write([]byte{'A'})
		hashWriteInt64(h, int64(len(val)))
		for _, item := range val {
			hashWriteAny(h, item)
		}
	case map[string]string:
		_, _ = h.Write([]byte{'M'})
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		hashWriteInt64(h, int64(len(keys)))
		for _, k := range keys {
			hashWriteString(h, k)
			hashWriteString(h, val[k])
		}
	case map[string]any:
		_, _ = h.Write([]byte{'O'})
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		hashWriteInt64(h, int64(len(keys)))
		for _, k := range keys {
			hashWriteString(h, k)
			hashWriteAny(h, val[k])
		}
	default:
		_, _ = h.Write([]byte{'J'})
		encoded, _ := json.Marshal(val)
		hashWriteString(h, string(encoded))
	}
}

func recoveryPointSemanticDigest(p recovery.RecoveryPoint) [16]byte {
	h := sha256.New()
	hashWriteString(h, strings.TrimSpace(p.ID))
	hashWriteString(h, strings.TrimSpace(string(p.Provider)))
	hashWriteString(h, strings.TrimSpace(string(p.Kind)))
	hashWriteString(h, strings.TrimSpace(string(p.Mode)))
	hashWriteString(h, strings.TrimSpace(string(p.Outcome)))
	hashWriteTimePtr(h, p.StartedAt)
	hashWriteTimePtr(h, p.CompletedAt)
	hashWriteInt64Ptr(h, p.SizeBytes)
	hashWriteBoolPtr(h, p.Verified)
	hashWriteBoolPtr(h, p.Encrypted)
	hashWriteBoolPtr(h, p.Immutable)
	hashWriteString(h, strings.TrimSpace(p.ProviderScope))
	hashWriteString(h, strings.TrimSpace(p.SubjectResourceID))
	hashWriteString(h, strings.TrimSpace(p.RepositoryResourceID))
	hashWriteExternalRef(h, p.SubjectRef)
	hashWriteExternalRef(h, p.RepositoryRef)
	hashWriteAny(h, p.Details)
	if p.Evidence == nil {
		hashWriteBool(h, false)
	} else {
		hashWriteBool(h, true)
		hashWriteString(h, p.Evidence.ID)
		hashWriteString(h, p.Evidence.Source.Provider)
		hashWriteString(h, p.Evidence.Source.Collector)
		hashWriteString(h, p.Evidence.Source.Instance)
		hashWriteString(h, p.Evidence.Subject.ResourceID)
		hashWriteString(h, p.Evidence.Subject.ProviderRef)
		hashWriteString(h, p.Evidence.Subject.ProviderScope)
		if !p.Evidence.ObservedAt.IsZero() {
			hashWriteInt64(h, p.Evidence.ObservedAt.UTC().UnixMilli())
		} else {
			hashWriteInt64(h, 0)
		}
		hashWriteTimePtr(h, p.Evidence.ValidUntil)
		hashWriteString(h, string(p.Evidence.Completeness))
		hashWriteString(h, string(p.Evidence.Confidence))
		hashWriteString(h, string(p.Evidence.Permissions))
		if p.Evidence.Reason != nil {
			hashWriteBool(h, true)
			hashWriteString(h, p.Evidence.Reason.Code)
			hashWriteString(h, p.Evidence.Reason.Message)
		} else {
			hashWriteBool(h, false)
		}
		if p.Evidence.PayloadRef != nil {
			hashWriteBool(h, true)
			hashWriteString(h, p.Evidence.PayloadRef.Kind)
			hashWriteString(h, p.Evidence.PayloadRef.ID)
		} else {
			hashWriteBool(h, false)
		}
		if p.Evidence.Correlation != nil {
			hashWriteBool(h, true)
			hashWriteString(h, p.Evidence.Correlation.Rule)
			hashWriteInt64(h, int64(p.Evidence.Correlation.CandidateCount))
			hashWriteAny(h, p.Evidence.Correlation.MatchedFields)
		} else {
			hashWriteBool(h, false)
		}
	}
	var sum [32]byte
	h.Sum(sum[:0])
	var out [16]byte
	copy(out[:], sum[:16])
	return out
}

func recoveryObservationsSemanticDigest(observations []recovery.ProtectionProviderObservation) [32]byte {
	h := sha256.New()
	hashWriteInt64(h, int64(len(observations)))
	for _, obs := range observations {
		hashWriteString(h, strings.TrimSpace(string(obs.Provider)))
		hashWriteString(h, strings.TrimSpace(obs.Source))
		hashWriteString(h, strings.TrimSpace(obs.Scope))
		hashWriteString(h, strings.TrimSpace(string(obs.JobState)))
		hashWriteString(h, strings.TrimSpace(string(obs.HistoryCompleteness)))
		hashWriteString(h, strings.TrimSpace(string(obs.Permissions)))
		hashWriteBool(h, obs.VerificationExpected)
		unread := recovery.NormalizeProtectionUnreadScopes(obs.UnreadScopes)
		hashWriteInt64(h, int64(len(unread)))
		for _, u := range unread {
			hashWriteString(h, string(u.Kind))
			hashWriteString(h, u.Node)
			hashWriteString(h, u.Storage)
			hashWriteString(h, u.Namespace)
		}
		if obs.Evidence.Reason != nil {
			hashWriteBool(h, true)
			hashWriteString(h, obs.Evidence.Reason.Code)
			hashWriteString(h, obs.Evidence.Reason.Message)
		} else {
			hashWriteBool(h, false)
		}
	}
	var out [32]byte
	h.Sum(out[:0])
	return out
}

func recoveryUnsettledDigest(evaluated, unsettled []string) [32]byte {
	h := sha256.New()
	writeSet := func(values []string) {
		set := make(map[string]struct{}, len(values))
		for _, v := range values {
			if v = strings.TrimSpace(v); v != "" {
				set[v] = struct{}{}
			}
		}
		sorted := make([]string, 0, len(set))
		for v := range set {
			sorted = append(sorted, v)
		}
		sort.Strings(sorted)
		hashWriteInt64(h, int64(len(sorted)))
		for _, v := range sorted {
			hashWriteString(h, v)
		}
	}
	writeSet(evaluated)
	writeSet(unsettled)
	var out [32]byte
	h.Sum(out[:0])
	return out
}

// retireUnsettledBackupScopes forgets the unsettled guests of PBS connections that
// have been removed, because a removed connection never polls again to replace its
// own answer. It runs on ingest, where the store is already open.
func (m *Monitor) retireUnsettledBackupScopes(ctx context.Context, store *recoverystore.Store, orgID string) {
	m.mu.RLock()
	cfg := m.config
	m.mu.RUnlock()
	if cfg == nil {
		return
	}
	scopes := make([]string, 0, len(cfg.PBSInstances))
	for i := range cfg.PBSInstances {
		scopes = append(scopes, cfg.PBSInstances[i].Name)
	}
	if err := store.RetainUnsettledProtectionScopes(ctx, scopes); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to retire unsettled backup scopes")
	}
}

func (m *Monitor) purgeStalePVEPBSBackupsBestEffort(ctx context.Context) {
	if m == nil {
		return
	}

	m.mu.RLock()
	rm := m.recoveryManager
	orgID := strings.TrimSpace(m.orgID)
	hasPBSDirectConnection := m.config != nil && len(m.config.PBSInstances) > 0
	m.mu.RUnlock()

	if !hasPBSDirectConnection || rm == nil {
		return
	}
	if orgID == "" {
		orgID = "default"
	}
	if ctx == nil {
		ctx = context.Background()
	}

	purgeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	store, err := rm.StoreForOrg(orgID)
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to open recovery store for stale PBS backup purge")
		return
	}

	if err := store.PurgeStalePVEPBSBackups(purgeCtx); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to purge stale PVE-sourced PBS backup entries")
		return
	}
	m.invalidateRecoverySteadyStateForOrg(orgID)
}

func buildProxmoxGuestInfoIndex(readState unifiedresources.ReadState) map[string]proxmoxmapper.GuestInfo {
	if readState == nil {
		return map[string]proxmoxmapper.GuestInfo{}
	}
	vmViews := readState.VMs()
	containerViews := readState.Containers()
	out := make(map[string]proxmoxmapper.GuestInfo, len(vmViews)+len(containerViews))

	for _, vm := range vmViews {
		if vm == nil || vm.Template() {
			continue
		}
		key := proxmoxmapperKey(vm.Instance(), vm.Node(), vm.VMID())
		resourceID := strings.TrimSpace(vm.ID())
		sourceID := strings.TrimSpace(vm.SourceID())
		if sourceID == "" {
			sourceID = makeGuestID(vm.Instance(), vm.Node(), vm.VMID())
		}
		out[key] = proxmoxmapper.GuestInfo{
			ResourceID:   resourceID,
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeVM,
			Name:         strings.TrimSpace(vm.Name()),
		}
	}

	for _, ct := range containerViews {
		if ct == nil || ct.Template() {
			continue
		}
		key := proxmoxmapperKey(ct.Instance(), ct.Node(), ct.VMID())
		resourceID := strings.TrimSpace(ct.ID())
		sourceID := strings.TrimSpace(ct.SourceID())
		if sourceID == "" {
			sourceID = makeGuestID(ct.Instance(), ct.Node(), ct.VMID())
		}
		out[key] = proxmoxmapper.GuestInfo{
			ResourceID:   resourceID,
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeSystemContainer,
			Name:         strings.TrimSpace(ct.Name()),
		}
	}

	return out
}

func buildPBSGuestCandidates(readState unifiedresources.ReadState) map[string][]proxmoxmapper.GuestCandidate {
	if readState == nil {
		return map[string][]proxmoxmapper.GuestCandidate{}
	}
	vmViews := readState.VMs()
	containerViews := readState.Containers()
	out := make(map[string][]proxmoxmapper.GuestCandidate, len(vmViews)+len(containerViews))

	for _, vm := range vmViews {
		if vm == nil || vm.Template() || vm.VMID() <= 0 {
			continue
		}
		key := "vm:" + fmt.Sprintf("%d", vm.VMID())
		resourceID := strings.TrimSpace(vm.ID())
		sourceID := strings.TrimSpace(vm.SourceID())
		if sourceID == "" {
			sourceID = makeGuestID(vm.Instance(), vm.Node(), vm.VMID())
		}
		out[key] = append(out[key], proxmoxmapper.GuestCandidate{
			ResourceID:   resourceID,
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeVM,
			DisplayName:  strings.TrimSpace(vm.Name()),
			InstanceName: strings.TrimSpace(vm.Instance()),
			NodeName:     strings.TrimSpace(vm.Node()),
			VMID:         vm.VMID(),
		})
	}

	for _, ct := range containerViews {
		if ct == nil || ct.Template() || ct.VMID() <= 0 {
			continue
		}
		key := "ct:" + fmt.Sprintf("%d", ct.VMID())
		resourceID := strings.TrimSpace(ct.ID())
		sourceID := strings.TrimSpace(ct.SourceID())
		if sourceID == "" {
			sourceID = makeGuestID(ct.Instance(), ct.Node(), ct.VMID())
		}
		out[key] = append(out[key], proxmoxmapper.GuestCandidate{
			ResourceID:   resourceID,
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeSystemContainer,
			DisplayName:  strings.TrimSpace(ct.Name()),
			InstanceName: strings.TrimSpace(ct.Instance()),
			NodeName:     strings.TrimSpace(ct.Node()),
			VMID:         ct.VMID(),
		})
	}

	return out
}

func proxmoxmapperKey(instanceName, nodeName string, vmid int) string {
	// Keep key format in one place so ingestion and mapping stay consistent.
	return fmt.Sprintf("%s|%s|%d", strings.TrimSpace(instanceName), strings.TrimSpace(nodeName), vmid)
}
