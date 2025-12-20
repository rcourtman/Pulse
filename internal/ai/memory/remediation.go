package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

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
		dataDir:    cfg.DataDir,
	}

	// Load existing records from disk
	if cfg.DataDir != "" {
		if err := r.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load remediation log from disk")
		} else if len(r.records) > 0 {
			log.Info().Int("count", len(r.records)).Msg("Loaded remediation log from disk")
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
			log.Warn().Err(err).Msg("Failed to save remediation log")
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

	r.Log(RemediationRecord{
		ResourceID:   resourceID,
		ResourceType: resourceType,
		ResourceName: resourceName,
		FindingID:    findingID,
		Problem:      problem,
		Action:       command,
		Output:       truncateOutput(output, 1000),
		Outcome:      outcome,
		Automatic:    automatic,
	})
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

// GetSimilar finds remediation records with similar problems
func (r *RemediationLog) GetSimilar(problem string, limit int) []RemediationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Simple keyword-based matching
	keywords := extractKeywords(problem)
	if len(keywords) == 0 {
		return nil
	}

	type scored struct {
		record RemediationRecord
		score  int
	}

	var matches []scored
	for _, record := range r.records {
		recordKeywords := extractKeywords(record.Problem)
		score := countMatches(keywords, recordKeywords)
		if score > 0 {
			matches = append(matches, scored{record: record, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	var result []RemediationRecord
	for i := 0; i < len(matches) && len(result) < limit; i++ {
		result = append(result, matches[i].record)
	}
	return result
}

// GetSuccessfulRemediations returns successful remediations for similar problems
func (r *RemediationLog) GetSuccessfulRemediations(problem string, limit int) []RemediationRecord {
	similar := r.GetSimilar(problem, limit*3) // Get more to filter
	var result []RemediationRecord
	for _, rec := range similar {
		if rec.Outcome == OutcomeResolved || rec.Outcome == OutcomePartial {
			result = append(result, rec)
			if len(result) >= limit {
				break
			}
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
		result += "- " + formatDuration(ago) + " ago: " + rec.Problem + "\n"
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

	path := filepath.Join(r.dataDir, "ai_remediations.json")
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
	if r.dataDir == "" {
		return nil
	}

	path := filepath.Join(r.dataDir, "ai_remediations.json")
	if st, err := os.Stat(path); err == nil {
		const maxOnDiskBytes = 10 << 20 // 10 MiB safety cap
		if st.Size() > maxOnDiskBytes {
			return fmt.Errorf("remediation history file too large (%d bytes)", st.Size())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var records []RemediationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	// Sort by timestamp
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

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

func extractKeywords(text string) []string {
	// Simple keyword extraction - split by common delimiters
	// In production, this could use NLP or embeddings
	var keywords []string
	var current string

	for _, c := range text {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			current += string(c)
		} else {
			if len(current) > 3 { // Only keep words > 3 chars
				keywords = append(keywords, current)
			}
			current = ""
		}
	}
	if len(current) > 3 {
		keywords = append(keywords, current)
	}

	return keywords
}

func countMatches(a, b []string) int {
	bSet := make(map[string]bool)
	for _, s := range b {
		bSet[s] = true
	}

	count := 0
	for _, s := range a {
		if bSet[s] {
			count++
		}
	}
	return count
}
