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

// HistoryStats represents typed alert history statistics.
type HistoryStats struct {
	TotalEntries int
	OldestEntry  time.Time
	NewestEntry  time.Time
	DataDir      string
	FileSize     int64
}

// AlertCallback is called when an alert is added to history
// This enables external systems to track alerts (e.g., pattern detection)
type AlertCallback func(alert Alert)

// HistoryManager manages persistent alert history
type HistoryManager struct {
	mu           sync.RWMutex
	saveMu       sync.Mutex // Serializes disk writes to prevent save race condition
	stopOnce     sync.Once
	dataDir      string
	historyFile  string
	backupFile   string
	history      []HistoryEntry
	saveInterval time.Duration
	stopChan     chan struct{}
	saveTicker   *time.Ticker
	callbacks    []AlertCallback // Called when alerts are added
	stopOnce     sync.Once
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
	if err := os.MkdirAll(dataDir, alertsDirPerm); err != nil {
		log.Error().Err(err).Str("dir", dataDir).Msg("Failed to create data directory")
	} else if err := os.Chmod(dataDir, alertsDirPerm); err != nil {
		log.Warn().Err(err).Str("dir", dataDir).Msg("Failed to harden history directory permissions")
	}

	// Load existing history
	if err := hm.loadHistory(); err != nil {
		log.Error().Err(err).Msg("failed to load alert history")
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
	callbacks := append([]AlertCallback(nil), hm.callbacks...)
	hm.mu.Unlock()

	log.Debug().Str("alertID", alert.ID).Msg("added alert to history")

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
	data, err := readLimitedRegularFile(hm.historyFile, maxAlertHistoryFileSizeBytes)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Str("file", hm.historyFile).Msg("Failed to read history file")
		}

		// Try backup file
		data, err = readLimitedRegularFile(hm.backupFile, maxAlertHistoryFileSizeBytes)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if os.IsPermission(err) {
				hadPermissionError = true
				log.Warn().
					Err(err).
					Str("file", source.path).
					Msg("Permission denied reading history file - check file ownership")
				continue
			}
			log.Warn().Err(err).Str("file", source.path).Msg("Failed to read history file")
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to read history file %q: %w", source.path, err)
			}
			continue
		}

		var history []HistoryEntry
		if err := json.Unmarshal(data, &history); err != nil {
			log.Warn().
				Err(err).
				Str("file", source.path).
				Msg("Failed to parse history file")
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to unmarshal history from %q: %w", source.path, err)
			}
			continue
		}

		hm.history = history
		if !source.isMain {
			log.Info().Msg("loaded alert history from backup file")
		}
		log.Info().Int("count", len(history)).Msg("Loaded alert history")

		// Clean old entries immediately
		hm.cleanOldEntries()
		return nil
	}

	if firstErr != nil {
		return firstErr
	}
	if hadPermissionError {
		return nil
	}

	// Both files don't exist - this is normal on first startup
	log.Debug().Msg("No alert history files found, starting fresh")
	return nil
}

// saveHistory saves history to disk with retry logic
func (hm *HistoryManager) saveHistory() error {
	return hm.saveHistoryWithRetry(3)
}

// saveHistoryWithRetry saves history with exponential backoff retry
func (hm *HistoryManager) saveHistoryWithRetry(maxRetries int) error {
	if maxRetries < 1 {
		maxRetries = 1
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
		if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("file", backupFile).Msg("Failed to remove existing backup file before save")
		}
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
		tempFile := fmt.Sprintf("%s.tmp-%d-%d", historyFile, os.Getpid(), time.Now().UnixNano())

		if err := writeFileSynced(tempFile, data, 0644); err != nil {
			lastErr = fmt.Errorf("failed to write temp history file: %w", err)
			_ = os.Remove(tempFile)
			log.Warn().
				Err(lastErr).
				Int("attempt", attempt).
				Int("maxRetries", maxRetries).
				Msg("failed to write history file, will retry")

			// Exponential backoff: 100ms, 200ms, 400ms
			if attempt < maxRetries {
				backoff := time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
				time.Sleep(backoff)
			}
			continue
		}

		if err := os.Rename(tempFile, historyFile); err != nil {
			lastErr = fmt.Errorf("failed to replace history file with temp file: %w", err)
			_ = os.Remove(tempFile)
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
		if err := os.Chmod(historyFile, alertsFilePerm); err != nil {
			lastErr = err
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("maxRetries", maxRetries).
				Msg("Failed to harden history file permissions, will retry")

			if attempt < maxRetries {
				backoff := time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
				time.Sleep(backoff)
			}
			continue
		}

		if err := syncDir(filepath.Dir(historyFile)); err != nil {
			log.Warn().Err(err).Str("dir", filepath.Dir(historyFile)).Msg("Failed to sync history directory")
		}

		// Success - remove backup file now that we've successfully written
		if backupCreated {
			if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("file", backupFile).Msg("failed to remove backup file after successful save")
			}
		}
		log.Debug().Int("entries", len(snapshot)).Msg("saved alert history")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("historyManager.saveHistoryWithRetry: no write attempts executed for %s", historyFile)
	}

	// All retries failed - restore backup if we have one
	if backupCreated {
		if restoreErr := os.Rename(backupFile, historyFile); restoreErr != nil {
			restoreErr = fmt.Errorf("restore backup file %s to %s: %w", backupFile, historyFile, restoreErr)
			log.Error().Err(restoreErr).Msg("failed to restore backup after all write attempts failed")
		} else {
			if err := syncDir(filepath.Dir(historyFile)); err != nil {
				log.Warn().Err(err).Str("dir", filepath.Dir(historyFile)).Msg("Failed to sync history directory after restore")
			}
			log.Info().Msg("restored backup after history save failure")
		}
		log.Info().Msg("Restored backup after history save failure")
	}

	return fmt.Errorf("historyManager.saveHistoryWithRetry: failed to write history file after %d attempts: %w", maxRetries, lastErr)
}

func writeFileSynced(path string, data []byte, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}

	return file.Close()
}

func syncDir(dirPath string) error {
	dir, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	return dir.Sync()
}

// startPeriodicSave starts the periodic save routine
func (hm *HistoryManager) startPeriodicSave() {
	hm.saveTicker = time.NewTicker(hm.saveInterval)

	go func() {
		for {
			select {
			case <-hm.saveTicker.C:
				if err := hm.saveHistory(); err != nil {
					log.Error().Err(err).Msg("failed to save alert history")
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
	startupDelay := time.NewTimer(1 * time.Minute)
	defer func() {
		if !startupDelay.Stop() {
			select {
			case <-startupDelay.C:
			default:
			}
		}
	}()

	select {
	case <-startupDelay.C:
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
			Msg("cleaned old alert history entries")
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
		log.Debug().Str("alertID", alertID).Msg("removed alert from history")
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

	log.Info().Msg("cleared all alert history")
	return nil
}

// Stop stops the history manager
func (hm *HistoryManager) Stop() {
	hm.stopOnce.Do(func() {
		close(hm.stopChan)
		if hm.saveTicker != nil {
			hm.saveTicker.Stop()
		}

		// Save one final time
		if err := hm.saveHistory(); err != nil {
			log.Error().Err(err).Msg("Failed to save alert history on shutdown")
		}
	})
}

// GetStats returns statistics about the alert history
func (hm *HistoryManager) GetStats() map[string]any {
	stats := hm.Stats()
	return map[string]any{
		"totalEntries": stats.TotalEntries,
		"oldestEntry":  stats.OldestEntry,
		"newestEntry":  stats.NewestEntry,
		"dataDir":      stats.DataDir,
		"fileSize":     stats.FileSize,
	}
}

// Stats returns typed statistics about the alert history.
func (hm *HistoryManager) Stats() HistoryStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	oldest := time.Now()
	newest := time.Time{}

	if len(hm.history) > 0 {
		oldest = hm.history[0].Timestamp
		newest = hm.history[len(hm.history)-1].Timestamp
	}

	return HistoryStats{
		TotalEntries: len(hm.history),
		OldestEntry:  oldest,
		NewestEntry:  newest,
		DataDir:      hm.dataDir,
		FileSize:     hm.getFileSize(),
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
