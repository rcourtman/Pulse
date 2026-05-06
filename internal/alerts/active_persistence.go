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

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

func asyncSaveActiveAlerts(reason string, save func() error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("reason", reason).Msg("panic in SaveActiveAlerts goroutine")
			}
		}()
		if err := save(); err != nil {
			log.Error().Err(err).Str("reason", reason).Msg("failed to save active alerts")
		}
	}()
}

// SaveActiveAlerts persists active alerts to disk.
func (m *Manager) SaveActiveAlerts() error {
	// Serialize snapshots and writes so concurrent async saves cannot
	// overwrite newer state with an older snapshot.
	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	m.mu.RLock()
	alerts := make([]*Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		clone := alert.Clone()
		backfillCanonicalIdentity(clone)
		alerts = append(alerts, clone)
	}
	m.mu.RUnlock()

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
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close active alerts temp file %s: %w", tmpName, err)
	}

	finalFile := filepath.Join(alertsDir, "active-alerts.json")
	if err := os.Rename(tmpName, finalFile); err != nil {
		return fmt.Errorf("failed to rename active alerts file from %s to %s: %w", tmpName, finalFile, err)
	}
	if err := os.Chmod(finalFile, alertsFilePerm); err != nil {
		return fmt.Errorf("failed to set active alerts file permissions: %w", err)
	}

	log.Debug().Int("count", len(alerts)).Msg("saved active alerts to disk")
	return nil
}

func (m *Manager) saveActiveAlertsAsync(context string) {
	go func() {
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

// LoadActiveAlerts restores active alerts from disk.
func (m *Manager) LoadActiveAlerts() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alertsFile := filepath.Join(m.getAlertsDir(), "active-alerts.json")
	data, err := readLimitedRegularFile(alertsFile, maxActiveAlertsFileSizeBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Info().Msg("No active alerts file found, starting fresh")
			return nil
		}
		return fmt.Errorf("failed to read active alerts: %w", err)
	}

	var alerts []*Alert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return fmt.Errorf("failed to unmarshal active alerts: %w", err)
	}
	if err := os.Chmod(alertsFile, alertsFilePerm); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("file", alertsFile).Msg("Failed to harden active alerts file permissions")
	}

	now := time.Now()
	restoredCount := 0
	duplicateCount := 0
	seen := make(map[string]bool)

	for _, alert := range alerts {
		backfillCanonicalIdentity(alert)

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
					}
				}
			}
		}

		if seen[alert.ID] {
			duplicateCount++
			log.Warn().Str("alertID", alert.ID).Msg("skipping duplicate alert during restore")
			continue
		}
		seen[alert.ID] = true

		if now.Sub(alert.StartTime) > 24*time.Hour {
			log.Debug().Str("alertID", alert.ID).Msg("skipping old alert during restore")
			continue
		}

		if alert.Acknowledged && alert.AckTime != nil && now.Sub(*alert.AckTime) > time.Hour {
			log.Debug().Str("alertID", alert.ID).Msg("skipping old acknowledged alert from activeAlerts but preserving ackState")
			ackTime := alert.StartTime
			if alert.AckTime != nil {
				ackTime = *alert.AckTime
			}
			m.setAckRecordNoLock(alert, alert.ID, ackRecord{
				acknowledged: true,
				user:         alert.AckUser,
				time:         ackTime,
			})
			continue
		}

		m.setActiveAlertNoLock(alert.ID, alert)
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

		// Critical alerts restored shortly after restart are redispatched after a
		// small delay so notification policy still controls delivery.
		if alert.Level == AlertLevelCritical && now.Sub(alert.StartTime) < 2*time.Hour {
			alertCopy := alert.Clone()
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
					log.Info().
						Str("alertID", a.ID).
						Str("resource", a.ResourceName).
						Msg("Attempting to send notification for restored critical alert")

					m.mu.Lock()
					m.dispatchAlert(a, false)
					m.mu.Unlock()
				case <-m.escalationStop:
					log.Debug().
						Str("alertID", a.ID).
						Msg("Cancelled startup notification due to shutdown")
					return
				}
			}(alertCopy)
		}
	}

	log.Info().
		Int("restored", restoredCount).
		Int("total", len(alerts)).
		Int("duplicates", duplicateCount).
		Msg("Restored active alerts from disk")
	return nil
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
