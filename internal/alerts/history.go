package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

const (
	// MaxHistoryDays is the maximum number of days to keep alert history
	MaxHistoryDays = 30
	// HistoryFileName is the name of the history file
	HistoryFileName = "alert-history.json"
	// HistoryBackupFileName is the name of the backup history file
	HistoryBackupFileName = "alert-history.backup.json"
)

// HistoryEntry represents a historical alert entry
type HistoryEntry struct {
	Alert     Alert     `json:"alert"`
	Timestamp time.Time `json:"timestamp"`
}

// AlertCallback is called when an alert is added to history
// This enables external systems to track alerts (e.g., pattern detection)
type AlertCallback func(alert Alert)

// HistoryManager manages persistent alert history
type HistoryManager struct {
	mu           sync.RWMutex
	saveMu       sync.Mutex // Serializes disk writes to prevent save race condition
	dataDir      string
	historyFile  string
	backupFile   string
	history      []HistoryEntry
	saveInterval time.Duration
	stopChan     chan struct{}
	saveTicker   *time.Ticker
	callbacks    []AlertCallback // Called when alerts are added
}

// NewHistoryManager creates a new history manager
func NewHistoryManager(dataDir string) *HistoryManager {
	if dataDir == "" {
		dataDir = utils.GetDataDir()
	}

	hm := &HistoryManager{
		dataDir:      dataDir,
		historyFile:  filepath.Join(dataDir, HistoryFileName),
		backupFile:   filepath.Join(dataDir, HistoryBackupFileName),
		history:      make([]HistoryEntry, 0),
		saveInterval: 5 * time.Minute,
		stopChan:     make(chan struct{}),
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Error().Err(err).Str("dir", dataDir).Msg("Failed to create data directory")
	}

	// Load existing history
	if err := hm.loadHistory(); err != nil {
		log.Error().Err(err).Msg("Failed to load alert history")
	}

	// Start periodic save routine
	hm.startPeriodicSave()

	// Start cleanup routine
	go hm.cleanupRoutine()

	return hm
}

// OnAlert registers a callback to be called when alerts are added
func (hm *HistoryManager) OnAlert(cb AlertCallback) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.callbacks = append(hm.callbacks, cb)
}

// AddAlert adds an alert to history
func (hm *HistoryManager) AddAlert(alert Alert) {
	hm.mu.Lock()

	entry := HistoryEntry{
		Alert:     *alert.Clone(),
		Timestamp: time.Now(),
	}

	hm.history = append(hm.history, entry)
	callbacks := hm.callbacks
	hm.mu.Unlock()

	log.Debug().Str("alertID", alert.ID).Msg("Added alert to history")

	// Call callbacks outside the lock
	for _, cb := range callbacks {
		cb(alert)
	}
}

// UpdateAlertLastSeen updates the LastSeen timestamp on the most recent
// history entry matching the given alert ID. This is called when an alert is
// resolved so that the stored history reflects the true duration of the alert,
// not just the snapshot captured at creation time.
func (hm *HistoryManager) UpdateAlertLastSeen(alertID string, lastSeen time.Time) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Iterate from newest to oldest to find the most recent entry for this alert
	for i := len(hm.history) - 1; i >= 0; i-- {
		if hm.history[i].Alert.ID == alertID {
			hm.history[i].Alert.LastSeen = lastSeen
			return
		}
	}
}

// GetHistory returns alert history within the specified time range
func (hm *HistoryManager) GetHistory(since time.Time, limit int) []Alert {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var results []Alert
	count := 0

	// Iterate from newest to oldest
	for i := len(hm.history) - 1; i >= 0 && (limit <= 0 || count < limit); i-- {
		entry := hm.history[i]
		if entry.Timestamp.After(since) {
			results = append(results, entry.Alert)
			count++
		}
	}

	return results
}

// GetAllHistory returns all alert history (up to limit)
func (hm *HistoryManager) GetAllHistory(limit int) []Alert {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if limit <= 0 || limit > len(hm.history) {
		limit = len(hm.history)
	}

	results := make([]Alert, 0, limit)
	start := len(hm.history) - limit

	for i := len(hm.history) - 1; i >= start; i-- {
		results = append(results, hm.history[i].Alert)
	}

	return results
}

// loadHistory loads history from disk
func (hm *HistoryManager) loadHistory() error {
	// Try loading from main file first
	sourceFile := hm.historyFile
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Str("file", sourceFile).Msg("Failed to read history file")
		}

		// Try backup file
		sourceFile = hm.backupFile
		data, err = os.ReadFile(sourceFile)
		if err != nil {
			if os.IsNotExist(err) {
				// Both files don't exist - this is normal on first startup
				log.Debug().Msg("No alert history files found, starting fresh")
				return nil
			}
			// Check if it's a permission error
			if os.IsPermission(err) {
				log.Warn().Err(err).Str("file", hm.backupFile).Msg("Permission denied reading backup history file - check file ownership")
				return nil // Continue without history rather than failing
			}
			return fmt.Errorf("historyManager.loadHistory: read backup history file %s: %w", hm.backupFile, err)
		}
		log.Info().Msg("Loaded alert history from backup file")
	}

	var history []HistoryEntry
	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("historyManager.loadHistory: unmarshal history from %s: %w", sourceFile, err)
	}

	hm.history = history
	log.Info().Int("count", len(history)).Msg("Loaded alert history")

	// Clean old entries immediately
	hm.cleanOldEntries()

	return nil
}

// saveHistory saves history to disk with retry logic
func (hm *HistoryManager) saveHistory() error {
	return hm.saveHistoryWithRetry(3)
}

// saveHistoryWithRetry saves history with exponential backoff retry
func (hm *HistoryManager) saveHistoryWithRetry(maxRetries int) error {
	if maxRetries < 1 {
		return fmt.Errorf("historyManager.saveHistoryWithRetry: maxRetries must be at least 1, got %d", maxRetries)
	}

	// Serialize all disk writes to prevent concurrent saves from overwriting each other
	hm.saveMu.Lock()
	defer hm.saveMu.Unlock()

	hm.mu.RLock()
	snapshot := make([]HistoryEntry, len(hm.history))
	copy(snapshot, hm.history)
	hm.mu.RUnlock()

	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("historyManager.saveHistoryWithRetry: marshal history snapshot: %w", err)
	}

	historyFile := hm.historyFile
	backupFile := hm.backupFile

	// Create backup of existing file once before any write attempts.
	// This ensures we don't lose data if all retries fail.
	backupCreated := false
	if _, err := os.Stat(historyFile); err == nil {
		if err := os.Rename(historyFile, backupFile); err != nil {
			log.Warn().
				Err(err).
				Str("source", historyFile).
				Str("backup", backupFile).
				Msg("Failed to create backup file")
		} else {
			backupCreated = true
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Write new file
		if err := os.WriteFile(historyFile, data, 0644); err != nil {
			lastErr = fmt.Errorf("write history file %s (attempt %d/%d): %w", historyFile, attempt, maxRetries, err)
			log.Warn().
				Err(lastErr).
				Int("attempt", attempt).
				Int("maxRetries", maxRetries).
				Msg("Failed to write history file, will retry")

			// Exponential backoff: 100ms, 200ms, 400ms
			if attempt < maxRetries {
				backoff := time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
				time.Sleep(backoff)
			}
			continue
		}

		// Success - remove backup file now that we've successfully written
		if backupCreated {
			if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("file", backupFile).Msg("Failed to remove backup file after successful save")
			}
		}
		log.Debug().Int("entries", len(snapshot)).Msg("Saved alert history")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("historyManager.saveHistoryWithRetry: no write attempts executed for %s", historyFile)
	}

	// All retries failed - restore backup if we have one
	if backupCreated {
		if restoreErr := os.Rename(backupFile, historyFile); restoreErr != nil {
			restoreErr = fmt.Errorf("restore backup file %s to %s: %w", backupFile, historyFile, restoreErr)
			log.Error().Err(restoreErr).Msg("Failed to restore backup after all write attempts failed")
			return fmt.Errorf("historyManager.saveHistoryWithRetry: failed to write history file after %d attempts: %w", maxRetries, errors.Join(lastErr, restoreErr))
		}
		log.Info().Msg("Restored backup after history save failure")
	}

	return fmt.Errorf("historyManager.saveHistoryWithRetry: failed to write history file after %d attempts: %w", maxRetries, lastErr)
}

// startPeriodicSave starts the periodic save routine
func (hm *HistoryManager) startPeriodicSave() {
	hm.saveTicker = time.NewTicker(hm.saveInterval)

	go func() {
		for {
			select {
			case <-hm.saveTicker.C:
				if err := hm.saveHistory(); err != nil {
					log.Error().Err(err).Msg("Failed to save alert history")
				}
			case <-hm.stopChan:
				return
			}
		}
	}()
}

// cleanupRoutine runs periodically to clean old entries
func (hm *HistoryManager) cleanupRoutine() {
	// Run cleanup daily
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Also run cleanup on startup after a delay
	// Also run cleanup on startup after a delay
	select {
	case <-time.After(1 * time.Minute):
		hm.cleanOldEntries()
	case <-hm.stopChan:
		return
	}

	for {
		select {
		case <-ticker.C:
			hm.cleanOldEntries()
		case <-hm.stopChan:
			return
		}
	}
}

// cleanOldEntries removes entries older than MaxHistoryDays
func (hm *HistoryManager) cleanOldEntries() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -MaxHistoryDays)
	newHistory := make([]HistoryEntry, 0, len(hm.history))

	removed := 0
	for _, entry := range hm.history {
		if entry.Timestamp.After(cutoff) {
			newHistory = append(newHistory, entry)
		} else {
			removed++
		}
	}

	if removed > 0 {
		hm.history = newHistory
		log.Info().
			Int("removed", removed).
			Int("remaining", len(newHistory)).
			Msg("Cleaned old alert history entries")
	}
}

// RemoveAlert removes a specific alert from history by ID
func (hm *HistoryManager) RemoveAlert(alertID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	newHistory := make([]HistoryEntry, 0, len(hm.history))
	removed := false

	for _, entry := range hm.history {
		if entry.Alert.ID != alertID {
			newHistory = append(newHistory, entry)
		} else {
			removed = true
		}
	}

	if removed {
		hm.history = newHistory
		log.Debug().Str("alertID", alertID).Msg("Removed alert from history")
	}
}

// ClearAllHistory clears all alert history
func (hm *HistoryManager) ClearAllHistory() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Clear the in-memory history
	hm.history = make([]HistoryEntry, 0)

	// Remove the history files
	var removeErrs []error
	if err := os.Remove(hm.historyFile); err != nil && !os.IsNotExist(err) {
		removeErrs = append(removeErrs, fmt.Errorf("remove history file %s: %w", hm.historyFile, err))
	}
	if err := os.Remove(hm.backupFile); err != nil && !os.IsNotExist(err) {
		removeErrs = append(removeErrs, fmt.Errorf("remove backup file %s: %w", hm.backupFile, err))
	}
	if len(removeErrs) > 0 {
		return fmt.Errorf("clear alert history files: %w", errors.Join(removeErrs...))
	}

	log.Info().Msg("Cleared all alert history")
	return nil
}

// Stop stops the history manager
func (hm *HistoryManager) Stop() {
	close(hm.stopChan)
	if hm.saveTicker != nil {
		hm.saveTicker.Stop()
	}

	// Save one final time
	if err := hm.saveHistory(); err != nil {
		log.Error().Err(err).Msg("Failed to save alert history on shutdown")
	}
}

// GetStats returns statistics about the alert history
func (hm *HistoryManager) GetStats() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	oldest := time.Now()
	newest := time.Time{}

	if len(hm.history) > 0 {
		oldest = hm.history[0].Timestamp
		newest = hm.history[len(hm.history)-1].Timestamp
	}

	return map[string]interface{}{
		"totalEntries": len(hm.history),
		"oldestEntry":  oldest,
		"newestEntry":  newest,
		"dataDir":      hm.dataDir,
		"fileSize":     hm.getFileSize(),
	}
}

// getFileSize returns the size of the history file
func (hm *HistoryManager) getFileSize() int64 {
	info, err := os.Stat(hm.historyFile)
	if err != nil {
		return 0
	}
	return info.Size()
}
