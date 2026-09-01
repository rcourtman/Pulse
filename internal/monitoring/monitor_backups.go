package monitoring

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxrecoverymapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

const (
	pbsBackupSnapshotFetchWorkers = 5
	// runningVzdumpClockSkew is the slack applied when deciding whether a
	// backup file's ctime falls inside a running vzdump task's window, so a
	// small clock offset between Pulse and the node cannot misclassify the
	// task's own partial output as a completed backup.
	runningVzdumpClockSkew = 2 * time.Minute
	// pbsBackupSnapshotsPerGroupLimit bounds how many real snapshots are
	// retained per backup group (newest first). It must comfortably exceed
	// common PBS keep policies: groups are always represented by real fetched
	// snapshots, never synthesized group metadata, because a synthesized entry
	// has no verification, size, file, or per-snapshot time data (issue #1541).
	pbsBackupSnapshotsPerGroupLimit = 100
	// pbsBackupLiveStateLimit bounds the per-instance PBS backup list as a
	// whole. Groups are processed newest-first so the newest restore points
	// win when the limit is hit (issue #1524).
	pbsBackupLiveStateLimit = 5000
)

func pveBackupTemplateSubjectKey(instance, guestType, node string, vmid int) string {
	return alerts.BuildBackupPVETemplateSubjectKey(instance, guestType, node, vmid)
}

func (m *Monitor) updatePVEBackupTemplateSubjectsForType(instanceName, guestType string, subjects map[string]struct{}) {
	if m == nil {
		return
	}
	instanceName = strings.TrimSpace(instanceName)
	guestType = strings.TrimSpace(guestType)
	if instanceName == "" || guestType == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pveBackupInventoryReady == nil {
		m.pveBackupInventoryReady = make(map[string]map[string]bool)
	}
	if m.pveBackupInventoryReady[instanceName] == nil {
		m.pveBackupInventoryReady[instanceName] = make(map[string]bool)
	}
	m.pveBackupInventoryReady[instanceName][guestType] = true

	if m.pveBackupTemplateSubjects == nil {
		m.pveBackupTemplateSubjects = make(map[string]map[string]struct{})
	}
	existing := m.pveBackupTemplateSubjects[instanceName]
	if existing == nil {
		existing = make(map[string]struct{})
	}
	prefix := instanceName + "\x00" + guestType + "\x00"
	for key := range existing {
		if strings.HasPrefix(key, prefix) {
			delete(existing, key)
		}
	}
	for key := range subjects {
		if key != "" {
			existing[key] = struct{}{}
		}
	}
	m.pveBackupTemplateSubjects[instanceName] = existing
}

func (m *Monitor) updatePVEBackupTemplateSubjectsFromClusterResources(instanceName string, resources []proxmox.ClusterResource) {
	qemuTemplates := make(map[string]struct{})
	lxcTemplates := make(map[string]struct{})
	for _, res := range resources {
		if res.Template != 1 {
			continue
		}
		switch strings.TrimSpace(res.Type) {
		case "qemu":
			if key := pveBackupTemplateSubjectKey(instanceName, "qemu", res.Node, res.VMID); key != "" {
				qemuTemplates[key] = struct{}{}
			}
		case "lxc":
			if key := pveBackupTemplateSubjectKey(instanceName, "lxc", res.Node, res.VMID); key != "" {
				lxcTemplates[key] = struct{}{}
			}
		}
	}
	m.updatePVEBackupTemplateSubjectsForType(instanceName, "qemu", qemuTemplates)
	m.updatePVEBackupTemplateSubjectsForType(instanceName, "lxc", lxcTemplates)
}

func quotePVEACLTokenID(tokenID string) string {
	return "'" + strings.ReplaceAll(tokenID, "'", `'"'"'`) + "'"
}

func pveBackupPermissionWarning(instanceCfg *config.PVEInstance) string {
	warning := "Missing PVEDatastoreAdmin permission on /storage. Run: pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin"
	if instanceCfg == nil {
		return warning + "; if using a privilege-separated API token, also grant PVEDatastoreAdmin on /storage to that token."
	}
	tokenID := strings.TrimSpace(instanceCfg.TokenName)
	if tokenID == "" || !strings.Contains(tokenID, "!") {
		return warning + "; if using a privilege-separated API token, also grant PVEDatastoreAdmin on /storage to that token."
	}
	return warning + " && pveum aclmod /storage -token " + quotePVEACLTokenID(tokenID) + " -role PVEDatastoreAdmin"
}

func (m *Monitor) backupInventoryScopeForAlerts() *alerts.BackupInventoryScope {
	if m == nil {
		return nil
	}
	scope := &alerts.BackupInventoryScope{
		PVEOrphanInventoryReady: make(map[string]map[string]bool),
		PVETemplateSubjects:     make(map[string]struct{}),
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for instance, readyByType := range m.pveBackupInventoryReady {
		if len(readyByType) == 0 {
			continue
		}
		scope.PVEOrphanInventoryReady[instance] = make(map[string]bool, len(readyByType))
		for guestType, ready := range readyByType {
			scope.PVEOrphanInventoryReady[instance][guestType] = ready
		}
	}
	for _, subjects := range m.pveBackupTemplateSubjects {
		for key := range subjects {
			scope.PVETemplateSubjects[key] = struct{}{}
		}
	}
	return scope
}

func (m *Monitor) pollStorageBackupsWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {

	var allBackups []models.StorageBackup
	var pbsConfirmations []models.PBSGuestConfirmation
	hasPBSDirectConnection := m.config != nil && len(m.config.PBSInstances) > 0
	seenVolids := make(map[string]bool) // Track seen volume IDs to avoid duplicates
	hadSuccessfulNode := false          // Track if at least one node responded successfully
	successfulNodeQueries := 0          // Number of nodes whose storage list was enumerated
	storagesWithBackup := 0             // Number of storages that should contain backups
	contentSuccess := 0                 // Number of successful storage content fetches
	contentFailures := 0                // Number of failed storage content fetches
	storageQueryErrors := 0             // Number of nodes where storage list could not be queried
	hadPermissionError := false         // Track if any permission errors occurred this cycle
	permissionFailureCount := 0         // Number of node/storage reads rejected by authorization
	storagePreserveNeeded := map[string]struct{}{}
	storageSuccess := map[string]struct{}{}
	readState := m.backupReadStateForInstance(instanceName)

	// Build guest lookup map to find actual node for each VMID
	snapshot := m.state.GetSnapshot()
	guestNodeMap := make(map[int]string) // VMID -> actual node name
	populateGuestNodeMapFromReadState(readState, instanceName, guestNodeMap)

	// vzdump writes archives under their final name, so a backup that is
	// still running is already listed by the storage content API as a
	// growing partial file. Correlate content entries with the live vzdump
	// tasks polled just before this pass (pollBackupTasks runs first in the
	// cycle): a guest backup file created at or after a running task's start
	// is that task's partial output, not a completed backup.
	runningVzdumpStart := make(map[int]time.Time) // VMID -> earliest running task start
	for _, task := range snapshot.PVEBackups.BackupTasks {
		if task.Instance != instanceName || task.VMID <= 0 || !task.EndTime.IsZero() {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if status != "" && status != "running" {
			continue
		}
		if existing, ok := runningVzdumpStart[task.VMID]; !ok || task.StartTime.Before(existing) {
			runningVzdumpStart[task.VMID] = task.StartTime
		}
	}

	// For each node, get storage and check content
	for _, node := range nodes {
		if nodeEffectiveStatus[node.Node] != "online" {
			for _, storageName := range storageNamesForNode(readState, instanceName, node.Node) {
				storagePreserveNeeded[storageName] = struct{}{}
			}
			continue
		}

		// Get storage for this node - retry once on timeout
		var storages []proxmox.Storage
		var err error

		for attempt := 1; attempt <= 2; attempt++ {
			storages, err = client.GetStorage(ctx, node.Node)
			if err == nil {
				break // Success
			}

			// Check if it's a timeout error
			errStr := err.Error()
			if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
				if attempt == 1 {
					log.Warn().
						Str("node", node.Node).
						Str("instance", instanceName).
						Msg("Storage query timed out, retrying with extended timeout...")
					// Give it a bit more time on retry
					time.Sleep(2 * time.Second)
					continue
				}
			}
			// Non-timeout error or second attempt failed
			break
		}

		if err != nil {
			if isPVEBackupPermissionError(err) {
				hadPermissionError = true
				permissionFailureCount++
			}
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_for_backups", instanceName, err).WithNode(node.Node)
			log.Warn().Err(monErr).Str("node", node.Node).Msg("failed to get storage for backups - skipping node")
			for _, storageName := range storageNamesForNode(readState, instanceName, node.Node) {
				storagePreserveNeeded[storageName] = struct{}{}
			}
			storageQueryErrors++
			continue
		}

		hadSuccessfulNode = true
		successfulNodeQueries++

		// For each storage that can contain backups or templates
		for _, storage := range storages {
			// Check if storage supports backup content
			if !strings.Contains(storage.Content, "backup") {
				continue
			}
			if !storageContentQueryable(storage) {
				continue
			}

			storagesWithBackup++

			// Get storage content
			contents, err := client.GetStorageContent(ctx, node.Node, storage.Storage)
			if err != nil {
				monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_content", instanceName, err).WithNode(node.Node)
				// Check if this is a permission error
				if isPVEBackupPermissionError(err) {
					hadPermissionError = true
					permissionFailureCount++
					warning := pveBackupPermissionWarning(m.getInstanceConfig(instanceName))
					m.mu.Lock()
					m.backupPermissionWarnings[instanceName] = warning
					m.mu.Unlock()
					log.Warn().
						Str("instance", instanceName).
						Str("node", node.Node).
						Str("storage", storage.Storage).
						Msg("Backup permission denied - PVEDatastoreAdmin role may be missing on /storage")
				} else {
					log.Debug().Err(monErr).
						Str("node", node.Node).
						Str("storage", storage.Storage).
						Msg("Failed to get storage content")
				}
				if _, ok := storageSuccess[storage.Storage]; !ok {
					storagePreserveNeeded[storage.Storage] = struct{}{}
				}
				contentFailures++
				continue
			}

			contentSuccess++
			storageSuccess[storage.Storage] = struct{}{}
			delete(storagePreserveNeeded, storage.Storage)

			// Convert to models
			for _, content := range contents {
				// Skip if we've already seen this item (shared storage duplicate)
				if seenVolids[content.Volid] {
					continue
				}
				seenVolids[content.Volid] = true

				// Skip templates and ISOs - they're not backups
				if content.Content == "vztmpl" || content.Content == "iso" {
					continue
				}

				// Determine type from content type and VMID
				backupType := "unknown"
				if content.VMID == 0 {
					backupType = "host"
				} else if strings.Contains(content.Volid, "/vm/") || strings.Contains(content.Volid, "qemu") {
					backupType = "qemu"
				} else if strings.Contains(content.Volid, "/ct/") || strings.Contains(content.Volid, "lxc") {
					backupType = "lxc"
				} else if strings.Contains(content.Format, "pbs-ct") {
					// PBS format check as fallback
					backupType = "lxc"
				} else if strings.Contains(content.Format, "pbs-vm") {
					// PBS format check as fallback
					backupType = "qemu"
				}

				// Determine the correct node: for guest backups (VMID > 0), use the actual guest's node
				// For host backups (VMID == 0), use the node where the backup was found
				backupNode := node.Node
				if content.VMID > 0 {
					if actualNode, found := guestNodeMap[content.VMID]; found {
						backupNode = actualNode
					}
					// If not found in map, fall back to queried node (shouldn't happen normally)
				}
				// Classify by storage type only. A dir/NFS storage is free to be
				// NAMED "pbs-…" (e.g. "pbs-backup"), and treating the name prefix
				// as PBS silently dropped its vzdump backups whenever a direct PBS
				// connection was also configured (#1592).
				isPBSStorage := storage.Type == "pbs"
				if isPBSStorage && hasPBSDirectConnection {
					// The direct PBS connection is authoritative for the
					// backup list, but which storage view listed a snapshot
					// is evidence only this connection has: it helps
					// attribute a snapshot whose VMID exists on several
					// clusters (#1639). Keep that association, tagged with
					// the storage it came from, even though the raw entry is
					// dropped - a listing only proves this connection can see
					// the snapshot, so consumers need the storage to tell an
					// exclusive view from a shared or synced datastore.
					if confirmationType := pbsGuestConfirmationType(backupType); confirmationType != "" && content.VMID > 0 && content.CTime > 0 {
						pbsConfirmations = append(pbsConfirmations, models.PBSGuestConfirmation{
							Storage:    storage.Storage,
							BackupType: confirmationType,
							VMID:       content.VMID,
							Time:       content.CTime,
						})
					}
					log.Debug().
						Str("instance", instanceName).
						Str("node", node.Node).
						Str("storage", storage.Storage).
						Str("volid", content.Volid).
						Msg("Skipping PBS backup from PVE storage - PBS direct connection is authoritative")
					continue
				}

				// Check verification status for PBS backups
				verified := false
				verificationInfo := ""
				if isPBSStorage {
					// Check if verified flag is set
					if content.Verified > 0 {
						verified = true
					}
					// Also check verification map if available
					if content.Verification != nil {
						if state, ok := content.Verification["state"].(string); ok {
							verified = (state == "ok")
							verificationInfo = state
						}
					}
				}

				// Partial-output window: the file belongs to a vzdump task
				// that is still running. PBS-type storages additionally list
				// in-flight snapshots without a size (no manifest yet), which
				// covers backups submitted by another cluster or host.
				inProgress := false
				if content.VMID > 0 {
					if start, ok := runningVzdumpStart[content.VMID]; ok &&
						!time.Unix(content.CTime, 0).Before(start.Add(-runningVzdumpClockSkew)) {
						inProgress = true
					}
					if isPBSStorage && content.Size == 0 {
						inProgress = true
					}
				}

				backup := models.StorageBackup{
					ID:           fmt.Sprintf("%s-%s", instanceName, content.Volid),
					Storage:      storage.Storage,
					Node:         backupNode,
					Instance:     instanceName,
					Type:         backupType,
					VMID:         content.VMID,
					Time:         time.Unix(content.CTime, 0),
					CTime:        content.CTime,
					Size:         int64(content.Size),
					Format:       content.Format,
					Notes:        content.Notes,
					Protected:    content.Protected > 0,
					Volid:        content.Volid,
					IsPBS:        isPBSStorage,
					Verified:     verified,
					Verification: verificationInfo,
					InProgress:   inProgress,
				}

				allBackups = append(allBackups, backup)
			}
		}
	}

	allBackups, preservedStorages := preserveFailedStorageBackups(instanceName, snapshot, storagePreserveNeeded, allBackups)
	if len(preservedStorages) > 0 {
		log.Warn().
			Str("instance", instanceName).
			Strs("storages", preservedStorages).
			Msg("Preserving previous storage backup data due to partial failures")
	}

	pbsConfirmations, preservedConfirmationStorages := preserveFailedPBSGuestConfirmations(
		m.state.PBSGuestConfirmationsForInstance(instanceName), storagePreserveNeeded, pbsConfirmations)
	if len(preservedConfirmationStorages) > 0 {
		log.Warn().
			Str("instance", instanceName).
			Strs("storages", preservedConfirmationStorages).
			Msg("Preserving previous PBS guest confirmation evidence due to partial failures")
	}

	protectionObservedAt := time.Now().UTC()
	protectionObservation, protectionObservationErr := buildPVEProtectionProviderObservation(
		instanceName,
		pveProtectionCollectionStats{
			NodeCount:              len(nodes),
			SuccessfulNodeQueries:  successfulNodeQueries,
			BackupStorageCount:     storagesWithBackup,
			ContentSuccesses:       contentSuccess,
			ContentFailures:        contentFailures,
			PermissionFailureCount: permissionFailureCount,
		},
		protectionObservedAt,
	)
	if protectionObservationErr != nil {
		log.Warn().
			Err(protectionObservationErr).
			Str("instance", instanceName).
			Msg("Failed to build PVE protection provider observation")
	}

	// Decide whether to keep existing backups when every query failed
	if shouldPreserveBackups(len(nodes), hadSuccessfulNode, storagesWithBackup, contentSuccess) {
		if protectionObservationErr == nil {
			m.ingestProtectionProviderObservationsAsync(
				[]recovery.ProtectionProviderObservation{protectionObservation},
			)
		}
		if len(nodes) > 0 && !hadSuccessfulNode {
			log.Warn().
				Str("instance", instanceName).
				Int("nodes", len(nodes)).
				Int("errors", storageQueryErrors).
				Msg("Failed to query storage on all nodes; keeping previous backup list")
		} else if storagesWithBackup > 0 && contentSuccess == 0 {
			log.Warn().
				Str("instance", instanceName).
				Int("storages", storagesWithBackup).
				Int("failures", contentFailures).
				Msg("All storage content queries failed; keeping previous backup list")
		}
		return
	}

	// Update state with storage backups for this instance
	m.state.UpdateStorageBackupsForInstance(instanceName, allBackups)
	m.state.UpdatePBSGuestConfirmationsForInstance(instanceName, pbsConfirmations)

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	guestInfo := buildProxmoxGuestInfoIndex(readState)
	points, evidenceErr := proxmoxrecoverymapper.FromPVEStorageBackupsWithEvidence(
		allBackups,
		guestInfo,
		protectionObservedAt,
	)
	if evidenceErr != nil {
		log.Warn().
			Err(evidenceErr).
			Str("instance", instanceName).
			Msg("Failed to attach PVE recovery evidence; preserving existing points")
		if protectionObservationErr == nil {
			m.ingestProtectionProviderObservationsAsync(
				[]recovery.ProtectionProviderObservation{protectionObservation},
			)
		}
	} else {
		observations := []recovery.ProtectionProviderObservation{}
		if protectionObservationErr == nil {
			observations = append(observations, protectionObservation)
		}
		m.ingestAndReconcileRecoveryPointsWithObservationsAsync(
			points,
			observations,
			recoveryReconcileScope{
				provider: string(recovery.ProviderProxmoxPVE),
				idPrefix: "pve-backup:",
				instance: instanceName,
			},
		)
	}

	// Sync backup times to VMs/Containers and republish them to canonical resources.
	m.syncGuestBackupTimesAndResourceStore()

	if m.alertManager != nil {
		guestsByKey, guestsByVMID := buildGuestLookupsFromReadState(m.GetUnifiedReadStateOrSnapshot(), m.guestMetadataStore)
		rollups, err := m.listBackupRollupsForAlerts(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list recovery rollups for backup alerts")
		} else {
			m.alertManager.CheckBackupsWithInventory(rollups, guestsByKey, guestsByVMID, m.backupInventoryScopeForAlerts())
		}
	}

	// Clear permission warning if no permission errors occurred this cycle
	if !hadPermissionError {
		m.mu.Lock()
		delete(m.backupPermissionWarnings, instanceName)
		m.mu.Unlock()
	}

	log.Debug().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("Storage backups polled")

	// Immediately broadcast the updated state so frontend sees new backups
	m.broadcastStateUpdate()
}

// pbsGuestConfirmationType maps a PVE storage-content backup type to PBS
// snapshot nomenclature. Host and unclassifiable entries return "" and are
// not usable as guest attribution evidence.
func pbsGuestConfirmationType(backupType string) string {
	switch backupType {
	case "qemu":
		return "vm"
	case "lxc":
		return "ct"
	default:
		return ""
	}
}

func shouldPreserveBackups(nodeCount int, hadSuccessfulNode bool, storagesWithBackup, contentSuccess int) bool {
	if nodeCount > 0 && !hadSuccessfulNode {
		return true
	}
	if storagesWithBackup > 0 && contentSuccess == 0 {
		return true
	}
	return false
}

func (m *Monitor) syncGuestBackupTimesAndResourceStore() {
	if m == nil || m.state == nil {
		return
	}

	m.state.SyncGuestBackupTimes()
	m.updateResourceStore(m.state.GetSnapshot())
}

func (m *Monitor) backupReadStateForInstance(instanceName string) unifiedresources.ReadState {
	if m == nil {
		return nil
	}
	readState := m.GetUnifiedReadStateOrSnapshot()
	if backupReadStateHasGuestForInstance(readState, instanceName) || m.state == nil {
		return readState
	}

	snapshot := m.state.GetSnapshot()
	if !backupSnapshotHasGuestForInstance(snapshot, instanceName) {
		return readState
	}

	m.updateResourceStore(snapshot)
	readState = m.GetUnifiedReadStateOrSnapshot()
	if backupReadStateHasGuestForInstance(readState, instanceName) {
		return readState
	}

	return monitorUnifiedStateViewFromSnapshot(snapshot).readState
}

func backupReadStateHasGuestForInstance(readState unifiedresources.ReadState, instanceName string) bool {
	if readState == nil {
		return false
	}
	for _, vm := range readState.VMs() {
		if vm != nil && vm.Instance() == instanceName {
			return true
		}
	}
	for _, ct := range readState.Containers() {
		if ct != nil && ct.Instance() == instanceName {
			return true
		}
	}
	return false
}

func backupSnapshotHasGuestForInstance(snapshot models.StateSnapshot, instanceName string) bool {
	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName {
			return true
		}
	}
	for _, ct := range snapshot.Containers {
		if ct.Instance == instanceName {
			return true
		}
	}
	return false
}

func storageNamesForNode(readState unifiedresources.ReadState, instanceName, nodeName string) []string {
	if readState == nil || nodeName == "" {
		return nil
	}

	var storages []string
	for _, storage := range readState.StoragePools() {
		if storage == nil || storage.Instance() != instanceName {
			continue
		}
		if storage.Name() == "" {
			continue
		}
		if !strings.Contains(storage.Content(), "backup") {
			continue
		}
		if storage.Node() == nodeName {
			storages = append(storages, storage.Name())
			continue
		}
		for _, node := range storage.AccessibleNodes() {
			if node == nodeName {
				storages = append(storages, storage.Name())
				break
			}
		}
	}

	return storages
}

// preserveFailedPBSGuestConfirmations carries forward the PBS guest
// confirmation evidence of storages whose content query failed this cycle,
// mirroring preserveFailedStorageBackups. Publishing the partial (possibly
// empty) set instead would evict attribution evidence on a transient poll
// failure and flip collision-VMID guests between cycles (#1639).
func preserveFailedPBSGuestConfirmations(previous []models.PBSGuestConfirmation, storagesToPreserve map[string]struct{}, current []models.PBSGuestConfirmation) ([]models.PBSGuestConfirmation, []string) {
	if len(storagesToPreserve) == 0 || len(previous) == 0 {
		return current, nil
	}

	existing := make(map[models.PBSGuestConfirmation]struct{}, len(current))
	for _, confirmation := range current {
		existing[confirmation] = struct{}{}
	}

	preserved := make(map[string]struct{})
	for _, confirmation := range previous {
		if _, ok := storagesToPreserve[confirmation.Storage]; !ok {
			continue
		}
		if _, duplicate := existing[confirmation]; duplicate {
			continue
		}
		current = append(current, confirmation)
		existing[confirmation] = struct{}{}
		preserved[confirmation.Storage] = struct{}{}
	}

	if len(preserved) == 0 {
		return current, nil
	}

	storages := make([]string, 0, len(preserved))
	for storage := range preserved {
		storages = append(storages, storage)
	}
	sort.Strings(storages)
	return current, storages
}

func preserveFailedStorageBackups(instanceName string, snapshot models.StateSnapshot, storagesToPreserve map[string]struct{}, current []models.StorageBackup) ([]models.StorageBackup, []string) {
	if len(storagesToPreserve) == 0 {
		return current, nil
	}

	existing := make(map[string]struct{}, len(current))
	for _, backup := range current {
		existing[backup.ID] = struct{}{}
	}

	preserved := make(map[string]struct{})
	for _, backup := range snapshot.PVEBackups.StorageBackups {
		if backup.Instance != instanceName {
			continue
		}
		if _, ok := storagesToPreserve[backup.Storage]; !ok {
			continue
		}
		if _, duplicate := existing[backup.ID]; duplicate {
			continue
		}
		current = append(current, backup)
		existing[backup.ID] = struct{}{}
		preserved[backup.Storage] = struct{}{}
	}

	if len(preserved) == 0 {
		return current, nil
	}

	storages := make([]string, 0, len(preserved))
	for storage := range preserved {
		storages = append(storages, storage)
	}
	sort.Strings(storages)
	return current, storages
}

func buildGuestLookupsFromReadState(readState unifiedresources.ReadState, metadataStore *config.GuestMetadataStore) (map[string]alerts.GuestLookup, map[string][]alerts.GuestLookup) {
	byKey := make(map[string]alerts.GuestLookup)
	byVMID := make(map[string][]alerts.GuestLookup)

	if readState == nil {
		if metadataStore != nil {
			enrichWithPersistedMetadata(metadataStore, byVMID)
		}
		return byKey, byVMID
	}

	for _, vm := range readState.VMs() {
		if vm == nil {
			continue
		}
		info := alerts.GuestLookup{
			ResourceID: makeGuestID(vm.Instance(), vm.Node(), vm.VMID()),
			Name:       vm.Name(),
			Instance:   vm.Instance(),
			Node:       vm.Node(),
			Type:       "qemu",
			VMID:       vm.VMID(),
			Tags:       vm.Tags(),
		}
		key := alerts.BuildGuestKey(vm.Instance(), vm.Node(), vm.VMID())
		byKey[key] = info

		vmidKey := strconv.Itoa(vm.VMID())
		byVMID[vmidKey] = append(byVMID[vmidKey], info)

		// Persist last-known name and type for this guest
		if metadataStore != nil && vm.Name() != "" {
			persistGuestIdentity(metadataStore, key, vm.Name(), info.Type)
		}
	}

	for _, ct := range readState.Containers() {
		if ct == nil {
			continue
		}
		guestType := firstNonEmptyString(ct.ContainerType(), "lxc")
		info := alerts.GuestLookup{
			ResourceID: makeGuestID(ct.Instance(), ct.Node(), ct.VMID()),
			Name:       ct.Name(),
			Instance:   ct.Instance(),
			Node:       ct.Node(),
			Type:       guestType,
			VMID:       ct.VMID(),
			Tags:       ct.Tags(),
		}
		key := alerts.BuildGuestKey(ct.Instance(), ct.Node(), ct.VMID())
		if _, exists := byKey[key]; !exists {
			byKey[key] = info
		}

		vmidKey := strconv.Itoa(ct.VMID())
		byVMID[vmidKey] = append(byVMID[vmidKey], info)

		// Persist last-known name and type for this guest
		if metadataStore != nil && ct.Name() != "" {
			persistGuestIdentity(metadataStore, key, ct.Name(), guestType)
		}
	}

	// Augment byVMID with persisted metadata for deleted guests
	if metadataStore != nil {
		enrichWithPersistedMetadata(metadataStore, byVMID)
	}

	return byKey, byVMID
}

func populateGuestNodeMapFromReadState(readState unifiedresources.ReadState, instanceName string, guestNodeMap map[int]string) {
	if readState == nil {
		return
	}
	for _, vm := range readState.VMs() {
		if vm == nil || vm.Instance() != instanceName {
			continue
		}
		guestNodeMap[vm.VMID()] = vm.Node()
	}
	for _, ct := range readState.Containers() {
		if ct == nil || ct.Instance() != instanceName {
			continue
		}
		guestNodeMap[ct.VMID()] = ct.Node()
	}
}

// enrichWithPersistedMetadata adds entries from the metadata store for guests
// that no longer exist in the live inventory but have persisted identity data
func enrichWithPersistedMetadata(metadataStore *config.GuestMetadataStore, byVMID map[string][]alerts.GuestLookup) {
	allMetadata := metadataStore.GetAll()
	for guestKey, meta := range allMetadata {
		if meta.LastKnownName == "" {
			continue // No name persisted, skip
		}

		// Parse the guest key (format: instance:node:vmid)
		// We need to extract instance, node, and vmid
		var instance, node string
		parts := strings.Split(guestKey, ":")
		if len(parts) != 3 {
			continue
		}
		instance, node = parts[0], parts[1]
		vmid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		vmidKey := strconv.Itoa(vmid)

		// Check if we already have a live entry for this exact guest
		hasLiveEntry := false
		for _, existing := range byVMID[vmidKey] {
			if existing.Instance == instance && existing.Node == node && existing.VMID == vmid {
				hasLiveEntry = true
				break
			}
		}

		// Only add persisted metadata if no live entry exists
		if !hasLiveEntry {
			byVMID[vmidKey] = append(byVMID[vmidKey], alerts.GuestLookup{
				Name:     meta.LastKnownName,
				Instance: instance,
				Node:     node,
				Type:     meta.LastKnownType,
				VMID:     vmid,
			})
		}
	}
}

// persistGuestIdentity updates the metadata store with the last-known name and type for a guest
func persistGuestIdentity(metadataStore *config.GuestMetadataStore, guestKey, name, guestType string) {
	existing := metadataStore.Get(guestKey)
	if existing == nil {
		existing = &config.GuestMetadata{
			ID:   guestKey,
			Tags: []string{},
		}
	}

	guestType = strings.TrimSpace(guestType)
	if guestType == "" {
		return
	}

	// Never "downgrade" OCI containers back to LXC. OCI classification can be transiently
	// unavailable if Proxmox config reads fail due to permissions or transient API errors.
	if existing.LastKnownType == "oci" && guestType != "oci" {
		guestType = existing.LastKnownType
	}

	// Only update if the name or type has changed
	if existing.LastKnownName != name || existing.LastKnownType != guestType {
		existing.LastKnownName = name
		existing.LastKnownType = guestType
		// Save without blocking the monitor. The store owns the goroutine so
		// Monitor.Stop can drain it; a detached goroutine here could write
		// after shutdown, into a data directory that was already being removed.
		metadataStore.SetAsync(guestKey, existing)
	}
}

func (m *Monitor) calculateBackupOperationTimeout(instanceName string) time.Duration {
	const (
		minTimeout      = 2 * time.Minute
		maxTimeout      = 5 * time.Minute
		timeoutPerGuest = 2 * time.Second
	)

	timeout := minTimeout
	readState := m.backupReadStateForInstance(instanceName)

	guestCount := 0
	for _, vm := range readState.VMs() {
		if vm != nil && vm.Instance() == instanceName && !vm.Template() {
			guestCount++
		}
	}
	for _, ct := range readState.Containers() {
		if ct != nil && ct.Instance() == instanceName && !ct.Template() {
			guestCount++
		}
	}

	if guestCount > 0 {
		dynamic := time.Duration(guestCount) * timeoutPerGuest
		if dynamic > timeout {
			timeout = dynamic
		}
	}

	if timeout > maxTimeout {
		return maxTimeout
	}

	return timeout
}

// pollPVEBackupsAndSnapshots runs the two backup-inventory scans under a shared
// bounded budget, then polls guest snapshots on the parent context so they get
// their own independent budget. A slow storage/backup scan can no longer starve
// snapshot discovery by exhausting the shared timeout before snapshots run.
func (m *Monitor) pollPVEBackupsAndSnapshots(parentCtx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string, timeout time.Duration) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	backupCtx, cancel := context.WithTimeout(parentCtx, timeout)

	// Poll backup tasks
	m.pollBackupTasks(backupCtx, instanceName, client)

	// Poll storage backups - pass nodes to avoid duplicate API calls
	m.pollStorageBackupsWithNodes(backupCtx, instanceName, client, nodes, nodeEffectiveStatus)

	backupErr := backupCtx.Err()
	cancel()

	if backupErr != nil && parentCtx.Err() == nil {
		log.Warn().
			Str("instance", instanceName).
			Err(backupErr).
			Msg("Backup storage polling budget was exhausted before guest snapshot polling; continuing snapshots with their own bounded poll budget")
	}

	// Snapshots are independent backup inventory. Passing parentCtx (no deadline)
	// lets pollGuestSnapshots establish its own bounded budget instead of
	// inheriting an already-exhausted backup deadline and skipping entirely.
	m.pollGuestSnapshots(parentCtx, instanceName, client)
}

// pollGuestSnapshots polls snapshots for all VMs and containers
func (m *Monitor) pollGuestSnapshots(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("polling guest snapshots")

	readState := m.backupReadStateForInstance(instanceName)
	var vms []models.VM
	for _, vm := range readState.VMs() {
		if vm == nil || vm.Instance() != instanceName {
			continue
		}
		vms = append(vms, vmFromReadStateView(vm))
	}
	var containers []models.Container
	for _, ct := range readState.Containers() {
		if ct == nil || ct.Instance() != instanceName {
			continue
		}
		containers = append(containers, containerFromReadStateView(ct))
	}

	previousSnapshots := make([]models.GuestSnapshot, 0)
	if m.state != nil {
		snapshot := m.state.GetSnapshot()
		for _, snap := range snapshot.PVEBackups.GuestSnapshots {
			if snap.Instance == instanceName {
				previousSnapshots = append(previousSnapshots, snap)
			}
		}
	}

	guestKey := func(instance, node string, vmid int) string {
		if instance == node {
			return fmt.Sprintf("%s-%d", node, vmid)
		}
		return fmt.Sprintf("%s-%s-%d", instance, node, vmid)
	}

	guestLookups := make(map[string]alerts.GuestLookup, len(vms)+len(containers))
	for _, vm := range vms {
		key := alerts.BuildGuestKey(vm.Instance, vm.Node, vm.VMID)
		guestLookups[key] = alerts.GuestLookup{
			ResourceID: key,
			Name:       vm.Name,
			Instance:   vm.Instance,
			Node:       vm.Node,
			Type:       "qemu",
			VMID:       vm.VMID,
			Tags:       append([]string(nil), vm.Tags...),
		}
	}
	for _, ct := range containers {
		key := alerts.BuildGuestKey(ct.Instance, ct.Node, ct.VMID)
		guestLookups[key] = alerts.GuestLookup{
			ResourceID: key,
			Name:       ct.Name,
			Instance:   ct.Instance,
			Node:       ct.Node,
			Type:       firstNonEmptyString(ct.Type, "lxc"),
			VMID:       ct.VMID,
			Tags:       append([]string(nil), ct.Tags...),
		}
	}

	activeGuests := 0
	for _, vm := range vms {
		if !vm.Template {
			activeGuests++
		}
	}
	for _, ct := range containers {
		if !ct.Template {
			activeGuests++
		}
	}

	const (
		minSnapshotTimeout              = 60 * time.Second
		maxSnapshotTimeout              = 4 * time.Minute
		snapshotTimeoutPerGuest         = 2 * time.Second
		maxConcurrentGuestSnapshotPolls = 8
	)

	timeout := minSnapshotTimeout
	if activeGuests > 0 {
		dynamic := time.Duration(activeGuests) * snapshotTimeoutPerGuest
		if dynamic > timeout {
			timeout = dynamic
		}
	}
	if timeout > maxSnapshotTimeout {
		timeout = maxSnapshotTimeout
	}

	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			log.Warn().
				Str("instance", instanceName).
				Msg("Skipping guest snapshot polling; backup context deadline exceeded")
			return
		}
		if timeout > remaining {
			timeout = remaining
		}
	}

	snapshotCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Debug().
		Str("instance", instanceName).
		Int("guestCount", activeGuests).
		Dur("timeout", timeout).
		Msg("Guest snapshot polling budget established")

	var allSnapshots []models.GuestSnapshot
	deadlineExceeded := false
	polledGuestKeys := make(map[string]struct{})

	type guestSnapshotPollTarget struct {
		key       string
		node      string
		guestType string
		vmid      int
		vmState   bool
	}

	targets := make([]guestSnapshotPollTarget, 0, activeGuests)

	for _, vm := range vms {
		if vm.Template {
			continue
		}
		targets = append(targets, guestSnapshotPollTarget{
			key:       guestKey(instanceName, vm.Node, vm.VMID),
			node:      vm.Node,
			guestType: "qemu",
			vmid:      vm.VMID,
			vmState:   true,
		})
	}

	for _, ct := range containers {
		if ct.Template {
			continue
		}
		targets = append(targets, guestSnapshotPollTarget{
			key:       guestKey(instanceName, ct.Node, ct.VMID),
			node:      ct.Node,
			guestType: "lxc",
			vmid:      ct.VMID,
			vmState:   false,
		})
	}

	type guestSnapshotPollResult struct {
		target    guestSnapshotPollTarget
		snapshots []models.GuestSnapshot
		err       error
		polled    bool
	}

	buildSnapshot := func(target guestSnapshotPollTarget, snap proxmox.Snapshot) models.GuestSnapshot {
		return models.GuestSnapshot{
			ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, target.node, target.vmid, snap.Name),
			Name:        snap.Name,
			Node:        target.node,
			Instance:    instanceName,
			Type:        target.guestType,
			VMID:        target.vmid,
			Time:        time.Unix(snap.SnapTime, 0),
			Description: snap.Description,
			Parent:      snap.Parent,
			VMState:     target.vmState,
		}
	}

	fetchSnapshots := func(target guestSnapshotPollTarget) ([]proxmox.Snapshot, error) {
		if target.guestType == "lxc" {
			return client.GetContainerSnapshots(snapshotCtx, target.node, target.vmid)
		}
		return client.GetVMSnapshots(snapshotCtx, target.node, target.vmid)
	}

	results := make(chan guestSnapshotPollResult, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentGuestSnapshotPolls)

	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-snapshotCtx.Done():
				results <- guestSnapshotPollResult{target: target, err: snapshotCtx.Err()}
				return
			}
			defer func() { <-sem }()

			snapshots, err := fetchSnapshots(target)
			result := guestSnapshotPollResult{target: target, err: err}
			if err == nil {
				result.polled = true
				result.snapshots = make([]models.GuestSnapshot, 0, len(snapshots))
				for _, snap := range snapshots {
					result.snapshots = append(result.snapshots, buildSnapshot(target, snap))
				}
			} else if target.guestType == "lxc" {
				errStr := err.Error()
				if strings.Contains(errStr, "596") || strings.Contains(errStr, "not available") {
					result.polled = true
				}
			}

			results <- result
		}()
	}

	wg.Wait()
	close(results)

	for result := range results {
		if result.polled {
			polledGuestKeys[result.target.key] = struct{}{}
			allSnapshots = append(allSnapshots, result.snapshots...)
			continue
		}

		if result.err == nil {
			continue
		}

		if snapshotCtx.Err() != nil {
			deadlineExceeded = true
			log.Warn().
				Str("instance", instanceName).
				Str("node", result.target.node).
				Str("type", result.target.guestType).
				Int("vmid", result.target.vmid).
				Err(snapshotCtx.Err()).
				Msg("Guest snapshot polling context expired before all guests completed")
			continue
		}

		if result.target.guestType == "lxc" {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_container_snapshots", instanceName, result.err).WithNode(result.target.node)
			log.Debug().
				Err(monErr).
				Str("node", result.target.node).
				Int("vmid", result.target.vmid).
				Msg("Failed to get container snapshots")
		} else {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vm_snapshots", instanceName, result.err).WithNode(result.target.node)
			log.Debug().
				Err(monErr).
				Str("node", result.target.node).
				Int("vmid", result.target.vmid).
				Msg("Failed to get VM snapshots")
		}
	}

	carriedForward := 0
	for _, prev := range previousSnapshots {
		if _, polled := polledGuestKeys[guestKey(instanceName, prev.Node, prev.VMID)]; polled {
			continue
		}
		allSnapshots = append(allSnapshots, prev)
		carriedForward++
	}

	if deadlineExceeded {
		log.Warn().
			Str("instance", instanceName).
			Int("freshlyPolled", len(polledGuestKeys)).
			Int("carriedForward", carriedForward).
			Msg("Guest snapshot polling timed out before completion; merged fresh results with previously-known snapshots for unpolled guests")
	} else if carriedForward > 0 {
		log.Debug().
			Str("instance", instanceName).
			Int("carriedForward", carriedForward).
			Msg("Guest snapshot polling completed; carried forward previous snapshots for guests with per-call errors")
	}

	if len(allSnapshots) > 0 && !deadlineExceeded {
		sizeMap := m.collectSnapshotSizes(snapshotCtx, instanceName, client, allSnapshots)
		if len(sizeMap) > 0 {
			for i := range allSnapshots {
				if size, ok := sizeMap[allSnapshots[i].ID]; ok && size > 0 {
					allSnapshots[i].SizeBytes = size
				}
			}
		}
	}

	// Update state with guest snapshots for this instance
	m.state.UpdateGuestSnapshotsForInstance(instanceName, allSnapshots)

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	guestInfo := buildProxmoxGuestInfoIndex(readState)
	points, evidenceErr := proxmoxrecoverymapper.FromPVEGuestSnapshotsWithEvidence(
		allSnapshots,
		guestInfo,
		time.Now().UTC(),
	)
	if evidenceErr != nil {
		log.Warn().
			Err(evidenceErr).
			Str("instance", instanceName).
			Msg("Failed to attach PVE snapshot recovery evidence; preserving existing points")
	} else {
		m.ingestAndReconcileRecoveryPointsAsync(
			points,
			recoveryReconcileScope{
				provider: string(recovery.ProviderProxmoxPVE),
				idPrefix: "pve-snapshot:",
				instance: instanceName,
			},
		)
	}

	if m.alertManager != nil {
		m.alertManager.CheckSnapshotsForInstance(instanceName, allSnapshots, guestLookups)
	}

	log.Debug().
		Str("instance", instanceName).
		Int("count", len(allSnapshots)).
		Msg("Guest snapshots polled")

	// Immediately broadcast the updated state so frontend sees new snapshots
	m.broadcastStateUpdate()
}

func (m *Monitor) collectSnapshotSizes(ctx context.Context, instanceName string, client PVEClientInterface, snapshots []models.GuestSnapshot) map[string]int64 {
	sizes := make(map[string]int64, len(snapshots))
	if len(snapshots) == 0 {
		return sizes
	}

	validSnapshots := make(map[string]struct{}, len(snapshots))
	nodes := make(map[string]struct{})

	for _, snap := range snapshots {
		validSnapshots[snap.ID] = struct{}{}
		if snap.Node != "" {
			nodes[snap.Node] = struct{}{}
		}
	}

	if len(nodes) == 0 {
		return sizes
	}

	seenVolids := make(map[string]struct{})

	for nodeName := range nodes {
		if ctx.Err() != nil {
			break
		}

		storages, err := client.GetStorage(ctx, nodeName)
		if err != nil {
			log.Debug().
				Err(err).
				Str("node", nodeName).
				Str("instance", instanceName).
				Msg("Failed to get storage list for snapshot sizing")
			continue
		}

		for _, storage := range storages {
			if ctx.Err() != nil {
				break
			}

			contentTypes := strings.ToLower(storage.Content)
			if !strings.Contains(contentTypes, "images") && !strings.Contains(contentTypes, "rootdir") {
				continue
			}
			if !storageContentQueryable(storage) {
				continue
			}

			contents, err := client.GetStorageContent(ctx, nodeName, storage.Storage)
			if err != nil {
				log.Debug().
					Err(err).
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("instance", instanceName).
					Msg("Failed to get storage content for snapshot sizing")
				continue
			}

			for _, item := range contents {
				if item.VMID <= 0 {
					continue
				}

				if _, seen := seenVolids[item.Volid]; seen {
					continue
				}

				snapName := extractSnapshotName(item.Volid)
				if snapName == "" {
					continue
				}

				key := fmt.Sprintf("%s-%s-%d-%s", instanceName, nodeName, item.VMID, snapName)
				if _, ok := validSnapshots[key]; !ok {
					continue
				}

				seenVolids[item.Volid] = struct{}{}

				size := int64(item.Size)
				if size < 0 {
					size = 0
				}

				sizes[key] += size
			}
		}
	}

	return sizes
}

func (m *Monitor) recordAuthFailure(instanceName string, nodeType string) {
	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	m.mu.Lock()
	m.authFailures[nodeID]++
	failures := m.authFailures[nodeID]
	m.lastAuthAttempt[nodeID] = time.Now()
	m.mu.Unlock()

	log.Warn().
		Str("node", nodeID).
		Int("failures", failures).
		Msg("Authentication failure recorded")

	const maxAuthFailures = 5
	if failures >= maxAuthFailures {
		// Clear tracking first, then perform removal outside the monitor lock.
		// Removal updates state/health and may need to acquire monitor locks internally.
		m.mu.Lock()
		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
		m.mu.Unlock()

		log.Error().
			Str("node", nodeID).
			Int("failures", failures).
			Msg("Maximum authentication failures reached, removing node from state")

		// Remove from state based on type
		if nodeType == "pve" {
			m.removeFailedPVENode(instanceName)
		} else if nodeType == "pbs" {
			m.removeFailedPBSNode(instanceName)
		} else if nodeType == "pmg" {
			m.removeFailedPMGInstance(instanceName)
		}
	}
}

// resetAuthFailures resets the failure count for a node after successful auth
func (m *Monitor) resetAuthFailures(instanceName string, nodeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	if count, exists := m.authFailures[nodeID]; exists && count > 0 {
		log.Info().
			Str("node", nodeID).
			Int("previousFailures", count).
			Msg("Authentication succeeded, resetting failure count")

		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
	}
}

// removeFailedPVENode updates a PVE node to show failed authentication status
func (m *Monitor) removeFailedPVENode(instanceName string) {
	// Get instance config to get host URL
	var hostURL string
	m.mu.RLock()
	if m.config != nil {
		for _, cfg := range m.config.PVEInstances {
			if cfg.Name == instanceName {
				hostURL = cfg.Host
				break
			}
		}
	}
	m.mu.RUnlock()

	// Create a failed node entry to show in UI with error status
	failedNode := models.Node{
		ID:               instanceName + "-failed",
		Name:             instanceName,
		DisplayName:      instanceName,
		Instance:         instanceName,
		Host:             hostURL, // Include host URL even for failed nodes
		Status:           "offline",
		Type:             "node",
		ConnectionHealth: "error",
		LastSeen:         time.Now(),
		// Set other fields to zero values to indicate no data
		CPU:    0,
		Memory: models.Memory{},
		Disk:   models.Disk{},
	}

	// Update with just the failed node
	m.state.UpdateNodesForInstance(instanceName, []models.Node{failedNode})

	// Remove all other resources associated with this instance
	m.state.UpdateVMsForInstance(instanceName, []models.VM{})
	m.state.UpdateContainersForInstance(instanceName, []models.Container{})
	m.state.UpdateStorageForInstance(instanceName, []models.Storage{})
	m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
	m.state.UpdateBackupTasksForInstance(instanceName, []models.BackupTask{})
	m.state.UpdateStorageBackupsForInstance(instanceName, []models.StorageBackup{})
	m.state.UpdatePBSGuestConfirmationsForInstance(instanceName, nil)
	m.state.UpdateGuestSnapshotsForInstance(instanceName, []models.GuestSnapshot{})

	// Set connection health to false
	m.setProviderConnectionHealth(InstanceTypePVE, instanceName, false)
}

// removeFailedPBSNode removes a PBS node and all its resources from state
func (m *Monitor) removeFailedPBSNode(instanceName string) {
	// Remove PBS instance by passing empty array
	currentInstances := m.state.PBSInstances
	var updatedInstances []models.PBSInstance
	for _, inst := range currentInstances {
		if inst.Name != instanceName {
			updatedInstances = append(updatedInstances, inst)
		}
	}
	m.state.UpdatePBSInstances(updatedInstances)

	// Remove PBS backups
	m.state.UpdatePBSBackups(instanceName, []models.PBSBackup{})

	// Set connection health to false
	m.setProviderConnectionHealth(InstanceTypePBS, instanceName, false)
}

// removeFailedPMGInstance removes PMG data from state when authentication fails repeatedly
func (m *Monitor) removeFailedPMGInstance(instanceName string) {
	currentInstances := m.state.PMGInstances
	updated := make([]models.PMGInstance, 0, len(currentInstances))
	for _, inst := range currentInstances {
		if inst.Name != instanceName {
			updated = append(updated, inst)
		}
	}

	m.state.UpdatePMGInstances(updated)
	m.state.UpdatePMGBackups(instanceName, nil)
	m.setProviderConnectionHealth(InstanceTypePMG, instanceName, false)
}

// pbsBackupCacheTTL controls how long cached PBS backup snapshots are reused
// before forcing a re-fetch. This ensures verification status changes (which
// don't alter backup count or timestamp) are picked up periodically.
const pbsBackupCacheTTL = 10 * time.Minute

type pbsBackupGroupKey struct {
	datastore  string
	namespace  string
	backupType string
	backupID   string
}

type cachedPBSGroup struct {
	snapshots []models.PBSBackup
	latest    time.Time
}

type pbsBackupFetchRequest struct {
	datastore string
	namespace string
	group     pbs.BackupGroup
	cached    cachedPBSGroup
}

// pollPBSBackups fetches all backups from PBS datastores
func (m *Monitor) pollPBSBackups(ctx context.Context, instanceName string, client *pbs.Client, datastores []models.PBSDatastore) {
	log.Debug().Str("instance", instanceName).Msg("polling PBS backups")

	// Cache existing PBS backups so we can avoid redundant API calls when no changes occurred.
	existingGroups := m.buildPBSBackupCache(instanceName)

	var allBackups []models.PBSBackup
	retainedGroups := make(map[pbsBackupGroupKey]struct{}, len(existingGroups))
	datastoreCount := len(datastores) // Number of datastores to query
	datastoreFetches := 0             // Number of successful datastore fetches
	datastoreErrors := 0              // Number of failed datastore fetches
	datastoreTerminalFailures := 0    // Number of datastores that failed only with terminal errors

	// Process each datastore
	for _, ds := range datastores {
		if ctx.Err() != nil {
			log.Warn().
				Str("instance", instanceName).
				Msg("PBS backup polling cancelled before completion")
			return
		}

		namespacePaths := namespacePathsForDatastore(ds)

		log.Info().
			Str("instance", instanceName).
			Str("datastore", ds.Name).
			Int("namespaces", len(namespacePaths)).
			Strs("namespace_paths", namespacePaths).
			Msg("Processing datastore namespaces")

		datastoreHadSuccess := false
		datastoreNamespaceErrors := 0
		datastoreTerminalNamespaceErrors := 0
		groupsReused := 0
		groupsRequested := 0

		for _, namespace := range namespacePaths {
			if ctx.Err() != nil {
				log.Warn().
					Str("instance", instanceName).
					Msg("PBS backup polling cancelled mid-datastore")
				return
			}

			groups, err := client.ListBackupGroups(ctx, ds.Name, namespace)
			if err != nil {
				datastoreNamespaceErrors++
				if !shouldReuseCachedPBSBackups(err) {
					datastoreTerminalNamespaceErrors++
				}
				log.Error().
					Err(err).
					Str("instance", instanceName).
					Str("datastore", ds.Name).
					Str("namespace", namespace).
					Msg("Failed to list PBS backup groups")
				continue
			}

			datastoreHadSuccess = true
			sortPBSBackupGroupsByLatest(groups)
			requests := make([]pbsBackupFetchRequest, 0, len(groups))
			projectedBackups := len(allBackups)

			for _, group := range groups {
				if projectedBackups >= pbsBackupLiveStateLimit {
					log.Warn().
						Str("instance", instanceName).
						Str("datastore", ds.Name).
						Str("namespace", namespace).
						Int("limit", pbsBackupLiveStateLimit).
						Msg("PBS backup live-state limit reached; skipping remaining groups")
					break
				}

				key := pbsBackupGroupKey{
					datastore:  ds.Name,
					namespace:  namespace,
					backupType: group.BackupType,
					backupID:   group.BackupID,
				}
				cached := existingGroups[key]

				// Group deleted (no backups left) - ensure cached data is dropped.
				if group.BackupCount == 0 {
					continue
				}
				retainedGroups[key] = struct{}{}

				lastBackupTime := time.Unix(group.LastBackup, 0)
				hasCachedData := len(cached.snapshots) > 0
				cacheCountMatches := len(cached.snapshots) == retainedPBSSnapshotCount(group.BackupCount)

				// Check if the cached data is still within its TTL.
				cacheAge := time.Since(m.pbsBackupCacheTimeFor(instanceName, key))
				cacheStillFresh := cacheAge < pbsBackupCacheTTL

				// A cached in-flight snapshot must be re-fetched every poll:
				// when it completes, neither the group's last-backup time nor
				// its count changes (the listing already included it), so the
				// normal reuse conditions would keep showing "backup running"
				// until the TTL expired.
				cachedHasInProgress := false
				for _, cachedSnapshot := range cached.snapshots {
					if cachedSnapshot.InProgress {
						cachedHasInProgress = true
						break
					}
				}

				// Only re-fetch when the backup count changes, the most recent backup
				// is newer, or the cache TTL has expired (to pick up verification changes).
				if hasCachedData &&
					cacheStillFresh &&
					cacheCountMatches &&
					!cachedHasInProgress &&
					!lastBackupTime.After(cached.latest) {

					allBackups = appendPBSBackupsWithinLimit(allBackups, cached.snapshots)
					groupsReused++
					projectedBackups = len(allBackups)
					continue
				}

				requests = append(requests, pbsBackupFetchRequest{
					datastore: ds.Name,
					namespace: namespace,
					group:     group,
					cached:    cached,
				})
				projectedBackups += retainedPBSSnapshotCount(group.BackupCount)
			}

			if len(requests) == 0 {
				continue
			}

			groupsRequested += len(requests)
			fetched := m.fetchPBSBackupSnapshots(ctx, client, instanceName, requests)
			if len(fetched) > 0 {
				allBackups = appendPBSBackupsWithinLimit(allBackups, fetched)
			}

			// Record fetch time for each requested group so the TTL tracks freshness.
			// We record for all requested groups — on fetch failure, fetchPBSBackupSnapshots
			// falls back to cached data, so the timestamp prevents hammering a failing
			// endpoint. The TTL ensures we retry within a bounded window.
			fetchedAt := time.Now()
			for _, req := range requests {
				reqKey := pbsBackupGroupKey{
					datastore:  req.datastore,
					namespace:  req.namespace,
					backupType: req.group.BackupType,
					backupID:   req.group.BackupID,
				}
				m.setPBSBackupCacheTime(instanceName, reqKey, fetchedAt)
			}
		}

		if datastoreHadSuccess {
			datastoreFetches++
			log.Info().
				Str("instance", instanceName).
				Str("datastore", ds.Name).
				Int("namespaces", len(namespacePaths)).
				Int("groups_reused", groupsReused).
				Int("groups_refreshed", groupsRequested).
				Msg("PBS datastore processed")
		} else {
			allNamespaceErrorsTerminal := datastoreNamespaceErrors > 0 &&
				datastoreTerminalNamespaceErrors == datastoreNamespaceErrors
			if allNamespaceErrorsTerminal {
				datastoreTerminalFailures++
				log.Warn().
					Str("instance", instanceName).
					Str("datastore", ds.Name).
					Int("namespace_errors", datastoreNamespaceErrors).
					Msg("No namespaces succeeded for PBS datastore due to terminal errors; clearing cached backups")
			} else {
				// Preserve cached data for this datastore when failures are transient.
				log.Warn().
					Str("instance", instanceName).
					Str("datastore", ds.Name).
					Msg("No namespaces succeeded for PBS datastore; using cached backups")
				for key, entry := range existingGroups {
					if key.datastore != ds.Name || len(entry.snapshots) == 0 {
						continue
					}
					allBackups = appendPBSBackupsWithinLimit(allBackups, entry.snapshots)
					retainedGroups[key] = struct{}{}
				}
			}
			datastoreErrors++
		}
	}

	log.Info().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("PBS backups fetched")

	protectionObservedAt := time.Now().UTC()
	protectionObservation, protectionObservationErr :=
		buildPBSProtectionProviderObservation(
			instanceName,
			datastoreCount,
			datastoreFetches,
			datastoreErrors,
			datastoreTerminalFailures,
			protectionObservedAt,
		)
	if protectionObservationErr != nil {
		log.Warn().
			Err(protectionObservationErr).
			Str("instance", instanceName).
			Msg("Failed to build PBS protection provider observation")
	}

	// Decide whether to keep existing backups when all queries failed
	if shouldPreservePBSBackupsWithTerminal(datastoreCount, datastoreFetches, datastoreTerminalFailures) {
		if protectionObservationErr == nil {
			m.ingestProtectionProviderObservationsAsync(
				[]recovery.ProtectionProviderObservation{protectionObservation},
			)
		}
		log.Warn().
			Str("instance", instanceName).
			Int("datastores", datastoreCount).
			Int("errors", datastoreErrors).
			Int("terminal_failures", datastoreTerminalFailures).
			Msg("All PBS datastore queries failed; keeping previous backup list")
		return
	}

	// A manifestless snapshot is incomplete, but it is not necessarily still
	// being written: failed backup and PBS-to-PBS sync tasks can leave that
	// shape behind. Correlate incomplete artifacts with PBS's live task view
	// before they feed guest "Backup Running" state. If task visibility is
	// denied or transiently unavailable, keep the previous conservative
	// inference rather than claiming that no write is active.
	if hasIncompletePBSBackup(allBackups) {
		// Write-activity evidence belongs to this poll, not to the snapshot
		// cache. A failed snapshot refresh may have reused an incomplete
		// artifact whose previous poll authoritatively found no writer. Clear
		// that old observation before asking for the current task view so a
		// task-query failure leaves visibility unknown instead of publishing a
		// stale terminal-failure conclusion.
		clearPBSWriteActivityObservation(allBackups)
		runningDataTasks, runningTasksErr := client.ListRunningDataTasks(ctx)
		if runningTasksErr != nil {
			log.Warn().Err(runningTasksErr).
				Str("instance", instanceName).
				Msg("Could not correlate incomplete PBS snapshots with running tasks")
		} else {
			applyPBSWriteActivity(allBackups, runningDataTasks)
		}
	}

	m.prunePBSBackupCacheTimes(instanceName, retainedGroups)

	// Update state
	m.state.UpdatePBSBackups(instanceName, allBackups)

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	candidates := buildPBSGuestCandidates(m.GetUnifiedReadStateOrSnapshot())
	points, evidenceErr := proxmoxrecoverymapper.FromPBSBackupsWithEvidence(
		allBackups,
		candidates,
		protectionObservedAt,
	)
	if evidenceErr != nil {
		log.Warn().
			Err(evidenceErr).
			Str("instance", instanceName).
			Msg("Failed to attach PBS recovery evidence; preserving existing points")
		if protectionObservationErr == nil {
			m.ingestProtectionProviderObservationsAsync(
				[]recovery.ProtectionProviderObservation{protectionObservation},
			)
		}
		return
	}
	observations := []recovery.ProtectionProviderObservation{}
	if protectionObservationErr == nil {
		observations = append(observations, protectionObservation)
	}
	m.ingestAndReconcileRecoveryPointsWithObservationsAsync(
		points,
		observations,
		recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPBS),
			idPrefix: "pbs-backup:",
			instance: instanceName,
		},
	)

	// Sync backup times to VMs/Containers and republish them to canonical resources.
	m.syncGuestBackupTimesAndResourceStore()

	if m.alertManager != nil {
		guestsByKey, guestsByVMID := buildGuestLookupsFromReadState(m.GetUnifiedReadStateOrSnapshot(), m.guestMetadataStore)
		rollups, err := m.listBackupRollupsForAlerts(context.Background())
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list recovery rollups for backup alerts")
		} else {
			m.alertManager.CheckBackupsWithInventory(rollups, guestsByKey, guestsByVMID, m.backupInventoryScopeForAlerts())
		}
	}

	// Immediately broadcast the updated state so frontend sees new backups
	m.broadcastStateUpdate()
}

func hasIncompletePBSBackup(backups []models.PBSBackup) bool {
	for _, backup := range backups {
		if backup.InProgress {
			return true
		}
	}
	return false
}

func clearPBSWriteActivityObservation(backups []models.PBSBackup) {
	for i := range backups {
		backups[i].WriteActivityObserved = false
		backups[i].WriteActive = false
	}
}

// applyPBSWriteActivity marks whether each incomplete snapshot is accounted
// for by a task which PBS currently reports as running. Backup task worker IDs
// normally identify "<store>:<type>/<id>". If a server version returns a less
// specific backup ID, the task remains a conservative instance-wide writer.
// Sync tasks can touch many groups and therefore apply instance-wide.
func applyPBSWriteActivity(backups []models.PBSBackup, tasks []pbs.RunningDataTask) {
	for i := range backups {
		if !backups[i].InProgress {
			continue
		}
		backups[i].WriteActivityObserved = true
		backups[i].WriteActive = runningPBSTaskMatchesBackup(tasks, backups[i])
	}
}

func runningPBSTaskMatchesBackup(tasks []pbs.RunningDataTask, backup models.PBSBackup) bool {
	subject := strings.ToLower(strings.TrimSpace(backup.BackupType + "/" + backup.VMID))
	store := strings.ToLower(strings.TrimSpace(backup.Datastore))
	for _, task := range tasks {
		switch strings.ToLower(strings.TrimSpace(task.WorkerType)) {
		case "syncjob":
			return true
		case "backup":
			workerID := strings.ToLower(strings.TrimSpace(task.WorkerID))
			if workerID == "" {
				return true
			}
			if !strings.Contains(workerID, "/") {
				// Unknown legacy/new worker-ID shape: do not hide a real write.
				return true
			}
			if !strings.HasSuffix(workerID, subject) {
				continue
			}
			if store == "" || strings.HasPrefix(workerID, store+":") || strings.Contains(workerID, ":"+store+":") {
				return true
			}
		}
	}
	return false
}

func (m *Monitor) buildPBSBackupCache(instanceName string) map[pbsBackupGroupKey]cachedPBSGroup {
	snapshot := m.state.GetSnapshot()
	cache := make(map[pbsBackupGroupKey]cachedPBSGroup)
	for _, backup := range snapshot.PBSBackups {
		if backup.Instance != instanceName {
			continue
		}
		key := pbsBackupGroupKey{
			datastore:  backup.Datastore,
			namespace:  normalizePBSNamespacePath(backup.Namespace),
			backupType: backup.BackupType,
			backupID:   backup.VMID,
		}
		entry := cache[key]
		entry.snapshots = append(entry.snapshots, backup)
		if backup.BackupTime.After(entry.latest) {
			entry.latest = backup.BackupTime
		}
		cache[key] = entry
	}
	for key, entry := range cache {
		sortPBSBackupsByLatest(entry.snapshots)
		if len(entry.snapshots) > pbsBackupSnapshotsPerGroupLimit {
			entry.snapshots = entry.snapshots[:pbsBackupSnapshotsPerGroupLimit]
		}
		cache[key] = entry
	}
	return cache
}

// pbsBackupCacheTimeFor returns the last fetch time for a PBS backup group.
func (m *Monitor) pbsBackupCacheTimeFor(instanceName string, key pbsBackupGroupKey) time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if perGroup, ok := m.pbsBackupCacheTime[instanceName]; ok {
		return perGroup[key]
	}
	return time.Time{}
}

// setPBSBackupCacheTime records when a PBS backup group was last fetched.
func (m *Monitor) setPBSBackupCacheTime(instanceName string, key pbsBackupGroupKey, t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pbsBackupCacheTime == nil {
		m.pbsBackupCacheTime = make(map[string]map[pbsBackupGroupKey]time.Time)
	}
	if m.pbsBackupCacheTime[instanceName] == nil {
		m.pbsBackupCacheTime[instanceName] = make(map[pbsBackupGroupKey]time.Time)
	}
	m.pbsBackupCacheTime[instanceName][key] = t
}

func (m *Monitor) prunePBSBackupCacheTimes(instanceName string, retained map[pbsBackupGroupKey]struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pbsBackupCacheTime == nil {
		return
	}
	perGroup := m.pbsBackupCacheTime[instanceName]
	if len(perGroup) == 0 {
		return
	}

	for key := range perGroup {
		if _, ok := retained[key]; !ok {
			delete(perGroup, key)
		}
	}
	if len(perGroup) == 0 {
		delete(m.pbsBackupCacheTime, instanceName)
	}
}

func normalizePBSNamespacePath(ns string) string {
	if ns == "/" {
		return ""
	}
	return ns
}

func namespacePathsForDatastore(ds models.PBSDatastore) []string {
	if len(ds.Namespaces) == 0 {
		return []string{""}
	}

	seen := make(map[string]struct{}, len(ds.Namespaces))
	var paths []string
	for _, ns := range ds.Namespaces {
		path := normalizePBSNamespacePath(ns.Path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func (m *Monitor) fetchPBSBackupSnapshots(ctx context.Context, client *pbs.Client, instanceName string, requests []pbsBackupFetchRequest) []models.PBSBackup {
	if len(requests) == 0 {
		return nil
	}

	workerCount := pbsBackupSnapshotFetchWorkers
	if len(requests) < workerCount {
		workerCount = len(requests)
	}

	jobs := make(chan pbsBackupFetchRequest)
	results := make(chan []models.PBSBackup, workerCount)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for req := range jobs {
				log.Debug().
					Str("instance", instanceName).
					Str("datastore", req.datastore).
					Str("namespace", req.namespace).
					Str("type", req.group.BackupType).
					Str("id", req.group.BackupID).
					Msg("Refreshing PBS backup group")

				snapshots, err := client.ListBackupSnapshots(ctx, req.datastore, req.namespace, req.group.BackupType, req.group.BackupID)
				if err != nil {
					log.Error().
						Err(err).
						Str("instance", instanceName).
						Str("datastore", req.datastore).
						Str("namespace", req.namespace).
						Str("type", req.group.BackupType).
						Str("id", req.group.BackupID).
						Msg("Failed to list PBS backup snapshots")

					if len(req.cached.snapshots) > 0 {
						select {
						case results <- req.cached.snapshots:
						case <-ctx.Done():
						}
					}
					continue
				}

				backups := convertPBSSnapshots(instanceName, req.datastore, req.namespace, snapshots)
				select {
				case results <- backups:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, req := range requests {
			select {
			case jobs <- req:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var combined []models.PBSBackup
	for backups := range results {
		if len(backups) == 0 {
			continue
		}
		combined = appendPBSBackupsWithinLimit(combined, backups)
	}

	return combined
}

func convertPBSSnapshots(instanceName, datastore, namespace string, snapshots []pbs.BackupSnapshot) []models.PBSBackup {
	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].BackupTime > snapshots[j].BackupTime
	})
	if len(snapshots) > pbsBackupSnapshotsPerGroupLimit {
		snapshots = snapshots[:pbsBackupSnapshotsPerGroupLimit]
	}

	backups := make([]models.PBSBackup, 0, len(snapshots))
	for _, snapshot := range snapshots {
		backupTime := time.Unix(snapshot.BackupTime, 0)
		backupID := fmt.Sprintf("pbs-%s-%s-%s-%s-%s-%d",
			instanceName, datastore, namespace,
			snapshot.BackupType, snapshot.BackupID,
			snapshot.BackupTime)

		var fileNames []string
		for _, file := range snapshot.Files {
			switch f := file.(type) {
			case string:
				fileNames = append(fileNames, f)
			case map[string]interface{}:
				if filename, ok := f["filename"].(string); ok {
					fileNames = append(fileNames, filename)
				}
			}
		}

		// A snapshot the PBS API lists while the backup is still being
		// written has no manifest yet: no size field (decoded as 0) and no
		// index.json.blob in files - typically only the guest config blob.
		// Treat it as in-progress, not as a completed backup.
		hasManifest := false
		for _, name := range fileNames {
			if name == "index.json.blob" {
				hasManifest = true
				break
			}
		}
		inProgress := snapshot.Size == 0 && !hasManifest

		verified := false
		if snapshot.Verification != nil {
			switch v := snapshot.Verification.(type) {
			case string:
				verified = v == "ok"
			case map[string]interface{}:
				if state, ok := v["state"].(string); ok {
					verified = state == "ok"
				}
			}

			log.Debug().
				Str("vmid", snapshot.BackupID).
				Int64("time", snapshot.BackupTime).
				Interface("verification", snapshot.Verification).
				Bool("verified", verified).
				Msg("PBS backup verification status")
		}

		backups = append(backups, models.PBSBackup{
			ID:              backupID,
			Instance:        instanceName,
			Datastore:       datastore,
			Namespace:       namespace,
			BackupType:      snapshot.BackupType,
			VMID:            snapshot.BackupID,
			BackupTime:      backupTime,
			Size:            snapshot.Size,
			Protected:       snapshot.Protected,
			Verified:        verified,
			InProgress:      inProgress,
			VerificationRaw: snapshot.Verification,
			Comment:         snapshot.Comment,
			Files:           fileNames,
			Owner:           snapshot.Owner,
		})
	}

	return backups
}

func sortPBSBackupGroupsByLatest(groups []pbs.BackupGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].LastBackup == groups[j].LastBackup {
			if groups[i].BackupType == groups[j].BackupType {
				return groups[i].BackupID < groups[j].BackupID
			}
			return groups[i].BackupType < groups[j].BackupType
		}
		return groups[i].LastBackup > groups[j].LastBackup
	})
}

func sortPBSBackupsByLatest(backups []models.PBSBackup) {
	sort.SliceStable(backups, func(i, j int) bool {
		if backups[i].BackupTime.Equal(backups[j].BackupTime) {
			return backups[i].ID < backups[j].ID
		}
		return backups[i].BackupTime.After(backups[j].BackupTime)
	})
}

// retainedPBSSnapshotCount is how many snapshots poll retention keeps for a
// group with the given backup count: the full group, capped at the per-group
// limit. It doubles as the expected cache size when deciding whether cached
// snapshots for a group are still complete.
func retainedPBSSnapshotCount(backupCount int) int {
	if backupCount <= 0 {
		return 0
	}
	if backupCount > pbsBackupSnapshotsPerGroupLimit {
		return pbsBackupSnapshotsPerGroupLimit
	}
	return backupCount
}

func appendPBSBackupsWithinLimit(backups []models.PBSBackup, additions []models.PBSBackup) []models.PBSBackup {
	if len(backups) >= pbsBackupLiveStateLimit || len(additions) == 0 {
		return backups
	}
	remaining := pbsBackupLiveStateLimit - len(backups)
	if len(additions) > remaining {
		additions = additions[:remaining]
	}
	return append(backups, additions...)
}

// pollBackupTasks polls backup tasks from a PVE instance
func (m *Monitor) pollBackupTasks(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("polling backup tasks")

	tasks, err := client.GetBackupTasks(ctx)
	if err != nil {
		monErr := errors.WrapAPIError("get_backup_tasks", instanceName, err, 0)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("failed to get backup tasks")
		return
	}

	var backupTasks []models.BackupTask
	var jobTasks []models.BackupTask
	jobUPIDs := make(map[string]string)
	for _, task := range tasks {
		// Extract VMID from task ID (format: "UPID:node:pid:starttime:type:vmid:user@realm:")
		vmid := 0
		if task.ID != "" {
			if vmidInt, err := strconv.Atoi(task.ID); err == nil {
				vmid = vmidInt
			}
		}

		taskID := fmt.Sprintf("%s-%s", instanceName, task.UPID)

		backupTask := models.BackupTask{
			ID:         taskID,
			Node:       task.Node,
			Instance:   instanceName,
			Type:       task.Type,
			VMID:       vmid,
			Status:     task.Status,
			StartTime:  time.Unix(task.StartTime, 0),
			ObservedAt: time.Now().UTC(),
		}

		if task.EndTime > 0 {
			backupTask.EndTime = time.Unix(task.EndTime, 0)
		}

		backupTasks = append(backupTasks, backupTask)

		// Scheduled multi-guest backup jobs run under a single UPID with an
		// empty VMID slot, so the task alone says nothing about which guests
		// it covered (#1452 area, regressed with the guest-centric redesign).
		if vmid == 0 && task.Type == "vzdump" && task.UPID != "" {
			jobUPIDs[backupTask.ID] = task.UPID
			jobTasks = append(jobTasks, backupTask)
		}
	}

	backupTasks = append(backupTasks, m.synthesizeVzdumpJobGuestTasks(ctx, instanceName, client, jobTasks, jobUPIDs)...)

	// Update state with new backup tasks for this instance
	m.state.UpdateBackupTasksForInstance(instanceName, backupTasks)

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	guestInfo := buildProxmoxGuestInfoIndex(m.backupReadStateForInstance(instanceName))
	points, evidenceErr := proxmoxrecoverymapper.FromPVEBackupTasksWithEvidence(
		backupTasks,
		guestInfo,
		time.Now().UTC(),
	)
	if evidenceErr != nil {
		log.Warn().
			Err(evidenceErr).
			Str("instance", instanceName).
			Msg("Failed to attach PVE backup-task recovery evidence; preserving existing points")
		return
	}
	m.ingestRecoveryPointsAsync(points)
}

// vzdumpJobTaskCacheEntry caches the per-guest tasks synthesized from one
// vzdump job run's log so finished jobs are fetched at most once.
type vzdumpJobTaskCacheEntry struct {
	tasks []models.BackupTask
	final bool
}

// maxVzdumpJobLogFetchesPerPoll bounds task-log requests per poll cycle so a
// large backlog of historical job runs (e.g. right after startup) trickles in
// over a few cycles instead of stalling the shared backup-poll budget.
const maxVzdumpJobLogFetchesPerPoll = 25

// synthesizeVzdumpJobGuestTasks expands multi-guest vzdump job runs into one
// synthetic per-guest task each by parsing the job's task log. Results are
// cached by instance|UPID; logs of finished jobs are immutable, so those are
// fetched only once. Running jobs are re-parsed every cycle to pick up guests
// as the job progresses.
func (m *Monitor) synthesizeVzdumpJobGuestTasks(ctx context.Context, instanceName string, client PVEClientInterface, jobTasks []models.BackupTask, jobUPIDs map[string]string) []models.BackupTask {
	if len(jobTasks) == 0 {
		m.pruneVzdumpJobTaskCache(instanceName, nil)
		return nil
	}

	now := time.Now().UTC()
	seen := make(map[string]struct{}, len(jobTasks))
	var synthesized []models.BackupTask
	fetches := 0

	for _, jobTask := range jobTasks {
		upid := jobUPIDs[jobTask.ID]
		key := instanceName + "|" + upid
		seen[key] = struct{}{}

		m.mu.RLock()
		entry, cached := m.vzdumpJobTaskCache[key]
		m.mu.RUnlock()

		if cached && entry.final {
			synthesized = append(synthesized, refreshObservedAt(entry.tasks, now)...)
			continue
		}

		if fetches >= maxVzdumpJobLogFetchesPerPoll || ctx.Err() != nil {
			// Serve a stale non-final entry if we have one; the next cycle
			// will catch up.
			if cached {
				synthesized = append(synthesized, refreshObservedAt(entry.tasks, now)...)
			}
			continue
		}
		fetches++

		logLines, err := client.GetTaskLog(ctx, jobTask.Node, upid)
		if err != nil {
			log.Debug().Err(err).
				Str("instance", instanceName).
				Str("upid", upid).
				Msg("failed to fetch vzdump job task log; per-guest status unavailable this cycle")
			if cached {
				synthesized = append(synthesized, refreshObservedAt(entry.tasks, now)...)
			}
			continue
		}

		guestTasks := parseVzdumpJobLog(jobTask, upid, logLines)
		m.mu.Lock()
		if m.vzdumpJobTaskCache == nil {
			m.vzdumpJobTaskCache = make(map[string]vzdumpJobTaskCacheEntry)
		}
		m.vzdumpJobTaskCache[key] = vzdumpJobTaskCacheEntry{
			tasks: guestTasks,
			final: !jobTask.EndTime.IsZero(),
		}
		m.mu.Unlock()
		synthesized = append(synthesized, refreshObservedAt(guestTasks, now)...)
	}

	m.pruneVzdumpJobTaskCache(instanceName, seen)
	return synthesized
}

// pruneVzdumpJobTaskCache drops cached job entries for this instance whose
// UPIDs are no longer present in the task list (fell out of the window).
func (m *Monitor) pruneVzdumpJobTaskCache(instanceName string, seen map[string]struct{}) {
	prefix := instanceName + "|"
	m.mu.Lock()
	for key := range m.vzdumpJobTaskCache {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if _, ok := seen[key]; !ok {
			delete(m.vzdumpJobTaskCache, key)
		}
	}
	m.mu.Unlock()
}

func refreshObservedAt(tasks []models.BackupTask, now time.Time) []models.BackupTask {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]models.BackupTask, len(tasks))
	copy(out, tasks)
	for i := range out {
		out[i].ObservedAt = now
	}
	return out
}

// vzdump job logs mark each guest's section with these lines.
var (
	vzdumpLogGuestStartRe  = regexp.MustCompile(`^INFO: Starting Backup of VM (\d+)`)
	vzdumpLogGuestFinishRe = regexp.MustCompile(`^INFO: Finished Backup of VM (\d+) \((\d+):(\d{2}):(\d{2})\)$`)
	vzdumpLogGuestFailRe   = regexp.MustCompile(`^ERROR: Backup of VM (\d+) failed -\s*(.*)$`)
)

// parseVzdumpJobLog turns a multi-guest vzdump job run into synthetic
// per-guest BackupTask entries. Log lines carry no timestamps, so per-guest
// times are reconstructed from the job start plus the durations vzdump prints
// on each "Finished Backup" line. Task IDs embed the parent UPID, keeping them
// stable across polls and distinct from individually-run guest backups.
func parseVzdumpJobLog(jobTask models.BackupTask, upid string, logLines []proxmox.TaskLogLine) []models.BackupTask {
	type guestSection struct {
		vmid   int
		start  time.Time
		end    time.Time
		status string
		errMsg string
	}

	var order []int
	sections := make(map[int]*guestSection)
	cursor := jobTask.StartTime

	ensureSection := func(vmid int) *guestSection {
		if sec, ok := sections[vmid]; ok {
			return sec
		}
		sec := &guestSection{vmid: vmid, start: cursor, status: "running"}
		sections[vmid] = sec
		order = append(order, vmid)
		return sec
	}

	for _, line := range logLines {
		text := strings.TrimSpace(line.Text)
		if match := vzdumpLogGuestStartRe.FindStringSubmatch(text); match != nil {
			vmid, err := strconv.Atoi(match[1])
			if err != nil || vmid <= 0 {
				continue
			}
			sec := ensureSection(vmid)
			sec.start = cursor
			sec.status = "running"
		} else if match := vzdumpLogGuestFinishRe.FindStringSubmatch(text); match != nil {
			vmid, err := strconv.Atoi(match[1])
			if err != nil || vmid <= 0 {
				continue
			}
			sec := ensureSection(vmid)
			if sec.status == "running" {
				sec.status = "OK"
			}
			hours, _ := strconv.Atoi(match[2])
			minutes, _ := strconv.Atoi(match[3])
			seconds, _ := strconv.Atoi(match[4])
			duration := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
			sec.end = sec.start.Add(duration)
			cursor = sec.end
		} else if match := vzdumpLogGuestFailRe.FindStringSubmatch(text); match != nil {
			vmid, err := strconv.Atoi(match[1])
			if err != nil || vmid <= 0 {
				continue
			}
			sec := ensureSection(vmid)
			sec.status = "error"
			sec.errMsg = strings.TrimSpace(match[2])
			if sec.errMsg == "" {
				sec.errMsg = "backup failed"
			}
		}
	}

	if len(order) == 0 {
		return nil
	}

	jobFinished := !jobTask.EndTime.IsZero()
	tasks := make([]models.BackupTask, 0, len(order))
	for _, vmid := range order {
		sec := sections[vmid]
		if jobFinished {
			if sec.status == "running" {
				// The job ended without a per-guest verdict; treat as failed
				// so the guest doesn't look covered by a backup that never
				// completed.
				sec.status = "error"
				sec.errMsg = "backup job ended before guest backup completed"
			}
			if sec.end.IsZero() {
				sec.end = jobTask.EndTime
			}
		}

		tasks = append(tasks, models.BackupTask{
			ID:        fmt.Sprintf("%s-%s-vm%d", jobTask.Instance, upid, vmid),
			Node:      jobTask.Node,
			Instance:  jobTask.Instance,
			Type:      jobTask.Type,
			VMID:      vmid,
			Status:    sec.status,
			Error:     sec.errMsg,
			StartTime: sec.start,
			EndTime:   sec.end,
		})
	}

	return tasks
}

func (m *Monitor) pollPVEBackupsAsync(
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
	nodeEffectiveStatus map[string]string,
) {
	// Poll backups if enabled - respect configured interval or cycle gating
	if !instanceCfg.MonitorBackups {
		return
	}

	if !m.backupPollingEnabledSetting() {
		log.Debug().
			Str("instance", instanceName).
			Msg("Skipping backup polling - globally disabled")
		return
	}

	now := time.Now()

	m.mu.RLock()
	lastPoll := m.lastPVEBackupPoll[instanceName]
	m.mu.RUnlock()

	shouldPoll, reason, newLast := m.shouldRunBackupPoll(lastPoll, now)
	if !shouldPoll {
		if reason != "" {
			log.Debug().
				Str("instance", instanceName).
				Str("reason", reason).
				Msg("Skipping PVE backup polling this cycle")
		}
		return
	}

	// Set initial timestamp before starting goroutine (prevents concurrent starts).
	// This optional work owns a runtime-scoped timeout below; the completed core
	// PVE generation must not be reclassified as unreachable merely because its
	// per-cycle context expired before background scheduling.
	m.mu.Lock()
	m.lastPVEBackupPoll[instanceName] = newLast
	m.mu.Unlock()

	// Run backup polling in a separate goroutine to avoid blocking real-time stats
	go func(startTime time.Time, inst string, pveClient PVEClientInterface) {
		defer recoverFromPanic(fmt.Sprintf("pollPVEBackups-%s", inst))
		timeout := m.calculateBackupOperationTimeout(inst)
		log.Info().
			Str("instance", inst).
			Dur("timeout", timeout).
			Msg("Starting background backup/snapshot polling")

		// The per-cycle ctx is canceled as soon as the main polling loop finishes,
		// so derive the backup poll context from the long-lived runtime context instead.
		parentCtx := m.getRuntimeContext()
		if parentCtx == nil {
			parentCtx = context.Background()
		}

		m.pollPVEBackupsAndSnapshots(parentCtx, inst, pveClient, nodes, nodeEffectiveStatus, timeout)

		duration := time.Since(startTime)
		log.Info().
			Str("instance", inst).
			Dur("duration", duration).
			Msg("Completed background backup/snapshot polling")

		// Update timestamp after completion for accurate interval scheduling
		m.mu.Lock()
		m.lastPVEBackupPoll[inst] = time.Now()
		m.mu.Unlock()
	}(now, instanceName, client)

}

// checkMockAlerts checks alerts for mock data
