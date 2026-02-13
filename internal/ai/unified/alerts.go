// Package unified provides a unified alert/finding system that bridges
// threshold-based alerts and AI-generated findings into a single intelligence layer.
package unified

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// FindingSource identifies where a finding originated
type FindingSource string

const (
	// SourceThreshold indicates a finding created from a threshold alert
	SourceThreshold FindingSource = "threshold"
	// SourceAIPatrol indicates a finding created by AI patrol analysis
	SourceAIPatrol FindingSource = "ai-patrol"
	// SourceAIChat indicates a finding from interactive AI chat
	SourceAIChat FindingSource = "ai-chat"
	// SourceAnomaly indicates a finding from baseline anomaly detection
	SourceAnomaly FindingSource = "anomaly"
	// SourceCorrelation indicates a finding from root-cause correlation
	SourceCorrelation FindingSource = "correlation"
	// SourceForecast indicates a proactive finding from trend forecasting
	SourceForecast FindingSource = "forecast"
)

// UnifiedSeverity maps different severity systems to a common scale
type UnifiedSeverity string

const (
	SeverityInfo     UnifiedSeverity = "info"
	SeverityWatch    UnifiedSeverity = "watch"
	SeverityWarning  UnifiedSeverity = "warning"
	SeverityCritical UnifiedSeverity = "critical"
)

// UnifiedCategory groups findings by type
type UnifiedCategory string

const (
	CategoryPerformance   UnifiedCategory = "performance"
	CategoryCapacity      UnifiedCategory = "capacity"
	CategoryReliability   UnifiedCategory = "reliability"
	CategoryBackup        UnifiedCategory = "backup"
	CategorySecurity      UnifiedCategory = "security"
	CategoryConnectivity  UnifiedCategory = "connectivity"
	CategoryConfiguration UnifiedCategory = "configuration"
	CategoryGeneral       UnifiedCategory = "general"
)

type UnifiedFindingLifecycleEvent struct {
	At       time.Time         `json:"at"`
	Type     string            `json:"type"`
	Message  string            `json:"message,omitempty"`
	From     string            `json:"from,omitempty"`
	To       string            `json:"to,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UnifiedFinding represents a unified finding that can originate from
// threshold alerts, AI analysis, or other detection methods
type UnifiedFinding struct {
	ID             string          `json:"id"`
	Source         FindingSource   `json:"source"`
	Severity       UnifiedSeverity `json:"severity"`
	Category       UnifiedCategory `json:"category"`
	ResourceID     string          `json:"resource_id"`
	ResourceName   string          `json:"resource_name"`
	ResourceType   string          `json:"resource_type"`
	Node           string          `json:"node,omitempty"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Recommendation string          `json:"recommendation,omitempty"`
	Evidence       string          `json:"evidence,omitempty"`

	// Threshold-specific fields (when Source == "threshold")
	AlertID     string  `json:"alert_id,omitempty"`
	AlertType   string  `json:"alert_type,omitempty"` // cpu, memory, disk, etc.
	Value       float64 `json:"value,omitempty"`
	Threshold   float64 `json:"threshold,omitempty"`
	IsThreshold bool    `json:"is_threshold"`

	// AI enhancement fields
	AIContext     string     `json:"ai_context,omitempty"`     // AI-added context/explanation
	RootCauseID   string     `json:"root_cause_id,omitempty"`  // Linked root cause finding
	CorrelatedIDs []string   `json:"correlated_ids,omitempty"` // Related findings
	RemediationID string     `json:"remediation_id,omitempty"` // Linked remediation plan
	AIConfidence  float64    `json:"ai_confidence,omitempty"`  // AI confidence score (0-1)
	EnhancedByAI  bool       `json:"enhanced_by_ai"`           // Whether AI has analyzed this
	AIEnhancedAt  *time.Time `json:"ai_enhanced_at,omitempty"` // When AI analyzed

	// Investigation fields (autonomous patrol investigation)
	InvestigationSessionID string                         `json:"investigation_session_id,omitempty"`
	InvestigationStatus    string                         `json:"investigation_status,omitempty"`
	InvestigationOutcome   string                         `json:"investigation_outcome,omitempty"`
	LastInvestigatedAt     *time.Time                     `json:"last_investigated_at,omitempty"`
	InvestigationAttempts  int                            `json:"investigation_attempts,omitempty"`
	LoopState              string                         `json:"loop_state,omitempty"`
	Lifecycle              []UnifiedFindingLifecycleEvent `json:"lifecycle,omitempty"`
	RegressionCount        int                            `json:"regression_count,omitempty"`
	LastRegressionAt       *time.Time                     `json:"last_regression_at,omitempty"`

	// Timestamps
	DetectedAt time.Time  `json:"detected_at"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// User feedback
	AcknowledgedAt  *time.Time `json:"acknowledged_at,omitempty"`
	SnoozedUntil    *time.Time `json:"snoozed_until,omitempty"`
	DismissedReason string     `json:"dismissed_reason,omitempty"`
	UserNote        string     `json:"user_note,omitempty"`
	Suppressed      bool       `json:"suppressed"`
	TimesRaised     int        `json:"times_raised"`
}

// IsActive returns true if the finding is active (not resolved, snoozed, or suppressed)
func (f *UnifiedFinding) IsActive() bool {
	return f.ResolvedAt == nil && !f.IsSnoozed() && !f.Suppressed && f.DismissedReason == ""
}

// IsSnoozed returns true if the finding is currently snoozed
func (f *UnifiedFinding) IsSnoozed() bool {
	return f.SnoozedUntil != nil && time.Now().Before(*f.SnoozedUntil)
}

// AlertToFindingConfig configures how alerts are converted to findings
type AlertToFindingConfig struct {
	// DefaultCategory is used when alert type doesn't map to a specific category
	DefaultCategory UnifiedCategory
	// TypeCategoryMap maps alert types to categories
	TypeCategoryMap map[string]UnifiedCategory
	// GenerateRecommendation enables AI-style recommendations for threshold alerts
	GenerateRecommendation bool
}

// DefaultAlertToFindingConfig returns sensible defaults
func DefaultAlertToFindingConfig() AlertToFindingConfig {
	return AlertToFindingConfig{
		DefaultCategory: CategoryPerformance,
		TypeCategoryMap: map[string]UnifiedCategory{
			"cpu":              CategoryPerformance,
			"memory":           CategoryPerformance,
			"disk":             CategoryCapacity,
			"diskRead":         CategoryPerformance,
			"diskWrite":        CategoryPerformance,
			"networkIn":        CategoryPerformance,
			"networkOut":       CategoryPerformance,
			"usage":            CategoryCapacity,
			"storage":          CategoryCapacity,
			"temperature":      CategoryReliability,
			"diskTemperature":  CategoryReliability,
			"offline":          CategoryConnectivity,
			"nodeOffline":      CategoryConnectivity,
			"poweredOff":       CategoryReliability,
			"backup":           CategoryBackup,
			"backupMissing":    CategoryBackup,
			"backupStale":      CategoryBackup,
			"snapshot":         CategoryBackup,
			"snapshotAge":      CategoryBackup,
			"snapshotSize":     CategoryCapacity,
			"restartLoop":      CategoryReliability,
			"oom":              CategoryReliability,
			"imageUpdateAvail": CategoryConfiguration,
		},
		GenerateRecommendation: true,
	}
}

// AlertAdapter provides the interface for reading alert data
type AlertAdapter interface {
	// GetAlertID returns the alert's unique identifier
	GetAlertID() string
	// GetAlertType returns the type of alert (cpu, memory, etc.)
	GetAlertType() string
	// GetAlertLevel returns the severity level
	GetAlertLevel() string
	// GetResourceID returns the affected resource ID
	GetResourceID() string
	// GetResourceName returns the human-readable resource name
	GetResourceName() string
	// GetNode returns the node name if applicable
	GetNode() string
	// GetMessage returns the alert message
	GetMessage() string
	// GetValue returns the current metric value
	GetValue() float64
	// GetThreshold returns the threshold that was exceeded
	GetThreshold() float64
	// GetStartTime returns when the alert started
	GetStartTime() time.Time
	// GetLastSeen returns when the alert was last seen
	GetLastSeen() time.Time
	// GetMetadata returns additional alert metadata
	GetMetadata() map[string]interface{}
}

// UnifiedStore manages unified findings
type UnifiedStore struct {
	mu sync.RWMutex

	findings   map[string]*UnifiedFinding
	byResource map[string][]string // resource_id -> []finding_id
	byAlert    map[string]string   // alert_id -> finding_id
	bySource   map[FindingSource][]string

	config AlertToFindingConfig

	// Callbacks
	onNewFinding      func(f *UnifiedFinding)
	onFindingResolved func(f *UnifiedFinding)
	onFindingEnhanced func(f *UnifiedFinding)

	// Persistence
	persistence UnifiedPersistence
	saveTimer   *time.Timer
	savePending bool
}

// UnifiedPersistence interface for saving/loading unified findings
type UnifiedPersistence interface {
	SaveFindings(findings map[string]*UnifiedFinding) error
	LoadFindings() (map[string]*UnifiedFinding, error)
}

// NewUnifiedStore creates a new unified store
func NewUnifiedStore(config AlertToFindingConfig) *UnifiedStore {
	return &UnifiedStore{
		findings:   make(map[string]*UnifiedFinding),
		byResource: make(map[string][]string),
		byAlert:    make(map[string]string),
		bySource:   make(map[FindingSource][]string),
		config:     config,
	}
}

// SetPersistence sets the persistence layer
func (s *UnifiedStore) SetPersistence(p UnifiedPersistence) error {
	s.mu.Lock()
	s.persistence = p
	s.mu.Unlock()

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
				if f.AlertID != "" {
					s.byAlert[f.AlertID] = id
				}
				s.bySource[f.Source] = append(s.bySource[f.Source], id)
			}
			s.mu.Unlock()
		}
	}
	return nil
}

// SetOnNewFinding sets the callback for new findings
func (s *UnifiedStore) SetOnNewFinding(cb func(f *UnifiedFinding)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onNewFinding = cb
}

// SetOnFindingResolved sets the callback for resolved findings
func (s *UnifiedStore) SetOnFindingResolved(cb func(f *UnifiedFinding)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onFindingResolved = cb
}

// SetOnFindingEnhanced sets the callback for AI-enhanced findings
func (s *UnifiedStore) SetOnFindingEnhanced(cb func(f *UnifiedFinding)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onFindingEnhanced = cb
}

// ConvertAlert converts a threshold alert to a unified finding
func (s *UnifiedStore) ConvertAlert(alert AlertAdapter) *UnifiedFinding {
	alertID := alert.GetAlertID()
	alertType := alert.GetAlertType()

	// Determine category
	category := s.config.DefaultCategory
	if cat, ok := s.config.TypeCategoryMap[alertType]; ok {
		category = cat
	}

	// Map alert level to unified severity
	severity := SeverityWarning
	if strings.EqualFold(alert.GetAlertLevel(), "critical") {
		severity = SeverityCritical
	}

	// Determine resource type from alert type and metadata
	resourceType := determineResourceType(alertType, alert.GetMetadata())

	// Generate finding ID
	findingID := fmt.Sprintf("alert-%s-%d", alertID, time.Now().UnixNano()%1000000)

	// Create the finding
	finding := &UnifiedFinding{
		ID:           findingID,
		Source:       SourceThreshold,
		Severity:     severity,
		Category:     category,
		ResourceID:   alert.GetResourceID(),
		ResourceName: alert.GetResourceName(),
		ResourceType: resourceType,
		Node:         alert.GetNode(),
		Title:        generateTitle(alertType, alert.GetResourceName(), alert.GetValue(), alert.GetThreshold()),
		Description:  alert.GetMessage(),
		AlertID:      alertID,
		AlertType:    alertType,
		Value:        alert.GetValue(),
		Threshold:    alert.GetThreshold(),
		IsThreshold:  true,
		DetectedAt:   alert.GetStartTime(),
		LastSeenAt:   alert.GetLastSeen(),
		TimesRaised:  1,
	}

	// Generate evidence
	finding.Evidence = fmt.Sprintf("Threshold alert triggered: %s = %.1f (threshold: %.1f)",
		alertType, alert.GetValue(), alert.GetThreshold())

	// Generate recommendation if enabled
	if s.config.GenerateRecommendation {
		finding.Recommendation = generateRecommendation(alertType, alert.GetValue(), alert.GetThreshold())
	}

	return finding
}

// AddFromAlert creates a unified finding from an alert
func (s *UnifiedStore) AddFromAlert(alert AlertAdapter) (*UnifiedFinding, bool) {
	alertID := alert.GetAlertID()

	s.mu.Lock()

	// Check if we already have a finding for this alert
	if existingID, ok := s.byAlert[alertID]; ok {
		if existing := s.findings[existingID]; existing != nil {
			// Update existing finding
			existing.LastSeenAt = alert.GetLastSeen()
			existing.Value = alert.GetValue()
			existing.TimesRaised++

			// Re-open if the finding was resolved (alert fired again)
			if existing.ResolvedAt != nil {
				existing.ResolvedAt = nil
				log.Debug().
					Str("finding_id", existing.ID).
					Str("alert_id", alertID).
					Msg("Re-opened resolved finding due to alert re-firing")
			}

			// Update severity if alert level changed
			newSeverity := SeverityWarning
			if strings.EqualFold(alert.GetAlertLevel(), "critical") {
				newSeverity = SeverityCritical
			}
			if severityOrder(newSeverity) > severityOrder(existing.Severity) {
				existing.Severity = newSeverity
			}

			s.mu.Unlock()
			s.scheduleSave()
			return existing, false
		}
	}

	// Convert and add new finding
	finding := s.ConvertAlert(alert)
	s.findings[finding.ID] = finding
	s.byResource[finding.ResourceID] = append(s.byResource[finding.ResourceID], finding.ID)
	s.byAlert[alertID] = finding.ID
	s.bySource[SourceThreshold] = append(s.bySource[SourceThreshold], finding.ID)

	callback := s.onNewFinding
	s.mu.Unlock()

	s.scheduleSave()

	// Fire callback
	if callback != nil {
		go callback(finding)
	}

	log.Debug().
		Str("finding_id", finding.ID).
		Str("alert_id", alertID).
		Str("resource", finding.ResourceName).
		Str("category", string(finding.Category)).
		Msg("Created unified finding from alert")

	return finding, true
}

// AddFromAI creates a unified finding from AI analysis
func (s *UnifiedStore) AddFromAI(finding *UnifiedFinding) (*UnifiedFinding, bool) {
	if finding == nil {
		return nil, false
	}

	now := time.Now()
	if finding.Source == "" {
		finding.Source = SourceAIPatrol
	}
	if finding.DetectedAt.IsZero() {
		finding.DetectedAt = now
	}
	if finding.LastSeenAt.IsZero() {
		finding.LastSeenAt = now
	}

	s.mu.Lock()

	// For AI patrol findings, the ID is stable and should be treated as the canonical key.
	// Always merge into the existing record to avoid duplicating index entries.
	if existing, exists := s.findings[finding.ID]; exists && existing != nil {
		// Basic fields
		existing.Source = finding.Source
		existing.ResourceID = finding.ResourceID
		existing.ResourceName = finding.ResourceName
		existing.ResourceType = finding.ResourceType
		existing.Node = finding.Node
		existing.Title = finding.Title
		if severityOrder(finding.Severity) > severityOrder(existing.Severity) {
			existing.Severity = finding.Severity
		}
		existing.Category = finding.Category

		if finding.Description != "" {
			existing.Description = finding.Description
		}
		if finding.Recommendation != "" {
			existing.Recommendation = finding.Recommendation
		}
		if finding.Evidence != "" {
			existing.Evidence = finding.Evidence
		}

		// Timestamps and counters
		if finding.LastSeenAt.After(existing.LastSeenAt) {
			existing.LastSeenAt = finding.LastSeenAt
		}
		if finding.TimesRaised > 0 {
			existing.TimesRaised = finding.TimesRaised
		} else {
			existing.TimesRaised++
		}
		// Allow reopening by clearing ResolvedAt if the incoming finding is active again.
		existing.ResolvedAt = finding.ResolvedAt

		// Investigation fields (allow clearing)
		existing.InvestigationSessionID = finding.InvestigationSessionID
		existing.InvestigationStatus = finding.InvestigationStatus
		existing.InvestigationOutcome = finding.InvestigationOutcome
		existing.LastInvestigatedAt = finding.LastInvestigatedAt
		existing.InvestigationAttempts = finding.InvestigationAttempts
		existing.LoopState = finding.LoopState
		existing.Lifecycle = finding.Lifecycle
		existing.RegressionCount = finding.RegressionCount
		existing.LastRegressionAt = finding.LastRegressionAt

		// User feedback (allow clearing)
		existing.AcknowledgedAt = finding.AcknowledgedAt
		existing.SnoozedUntil = finding.SnoozedUntil
		existing.DismissedReason = finding.DismissedReason
		existing.UserNote = finding.UserNote
		existing.Suppressed = finding.Suppressed

		// AI enhancement fields (best-effort merge)
		if finding.AIContext != "" {
			existing.AIContext = finding.AIContext
		}
		if finding.RootCauseID != "" {
			existing.RootCauseID = finding.RootCauseID
		}
		if len(finding.CorrelatedIDs) > 0 {
			existing.CorrelatedIDs = finding.CorrelatedIDs
		}
		if finding.RemediationID != "" {
			existing.RemediationID = finding.RemediationID
		}
		if finding.AIConfidence != 0 {
			existing.AIConfidence = finding.AIConfidence
		}
		if finding.EnhancedByAI {
			existing.EnhancedByAI = true
			existing.AIEnhancedAt = finding.AIEnhancedAt
		}

		s.mu.Unlock()
		s.scheduleSave()
		return existing, false
	}

	// New finding
	if finding.TimesRaised <= 0 {
		finding.TimesRaised = 1
	}
	s.findings[finding.ID] = finding
	s.byResource[finding.ResourceID] = append(s.byResource[finding.ResourceID], finding.ID)
	s.bySource[finding.Source] = append(s.bySource[finding.Source], finding.ID)

	callback := s.onNewFinding
	s.mu.Unlock()

	s.scheduleSave()

	if callback != nil {
		go callback(finding)
	}

	return finding, true
}

// ResolveByAlert resolves a finding by its alert ID
func (s *UnifiedStore) ResolveByAlert(alertID string) bool {
	s.mu.Lock()

	findingID, ok := s.byAlert[alertID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	finding, ok := s.findings[findingID]
	if !ok || finding.ResolvedAt != nil {
		s.mu.Unlock()
		return false
	}

	now := time.Now()
	finding.ResolvedAt = &now

	callback := s.onFindingResolved
	s.mu.Unlock()

	s.scheduleSave()

	if callback != nil {
		go callback(finding)
	}

	log.Debug().
		Str("finding_id", findingID).
		Str("alert_id", alertID).
		Msg("Resolved unified finding from alert")

	return true
}

// EnhanceWithAI adds AI context to a finding
func (s *UnifiedStore) EnhanceWithAI(findingID string, context string, confidence float64, rootCauseID string, correlatedIDs []string) bool {
	s.mu.Lock()

	finding, ok := s.findings[findingID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	finding.AIContext = context
	finding.AIConfidence = confidence
	finding.EnhancedByAI = true
	now := time.Now()
	finding.AIEnhancedAt = &now

	if rootCauseID != "" {
		finding.RootCauseID = rootCauseID
	}
	if len(correlatedIDs) > 0 {
		finding.CorrelatedIDs = correlatedIDs
	}

	callback := s.onFindingEnhanced
	s.mu.Unlock()

	s.scheduleSave()

	if callback != nil {
		go callback(finding)
	}

	return true
}

// LinkRemediation links a remediation plan to a finding
func (s *UnifiedStore) LinkRemediation(findingID, remediationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	finding, ok := s.findings[findingID]
	if !ok {
		return false
	}

	finding.RemediationID = remediationID
	s.scheduleSave()
	return true
}

// Get returns a finding by ID
func (s *UnifiedStore) Get(findingID string) *UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if f, ok := s.findings[findingID]; ok {
		copy := *f
		return &copy
	}
	return nil
}

// GetByAlert returns a finding by its alert ID
func (s *UnifiedStore) GetByAlert(alertID string) *UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if findingID, ok := s.byAlert[alertID]; ok {
		if f, ok := s.findings[findingID]; ok {
			copy := *f
			return &copy
		}
	}
	return nil
}

// GetByResource returns all active findings for a resource
func (s *UnifiedStore) GetByResource(resourceID string) []*UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*UnifiedFinding
	for _, id := range s.byResource[resourceID] {
		if f, ok := s.findings[id]; ok && f.IsActive() {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// GetBySource returns all active findings from a specific source
func (s *UnifiedStore) GetBySource(source FindingSource) []*UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*UnifiedFinding
	for _, id := range s.bySource[source] {
		if f, ok := s.findings[id]; ok && f.IsActive() {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// GetActive returns all active findings
func (s *UnifiedStore) GetActive() []*UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*UnifiedFinding
	for _, f := range s.findings {
		if f.IsActive() {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// GetAll returns all findings regardless of status.
func (s *UnifiedStore) GetAll() []*UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*UnifiedFinding
	for _, f := range s.findings {
		copy := *f
		result = append(result, &copy)
	}
	return result
}

// GetThresholdFindings returns all active threshold-based findings
func (s *UnifiedStore) GetThresholdFindings() []*UnifiedFinding {
	return s.GetBySource(SourceThreshold)
}

// GetAIFindings returns all active AI-generated findings
func (s *UnifiedStore) GetAIFindings() []*UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*UnifiedFinding
	for _, f := range s.findings {
		if f.IsActive() && f.Source != SourceThreshold {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// GetUnenhanсedThresholdFindings returns threshold findings not yet enhanced by AI
func (s *UnifiedStore) GetUnenhancedThresholdFindings() []*UnifiedFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*UnifiedFinding
	for _, f := range s.findings {
		if f.IsActive() && f.Source == SourceThreshold && !f.EnhancedByAI {
			copy := *f
			result = append(result, &copy)
		}
	}
	return result
}

// Dismiss dismisses a finding with a reason
func (s *UnifiedStore) Dismiss(findingID, reason, note string) bool {
	s.mu.Lock()

	f, ok := s.findings[findingID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	f.DismissedReason = reason
	if note != "" {
		f.UserNote = note
	}
	now := time.Now()
	f.AcknowledgedAt = &now

	if reason == "not_an_issue" {
		f.Suppressed = true
	}

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Snooze snoozes a finding for the specified duration
func (s *UnifiedStore) Snooze(findingID string, duration time.Duration) bool {
	s.mu.Lock()

	f, ok := s.findings[findingID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	until := time.Now().Add(duration)
	f.SnoozedUntil = &until

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Acknowledge acknowledges a finding
func (s *UnifiedStore) Acknowledge(findingID string) bool {
	s.mu.Lock()

	f, ok := s.findings[findingID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	now := time.Now()
	f.AcknowledgedAt = &now

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// SetUserNote sets or updates the user note on a finding
func (s *UnifiedStore) SetUserNote(findingID string, note string) bool {
	s.mu.Lock()

	f, ok := s.findings[findingID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	f.UserNote = note

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// Resolve marks a finding as resolved
func (s *UnifiedStore) Resolve(findingID string) bool {
	s.mu.Lock()

	f, ok := s.findings[findingID]
	if !ok {
		s.mu.Unlock()
		return false
	}

	now := time.Now()
	f.ResolvedAt = &now
	f.SnoozedUntil = nil

	s.mu.Unlock()
	s.scheduleSave()
	return true
}

// GetSummary returns a summary of active findings
func (s *UnifiedStore) GetSummary() UnifiedSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := UnifiedSummary{
		BySource:   make(map[FindingSource]int),
		ByCategory: make(map[UnifiedCategory]int),
	}

	for _, f := range s.findings {
		summary.Total++
		if f.IsActive() {
			summary.Active++
			summary.BySource[f.Source]++
			summary.ByCategory[f.Category]++

			switch f.Severity {
			case SeverityCritical:
				summary.Critical++
			case SeverityWarning:
				summary.Warning++
			case SeverityWatch:
				summary.Watch++
			case SeverityInfo:
				summary.Info++
			}

			if f.EnhancedByAI {
				summary.EnhancedByAI++
			}
		}
	}

	return summary
}

// UnifiedSummary provides statistics about unified findings
type UnifiedSummary struct {
	Total        int                     `json:"total"`
	Active       int                     `json:"active"`
	Critical     int                     `json:"critical"`
	Warning      int                     `json:"warning"`
	Watch        int                     `json:"watch"`
	Info         int                     `json:"info"`
	EnhancedByAI int                     `json:"enhanced_by_ai"`
	BySource     map[FindingSource]int   `json:"by_source"`
	ByCategory   map[UnifiedCategory]int `json:"by_category"`
}

// FormatForContext formats unified findings for AI prompt injection
func (s *UnifiedStore) FormatForContext() string {
	active := s.GetActive()
	if len(active) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("\n## Active Issues (Unified View)\n\n")

	// Group by source
	bySource := make(map[FindingSource][]*UnifiedFinding)
	for _, f := range active {
		bySource[f.Source] = append(bySource[f.Source], f)
	}

	// Threshold alerts first
	if findings := bySource[SourceThreshold]; len(findings) > 0 {
		result.WriteString("### Threshold Alerts\n")
		for _, f := range findings {
			enhanced := ""
			if f.EnhancedByAI {
				enhanced = " [AI-analyzed]"
			}
			result.WriteString(fmt.Sprintf("- [%s] %s: %s%s\n",
				f.Severity, f.ResourceName, f.Title, enhanced))
			if f.AIContext != "" {
				result.WriteString(fmt.Sprintf("  AI context: %s\n", f.AIContext))
			}
		}
		result.WriteString("\n")
	}

	// AI findings
	for source, findings := range bySource {
		if source == SourceThreshold || len(findings) == 0 {
			continue
		}
		result.WriteString(fmt.Sprintf("### %s Findings\n", formatSourceName(source)))
		for _, f := range findings {
			result.WriteString(fmt.Sprintf("- [%s] %s: %s\n",
				f.Severity, f.ResourceName, f.Title))
			if f.RootCauseID != "" {
				result.WriteString(fmt.Sprintf("  Root cause linked: %s\n", f.RootCauseID))
			}
		}
		result.WriteString("\n")
	}

	return result.String()
}

// scheduleSave schedules a debounced save operation
func (s *UnifiedStore) scheduleSave() {
	if s.persistence == nil {
		return
	}

	if s.savePending {
		return
	}

	s.savePending = true
	s.saveTimer = time.AfterFunc(5*time.Second, func() {
		s.mu.Lock()
		s.savePending = false
		findingsCopy := make(map[string]*UnifiedFinding, len(s.findings))
		for id, f := range s.findings {
			copy := *f
			findingsCopy[id] = &copy
		}
		persistence := s.persistence
		s.mu.Unlock()

		if persistence != nil {
			if err := persistence.SaveFindings(findingsCopy); err != nil {
				log.Error().Err(err).Msg("failed to save unified findings")
			}
		}
	})
}

// ForceSave immediately saves findings
func (s *UnifiedStore) ForceSave() error {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.savePending = false

	findingsCopy := make(map[string]*UnifiedFinding, len(s.findings))
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

// Helper functions

func severityOrder(s UnifiedSeverity) int {
	switch s {
	case SeverityInfo:
		return 0
	case SeverityWatch:
		return 1
	case SeverityWarning:
		return 2
	case SeverityCritical:
		return 3
	default:
		return 0
	}
}

func determineResourceType(alertType string, metadata map[string]interface{}) string {
	// Check metadata for explicit type
	if metadata != nil {
		if t, ok := metadata["resourceType"].(string); ok && t != "" {
			return t
		}
	}

	// Infer from alert type
	switch alertType {
	case "nodeOffline", "temperature":
		return "node"
	case "usage", "storage":
		return "storage"
	case "backup", "backupMissing", "backupStale":
		return "backup"
	case "snapshot", "snapshotAge", "snapshotSize":
		return "snapshot"
	case "restartLoop", "oom", "imageUpdateAvail":
		return "docker"
	default:
		return "guest"
	}
}

func generateTitle(alertType, resourceName string, value, threshold float64) string {
	switch alertType {
	case "cpu":
		return fmt.Sprintf("High CPU usage on %s (%.0f%%)", resourceName, value)
	case "memory":
		return fmt.Sprintf("High memory usage on %s (%.0f%%)", resourceName, value)
	case "disk":
		return fmt.Sprintf("High disk usage on %s (%.0f%%)", resourceName, value)
	case "usage", "storage":
		return fmt.Sprintf("High storage usage on %s (%.0f%%)", resourceName, value)
	case "temperature":
		return fmt.Sprintf("High temperature on %s (%.0f°C)", resourceName, value)
	case "offline", "nodeOffline":
		return fmt.Sprintf("%s is offline", resourceName)
	case "poweredOff":
		return fmt.Sprintf("%s is powered off", resourceName)
	default:
		return fmt.Sprintf("%s alert on %s", alertType, resourceName)
	}
}

func generateRecommendation(alertType string, value, threshold float64) string {
	switch alertType {
	case "cpu":
		return "Consider investigating high CPU processes or allocating more CPU resources."
	case "memory":
		return "Consider investigating memory-intensive processes or increasing RAM allocation."
	case "disk":
		if value > 95 {
			return "URGENT: Disk is nearly full. Free up space immediately or expand storage."
		}
		return "Consider cleaning up unused files or expanding disk capacity."
	case "usage", "storage":
		return "Consider removing unused data or expanding storage capacity."
	case "temperature":
		return "Check cooling systems and airflow. Consider reducing workload."
	case "offline", "nodeOffline":
		return "Investigate connectivity and check if the resource is accessible."
	case "poweredOff":
		return "Start the resource if it should be running, or acknowledge if maintenance is planned."
	default:
		return "Investigate the alert and take appropriate action."
	}
}

func formatSourceName(source FindingSource) string {
	switch source {
	case SourceThreshold:
		return "Threshold Alert"
	case SourceAIPatrol:
		return "Pulse Patrol"
	case SourceAIChat:
		return "Pulse Assistant"
	case SourceAnomaly:
		return "Anomaly Detection"
	case SourceCorrelation:
		return "Root Cause Analysis"
	case SourceForecast:
		return "Proactive Forecast"
	default:
		return string(source)
	}
}
