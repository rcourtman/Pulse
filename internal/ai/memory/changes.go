// Package memory provides operational memory for AI context.
// It tracks changes to infrastructure, remediation actions, and their outcomes
// to enable the AI to provide more informed, experience-based recommendations.
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ChangeType represents the type of infrastructure change detected
type ChangeType string

const (
	ChangeCreated   ChangeType = "created"   // New resource appeared
	ChangeDeleted   ChangeType = "deleted"   // Resource removed
	ChangeConfig    ChangeType = "config"    // Configuration changed (RAM, CPU, etc)
	ChangeStatus    ChangeType = "status"    // Status changed (started, stopped, paused)
	ChangeMigrated  ChangeType = "migrated"  // Moved to different node
	ChangeRestarted ChangeType = "restarted" // Resource was restarted
	ChangeBackedUp  ChangeType = "backed_up" // Backup completed
)

// Change represents a detected change to infrastructure
type Change struct {
	ID           string      `json:"id"`
	ResourceID   string      `json:"resource_id"`
	ResourceType string      `json:"resource_type"` // vm, container, node, storage
	ResourceName string      `json:"resource_name"`
	ChangeType   ChangeType  `json:"change_type"`
	Before       interface{} `json:"before,omitempty"`
	After        interface{} `json:"after,omitempty"`
	DetectedAt   time.Time   `json:"detected_at"`
	Description  string      `json:"description"`
}

// ResourceSnapshot captures key attributes for change detection
type ResourceSnapshot struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Node         string    `json:"node,omitempty"`
	CPUCores     int       `json:"cpu_cores,omitempty"`
	MemoryBytes  int64     `json:"memory_bytes,omitempty"`
	DiskBytes    int64     `json:"disk_bytes,omitempty"`
	LastBackup   time.Time `json:"last_backup,omitempty"`
	SnapshotTime time.Time `json:"snapshot_time"`
}

// ChangeDetector tracks infrastructure changes over time
type ChangeDetector struct {
	mu            sync.RWMutex
	previousState map[string]ResourceSnapshot // resourceID -> snapshot
	changes       []Change
	maxChanges    int

	// Persistence
	dataDir string

	// saveStateMu guards asynchronous save scheduling state.
	saveStateMu   sync.Mutex
	saveRunning   bool
	saveRequested bool
}

// ChangeDetectorConfig configures the change detector
type ChangeDetectorConfig struct {
	MaxChanges int    // Maximum changes to retain (default: 1000)
	DataDir    string // Directory for persistence
}

// NewChangeDetector creates a new change detector
func NewChangeDetector(cfg ChangeDetectorConfig) *ChangeDetector {
	if cfg.MaxChanges <= 0 {
		cfg.MaxChanges = 1000
	}

	d := &ChangeDetector{
		previousState: make(map[string]ResourceSnapshot),
		changes:       make([]Change, 0),
		maxChanges:    cfg.MaxChanges,
		dataDir:       cfg.DataDir,
	}

	// Load existing changes from disk
	if cfg.DataDir != "" {
		if err := d.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load change history from disk")
		} else if len(d.changes) > 0 {
			log.Info().Int("count", len(d.changes)).Msg("Loaded change history from disk")
		}
	}

	return d
}

// DetectChanges compares current snapshots to previous state and detects changes
func (d *ChangeDetector) DetectChanges(currentSnapshots []ResourceSnapshot) []Change {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	var newChanges []Change

	// Track which resources we've seen in current snapshot
	currentIDs := make(map[string]bool)

	for _, current := range currentSnapshots {
		currentIDs[current.ID] = true

		prev, exists := d.previousState[current.ID]
		if !exists {
			// New resource
			change := Change{
				ID:           generateChangeID(),
				ResourceID:   current.ID,
				ResourceType: current.Type,
				ResourceName: current.Name,
				ChangeType:   ChangeCreated,
				After:        current,
				DetectedAt:   now,
				Description:  formatCreateDescription(current),
			}
			newChanges = append(newChanges, change)
		} else {
			// Check for changes
			changes := d.detectResourceChanges(prev, current, now)
			newChanges = append(newChanges, changes...)
		}

		// Update previous state
		d.previousState[current.ID] = current
	}

	// Check for deleted resources
	for id, prev := range d.previousState {
		if !currentIDs[id] {
			change := Change{
				ID:           generateChangeID(),
				ResourceID:   id,
				ResourceType: prev.Type,
				ResourceName: prev.Name,
				ChangeType:   ChangeDeleted,
				Before:       prev,
				DetectedAt:   now,
				Description:  formatDeleteDescription(prev),
			}
			newChanges = append(newChanges, change)
			delete(d.previousState, id)
		}
	}

	// Store new changes
	if len(newChanges) > 0 {
		d.changes = append(d.changes, newChanges...)
		d.trimChanges()

		d.requestAsyncSave()
	}

	return newChanges
}

func (d *ChangeDetector) requestAsyncSave() {
	if d.dataDir == "" {
		return
	}

	d.saveStateMu.Lock()
	d.saveRequested = true
	if d.saveRunning {
		d.saveStateMu.Unlock()
		return
	}
	d.saveRunning = true
	d.saveStateMu.Unlock()

	go d.runSaveLoop()
}

func (d *ChangeDetector) runSaveLoop() {
	for {
		d.saveStateMu.Lock()
		shouldSave := d.saveRequested
		d.saveRequested = false
		if !shouldSave {
			d.saveRunning = false
			d.saveStateMu.Unlock()
			return
		}
		d.saveStateMu.Unlock()

		if err := d.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to save change history")
		}
	}
}

// detectResourceChanges checks for changes between two snapshots of the same resource
func (d *ChangeDetector) detectResourceChanges(prev, current ResourceSnapshot, now time.Time) []Change {
	var changes []Change

	// Status change
	if prev.Status != current.Status {
		change := Change{
			ID:           generateChangeID(),
			ResourceID:   current.ID,
			ResourceType: current.Type,
			ResourceName: current.Name,
			ChangeType:   ChangeStatus,
			Before:       prev.Status,
			After:        current.Status,
			DetectedAt:   now,
			Description:  formatStatusDescription(current.Name, prev.Status, current.Status),
		}
		changes = append(changes, change)
	}

	// Node change (migration)
	if prev.Node != "" && current.Node != "" && prev.Node != current.Node {
		change := Change{
			ID:           generateChangeID(),
			ResourceID:   current.ID,
			ResourceType: current.Type,
			ResourceName: current.Name,
			ChangeType:   ChangeMigrated,
			Before:       prev.Node,
			After:        current.Node,
			DetectedAt:   now,
			Description:  formatMigrationDescription(current.Name, prev.Node, current.Node),
		}
		changes = append(changes, change)
	}

	// CPU change
	if prev.CPUCores > 0 && current.CPUCores > 0 && prev.CPUCores != current.CPUCores {
		change := Change{
			ID:           generateChangeID(),
			ResourceID:   current.ID,
			ResourceType: current.Type,
			ResourceName: current.Name,
			ChangeType:   ChangeConfig,
			Before:       map[string]int{"cpu_cores": prev.CPUCores},
			After:        map[string]int{"cpu_cores": current.CPUCores},
			DetectedAt:   now,
			Description:  formatCPUChangeDescription(current.Name, prev.CPUCores, current.CPUCores),
		}
		changes = append(changes, change)
	}

	// Memory change (significant change > 5%)
	if prev.MemoryBytes > 0 && current.MemoryBytes > 0 {
		pctChange := float64(current.MemoryBytes-prev.MemoryBytes) / float64(prev.MemoryBytes)
		if pctChange > 0.05 || pctChange < -0.05 {
			change := Change{
				ID:           generateChangeID(),
				ResourceID:   current.ID,
				ResourceType: current.Type,
				ResourceName: current.Name,
				ChangeType:   ChangeConfig,
				Before:       map[string]int64{"memory_bytes": prev.MemoryBytes},
				After:        map[string]int64{"memory_bytes": current.MemoryBytes},
				DetectedAt:   now,
				Description:  formatMemoryChangeDescription(current.Name, prev.MemoryBytes, current.MemoryBytes),
			}
			changes = append(changes, change)
		}
	}

	// Backup completed
	if !prev.LastBackup.IsZero() && !current.LastBackup.IsZero() &&
		current.LastBackup.After(prev.LastBackup) {
		change := Change{
			ID:           generateChangeID(),
			ResourceID:   current.ID,
			ResourceType: current.Type,
			ResourceName: current.Name,
			ChangeType:   ChangeBackedUp,
			Before:       prev.LastBackup,
			After:        current.LastBackup,
			DetectedAt:   now,
			Description:  formatBackupDescription(current.Name, current.LastBackup),
		}
		changes = append(changes, change)
	}

	return changes
}

// GetChangesForResource returns recent changes for a specific resource
func (d *ChangeDetector) GetChangesForResource(resourceID string, limit int) []Change {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []Change
	// Iterate in reverse to get most recent first
	for i := len(d.changes) - 1; i >= 0 && len(result) < limit; i-- {
		if d.changes[i].ResourceID == resourceID {
			result = append(result, d.changes[i])
		}
	}
	return result
}

// GetRecentChanges returns the most recent changes across all resources
func (d *ChangeDetector) GetRecentChanges(limit int, since time.Time) []Change {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []Change
	for i := len(d.changes) - 1; i >= 0 && len(result) < limit; i-- {
		if d.changes[i].DetectedAt.After(since) {
			result = append(result, d.changes[i])
		}
	}
	return result
}

// GetChangesSummary returns a formatted summary of recent changes for AI context
func (d *ChangeDetector) GetChangesSummary(since time.Time, maxChanges int) string {
	changes := d.GetRecentChanges(maxChanges, since)
	if len(changes) == 0 {
		return ""
	}

	var result string
	for _, c := range changes {
		ago := time.Since(c.DetectedAt)
		result += "- " + c.Description + " (" + formatDuration(ago) + " ago)\n"
	}
	return result
}

// trimChanges removes old changes beyond maxChanges
func (d *ChangeDetector) trimChanges() {
	if len(d.changes) > d.maxChanges {
		// Keep most recent
		d.changes = d.changes[len(d.changes)-d.maxChanges:]
	}
}

// saveToDisk persists changes to JSON file
func (d *ChangeDetector) saveToDisk() error {
	if d.dataDir == "" {
		return nil
	}

	d.mu.RLock()
	changes := make([]Change, len(d.changes))
	copy(changes, d.changes)
	d.mu.RUnlock()

	path := filepath.Join(d.dataDir, "ai_changes.json")
	data, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads changes from JSON file
func (d *ChangeDetector) loadFromDisk() error {
	if d.dataDir == "" {
		return nil
	}

	path := filepath.Join(d.dataDir, "ai_changes.json")
	if st, err := os.Stat(path); err == nil {
		const maxOnDiskBytes = 10 << 20 // 10 MiB safety cap
		if st.Size() > maxOnDiskBytes {
			return fmt.Errorf("change history file too large (%d bytes)", st.Size())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var changes []Change
	if err := json.Unmarshal(data, &changes); err != nil {
		return err
	}

	// Sort by time
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].DetectedAt.Before(changes[j].DetectedAt)
	})

	d.changes = changes
	d.trimChanges()
	return nil
}

// Helper functions

var changeCounter atomic.Uint64

func generateChangeID() string {
	count := changeCounter.Add(1)
	return time.Now().Format("20060102150405") + "-" + intToString(int(count%1000))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return formatUnit(int(d.Minutes()), "minute")
	}
	if d < 24*time.Hour {
		return formatUnit(int(d.Hours()), "hour")
	}
	return formatUnit(int(d.Hours()/24), "day")
}

func formatUnit(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return intToString(n) + " " + unit + "s"
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/GB) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/MB) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/KB) + " KB"
	default:
		return intToString(int(bytes)) + " B"
	}
}

func formatFloat(v float64) string {
	// Simple formatting without fmt dependency
	whole := int(v)
	frac := int((v - float64(whole)) * 10)
	if frac == 0 {
		return intToString(whole)
	}
	return intToString(whole) + "." + string(rune('0'+frac))
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var result string
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func formatCreateDescription(r ResourceSnapshot) string {
	return r.Type + " '" + r.Name + "' created"
}

func formatDeleteDescription(r ResourceSnapshot) string {
	return r.Type + " '" + r.Name + "' deleted"
}

func formatStatusDescription(name, before, after string) string {
	return "'" + name + "' status changed: " + before + " → " + after
}

func formatMigrationDescription(name, fromNode, toNode string) string {
	return "'" + name + "' migrated from " + fromNode + " to " + toNode
}

func formatCPUChangeDescription(name string, before, after int) string {
	direction := "increased"
	if after < before {
		direction = "decreased"
	}
	return "'" + name + "' CPU " + direction + ": " + intToString(before) + " → " + intToString(after) + " cores"
}

func formatMemoryChangeDescription(name string, before, after int64) string {
	direction := "increased"
	if after < before {
		direction = "decreased"
	}
	return "'" + name + "' memory " + direction + ": " + formatBytes(before) + " → " + formatBytes(after)
}

func formatBackupDescription(name string, backupTime time.Time) string {
	return "'" + name + "' backup completed at " + backupTime.Format("2006-01-02 15:04")
}
