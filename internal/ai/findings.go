// Package ai provides AI-powered infrastructure monitoring and investigation.
package ai

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// FindingSeverity represents how urgent a finding is
type FindingSeverity string

const (
	// FindingSeverityInfo is informational - user may want to know
	FindingSeverityInfo FindingSeverity = "info"
	// FindingSeverityWatch means something to keep an eye on
	FindingSeverityWatch FindingSeverity = "watch"
	// FindingSeverityWarning means action should be taken soon
	FindingSeverityWarning FindingSeverity = "warning"
	// FindingSeverityCritical means immediate action needed
	FindingSeverityCritical FindingSeverity = "critical"
)

// FindingCategory groups findings by type
type FindingCategory string

const (
	FindingCategoryPerformance FindingCategory = "performance"
	FindingCategoryCapacity    FindingCategory = "capacity"
	FindingCategoryReliability FindingCategory = "reliability"
	FindingCategoryBackup      FindingCategory = "backup"
	FindingCategorySecurity    FindingCategory = "security"
	FindingCategoryGeneral     FindingCategory = "general"
)

// Finding represents an AI-discovered insight about infrastructure
type Finding struct {
	ID             string          `json:"id"`
	Key            string          `json:"key,omitempty"` // Stable issue key for runbook matching
	Severity       FindingSeverity `json:"severity"`
	Category       FindingCategory `json:"category"`
	ResourceID     string          `json:"resource_id"`
	ResourceName   string          `json:"resource_name"`
	ResourceType   string          `json:"resource_type"` // node, vm, container, docker, storage, host, pbs, host_raid
	Node           string          `json:"node,omitempty"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Recommendation string          `json:"recommendation,omitempty"`
	Evidence       string          `json:"evidence,omitempty"` // data/commands that led to this finding
	Source         string          `json:"source,omitempty"`   // "ai-analysis" for LLM findings, empty for rule-based
	DetectedAt     time.Time       `json:"detected_at"`
	LastSeenAt     time.Time       `json:"last_seen_at"`
	ResolvedAt     *time.Time      `json:"resolved_at,omitempty"`
	AutoResolved   bool            `json:"auto_resolved"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty"`
	SnoozedUntil   *time.Time      `json:"snoozed_until,omitempty"` // Finding hidden until this time
	// Link to alert if this finding was triggered by or attached to an alert
	AlertID string `json:"alert_id,omitempty"`

	// User feedback fields - enables LLM "memory" by tracking how users respond
	// This helps prevent the LLM from repeatedly raising the same dismissed issues
	DismissedReason string `json:"dismissed_reason,omitempty"` // "not_an_issue", "expected_behavior", "will_fix_later"
	UserNote        string `json:"user_note,omitempty"`        // Freeform user explanation, included in LLM context
	TimesRaised     int    `json:"times_raised"`               // How many times this finding has been detected
	Suppressed      bool   `json:"suppressed"`                 // Permanently suppress similar findings for this resource
}

// IsActive returns true if the finding is still active (not resolved, not snoozed, not suppressed, not dismissed)
func (f *Finding) IsActive() bool {
	return f.ResolvedAt == nil && !f.IsSnoozed() && !f.Suppressed && f.DismissedReason == ""
}

// IsDismissed returns true if the user has dismissed this finding with a reason
func (f *Finding) IsDismissed() bool {
	return f.DismissedReason != ""
}

// IsSnoozed returns true if the finding is currently snoozed
func (f *Finding) IsSnoozed() bool {
	return f.SnoozedUntil != nil && time.Now().Before(*f.SnoozedUntil)
}

// IsResolved returns true if the finding has been resolved (ignores snooze)
func (f *Finding) IsResolved() bool {
	return f.ResolvedAt != nil
}

// SuppressionRule represents a user-defined rule to suppress certain AI findings
// Users can create these manually to prevent alerts before they happen
type SuppressionRule struct {
	ID           string          `json:"id"`
	ResourceID   string          `json:"resource_id,omitempty"`   // Empty means "any resource"
	ResourceName string          `json:"resource_name,omitempty"` // Human-readable name for display
	Category     FindingCategory `json:"category,omitempty"`      // Empty means "any category"
	Description  string          `json:"description"`             // User's reason, e.g., "dev VM runs hot"
	CreatedAt    time.Time       `json:"created_at"`
	CreatedFrom  string          `json:"created_from,omitempty"` // "finding" if created from a dismissed finding, "manual" if user-created
	FindingID    string          `json:"finding_id,omitempty"`   // Original finding ID if created from dismissal
}

// FindingsPersistence interface for saving/loading findings (avoids circular imports)
type FindingsPersistence interface {
	SaveFindings(findings map[string]*Finding) error
	LoadFindings() (map[string]*Finding, error)
}

// FindingsStore provides thread-safe storage for AI findings with optional persistence
type FindingsStore struct {
	mu       sync.RWMutex
	findings map[string]*Finding // keyed by ID
	// Index by resource for quick lookups
	byResource map[string][]string // resource_id -> []finding_id
	// Keep track of active findings count by severity (cached, but GetSummary calculates dynamically)
	activeCounts map[FindingSeverity]int
	// User-defined suppression rules (separate from dismissed findings)
	suppressionRules map[string]*SuppressionRule
	// Persistence layer (optional)
	persistence FindingsPersistence
	// Debounce save operations
	saveTimer    *time.Timer
	savePending  bool
	saveDebounce time.Duration
}

// NewFindingsStore creates a new findings store
func NewFindingsStore() *FindingsStore {
	return &FindingsStore{
		findings:         make(map[string]*Finding),
		byResource:       make(map[string][]string),
		activeCounts:     make(map[FindingSeverity]int),
		suppressionRules: make(map[string]*SuppressionRule),
		saveDebounce:     5 * time.Second, // Debounce saves by 5 seconds
	}
}

// SetPersistence sets the persistence layer and loads existing findings
func (s *FindingsStore) SetPersistence(p FindingsPersistence) error {
	s.mu.Lock()
	s.persistence = p
	s.mu.Unlock()

	// Load existing findings from disk
	if p != nil {
		findings, err := p.LoadFindings()
		if err != nil {
			return err
		}
		if len(findings) > 0 {
			s.mu.Lock()
			for id, f := range findings {
				s.findings[id] = f
				s.byResource[f.ResourceID] = append(s.byResource[f.ResourceID], id)
				if f.IsActive() {
					s.activeCounts[f.Severity]++
				}
			}
			s.mu.Unlock()
		}
	}
	return nil
}

// scheduleSave schedules a debounced save operation
func (s *FindingsStore) scheduleSave() {
	if s.persistence == nil {
		return
	}

	// Already have a save pending
	if s.savePending {
		return
	}

	s.savePending = true
	s.saveTimer = time.AfterFunc(s.saveDebounce, func() {
		s.mu.Lock()
		s.savePending = false
		// Make a copy for saving
		findingsCopy := make(map[string]*Finding, len(s.findings))
		for id, f := range s.findings {
			copy := *f
			findingsCopy[id] = &copy
		}
		persistence := s.persistence
		s.mu.Unlock()

		if persistence != nil {
			if err := persistence.SaveFindings(findingsCopy); err != nil {
				// Log error but don't fail - persistence is best-effort
				// (log import would create circular dep, so we silently fail)
			}
		}
	})
}

// ForceSave immediately saves findings (useful for shutdown)
func (s *FindingsStore) ForceSave() error {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.savePending = false

	findingsCopy := make(map[string]*Finding, len(s.findings))
	for id, f := range s.findings {
		copy := *f
		findingsCopy[id] = &copy
	}
	persistence := s.persistence
	s.mu.Unlock()

	if persistence != nil {
		return persistence.SaveFindings(findingsCopy)
	}
	return nil
}

// Add adds or updates a finding
// If a finding with the same ID exists, it updates LastSeenAt and increments TimesRaised
// If the finding is suppressed or dismissed, it may be skipped
// Returns true if this is a new finding
func (s *FindingsStore) Add(f *Finding) bool {
	s.mu.Lock()

	existing, exists := s.findings[f.ID]
	if exists {
		// Check if it's permanently suppressed - don't update at all
		if existing.Suppressed {
			s.mu.Unlock()
			return false
		}

		// Check if dismissed - only update if severity has escalated
		if existing.DismissedReason != "" {
			severityOrder := map[FindingSeverity]int{
				FindingSeverityInfo:     0,
				FindingSeverityWatch:    1,
				FindingSeverityWarning:  2,
				FindingSeverityCritical: 3,
			}
			// If new severity is same or lower, don't reactivate
			if severityOrder[f.Severity] <= severityOrder[existing.Severity] {
				existing.LastSeenAt = time.Now()
				existing.TimesRaised++
				s.mu.Unlock()
				s.scheduleSave()
				return false
			}
			// Severity escalated - clear dismissal and reactivate
			existing.DismissedReason = ""
			existing.UserNote = "" // Clear note since situation changed
			existing.AcknowledgedAt = nil
		}

		// Update existing finding
		existing.LastSeenAt = time.Now()
		existing.Description = f.Description
		existing.Recommendation = f.Recommendation
		existing.Evidence = f.Evidence
		existing.Title = f.Title // Update title in case LLM phrased it better
		existing.Severity = f.Severity
		existing.TimesRaised++ // Track recurrence
		s.mu.Unlock()
		s.scheduleSave()
		return false
	}

	// New finding - check if resource+category is suppressed
	if s.isSuppressedInternal(f.ResourceID, f.Category) {
		s.mu.Unlock()
		return false
	}

	// New finding
	if f.DetectedAt.IsZero() {
		f.DetectedAt = time.Now()
	}
	f.LastSeenAt = time.Now()

	s.findings[f.ID] = f
	s.byResource[f.ResourceID] = append(s.byResource[f.ResourceID], f.ID)
	if f.IsActive() {
		s.activeCounts[f.Severity]++
	}
	s.mu.Unlock()
	s.scheduleSave()

	return true
}

// Resolve marks a finding as resolved
func (s *FindingsStore) Resolve(id string, auto bool) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists || !f.IsActive() {
		s.mu.Unlock()
		return false
	}

	now := time.Now()
	f.ResolvedAt = &now
	f.AutoResolved = auto
	s.activeCounts[f.Severity]--
	s.mu.Unlock()
	s.scheduleSave()

	return true
}

// Acknowledge marks a finding as acknowledged
func (s *FindingsStore) Acknowledge(id string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	now := time.Now()
	f.AcknowledgedAt = &now
	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Snooze hides a finding for the specified duration
// Common durations: 1h, 24h, 7d (168h)
func (s *FindingsStore) Snooze(id string, duration time.Duration) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists || f.IsResolved() {
		s.mu.Unlock()
		return false
	}

	// If was previously active (not snoozed), decrement count
	if f.SnoozedUntil == nil || time.Now().After(*f.SnoozedUntil) {
		s.activeCounts[f.Severity]--
	}

	until := time.Now().Add(duration)
	f.SnoozedUntil = &until
	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Unsnooze removes the snooze from a finding, making it active again
func (s *FindingsStore) Unsnooze(id string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists || f.IsResolved() {
		s.mu.Unlock()
		return false
	}

	if f.SnoozedUntil != nil {
		f.SnoozedUntil = nil
		s.activeCounts[f.Severity]++
	}
	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Dismiss marks a finding as dismissed with a reason and optional note
// Reasons: "not_an_issue", "expected_behavior", "will_fix_later"
func (s *FindingsStore) Dismiss(id, reason, note string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	f.DismissedReason = reason
	if note != "" {
		f.UserNote = note
	}
	// Also mark as acknowledged
	now := time.Now()
	f.AcknowledgedAt = &now

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// SetUserNote updates the user note on a finding
func (s *FindingsStore) SetUserNote(id, note string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	f.UserNote = note
	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Suppress marks a finding type as permanently suppressed for a resource
// Future findings with the same resource+category will be auto-dismissed
func (s *FindingsStore) Suppress(id string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	f.Suppressed = true
	f.DismissedReason = "suppressed"
	now := time.Now()
	f.AcknowledgedAt = &now

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// IsSuppressed checks if findings of this type for this resource are suppressed
// Checks by resource+category only (not title, since LLM titles vary)
func (s *FindingsStore) IsSuppressed(resourceID string, category FindingCategory) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isSuppressedInternal(resourceID, category)
}

// isSuppressedInternal checks suppression without locking (caller must hold lock)
func (s *FindingsStore) isSuppressedInternal(resourceID string, category FindingCategory) bool {
	// Check if any finding for this resource+category is suppressed
	ids := s.byResource[resourceID]
	for _, id := range ids {
		if f, exists := s.findings[id]; exists {
			// Match by resource+category only
			if f.Suppressed && f.Category == category {
				return true
			}
		}
	}

	// Also check manual suppression rules
	for _, rule := range s.suppressionRules {
		resourceMatches := rule.ResourceID == "" || rule.ResourceID == resourceID
		categoryMatches := rule.Category == "" || rule.Category == category
		if resourceMatches && categoryMatches {
			return true
		}
	}

	return false
}

// Get returns a finding by ID
func (s *FindingsStore) Get(id string) *Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if f, exists := s.findings[id]; exists {
		// Return a copy to prevent mutations
		copy := *f
		return &copy
	}
	return nil
}

// GetByResource returns all active findings for a resource
func (s *FindingsStore) GetByResource(resourceID string) []*Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byResource[resourceID]
	result := make([]*Finding, 0, len(ids))
	for _, id := range ids {
		if f, exists := s.findings[id]; exists && f.IsActive() {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// GetActive returns all active findings, optionally filtered by severity
func (s *FindingsStore) GetActive(minSeverity FindingSeverity) []*Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	severityOrder := map[FindingSeverity]int{
		FindingSeverityInfo:     0,
		FindingSeverityWatch:    1,
		FindingSeverityWarning:  2,
		FindingSeverityCritical: 3,
	}
	minOrder := severityOrder[minSeverity]

	result := make([]*Finding, 0)
	for _, f := range s.findings {
		if f.IsActive() && severityOrder[f.Severity] >= minOrder {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// GetSummary returns a summary of active findings
// Note: This calculates counts dynamically to handle time-based snooze expiration
func (s *FindingsStore) GetSummary() FindingsSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := FindingsSummary{
		Total: len(s.findings),
	}

	// Calculate active counts dynamically since IsActive() checks time-based snooze
	for _, f := range s.findings {
		if f.IsActive() {
			switch f.Severity {
			case FindingSeverityCritical:
				summary.Critical++
			case FindingSeverityWarning:
				summary.Warning++
			case FindingSeverityWatch:
				summary.Watch++
			case FindingSeverityInfo:
				summary.Info++
			}
		}
	}

	return summary
}

// GetAll returns all findings including resolved ones (for history)
// Results can be filtered by time range using startTime parameter
func (s *FindingsStore) GetAll(startTime *time.Time) []*Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Finding, 0, len(s.findings))
	for _, f := range s.findings {
		// If startTime specified, only include findings detected after it
		if startTime != nil && f.DetectedAt.Before(*startTime) {
			continue
		}
		copy := *f
		result = append(result, &copy)
	}
	return result
}

// Cleanup removes old resolved findings
func (s *FindingsStore) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, f := range s.findings {
		if f.ResolvedAt != nil && f.ResolvedAt.Before(cutoff) {
			delete(s.findings, id)
			// Clean up resource index
			ids := s.byResource[f.ResourceID]
			for i, fid := range ids {
				if fid == id {
					s.byResource[f.ResourceID] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
			removed++
		}
	}

	return removed
}

// GetDismissedForContext returns findings that the user has dismissed/acknowledged,
// formatted for injection into LLM prompts. This is the core of the "memory" system -
// it tells the LLM what not to re-raise.
func (s *FindingsStore) GetDismissedForContext() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var suppressed, dismissed, snoozed []string

	for _, f := range s.findings {
		// Skip very old findings (more than 30 days)
		if time.Since(f.LastSeenAt) > 30*24*time.Hour {
			continue
		}

		// Collect suppressed findings
		if f.Suppressed {
			note := ""
			if f.UserNote != "" {
				note = " - User note: " + f.UserNote
			}
			suppressed = append(suppressed,
				fmt.Sprintf("- %s on %s: %s%s", f.Title, f.ResourceName, f.DismissedReason, note))
			continue
		}

		// Collect dismissed/acknowledged findings
		if f.DismissedReason != "" {
			note := ""
			if f.UserNote != "" {
				note = " - User note: " + f.UserNote
			}
			dismissed = append(dismissed,
				fmt.Sprintf("- %s on %s (%s)%s", f.Title, f.ResourceName, f.DismissedReason, note))
			continue
		}

		// Collect snoozed findings
		if f.IsSnoozed() {
			snoozed = append(snoozed,
				fmt.Sprintf("- %s on %s (snoozed until %s)",
					f.Title, f.ResourceName, f.SnoozedUntil.Format("Jan 2")))
		}
	}

	if len(suppressed) == 0 && len(dismissed) == 0 && len(snoozed) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("\n## Previous Findings - User Feedback\n")
	result.WriteString("The following findings have been addressed by the user. Do NOT re-raise these unless the situation has significantly worsened:\n\n")

	if len(suppressed) > 0 {
		result.WriteString("### Permanently Suppressed (never re-raise):\n")
		for _, s := range suppressed {
			result.WriteString(s + "\n")
		}
		result.WriteString("\n")
	}

	if len(dismissed) > 0 {
		result.WriteString("### Dismissed by User:\n")
		for _, d := range dismissed {
			result.WriteString(d + "\n")
		}
		result.WriteString("\n")
	}

	if len(snoozed) > 0 {
		result.WriteString("### Temporarily Snoozed:\n")
		for _, s := range snoozed {
			result.WriteString(s + "\n")
		}
	}

	return result.String()
}

// FindingsSummary provides a quick count of findings by severity
type FindingsSummary struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Watch    int `json:"watch"`
	Info     int `json:"info"`
	Total    int `json:"total"`
}

// HasIssues returns true if there are any warning or critical findings
func (s FindingsSummary) HasIssues() bool {
	return s.Critical > 0 || s.Warning > 0
}

// IsHealthy returns true if there are no watch, warning, or critical findings
func (s FindingsSummary) IsHealthy() bool {
	return s.Critical == 0 && s.Warning == 0 && s.Watch == 0
}

// --- Suppression Rule Management ---

// AddSuppressionRule creates a new user-defined suppression rule
func (s *FindingsStore) AddSuppressionRule(resourceID, resourceName string, category FindingCategory, description string) *SuppressionRule {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID based on resource+category
	ruleID := fmt.Sprintf("rule_%s_%s_%d", resourceID, category, time.Now().UnixNano())
	if resourceID == "" {
		ruleID = fmt.Sprintf("rule_any_%s_%d", category, time.Now().UnixNano())
	}
	if category == "" {
		ruleID = fmt.Sprintf("rule_%s_any_%d", resourceID, time.Now().UnixNano())
	}

	rule := &SuppressionRule{
		ID:           ruleID,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		Category:     category,
		Description:  description,
		CreatedAt:    time.Now(),
		CreatedFrom:  "manual",
	}

	s.suppressionRules[ruleID] = rule
	s.scheduleSave()
	return rule
}

// GetSuppressionRules returns all suppression rules (both manual and from dismissed findings)
func (s *FindingsStore) GetSuppressionRules() []*SuppressionRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rules []*SuppressionRule

	// Add explicit suppression rules
	for _, rule := range s.suppressionRules {
		rules = append(rules, rule)
	}

	// Also include suppressed findings as rules (for visibility)
	for _, f := range s.findings {
		if f.Suppressed {
			rules = append(rules, &SuppressionRule{
				ID:           "finding_" + f.ID,
				ResourceID:   f.ResourceID,
				ResourceName: f.ResourceName,
				Category:     f.Category,
				Description:  f.UserNote,
				CreatedAt:    *f.AcknowledgedAt,
				CreatedFrom:  "finding",
				FindingID:    f.ID,
			})
		}
	}

	return rules
}

// DeleteSuppressionRule removes a suppression rule
func (s *FindingsStore) DeleteSuppressionRule(ruleID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if it's an explicit rule
	if _, exists := s.suppressionRules[ruleID]; exists {
		delete(s.suppressionRules, ruleID)
		s.scheduleSave()
		return true
	}

	// Check if it's a finding-based rule (e.g., "finding_abc123")
	if strings.HasPrefix(ruleID, "finding_") {
		findingID := strings.TrimPrefix(ruleID, "finding_")
		if f, exists := s.findings[findingID]; exists && f.Suppressed {
			// Un-suppress the finding
			f.Suppressed = false
			f.DismissedReason = ""
			s.scheduleSave()
			return true
		}
	}

	return false
}

// GetDismissedFindings returns all findings that have been dismissed or suppressed
func (s *FindingsStore) GetDismissedFindings() []*Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var dismissed []*Finding
	for _, f := range s.findings {
		if f.DismissedReason != "" || f.Suppressed {
			copy := *f
			dismissed = append(dismissed, &copy)
		}
	}
	return dismissed
}

// MatchesSuppressionRule checks if a finding matches any suppression rule
func (s *FindingsStore) MatchesSuppressionRule(resourceID string, category FindingCategory) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, rule := range s.suppressionRules {
		// Check if resource matches (or rule is for "any resource")
		resourceMatches := rule.ResourceID == "" || rule.ResourceID == resourceID
		// Check if category matches (or rule is for "any category")
		categoryMatches := rule.Category == "" || rule.Category == category

		if resourceMatches && categoryMatches {
			return true
		}
	}
	return false
}
