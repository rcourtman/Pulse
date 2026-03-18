package storagehealth

import (
	"testing"
)

// --- AssessSample: comprehensive branch coverage ---

func TestAssessSample_HealthyDisk(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:      "PASSED",
		Temperature: 35,
		Wearout:     80,
	})
	if assessment.Level != RiskHealthy {
		t.Errorf("expected healthy, got %s", assessment.Level)
	}
	if len(assessment.Reasons) != 0 {
		t.Errorf("expected 0 reasons, got %d", len(assessment.Reasons))
	}
}

func TestAssessSample_FailedHealthStatus(t *testing.T) {
	assessment := AssessSample(Sample{
		Health: "FAILED",
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for FAILED health, got %s", assessment.Level)
	}
	found := false
	for _, r := range assessment.Reasons {
		if r.Code == "health_status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected health_status reason")
	}
}

func TestAssessSample_PassedHealthDoesNotTrigger(t *testing.T) {
	for _, health := range []string{"PASSED", "OK", "UNKNOWN", ""} {
		assessment := AssessSample(Sample{Health: health})
		for _, r := range assessment.Reasons {
			if r.Code == "health_status" {
				t.Errorf("health %q should not trigger health_status reason", health)
			}
		}
	}
}

func TestAssessSample_FailedHealthWithFirmwareBug(t *testing.T) {
	assessment := AssessSample(Sample{
		Model:  "Samsung SSD 980 Pro",
		Health: "FAILED",
	})
	// Known firmware bug should suppress the FAILED health escalation.
	for _, r := range assessment.Reasons {
		if r.Code == "health_status" {
			t.Error("Samsung 980 FAILED should be suppressed by firmware bug detection")
		}
	}
}

func TestAssessSample_PendingSectors(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		PendingSectors: 5,
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for pending sectors, got %s", assessment.Level)
	}
	found := false
	for _, r := range assessment.Reasons {
		if r.Code == "pending_sectors" && r.Severity == RiskCritical {
			found = true
		}
	}
	if !found {
		t.Error("expected pending_sectors reason at critical severity")
	}
}

func TestAssessSample_OfflineUncorrectable(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:               "PASSED",
		OfflineUncorrectable: 2,
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for offline uncorrectable, got %s", assessment.Level)
	}
}

func TestAssessSample_MediaErrors(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:      "PASSED",
		MediaErrors: 1,
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for media errors, got %s", assessment.Level)
	}
}

func TestAssessSample_WearoutCritical(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:  "PASSED",
		Wearout: 3, // <=5 should be critical
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for wearout=3, got %s", assessment.Level)
	}
	found := false
	for _, r := range assessment.Reasons {
		if r.Code == "wearout_low" && r.Severity == RiskCritical {
			found = true
		}
	}
	if !found {
		t.Error("expected wearout_low reason at critical severity")
	}
}

func TestAssessSample_WearoutWarning(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:  "PASSED",
		Wearout: 8, // >5, <10 should be warning
	})
	if assessment.Level != RiskWarning {
		t.Errorf("expected warning for wearout=8, got %s", assessment.Level)
	}
}

func TestAssessSample_WearoutHealthy(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:  "PASSED",
		Wearout: 50,
	})
	for _, r := range assessment.Reasons {
		if r.Code == "wearout_low" {
			t.Error("wearout=50 should not trigger wearout_low")
		}
	}
}

func TestAssessSample_NVMeAvailableSpareCritical(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		AvailableSpare: 8, // <=10 critical
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for available spare=8, got %s", assessment.Level)
	}
}

func TestAssessSample_NVMeAvailableSpareWarning(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		AvailableSpare: 15, // >10, <20 warning
	})
	if assessment.Level != RiskWarning {
		t.Errorf("expected warning for available spare=15, got %s", assessment.Level)
	}
}

func TestAssessSample_NVMeAvailableSpareHealthy(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		AvailableSpare: 50,
	})
	for _, r := range assessment.Reasons {
		if r.Code == "nvme_available_spare_low" {
			t.Error("available spare=50 should not trigger reason")
		}
	}
}

func TestAssessSample_NVMePercentageUsedCritical(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		PercentageUsed: 97, // >=95 critical
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for percentage used=97, got %s", assessment.Level)
	}
}

func TestAssessSample_NVMePercentageUsedWarning(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		PercentageUsed: 92, // >=90, <95 warning
	})
	if assessment.Level != RiskWarning {
		t.Errorf("expected warning for percentage used=92, got %s", assessment.Level)
	}
}

func TestAssessSample_TemperatureCritical(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:      "PASSED",
		Temperature: 72, // >=70 critical
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical for temp=72, got %s", assessment.Level)
	}
}

func TestAssessSample_TemperatureWarning(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:      "PASSED",
		Temperature: 63, // >=60, <70 warning
	})
	if assessment.Level != RiskWarning {
		t.Errorf("expected warning for temp=63, got %s", assessment.Level)
	}
}

func TestAssessSample_TemperatureHealthy(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:      "PASSED",
		Temperature: 45,
	})
	for _, r := range assessment.Reasons {
		if r.Code == "temperature_high" {
			t.Error("temp=45 should not trigger temperature_high")
		}
	}
}

func TestAssessSample_ReallocatedSectors(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:             "PASSED",
		ReallocatedSectors: 4,
	})
	if assessment.Level != RiskWarning {
		t.Errorf("expected warning for reallocated sectors, got %s", assessment.Level)
	}
	found := false
	for _, r := range assessment.Reasons {
		if r.Code == "reallocated_sectors" && r.Severity == RiskWarning {
			found = true
		}
	}
	if !found {
		t.Error("expected reallocated_sectors at warning severity")
	}
}

func TestAssessSample_CRCErrors(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:        "PASSED",
		UDMACRCErrors: 10,
	})
	if assessment.Level != RiskMonitor {
		t.Errorf("expected monitor for CRC errors, got %s", assessment.Level)
	}
}

func TestAssessSample_MultipleIssuesTakesHighest(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:         "PASSED",
		Temperature:    63, // warning
		PendingSectors: 1,  // critical
		UDMACRCErrors:  10, // monitor
	})
	if assessment.Level != RiskCritical {
		t.Errorf("expected critical (highest), got %s", assessment.Level)
	}
	if len(assessment.Reasons) != 3 {
		t.Errorf("expected 3 reasons, got %d", len(assessment.Reasons))
	}
}

func TestAssessSample_ReasonsSortedBySeverityDescending(t *testing.T) {
	assessment := AssessSample(Sample{
		Health:             "PASSED",
		Temperature:        63, // warning
		PendingSectors:     1,  // critical
		UDMACRCErrors:      10, // monitor
		ReallocatedSectors: 1,  // warning
	})
	for i := 1; i < len(assessment.Reasons); i++ {
		prevRank := severityRank(assessment.Reasons[i-1].Severity)
		currRank := severityRank(assessment.Reasons[i].Severity)
		if prevRank < currRank {
			t.Errorf("reasons should be sorted by severity descending at position %d: %s (%d) < %s (%d)",
				i, assessment.Reasons[i-1].Severity, prevRank, assessment.Reasons[i].Severity, currRank)
		}
	}
}

// --- HasKnownFirmwareBug ---

func TestHasKnownFirmwareBug(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"Samsung SSD 980 Pro 1TB", true},
		{"SAMSUNG SSD 980 PRO", true},
		{"samsung ssd 990 evo", true},
		{"Samsung 980 500G", true},
		{"Samsung 990 Pro 2TB", true},
		{"Crucial MX500", false},
		{"WD Blue SN570", false},
		{"", false},
		{"Samsung SSD 870 EVO", false},
	}
	for _, tt := range tests {
		got := HasKnownFirmwareBug(tt.model)
		if got != tt.expected {
			t.Errorf("HasKnownFirmwareBug(%q) = %v, want %v", tt.model, got, tt.expected)
		}
	}
}

// --- severityRank ---

func TestSeverityRank_Ordering(t *testing.T) {
	if severityRank(RiskHealthy) >= severityRank(RiskMonitor) {
		t.Error("healthy should rank lower than monitor")
	}
	if severityRank(RiskMonitor) >= severityRank(RiskWarning) {
		t.Error("monitor should rank lower than warning")
	}
	if severityRank(RiskWarning) >= severityRank(RiskCritical) {
		t.Error("warning should rank lower than critical")
	}
}

func TestSeverityRank_UnknownIsZero(t *testing.T) {
	if severityRank("bogus") != 0 {
		t.Error("unknown level should rank 0")
	}
}

// --- normalizeHealth ---

func TestNormalizeHealth(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"passed", "PASSED"},
		{" OK ", "OK"},
		{"FAILED", "FAILED"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeHealth(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeHealth(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
