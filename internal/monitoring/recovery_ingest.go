package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// recoveryReconcileScope identifies one class of points for one polled
// connection whose latest enumeration is authoritative: after the batch is
// upserted, points in scope that were not part of the batch are deleted.
type recoveryReconcileScope struct {
	provider string
	idPrefix string
	instance string
}

// recoveryIngestBatch is one poll cycle's worth of recovery points, plus an
// optional reconcile scope when the points are a complete enumeration.
type recoveryIngestBatch struct {
	points       []recovery.RecoveryPoint
	observations []recovery.ProtectionProviderObservation
	reconcile    *recoveryReconcileScope
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
		// Queue rather than replace: batches come from different sources (PVE
		// storage, PBS, snapshots), and dropping one loses a full poll cycle
		// for that source.
		m.recoveryIngestPending = append(m.recoveryIngestPending, batch)
		m.recoveryIngestMu.Unlock()
		log.Debug().
			Int("points", len(batch.points)).
			Int("provider_observations", len(batch.observations)).
			Msg("Queued recovery point ingest behind active batch")
		return
	}
	m.recoveryIngestRunning = true
	m.recoveryIngestMu.Unlock()

	go m.runRecoveryPointIngestLoop([]recoveryIngestBatch{batch})
}

func (m *Monitor) runRecoveryPointIngestLoop(batches []recoveryIngestBatch) {
	for len(batches) > 0 {
		for _, batch := range batches {
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
	if _, err := store.ReconcileInstancePoints(ctx, batch.reconcile.provider, batch.reconcile.idPrefix, batch.reconcile.instance, keepIDs); err != nil {
		log.Warn().
			Err(err).
			Str("org_id", orgID).
			Str("instance", batch.reconcile.instance).
			Str("idPrefix", batch.reconcile.idPrefix).
			Msg("Failed to reconcile recovery points against source enumeration")
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
	}
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
		sourceID := strings.TrimSpace(vm.ID())
		if sourceID == "" {
			sourceID = makeGuestID(vm.Instance(), vm.Node(), vm.VMID())
		}
		out[key] = proxmoxmapper.GuestInfo{
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
		sourceID := strings.TrimSpace(ct.ID())
		if sourceID == "" {
			sourceID = makeGuestID(ct.Instance(), ct.Node(), ct.VMID())
		}
		out[key] = proxmoxmapper.GuestInfo{
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
		sourceID := strings.TrimSpace(vm.ID())
		if sourceID == "" {
			sourceID = makeGuestID(vm.Instance(), vm.Node(), vm.VMID())
		}
		out[key] = append(out[key], proxmoxmapper.GuestCandidate{
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
		sourceID := strings.TrimSpace(ct.ID())
		if sourceID == "" {
			sourceID = makeGuestID(ct.Instance(), ct.Node(), ct.VMID())
		}
		out[key] = append(out[key], proxmoxmapper.GuestCandidate{
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
