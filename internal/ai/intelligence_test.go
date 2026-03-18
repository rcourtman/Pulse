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
)

func TestNewIntelligence(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{DataDir: "/tmp/test"})
	if intel == nil {
		t.Fatal("Expected non-nil Intelligence")
	}
}

func TestIntelligence_GetSummary_NoSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	summary := intel.GetSummary()
	if summary == nil {
		t.Fatal("Expected non-nil summary")
	}

	// Should have default healthy state
	if summary.OverallHealth.Score != 100 {
		t.Errorf("Expected health score 100, got %f", summary.OverallHealth.Score)
	}
	if summary.OverallHealth.Grade != HealthGradeA {
		t.Errorf("Expected grade A, got %s", summary.OverallHealth.Grade)
	}
	if summary.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestIntelligence_GetResourceIntelligence_NoSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	resourceIntel := intel.GetResourceIntelligence("test-resource")
	if resourceIntel == nil {
		t.Fatal("Expected non-nil resource intelligence")
	}

	if resourceIntel.ResourceID != "test-resource" {
		t.Errorf("Expected resource ID 'test-resource', got %s", resourceIntel.ResourceID)
	}

	// Should have default healthy state
	if resourceIntel.Health.Score != 100 {
		t.Errorf("Expected health score 100, got %f", resourceIntel.Health.Score)
	}
}

func TestIntelligence_FormatContext_NoSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// With no subsystems, context should be empty
	ctx := intel.FormatContext("test-resource")
	if ctx != "" {
		t.Errorf("Expected empty context with no subsystems, got: %s", ctx)
	}
}

func TestIntelligence_CreatePredictionFinding(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	pred := patterns.FailurePrediction{
		ResourceID: "vm-100",
		EventType:  patterns.EventHighCPU, // Use the constant instead of cast
		DaysUntil:  0.5,                   // Less than 1 day
		Confidence: 0.85,                  // High confidence
		Basis:      "Pattern detected",
	}

	finding := intel.CreatePredictionFinding(pred)
	if finding == nil {
		t.Fatal("Expected non-nil finding")
	}

	// High confidence + < 1 day should be critical
	if finding.Severity != FindingSeverityCritical {
		t.Errorf("Expected critical severity for imminent high-confidence prediction, got %s", finding.Severity)
	}

	if finding.ResourceID != "vm-100" {
		t.Errorf("Expected resource ID 'vm-100', got %s", finding.ResourceID)
	}
}

func TestIntelligence_SetSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create a findings store
	findings := NewFindingsStore()

	// Add a finding
	findings.Add(&Finding{
		ID:           "test-finding",
		Key:          "test:finding",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-100",
		ResourceName: "test-vm",
		ResourceType: "vm",
		Title:        "Test Finding",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})

	// Set subsystems with just findings
	intel.SetSubsystems(findings, nil, nil, nil, nil, nil, nil, nil)

	// Get summary
	summary := intel.GetSummary()

	// Should have 1 warning
	if summary.FindingsCount.Warning != 1 {
		t.Errorf("Expected 1 warning, got %d", summary.FindingsCount.Warning)
	}
	if summary.FindingsCount.Total != 1 {
		t.Errorf("Expected 1 total finding, got %d", summary.FindingsCount.Total)
	}

	// Health should be reduced due to warning
	if summary.OverallHealth.Score >= 100 {
		t.Error("Expected health score < 100 due to warning finding")
	}
}

func TestIntelligence_HealthGrades(t *testing.T) {
	tests := []struct {
		score float64
		grade HealthGrade
	}{
		{100, HealthGradeA},
		{95, HealthGradeA},
		{90, HealthGradeA},
		{85, HealthGradeB},
		{75, HealthGradeB},
		{70, HealthGradeC},
		{60, HealthGradeC},
		{55, HealthGradeD},
		{40, HealthGradeD},
		{30, HealthGradeF},
		{0, HealthGradeF},
	}

	for _, tt := range tests {
		grade := scoreToGrade(tt.score)
		if grade != tt.grade {
			t.Errorf("scoreToGrade(%f) = %s, expected %s", tt.score, grade, tt.grade)
		}
	}
}

func TestIntelligence_CheckBaselinesForResource(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// With no baseline store, should return nil
	anomalies := intel.CheckBaselinesForResource("vm-100", map[string]float64{
		"cpu":    85.0,
		"memory": 90.0,
	})

	if anomalies != nil {
		t.Error("Expected nil anomalies when baseline store not configured")
	}
}

// Note: mockStateProvider is already defined in alert_triggered_test.go

func TestIntelligence_FormatGlobalContext(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// With no subsystems, should return empty
	ctx := intel.FormatGlobalContext()
	if ctx != "" {
		t.Errorf("Expected empty context with no subsystems, got: %s", ctx)
	}

	// Set up knowledge store
	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create knowledge store: %v", err)
	}
	intel.SetSubsystems(nil, nil, nil, nil, nil, knowledgeStore, nil, nil)

	// Should still be empty (no knowledge saved)
	ctx = intel.FormatGlobalContext()
	// Empty or with headers only is fine
}

func TestIntelligence_RecordLearning(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Without knowledge store, should return nil
	err := intel.RecordLearning("vm-100", "test-vm", "vm", "Test Title", "Test content")
	if err != nil {
		t.Errorf("Expected nil error without knowledge store, got: %v", err)
	}

	// With knowledge store
	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create knowledge store: %v", err)
	}
	intel.SetSubsystems(nil, nil, nil, nil, nil, knowledgeStore, nil, nil)

	err = intel.RecordLearning("vm-100", "test-vm", "vm", "Test Title", "Test content")
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}
}

func TestSeverityOrder(t *testing.T) {
	tests := []struct {
		severity FindingSeverity
		expected int
	}{
		{FindingSeverityCritical, 0},
		{FindingSeverityWarning, 1},
		{FindingSeverityWatch, 2},
		{FindingSeverityInfo, 3},
		{FindingSeverity("unknown"), 4},
	}

	for _, tt := range tests {
		result := severityOrder(tt.severity)
		if result != tt.expected {
			t.Errorf("severityOrder(%s) = %d, expected %d", tt.severity, result, tt.expected)
		}
	}
}

func TestIntelligence_CheckBaselinesForResource_WithBaselines(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create baseline store with learned data
	baselineStore := baseline.NewStore(baseline.StoreConfig{MinSamples: 10})

	// Learn baseline for CPU at ~20%
	points := make([]baseline.MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = baseline.MetricPoint{Value: 20 + float64(i%3) - 1} // 19-21
	}
	_ = baselineStore.Learn("vm-100", "vm", "cpu", points)

	intel.SetSubsystems(nil, nil, nil, baselineStore, nil, nil, nil, nil)

	// Check with anomalous value (80% is 4x baseline)
	anomalies := intel.CheckBaselinesForResource("vm-100", map[string]float64{
		"cpu": 80.0,
	})

	// Should detect anomaly for CPU (80% with baseline of 20% is 4x = anomalous)
	if len(anomalies) == 0 {
		t.Error("Expected anomaly for CPU at 80% with baseline of 20%")
	}
}

func TestIntelligence_GetResourceIntelligence_WithSubsystems(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create findings store with a finding for the resource
	findings := NewFindingsStore()
	findings.Add(&Finding{
		ID:           "f1",
		Key:          "test:f1",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-200",
		ResourceName: "critical-vm",
		ResourceType: "vm",
		Title:        "Critical CPU",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
		Source:       "test",
	})

	// Create correlation detector
	correlationDetector := correlation.NewDetector(correlation.DefaultConfig())

	intel.SetSubsystems(findings, nil, correlationDetector, nil, nil, nil, nil, nil)

	resourceIntel := intel.GetResourceIntelligence("vm-200")

	if len(resourceIntel.ActiveFindings) != 1 {
		t.Errorf("Expected 1 active finding, got %d", len(resourceIntel.ActiveFindings))
	}

	if resourceIntel.ResourceName != "critical-vm" {
		t.Errorf("Expected resource name 'critical-vm', got %s", resourceIntel.ResourceName)
	}

	// Health should be reduced due to critical finding
	if resourceIntel.Health.Score >= 100 {
		t.Error("Expected reduced health score due to critical finding")
	}

	// Grade should be less than A
	if resourceIntel.Health.Grade == HealthGradeA {
		t.Error("Expected health grade less than A due to critical finding")
	}
}

func TestIntelligence_FormatContext_WithKnowledge(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create knowledge store with data
	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create knowledge store: %v", err)
	}
	_ = knowledgeStore.SaveNote("vm-300", "test-vm", "vm", "general", "Test Note", "This is test content")

	intel.SetSubsystems(nil, nil, nil, nil, nil, knowledgeStore, nil, nil)

	ctx := intel.FormatContext("vm-300")

	// Should contain the knowledge context
	if ctx == "" {
		t.Error("Expected non-empty context with knowledge")
	}
}

func TestIntelligence_CreatePredictionFinding_LowSeverity(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Prediction far in the future with low confidence
	pred := patterns.FailurePrediction{
		ResourceID: "vm-100",
		EventType:  patterns.EventHighMemory,
		DaysUntil:  14,  // Far away
		Confidence: 0.3, // Low confidence
		Basis:      "Pattern detected",
	}

	finding := intel.CreatePredictionFinding(pred)

	// Should be watch severity (not critical or warning)
	if finding.Severity != FindingSeverityWatch {
		t.Errorf("Expected watch severity for far-off low-confidence prediction, got %s", finding.Severity)
	}
}

func TestIntelligence_CreatePredictionFinding_WarningSeverity(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Prediction soon but low confidence
	pred := patterns.FailurePrediction{
		ResourceID: "vm-100",
		EventType:  patterns.EventHighCPU,
		DaysUntil:  0.5, // Soon
		Confidence: 0.6, // Medium confidence (not > 0.8)
		Basis:      "Pattern detected",
	}

	finding := intel.CreatePredictionFinding(pred)

	// Should be warning (soon but not high confidence)
	if finding.Severity != FindingSeverityWarning {
		t.Errorf("Expected warning severity for imminent medium-confidence prediction, got %s", finding.Severity)
	}
}

func TestIntelligence_GetSummary_WithCriticalFindings(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	findings := NewFindingsStore()

	// Add multiple critical findings
	for i := 0; i < 3; i++ {
		findings.Add(&Finding{
			ID:         "crit-" + string(rune('A'+i)),
			Key:        "critical:" + string(rune('A'+i)),
			Severity:   FindingSeverityCritical,
			Category:   FindingCategoryReliability,
			ResourceID: "vm-crit-" + string(rune('A'+i)),
			Title:      "Critical Issue",
			DetectedAt: time.Now(),
			LastSeenAt: time.Now(),
			Source:     "test",
		})
	}

	intel.SetSubsystems(findings, nil, nil, nil, nil, nil, nil, nil)

	summary := intel.GetSummary()

	// Should have 3 critical
	if summary.FindingsCount.Critical != 3 {
		t.Errorf("Expected 3 critical findings, got %d", summary.FindingsCount.Critical)
	}

	// Health should be significantly reduced
	// Note: critical impact is capped at 40 points, so score = 100 - 40 = 60
	if summary.OverallHealth.Score > 60 {
		t.Errorf("Expected health score <= 60 with 3 critical findings, got %f", summary.OverallHealth.Score)
	}

	// Prediction text should mention critical issues
	if summary.OverallHealth.Prediction == "" {
		t.Error("Expected non-empty prediction text")
	}
}

func TestAbsFloatIntel(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{5.5, 5.5},
		{-5.5, 5.5},
		{0, 0},
		{-0, 0},
	}

	for _, tt := range tests {
		result := absFloatIntel(tt.input)
		if result != tt.expected {
			t.Errorf("absFloatIntel(%f) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

func TestIntelligence_GetSummary_WithPatterns(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create pattern detector with predictions
	patternDetector := patterns.NewDetector(patterns.DefaultConfig())

	// Set up the subsystems
	intel.SetSubsystems(nil, patternDetector, nil, nil, nil, nil, nil, nil)

	summary := intel.GetSummary()

	// Predictions count should be available
	if summary.PredictionsCount < 0 {
		t.Error("PredictionsCount should not be negative")
	}
}

func TestIntelligence_GetResourceIntelligence_WithBaselines(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create baseline store with learned data
	baselineStore := baseline.NewStore(baseline.StoreConfig{MinSamples: 10})

	// Learn baseline for CPU
	points := make([]baseline.MetricPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = baseline.MetricPoint{Value: 30 + float64(i%5) - 2}
	}
	_ = baselineStore.Learn("vm-with-baseline", "vm", "cpu", points)
	_ = baselineStore.Learn("vm-with-baseline", "vm", "memory", points)

	intel.SetSubsystems(nil, nil, nil, baselineStore, nil, nil, nil, nil)

	resourceIntel := intel.GetResourceIntelligence("vm-with-baseline")

	// Should have baselines
	if len(resourceIntel.Baselines) == 0 {
		t.Error("Expected baselines to be populated")
	}

	// Check CPU baseline exists
	if _, ok := resourceIntel.Baselines["cpu"]; !ok {
		t.Error("Expected CPU baseline")
	}
	if _, ok := resourceIntel.Baselines["memory"]; !ok {
		t.Error("Expected memory baseline")
	}
}

func TestIntelligence_GetResourceIntelligence_WithIncidents(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create incident store with some incidents
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{
		MaxIncidents: 10,
	})

	// Record an incident for the resource
	incidentStore.RecordAnalysis("alert-vm-500", "Analysis for vm-500", nil)

	intel.SetSubsystems(nil, nil, nil, nil, incidentStore, nil, nil, nil)

	resourceIntel := intel.GetResourceIntelligence("vm-500")

	// Should have the resource ID
	if resourceIntel.ResourceID != "vm-500" {
		t.Errorf("Expected resource ID 'vm-500', got %s", resourceIntel.ResourceID)
	}
}

func TestIntelligence_calculateResourceHealth_WithAnomalies(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create resource intelligence with anomalies
	resourceIntel := &ResourceIntelligence{
		ResourceID: "test-vm",
		Anomalies: []AnomalyReport{
			{
				Metric:       "cpu",
				CurrentValue: 90,
				BaselineMean: 30,
				ZScore:       5.0,
				Severity:     baseline.AnomalyCritical,
				Description:  "CPU is 3x baseline",
			},
			{
				Metric:       "memory",
				CurrentValue: 80,
				BaselineMean: 50,
				ZScore:       3.0,
				Severity:     baseline.AnomalyMedium,
				Description:  "Memory is elevated",
			},
		},
	}

	health := intel.calculateResourceHealth(resourceIntel)

	// Health should be reduced due to anomalies
	if health.Score >= 100 {
		t.Error("Expected reduced health score due to anomalies")
	}

	// Should have factors for anomalies
	hasAnomalyFactor := false
	for _, f := range health.Factors {
		if f.Category == "baseline" {
			hasAnomalyFactor = true
			break
		}
	}
	if !hasAnomalyFactor {
		t.Error("Expected baseline/anomaly factor in health factors")
	}
}

func TestIntelligence_calculateResourceHealth_WithPredictions(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create resource intelligence with predictions
	resourceIntel := &ResourceIntelligence{
		ResourceID: "test-vm",
		Predictions: []patterns.FailurePrediction{
			{
				ResourceID: "test-vm",
				EventType:  patterns.EventHighCPU,
				DaysUntil:  2.0,
				Confidence: 0.8,
				Basis:      "Pattern detected",
			},
		},
	}

	health := intel.calculateResourceHealth(resourceIntel)

	// Health should be reduced due to predictions
	if health.Score >= 100 {
		t.Error("Expected reduced health score due to predictions")
	}

	// Should have a prediction factor
	hasPredictionFactor := false
	for _, f := range health.Factors {
		if f.Category == "prediction" {
			hasPredictionFactor = true
			break
		}
	}
	if !hasPredictionFactor {
		t.Error("Expected prediction factor in health factors")
	}
}

func TestIntelligence_calculateResourceHealth_WithNotes(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create resource intelligence with notes (bonus for documentation)
	resourceIntel := &ResourceIntelligence{
		ResourceID: "test-vm",
		NoteCount:  3,
	}

	health := intel.calculateResourceHealth(resourceIntel)

	// Health should have a bonus for having notes
	if health.Score < 100 {
		t.Error("Expected health score >= 100 with only notes (bonus)")
	}

	// Should have a learning factor
	hasLearningFactor := false
	for _, f := range health.Factors {
		if f.Category == "learning" {
			hasLearningFactor = true
			break
		}
	}
	if !hasLearningFactor {
		t.Error("Expected learning factor for documented resource")
	}
}

func TestIntelligence_GetSummary_WithLearningBonus(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	// Create knowledge store with many resources
	knowledgeStore, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create knowledge store: %v", err)
	}

	// Add knowledge for 6+ resources to trigger learning bonus
	for i := 0; i < 7; i++ {
		resourceID := "vm-" + string(rune('A'+i))
		_ = knowledgeStore.SaveNote(resourceID, "VM "+string(rune('A'+i)), "vm", "general", "Note", "Content")
	}

	intel.SetSubsystems(nil, nil, nil, nil, nil, knowledgeStore, nil, nil)

	summary := intel.GetSummary()

	// With 6+ resources learned, should have learning bonus factor
	hasLearningFactor := false
	for _, f := range summary.OverallHealth.Factors {
		if f.Category == "learning" {
			hasLearningFactor = true
			break
		}
	}
	if !hasLearningFactor {
		t.Error("Expected learning factor with 6+ resources learned")
	}
}

func TestIntelligence_generateHealthPrediction_Warnings(t *testing.T) {
	intel := NewIntelligence(IntelligenceConfig{})

	health := HealthScore{
		Score: 80,
		Grade: HealthGradeB,
	}

	summary := &IntelligenceSummary{
		FindingsCount: FindingsCounts{
			Warning: 3,
		},
	}

	prediction := intel.generateHealthPrediction(health, summary)

	if prediction == "" {
		t.Error("Expected non-empty prediction")
	}
	if !strings.Contains(prediction, "warning") && !strings.Contains(prediction, "Warning") {
		t.Errorf("Expected prediction to mention warnings, got: %s", prediction)
	}
}
