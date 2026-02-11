// Package learning provides feedback learning capabilities for AI patrol.
// It tracks user actions on findings and learns patterns to improve future patrol runs.
package learning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// UserAction represents a user action on a finding
type UserAction string

const (
	ActionDismissNotAnIssue   UserAction = "dismiss_not_an_issue"
	ActionDismissExpected     UserAction = "dismiss_expected"
	ActionDismissWillFixLater UserAction = "dismiss_will_fix_later"
	ActionSnooze              UserAction = "snooze"
	ActionAcknowledge         UserAction = "acknowledge"
	ActionQuickFix            UserAction = "quick_fix"
	ActionIgnore              UserAction = "ignore" // No action taken after viewing
	ActionThumbsUp            UserAction = "thumbs_up"
	ActionThumbsDown          UserAction = "thumbs_down"
)

// FeedbackSignal represents what the system learned from a user action
type FeedbackSignal struct {
	// Severity adjustment (-1 = lower, 0 = correct, 1 = raise)
	SeverityAdjustment int
	// Confidence in this signal (0-1)
	Confidence float64
	// Whether this indicates a false positive
	IsFalsePositive bool
	// Whether the finding was actionable
	WasActionable bool
}

// FeedbackRecord stores feedback about a specific finding
type FeedbackRecord struct {
	ID           string        `json:"id"`
	FindingID    string        `json:"finding_id"`
	FindingKey   string        `json:"finding_key,omitempty"` // Stable key for the issue type
	ResourceID   string        `json:"resource_id"`
	Category     string        `json:"category"`
	Severity     string        `json:"severity"`
	Action       UserAction    `json:"action"`
	UserNote     string        `json:"user_note,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
	TimeToAction time.Duration `json:"time_to_action,omitempty"` // Time from detection to action

	// Computed signals
	Signal FeedbackSignal `json:"signal"`
}

// ResourcePreference stores learned preferences for a resource
type ResourcePreference struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name,omitempty"`

	// Severity thresholds by category
	// If a finding is below this severity, suppress it
	SeverityThresholds map[string]string `json:"severity_thresholds,omitempty"`

	// Known acceptable states
	// e.g., "85% disk usage is normal for this storage"
	AcceptableStates map[string]float64 `json:"acceptable_states,omitempty"`

	// Notes from user dismissals
	Notes []string `json:"notes,omitempty"`

	// Statistics
	TotalFindings     int     `json:"total_findings"`
	DismissedCount    int     `json:"dismissed_count"`
	ActionedCount     int     `json:"actioned_count"`
	FalsePositiveRate float64 `json:"false_positive_rate"`

	LastUpdated time.Time `json:"last_updated"`
}

// CategoryPreference stores learned preferences by category
type CategoryPreference struct {
	Category string `json:"category"`

	// Severity weight adjustment (0.5 = half importance, 2.0 = double importance)
	SeverityWeight float64 `json:"severity_weight"`

	// Statistics
	TotalFindings       int           `json:"total_findings"`
	ActionedCount       int           `json:"actioned_count"`
	DismissedCount      int           `json:"dismissed_count"`
	AverageTimeToAction time.Duration `json:"avg_time_to_action,omitempty"`
	ActionRate          float64       `json:"action_rate"` // How often user takes action

	LastUpdated time.Time `json:"last_updated"`
}

// LearningStore stores and manages learning data
type LearningStore struct {
	mu sync.RWMutex

	// Feedback records (keyed by finding ID)
	feedbackRecords map[string]*FeedbackRecord

	// Aggregated learning
	resourcePreferences map[string]*ResourcePreference
	categoryPreferences map[string]*CategoryPreference

	// Configuration
	dataDir       string
	maxRecords    int
	retentionDays int

	// State
	dirty bool
}

// LearningStoreConfig configures the learning store
type LearningStoreConfig struct {
	DataDir       string
	MaxRecords    int
	RetentionDays int
}

// DefaultLearningStoreConfig returns sensible defaults
func DefaultLearningStoreConfig() LearningStoreConfig {
	return LearningStoreConfig{
		MaxRecords:    10000,
		RetentionDays: 90,
	}
}

// NewLearningStore creates a new learning store
func NewLearningStore(cfg LearningStoreConfig) *LearningStore {
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = 10000
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 90
	}

	store := &LearningStore{
		feedbackRecords:     make(map[string]*FeedbackRecord),
		resourcePreferences: make(map[string]*ResourcePreference),
		categoryPreferences: make(map[string]*CategoryPreference),
		dataDir:             cfg.DataDir,
		maxRecords:          cfg.MaxRecords,
		retentionDays:       cfg.RetentionDays,
	}

	// Load existing data
	if cfg.DataDir != "" {
		if err := store.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load learning data from disk")
		} else {
			log.Info().
				Int("feedback_records", len(store.feedbackRecords)).
				Int("resource_prefs", len(store.resourcePreferences)).
				Msg("Loaded learning data from disk")
		}
	}

	return store
}

// RecordFeedback records user feedback on a finding
func (s *LearningStore) RecordFeedback(record FeedbackRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID if not set
	if record.ID == "" {
		record.ID = fmt.Sprintf("fb-%s-%d", record.FindingID, time.Now().UnixNano())
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// Compute feedback signal
	record.Signal = computeFeedbackSignal(record.Action)

	// Store record
	s.feedbackRecords[record.ID] = &record

	// Update resource preferences
	s.updateResourcePreferences(&record)

	// Update category preferences
	s.updateCategoryPreferences(&record)

	// Mark as dirty for persistence
	s.dirty = true

	log.Debug().
		Str("finding_id", record.FindingID).
		Str("action", string(record.Action)).
		Msg("Recorded learning feedback")

	// Trigger async save
	go s.saveIfDirty()
}

// computeFeedbackSignal derives a learning signal from a user action
func computeFeedbackSignal(action UserAction) FeedbackSignal {
	switch action {
	case ActionDismissNotAnIssue:
		return FeedbackSignal{
			SeverityAdjustment: -1,
			Confidence:         0.9,
			IsFalsePositive:    true,
			WasActionable:      false,
		}
	case ActionDismissExpected:
		return FeedbackSignal{
			SeverityAdjustment: -1,
			Confidence:         0.7,
			IsFalsePositive:    false, // Expected behavior isn't false positive
			WasActionable:      false,
		}
	case ActionDismissWillFixLater:
		return FeedbackSignal{
			SeverityAdjustment: 0,
			Confidence:         0.8,
			IsFalsePositive:    false,
			WasActionable:      true,
		}
	case ActionSnooze:
		return FeedbackSignal{
			SeverityAdjustment: 0,
			Confidence:         0.5,
			IsFalsePositive:    false,
			WasActionable:      false,
		}
	case ActionAcknowledge:
		return FeedbackSignal{
			SeverityAdjustment: 0,
			Confidence:         0.6,
			IsFalsePositive:    false,
			WasActionable:      true,
		}
	case ActionQuickFix:
		return FeedbackSignal{
			SeverityAdjustment: 0,
			Confidence:         0.95,
			IsFalsePositive:    false,
			WasActionable:      true,
		}
	case ActionIgnore:
		return FeedbackSignal{
			SeverityAdjustment: -1,
			Confidence:         0.3, // Low confidence - might just not have gotten to it
			IsFalsePositive:    false,
			WasActionable:      false,
		}
	case ActionThumbsUp:
		return FeedbackSignal{
			SeverityAdjustment: 0,
			Confidence:         0.9,
			IsFalsePositive:    false,
			WasActionable:      true,
		}
	case ActionThumbsDown:
		return FeedbackSignal{
			SeverityAdjustment: -1,
			Confidence:         0.85,
			IsFalsePositive:    true,
			WasActionable:      false,
		}
	default:
		return FeedbackSignal{
			Confidence: 0.5,
		}
	}
}

// updateResourcePreferences updates preferences for a resource based on feedback
func (s *LearningStore) updateResourcePreferences(record *FeedbackRecord) {
	pref, ok := s.resourcePreferences[record.ResourceID]
	if !ok {
		pref = &ResourcePreference{
			ResourceID:         record.ResourceID,
			SeverityThresholds: make(map[string]string),
			AcceptableStates:   make(map[string]float64),
			Notes:              make([]string, 0),
		}
		s.resourcePreferences[record.ResourceID] = pref
	}

	pref.TotalFindings++
	pref.LastUpdated = time.Now()

	if record.Signal.WasActionable {
		pref.ActionedCount++
	}
	if record.Signal.IsFalsePositive {
		pref.DismissedCount++
	}

	// Update false positive rate
	if pref.TotalFindings > 0 {
		pref.FalsePositiveRate = float64(pref.DismissedCount) / float64(pref.TotalFindings)
	}

	// Store user notes
	if record.UserNote != "" {
		// Limit notes to prevent unbounded growth
		if len(pref.Notes) >= 10 {
			pref.Notes = pref.Notes[1:]
		}
		pref.Notes = append(pref.Notes, record.UserNote)
	}

	// If consistently dismissing a category, adjust threshold
	if record.Signal.IsFalsePositive {
		// Record that this severity level is too sensitive for this resource+category
		key := record.Category
		pref.SeverityThresholds[key] = record.Severity
	}
}

// updateCategoryPreferences updates preferences for a category based on feedback
func (s *LearningStore) updateCategoryPreferences(record *FeedbackRecord) {
	pref, ok := s.categoryPreferences[record.Category]
	if !ok {
		pref = &CategoryPreference{
			Category:       record.Category,
			SeverityWeight: 1.0,
		}
		s.categoryPreferences[record.Category] = pref
	}

	pref.TotalFindings++
	pref.LastUpdated = time.Now()

	if record.Signal.WasActionable {
		pref.ActionedCount++
	}
	if record.Signal.IsFalsePositive {
		pref.DismissedCount++
	}

	// Update action rate
	if pref.TotalFindings > 0 {
		pref.ActionRate = float64(pref.ActionedCount) / float64(pref.TotalFindings)
	}

	// Update average time to action
	if record.TimeToAction > 0 && record.Signal.WasActionable {
		if pref.AverageTimeToAction == 0 {
			pref.AverageTimeToAction = record.TimeToAction
		} else {
			// Rolling average
			pref.AverageTimeToAction = (pref.AverageTimeToAction + record.TimeToAction) / 2
		}
	}

	// Adjust severity weight based on action rate
	// Higher action rate = more valuable findings = higher weight
	if pref.TotalFindings >= 10 {
		pref.SeverityWeight = 0.5 + pref.ActionRate // 0.5 to 1.5 range
	}
}

// GetResourcePreference returns learned preferences for a resource
func (s *LearningStore) GetResourcePreference(resourceID string) *ResourcePreference {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pref, ok := s.resourcePreferences[resourceID]; ok {
		// Return a copy
		copy := *pref
		return &copy
	}
	return nil
}

// GetCategoryPreference returns learned preferences for a category
func (s *LearningStore) GetCategoryPreference(category string) *CategoryPreference {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pref, ok := s.categoryPreferences[category]; ok {
		// Return a copy
		copy := *pref
		return &copy
	}
	return nil
}

// ShouldSuppress checks if a finding should be suppressed based on learned preferences
func (s *LearningStore) ShouldSuppress(resourceID, category, severity string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check resource-specific preferences
	if pref, ok := s.resourcePreferences[resourceID]; ok {
		// If false positive rate is high, suppress lower severity findings
		if pref.FalsePositiveRate > 0.7 && (severity == "info" || severity == "watch") {
			return true
		}

		// Check if this category+severity has been consistently dismissed
		if threshold, ok := pref.SeverityThresholds[category]; ok {
			if severityLevel(severity) <= severityLevel(threshold) {
				return true
			}
		}
	}

	return false
}

// GetSeverityWeight returns the learned severity weight for a category
func (s *LearningStore) GetSeverityWeight(category string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pref, ok := s.categoryPreferences[category]; ok {
		return pref.SeverityWeight
	}
	return 1.0
}

// FormatForContext formats learned preferences for AI prompt injection
func (s *LearningStore) FormatForContext() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.resourcePreferences) == 0 && len(s.categoryPreferences) == 0 {
		return ""
	}

	var result string
	result = "\n## Learned Preferences\n"
	result += "Based on user feedback, the following preferences have been learned:\n\n"

	// Resource-specific preferences
	for _, pref := range s.resourcePreferences {
		if pref.TotalFindings < 5 {
			continue // Not enough data
		}

		if pref.FalsePositiveRate > 0.5 {
			result += fmt.Sprintf("- %s: User considers many findings as noise (%.0f%% false positive rate)\n",
				pref.ResourceID, pref.FalsePositiveRate*100)
		}

		for category, threshold := range pref.SeverityThresholds {
			result += fmt.Sprintf("- %s: User considers %s %s warnings acceptable\n",
				pref.ResourceID, category, threshold)
		}

		if len(pref.Notes) > 0 {
			result += fmt.Sprintf("- %s: User notes: %s\n", pref.ResourceID, pref.Notes[len(pref.Notes)-1])
		}
	}

	// Category preferences
	result += "\nCategory value (by user action rate):\n"
	for _, pref := range s.categoryPreferences {
		if pref.TotalFindings < 10 {
			continue
		}
		actionPct := pref.ActionRate * 100
		if actionPct > 50 {
			result += fmt.Sprintf("- %s: High value (%.0f%% action rate)\n", pref.Category, actionPct)
		} else if actionPct < 20 {
			result += fmt.Sprintf("- %s: Low value (%.0f%% action rate)\n", pref.Category, actionPct)
		}
	}

	return result
}

// GetStatistics returns overall learning statistics
type LearningStatistics struct {
	TotalFeedbackRecords int                            `json:"total_feedback_records"`
	ResourcePreferences  int                            `json:"resource_preferences"`
	CategoryPreferences  int                            `json:"category_preferences"`
	OverallActionRate    float64                        `json:"overall_action_rate"`
	OverallFPRate        float64                        `json:"overall_false_positive_rate"`
	CategoryStats        map[string]*CategoryPreference `json:"category_stats,omitempty"`
}

// GetStatistics returns overall learning statistics
func (s *LearningStore) GetStatistics() LearningStatistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := LearningStatistics{
		TotalFeedbackRecords: len(s.feedbackRecords),
		ResourcePreferences:  len(s.resourcePreferences),
		CategoryPreferences:  len(s.categoryPreferences),
		CategoryStats:        make(map[string]*CategoryPreference),
	}

	var totalActions, totalFindings, totalFP int
	for _, pref := range s.categoryPreferences {
		totalActions += pref.ActionedCount
		totalFindings += pref.TotalFindings
		totalFP += pref.DismissedCount
		stats.CategoryStats[pref.Category] = pref
	}

	if totalFindings > 0 {
		stats.OverallActionRate = float64(totalActions) / float64(totalFindings)
		stats.OverallFPRate = float64(totalFP) / float64(totalFindings)
	}

	return stats
}

// Cleanup removes old feedback records
func (s *LearningStore) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	removed := 0

	for id, record := range s.feedbackRecords {
		if record.Timestamp.Before(cutoff) {
			delete(s.feedbackRecords, id)
			removed++
		}
	}

	// Trim to max records if needed
	if len(s.feedbackRecords) > s.maxRecords {
		// Find oldest records and remove them
		// Simple approach: just trim the map (order isn't guaranteed but acceptable for cleanup)
		toRemove := len(s.feedbackRecords) - s.maxRecords
		for id := range s.feedbackRecords {
			if toRemove <= 0 {
				break
			}
			delete(s.feedbackRecords, id)
			toRemove--
			removed++
		}
	}

	if removed > 0 {
		s.dirty = true
		go s.saveIfDirty()
	}

	return removed
}

// saveIfDirty saves to disk if there are unsaved changes
func (s *LearningStore) saveIfDirty() {
	s.mu.Lock()
	if !s.dirty || s.dataDir == "" {
		s.mu.Unlock()
		return
	}
	s.dirty = false
	s.mu.Unlock()

	if err := s.saveToDisk(); err != nil {
		log.Warn().Err(err).Msg("Failed to save learning data")
		s.mu.Lock()
		s.dirty = true // Mark dirty again for retry
		s.mu.Unlock()
	}
}

// saveToDisk persists learning data
func (s *LearningStore) saveToDisk() error {
	if s.dataDir == "" {
		return nil
	}

	s.mu.RLock()
	data := struct {
		FeedbackRecords     map[string]*FeedbackRecord     `json:"feedback_records"`
		ResourcePreferences map[string]*ResourcePreference `json:"resource_preferences"`
		CategoryPreferences map[string]*CategoryPreference `json:"category_preferences"`
	}{
		FeedbackRecords:     s.feedbackRecords,
		ResourcePreferences: s.resourcePreferences,
		CategoryPreferences: s.categoryPreferences,
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	path := filepath.Join(s.dataDir, "ai_learning.json")
	tmpPath := path + ".tmp"
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// loadFromDisk loads learning data
func (s *LearningStore) loadFromDisk() error {
	if s.dataDir == "" {
		return nil
	}

	path := filepath.Join(s.dataDir, "ai_learning.json")
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data struct {
		FeedbackRecords     map[string]*FeedbackRecord     `json:"feedback_records"`
		ResourcePreferences map[string]*ResourcePreference `json:"resource_preferences"`
		CategoryPreferences map[string]*CategoryPreference `json:"category_preferences"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	if data.FeedbackRecords != nil {
		s.feedbackRecords = data.FeedbackRecords
	}
	if data.ResourcePreferences != nil {
		s.resourcePreferences = data.ResourcePreferences
	}
	if data.CategoryPreferences != nil {
		s.categoryPreferences = data.CategoryPreferences
	}

	return nil
}

// ForceSave immediately saves learning data
func (s *LearningStore) ForceSave() error {
	s.mu.Lock()
	s.dirty = false
	s.mu.Unlock()
	return s.saveToDisk()
}

// Helper function to convert severity to numeric level
func severityLevel(severity string) int {
	switch severity {
	case "info":
		return 0
	case "watch":
		return 1
	case "warning":
		return 2
	case "critical":
		return 3
	default:
		return 0
	}
}
