// Package ai provides AI-powered infrastructure monitoring and investigation.
// This file contains the unified AIIntelligence orchestrator that ties together
// all AI subsystems into one coherent intelligence layer.
package ai

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/correlation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/patterns"
)

// HealthGrade represents the overall health assessment
type HealthGrade string

const (
	HealthGradeA HealthGrade = "A" // Excellent - no issues
	HealthGradeB HealthGrade = "B" // Good - minor issues
	HealthGradeC HealthGrade = "C" // Fair - some concerns
	HealthGradeD HealthGrade = "D" // Poor - needs attention
	HealthGradeF HealthGrade = "F" // Critical - immediate action needed
)

// HealthTrend indicates the direction of health over time
type HealthTrend string

const (
	HealthTrendImproving HealthTrend = "improving"
	HealthTrendStable    HealthTrend = "stable"
	HealthTrendDeclining HealthTrend = "declining"
)

// HealthFactor represents a single component affecting health
type HealthFactor struct {
	Name        string  `json:"name"`
	Impact      float64 `json:"impact"` // -1 to 1, negative is bad
	Description string  `json:"description"`
	Category    string  `json:"category"` // finding, prediction, baseline, incident
}

// HealthScore represents the overall health of a resource or system
type HealthScore struct {
	Score      float64        `json:"score"`      // 0-100
	Grade      HealthGrade    `json:"grade"`      // A, B, C, D, F
	Trend      HealthTrend    `json:"trend"`      // improving, stable, declining
	Factors    []HealthFactor `json:"factors"`    // What's affecting the score
	Prediction string         `json:"prediction"` // Human-readable outlook
}

// ResourceIntelligence aggregates all AI knowledge about a single resource
type ResourceIntelligence struct {
	ResourceID      string                            `json:"resource_id"`
	ResourceName    string                            `json:"resource_name,omitempty"`
	ResourceType    string                            `json:"resource_type,omitempty"`
	Health          HealthScore                       `json:"health"`
	ActiveFindings  []*Finding                        `json:"active_findings,omitempty"`
	Predictions     []patterns.FailurePrediction      `json:"predictions,omitempty"`
	Dependencies    []string                          `json:"dependencies,omitempty"` // Resources this depends on
	Dependents      []string                          `json:"dependents,omitempty"`   // Resources that depend on this
	Correlations    []*correlation.Correlation        `json:"correlations,omitempty"`
	Baselines       map[string]*baseline.FlatBaseline `json:"baselines,omitempty"`
	Anomalies       []AnomalyReport                   `json:"anomalies,omitempty"`
	RecentIncidents []*memory.Incident                `json:"recent_incidents,omitempty"`
	Knowledge       *knowledge.GuestKnowledge         `json:"knowledge,omitempty"`
	NoteCount       int                               `json:"note_count"`
}

// AnomalyReport describes a metric that's deviating from baseline
type AnomalyReport struct {
	Metric       string                   `json:"metric"`
	CurrentValue float64                  `json:"current_value"`
	BaselineMean float64                  `json:"baseline_mean"`
	ZScore       float64                  `json:"z_score"`
	Severity     baseline.AnomalySeverity `json:"severity"`
	Description  string                   `json:"description"`
}

// IntelligenceSummary provides a system-wide intelligence overview
type IntelligenceSummary struct {
	Timestamp     time.Time   `json:"timestamp"`
	OverallHealth HealthScore `json:"overall_health"`

	// Findings summary
	FindingsCount FindingsCounts `json:"findings_count"`
	TopFindings   []*Finding     `json:"top_findings,omitempty"` // Most critical

	// Predictions
	PredictionsCount int                          `json:"predictions_count"`
	UpcomingRisks    []patterns.FailurePrediction `json:"upcoming_risks,omitempty"`

	// Recent activity
	RecentChangesCount int                        `json:"recent_changes_count"`
	RecentRemediations []memory.RemediationRecord `json:"recent_remediations,omitempty"`

	// Learning progress
	Learning LearningStats `json:"learning"`

	// Resources needing attention
	ResourcesAtRisk []ResourceRiskSummary `json:"resources_at_risk,omitempty"`
}

// FindingsCounts provides a breakdown of findings by severity
type FindingsCounts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Watch    int `json:"watch"`
	Info     int `json:"info"`
	Total    int `json:"total"`
}

// LearningStats shows how much the AI has learned
type LearningStats struct {
	ResourcesWithKnowledge int `json:"resources_with_knowledge"`
	TotalNotes             int `json:"total_notes"`
	ResourcesWithBaselines int `json:"resources_with_baselines"`
	PatternsDetected       int `json:"patterns_detected"`
	CorrelationsLearned    int `json:"correlations_learned"`
	IncidentsTracked       int `json:"incidents_tracked"`
}

// ResourceRiskSummary is a brief summary of a resource at risk
type ResourceRiskSummary struct {
	ResourceID   string      `json:"resource_id"`
	ResourceName string      `json:"resource_name"`
	ResourceType string      `json:"resource_type"`
	Health       HealthScore `json:"health"`
	TopIssue     string      `json:"top_issue"`
}

// Intelligence orchestrates all AI subsystems into a unified system
type Intelligence struct {
	mu sync.RWMutex

	// Core subsystems
	findings     *FindingsStore
	patterns     *patterns.Detector
	correlations *correlation.Detector
	baselines    *baseline.Store
	incidents    *memory.IncidentStore
	knowledge    *knowledge.Store
	changes      *memory.ChangeDetector
	remediations *memory.RemediationLog

	// State access
	stateProvider StateProvider

	// Optional hook for anomaly detection (used by patrol integration/tests)
	anomalyDetector func(resourceID string) []AnomalyReport

	// Configuration
	dataDir string
}

// IntelligenceConfig configures the unified intelligence layer
type IntelligenceConfig struct {
	DataDir string
}

// NewIntelligence creates a new unified intelligence orchestrator
func NewIntelligence(cfg IntelligenceConfig) *Intelligence {
	return &Intelligence{
		dataDir: cfg.DataDir,
	}
}

// SetSubsystems wires up all the AI subsystems
func (i *Intelligence) SetSubsystems(
	findings *FindingsStore,
	patternsDetector *patterns.Detector,
	correlationsDetector *correlation.Detector,
	baselinesStore *baseline.Store,
	incidentsStore *memory.IncidentStore,
	knowledgeStore *knowledge.Store,
	changesDetector *memory.ChangeDetector,
	remediationsLog *memory.RemediationLog,
) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.findings = findings
	i.patterns = patternsDetector
	i.correlations = correlationsDetector
	i.baselines = baselinesStore
	i.incidents = incidentsStore
	i.knowledge = knowledgeStore
	i.changes = changesDetector
	i.remediations = remediationsLog
}

// SetStateProvider sets the state provider for current metrics
func (i *Intelligence) SetStateProvider(sp StateProvider) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stateProvider = sp
}

// GetSummary returns a comprehensive intelligence summary
func (i *Intelligence) GetSummary() *IntelligenceSummary {
	i.mu.RLock()
	defer i.mu.RUnlock()

	summary := &IntelligenceSummary{
		Timestamp: time.Now(),
	}

	// Aggregate findings
	if i.findings != nil {
		all := i.findings.GetActive(FindingSeverityInfo) // Get all active findings
		summary.FindingsCount = i.countFindings(all)
		summary.TopFindings = i.getTopFindings(all, 5)
	}

	// Aggregate predictions
	if i.patterns != nil {
		predictions := i.patterns.GetPredictions()
		summary.PredictionsCount = len(predictions)
		summary.UpcomingRisks = i.getUpcomingRisks(predictions, 5)
	}

	// Aggregate recent activity
	if i.changes != nil {
		recent := i.changes.GetRecentChanges(100, time.Now().Add(-24*time.Hour))
		summary.RecentChangesCount = len(recent)
	}

	if i.remediations != nil {
		recent := i.remediations.GetRecentRemediations(5, time.Now().Add(-24*time.Hour))
		summary.RecentRemediations = recent
	}

	// Learning stats
	summary.Learning = i.getLearningStats()

	// Calculate overall health
	summary.OverallHealth = i.calculateOverallHealth(summary)

	// Resources at risk
	summary.ResourcesAtRisk = i.getResourcesAtRisk(5)

	return summary
}

// GetResourceIntelligence returns aggregated intelligence for a specific resource
func (i *Intelligence) GetResourceIntelligence(resourceID string) *ResourceIntelligence {
	i.mu.RLock()
	defer i.mu.RUnlock()

	intel := &ResourceIntelligence{
		ResourceID: resourceID,
	}

	// Active findings
	if i.findings != nil {
		intel.ActiveFindings = i.findings.GetByResource(resourceID)
		if len(intel.ActiveFindings) > 0 {
			intel.ResourceName = intel.ActiveFindings[0].ResourceName
			intel.ResourceType = intel.ActiveFindings[0].ResourceType
		}
	}

	// Predictions
	if i.patterns != nil {
		intel.Predictions = i.patterns.GetPredictionsForResource(resourceID)
	}

	// Correlations and dependencies
	if i.correlations != nil {
		intel.Correlations = i.correlations.GetCorrelationsForResource(resourceID)
		intel.Dependencies = i.correlations.GetDependsOn(resourceID)
		intel.Dependents = i.correlations.GetDependencies(resourceID)
	}

	// Baselines
	if i.baselines != nil {
		if rb, ok := i.baselines.GetResourceBaseline(resourceID); ok {
			intel.Baselines = make(map[string]*baseline.FlatBaseline)
			for metric, mb := range rb.Metrics {
				intel.Baselines[metric] = &baseline.FlatBaseline{
					ResourceID: resourceID,
					Metric:     metric,
					Mean:       mb.Mean,
					StdDev:     mb.StdDev,
					Samples:    mb.SampleCount,
					LastUpdate: rb.LastUpdated,
				}
			}
		}
	}

	// Recent incidents
	if i.incidents != nil {
		intel.RecentIncidents = i.incidents.ListIncidentsByResource(resourceID, 5)
	}

	// Knowledge
	if i.knowledge != nil {
		if k, err := i.knowledge.GetKnowledge(resourceID); err == nil && k != nil {
			intel.Knowledge = k
			intel.NoteCount = len(k.Notes)
			if intel.ResourceName == "" && k.GuestName != "" {
				intel.ResourceName = k.GuestName
			}
			if intel.ResourceType == "" && k.GuestType != "" {
				intel.ResourceType = k.GuestType
			}
		}
	}

	// Calculate health score
	intel.Health = i.calculateResourceHealth(intel)

	return intel
}

// FormatContext builds a comprehensive context string for AI prompts
func (i *Intelligence) FormatContext(resourceID string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var sections []string

	// Knowledge (most important - what we've learned)
	if i.knowledge != nil {
		if ctx := i.knowledge.FormatForContext(resourceID); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Baselines (what's normal for this resource)
	if i.baselines != nil {
		if ctx := i.formatBaselinesForContext(resourceID); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Current anomalies
	if anomalies := i.detectCurrentAnomalies(resourceID); len(anomalies) > 0 {
		sections = append(sections, i.formatAnomaliesForContext(anomalies))
	}

	// Patterns/Predictions
	if i.patterns != nil {
		if ctx := i.patterns.FormatForContext(resourceID); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Correlations
	if i.correlations != nil {
		if ctx := i.correlations.FormatForContext(resourceID); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Incidents
	if i.incidents != nil {
		if ctx := i.incidents.FormatForResource(resourceID, 5); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	return strings.Join(sections, "\n")
}

// FormatGlobalContext builds context for infrastructure-wide analysis
func (i *Intelligence) FormatGlobalContext() string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var sections []string

	// All saved knowledge (limited)
	if i.knowledge != nil {
		if ctx := i.knowledge.FormatAllForContext(); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Recent incidents across infrastructure
	if i.incidents != nil {
		if ctx := i.incidents.FormatForPatrol(8); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Top correlations
	if i.correlations != nil {
		if ctx := i.correlations.FormatForContext(""); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	// Top predictions
	if i.patterns != nil {
		if ctx := i.patterns.FormatForContext(""); ctx != "" {
			sections = append(sections, ctx)
		}
	}

	return strings.Join(sections, "\n")
}

// RecordLearning saves a learning to the knowledge store after a fix
func (i *Intelligence) RecordLearning(resourceID, resourceName, resourceType, title, content string) error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.knowledge == nil {
		return nil
	}

	return i.knowledge.SaveNote(resourceID, resourceName, resourceType, "learning", title, content)
}

// CheckBaselinesForResource checks current metrics against baselines and returns anomalies
func (i *Intelligence) CheckBaselinesForResource(resourceID string, metrics map[string]float64) []AnomalyReport {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.baselines == nil {
		return nil
	}

	var anomalies []AnomalyReport
	for metric, value := range metrics {
		severity, zScore, bl := i.baselines.CheckAnomaly(resourceID, metric, value)
		if severity != baseline.AnomalyNone && bl != nil {
			anomalies = append(anomalies, AnomalyReport{
				Metric:       metric,
				CurrentValue: value,
				BaselineMean: bl.Mean,
				ZScore:       zScore,
				Severity:     severity,
				Description:  i.formatAnomalyDescription(metric, value, bl, zScore),
			})
		}
	}

	return anomalies
}

// CreatePredictionFinding creates a finding from a prediction that's imminent
func (i *Intelligence) CreatePredictionFinding(pred patterns.FailurePrediction) *Finding {
	severity := FindingSeverityWatch
	if pred.DaysUntil < 1 {
		severity = FindingSeverityWarning
	}
	if pred.Confidence > 0.8 && pred.DaysUntil < 1 {
		severity = FindingSeverityCritical
	}

	return &Finding{
		ID:          fmt.Sprintf("pred-%s-%s", pred.ResourceID, pred.EventType),
		Key:         fmt.Sprintf("prediction:%s:%s", pred.ResourceID, pred.EventType),
		Severity:    severity,
		Category:    FindingCategoryReliability,
		ResourceID:  pred.ResourceID,
		Title:       fmt.Sprintf("Predicted: %s", pred.EventType),
		Description: pred.Basis,
		DetectedAt:  time.Now(),
		LastSeenAt:  time.Now(),
	}
}

// Helper methods

func (i *Intelligence) countFindings(findings []*Finding) FindingsCounts {
	counts := FindingsCounts{}
	for _, f := range findings {
		if f == nil {
			continue
		}
		counts.Total++
		switch f.Severity {
		case FindingSeverityCritical:
			counts.Critical++
		case FindingSeverityWarning:
			counts.Warning++
		case FindingSeverityWatch:
			counts.Watch++
		case FindingSeverityInfo:
			counts.Info++
		}
	}
	return counts
}

func (i *Intelligence) getTopFindings(findings []*Finding, limit int) []*Finding {
	if len(findings) == 0 {
		return nil
	}

	// Sort by severity (critical first) then by detection time (newest first)
	sorted := make([]*Finding, len(findings))
	copy(sorted, findings)
	sort.Slice(sorted, func(a, b int) bool {
		sevA := severityOrder(sorted[a].Severity)
		sevB := severityOrder(sorted[b].Severity)
		if sevA != sevB {
			return sevA < sevB
		}
		return sorted[a].DetectedAt.After(sorted[b].DetectedAt)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

func severityOrder(s FindingSeverity) int {
	switch s {
	case FindingSeverityCritical:
		return 0
	case FindingSeverityWarning:
		return 1
	case FindingSeverityWatch:
		return 2
	case FindingSeverityInfo:
		return 3
	default:
		return 4
	}
}

func (i *Intelligence) getUpcomingRisks(predictions []patterns.FailurePrediction, limit int) []patterns.FailurePrediction {
	if len(predictions) == 0 {
		return nil
	}

	// Filter to next 7 days and sort by days until
	var upcoming []patterns.FailurePrediction
	for _, p := range predictions {
		if p.DaysUntil <= 7 && p.Confidence >= 0.5 {
			upcoming = append(upcoming, p)
		}
	}

	sort.Slice(upcoming, func(a, b int) bool {
		return upcoming[a].DaysUntil < upcoming[b].DaysUntil
	})

	if len(upcoming) > limit {
		upcoming = upcoming[:limit]
	}
	return upcoming
}

func (i *Intelligence) getLearningStats() LearningStats {
	stats := LearningStats{}

	if i.knowledge != nil {
		guests, _ := i.knowledge.ListGuests()
		for _, guestID := range guests {
			if k, err := i.knowledge.GetKnowledge(guestID); err == nil && k != nil && len(k.Notes) > 0 {
				stats.ResourcesWithKnowledge++
				stats.TotalNotes += len(k.Notes)
			}
		}
	}

	if i.baselines != nil {
		stats.ResourcesWithBaselines = i.baselines.ResourceCount()
	}

	if i.patterns != nil {
		p := i.patterns.GetPatterns()
		stats.PatternsDetected = len(p)
	}

	if i.correlations != nil {
		c := i.correlations.GetCorrelations()
		stats.CorrelationsLearned = len(c)
	}

	if i.incidents != nil {
		// Count is not available, so we skip this stat for now
		// Could be added to IncidentStore if needed
		stats.IncidentsTracked = 0
	}

	return stats
}

func (i *Intelligence) calculateOverallHealth(summary *IntelligenceSummary) HealthScore {
	health := HealthScore{
		Score:   100,
		Grade:   HealthGradeA,
		Trend:   HealthTrendStable,
		Factors: []HealthFactor{},
	}

	// Deduct for findings
	if summary.FindingsCount.Critical > 0 {
		impact := float64(summary.FindingsCount.Critical) * 20
		if impact > 40 {
			impact = 40
		}
		health.Score -= impact
		health.Factors = append(health.Factors, HealthFactor{
			Name:        "Critical findings",
			Impact:      -impact / 100,
			Description: fmt.Sprintf("%d critical issues need immediate attention", summary.FindingsCount.Critical),
			Category:    "finding",
		})
	}

	if summary.FindingsCount.Warning > 0 {
		impact := float64(summary.FindingsCount.Warning) * 10
		if impact > 20 {
			impact = 20
		}
		health.Score -= impact
		health.Factors = append(health.Factors, HealthFactor{
			Name:        "Warnings",
			Impact:      -impact / 100,
			Description: fmt.Sprintf("%d warnings need attention soon", summary.FindingsCount.Warning),
			Category:    "finding",
		})
	}

	// Deduct for imminent predictions
	for _, pred := range summary.UpcomingRisks {
		if pred.DaysUntil < 3 && pred.Confidence > 0.7 {
			impact := 10.0
			health.Score -= impact
			health.Factors = append(health.Factors, HealthFactor{
				Name:        "Predicted issue",
				Impact:      -impact / 100,
				Description: fmt.Sprintf("%s predicted within %.1f days", pred.EventType, pred.DaysUntil),
				Category:    "prediction",
			})
		}
	}

	// Bonus for learning progress
	if summary.Learning.ResourcesWithKnowledge > 5 {
		bonus := 5.0
		health.Score += bonus
		health.Factors = append(health.Factors, HealthFactor{
			Name:        "Knowledge learned",
			Impact:      bonus / 100,
			Description: fmt.Sprintf("Pulse Patrol has learned about %d resources", summary.Learning.ResourcesWithKnowledge),
			Category:    "learning",
		})
	}

	// Clamp score
	if health.Score < 0 {
		health.Score = 0
	}
	if health.Score > 100 {
		health.Score = 100
	}

	// Assign grade
	health.Grade = scoreToGrade(health.Score)

	// Generate prediction text
	health.Prediction = i.generateHealthPrediction(health, summary)

	return health
}

func (i *Intelligence) calculateResourceHealth(intel *ResourceIntelligence) HealthScore {
	health := HealthScore{
		Score:   100,
		Grade:   HealthGradeA,
		Trend:   HealthTrendStable,
		Factors: []HealthFactor{},
	}

	// Deduct for active findings
	for _, f := range intel.ActiveFindings {
		if f == nil {
			continue
		}
		var impact float64
		switch f.Severity {
		case FindingSeverityCritical:
			impact = 30
		case FindingSeverityWarning:
			impact = 15
		case FindingSeverityWatch:
			impact = 5
		case FindingSeverityInfo:
			impact = 2
		}
		health.Score -= impact
		health.Factors = append(health.Factors, HealthFactor{
			Name:        f.Title,
			Impact:      -impact / 100,
			Description: f.Description,
			Category:    "finding",
		})
	}

	// Deduct for predictions
	for _, p := range intel.Predictions {
		if p.DaysUntil < 7 && p.Confidence > 0.5 {
			impact := 10.0 * p.Confidence
			health.Score -= impact
			health.Factors = append(health.Factors, HealthFactor{
				Name:        "Predicted: " + string(p.EventType),
				Impact:      -impact / 100,
				Description: p.Basis,
				Category:    "prediction",
			})
		}
	}

	// Deduct for anomalies
	for _, a := range intel.Anomalies {
		var impact float64
		switch a.Severity {
		case baseline.AnomalyCritical:
			impact = 20
		case baseline.AnomalyHigh:
			impact = 10
		case baseline.AnomalyMedium:
			impact = 5
		case baseline.AnomalyLow:
			impact = 2
		}
		health.Score -= impact
		health.Factors = append(health.Factors, HealthFactor{
			Name:        a.Metric + " anomaly",
			Impact:      -impact / 100,
			Description: a.Description,
			Category:    "baseline",
		})
	}

	// Bonus for having knowledge
	if intel.NoteCount > 0 {
		bonus := 2.0
		health.Score += bonus
		health.Factors = append(health.Factors, HealthFactor{
			Name:        "Documented",
			Impact:      bonus / 100,
			Description: fmt.Sprintf("%d notes saved for this resource", intel.NoteCount),
			Category:    "learning",
		})
	}

	// Clamp
	if health.Score < 0 {
		health.Score = 0
	}
	if health.Score > 100 {
		health.Score = 100
	}

	health.Grade = scoreToGrade(health.Score)

	return health
}

func scoreToGrade(score float64) HealthGrade {
	switch {
	case score >= 90:
		return HealthGradeA
	case score >= 75:
		return HealthGradeB
	case score >= 60:
		return HealthGradeC
	case score >= 40:
		return HealthGradeD
	default:
		return HealthGradeF
	}
}

func (i *Intelligence) generateHealthPrediction(health HealthScore, summary *IntelligenceSummary) string {
	if health.Grade == HealthGradeA {
		return "Infrastructure is healthy with no significant issues detected."
	}

	if summary.FindingsCount.Critical > 0 {
		return fmt.Sprintf("Immediate attention required: %d critical issues.", summary.FindingsCount.Critical)
	}

	if len(summary.UpcomingRisks) > 0 {
		risk := summary.UpcomingRisks[0]
		return fmt.Sprintf("Predicted %s event on resource within %.1f days (%.0f%% confidence).",
			risk.EventType, risk.DaysUntil, risk.Confidence*100)
	}

	if summary.FindingsCount.Warning > 0 {
		return fmt.Sprintf("%d warnings should be addressed soon to maintain stability.", summary.FindingsCount.Warning)
	}

	return "Infrastructure is stable with minor issues to monitor."
}

func (i *Intelligence) getResourcesAtRisk(limit int) []ResourceRiskSummary {
	if i.findings == nil {
		return nil
	}

	// Group findings by resource
	byResource := make(map[string][]*Finding)
	for _, f := range i.findings.GetActive(FindingSeverityInfo) {
		byResource[f.ResourceID] = append(byResource[f.ResourceID], f)
	}

	// Calculate risk for each resource
	type resourceRisk struct {
		id    string
		name  string
		rtype string
		score float64
		top   string
	}

	var risks []resourceRisk
	for id, findings := range byResource {
		score := 0.0
		var topFinding *Finding
		for _, f := range findings {
			switch f.Severity {
			case FindingSeverityCritical:
				score += 30
			case FindingSeverityWarning:
				score += 15
			case FindingSeverityWatch:
				score += 5
			case FindingSeverityInfo:
				score += 2
			}
			if topFinding == nil || severityOrder(f.Severity) < severityOrder(topFinding.Severity) {
				topFinding = f
			}
		}

		if score > 0 && topFinding != nil {
			risks = append(risks, resourceRisk{
				id:    id,
				name:  topFinding.ResourceName,
				rtype: topFinding.ResourceType,
				score: score,
				top:   topFinding.Title,
			})
		}
	}

	// Sort by risk score descending
	sort.Slice(risks, func(a, b int) bool {
		return risks[a].score > risks[b].score
	})

	if len(risks) > limit {
		risks = risks[:limit]
	}

	// Convert to summaries
	var summaries []ResourceRiskSummary
	for _, r := range risks {
		health := HealthScore{
			Score: 100 - r.score,
			Grade: scoreToGrade(100 - r.score),
		}
		summaries = append(summaries, ResourceRiskSummary{
			ResourceID:   r.id,
			ResourceName: r.name,
			ResourceType: r.rtype,
			Health:       health,
			TopIssue:     r.top,
		})
	}

	return summaries
}

func (i *Intelligence) detectCurrentAnomalies(resourceID string) []AnomalyReport {
	if i.anomalyDetector != nil {
		return i.anomalyDetector(resourceID)
	}
	// This would be called with current metrics from state
	// For now, return empty - will be integrated with patrol
	return nil
}

func (i *Intelligence) formatBaselinesForContext(resourceID string) string {
	if i.baselines == nil {
		return ""
	}

	rb, ok := i.baselines.GetResourceBaseline(resourceID)
	if !ok || len(rb.Metrics) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "\n## Learned Baselines")
	lines = append(lines, "Normal operating ranges for this resource:")

	for metric, mb := range rb.Metrics {
		lines = append(lines, fmt.Sprintf("- %s: mean %.1f, stddev %.1f (samples: %d)",
			metric, mb.Mean, mb.StdDev, mb.SampleCount))
	}

	return strings.Join(lines, "\n")
}

func (i *Intelligence) formatAnomaliesForContext(anomalies []AnomalyReport) string {
	if len(anomalies) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "\n## Current Anomalies")
	lines = append(lines, "Metrics deviating from normal:")

	for _, a := range anomalies {
		lines = append(lines, fmt.Sprintf("- %s: %s", a.Metric, a.Description))
	}

	return strings.Join(lines, "\n")
}

func (i *Intelligence) formatAnomalyDescription(_ string, value float64, bl *baseline.MetricBaseline, zScore float64) string {
	direction := "above"
	if zScore < 0 {
		direction = "below"
	}
	return fmt.Sprintf("%.1f is %.1f std devs %s baseline (mean: %.1f)",
		value, absFloatIntel(zScore), direction, bl.Mean)
}

// absFloatIntel is a local helper (service.go has its own)
func absFloatIntel(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
