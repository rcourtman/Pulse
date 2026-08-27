package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

func (m *Manager) saveActiveAlertsAsync(context string) {
	m.stopMu.RLock()
	if m.stopping {
		m.stopMu.RUnlock()
		return
	}
	m.workerWG.Add(1)
	m.stopMu.RUnlock()

	go func() {
		defer m.workerWG.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Str("context", context).
					Msg("Panic in SaveActiveAlerts goroutine")
			}
		}()
		if err := m.SaveActiveAlerts(); err != nil {
			log.Error().
				Err(err).
				Str("context", context).
				Msg("Failed to save active alerts")
		}
	}()
}

// SaveActiveAlerts persists active alerts to disk.
func (m *Manager) SaveActiveAlerts() error {
	// Serialize snapshots and writes so concurrent async saves cannot
	// overwrite newer state with an older snapshot.
	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	const maxCheckpointAttempts = 8
	for attempt := 0; attempt < maxCheckpointAttempts; attempt++ {
		store := m.eventLogStore()
		revision := int64(0)
		if store != nil {
			var err error
			revision, err = store.ActiveStateRevision()
			if err != nil {
				alerts := m.snapshotActiveAlerts()
				if mirrorErr := m.writeActiveAlertsRecoveryMirror(alerts); mirrorErr != nil {
					return errors.Join(err, mirrorErr)
				}
				m.markActiveStateDegraded(err)
				return fmt.Errorf("failed to read SQLite active alert revision: %w", err)
			}
		}

		alerts := m.snapshotActiveAlerts()
		mirrorErr := m.writeActiveAlertsRecoveryMirror(alerts)
		intentErr := m.saveIntentPendingSnapshot()
		if store == nil {
			if err := errors.Join(mirrorErr, intentErr); err != nil {
				return err
			}
			log.Debug().Int("count", len(alerts)).Msg("saved active alert recovery mirror")
			return nil
		}

		snapshots, err := activeStateSnapshots(alerts)
		if err != nil {
			m.markActiveStateDegraded(err)
			return err
		}
		replaced, err := store.ReplaceActiveStateIfRevision(snapshots, revision)
		if err != nil {
			m.markActiveStateDegraded(err)
			return fmt.Errorf("failed to checkpoint active alerts in SQLite: %w", err)
		}
		if !replaced {
			// A lifecycle event committed after this snapshot began. Retry from
			// the manager's current state instead of overwriting that newer event.
			continue
		}
		m.activeStateAuthoritative.Store(true)
		m.clearActiveStateDegraded()
		if err := errors.Join(mirrorErr, intentErr); err != nil {
			return fmt.Errorf("SQLite active alert checkpoint succeeded but recovery persistence failed: %w", err)
		}
		log.Debug().Int("count", len(alerts)).Msg("saved active alerts to durable state and recovery mirror")
		return nil
	}

	return fmt.Errorf("active alert checkpoint changed during %d consecutive attempts", maxCheckpointAttempts)
}

func (m *Manager) snapshotActiveAlerts() []*Alert {
	m.mu.RLock()
	alerts := make([]*Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		clone := alert.Clone()
		backfillCanonicalIdentity(clone)
		clone.ID = exportedAlertID(clone, clone.ID)
		alerts = append(alerts, clone)
	}
	m.mu.RUnlock()
	return alerts
}

func (m *Manager) writeActiveAlertsRecoveryMirror(alerts []*Alert) error {
	if m.activeRecoveryWriteBlock.Load() {
		return fmt.Errorf("active alert recovery mirror is write-blocked because its startup source was unreadable")
	}
	alertsDir := m.getAlertsDir()
	if err := os.MkdirAll(alertsDir, alertsDirPerm); err != nil {
		return fmt.Errorf("failed to create alerts directory: %w", err)
	}
	if err := os.Chmod(alertsDir, alertsDirPerm); err != nil {
		return fmt.Errorf("failed to set alerts directory permissions: %w", err)
	}

	data, err := json.Marshal(alerts)
	if err != nil {
		return fmt.Errorf("failed to marshal active alerts: %w", err)
	}

	// Write to temporary file first, then rename. Use a unique temp file so
	// periodic saves, explicit saves, and shutdown saves cannot race on a name.
	tmpFile, err := os.CreateTemp(alertsDir, "active-alerts-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	cleanupTemp := true

	defer func() {
		if !cleanupTemp {
			return
		}
		if err := os.Remove(tmpName); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("file", tmpName).Msg("Failed to remove temp active alerts file")
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		writeErr := fmt.Errorf("failed to write active alerts temp file %s: %w", tmpName, err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			closeErr = fmt.Errorf("failed to close temp file %s after write failure: %w", tmpName, closeErr)
			return fmt.Errorf("failed to persist active alerts: %w", errors.Join(writeErr, closeErr))
		}
		return writeErr
	}
	if err := tmpFile.Chmod(alertsFilePerm); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("file", tmpName).Msg("Failed to close temp file after chmod error")
		}
		return fmt.Errorf("failed to set active alerts temp file permissions: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			return fmt.Errorf("failed to sync and close active alerts temp file %s: %w", tmpName, errors.Join(err, closeErr))
		}
		return fmt.Errorf("failed to sync active alerts temp file %s: %w", tmpName, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close active alerts temp file %s: %w", tmpName, err)
	}

	finalFile := filepath.Join(alertsDir, "active-alerts.json")
	if err := replaceActiveAlertsFile(tmpName, finalFile); err != nil {
		return fmt.Errorf("failed to rename active alerts file from %s to %s: %w", tmpName, finalFile, err)
	}
	if err := os.Chmod(finalFile, alertsFilePerm); err != nil {
		return fmt.Errorf("failed to set active alerts file permissions: %w", err)
	}
	if err := syncActiveAlertsDirectory(alertsDir); err != nil {
		return err
	}
	return nil
}

// LoadActiveAlerts restores active alerts from disk.
func (m *Manager) LoadActiveAlerts() error {
	m.activeRecoveryReadable.Store(false)
	m.activeRecoveryWriteBlock.Store(false)
	alertsFile := filepath.Join(m.getAlertsDir(), "active-alerts.json")
	data, err := readLimitedRegularFile(alertsFile, maxActiveAlertsFileSizeBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			m.activeRecoveryReadable.Store(true)
			log.Info().Msg("No active alerts file found, starting fresh")
			m.mu.Lock()
			defer m.mu.Unlock()
			if err := m.loadIntentPendingNoLock(); err != nil {
				return err
			}
			m.seedReducerCoreNoLock()
			return nil
		}
		m.activeRecoveryWriteBlock.Store(true)
		return fmt.Errorf("failed to read active alerts: %w", err)
	}

	var alerts []*Alert
	if err := json.Unmarshal(data, &alerts); err != nil {
		m.activeRecoveryWriteBlock.Store(true)
		return fmt.Errorf("failed to unmarshal active alerts: %w", err)
	}
	if err := os.Chmod(alertsFile, alertsFilePerm); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("file", alertsFile).Msg("Failed to harden active alerts file permissions")
	}
	if err := m.restoreActiveAlertSnapshots(alerts, "JSON recovery mirror", false); err != nil {
		return err
	}
	m.activeRecoveryReadable.Store(true)
	return nil
}

func (m *Manager) restoreActiveAlertSnapshots(alerts []*Alert, source string, replace bool) error {
	m.mu.Lock()
	if replace {
		m.activeAlerts = make(map[string]*Alert)
		m.activeAlertAlias = make(map[string]string)
		m.ackState = make(map[string]ackRecord)
		m.ackStateByCanonical = make(map[string]ackRecord)
		m.core = reducer.NewState()
	}

	restoredCount := 0
	duplicateCount := 0
	identityMigrationCount := 0
	seen := make(map[string]bool)
	restoredCritical := make([]*Alert, 0)

	for _, alert := range alerts {
		if alert == nil {
			continue
		}

		// Migrate legacy guest alert IDs (instance-node-VMID -> instance-VMID).
		isGuestAlert := strings.Contains(alert.Type, "cpu") || strings.Contains(alert.Type, "memory") ||
			strings.Contains(alert.Type, "disk") || strings.Contains(alert.Type, "network") ||
			alert.Type == "guest-offline"
		if isGuestAlert {
			parts := strings.Split(alert.ResourceID, "-")

			if alert.Node != "" && len(parts) >= 2 {
				var newResourceID string

				vmidStr := parts[len(parts)-1]
				if _, err := strconv.Atoi(vmidStr); err == nil {
					if len(parts) == 3 && alert.Instance != "" && alert.Instance != alert.Node {
						newResourceID = fmt.Sprintf("%s-%s", alert.Instance, vmidStr)
					} else if len(parts) == 2 && alert.Instance == alert.Node {
						newResourceID = fmt.Sprintf("%s-%s", alert.Instance, vmidStr)
					}

					if newResourceID != "" && newResourceID != alert.ResourceID {
						log.Info().
							Str("oldID", alert.ResourceID).
							Str("newID", newResourceID).
							Str("alertType", alert.Type).
							Msg("Migrating active alert from legacy guest ID format")

						oldResourceID := alert.ResourceID
						alert.ResourceID = newResourceID
						alert.ID = strings.Replace(alert.ID, oldResourceID, newResourceID, 1)
						alert.CanonicalSpecID = ""
						alert.CanonicalState = ""
						alert.CanonicalKind = ""
						if alert.Metadata != nil {
							delete(alert.Metadata, "canonicalSpecID")
							delete(alert.Metadata, "canonicalAlertKind")
						}
					}
				}
			}
		}

		legacyID := alert.ID
		backfillCanonicalIdentity(alert)
		alert.ID = exportedAlertID(alert, alert.ID)
		if alert.ID != legacyID {
			identityMigrationCount++
		}

		if seen[alert.ID] {
			duplicateCount++
			log.Warn().Str("alertID", alert.ID).Msg("skipping duplicate alert during restore")
			continue
		}
		seen[alert.ID] = true

		m.setActiveAlertNoLock(alert.ID, alert)
		if legacyID != alert.ID {
			if m.activeAlertAlias == nil {
				m.activeAlertAlias = make(map[string]string)
			}
			m.activeAlertAlias[legacyID] = activeAlertStorageKey(alert, alert.ID)
		}
		if alert.Acknowledged {
			ackTime := alert.StartTime
			if alert.AckTime != nil {
				ackTime = *alert.AckTime
			}
			m.setAckRecordNoLock(alert, alert.ID, ackRecord{
				acknowledged: true,
				user:         alert.AckUser,
				time:         ackTime,
			})
		}
		restoredCount++

		if alert.Level == AlertLevelCritical {
			restoredCritical = append(restoredCritical, alert.Clone())
		}
	}
	if err := m.loadIntentPendingNoLock(); err != nil {
		m.mu.Unlock()
		return err
	}
	// Rebuild the reducer from the exact restored active set. Active incidents
	// never expire merely because Pulse was restarted; only fresh healthy
	// evidence or an explicit operator action may resolve them.
	m.seedReducerCoreNoLock()
	m.mu.Unlock()

	log.Info().
		Int("restored", restoredCount).
		Int("total", len(alerts)).
		Int("duplicates", duplicateCount).
		Int("identityMigrations", identityMigrationCount).
		Str("source", source).
		Msg("Restored active alerts")
	epoch := m.restoredAlertEpoch.Add(1)
	for _, alert := range restoredCritical {
		m.scheduleRestoredCriticalAlert(alert, epoch)
	}
	if identityMigrationCount > 0 {
		// The asynchronous save waits for the load lock, then atomically replaces
		// the legacy file with the canonical IDs just restored into memory.
		m.saveActiveAlertsAsync("canonical-identity-migration")
	}
	return nil
}

func activeStateSnapshots(alerts []*Alert) ([]eventlog.ActiveStateSnapshot, error) {
	snapshots := make([]eventlog.ActiveStateSnapshot, 0, len(alerts))
	for _, alert := range alerts {
		if alert == nil {
			continue
		}
		snapshot, err := json.Marshal(alert)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal active alert %q for SQLite: %w", alert.ID, err)
		}
		snapshots = append(snapshots, eventlog.ActiveStateSnapshot{
			AlertID:             alert.ID,
			OccurrenceStartedAt: alert.StartTime,
			UpdatedAt:           time.Now().UTC(),
			Snapshot:            snapshot,
		})
	}
	return snapshots, nil
}

func (m *Manager) scheduleRestoredCriticalAlert(alert *Alert, epoch uint64) {
	if m == nil || alert == nil {
		return
	}
	go func(a *Alert) {
		delay := time.NewTimer(10 * time.Second)
		defer func() {
			if !delay.Stop() {
				select {
				case <-delay.C:
				default:
				}
			}
		}()

		select {
		case <-delay.C:
			if m.restoredAlertEpoch.Load() != epoch {
				return
			}
			log.Info().
				Str("alertID", a.ID).
				Str("resource", a.ResourceName).
				Msg("Attempting to send notification for restored critical alert")

			m.mu.Lock()
			if active, ok := m.getActiveAlertNoLock(a.ID); ok && active.StartTime.Equal(a.StartTime) {
				m.dispatchAlert(active, false)
			}
			m.mu.Unlock()
		case <-m.escalationStop:
			log.Debug().
				Str("alertID", a.ID).
				Msg("Cancelled startup notification due to shutdown")
		}
	}(alert.Clone())
}

func (m *Manager) getAlertsDir() string {
	if strings.TrimSpace(m.alertsDir) != "" {
		return m.alertsDir
	}

	return filepath.Join(utils.GetDataDir(), "alerts")
}

// periodicSaveAlerts saves active alerts to disk periodically.
func (m *Manager) periodicSaveAlerts() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.SaveActiveAlerts(); err != nil {
				log.Error().Err(err).Msg("failed to save active alerts during periodic save")
			}
		case <-m.escalationStop:
			return
		}
	}
}
