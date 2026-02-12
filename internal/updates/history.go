package updates

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog/log"
)

// UpdateAction represents the type of update action
type UpdateAction string

const (
	ActionUpdate   UpdateAction = "update"
	ActionRollback UpdateAction = "rollback"
)

// UpdateStatus represents the outcome of an update
type UpdateStatusType string

const (
	StatusInProgress UpdateStatusType = "in_progress"
	StatusSuccess    UpdateStatusType = "success"
	StatusFailed     UpdateStatusType = "failed"
	StatusRolledBack UpdateStatusType = "rolled_back"
	StatusCancelled  UpdateStatusType = "cancelled"
)

// InitiatedBy represents who triggered the update
type InitiatedBy string

const (
	InitiatedByUser InitiatedBy = "user"
	InitiatedByAuto InitiatedBy = "auto"
	InitiatedByAPI  InitiatedBy = "api"
)

// InitiatedVia represents the interface used to trigger the update
type InitiatedVia string

const (
	InitiatedViaUI      InitiatedVia = "ui"
	InitiatedViaAPI     InitiatedVia = "api"
	InitiatedViaCLI     InitiatedVia = "cli"
	InitiatedViaScript  InitiatedVia = "script"
	InitiatedViaWebhook InitiatedVia = "webhook"
)

// UpdateHistoryEntry represents a single update event
type UpdateHistoryEntry struct {
	EventID        string           `json:"event_id"`
	Timestamp      time.Time        `json:"timestamp"`
	Action         UpdateAction     `json:"action"`
	Channel        string           `json:"channel"`
	VersionFrom    string           `json:"version_from"`
	VersionTo      string           `json:"version_to"`
	DeploymentType string           `json:"deployment_type"`
	InitiatedBy    InitiatedBy      `json:"initiated_by"`
	InitiatedVia   InitiatedVia     `json:"initiated_via"`
	Status         UpdateStatusType `json:"status"`
	DurationMs     int64            `json:"duration_ms"`
	BackupPath     string           `json:"backup_path,omitempty"`
	LogPath        string           `json:"log_path,omitempty"`
	Error          *UpdateError     `json:"error,omitempty"`
	DownloadBytes  int64            `json:"download_bytes,omitempty"`
	RelatedEventID string           `json:"related_event_id,omitempty"`
	Notes          string           `json:"notes,omitempty"`
}

// UpdateError represents error information
type UpdateError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HistoryFilter represents filters for querying update history
type HistoryFilter struct {
	Status         UpdateStatusType
	Action         UpdateAction
	DeploymentType string
	Limit          int
}

// UpdateHistory manages the update history log
type UpdateHistory struct {
	logPath  string
	mu       sync.RWMutex
	cache    []UpdateHistoryEntry
	maxCache int
}

// NewUpdateHistory creates a new update history manager
func NewUpdateHistory(dataDir string) (*UpdateHistory, error) {
	if dataDir == "" {
		dataDir = "/var/lib/pulse"
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	logPath := filepath.Join(dataDir, "update-history.jsonl")

	h := &UpdateHistory{
		logPath:  logPath,
		cache:    make([]UpdateHistoryEntry, 0, 100),
		maxCache: 100,
	}

	// Load existing entries into cache
	if err := h.loadCache(); err != nil {
		log.Warn().Err(err).Msg("Failed to load update history cache")
	}

	return h, nil
}

// CreateEntry creates a new update history entry and returns the event ID
func (h *UpdateHistory) CreateEntry(ctx context.Context, entry UpdateHistoryEntry) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Generate event ID if not provided
	if entry.EventID == "" {
		entry.EventID = ulid.Make().String()
	}

	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Write to file
	if err := h.appendToFile(entry); err != nil {
		return "", fmt.Errorf("failed to write to history file: %w", err)
	}

	// Add to cache
	h.addToCache(entry)

	log.Info().
		Str("event_id", entry.EventID).
		Str("action", string(entry.Action)).
		Str("version_from", entry.VersionFrom).
		Str("version_to", entry.VersionTo).
		Msg("Created update history entry")

	return entry.EventID, nil
}

// UpdateEntry updates an existing entry (used to update status after completion)
func (h *UpdateHistory) UpdateEntry(ctx context.Context, eventID string, updateFn func(*UpdateHistoryEntry) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find entry in cache
	var entry *UpdateHistoryEntry
	for i := range h.cache {
		if h.cache[i].EventID == eventID {
			entry = &h.cache[i]
			break
		}
	}

	if entry == nil {
		return fmt.Errorf("entry not found: %s", eventID)
	}

	// Apply update
	if err := updateFn(entry); err != nil {
		return err
	}

	// Rewrite the entire file (JSONL doesn't support in-place updates)
	// For small files this is acceptable; for large files we'd need a database
	if err := h.rewriteFile(); err != nil {
		return fmt.Errorf("failed to update history file: %w", err)
	}

	log.Info().
		Str("event_id", eventID).
		Str("status", string(entry.Status)).
		Msg("Updated update history entry")

	return nil
}

// GetEntry retrieves a specific entry by ID
func (h *UpdateHistory) GetEntry(eventID string) (*UpdateHistoryEntry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.cache {
		if h.cache[i].EventID == eventID {
			return cloneHistoryEntry(h.cache[i]), nil
		}
	}

	return nil, fmt.Errorf("entry not found: %s", eventID)
}

// ListEntries returns entries matching the filter
func (h *UpdateHistory) ListEntries(filter HistoryFilter) []UpdateHistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]UpdateHistoryEntry, 0)

	for i := len(h.cache) - 1; i >= 0; i-- {
		entry := h.cache[i]

		// Apply filters
		if filter.Status != "" && entry.Status != filter.Status {
			continue
		}
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		if filter.DeploymentType != "" && entry.DeploymentType != filter.DeploymentType {
			continue
		}

		result = append(result, entry)

		// Check limit
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result
}

// GetLatestSuccessful returns the most recent successful update
func (h *UpdateHistory) GetLatestSuccessful() (*UpdateHistoryEntry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.cache) - 1; i >= 0; i-- {
		if h.cache[i].Status == StatusSuccess {
			return cloneHistoryEntry(h.cache[i]), nil
		}
	}

	return nil, fmt.Errorf("no successful updates found")
}

// loadCache loads entries from the JSONL file into memory
func (h *UpdateHistory) loadCache() error {
	file, err := os.Open(h.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's OK
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	entries := make([]UpdateHistoryEntry, 0)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry UpdateHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Warn().Err(err).Str("line", line).Msg("Failed to parse history entry")
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Keep only the most recent entries
	if len(entries) > h.maxCache {
		entries = entries[len(entries)-h.maxCache:]
	}

	h.cache = entries

	log.Info().Int("count", len(h.cache)).Msg("Loaded update history cache")

	return nil
}

// appendToFile appends an entry to the JSONL file
func (h *UpdateHistory) appendToFile(entry UpdateHistoryEntry) error {
	file, err := os.OpenFile(h.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}

	// Sync to disk
	return file.Sync()
}

// rewriteFile rewrites the entire history file from cache
func (h *UpdateHistory) rewriteFile() error {
	// Write to temp file first
	tempPath := h.logPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	for _, entry := range h.cache {
		data, err := json.Marshal(entry)
		if err != nil {
			file.Close()
			os.Remove(tempPath)
			return err
		}

		if _, err := file.Write(append(data, '\n')); err != nil {
			file.Close()
			os.Remove(tempPath)
			return err
		}
	}

	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tempPath)
		return err
	}

	file.Close()

	// Atomic rename
	return os.Rename(tempPath, h.logPath)
}

// addToCache adds an entry to the in-memory cache
func (h *UpdateHistory) addToCache(entry UpdateHistoryEntry) {
	h.cache = append(h.cache, entry)

	// Trim cache if it exceeds max size
	if len(h.cache) > h.maxCache {
		h.cache = h.cache[len(h.cache)-h.maxCache:]
	}
}

func cloneHistoryEntry(entry UpdateHistoryEntry) *UpdateHistoryEntry {
	entryCopy := entry
	if entry.Error != nil {
		errCopy := *entry.Error
		entryCopy.Error = &errCopy
	}
	return &entryCopy
}
