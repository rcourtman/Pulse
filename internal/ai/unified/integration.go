// Package unified provides a unified alert/finding system.
package unified

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Integration provides a high-level API for the unified alert/finding system
type Integration struct {
	store  *UnifiedStore
	bridge *AlertBridge

	// Component references for cross-feature integration
	correlationEngine CorrelationEngine
	remediationEngine RemediationEngine
	learningStore     LearningStore
}

// CorrelationEngine interface for root-cause correlation
type CorrelationEngine interface {
	// AnalyzeForFinding performs root-cause analysis for a finding
	AnalyzeForFinding(findingID string, resourceID string) (rootCauseID string, correlatedIDs []string, explanation string, err error)
}

// RemediationEngine interface for remediation
type RemediationEngine interface {
	// GeneratePlanForFinding generates a remediation plan for a finding
	GeneratePlanForFinding(finding *UnifiedFinding) (planID string, err error)
}

// LearningStore interface for feedback learning
type LearningStore interface {
	// RecordFindingFeedback records user feedback on a finding
	RecordFindingFeedback(findingID, resourceID, category, action, reason, note string)
	// ShouldSuppress checks if findings of this type should be suppressed
	ShouldSuppress(resourceID, category, severity string) bool
}

// IntegrationConfig configures the integration
type IntegrationConfig struct {
	DataDir              string
	AlertToFindingConfig AlertToFindingConfig
	BridgeConfig         BridgeConfig
}

// DefaultIntegrationConfig returns sensible defaults
func DefaultIntegrationConfig(dataDir string) IntegrationConfig {
	return IntegrationConfig{
		DataDir:              dataDir,
		AlertToFindingConfig: DefaultAlertToFindingConfig(),
		BridgeConfig:         DefaultBridgeConfig(),
	}
}

// NewIntegration creates a new unified system integration
func NewIntegration(config IntegrationConfig) *Integration {
	store := NewUnifiedStore(config.AlertToFindingConfig)
	bridge := NewAlertBridge(store, config.BridgeConfig)

	// Set up persistence
	persistence := NewVersionedPersistence(config.DataDir)
	if err := store.SetPersistence(persistence); err != nil {
		log.Error().Err(err).Msg("Failed to set up unified findings persistence")
	}

	return &Integration{
		store:  store,
		bridge: bridge,
	}
}

// SetCorrelationEngine sets the correlation engine
func (i *Integration) SetCorrelationEngine(engine CorrelationEngine) {
	i.correlationEngine = engine

	// Set up AI enhancement to use correlation
	i.bridge.SetAIEnhancement(func(findingID string) error {
		return i.enhanceFindingWithCorrelation(findingID)
	})
}

// SetRemediationEngine sets the remediation engine
func (i *Integration) SetRemediationEngine(engine RemediationEngine) {
	i.remediationEngine = engine
}

// SetLearningStore sets the learning store
func (i *Integration) SetLearningStore(store LearningStore) {
	i.learningStore = store
}

// SetAlertProvider connects the alert system
func (i *Integration) SetAlertProvider(provider AlertProvider) {
	i.bridge.SetAlertProvider(provider)
}

// SetPatrolTrigger sets the patrol trigger function
func (i *Integration) SetPatrolTrigger(fn PatrolTriggerFunc) {
	i.bridge.SetPatrolTrigger(fn)
}

// Start starts the unified system
func (i *Integration) Start() {
	i.bridge.Start()
	log.Info().Msg("Unified alert/finding system started")
}

// Stop stops the unified system
func (i *Integration) Stop() {
	i.bridge.Stop()
	if err := i.store.ForceSave(); err != nil {
		log.Error().Err(err).Msg("Failed to save unified findings on shutdown")
	}
	log.Info().Msg("Unified alert/finding system stopped")
}

// GetStore returns the unified store
func (i *Integration) GetStore() *UnifiedStore {
	return i.store
}

// GetBridge returns the alert bridge
func (i *Integration) GetBridge() *AlertBridge {
	return i.bridge
}

// AddAIFinding adds an AI-generated finding
func (i *Integration) AddAIFinding(finding *UnifiedFinding) (*UnifiedFinding, bool) {
	// Check learning store for suppression
	if i.learningStore != nil {
		if i.learningStore.ShouldSuppress(finding.ResourceID, string(finding.Category), string(finding.Severity)) {
			log.Debug().
				Str("resource", finding.ResourceID).
				Str("category", string(finding.Category)).
				Msg("Finding suppressed by learning store")
			return nil, false
		}
	}

	result, isNew := i.store.AddFromAI(finding)

	// Generate remediation plan for new findings
	if isNew && i.remediationEngine != nil {
		go func() {
			if planID, err := i.remediationEngine.GeneratePlanForFinding(result); err == nil {
				i.store.LinkRemediation(result.ID, planID)
			}
		}()
	}

	return result, isNew
}

// DismissFinding dismisses a finding and records feedback
func (i *Integration) DismissFinding(findingID, reason, note string) bool {
	finding := i.store.Get(findingID)
	if finding == nil {
		return false
	}

	// Record feedback for learning
	if i.learningStore != nil {
		i.learningStore.RecordFindingFeedback(
			findingID,
			finding.ResourceID,
			string(finding.Category),
			"dismiss",
			reason,
			note,
		)
	}

	return i.store.Dismiss(findingID, reason, note)
}

// SnoozeFinding snoozes a finding
func (i *Integration) SnoozeFinding(findingID string, duration time.Duration) bool {
	finding := i.store.Get(findingID)
	if finding == nil {
		return false
	}

	// Record feedback for learning
	if i.learningStore != nil {
		i.learningStore.RecordFindingFeedback(
			findingID,
			finding.ResourceID,
			string(finding.Category),
			"snooze",
			"",
			fmt.Sprintf("Snoozed for %v", duration),
		)
	}

	return i.store.Snooze(findingID, duration)
}

// enhanceFindingWithCorrelation enhances a finding using the correlation engine
func (i *Integration) enhanceFindingWithCorrelation(findingID string) error {
	if i.correlationEngine == nil {
		return nil
	}

	finding := i.store.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}

	rootCauseID, correlatedIDs, explanation, err := i.correlationEngine.AnalyzeForFinding(findingID, finding.ResourceID)
	if err != nil {
		return err
	}

	// Calculate confidence based on correlation results
	confidence := 0.5 // Base confidence
	if rootCauseID != "" {
		confidence += 0.3
	}
	if len(correlatedIDs) > 0 {
		confidence += 0.1 * float64(minInt(len(correlatedIDs), 2))
	}

	if !i.store.EnhanceWithAI(findingID, explanation, confidence, rootCauseID, correlatedIDs) {
		return fmt.Errorf("failed to enhance finding %s", findingID)
	}
	return nil
}

// GetActiveIssuesSummary returns a human-readable summary of active issues
func (i *Integration) GetActiveIssuesSummary() string {
	summary := i.store.GetSummary()
	if summary.Active == 0 {
		return "No active issues detected."
	}

	var parts []string

	if summary.Critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", summary.Critical))
	}
	if summary.Warning > 0 {
		parts = append(parts, fmt.Sprintf("%d warning", summary.Warning))
	}
	if summary.Watch > 0 {
		parts = append(parts, fmt.Sprintf("%d watch", summary.Watch))
	}
	if summary.Info > 0 {
		parts = append(parts, fmt.Sprintf("%d info", summary.Info))
	}

	result := fmt.Sprintf("%d active issues: %s", summary.Active, strings.Join(parts, ", "))

	if summary.EnhancedByAI > 0 {
		result += fmt.Sprintf(" (%d Patrol-enhanced)", summary.EnhancedByAI)
	}

	return result
}

// GetContextForPatrol returns context for AI patrol prompts
func (i *Integration) GetContextForPatrol() string {
	return i.store.FormatForContext()
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FindingsSnapshot represents a point-in-time view of findings for comparison
type FindingsSnapshot struct {
	Timestamp time.Time
	Active    []*UnifiedFinding
	Critical  int
	Warning   int
	BySource  map[FindingSource]int
}

// TakeSnapshot captures current state
func (i *Integration) TakeSnapshot() *FindingsSnapshot {
	active := i.store.GetActive()
	snapshot := &FindingsSnapshot{
		Timestamp: time.Now(),
		Active:    active,
		BySource:  make(map[FindingSource]int),
	}

	for _, f := range active {
		snapshot.BySource[f.Source]++
		if f.Severity == SeverityCritical {
			snapshot.Critical++
		} else if f.Severity == SeverityWarning {
			snapshot.Warning++
		}
	}

	return snapshot
}

// CompareSnapshots compares two snapshots and returns changes
func CompareSnapshots(before, after *FindingsSnapshot) *SnapshotDiff {
	diff := &SnapshotDiff{
		NewFindings:      make([]*UnifiedFinding, 0),
		ResolvedFindings: make([]*UnifiedFinding, 0),
		ChangedFindings:  make([]*UnifiedFinding, 0),
	}

	beforeIDs := make(map[string]*UnifiedFinding)
	for _, f := range before.Active {
		beforeIDs[f.ID] = f
	}

	afterIDs := make(map[string]*UnifiedFinding)
	for _, f := range after.Active {
		afterIDs[f.ID] = f
	}

	// Find new findings
	for id, f := range afterIDs {
		if _, exists := beforeIDs[id]; !exists {
			diff.NewFindings = append(diff.NewFindings, f)
		}
	}

	// Find resolved findings
	for id, f := range beforeIDs {
		if _, exists := afterIDs[id]; !exists {
			diff.ResolvedFindings = append(diff.ResolvedFindings, f)
		}
	}

	// Find changed findings (severity changed)
	for id, afterF := range afterIDs {
		if beforeF, exists := beforeIDs[id]; exists {
			if beforeF.Severity != afterF.Severity {
				diff.ChangedFindings = append(diff.ChangedFindings, afterF)
			}
		}
	}

	diff.CriticalDelta = after.Critical - before.Critical
	diff.WarningDelta = after.Warning - before.Warning

	return diff
}

// SnapshotDiff represents changes between two snapshots
type SnapshotDiff struct {
	NewFindings      []*UnifiedFinding
	ResolvedFindings []*UnifiedFinding
	ChangedFindings  []*UnifiedFinding
	CriticalDelta    int
	WarningDelta     int
}

// HasChanges returns true if there are any changes
func (d *SnapshotDiff) HasChanges() bool {
	return len(d.NewFindings) > 0 || len(d.ResolvedFindings) > 0 || len(d.ChangedFindings) > 0
}

// Summary returns a human-readable summary of changes
func (d *SnapshotDiff) Summary() string {
	if !d.HasChanges() {
		return "No changes detected."
	}

	var parts []string

	if len(d.NewFindings) > 0 {
		parts = append(parts, fmt.Sprintf("%d new issues", len(d.NewFindings)))
	}
	if len(d.ResolvedFindings) > 0 {
		parts = append(parts, fmt.Sprintf("%d resolved", len(d.ResolvedFindings)))
	}
	if len(d.ChangedFindings) > 0 {
		parts = append(parts, fmt.Sprintf("%d severity changes", len(d.ChangedFindings)))
	}

	return strings.Join(parts, ", ")
}
