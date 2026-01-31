// Package ai provides AI-powered infrastructure monitoring and investigation.
package ai

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Investigation limits for automatic finding investigation.
const (
	investigationCooldown    = 1 * time.Hour // Minimum time between investigation attempts
	maxInvestigationAttempts = 3             // Maximum number of investigation retries
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

// InvestigationStatus represents the current investigation state of a finding
type InvestigationStatus string

const (
	InvestigationStatusPending        InvestigationStatus = "pending"         // Queued for investigation
	InvestigationStatusRunning        InvestigationStatus = "running"         // Currently being investigated
	InvestigationStatusCompleted      InvestigationStatus = "completed"       // Investigation finished
	InvestigationStatusFailed         InvestigationStatus = "failed"          // Investigation errored out
	InvestigationStatusNeedsAttention InvestigationStatus = "needs_attention" // Requires user input
)

// InvestigationOutcome represents the result of an investigation
type InvestigationOutcome string

const (
	InvestigationOutcomeResolved       InvestigationOutcome = "resolved"        // Issue was automatically fixed
	InvestigationOutcomeFixQueued      InvestigationOutcome = "fix_queued"      // Fix identified, awaiting approval
	InvestigationOutcomeNeedsAttention InvestigationOutcome = "needs_attention" // Requires user intervention
	InvestigationOutcomeCannotFix      InvestigationOutcome = "cannot_fix"      // AI determined it cannot fix this
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
	ResolveReason  string          `json:"resolve_reason,omitempty"` // Why the finding was resolved (e.g., "No longer detected by patrol")
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

	// Investigation fields - tracks autonomous AI investigation of findings
	InvestigationSessionID string     `json:"investigation_session_id,omitempty"` // Chat session ID if being investigated
	InvestigationStatus    string     `json:"investigation_status,omitempty"`     // pending, running, completed, failed, needs_attention
	InvestigationOutcome   string     `json:"investigation_outcome,omitempty"`    // resolved, fix_queued, needs_attention, cannot_fix
	LastInvestigatedAt     *time.Time `json:"last_investigated_at,omitempty"`     // When last investigation completed
	InvestigationAttempts  int        `json:"investigation_attempts"`             // Number of investigation attempts
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

// ShouldInvestigate returns true if this finding should be automatically investigated
// based on autonomy level, severity, and investigation history.
// A finding will NOT be investigated if:
// - Already being investigated (status = running)
// - Investigated within the last hour (cooldown)
// - Already attempted 3 times (max attempts)
// - Severity is info/watch (not actionable)
// - Already resolved/dismissed/suppressed
func (f *Finding) ShouldInvestigate(autonomyLevel string) bool {
	// Only investigate if autonomy is enabled (approval or full mode)
	if autonomyLevel == "" || autonomyLevel == "monitor" {
		return false
	}

	// Don't investigate already resolved/dismissed findings
	if f.ResolvedAt != nil || f.Suppressed || f.DismissedReason != "" {
		return false
	}

	// Don't investigate snoozed findings
	if f.IsSnoozed() {
		return false
	}

	// Don't re-investigate findings where the fix was verified as failed
	// (user should review manually)
	if f.InvestigationOutcome == "fix_verification_failed" {
		return false
	}

	// Only investigate warning and critical severity (info/watch are not actionable)
	if f.Severity != FindingSeverityWarning && f.Severity != FindingSeverityCritical {
		return false
	}

	// Don't re-investigate if already running
	if f.InvestigationStatus == string(InvestigationStatusRunning) {
		return false
	}

	// Don't re-investigate if at max attempts (3)
	if f.InvestigationAttempts >= maxInvestigationAttempts {
		return false
	}

	// Don't re-investigate within cooldown period (1 hour)
	if f.LastInvestigatedAt != nil {
		if time.Since(*f.LastInvestigatedAt) < investigationCooldown {
			return false
		}
	}

	return true
}

func inferFindingResourceType(resourceID, resourceName string) string {
	joined := strings.ToLower(strings.TrimSpace(resourceID + " " + resourceName))

	switch {
	case strings.Contains(joined, "pbs") || strings.Contains(joined, "backup"):
		return "pbs"
	case strings.Contains(joined, "storage") || strings.Contains(joined, "pool") || strings.Contains(joined, "zfs"):
		return "storage"
	case strings.Contains(joined, "docker"):
		return "docker_container"
	case strings.Contains(joined, "lxc") || strings.Contains(joined, "ct") || strings.Contains(joined, "container"):
		return "container"
	case strings.Contains(joined, "vm"):
		return "vm"
	case strings.Contains(joined, "node") || strings.Contains(joined, "host"):
		return "node"
	}

	if hasFindingNumericSuffix(resourceID) {
		return "vm"
	}

	return "node"
}

func hasFindingNumericSuffix(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	sep := strings.LastIndexAny(value, ":/")
	if sep >= 0 && sep+1 < len(value) {
		value = value[sep+1:]
	}
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// IsBeingInvestigated returns true if an investigation is currently in progress
func (f *Finding) IsBeingInvestigated() bool {
	return f.InvestigationStatus == string(InvestigationStatusRunning)
}

// CanRetryInvestigation returns true if the finding can be re-investigated
func (f *Finding) CanRetryInvestigation() bool {
	// Can't retry if at max attempts
	if f.InvestigationAttempts >= maxInvestigationAttempts {
		return false
	}
	// Can't retry if still in cooldown
	if f.LastInvestigatedAt != nil {
		if time.Since(*f.LastInvestigatedAt) < investigationCooldown {
			return false
		}
	}
	// Can't retry if currently running
	if f.InvestigationStatus == string(InvestigationStatusRunning) {
		return false
	}
	return true
}

// Getter methods for investigation.AIFinding interface

func (f *Finding) GetID() string                     { return f.ID }
func (f *Finding) GetSeverity() string               { return string(f.Severity) }
func (f *Finding) GetCategory() string               { return string(f.Category) }
func (f *Finding) GetResourceID() string             { return f.ResourceID }
func (f *Finding) GetResourceName() string           { return f.ResourceName }
func (f *Finding) GetResourceType() string           { return f.ResourceType }
func (f *Finding) GetTitle() string                  { return f.Title }
func (f *Finding) GetDescription() string            { return f.Description }
func (f *Finding) GetRecommendation() string         { return f.Recommendation }
func (f *Finding) GetEvidence() string               { return f.Evidence }
func (f *Finding) GetInvestigationSessionID() string { return f.InvestigationSessionID }
func (f *Finding) GetInvestigationStatus() string    { return f.InvestigationStatus }
func (f *Finding) GetInvestigationOutcome() string   { return f.InvestigationOutcome }
func (f *Finding) GetLastInvestigatedAt() *time.Time { return f.LastInvestigatedAt }
func (f *Finding) GetInvestigationAttempts() int     { return f.InvestigationAttempts }

// Setter methods for investigation.AIFinding interface

func (f *Finding) SetInvestigationSessionID(v string) { f.InvestigationSessionID = v }
func (f *Finding) SetInvestigationStatus(v string)    { f.InvestigationStatus = v }
func (f *Finding) SetInvestigationOutcome(v string)   { f.InvestigationOutcome = v }
func (f *Finding) SetLastInvestigatedAt(v *time.Time) { f.LastInvestigatedAt = v }
func (f *Finding) SetInvestigationAttempts(v int)     { f.InvestigationAttempts = v }

// SuppressionRule represents a user-defined rule to suppress certain AI findings
// Users can create these manually to prevent alerts before they happen
type SuppressionRule struct {
	ID              string          `json:"id"`
	ResourceID      string          `json:"resource_id,omitempty"`      // Empty means "any resource"
	ResourceName    string          `json:"resource_name,omitempty"`    // Human-readable name for display
	Category        FindingCategory `json:"category,omitempty"`         // Empty means "any category"
	Description     string          `json:"description"`                // User's reason, e.g., "dev VM runs hot"
	DismissedReason string          `json:"dismissed_reason,omitempty"` // "not_an_issue", "expected_behavior", "will_fix_later"
	CreatedAt       time.Time       `json:"created_at"`
	CreatedFrom     string          `json:"created_from,omitempty"` // "finding" if suppressed, "dismissed" if just dismissed, "manual" if user-created
	FindingID       string          `json:"finding_id,omitempty"`   // Original finding ID if created from dismissal
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
	// Error tracking for persistence failures
	lastSaveError error           // Last error from save operation
	onSaveError   func(err error) // Optional callback for save errors
	lastSaveTime  time.Time       // Last successful save time
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
// This method is lock-safe and can be called without holding the store lock.
func (s *FindingsStore) scheduleSave() {
	s.mu.Lock()
	if s.persistence == nil || s.savePending {
		s.mu.Unlock()
		return
	}

	s.savePending = true
	saveDebounce := s.saveDebounce
	s.saveTimer = time.AfterFunc(saveDebounce, func() {
		s.mu.Lock()
		s.savePending = false
		// Make a copy for saving, excluding demo findings
		findingsCopy := make(map[string]*Finding, len(s.findings))
		for id, f := range s.findings {
			// Skip demo findings - they should never be persisted
			if strings.HasPrefix(id, "demo-") {
				continue
			}
			copy := *f
			findingsCopy[id] = &copy
		}
		persistence := s.persistence
		onError := s.onSaveError
		s.mu.Unlock()

		if persistence != nil {
			if err := persistence.SaveFindings(findingsCopy); err != nil {
				// Track the error for visibility
				s.mu.Lock()
				s.lastSaveError = err
				s.mu.Unlock()
				// Call error callback if set
				if onError != nil {
					onError(err)
				}
			} else {
				// Clear error and update timestamp on success
				s.mu.Lock()
				s.lastSaveError = nil
				s.lastSaveTime = time.Now()
				s.mu.Unlock()
			}
		}
	})
	s.mu.Unlock()
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
		// Skip demo findings - they should never be persisted
		if strings.HasPrefix(id, "demo-") {
			continue
		}
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

// SetOnSaveError sets a callback function that will be called when a save operation fails.
// This allows external code (e.g., logging) to be notified of persistence errors.
func (s *FindingsStore) SetOnSaveError(callback func(err error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSaveError = callback
}

// GetPersistenceStatus returns the current persistence state:
// - lastError: the most recent save error, or nil if last save succeeded
// - lastSaveTime: when findings were last successfully saved
// - hasPersistence: whether a persistence layer is configured
func (s *FindingsStore) GetPersistenceStatus() (lastError error, lastSaveTime time.Time, hasPersistence bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSaveError, s.lastSaveTime, s.persistence != nil
}

// Add adds or updates a finding
// If a finding with the same ID exists, it updates LastSeenAt and increments TimesRaised
// If the finding is suppressed or dismissed, it may be skipped
// Returns true if this is a new finding
func (s *FindingsStore) Add(f *Finding) bool {
	s.mu.Lock()

	if f.ResourceType == "" {
		f.ResourceType = inferFindingResourceType(f.ResourceID, f.ResourceName)
	}

	existing, exists := s.findings[f.ID]
	if exists {
		wasResolved := existing.ResolvedAt != nil
		if existing.ResourceType == "" {
			if f.ResourceType != "" {
				existing.ResourceType = f.ResourceType
			} else {
				existing.ResourceType = inferFindingResourceType(existing.ResourceID, existing.ResourceName)
			}
		}

		// Check if dismissed or suppressed - only update if severity has escalated
		if existing.DismissedReason != "" || existing.Suppressed {
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
			// Severity escalated - clear dismissal/suppression and reactivate
			existing.DismissedReason = ""
			existing.Suppressed = false
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
		if wasResolved {
			existing.ResolvedAt = nil
			existing.AutoResolved = false
			existing.ResolveReason = ""
			if existing.IsActive() {
				s.activeCounts[existing.Severity]++
			}
		}
		severity := existing.Severity
		s.mu.Unlock()
		s.scheduleSave()
		// Bypass debounce for warning+ findings to avoid data loss on crash
		if severity == FindingSeverityWarning || severity == FindingSeverityCritical {
			_ = s.ForceSave()
		}
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
	severity := f.Severity
	s.mu.Unlock()
	s.scheduleSave()
	// Bypass debounce for warning+ findings to avoid data loss on crash
	if severity == FindingSeverityWarning || severity == FindingSeverityCritical {
		_ = s.ForceSave()
	}

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

// ResolveWithReason marks a finding as resolved with a specific reason string.
// This is used by auto-resolution to distinguish why a finding was resolved.
func (s *FindingsStore) ResolveWithReason(id string, reason string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists || !f.IsActive() {
		s.mu.Unlock()
		return false
	}

	now := time.Now()
	f.ResolvedAt = &now
	f.AutoResolved = true
	f.ResolveReason = reason
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
//
// Behavior by reason:
// - "not_an_issue": Permanent suppression (true false positive in detection logic)
// - "expected_behavior": Acknowledged only (removed from active list, stays in dismissed history)
// - "will_fix_later": Acknowledged only (removed from active list, stays in dismissed history)
//
// Rationale: Only true false positives ("not_an_issue") should be permanently suppressed.
// For "expected_behavior" and "will_fix_later", the finding stays visible (transparent)
// but is marked as acknowledged so the user knows they've reviewed it.
// Severity escalation will still clear the dismissal and reactivate the finding.
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
	// Mark as acknowledged for all dismiss reasons
	now := time.Now()
	f.AcknowledgedAt = &now

	// Only "not_an_issue" creates permanent suppression
	// This is for true false positives where the detection logic is wrong
	if reason == "not_an_issue" {
		f.Suppressed = true
	}
	// For "expected_behavior" and "will_fix_later":
	// - Finding stays visible (not suppressed, not snoozed)
	// - But is marked as dismissed/acknowledged so user knows they've reviewed it
	// - Severity escalation will clear DismissedReason and reactivate

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Undismiss reverts a dismissed finding back to active state
// This clears DismissedReason, Suppressed, and AcknowledgedAt
// allowing the finding to be raised again
func (s *FindingsStore) Undismiss(id string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	// Check if it was actually dismissed
	if f.DismissedReason == "" && !f.Suppressed {
		s.mu.Unlock()
		return false
	}

	// Clear dismissal state
	f.DismissedReason = ""
	f.Suppressed = false
	f.AcknowledgedAt = nil
	// Keep UserNote in case user wants to see their notes

	// If it was resolved, don't reactivate - user should manually reopen
	// But if it's not resolved, it becomes active again
	if f.ResolvedAt == nil && !f.IsSnoozed() {
		s.activeCounts[f.Severity]++
	}

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

// UpdateInvestigationOutcome updates the investigation outcome on a finding
func (s *FindingsStore) UpdateInvestigationOutcome(id, outcome string) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	f.InvestigationOutcome = outcome
	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// UpdateInvestigation updates all investigation fields on a finding
func (s *FindingsStore) UpdateInvestigation(id, sessionID, status, outcome string, lastInvestigatedAt *time.Time, attempts int) bool {
	s.mu.Lock()

	f, exists := s.findings[id]
	if !exists {
		s.mu.Unlock()
		return false
	}

	f.InvestigationSessionID = sessionID
	f.InvestigationStatus = status
	f.InvestigationOutcome = outcome
	f.LastInvestigatedAt = lastInvestigatedAt
	f.InvestigationAttempts = attempts
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

	// Create a suppression rule to block future findings with same resource+category
	s.addSuppressionRuleInternal(f.ResourceID, f.ResourceName, f.Category, "Suppressed via finding", "suppress")

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
// IMPORTANT: Only explicit manual suppression rules block new findings.
// A dismissed finding (even with "not_an_issue") does NOT block future findings.
// This ensures that if an issue recurs, it will be detected and alerted.
// For permanent suppression, users must create an explicit suppression rule.
func (s *FindingsStore) isSuppressedInternal(resourceID string, category FindingCategory) bool {
	// Only check manual suppression rules - dismissed findings should NOT block new findings
	// Rationale: If frigate-storage was full and user dismissed it as "fixed", we still
	// want to alert if frigate-storage fills up again in the future.
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

// ClearAll removes all findings from the store
// Returns the number of findings removed
func (s *FindingsStore) ClearAll() int {
	s.mu.Lock()
	count := len(s.findings)
	s.findings = make(map[string]*Finding)
	s.byResource = make(map[string][]string)
	s.activeCounts = make(map[FindingSeverity]int)
	s.mu.Unlock()
	s.scheduleSave()
	return count
}

// Cleanup removes old resolved findings (and trims stale dismissed history).
func (s *FindingsStore) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-maxAge)
	dismissedCutoff := now.Add(-30 * 24 * time.Hour)
	removed := 0

	for id, f := range s.findings {
		shouldRemove := false

		// Remove old resolved findings
		if f.ResolvedAt != nil && f.ResolvedAt.Before(cutoff) {
			shouldRemove = true
		}

		// Trim stale dismissed findings, but retain suppressed ones for memory.
		if f.DismissedReason != "" && !f.Suppressed && f.LastSeenAt.Before(dismissedCutoff) {
			shouldRemove = true
		}

		if shouldRemove {
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

		// Skip very old findings (more than 30 days)
		if time.Since(f.LastSeenAt) > 30*24*time.Hour {
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
	rule := s.addSuppressionRuleInternal(resourceID, resourceName, category, description, "manual")
	s.mu.Unlock()
	s.scheduleSave()
	return rule
}

// addSuppressionRuleInternal creates a suppression rule without locking (caller must hold lock)
func (s *FindingsStore) addSuppressionRuleInternal(resourceID, resourceName string, category FindingCategory, description, createdFrom string) *SuppressionRule {
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
		CreatedFrom:  createdFrom,
	}

	s.suppressionRules[ruleID] = rule
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

	// Include all dismissed findings as rules (for visibility and to allow reverting)
	// This covers both suppressed ("not_an_issue") and acknowledged ("expected_behavior", "will_fix_later")
	for _, f := range s.findings {
		if f.Suppressed || f.DismissedReason != "" {
			createdFrom := "finding"
			if !f.Suppressed && f.DismissedReason != "" {
				// Mark as "dismissed" type to distinguish from permanent suppression
				createdFrom = "dismissed"
			}
			// Handle nil AcknowledgedAt (shouldn't happen but be safe)
			createdAt := f.LastSeenAt
			if f.AcknowledgedAt != nil {
				createdAt = *f.AcknowledgedAt
			}
			rules = append(rules, &SuppressionRule{
				ID:              "finding_" + f.ID,
				ResourceID:      f.ResourceID,
				ResourceName:    f.ResourceName,
				Category:        f.Category,
				Description:     f.UserNote,
				DismissedReason: f.DismissedReason,
				CreatedAt:       createdAt,
				CreatedFrom:     createdFrom,
				FindingID:       f.ID,
			})
		}
	}

	return rules
}

// DeleteSuppressionRule removes a suppression rule
func (s *FindingsStore) DeleteSuppressionRule(ruleID string) bool {
	s.mu.Lock()

	// Check if it's an explicit rule
	if _, exists := s.suppressionRules[ruleID]; exists {
		delete(s.suppressionRules, ruleID)
		s.mu.Unlock()
		s.scheduleSave()
		return true
	}

	// Check if it's a finding-based rule (e.g., "finding_abc123")
	if strings.HasPrefix(ruleID, "finding_") {
		findingID := strings.TrimPrefix(ruleID, "finding_")
		if f, exists := s.findings[findingID]; exists && (f.Suppressed || f.DismissedReason != "") {
			// Un-suppress/undismiss the finding
			wasActive := f.IsActive()
			f.Suppressed = false
			f.DismissedReason = ""
			f.AcknowledgedAt = nil
			// If it wasn't active before but is now, increment count
			if !wasActive && f.IsActive() {
				s.activeCounts[f.Severity]++
			}
			s.mu.Unlock()
			s.scheduleSave()
			return true
		}
	}

	s.mu.Unlock()
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

// --- Semantic Deduplication ---

// FindingCluster represents a group of semantically related findings
type FindingCluster struct {
	ID               string          `json:"id"`
	PrimaryFindingID string          `json:"primary_finding_id"` // The "main" finding in the cluster
	RelatedIDs       []string        `json:"related_ids"`        // Other findings in the cluster
	CommonCategory   FindingCategory `json:"common_category"`
	CommonResource   string          `json:"common_resource,omitempty"` // If all findings are for same resource
	Summary          string          `json:"summary"`                   // Aggregated summary
	HighestSeverity  FindingSeverity `json:"highest_severity"`
	TotalCount       int             `json:"total_count"`
}

// SemanticSimilarity calculates similarity between two findings (0-1)
func SemanticSimilarity(f1, f2 *Finding) float64 {
	if f1 == nil || f2 == nil {
		return 0
	}

	var similarity float64

	// Same resource is a strong signal
	if f1.ResourceID == f2.ResourceID {
		similarity += 0.3
	}

	// Same category
	if f1.Category == f2.Category {
		similarity += 0.2
	}

	// Similar key (if set)
	if f1.Key != "" && f1.Key == f2.Key {
		similarity += 0.4
	}

	// Title keyword overlap
	titleSim := keywordOverlap(f1.Title, f2.Title)
	similarity += titleSim * 0.2

	// Description keyword overlap
	descSim := keywordOverlap(f1.Description, f2.Description)
	similarity += descSim * 0.1

	return minFloat(similarity, 1.0)
}

// keywordOverlap calculates the Jaccard similarity of keywords between two strings
func keywordOverlap(s1, s2 string) float64 {
	words1 := extractKeywords(s1)
	words2 := extractKeywords(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	// Calculate intersection
	intersection := 0
	for word := range words1 {
		if words2[word] {
			intersection++
		}
	}

	// Calculate union
	union := len(words1)
	for word := range words2 {
		if !words1[word] {
			union++
		}
	}

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// extractKeywords extracts significant keywords from a string
func extractKeywords(s string) map[string]bool {
	keywords := make(map[string]bool)

	// Common stop words to ignore
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true, "through": true,
		"and": true, "or": true, "but": true, "if": true, "then": true, "than": true,
		"this": true, "that": true, "these": true, "those": true, "it": true,
	}

	// Split into words and normalize
	word := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			if c >= 'A' && c <= 'Z' {
				c = c + 32 // lowercase
			}
			word += string(c)
		} else {
			if len(word) > 2 && !stopWords[word] {
				keywords[word] = true
			}
			word = ""
		}
	}
	if len(word) > 2 && !stopWords[word] {
		keywords[word] = true
	}

	return keywords
}

// FindSimilarFindings finds findings similar to the given one
func (s *FindingsStore) FindSimilarFindings(f *Finding, minSimilarity float64) []*Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var similar []*Finding

	for _, existing := range s.findings {
		if existing.ID == f.ID {
			continue
		}
		if !existing.IsActive() {
			continue
		}

		similarity := SemanticSimilarity(f, existing)
		if similarity >= minSimilarity {
			copy := *existing
			similar = append(similar, &copy)
		}
	}

	return similar
}

// AddWithDeduplication adds a finding, merging with similar existing findings if found
// Returns the finding ID (may be existing finding if merged) and whether it was new
func (s *FindingsStore) AddWithDeduplication(f *Finding, minSimilarity float64) (string, bool) {
	// First check for similar findings
	similar := s.FindSimilarFindings(f, minSimilarity)

	if len(similar) > 0 {
		// Find the most similar one
		var bestMatch *Finding
		var bestSimilarity float64

		for _, existing := range similar {
			sim := SemanticSimilarity(f, existing)
			if sim > bestSimilarity {
				bestMatch = existing
				bestSimilarity = sim
			}
		}

		if bestMatch != nil {
			// Merge with existing finding
			s.mu.Lock()
			if existing, ok := s.findings[bestMatch.ID]; ok {
				existing.LastSeenAt = time.Now()
				existing.TimesRaised++

				// Update to higher severity if new finding has higher severity
				severityOrder := map[FindingSeverity]int{
					FindingSeverityInfo:     0,
					FindingSeverityWatch:    1,
					FindingSeverityWarning:  2,
					FindingSeverityCritical: 3,
				}
				if severityOrder[f.Severity] > severityOrder[existing.Severity] {
					existing.Severity = f.Severity
				}

				// Append evidence if different
				if f.Evidence != "" && f.Evidence != existing.Evidence {
					existing.Evidence = existing.Evidence + "\n---\n" + f.Evidence
					// Truncate if too long
					if len(existing.Evidence) > 5000 {
						existing.Evidence = existing.Evidence[:5000] + "..."
					}
				}
			}
			s.mu.Unlock()
			s.scheduleSave()
			return bestMatch.ID, false
		}
	}

	// No similar finding found, add as new
	isNew := s.Add(f)
	return f.ID, isNew
}

// GetFindingClusters groups active findings into clusters based on semantic similarity
func (s *FindingsStore) GetFindingClusters(minSimilarity float64) []*FindingCluster {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get all active findings
	var activeFindings []*Finding
	for _, f := range s.findings {
		if f.IsActive() {
			copy := *f
			activeFindings = append(activeFindings, &copy)
		}
	}

	if len(activeFindings) == 0 {
		return nil
	}

	// Track which findings have been clustered
	clustered := make(map[string]bool)
	var clusters []*FindingCluster

	// For each finding, find similar ones and create a cluster
	for _, f := range activeFindings {
		if clustered[f.ID] {
			continue
		}

		cluster := &FindingCluster{
			ID:               fmt.Sprintf("cluster-%s", f.ID[:8]),
			PrimaryFindingID: f.ID,
			RelatedIDs:       make([]string, 0),
			CommonCategory:   f.Category,
			CommonResource:   f.ResourceID,
			HighestSeverity:  f.Severity,
			TotalCount:       1,
		}

		clustered[f.ID] = true

		// Find all similar findings
		for _, other := range activeFindings {
			if other.ID == f.ID || clustered[other.ID] {
				continue
			}

			similarity := SemanticSimilarity(f, other)
			if similarity >= minSimilarity {
				cluster.RelatedIDs = append(cluster.RelatedIDs, other.ID)
				cluster.TotalCount++
				clustered[other.ID] = true

				// Track if all are same resource
				if other.ResourceID != f.ResourceID {
					cluster.CommonResource = ""
				}

				// Track highest severity
				severityOrder := map[FindingSeverity]int{
					FindingSeverityInfo:     0,
					FindingSeverityWatch:    1,
					FindingSeverityWarning:  2,
					FindingSeverityCritical: 3,
				}
				if severityOrder[other.Severity] > severityOrder[cluster.HighestSeverity] {
					cluster.HighestSeverity = other.Severity
				}
			}
		}

		// Generate cluster summary
		if cluster.TotalCount > 1 {
			cluster.Summary = fmt.Sprintf("%d related %s findings", cluster.TotalCount, cluster.CommonCategory)
			if cluster.CommonResource != "" {
				cluster.Summary += " for " + cluster.CommonResource
			}
		} else {
			cluster.Summary = f.Title
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
