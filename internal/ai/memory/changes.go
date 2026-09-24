// Package memory provides operational memory for AI context.
// It keeps legacy infrastructure change history, remediation actions, and their
// outcomes to enable the AI to provide more informed, experience-based
// recommendations.
package memory

import (
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

// ChangeDetector serves legacy infrastructure change history.
//
// Recent-change context comes from the unified-resource timeline; the detector
// is only its fallback. Its history (ai_changes.json) was written by releases
// that still ran change detection, which patrol dropped when it moved to
// agentic execution. The history is loaded once at construction and never
// modified afterwards, so reads need no lock.
type ChangeDetector struct {
	changes    []Change
	maxChanges int

	// dataDir holds the legacy history read by loadFromDisk.
	dataDir string
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
		changes:    make([]Change, 0),
		maxChanges: cfg.MaxChanges,
		dataDir:    normalizeOptionalMemoryDataDir(cfg.DataDir),
	}

	// Load existing changes from disk
	if cfg.DataDir != "" {
		if err := d.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to load change history from disk")
		} else if len(d.changes) > 0 {
			log.Info().Int("count", len(d.changes)).Msg("loaded change history from disk")
		}
	}

	return d
}

// GetChangesForResource returns recent changes for a specific resource
func (d *ChangeDetector) GetChangesForResource(resourceID string, limit int) []Change {
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
	return FormatRecentChangesContext(changes, false, "##")
}

// trimChanges removes old changes beyond maxChanges
func (d *ChangeDetector) trimChanges() {
	if len(d.changes) > d.maxChanges {
		// Keep most recent
		d.changes = d.changes[len(d.changes)-d.maxChanges:]
	}
}

// loadFromDisk loads the legacy change history from its JSON file
func (d *ChangeDetector) loadFromDisk() error {
	changes, ok, err := loadMemoryHistory(d.dataDir, changeHistoryFileName, "change history", func(a, b Change) bool {
		return a.DetectedAt.Before(b.DetectedAt)
	})
	if err != nil || !ok {
		return err
	}

	d.changes = changes
	d.trimChanges()
	return nil
}

// Helper functions

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
