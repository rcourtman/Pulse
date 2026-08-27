package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rs/zerolog/log"
)

const activeStateDegradedMarker = "active-state-sqlite-degraded"

// bootstrapActiveState establishes one restart authority. Existing databases
// load their projection; pre-projection databases import the already-decoded
// JSON recovery mirror. A degraded marker reverses that preference exactly
// once so a newer mirror can repair a previously failed SQLite checkpoint.
func (m *Manager) bootstrapActiveState(store *eventlog.Store) bool {
	if m == nil || store == nil {
		return false
	}

	initialized, err := store.ActiveStateInitialized()
	if err != nil {
		m.markActiveStateDegraded(err)
		log.Error().Err(err).Msg("SQLite active alert authority unavailable; keeping recovery mirror state")
		return false
	}
	degraded := m.activeStateDegraded()
	recoveryFileExists := false
	if _, err := os.Stat(filepath.Join(m.getAlertsDir(), "active-alerts.json")); err == nil {
		recoveryFileExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Warn().Err(err).Msg("could not inspect active alert recovery mirror")
	}

	if initialized && !m.skipPersistedRestore && (!degraded || !recoveryFileExists || !m.activeRecoveryReadable.Load()) {
		snapshots, err := store.LoadActiveState()
		if err != nil {
			m.markActiveStateDegraded(err)
			log.Error().Err(err).Msg("failed to load SQLite active alert authority; keeping recovery mirror state")
			return false
		}
		alerts, err := decodeActiveStateSnapshots(snapshots)
		if err != nil {
			m.markActiveStateDegraded(err)
			log.Error().Err(err).Msg("SQLite active alert authority is invalid; keeping recovery mirror state")
			return false
		}
		if err := m.restoreActiveAlertSnapshots(alerts, "SQLite active-state projection", true); err != nil {
			m.markActiveStateDegraded(err)
			log.Error().Err(err).Msg("failed to restore SQLite active alert authority")
			return false
		}
		m.activeStateAuthoritative.Store(true)
		m.clearActiveStateDegraded()
		return true
	}

	// A new database, an explicit clean-room manager, or a prior failed
	// checkpoint starts from the in-memory recovery state already loaded by the
	// constructor. Never overwrite a new database from malformed JSON.
	if !m.skipPersistedRestore && recoveryFileExists && !m.activeRecoveryReadable.Load() {
		log.Error().Msg("active alert recovery mirror is unreadable; SQLite authority initialization deferred")
		return false
	}
	snapshots, err := m.currentActiveStateSnapshots()
	if err != nil {
		m.markActiveStateDegraded(err)
		log.Error().Err(err).Msg("failed to encode active alerts for SQLite authority")
		return false
	}
	if err := store.ReplaceActiveState(snapshots); err != nil {
		m.markActiveStateDegraded(err)
		log.Error().Err(err).Msg("failed to initialize SQLite active alert authority")
		return false
	}
	m.activeStateAuthoritative.Store(true)
	m.clearActiveStateDegraded()
	log.Info().Int("alerts", len(snapshots)).Msg("SQLite active alert authority initialized from recovery state")
	return true
}

func decodeActiveStateSnapshots(snapshots []eventlog.ActiveStateSnapshot) ([]*Alert, error) {
	alerts := make([]*Alert, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var alert Alert
		if err := json.Unmarshal(snapshot.Snapshot, &alert); err != nil {
			return nil, fmt.Errorf("decode SQLite active alert %q: %w", snapshot.AlertID, err)
		}
		if alert.ID == "" {
			alert.ID = snapshot.AlertID
		}
		if alert.ID == "" {
			return nil, fmt.Errorf("SQLite active alert has no identity")
		}
		alerts = append(alerts, &alert)
	}
	return alerts, nil
}

func (m *Manager) currentActiveStateSnapshots() ([]eventlog.ActiveStateSnapshot, error) {
	m.mu.RLock()
	alerts := make([]*Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		alerts = append(alerts, cloneAlertForOutput(alert))
	}
	m.mu.RUnlock()
	return activeStateSnapshots(alerts)
}

func (m *Manager) activeStateDegradedPath() string {
	return filepath.Join(m.getAlertsDir(), activeStateDegradedMarker)
}

func (m *Manager) activeStateDegraded() bool {
	_, err := os.Stat(m.activeStateDegradedPath())
	return err == nil
}

func (m *Manager) markActiveStateDegraded(cause error) {
	if m == nil {
		return
	}
	m.activeStateAuthoritative.Store(false)
	if err := os.MkdirAll(m.getAlertsDir(), alertsDirPerm); err != nil {
		log.Error().Err(err).Msg("failed to create alerts directory for SQLite degradation marker")
		return
	}
	message := "SQLite active alert state requires recovery from active-alerts.json\n"
	if cause != nil {
		message += cause.Error() + "\n"
	}
	if err := os.WriteFile(m.activeStateDegradedPath(), []byte(message), alertsFilePerm); err != nil {
		log.Error().Err(err).Msg("failed to persist SQLite active alert degradation marker")
	}
}

func (m *Manager) clearActiveStateDegraded() {
	if m == nil {
		return
	}
	if err := os.Remove(m.activeStateDegradedPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warn().Err(err).Msg("failed to clear SQLite active alert degradation marker")
	}
}
