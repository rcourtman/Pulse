package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// Outcome represents the result of a remediation action
type Outcome string

const (
	OutcomeResolved Outcome = "resolved" // Problem was fixed
	OutcomePartial  Outcome = "partial"  // Partially fixed
	OutcomeFailed   Outcome = "failed"   // Action failed
	OutcomeUnknown  Outcome = "unknown"  // Outcome not determined
)

// RollbackInfo contains information about reversibility of an action.
type RollbackInfo struct {
	Reversible   bool       `json:"reversible"`             // Whether this action can be undone
	RollbackCmd  string     `json:"rollbackCmd,omitempty"`  // Command to undo
	PreState     string     `json:"preState,omitempty"`     // JSON snapshot of state before action
	RolledBack   bool       `json:"rolledBack"`             // Whether this was rolled back
	RolledBackAt *time.Time `json:"rolledBackAt,omitempty"` // When it was rolled back
	RolledBackBy string     `json:"rolledBackBy,omitempty"` // Who rolled it back
	RollbackID   string     `json:"rollbackId,omitempty"`   // ID of the rollback remediation record
}

// RemediationRecord represents a logged remediation action
type RemediationRecord struct {
	ID           string        `json:"id"`
	Timestamp    time.Time     `json:"timestamp"`
	ResourceID   string        `json:"resource_id"`
	ResourceType string        `json:"resource_type,omitempty"`
	ResourceName string        `json:"resource_name,omitempty"`
	FindingID    string        `json:"finding_id,omitempty"` // Linked AI finding if any
	Problem      string        `json:"problem"`              // What was wrong (user's original message)
	Summary      string        `json:"summary,omitempty"`    // AI-generated summary of what was achieved
	Action       string        `json:"action"`               // What was done (command or action)
	Output       string        `json:"output,omitempty"`     // Command output if any
	Outcome      Outcome       `json:"outcome"`              // Did it work?
	Duration     time.Duration `json:"duration,omitempty"`   // How long until resolved
	Note         string        `json:"note,omitempty"`       // Optional user/AI note
	Automatic    bool          `json:"automatic"`            // Was this triggered automatically by AI
	Rollback     *RollbackInfo `json:"rollback,omitempty"`   // Rollback information (Pro feature)
	IsRollback   bool          `json:"isRollback,omitempty"` // True if this is a rollback of another action
	RollbackOf   string        `json:"rollbackOf,omitempty"` // ID of original action if this is a rollback
}

// RemediationLog stores remediation history
type RemediationLog struct {
	mu         sync.RWMutex
	records    []RemediationRecord
	maxRecords int

	// Persistence
	dataDir string
}

// RemediationLogConfig configures the remediation log
type RemediationLogConfig struct {
	MaxRecords int
	DataDir    string
}

// NewRemediationLog creates a new remediation log
func NewRemediationLog(cfg RemediationLogConfig) *RemediationLog {
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = 500
	}

	r := &RemediationLog{
		records:    make([]RemediationRecord, 0),
		maxRecords: cfg.MaxRecords,
		dataDir:    normalizeOptionalMemoryDataDir(cfg.DataDir),
	}

	// Load existing records from disk
	if cfg.DataDir != "" {
		if err := r.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to load remediation log from disk")
		} else if len(r.records) > 0 {
			log.Info().Int("count", len(r.records)).Msg("loaded remediation log from disk")
		}
	}

	return r
}

// Log records a remediation action
func (r *RemediationLog) Log(record RemediationRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record.ID == "" {
		record.ID = generateRecordID()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	r.records = append(r.records, record)
	r.trimRecords()

	// Persist asynchronously
	go func() {
		if err := r.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to save remediation log")
		}
	}()

	return nil
}

// LogCommand is a convenience method for logging a command execution
func (r *RemediationLog) LogCommand(resourceID, resourceType, resourceName, findingID, problem, command, output string, success bool, automatic bool) {
	outcome := OutcomeUnknown
	if success {
		outcome = OutcomeResolved
	} else {
		outcome = OutcomeFailed
	}

	if err := r.Log(RemediationRecord{
		ResourceID:   resourceID,
		ResourceType: resourceType,
		ResourceName: resourceName,
		FindingID:    findingID,
		Problem:      problem,
		Action:       command,
		Output:       truncateOutput(output, 1000),
		Outcome:      outcome,
		Automatic:    automatic,
	}); err != nil {
		log.Warn().Err(err).Msg("failed to log remediation command")
	}
}

// GetForResource returns remediation history for a specific resource
func (r *RemediationLog) GetForResource(resourceID string, limit int) []RemediationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []RemediationRecord
	// Iterate in reverse to get most recent first
	for i := len(r.records) - 1; i >= 0 && len(result) < limit; i-- {
		if r.records[i].ResourceID == resourceID {
			result = append(result, r.records[i])
		}
	}
	return result
}

// GetForFinding returns remediation history linked to a specific finding
func (r *RemediationLog) GetForFinding(findingID string, limit int) []RemediationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []RemediationRecord
	for i := len(r.records) - 1; i >= 0 && len(result) < limit; i-- {
		if r.records[i].FindingID == findingID {
			result = append(result, r.records[i])
		}
	}
	return result
}

// GetRecentRemediations returns the most recent remediations
func (r *RemediationLog) GetRecentRemediations(limit int, since time.Time) []RemediationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []RemediationRecord
	for i := len(r.records) - 1; i >= 0 && len(result) < limit; i-- {
		if r.records[i].Timestamp.After(since) {
			result = append(result, r.records[i])
		}
	}
	return result
}

// GetRecentRemediationStats returns remediation stats for actions since the given time.
func (r *RemediationLog) GetRecentRemediationStats(since time.Time) map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]int{
		"total":     0,
		"resolved":  0,
		"partial":   0,
		"failed":    0,
		"unknown":   0,
		"automatic": 0,
		"manual":    0,
	}

	for _, rec := range r.records {
		if rec.Timestamp.Before(since) {
			continue
		}
		stats["total"]++
		switch rec.Outcome {
		case OutcomeResolved:
			stats["resolved"]++
		case OutcomePartial:
			stats["partial"]++
		case OutcomeFailed:
			stats["failed"]++
		default:
			stats["unknown"]++
		}
		if rec.Automatic {
			stats["automatic"]++
		} else {
			stats["manual"]++
		}
	}

	return stats
}

// FormatForContext creates AI-consumable summary of remediation history
func (r *RemediationLog) FormatForContext(resourceID string, limit int) string {
	records := r.GetForResource(resourceID, limit)
	if len(records) == 0 {
		return ""
	}

	var result string
	for _, rec := range records {
		ago := time.Since(rec.Timestamp)
		outcomeStr := string(rec.Outcome)
		result += "- " + utils.FormatDurationAgo(ago) + ": " + rec.Problem + "\n"
		result += "  Action: " + rec.Action + " (" + outcomeStr + ")\n"
		if rec.Note != "" {
			result += "  Note: " + rec.Note + "\n"
		}
	}
	return result
}

// GetRemediationStats returns statistics about remediation effectiveness
func (r *RemediationLog) GetRemediationStats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]int{
		"total":    len(r.records),
		"resolved": 0,
		"partial":  0,
		"failed":   0,
		"unknown":  0,
	}

	for _, rec := range r.records {
		switch rec.Outcome {
		case OutcomeResolved:
			stats["resolved"]++
		case OutcomePartial:
			stats["partial"]++
		case OutcomeFailed:
			stats["failed"]++
		default:
			stats["unknown"]++
		}
	}

	return stats
}

// trimRecords removes old records beyond maxRecords
func (r *RemediationLog) trimRecords() {
	if len(r.records) > r.maxRecords {
		r.records = r.records[len(r.records)-r.maxRecords:]
	}
}

// saveToDisk persists records to JSON file
func (r *RemediationLog) saveToDisk() error {
	if r.dataDir == "" {
		return nil
	}

	r.mu.RLock()
	records := make([]RemediationRecord, len(r.records))
	copy(records, r.records)
	r.mu.RUnlock()

	path, err := memoryPersistencePath(r.dataDir, remediationHistoryFileName)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads records from JSON file
func (r *RemediationLog) loadFromDisk() error {
	records, ok, err := loadMemoryHistory(r.dataDir, remediationHistoryFileName, "remediation history", func(a, b RemediationRecord) bool {
		return a.Timestamp.Before(b.Timestamp)
	})
	if err != nil || !ok {
		return err
	}

	r.records = records
	r.trimRecords()
	return nil
}

// Helper functions

var recordCounter int64

func generateRecordID() string {
	recordCounter++
	return "rem-" + time.Now().Format("20060102150405") + "-" + intToString(int(recordCounter%1000))
}

func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen-3] + "..."
}

// GetByID returns a remediation record by its ID.
func (r *RemediationLog) GetByID(id string) (*RemediationRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.records {
		if r.records[i].ID == id {
			return &r.records[i], true
		}
	}
	return nil, false
}

// MarkRolledBack marks a remediation record as rolled back.
func (r *RemediationLog) MarkRolledBack(id, rollbackID, username string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.records {
		if r.records[i].ID == id {
			if r.records[i].Rollback == nil {
				r.records[i].Rollback = &RollbackInfo{}
			}
			now := time.Now()
			r.records[i].Rollback.RolledBack = true
			r.records[i].Rollback.RolledBackAt = &now
			r.records[i].Rollback.RolledBackBy = username
			r.records[i].Rollback.RollbackID = rollbackID

			// Persist
			go func() {
				if err := r.saveToDisk(); err != nil {
					log.Warn().Err(err).Msg("failed to save remediation log after rollback")
				}
			}()

			return nil
		}
	}
	return fmt.Errorf("remediation record not found: %s", id)
}

// GetRollbackable returns remediations that can be rolled back.
func (r *RemediationLog) GetRollbackable(limit int) []RemediationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []RemediationRecord
	for i := len(r.records) - 1; i >= 0 && len(result) < limit; i-- {
		rec := r.records[i]
		// Must have rollback info, be reversible, and not already rolled back
		if rec.Rollback != nil && rec.Rollback.Reversible && !rec.Rollback.RolledBack && !rec.IsRollback {
			result = append(result, rec)
		}
	}
	return result
}
