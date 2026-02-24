package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) ingestRecoveryPointsAsync(points []recovery.RecoveryPoint) {
	if m == nil || len(points) == 0 {
		return
	}
	go func() {
		// SQLite upserts are usually quick, but allow a bit of time for large batches.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		m.ingestRecoveryPointsBestEffort(ctx, points)
	}()
}

func (m *Monitor) ingestRecoveryPointsBestEffort(ctx context.Context, points []recovery.RecoveryPoint) {
	if m == nil || len(points) == 0 {
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

	if err := store.UpsertPoints(ctx, points); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Int("points", len(points)).Msg("Failed to upsert recovery points from backup polling")
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

func buildProxmoxGuestInfoIndex(snapshot models.StateSnapshot) map[string]proxmoxmapper.GuestInfo {
	out := make(map[string]proxmoxmapper.GuestInfo, len(snapshot.VMs)+len(snapshot.Containers))

	for _, vm := range snapshot.VMs {
		if vm.Template {
			continue
		}
		key := proxmoxmapperKey(vm.Instance, vm.Node, vm.VMID)
		sourceID := strings.TrimSpace(vm.ID)
		if sourceID == "" {
			sourceID = makeGuestID(vm.Instance, vm.Node, vm.VMID)
		}
		out[key] = proxmoxmapper.GuestInfo{
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeVM,
			Name:         strings.TrimSpace(vm.Name),
		}
	}

	for _, ct := range snapshot.Containers {
		if ct.Template {
			continue
		}
		key := proxmoxmapperKey(ct.Instance, ct.Node, ct.VMID)
		sourceID := strings.TrimSpace(ct.ID)
		if sourceID == "" {
			sourceID = makeGuestID(ct.Instance, ct.Node, ct.VMID)
		}
		out[key] = proxmoxmapper.GuestInfo{
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeSystemContainer,
			Name:         strings.TrimSpace(ct.Name),
		}
	}

	return out
}

func proxmoxmapperKey(instanceName, nodeName string, vmid int) string {
	// Keep key format in one place so ingestion and mapping stay consistent.
	return fmt.Sprintf("%s|%s|%d", strings.TrimSpace(instanceName), strings.TrimSpace(nodeName), vmid)
}
