package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- PhysicalDiskRiskFromAssessment ---

func TestPhysicalDiskRiskFromAssessment_HealthyNoReasons(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	result := PhysicalDiskRiskFromAssessment(assessment)
	if result != nil {
		t.Error("healthy assessment with no reasons should return nil")
	}
}

func TestPhysicalDiskRiskFromAssessment_HealthyWithReasons(t *testing.T) {
	// Edge case: healthy level but with reasons should still return a risk.
	assessment := storagehealth.Assessment{
		Level: storagehealth.RiskHealthy,
		Reasons: []storagehealth.Reason{
			{Code: "info", Severity: storagehealth.RiskHealthy, Summary: "All good"},
		},
	}
	result := PhysicalDiskRiskFromAssessment(assessment)
	if result == nil {
		t.Fatal("healthy assessment with reasons should return non-nil risk")
	}
	if result.Level != storagehealth.RiskHealthy {
		t.Errorf("expected healthy level, got %s", result.Level)
	}
	if len(result.Reasons) != 1 {
		t.Errorf("expected 1 reason, got %d", len(result.Reasons))
	}
}

func TestPhysicalDiskRiskFromAssessment_ConvertsReasons(t *testing.T) {
	assessment := storagehealth.Assessment{
		Level: storagehealth.RiskCritical,
		Reasons: []storagehealth.Reason{
			{Code: "pending_sectors", Severity: storagehealth.RiskCritical, Summary: "Pending sectors detected (5)"},
			{Code: "temperature_high", Severity: storagehealth.RiskWarning, Summary: "Disk temperature is 65C"},
		},
	}
	result := PhysicalDiskRiskFromAssessment(assessment)
	if result == nil {
		t.Fatal("expected non-nil risk")
	}
	if result.Level != storagehealth.RiskCritical {
		t.Errorf("expected critical, got %s", result.Level)
	}
	if len(result.Reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(result.Reasons))
	}
	if result.Reasons[0].Code != "pending_sectors" {
		t.Errorf("expected first reason code 'pending_sectors', got %q", result.Reasons[0].Code)
	}
	if result.Reasons[1].Severity != storagehealth.RiskWarning {
		t.Errorf("expected second reason severity warning, got %s", result.Reasons[1].Severity)
	}
}

// --- physicalDiskAssessmentFromMeta ---

func TestPhysicalDiskAssessmentFromMeta_Nil(t *testing.T) {
	assessment := physicalDiskAssessmentFromMeta(nil)
	if assessment.Level != storagehealth.RiskHealthy {
		t.Errorf("nil meta should produce healthy assessment, got %s", assessment.Level)
	}
	if len(assessment.Reasons) != 0 {
		t.Errorf("nil meta should have no reasons, got %d", len(assessment.Reasons))
	}
}

func TestPhysicalDiskAssessmentFromMeta_HealthyDisk(t *testing.T) {
	meta := &PhysicalDiskMeta{
		Health:      "PASSED",
		Temperature: 35,
		Wearout:     95,
	}
	assessment := physicalDiskAssessmentFromMeta(meta)
	if assessment.Level != storagehealth.RiskHealthy {
		t.Errorf("healthy disk should produce healthy assessment, got %s", assessment.Level)
	}
}

func TestPhysicalDiskAssessmentFromMeta_PendingSectors(t *testing.T) {
	meta := &PhysicalDiskMeta{
		Health: "PASSED",
		SMART: &SMARTMeta{
			PendingSectors: 3,
		},
	}
	assessment := physicalDiskAssessmentFromMeta(meta)
	if assessment.Level != storagehealth.RiskCritical {
		t.Errorf("pending sectors should produce critical assessment, got %s", assessment.Level)
	}

	foundPending := false
	for _, reason := range assessment.Reasons {
		if reason.Code == "pending_sectors" {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Error("expected pending_sectors reason in assessment")
	}
}

func TestPhysicalDiskAssessmentFromMeta_HighTemperature(t *testing.T) {
	meta := &PhysicalDiskMeta{
		Health:      "PASSED",
		Temperature: 72,
	}
	assessment := physicalDiskAssessmentFromMeta(meta)
	if assessment.Level != storagehealth.RiskCritical {
		t.Errorf("72°C should produce critical assessment, got %s", assessment.Level)
	}
}

func TestPhysicalDiskAssessmentFromMeta_LowWearout(t *testing.T) {
	meta := &PhysicalDiskMeta{
		Health:  "PASSED",
		Wearout: 3,
	}
	assessment := physicalDiskAssessmentFromMeta(meta)
	if assessment.Level != storagehealth.RiskCritical {
		t.Errorf("3%% wearout should produce critical assessment, got %s", assessment.Level)
	}
}

// --- physicalDiskStatus ---

func TestPhysicalDiskStatus_CriticalAssessment(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskCritical}
	got := physicalDiskStatus("SSD-Model", "PASSED", assessment)
	if got != StatusWarning {
		t.Errorf("critical assessment should produce warning status, got %s", got)
	}
}

func TestPhysicalDiskStatus_WarningAssessment(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskWarning}
	got := physicalDiskStatus("SSD-Model", "PASSED", assessment)
	if got != StatusWarning {
		t.Errorf("warning assessment should produce warning status, got %s", got)
	}
}

func TestPhysicalDiskStatus_PassedHealth(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("SSD-Model", "PASSED", assessment)
	if got != StatusOnline {
		t.Errorf("passed health with healthy assessment should produce online, got %s", got)
	}
}

func TestPhysicalDiskStatus_OKHealth(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("SSD-Model", "OK", assessment)
	if got != StatusOnline {
		t.Errorf("OK health should produce online, got %s", got)
	}
}

func TestPhysicalDiskStatus_FailedHealth(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("Generic-SSD", "FAILED", assessment)
	if got != StatusOffline {
		t.Errorf("failed health on normal disk should produce offline, got %s", got)
	}
}

func TestPhysicalDiskStatus_FailedHealthWithFirmwareBug(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("Samsung SSD 980 Pro", "FAILED", assessment)
	if got != StatusUnknown {
		t.Errorf("failed health on known-buggy disk should produce unknown, got %s", got)
	}
}

func TestPhysicalDiskStatus_UnknownHealth(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("SSD-Model", "UNKNOWN", assessment)
	if got != StatusUnknown {
		t.Errorf("unknown health should produce unknown status, got %s", got)
	}
}

func TestPhysicalDiskStatus_EmptyHealth(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("SSD-Model", "", assessment)
	if got != StatusUnknown {
		t.Errorf("empty health should produce unknown status, got %s", got)
	}
}

func TestPhysicalDiskStatus_CaseInsensitiveHealth(t *testing.T) {
	assessment := storagehealth.Assessment{Level: storagehealth.RiskHealthy}
	got := physicalDiskStatus("SSD-Model", " passed ", assessment)
	if got != StatusOnline {
		t.Errorf("'passed' (lowercase with whitespace) should produce online, got %s", got)
	}
}

func TestPhysicalDiskStatus_AssessmentPrioritized(t *testing.T) {
	// Even if health says "PASSED", a critical assessment should still produce warning.
	assessment := storagehealth.Assessment{
		Level: storagehealth.RiskCritical,
		Reasons: []storagehealth.Reason{
			{Code: "pending_sectors", Severity: storagehealth.RiskCritical, Summary: "bad sectors"},
		},
	}
	got := physicalDiskStatus("SSD-Model", "PASSED", assessment)
	if got != StatusWarning {
		t.Errorf("critical assessment should override healthy SMART status, got %s", got)
	}
}
