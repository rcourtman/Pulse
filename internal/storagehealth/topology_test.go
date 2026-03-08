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

func TestAssessUnraidStorageParityUnavailable(t *testing.T) {
	assessment := AssessUnraidStorage(models.HostUnraidStorage{
		ArrayStarted: true,
		Disks: []models.HostUnraidDisk{
			{Name: "parity", Role: "parity", Status: "disabled"},
			{Name: "disk1", Role: "data", Status: "online"},
		},
	})

	if assessment.Level != RiskCritical {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskCritical)
	}
	if len(assessment.Reasons) == 0 || assessment.Reasons[0].Code != "unraid_parity_unavailable" {
		t.Fatalf("unexpected reasons %+v", assessment.Reasons)
	}
}

func TestAssessUnraidStorageSyncInProgress(t *testing.T) {
	assessment := AssessUnraidStorage(models.HostUnraidStorage{
		ArrayStarted: true,
		SyncAction:   "check",
		SyncProgress: 65,
		Disks: []models.HostUnraidDisk{
			{Name: "parity", Role: "parity", Status: "online"},
			{Name: "disk1", Role: "data", Status: "online"},
		},
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
	if len(assessment.Reasons) == 0 || assessment.Reasons[0].Code != "unraid_sync_active" {
		t.Fatalf("unexpected reasons %+v", assessment.Reasons)
	}
}
