package monitoring

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	proxmoxrecoverymapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) pollStorageBackupsWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {

	var allBackups []models.StorageBackup
	hasPBSDirectConnection := m.config != nil && len(m.config.PBSInstances) > 0
	seenVolids := make(map[string]bool) // Track seen volume IDs to avoid duplicates
	hadSuccessfulNode := false          // Track if at least one node responded successfully
	storagesWithBackup := 0             // Number of storages that should contain backups
	contentSuccess := 0                 // Number of successful storage content fetches
	contentFailures := 0                // Number of failed storage content fetches
	storageQueryErrors := 0             // Number of nodes where storage list could not be queried
	hadPermissionError := false         // Track if any permission errors occurred this cycle
	storagePreserveNeeded := map[string]struct{}{}
	storageSuccess := map[string]struct{}{}

	// Build guest lookup map to find actual node for each VMID
	snapshot := m.state.GetSnapshot()
	guestNodeMap := make(map[int]string) // VMID -> actual node name
	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName {
			guestNodeMap[vm.VMID] = vm.Node
		}
	}
	for _, ct := range snapshot.Containers {
		if ct.Instance == instanceName {
			guestNodeMap[ct.VMID] = ct.Node
		}
	}

	// For each node, get storage and check content
	for _, node := range nodes {
		if nodeEffectiveStatus[node.Node] != "online" {
			for _, storageName := range storageNamesForNode(instanceName, node.Node, snapshot) {
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
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_for_backups", instanceName, err).WithNode(node.Node)
			log.Warn().Err(monErr).Str("node", node.Node).Msg("failed to get storage for backups - skipping node")
			for _, storageName := range storageNamesForNode(instanceName, node.Node, snapshot) {
				storagePreserveNeeded[storageName] = struct{}{}
			}
			storageQueryErrors++
			continue
		}

		hadSuccessfulNode = true

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
				errStr := strings.ToLower(err.Error())

				// Check if this is a permission error
				if strings.Contains(errStr, "403") || strings.Contains(errStr, "401") ||
					strings.Contains(errStr, "permission") || strings.Contains(errStr, "forbidden") {
					hadPermissionError = true
					m.mu.Lock()
					m.backupPermissionWarnings[instanceName] = "Missing PVEDatastoreAdmin permission on /storage. Run: pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin"
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
				isPBSStorage := strings.HasPrefix(storage.Storage, "pbs-") || storage.Type == "pbs"
				if isPBSStorage && hasPBSDirectConnection {
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

	// Decide whether to keep existing backups when every query failed
	if shouldPreserveBackups(len(nodes), hadSuccessfulNode, storagesWithBackup, contentSuccess) {
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

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	guestInfo := buildProxmoxGuestInfoIndex(snapshot)
	m.ingestRecoveryPointsAsync(proxmoxrecoverymapper.FromPVEStorageBackups(allBackups, guestInfo))

	// Sync backup times to VMs/Containers for backup status indicators
	m.state.SyncGuestBackupTimes()

	if m.alertManager != nil {
		snapshot := m.state.GetSnapshot()
		guestsByKey, guestsByVMID := buildGuestLookups(snapshot, m.guestMetadataStore)
		rollups, err := m.listBackupRollupsForAlerts(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list recovery rollups for backup alerts")
		} else {
			m.alertManager.CheckBackups(rollups, guestsByKey, guestsByVMID)
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

func shouldPreserveBackups(nodeCount int, hadSuccessfulNode bool, storagesWithBackup, contentSuccess int) bool {
	if nodeCount > 0 && !hadSuccessfulNode {
		return true
	}
	if storagesWithBackup > 0 && contentSuccess == 0 {
		return true
	}
	return false
}

func shouldPreservePBSBackups(datastoreCount, datastoreFetches int) bool {
	// If there are datastores but all fetches failed, preserve existing backups
	if datastoreCount > 0 && datastoreFetches == 0 {
		return true
	}
	return false
}

func storageNamesForNode(instanceName, nodeName string, snapshot models.StateSnapshot) []string {
	if nodeName == "" {
		return nil
	}

	var storages []string
	for _, storage := range snapshot.Storage {
		if storage.Instance != instanceName {
			continue
		}
		if storage.Name == "" {
			continue
		}
		if !strings.Contains(storage.Content, "backup") {
			continue
		}
		if storage.Node == nodeName {
			storages = append(storages, storage.Name)
			continue
		}
		for _, node := range storage.Nodes {
			if node == nodeName {
				storages = append(storages, storage.Name)
				break
			}
		}
	}

	return storages
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

func buildGuestLookups(snapshot models.StateSnapshot, metadataStore *config.GuestMetadataStore) (map[string]alerts.GuestLookup, map[string][]alerts.GuestLookup) {
	byKey := make(map[string]alerts.GuestLookup)
	byVMID := make(map[string][]alerts.GuestLookup)

	for _, vm := range snapshot.VMs {
		info := alerts.GuestLookup{
			ResourceID: makeGuestID(vm.Instance, vm.Node, vm.VMID),
			Name:       vm.Name,
			Instance:   vm.Instance,
			Node:       vm.Node,
			Type:       vm.Type,
			VMID:       vm.VMID,
		}
		key := alerts.BuildGuestKey(vm.Instance, vm.Node, vm.VMID)
		byKey[key] = info

		vmidKey := strconv.Itoa(vm.VMID)
		byVMID[vmidKey] = append(byVMID[vmidKey], info)

		// Persist last-known name and type for this guest
		if metadataStore != nil && vm.Name != "" {
			persistGuestIdentity(metadataStore, key, vm.Name, vm.Type)
		}
	}

	for _, ct := range snapshot.Containers {
		info := alerts.GuestLookup{
			ResourceID: makeGuestID(ct.Instance, ct.Node, ct.VMID),
			Name:       ct.Name,
			Instance:   ct.Instance,
			Node:       ct.Node,
			Type:       ct.Type,
			VMID:       ct.VMID,
		}
		key := alerts.BuildGuestKey(ct.Instance, ct.Node, ct.VMID)
		if _, exists := byKey[key]; !exists {
			byKey[key] = info
		}

		vmidKey := strconv.Itoa(ct.VMID)
		byVMID[vmidKey] = append(byVMID[vmidKey], info)

		// Persist last-known name and type for this guest
		if metadataStore != nil && ct.Name != "" {
			persistGuestIdentity(metadataStore, key, ct.Name, ct.Type)
		}
	}

	// Augment byVMID with persisted metadata for deleted guests
	if metadataStore != nil {
		enrichWithPersistedMetadata(metadataStore, byVMID)
	}

	return byKey, byVMID
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
		// Save asynchronously to avoid blocking the monitor
		go func() {
			if err := metadataStore.Set(guestKey, existing); err != nil {
				log.Error().Err(err).Str("guestKey", guestKey).Msg("failed to persist guest identity")
			}
		}()
	}
}

func (m *Monitor) calculateBackupOperationTimeout(instanceName string) time.Duration {
	const (
		minTimeout      = 2 * time.Minute
		maxTimeout      = 5 * time.Minute
		timeoutPerGuest = 2 * time.Second
	)

	timeout := minTimeout
	snapshot := m.state.GetSnapshot()

	guestCount := 0
	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName && !vm.Template {
			guestCount++
		}
	}
	for _, ct := range snapshot.Containers {
		if ct.Instance == instanceName && !ct.Template {
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

// pollGuestSnapshots polls snapshots for all VMs and containers
func (m *Monitor) pollGuestSnapshots(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("polling guest snapshots")

	// Get current VMs and containers from a properly-locked state snapshot.
	// Using GetSnapshot() ensures we read a consistent view of VMs/containers
	// with the State's internal mutex, avoiding data races.
	snapshot := m.state.GetSnapshot()
	var vms []models.VM
	for _, vm := range snapshot.VMs {
		if vm.Instance == instanceName {
			vms = append(vms, vm)
		}
	}
	var containers []models.Container
	for _, ct := range snapshot.Containers {
		if ct.Instance == instanceName {
			containers = append(containers, ct)
		}
	}

	guestKey := func(instance, node string, vmid int) string {
		if instance == node {
			return fmt.Sprintf("%s-%d", node, vmid)
		}
		return fmt.Sprintf("%s-%s-%d", instance, node, vmid)
	}

	guestNames := make(map[string]string, len(vms)+len(containers))
	for _, vm := range vms {
		guestNames[guestKey(instanceName, vm.Node, vm.VMID)] = vm.Name
	}
	for _, ct := range containers {
		guestNames[guestKey(instanceName, ct.Node, ct.VMID)] = ct.Name
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
		minSnapshotTimeout      = 60 * time.Second
		maxSnapshotTimeout      = 4 * time.Minute
		snapshotTimeoutPerGuest = 2 * time.Second
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

	// Poll VM snapshots
	for _, vm := range vms {
		// Skip templates
		if vm.Template {
			continue
		}

		snapshots, err := client.GetVMSnapshots(snapshotCtx, vm.Node, vm.VMID)
		if err != nil {
			if snapshotCtx.Err() != nil {
				log.Warn().
					Str("instance", instanceName).
					Str("node", vm.Node).
					Int("vmid", vm.VMID).
					Err(snapshotCtx.Err()).
					Msg("Aborting guest snapshot polling due to context cancellation while fetching VM snapshots")
				deadlineExceeded = true
				break
			}
			// This is common for VMs without snapshots, so use debug level
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vm_snapshots", instanceName, err).WithNode(vm.Node)
			log.Debug().
				Err(monErr).
				Str("node", vm.Node).
				Int("vmid", vm.VMID).
				Msg("Failed to get VM snapshots")
			continue
		}

		for _, snap := range snapshots {
			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, vm.Node, vm.VMID, snap.Name),
				Name:        snap.Name,
				Node:        vm.Node,
				Instance:    instanceName,
				Type:        "qemu",
				VMID:        vm.VMID,
				Time:        time.Unix(snap.SnapTime, 0),
				Description: snap.Description,
				Parent:      snap.Parent,
				VMState:     true, // VM state support enabled
			}

			allSnapshots = append(allSnapshots, snapshot)
		}
	}

	if deadlineExceeded {
		log.Warn().
			Str("instance", instanceName).
			Msg("Guest snapshot polling timed out before completing VM collection; retaining previous snapshots")
		return
	}

	// Poll container snapshots
	for _, ct := range containers {
		// Skip templates
		if ct.Template {
			continue
		}

		snapshots, err := client.GetContainerSnapshots(snapshotCtx, ct.Node, ct.VMID)
		if err != nil {
			if snapshotCtx.Err() != nil {
				log.Warn().
					Str("instance", instanceName).
					Str("node", ct.Node).
					Int("vmid", ct.VMID).
					Err(snapshotCtx.Err()).
					Msg("Aborting guest snapshot polling due to context cancellation while fetching container snapshots")
				deadlineExceeded = true
				break
			}
			// API error 596 means snapshots not supported/available - this is expected for many containers
			errStr := err.Error()
			if strings.Contains(errStr, "596") || strings.Contains(errStr, "not available") {
				// Silently skip containers without snapshot support
				continue
			}
			// Log other errors at debug level
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_container_snapshots", instanceName, err).WithNode(ct.Node)
			log.Debug().
				Err(monErr).
				Str("node", ct.Node).
				Int("vmid", ct.VMID).
				Msg("Failed to get container snapshots")
			continue
		}

		for _, snap := range snapshots {
			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, ct.Node, ct.VMID, snap.Name),
				Name:        snap.Name,
				Node:        ct.Node,
				Instance:    instanceName,
				Type:        "lxc",
				VMID:        ct.VMID,
				Time:        time.Unix(snap.SnapTime, 0),
				Description: snap.Description,
				Parent:      snap.Parent,
				VMState:     false,
			}

			allSnapshots = append(allSnapshots, snapshot)
		}
	}

	if deadlineExceeded || snapshotCtx.Err() != nil {
		log.Warn().
			Str("instance", instanceName).
			Msg("Guest snapshot polling timed out before completion; retaining previous snapshots")
		return
	}

	if len(allSnapshots) > 0 {
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
	guestInfo := buildProxmoxGuestInfoIndex(snapshot)
	m.ingestRecoveryPointsAsync(proxmoxrecoverymapper.FromPVEGuestSnapshots(allSnapshots, guestInfo))

	if m.alertManager != nil {
		m.alertManager.CheckSnapshotsForInstance(instanceName, allSnapshots, guestNames)
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
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	// Increment failure count
	m.authFailures[nodeID]++
	m.lastAuthAttempt[nodeID] = time.Now()

	log.Warn().
		Str("node", nodeID).
		Int("failures", m.authFailures[nodeID]).
		Msg("Authentication failure recorded")

	// If we've exceeded the threshold, remove the node
	const maxAuthFailures = 5
	if m.authFailures[nodeID] >= maxAuthFailures {
		log.Error().
			Str("node", nodeID).
			Int("failures", m.authFailures[nodeID]).
			Msg("Maximum authentication failures reached, removing node from state")

		// Remove from state based on type
		if nodeType == "pve" {
			m.removeFailedPVENode(instanceName)
		} else if nodeType == "pbs" {
			m.removeFailedPBSNode(instanceName)
		} else if nodeType == "pmg" {
			m.removeFailedPMGInstance(instanceName)
		}

		// Reset the counter since we've removed the node
		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
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
	for _, cfg := range m.config.PVEInstances {
		if cfg.Name == instanceName {
			hostURL = cfg.Host
			break
		}
	}

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
	m.state.UpdateGuestSnapshotsForInstance(instanceName, []models.GuestSnapshot{})

	// Set connection health to false
	m.state.SetConnectionHealth(instanceName, false)
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
	m.state.SetConnectionHealth("pbs-"+instanceName, false)
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
	m.state.SetConnectionHealth("pmg-"+instanceName, false)
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
			requests := make([]pbsBackupFetchRequest, 0, len(groups))

			for _, group := range groups {
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

				lastBackupTime := time.Unix(group.LastBackup, 0)
				hasCachedData := len(cached.snapshots) > 0

				// Check if the cached data is still within its TTL.
				cacheAge := time.Since(m.pbsBackupCacheTimeFor(instanceName, key))
				cacheStillFresh := cacheAge < pbsBackupCacheTTL

				// Only re-fetch when the backup count changes, the most recent backup
				// is newer, or the cache TTL has expired (to pick up verification changes).
				if hasCachedData &&
					cacheStillFresh &&
					len(cached.snapshots) == group.BackupCount &&
					!lastBackupTime.After(cached.latest) {

					allBackups = append(allBackups, cached.snapshots...)
					groupsReused++
					continue
				}

				requests = append(requests, pbsBackupFetchRequest{
					datastore: ds.Name,
					namespace: namespace,
					group:     group,
					cached:    cached,
				})
			}

			if len(requests) == 0 {
				continue
			}

			groupsRequested += len(requests)
			fetched := m.fetchPBSBackupSnapshots(ctx, client, instanceName, requests)
			if len(fetched) > 0 {
				allBackups = append(allBackups, fetched...)
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
					allBackups = append(allBackups, entry.snapshots...)
				}
			}
			datastoreErrors++
		}
	}

	log.Info().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("PBS backups fetched")

	// Decide whether to keep existing backups when all queries failed
	if shouldPreservePBSBackupsWithTerminal(datastoreCount, datastoreFetches, datastoreTerminalFailures) {
		log.Warn().
			Str("instance", instanceName).
			Int("datastores", datastoreCount).
			Int("errors", datastoreErrors).
			Int("terminal_failures", datastoreTerminalFailures).
			Msg("All PBS datastore queries failed; keeping previous backup list")
		return
	}

	// Update state
	m.state.UpdatePBSBackups(instanceName, allBackups)

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	snapshot := m.state.GetSnapshot()
	candidates := make(map[string][]proxmoxrecoverymapper.GuestCandidate)
	for _, vm := range snapshot.VMs {
		if vm.Template || vm.VMID <= 0 {
			continue
		}
		key := "vm:" + strconv.Itoa(vm.VMID)
		sourceID := strings.TrimSpace(vm.ID)
		if sourceID == "" {
			sourceID = makeGuestID(vm.Instance, vm.Node, vm.VMID)
		}
		candidates[key] = append(candidates[key], proxmoxrecoverymapper.GuestCandidate{
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeVM,
			DisplayName:  strings.TrimSpace(vm.Name),
			InstanceName: strings.TrimSpace(vm.Instance),
			NodeName:     strings.TrimSpace(vm.Node),
			VMID:         vm.VMID,
		})
	}
	for _, ct := range snapshot.Containers {
		if ct.Template || ct.VMID <= 0 {
			continue
		}
		key := "ct:" + strconv.Itoa(ct.VMID)
		sourceID := strings.TrimSpace(ct.ID)
		if sourceID == "" {
			sourceID = makeGuestID(ct.Instance, ct.Node, ct.VMID)
		}
		candidates[key] = append(candidates[key], proxmoxrecoverymapper.GuestCandidate{
			SourceID:     sourceID,
			ResourceType: unifiedresources.ResourceTypeLXC,
			DisplayName:  strings.TrimSpace(ct.Name),
			InstanceName: strings.TrimSpace(ct.Instance),
			NodeName:     strings.TrimSpace(ct.Node),
			VMID:         ct.VMID,
		})
	}
	m.ingestRecoveryPointsAsync(proxmoxrecoverymapper.FromPBSBackups(allBackups, candidates))

	// Sync backup times to VMs/Containers for backup status indicators
	m.state.SyncGuestBackupTimes()

	if m.alertManager != nil {
		snapshot := m.state.GetSnapshot()
		guestsByKey, guestsByVMID := buildGuestLookups(snapshot, m.guestMetadataStore)
		rollups, err := m.listBackupRollupsForAlerts(context.Background())
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list recovery rollups for backup alerts")
		} else {
			m.alertManager.CheckBackups(rollups, guestsByKey, guestsByVMID)
		}
	}

	// Immediately broadcast the updated state so frontend sees new backups
	m.broadcastStateUpdate()
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

	results := make(chan []models.PBSBackup, len(requests))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, req := range requests {
		req := req
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

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
					results <- req.cached.snapshots
				}
				return
			}

			results <- convertPBSSnapshots(instanceName, req.datastore, req.namespace, snapshots)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var combined []models.PBSBackup
	for backups := range results {
		if len(backups) == 0 {
			continue
		}
		combined = append(combined, backups...)
	}

	return combined
}

func convertPBSSnapshots(instanceName, datastore, namespace string, snapshots []pbs.BackupSnapshot) []models.PBSBackup {
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
			ID:         backupID,
			Instance:   instanceName,
			Datastore:  datastore,
			Namespace:  namespace,
			BackupType: snapshot.BackupType,
			VMID:       snapshot.BackupID,
			BackupTime: backupTime,
			Size:       snapshot.Size,
			Protected:  snapshot.Protected,
			Verified:   verified,
			Comment:    snapshot.Comment,
			Files:      fileNames,
			Owner:      snapshot.Owner,
		})
	}

	return backups
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
			ID:        taskID,
			Node:      task.Node,
			Instance:  instanceName,
			Type:      task.Type,
			VMID:      vmid,
			Status:    task.Status,
			StartTime: time.Unix(task.StartTime, 0),
		}

		if task.EndTime > 0 {
			backupTask.EndTime = time.Unix(task.EndTime, 0)
		}

		backupTasks = append(backupTasks, backupTask)
	}

	// Update state with new backup tasks for this instance
	m.state.UpdateBackupTasksForInstance(instanceName, backupTasks)

	// Best-effort ingestion into recovery store (for rollups / unified backups UX).
	snapshot := m.state.GetSnapshot()
	guestInfo := buildProxmoxGuestInfoIndex(snapshot)
	m.ingestRecoveryPointsAsync(proxmoxrecoverymapper.FromPVEBackupTasks(backupTasks, guestInfo))
}

func (m *Monitor) pollPVEBackupsAsync(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
	nodeEffectiveStatus map[string]string,
) error {
	// Poll backups if enabled - respect configured interval or cycle gating
	if !instanceCfg.MonitorBackups {
		return nil
	}

	if !m.config.EnableBackupPolling {
		log.Debug().
			Str("instance", instanceName).
			Msg("Skipping backup polling - globally disabled")
		return nil
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
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Set initial timestamp before starting goroutine (prevents concurrent starts)
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
			parentCtx := m.runtimeCtx
			if parentCtx == nil {
				parentCtx = context.Background()
			}

			backupCtx, cancel := context.WithTimeout(parentCtx, timeout)
			defer cancel()

			// Poll backup tasks
			m.pollBackupTasks(backupCtx, inst, pveClient)

			// Poll storage backups - pass nodes to avoid duplicate API calls
			m.pollStorageBackupsWithNodes(backupCtx, inst, pveClient, nodes, nodeEffectiveStatus)

			// Poll guest snapshots
			m.pollGuestSnapshots(backupCtx, inst, pveClient)

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

	return nil
}

// checkMockAlerts checks alerts for mock data
