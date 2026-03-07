package storagehealth

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAssessHostSMARTDiskCriticalSignals(t *testing.T) {
	pending := int64(3)
	media := int64(1)

	assessment := AssessHostSMARTDisk(models.HostDiskSMART{
		Model:       "Seagate IronWolf",
		Health:      "PASSED",
		Temperature: 62,
		Attributes: &models.SMARTAttributes{
			PendingSectors: &pending,
			MediaErrors:    &media,
		},
	})

	if assessment.Level != RiskCritical {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskCritical)
	}
	if len(assessment.Reasons) < 2 {
		t.Fatalf("expected multiple reasons, got %+v", assessment.Reasons)
	}
}

func TestAssessPhysicalDiskKnownFirmwareBugDoesNotEscalateFalseHealth(t *testing.T) {
	assessment := AssessPhysicalDisk(models.PhysicalDisk{
		Model:   "Samsung SSD 990 PRO 2TB",
		Health:  "FAILED",
		Wearout: 99,
	})

	if assessment.Level != RiskHealthy {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskHealthy)
	}
	if len(assessment.Reasons) != 0 {
		t.Fatalf("expected no reasons, got %+v", assessment.Reasons)
	}
}

func TestAssessPhysicalDiskWarningSignals(t *testing.T) {
	reallocated := int64(4)
	assessment := AssessPhysicalDisk(models.PhysicalDisk{
		Model:       "Crucial MX500",
		Health:      "PASSED",
		Temperature: 61,
		SmartAttributes: &models.SMARTAttributes{
			ReallocatedSectors: &reallocated,
		},
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
	if len(assessment.Reasons) == 0 {
		t.Fatal("expected warning reasons")
	}
}
