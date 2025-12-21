package ai

import (
	"testing"
	"time"

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
		ID:         "test-finding",
		Key:        "test:finding",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryPerformance,
		ResourceID: "vm-100",
		ResourceName: "test-vm",
		ResourceType: "vm",
		Title:      "Test Finding",
		DetectedAt: time.Now(),
		LastSeenAt: time.Now(),
		Source:     "test",
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
