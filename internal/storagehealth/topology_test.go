package storagehealth

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAssessHostRAIDArrayDegraded(t *testing.T) {
	assessment := AssessHostRAIDArray(models.HostRAIDArray{
		Device:        "/dev/md2",
		State:         "degraded",
		TotalDevices:  2,
		ActiveDevices: 1,
		FailedDevices: 1,
	})

	if assessment.Level != RiskCritical {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskCritical)
	}
	if len(assessment.Reasons) == 0 || assessment.Reasons[0].Code != "raid_degraded" {
		t.Fatalf("unexpected reasons %+v", assessment.Reasons)
	}
}

func TestAssessHostRAIDArrayRebuilding(t *testing.T) {
	assessment := AssessHostRAIDArray(models.HostRAIDArray{
		Device:         "/dev/md3",
		State:          "recovering",
		RebuildPercent: 42,
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
}

func TestAssessZFSPoolCriticalState(t *testing.T) {
	assessment := AssessZFSPool(models.ZFSPool{
		Name:  "tank",
		State: "FAULTED",
	})

	if assessment.Level != RiskCritical {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskCritical)
	}
}

func TestAssessZFSPoolErrors(t *testing.T) {
	assessment := AssessZFSPool(models.ZFSPool{
		Name:           "tank",
		State:          "ONLINE",
		ChecksumErrors: 3,
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
}
