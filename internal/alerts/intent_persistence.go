package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

const intentPendingFileName = "intent-pending.json"

func (m *Manager) saveIntentPendingSnapshot() error {
	m.mu.RLock()
	states := make([]IntentPendingState, 0, len(m.intentPending))
	for _, state := range m.intentPending {
		states = append(states, state)
	}
	m.mu.RUnlock()
	sort.Slice(states, func(i, j int) bool { return states[i].TrackingKey < states[j].TrackingKey })
	data, err := json.Marshal(states)
	if err != nil {
		return fmt.Errorf("marshal alert intent pending state: %w", err)
	}
	alertsDir := m.getAlertsDir()
	if err := os.MkdirAll(alertsDir, alertsDirPerm); err != nil {
		return fmt.Errorf("create alert intent state directory: %w", err)
	}
	tmp, err := os.CreateTemp(alertsDir, "intent-pending-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create alert intent state temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err := os.Remove(tmpName); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("file", tmpName).Msg("Failed to remove alert intent state temp file")
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write alert intent state temp file: %w", err)
	}
	if err := tmp.Chmod(alertsFilePerm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod alert intent state temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close alert intent state temp file: %w", err)
	}
	finalPath := filepath.Join(alertsDir, intentPendingFileName)
	if err := os.Rename(tmpName, finalPath); err != nil {
		return fmt.Errorf("persist alert intent pending state: %w", err)
	}
	if err := os.Chmod(finalPath, alertsFilePerm); err != nil {
		return fmt.Errorf("chmod alert intent pending state: %w", err)
	}
	return nil
}

// loadIntentPendingNoLock restores pending policy candidates. Caller holds m.mu.
func (m *Manager) loadIntentPendingNoLock() error {
	path := filepath.Join(m.getAlertsDir(), intentPendingFileName)
	data, err := readLimitedRegularFile(path, maxActiveAlertsFileSizeBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read alert intent pending state: %w", err)
	}
	var states []IntentPendingState
	if err := json.Unmarshal(data, &states); err != nil {
		return fmt.Errorf("decode alert intent pending state: %w", err)
	}
	now := time.Now()
	if m.intentPending == nil {
		m.intentPending = make(map[string]IntentPendingState)
	}
	for _, state := range states {
		if state.TrackingKey == "" || state.ResourceID == "" || state.Signal == "" || state.FirstMatchedAt.IsZero() {
			continue
		}
		if now.Sub(state.LastObservedAt) > 24*time.Hour || state.LastObservedAt.After(now.Add(time.Minute)) || state.FirstMatchedAt.After(now.Add(time.Minute)) {
			continue
		}
		m.intentPending[state.TrackingKey] = state
	}
	return nil
}
