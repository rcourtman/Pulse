package alerts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rs/zerolog/log"
)

const (
	activeStateDegradedMarker        = "active-state-sqlite-degraded"
	activeStateRecoverySchemaVersion = 1
)

type activeStateRecoveryEnvelope struct {
	SchemaVersion int       `json:"schemaVersion"`
	RecordedAt    time.Time `json:"recordedAt"`
	Cause         string    `json:"cause,omitempty"`
	Alerts        []*Alert  `json:"alerts"`
}

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

	if initialized && !m.skipPersistedRestore && !degraded {
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
		return true
	}

	if degraded && !m.skipPersistedRestore {
		recovered, selfContained, err := m.loadActiveStateDegradedRecovery()
		if err != nil {
			log.Error().Err(err).Msg("active alert recovery marker is invalid; SQLite authority initialization deferred")
			return false
		}
		if selfContained {
			if err := m.restoreActiveAlertSnapshots(recovered, "crash-safe SQLite recovery marker", true); err != nil {
				log.Error().Err(err).Msg("failed to restore active alerts from crash-safe recovery marker")
				return false
			}
			// The self-contained marker is the trusted source, so it can also
			// replace a missing or malformed compatibility mirror. Keep the
			// marker until both recovery copies and SQLite agree.
			m.activeRecoveryWriteBlock.Store(false)
			if err := m.writeActiveAlertsRecoveryMirror(recovered); err != nil {
				log.Error().Err(err).Msg("failed to repair active alert recovery mirror from crash-safe marker")
				return false
			}
			m.activeRecoveryReadable.Store(true)
		} else if !recoveryFileExists || !m.activeRecoveryReadable.Load() {
			log.Error().Msg("legacy active alert degradation marker has no readable recovery mirror; SQLite authority initialization deferred")
			return false
		}
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
	if err := m.clearActiveStateDegraded(); err != nil {
		log.Error().Err(err).Msg("failed to durably clear repaired SQLite active alert degradation marker")
		return false
	}
	m.activeStateAuthoritative.Store(true)
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
	message := "SQLite active alert state requires recovery from active-alerts.json\n"
	if cause != nil {
		message += cause.Error() + "\n"
	}
	m.recoveryMirrorMu.Lock()
	_, statErr := os.Stat(m.activeStateDegradedPath())
	var err error
	if errors.Is(statErr, os.ErrNotExist) {
		err = m.writeActiveStateDegradedMarker([]byte(message))
	} else if statErr != nil {
		err = fmt.Errorf("inspect existing SQLite active alert degradation marker: %w", statErr)
	}
	m.recoveryMirrorMu.Unlock()
	if err != nil {
		log.Error().Err(err).Msg("failed to persist SQLite active alert degradation marker")
	}
}

func (m *Manager) writeActiveStateDegradedRecovery(alerts []*Alert, cause error) error {
	envelope := activeStateRecoveryEnvelope{
		SchemaVersion: activeStateRecoverySchemaVersion,
		RecordedAt:    time.Now().UTC(),
		Alerts:        alerts,
	}
	if envelope.Alerts == nil {
		envelope.Alerts = []*Alert{}
	}
	if cause != nil {
		envelope.Cause = cause.Error()
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal SQLite active alert recovery marker: %w", err)
	}
	return m.writeActiveStateDegradedMarker(data)
}

func (m *Manager) writeActiveStateDegradedMarker(data []byte) error {
	alertsDir := m.getAlertsDir()
	if err := os.MkdirAll(alertsDir, alertsDirPerm); err != nil {
		return fmt.Errorf("create alerts directory for SQLite degradation marker: %w", err)
	}
	if err := os.Chmod(alertsDir, alertsDirPerm); err != nil {
		return fmt.Errorf("set alerts directory permissions for SQLite degradation marker: %w", err)
	}
	tmp, err := os.CreateTemp(alertsDir, ".active-state-sqlite-degraded-*.tmp")
	if err != nil {
		return fmt.Errorf("create SQLite degradation marker temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write SQLite degradation marker temp file: %w", err)
	}
	if err := tmp.Chmod(alertsFilePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set SQLite degradation marker permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync SQLite degradation marker temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close SQLite degradation marker temp file: %w", err)
	}
	if err := replaceActiveAlertsFile(tmpName, m.activeStateDegradedPath()); err != nil {
		return fmt.Errorf("replace SQLite degradation marker: %w", err)
	}
	cleanup = false
	if err := syncActiveAlertsDirectory(alertsDir); err != nil {
		return fmt.Errorf("sync alerts directory after SQLite degradation marker: %w", err)
	}
	return nil
}

func (m *Manager) loadActiveStateDegradedRecovery() ([]*Alert, bool, error) {
	data, err := readLimitedRegularFile(m.activeStateDegradedPath(), maxActiveAlertsFileSizeBytes)
	if err != nil {
		return nil, false, fmt.Errorf("read SQLite active alert recovery marker: %w", err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false, nil
	}
	var envelope activeStateRecoveryEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, false, fmt.Errorf("decode SQLite active alert recovery marker: %w", err)
	}
	if envelope.SchemaVersion != activeStateRecoverySchemaVersion {
		return nil, false, fmt.Errorf("unsupported SQLite active alert recovery marker schema %d", envelope.SchemaVersion)
	}
	if envelope.Alerts == nil {
		return nil, false, fmt.Errorf("SQLite active alert recovery marker has no alert snapshot")
	}
	return envelope.Alerts, true, nil
}

func (m *Manager) clearActiveStateDegraded() error {
	if m == nil {
		return nil
	}
	err := os.Remove(m.activeStateDegradedPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove SQLite active alert degradation marker: %w", err)
	}
	if err := syncActiveAlertsDirectory(m.getAlertsDir()); err != nil {
		return fmt.Errorf("sync alerts directory after clearing SQLite degradation marker: %w", err)
	}
	return nil
}
