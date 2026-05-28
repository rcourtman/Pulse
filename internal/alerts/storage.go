package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// CheckStorage checks storage against thresholds
func (m *Manager) CheckStorage(storage models.Storage) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	resourceIDs := storageAlertResourceIDs(storage)
	if m.config.DisableAllStorage {
		m.mu.RUnlock()
		// Clear any existing storage alerts when all storage alerts are disabled
		m.mu.Lock()
		for _, resourceID := range resourceIDs {
			usageAlertID := canonicalMetricStateID(resourceID, "usage")
			if m.clearActiveAlertIfPresentNoLock(usageAlertID) {
				log.Info().
					Str("alertID", usageAlertID).
					Str("storage", storage.Name).
					Msg("Cleared usage alert - all storage alerts disabled")
			}
			offlineAlertID := canonicalConnectivityStateID(resourceID)
			if m.clearActiveAlertIfPresentNoLock(offlineAlertID) {
				log.Info().
					Str("alertID", offlineAlertID).
					Str("storage", storage.Name).
					Msg("Cleared offline alert - all storage alerts disabled")
			}
		}
		m.mu.Unlock()
		return
	}

	thresholds := m.resolveStorageThresholdsNoLock(storage)
	m.mu.RUnlock()

	if thresholds.Disabled {
		m.mu.Lock()
		for _, resourceID := range resourceIDs {
			delete(m.offlineConfirmations, resourceID)
		}
		m.mu.Unlock()
		for _, resourceID := range resourceIDs {
			m.clearAlert(canonicalMetricStateID(resourceID, "usage"))
			m.clearAlert(canonicalConnectivityStateID(resourceID))
		}
		return
	}

	m.clearStorageAliasAlerts(storage)

	// Check if storage is truly offline/unavailable (not just inactive from other nodes)
	// Note: In a cluster, local storage from other nodes shows as inactive which is normal
	if thresholds.DisableConnectivity {
		m.mu.Lock()
		for _, resourceID := range resourceIDs {
			delete(m.offlineConfirmations, resourceID)
		}
		m.mu.Unlock()
		for _, resourceID := range resourceIDs {
			m.clearAlert(canonicalConnectivityStateID(resourceID))
		}
	} else if storage.Status == "offline" || storage.Status == "unavailable" {
		m.checkStorageOffline(storage)
	} else {
		// Clear any existing offline alert if storage is back online
		m.clearStorageOfflineAlert(storage)
	}

	// Check usage if storage has valid data (even if not currently active on this node)
	// In clusters, storage may show as inactive on nodes where it's not currently mounted
	// but we still want to alert on high usage
	log.Debug().
		Str("storage", storage.Name).
		Str("id", storage.ID).
		Float64("usage", storage.Usage).
		Str("status", storage.Status).
		Float64("trigger", thresholds.Usage.Trigger).
		Float64("clear", thresholds.Usage.Clear).
		Msg("Checking storage thresholds")

	// Check usage if storage is online - checkMetric will skip if threshold is nil or <= 0
	if storage.Status != "offline" && storage.Status != "unavailable" && storage.Usage > 0 {
		m.evaluateUnifiedMetrics(&UnifiedResourceInput{
			ID:       storage.ID,
			Type:     "storage",
			Name:     storage.Name,
			Node:     storage.Node,
			Instance: storage.Instance,
			Disk:     &UnifiedResourceMetric{Percent: storage.Usage},
		}, thresholds, nil)
	}

	// Check ZFS pool status if this is ZFS storage
	if storage.ZFSPool != nil {
		m.checkZFSPoolHealth(storage)
	}
}

func (m *Manager) clearStorageAliasAlerts(storage models.Storage) {
	for i, resourceID := range storageAlertResourceIDs(storage) {
		if i == 0 {
			continue
		}
		m.clearAlert(canonicalMetricStateID(resourceID, "usage"))
		m.clearAlert(canonicalConnectivityStateID(resourceID))
	}
}

// checkZFSPoolHealth checks ZFS pool for errors and degraded state
func (m *Manager) checkZFSPoolHealth(storage models.Storage) {
	pool := storage.ZFSPool
	if pool == nil {
		return
	}

	poolResourceName := fmt.Sprintf("%s (%s)", storage.Name, pool.Name)
	poolAssessment := storagehealth.AssessZFSPool(*pool)

	// Check pool state (DEGRADED, FAULTED, etc.)
	stateAlertID := fmt.Sprintf("zfs-pool-state-%s", storage.ID)
	stateMetadata := map[string]interface{}{
		"pool_name":  pool.Name,
		"pool_state": pool.State,
	}
	stateReasons := filterStorageHealthReasonsByCodes(poolAssessment.Reasons, zfsPoolAssessmentCodes)
	stateResult, _ := m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
		SpecID:         storage.ID + "/zfs-pool:" + sanitizeHostComponent(pool.Name) + "-state",
		Signal:         "zfs-pool",
		Codes:          zfsPoolAssessmentCodes,
		Reasons:        stateReasons,
		AlertID:        stateAlertID,
		AlertType:      "zfs-pool-state",
		SpecResourceID: storage.ID + "/zfs-pool:" + sanitizeHostComponent(pool.Name),
		ResourceID:     storage.ID,
		ResourceName:   poolResourceName,
		ResourceType:   unifiedresources.ResourceTypeStorage,
		Node:           storage.Node,
		Instance:       storage.Instance,
		Metadata:       stateMetadata,
	})
	if stateResult.Transition != nil && stateResult.Transition.Kind == alertspecs.EvaluationTransitionActivated {
		log.Warn().
			Str("pool", pool.Name).
			Str("state", pool.State).
			Str("node", storage.Node).
			Msg("ZFS pool is not healthy")
	}

	// Check for read/write/checksum errors
	totalErrors := pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors
	errorsAlertID := fmt.Sprintf("zfs-pool-errors-%s", storage.ID)
	errorsSpecResourceID := storage.ID + "/zfs-pool:" + sanitizeHostComponent(pool.Name)
	errorsSpecID := errorsSpecResourceID + "-errors"
	if totalErrors > 0 {
		existingValue, exists := m.activeAlertValue(buildCanonicalStateID(errorsSpecResourceID, errorsSpecID))
		if !exists || float64(totalErrors) > existingValue {
			errorMetadata := map[string]interface{}{
				"pool_name":       pool.Name,
				"read_errors":     pool.ReadErrors,
				"write_errors":    pool.WriteErrors,
				"checksum_errors": pool.ChecksumErrors,
			}
			errorReasons := filterStorageHealthReasonsByCodes(poolAssessment.Reasons, zfsPoolErrorAssessmentCodes)
			result, _ := m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
				SpecID:         errorsSpecID,
				Signal:         "zfs-pool-errors",
				Codes:          zfsPoolErrorAssessmentCodes,
				Reasons:        errorReasons,
				AlertID:        errorsAlertID,
				AlertType:      "zfs-pool-errors",
				SpecResourceID: errorsSpecResourceID,
				ResourceID:     storage.ID,
				ResourceName:   poolResourceName,
				ResourceType:   unifiedresources.ResourceTypeStorage,
				Node:           storage.Node,
				Instance:       storage.Instance,
				Metadata:       errorMetadata,
				MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
					return fmt.Sprintf(
						"ZFS pool '%s' has errors: %d read, %d write, %d checksum",
						pool.Name, pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors,
					), float64(totalErrors), 0
				},
			})
			if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
				log.Error().
					Str("pool", pool.Name).
					Int64("read_errors", pool.ReadErrors).
					Int64("write_errors", pool.WriteErrors).
					Int64("checksum_errors", pool.ChecksumErrors).
					Str("node", storage.Node).
					Msg("ZFS pool has I/O errors")
			}
		}
	} else {
		m.clearAlert(buildCanonicalStateID(errorsSpecResourceID, errorsSpecID))
	}

	// Check individual devices for errors
	for _, device := range pool.Devices {
		alertID := fmt.Sprintf("zfs-device-%s-%s", storage.ID, device.Name)
		deviceAssessment := zfsDeviceAssessment(device)
		metadata := map[string]interface{}{
			"pool_name":       pool.Name,
			"device_name":     device.Name,
			"device_state":    device.State,
			"read_errors":     device.ReadErrors,
			"write_errors":    device.WriteErrors,
			"checksum_errors": device.ChecksumErrors,
		}
		result, _ := m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
			SpecID:         storage.ID + "/zfs-pool:" + sanitizeHostComponent(pool.Name) + "/device:" + sanitizeHostComponent(device.Name) + "-health",
			Signal:         "zfs-device",
			Codes:          zfsDeviceAssessmentCodes,
			Reasons:        deviceAssessment.Reasons,
			AlertID:        alertID,
			AlertType:      "zfs-device",
			SpecResourceID: storage.ID + "/zfs-pool:" + sanitizeHostComponent(pool.Name) + "/device:" + sanitizeHostComponent(device.Name),
			ResourceID:     storage.ID,
			ResourceName:   formatZFSDeviceResourceName(storage.Name, pool.Name, device.Name),
			ResourceType:   unifiedresources.ResourceTypeStorage,
			Node:           storage.Node,
			Instance:       storage.Instance,
			Metadata:       metadata,
			MessageBuilder: func(result alertspecs.EvaluationResult) (string, float64, float64) {
				return strings.Join(storageHealthReasonSummaries(deviceAssessment.Reasons), "; "), float64(device.ReadErrors + device.WriteErrors + device.ChecksumErrors), 0
			},
		})
		if result.Transition != nil && result.Transition.Kind == alertspecs.EvaluationTransitionActivated {
			log.Warn().
				Str("pool", pool.Name).
				Str("device", device.Name).
				Str("state", device.State).
				Int64("errors", device.ReadErrors+device.WriteErrors+device.ChecksumErrors).
				Str("node", storage.Node).
				Msg("ZFS device has issues")
		}
	}
}

func formatZFSDeviceResourceName(storageName, poolName, deviceName string) string {
	storageName = strings.TrimSpace(storageName)
	if storageName == "" {
		storageName = "storage"
	}

	parts := make([]string, 0, 2)
	if poolName = strings.TrimSpace(poolName); poolName != "" {
		parts = append(parts, poolName)
	}
	if deviceName = strings.TrimSpace(deviceName); deviceName != "" {
		parts = append(parts, deviceName)
	}
	if len(parts) == 0 {
		return storageName
	}
	return fmt.Sprintf("%s (%s)", storageName, strings.Join(parts, ", "))
}

// checkStorageOffline creates an alert for offline/unavailable storage
func (m *Manager) checkStorageOffline(storage models.Storage) {
	alertID := fmt.Sprintf("storage-offline-%s", storage.ID)

	m.mu.Lock()
	delete(m.offlineRecoveryConfirmations, canonicalConnectivityStateID(storage.ID))
	m.mu.Unlock()

	m.mu.RLock()
	thresholds := m.resolveStorageThresholdsNoLock(storage)
	m.mu.RUnlock()
	spec, err := buildCanonicalConnectivitySpec(storage.ID, storage.Name, unifiedresources.ResourceTypeStorage, AlertLevelWarning, 2, thresholds.Disabled || thresholds.DisableConnectivity)
	if err != nil {
		log.Warn().
			Err(err).
			Str("storage", storage.Name).
			Str("storageID", storage.ID).
			Msg("Skipping invalid canonical storage connectivity spec")
		return
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec:         spec,
		Evidence:     alertspecs.AlertEvidence{ObservedAt: time.Now(), Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false}},
		Tracking:     m.offlineConfirmations,
		TrackingKey:  storage.ID,
		AlertID:      alertID,
		AlertType:    "offline",
		ResourceID:   storage.ID,
		ResourceName: storage.Name,
		Node:         storage.Node,
		Instance:     storage.Instance,
		Message:      fmt.Sprintf("Storage %s on node %s is unavailable", storage.Name, storage.Node),
		Metadata: map[string]interface{}{
			"resourceType": "storage",
			"status":       storage.Status,
		},
		RateLimit:     true,
		DispatchAsync: true,
	})
}

// clearStorageOfflineAlert removes offline alert when storage comes back online
func (m *Manager) clearStorageOfflineAlert(storage models.Storage) {
	m.clearResourceOfflineAlert(storage.ID, storage.Name, storage.Node, "Storage", offlineRecoveryConfirmationsStorage)
}
