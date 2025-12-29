package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/correlation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/patterns"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestIntelligence_formatBaselinesForContext(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	store := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	if err := store.Learn("res-1", "vm", "cpu", []baseline.MetricPoint{{Value: 12.5}}); err != nil {
		t.Fatalf("Learn: %v", err)
	}
	intel.SetSubsystems(nil, nil, nil, store, nil, nil, nil, nil)

	ctx := intel.formatBaselinesForContext("res-1")
	if !strings.Contains(ctx, "Learned Baselines") {
		t.Fatalf("expected baseline header, got %q", ctx)
	}
	if !strings.Contains(ctx, "cpu: mean") {
		t.Fatalf("expected cpu baseline line, got %q", ctx)
	}

	if got := intel.formatBaselinesForContext("missing"); got != "" {
		t.Errorf("expected empty context for missing baseline, got %q", got)
	}

	empty := NewIntelligence(IntelligenceConfig{})
	if got := empty.formatBaselinesForContext("res-1"); got != "" {
		t.Errorf("expected empty context with no baseline store, got %q", got)
	}
}

func TestIntelligence_formatAnomaliesForContext(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	if got := intel.formatAnomaliesForContext(nil); got != "" {
		t.Errorf("expected empty anomalies context, got %q", got)
	}

	anomalies := []AnomalyReport{{Metric: "cpu", Description: "CPU high"}}
	ctx := intel.formatAnomaliesForContext(anomalies)
	if !strings.Contains(ctx, "Current Anomalies") {
		t.Fatalf("expected anomalies header, got %q", ctx)
	}
	if !strings.Contains(ctx, "CPU high") {
		t.Fatalf("expected anomaly description, got %q", ctx)
	}
}

func TestIntelligence_formatAnomalyDescription(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	bl := &baseline.MetricBaseline{Mean: 10}

	above := intel.formatAnomalyDescription("cpu", 20, bl, 2.5)
	if !strings.Contains(above, "above baseline") {
		t.Errorf("expected above-baseline description, got %q", above)
	}

	below := intel.formatAnomalyDescription("cpu", 5, bl, -1.5)
	if !strings.Contains(below, "below baseline") {
		t.Errorf("expected below-baseline description, got %q", below)
	}
}

func TestIntelligence_FormatContext_Anomalies(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	intel.anomalyDetector = func(resourceID string) []AnomalyReport {
		return []AnomalyReport{{Metric: "cpu", Description: "CPU high"}}
	}

	ctx := intel.FormatContext("res-1")
	if !strings.Contains(ctx, "Current Anomalies") {
		t.Fatalf("expected anomalies context, got %q", ctx)
	}
	if !strings.Contains(ctx, "CPU high") {
		t.Fatalf("expected anomaly description, got %q", ctx)
	}
}

func TestIntelligence_getUpcomingRisks(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	predictions := []patterns.FailurePrediction{
		{ResourceID: "r1", EventType: patterns.EventHighCPU, DaysUntil: 2, Confidence: 0.6},
		{ResourceID: "r2", EventType: patterns.EventHighMemory, DaysUntil: 9, Confidence: 0.9},
		{ResourceID: "r3", EventType: patterns.EventDiskFull, DaysUntil: 4, Confidence: 0.4},
		{ResourceID: "r4", EventType: patterns.EventOOM, DaysUntil: 1, Confidence: 0.8},
		{ResourceID: "r5", EventType: patterns.EventRestart, DaysUntil: 5, Confidence: 0.7},
	}

	risk := intel.getUpcomingRisks(predictions, 2)
	if len(risk) != 2 {
		t.Fatalf("expected 2 risks, got %d", len(risk))
	}
	if risk[0].ResourceID != "r4" || risk[1].ResourceID != "r1" {
		t.Errorf("unexpected ordering: %v", []string{risk[0].ResourceID, risk[1].ResourceID})
	}
}

func TestIntelligence_countFindings(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	findings := []*Finding{
		nil,
		{Severity: FindingSeverityCritical},
		{Severity: FindingSeverityWarning},
		{Severity: FindingSeverityWatch},
		{Severity: FindingSeverityInfo},
	}
	counts := intel.countFindings(findings)
	if counts.Total != 4 {
		t.Errorf("expected total 4, got %d", counts.Total)
	}
	if counts.Critical != 1 || counts.Warning != 1 || counts.Watch != 1 || counts.Info != 1 {
		t.Errorf("unexpected counts: %+v", counts)
	}
}

func TestIntelligence_getTopFindings(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	base := time.Now()
	findings := []*Finding{
		{Severity: FindingSeverityWarning, DetectedAt: base.Add(-2 * time.Hour)},
		{Severity: FindingSeverityCritical, DetectedAt: base.Add(-3 * time.Hour), Title: "older critical"},
		{Severity: FindingSeverityCritical, DetectedAt: base.Add(-1 * time.Hour), Title: "newer critical"},
		{Severity: FindingSeverityWatch, DetectedAt: base.Add(-30 * time.Minute)},
	}

	top := intel.getTopFindings(findings, 3)
	if len(top) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(top))
	}
	if top[0].Title != "newer critical" || top[1].Title != "older critical" {
		t.Errorf("unexpected ordering: %q, %q", top[0].Title, top[1].Title)
	}
	if top[2].Severity != FindingSeverityWarning {
		t.Errorf("expected warning in third position, got %s", top[2].Severity)
	}
}

func TestIntelligence_getTopFindings_Empty(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	if got := intel.getTopFindings(nil, 5); got != nil {
		t.Errorf("expected nil for empty findings, got %v", got)
	}
}

func TestIntelligence_getLearningStats(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	knowledgeStore.SaveNote("vm-1", "vm-1", "vm", "general", "Note", "Content")
	knowledgeStore.SaveNote("vm-2", "vm-2", "vm", "general", "Note", "Content")

	baselineStore := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	baselineStore.Learn("vm-1", "vm", "cpu", []baseline.MetricPoint{{Value: 10}})

	patternDetector := patterns.NewDetector(patterns.DetectorConfig{
		MinOccurrences:  2,
		PatternWindow:   48 * time.Hour,
		PredictionLimit: 30 * 24 * time.Hour,
	})
	patternStart := time.Now().Add(-90 * time.Minute)
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-1", EventType: patterns.EventHighCPU, Timestamp: patternStart})
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-1", EventType: patterns.EventHighCPU, Timestamp: patternStart.Add(60 * time.Minute)})

	correlationDetector := correlation.NewDetector(correlation.Config{
		MinOccurrences:    1,
		CorrelationWindow: 2 * time.Hour,
		RetentionWindow:   24 * time.Hour,
	})
	corrStart := time.Now().Add(-30 * time.Minute)
	for i := 0; i < 2; i++ {
		base := corrStart.Add(time.Duration(i) * 10 * time.Minute)
		correlationDetector.RecordEvent(correlation.Event{ResourceID: "node-a", ResourceName: "node-a", ResourceType: "node", EventType: correlation.EventHighCPU, Timestamp: base})
		correlationDetector.RecordEvent(correlation.Event{ResourceID: "vm-b", ResourceName: "vm-b", ResourceType: "vm", EventType: correlation.EventRestart, Timestamp: base.Add(1 * time.Minute)})
	}

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{MaxIncidents: 5})

	intel.SetSubsystems(nil, patternDetector, correlationDetector, baselineStore, incidentStore, knowledgeStore, nil, nil)

	stats := intel.getLearningStats()
	if stats.ResourcesWithKnowledge != 2 {
		t.Errorf("expected 2 resources with knowledge, got %d", stats.ResourcesWithKnowledge)
	}
	if stats.TotalNotes != 2 {
		t.Errorf("expected 2 total notes, got %d", stats.TotalNotes)
	}
	if stats.ResourcesWithBaselines != 1 {
		t.Errorf("expected 1 resource with baseline, got %d", stats.ResourcesWithBaselines)
	}
	if stats.PatternsDetected != 1 {
		t.Errorf("expected 1 pattern, got %d", stats.PatternsDetected)
	}
	if stats.CorrelationsLearned != 1 {
		t.Errorf("expected 1 correlation, got %d", stats.CorrelationsLearned)
	}
}

func TestIntelligence_getResourcesAtRisk(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	findings := NewFindingsStore()
	findings.Add(&Finding{
		ID:           "crit-1",
		Key:          "crit-1",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryReliability,
		ResourceID:   "res-critical",
		ResourceName: "critical-vm",
		ResourceType: "vm",
		Title:        "Critical outage",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})
	findings.Add(&Finding{
		ID:           "warn-1",
		Key:          "warn-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "res-warning",
		ResourceName: "warning-vm",
		ResourceType: "vm",
		Title:        "Warning issue",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})
	findings.Add(&Finding{
		ID:           "watch-1",
		Key:          "watch-1",
		Severity:     FindingSeverityWatch,
		Category:     FindingCategoryPerformance,
		ResourceID:   "res-warning",
		ResourceName: "warning-vm",
		ResourceType: "vm",
		Title:        "Watch issue",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})
	findings.Add(&Finding{
		ID:           "info-1",
		Key:          "info-1",
		Severity:     FindingSeverityInfo,
		Category:     FindingCategoryPerformance,
		ResourceID:   "res-warning",
		ResourceName: "warning-vm",
		ResourceType: "vm",
		Title:        "Info issue",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})

	intel.SetSubsystems(findings, nil, nil, nil, nil, nil, nil, nil)
	risks := intel.getResourcesAtRisk(1)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d", len(risks))
	}
	if risks[0].ResourceID != "res-critical" {
		t.Errorf("expected critical resource first, got %s", risks[0].ResourceID)
	}
	if risks[0].TopIssue != "Critical outage" {
		t.Errorf("expected top issue to be critical, got %s", risks[0].TopIssue)
	}
}

func TestIntelligence_calculateResourceHealth_ClampAndSeverities(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	resourceIntel := &ResourceIntelligence{
		ResourceID: "test-vm",
		ActiveFindings: []*Finding{
			nil,
			{Severity: FindingSeverityCritical, Title: "crit-1"},
			{Severity: FindingSeverityCritical, Title: "crit-2"},
			{Severity: FindingSeverityCritical, Title: "crit-3"},
			{Severity: FindingSeverityCritical, Title: "crit-4"},
			{Severity: FindingSeverityWatch, Title: "watch"},
			{Severity: FindingSeverityInfo, Title: "info"},
		},
		Anomalies: []AnomalyReport{
			{Metric: "cpu", Severity: baseline.AnomalyHigh, Description: "high"},
			{Metric: "disk", Severity: baseline.AnomalyLow, Description: "low"},
		},
	}

	health := intel.calculateResourceHealth(resourceIntel)
	if health.Score != 0 {
		t.Errorf("expected score clamped to 0, got %f", health.Score)
	}
}

func TestIntelligence_calculateOverallHealth_Clamps(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	var predictions []patterns.FailurePrediction
	for i := 0; i < 10; i++ {
		predictions = append(predictions, patterns.FailurePrediction{EventType: patterns.EventHighCPU, DaysUntil: 1, Confidence: 0.9})
	}
	negative := intel.calculateOverallHealth(&IntelligenceSummary{
		FindingsCount: FindingsCounts{Critical: 10, Warning: 10},
		UpcomingRisks: predictions,
	})
	if negative.Score != 0 {
		t.Errorf("expected score clamped to 0, got %f", negative.Score)
	}

	positive := intel.calculateOverallHealth(&IntelligenceSummary{
		Learning: LearningStats{ResourcesWithKnowledge: 10},
	})
	if positive.Score != 100 {
		t.Errorf("expected score clamped to 100, got %f", positive.Score)
	}
	if len(positive.Factors) == 0 {
		t.Error("expected learning factor for positive health")
	}
}

func TestIntelligence_generateHealthPrediction_Branches(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	gradeA := intel.generateHealthPrediction(HealthScore{Grade: HealthGradeA}, &IntelligenceSummary{})
	if !strings.Contains(gradeA, "healthy") {
		t.Errorf("expected healthy prediction, got %q", gradeA)
	}

	critical := intel.generateHealthPrediction(HealthScore{Grade: HealthGradeB}, &IntelligenceSummary{
		FindingsCount: FindingsCounts{Critical: 2},
	})
	if !strings.Contains(critical, "Immediate attention") {
		t.Errorf("expected critical prediction, got %q", critical)
	}

	risk := intel.generateHealthPrediction(HealthScore{Grade: HealthGradeB}, &IntelligenceSummary{
		UpcomingRisks: []patterns.FailurePrediction{{EventType: patterns.EventHighCPU, DaysUntil: 2, Confidence: 0.8}},
	})
	if !strings.Contains(risk, "Predicted") {
		t.Errorf("expected prediction text, got %q", risk)
	}

	stable := intel.generateHealthPrediction(HealthScore{Grade: HealthGradeC}, &IntelligenceSummary{})
	if !strings.Contains(stable, "stable") {
		t.Errorf("expected stable prediction, got %q", stable)
	}
}

func TestIntelligence_FormatContext_AllSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	knowledgeStore, _ := knowledge.NewStore(t.TempDir())
	knowledgeStore.SaveNote("vm-ctx", "vm-ctx", "vm", "general", "Note", "Content")

	baselineStore := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	baselineStore.Learn("vm-ctx", "vm", "cpu", []baseline.MetricPoint{{Value: 10}})

	patternDetector := patterns.NewDetector(patterns.DetectorConfig{MinOccurrences: 2, PatternWindow: 48 * time.Hour, PredictionLimit: 30 * 24 * time.Hour})
	patternStart := time.Now().Add(-90 * time.Minute)
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-ctx", EventType: patterns.EventHighCPU, Timestamp: patternStart})
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-ctx", EventType: patterns.EventHighCPU, Timestamp: patternStart.Add(60 * time.Minute)})

	correlationDetector := correlation.NewDetector(correlation.Config{MinOccurrences: 1, CorrelationWindow: 2 * time.Hour, RetentionWindow: 24 * time.Hour})
	corrBase := time.Now().Add(-30 * time.Minute)
	correlationDetector.RecordEvent(correlation.Event{ResourceID: "vm-ctx", ResourceName: "vm-ctx", ResourceType: "vm", EventType: correlation.EventHighCPU, Timestamp: corrBase})
	correlationDetector.RecordEvent(correlation.Event{ResourceID: "node-ctx", ResourceName: "node-ctx", ResourceType: "node", EventType: correlation.EventRestart, Timestamp: corrBase.Add(1 * time.Minute)})

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{MaxIncidents: 10})
	incidentStore.RecordAlertFired(&alerts.Alert{ID: "alert-ctx", ResourceID: "vm-ctx", ResourceName: "vm-ctx", Type: "cpu", StartTime: time.Now()})

	intel.SetSubsystems(nil, patternDetector, correlationDetector, baselineStore, incidentStore, knowledgeStore, nil, nil)
	ctx := intel.FormatContext("vm-ctx")
	if !strings.Contains(ctx, "Learned Baselines") {
		t.Fatalf("expected baseline context, got %q", ctx)
	}
	if !strings.Contains(ctx, "Failure Predictions") {
		t.Fatalf("expected predictions context, got %q", ctx)
	}
	if !strings.Contains(ctx, "Resource Correlations") {
		t.Fatalf("expected correlations context, got %q", ctx)
	}
	if !strings.Contains(ctx, "Incident Memory") {
		t.Fatalf("expected incident context, got %q", ctx)
	}
}

func TestIntelligence_FormatGlobalContext_AllSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	knowledgeStore, _ := knowledge.NewStore(t.TempDir())
	knowledgeStore.SaveNote("vm-global", "vm-global", "vm", "general", "Note", "Content")

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{MaxIncidents: 10})
	incidentStore.RecordAlertFired(&alerts.Alert{ID: "alert-global", ResourceID: "vm-global", ResourceName: "vm-global", Type: "cpu", StartTime: time.Now()})

	correlationDetector := correlation.NewDetector(correlation.Config{MinOccurrences: 1, CorrelationWindow: 2 * time.Hour, RetentionWindow: 24 * time.Hour})
	corrBase := time.Now().Add(-30 * time.Minute)
	for i := 0; i < 2; i++ {
		base := corrBase.Add(time.Duration(i) * 10 * time.Minute)
		correlationDetector.RecordEvent(correlation.Event{ResourceID: "node-a", ResourceName: "node-a", ResourceType: "node", EventType: correlation.EventHighCPU, Timestamp: base})
		correlationDetector.RecordEvent(correlation.Event{ResourceID: "vm-global", ResourceName: "vm-global", ResourceType: "vm", EventType: correlation.EventRestart, Timestamp: base.Add(1 * time.Minute)})
	}

	patternDetector := patterns.NewDetector(patterns.DetectorConfig{MinOccurrences: 2, PatternWindow: 48 * time.Hour, PredictionLimit: 30 * 24 * time.Hour})
	patternStart := time.Now().Add(-90 * time.Minute)
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-global", EventType: patterns.EventHighCPU, Timestamp: patternStart})
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-global", EventType: patterns.EventHighCPU, Timestamp: patternStart.Add(60 * time.Minute)})

	intel.SetSubsystems(nil, patternDetector, correlationDetector, nil, incidentStore, knowledgeStore, nil, nil)
	ctx := intel.FormatGlobalContext()
	if ctx == "" {
		t.Fatal("expected non-empty global context")
	}
	if !strings.Contains(ctx, "Resource Correlations") {
		t.Fatalf("expected correlations in global context, got %q", ctx)
	}
}

func TestIntelligence_GetSummary_WithSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	findings := NewFindingsStore()
	findings.Add(&Finding{
		ID:           "f1",
		Key:          "f1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-sum",
		ResourceName: "vm-sum",
		ResourceType: "vm",
		Title:        "Warning",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})

	patternDetector := patterns.NewDetector(patterns.DetectorConfig{MinOccurrences: 2, PatternWindow: 48 * time.Hour, PredictionLimit: 30 * 24 * time.Hour})
	patternStart := time.Now().Add(-90 * time.Minute)
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-sum", EventType: patterns.EventHighCPU, Timestamp: patternStart})
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-sum", EventType: patterns.EventHighCPU, Timestamp: patternStart.Add(60 * time.Minute)})

	changes := memory.NewChangeDetector(memory.ChangeDetectorConfig{MaxChanges: 10})
	changes.DetectChanges([]memory.ResourceSnapshot{{ID: "vm-sum", Name: "vm-sum", Type: "vm", Status: "running", SnapshotTime: time.Now()}})

	remediations := memory.NewRemediationLog(memory.RemediationLogConfig{MaxRecords: 10})
	remediations.Log(memory.RemediationRecord{ResourceID: "vm-sum", Problem: "cpu", Action: "restart", Outcome: memory.OutcomeResolved})

	intel.SetSubsystems(findings, patternDetector, nil, nil, nil, nil, changes, remediations)

	summary := intel.GetSummary()
	if summary.FindingsCount.Total == 0 {
		t.Error("expected findings in summary")
	}
	if summary.PredictionsCount == 0 {
		t.Error("expected predictions in summary")
	}
	if len(summary.UpcomingRisks) == 0 {
		t.Error("expected upcoming risks in summary")
	}
	if summary.RecentChangesCount == 0 {
		t.Error("expected recent changes in summary")
	}
	if len(summary.RecentRemediations) == 0 {
		t.Error("expected recent remediations in summary")
	}
	if len(summary.ResourcesAtRisk) == 0 {
		t.Error("expected resources at risk in summary")
	}
}

func TestIntelligence_GetResourceIntelligence_WithAllSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	findings := NewFindingsStore()
	findings.Add(&Finding{
		ID:           "f1",
		Key:          "f1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-intel",
		ResourceName: "vm-intel",
		ResourceType: "vm",
		Title:        "Warning",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})

	patternDetector := patterns.NewDetector(patterns.DetectorConfig{MinOccurrences: 2, PatternWindow: 48 * time.Hour, PredictionLimit: 30 * 24 * time.Hour})
	patternStart := time.Now().Add(-90 * time.Minute)
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-intel", EventType: patterns.EventHighCPU, Timestamp: patternStart})
	patternDetector.RecordEvent(patterns.HistoricalEvent{ResourceID: "vm-intel", EventType: patterns.EventHighCPU, Timestamp: patternStart.Add(60 * time.Minute)})

	correlationDetector := correlation.NewDetector(correlation.Config{MinOccurrences: 1, CorrelationWindow: 2 * time.Hour, RetentionWindow: 24 * time.Hour})
	corrBase := time.Now().Add(-30 * time.Minute)
	correlationDetector.RecordEvent(correlation.Event{ResourceID: "node-intel", ResourceName: "node-intel", ResourceType: "node", EventType: correlation.EventHighCPU, Timestamp: corrBase})
	correlationDetector.RecordEvent(correlation.Event{ResourceID: "vm-intel", ResourceName: "vm-intel", ResourceType: "vm", EventType: correlation.EventRestart, Timestamp: corrBase.Add(1 * time.Minute)})
	correlationDetector.RecordEvent(correlation.Event{ResourceID: "vm-intel", ResourceName: "vm-intel", ResourceType: "vm", EventType: correlation.EventHighCPU, Timestamp: corrBase.Add(2 * time.Minute)})
	correlationDetector.RecordEvent(correlation.Event{ResourceID: "vm-child", ResourceName: "vm-child", ResourceType: "vm", EventType: correlation.EventRestart, Timestamp: corrBase.Add(3 * time.Minute)})

	baselineStore := baseline.NewStore(baseline.StoreConfig{MinSamples: 1})
	baselineStore.Learn("vm-intel", "vm", "cpu", []baseline.MetricPoint{{Value: 10}})

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{MaxIncidents: 10})
	incidentStore.RecordAlertFired(&alerts.Alert{ID: "alert-intel", ResourceID: "vm-intel", ResourceName: "vm-intel", Type: "cpu", StartTime: time.Now()})

	knowledgeStore, _ := knowledge.NewStore(t.TempDir())
	knowledgeStore.SaveNote("vm-intel", "vm-intel", "vm", "general", "Note", "Content")

	intel.SetSubsystems(findings, patternDetector, correlationDetector, baselineStore, incidentStore, knowledgeStore, nil, nil)
	res := intel.GetResourceIntelligence("vm-intel")
	if len(res.ActiveFindings) == 0 {
		t.Error("expected active findings")
	}
	if len(res.Predictions) == 0 {
		t.Error("expected predictions")
	}
	if len(res.Correlations) == 0 {
		t.Error("expected correlations")
	}
	if len(res.Dependents) == 0 || len(res.Dependencies) == 0 {
		t.Error("expected dependencies and dependents")
	}
	if len(res.Baselines) == 0 {
		t.Error("expected baselines")
	}
	if len(res.RecentIncidents) == 0 {
		t.Error("expected incidents")
	}
	if res.Knowledge == nil || res.NoteCount == 0 {
		t.Error("expected knowledge")
	}
}

func TestIntelligence_GetResourceIntelligence_KnowledgeFallback(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})
	knowledgeStore, _ := knowledge.NewStore(t.TempDir())
	knowledgeStore.SaveNote("vm-know", "knowledge-vm", "vm", "general", "Note", "Content")

	intel.SetSubsystems(nil, nil, nil, nil, nil, knowledgeStore, nil, nil)
	res := intel.GetResourceIntelligence("vm-know")
	if res.ResourceName != "knowledge-vm" {
		t.Errorf("expected resource name from knowledge, got %q", res.ResourceName)
	}
	if res.ResourceType != "vm" {
		t.Errorf("expected resource type from knowledge, got %q", res.ResourceType)
	}
}
