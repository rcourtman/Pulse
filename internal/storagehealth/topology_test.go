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
		Operation:      "recovery",
		RebuildPercent: 42,
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
}

func TestAssessHostRAIDArrayRecoveryOperationAlerts(t *testing.T) {
	assessment := AssessHostRAIDArray(models.HostRAIDArray{
		Device:         "/dev/md2",
		State:          "active, recovering",
		Operation:      "recovery",
		RebuildPercent: 12.6,
		TotalDevices:   2,
		ActiveDevices:  2,
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
	if !hasReasonCode(assessment, "raid_rebuilding") {
		t.Fatalf("expected raid_rebuilding reason, got %+v", assessment.Reasons)
	}
}

func TestAssessHostRAIDArrayCheckOperationIsSilent(t *testing.T) {
	assessment := AssessHostRAIDArray(models.HostRAIDArray{
		Device:         "/dev/md2",
		State:          "active",
		Operation:      "check",
		RebuildPercent: 12.6,
		TotalDevices:   3,
		ActiveDevices:  3,
	})

	if assessment.Level != RiskHealthy {
		t.Fatalf("Level = %q, want %q; reasons=%+v", assessment.Level, RiskHealthy, assessment.Reasons)
	}
	if hasReasonCode(assessment, "raid_rebuilding") {
		t.Fatalf("did not expect raid_rebuilding for scrub operation, got %+v", assessment.Reasons)
	}
}

func TestAssessHostRAIDArrayResyncOperationIsSilent(t *testing.T) {
	assessment := AssessHostRAIDArray(models.HostRAIDArray{
		Device:         "/dev/md2",
		State:          "active, resyncing",
		Operation:      "resync",
		RebuildPercent: 12.6,
		TotalDevices:   2,
		ActiveDevices:  2,
	})

	if assessment.Level != RiskHealthy {
		t.Fatalf("Level = %q, want %q; reasons=%+v", assessment.Level, RiskHealthy, assessment.Reasons)
	}
	if hasReasonCode(assessment, "raid_rebuilding") {
		t.Fatalf("did not expect raid_rebuilding for resync operation, got %+v", assessment.Reasons)
	}
}

func hasReasonCode(assessment Assessment, code string) bool {
	for _, reason := range assessment.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestFilterVendorManagedSystemRAIDArrays(t *testing.T) {
	testCases := []struct {
		name        string
		host        models.Host
		arrays      []models.HostRAIDArray
		wantDevices []string
	}{
		{
			name: "synology filters md0 and md1",
			host: models.Host{
				Hostname: "synology",
				OSName:   "Synology DSM",
			},
			arrays: []models.HostRAIDArray{
				{Device: "/dev/md0"},
				{Device: "/dev/md1"},
				{Device: "/dev/md2"},
			},
			wantDevices: []string{"/dev/md2"},
		},
		{
			name: "qnap filters md9 and md13",
			host: models.Host{
				Hostname: "qnap",
				OSName:   "QNAP QTS",
			},
			arrays: []models.HostRAIDArray{
				{Device: "/dev/md9"},
				{Device: "/dev/md13"},
				{Device: "/dev/md2"},
			},
			wantDevices: []string{"/dev/md2"},
		},
		{
			name: "generic host keeps md arrays",
			host: models.Host{
				Hostname: "ubuntu",
				OSName:   "Ubuntu",
			},
			arrays: []models.HostRAIDArray{
				{Device: "/dev/md0"},
				{Device: "/dev/md2"},
			},
			wantDevices: []string{"/dev/md0", "/dev/md2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filtered := FilterVendorManagedSystemRAIDArrays(tc.host, tc.arrays)
			if len(filtered) != len(tc.wantDevices) {
				t.Fatalf("filtered count = %d, want %d", len(filtered), len(tc.wantDevices))
			}
			for i, want := range tc.wantDevices {
				if filtered[i].Device != want {
					t.Fatalf("filtered[%d].Device = %q, want %q", i, filtered[i].Device, want)
				}
			}
		})
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
	foundParity := false
	for _, reason := range assessment.Reasons {
		if reason.Code == "unraid_parity_unavailable" {
			foundParity = true
			break
		}
	}
	if !foundParity {
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

func TestAssessUnraidStorageTreatsEmptyNoPresentSlotsAsUnprotected(t *testing.T) {
	assessment := AssessUnraidStorage(models.HostUnraidStorage{
		ArrayStarted: true,
		NumDisabled:  2,
		NumInvalid:   2,
		NumMissing:   0,
		Disks: []models.HostUnraidDisk{
			{Name: "parity", Role: "parity", Status: "missing", RawStatus: "DISK_NP_DSBL"},
			{Name: "md1p1", Device: "/dev/sde", Status: "online", RawStatus: "DISK_OK", SizeBytes: 5860522532},
			{Status: "missing", RawStatus: "DISK_NP", Slot: 5},
			{Status: "missing", RawStatus: "DISK_NP_DSBL", Slot: 29},
		},
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q; reasons %+v", assessment.Level, RiskWarning, assessment.Reasons)
	}
	foundNoParity := false
	for _, reason := range assessment.Reasons {
		switch reason.Code {
		case "unraid_no_parity":
			foundNoParity = true
		case "unraid_missing_disks", "unraid_parity_unavailable", "unraid_disabled_disks", "unraid_invalid_disks":
			t.Fatalf("unexpected failure reason for empty no-present slots: %+v", assessment.Reasons)
		}
	}
	if !foundNoParity {
		t.Fatalf("expected no-parity reason, got %+v", assessment.Reasons)
	}
}

func TestAssessUnraidStorageUsesDiskStatusesOverAggregateCounters(t *testing.T) {
	assessment := AssessUnraidStorage(models.HostUnraidStorage{
		ArrayStarted: true,
		SyncAction:   "check",
		NumDisabled:  1,
		NumInvalid:   1,
		Disks: []models.HostUnraidDisk{
			{Name: "parity", Role: "parity", Status: "online"},
			{Name: "disk1", Role: "data", Status: "online"},
		},
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
	for _, reason := range assessment.Reasons {
		if reason.Code == "unraid_disabled_disks" || reason.Code == "unraid_invalid_disks" {
			t.Fatalf("unexpected aggregate-count reason when structured disk state is healthy: %+v", assessment.Reasons)
		}
	}
}

func TestAssessUnraidStorageFallsBackToAggregateCountersWithoutDiskStatuses(t *testing.T) {
	assessment := AssessUnraidStorage(models.HostUnraidStorage{
		ArrayStarted: true,
		NumDisabled:  1,
		NumInvalid:   1,
	})

	if assessment.Level != RiskCritical {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskCritical)
	}
	foundDisabled := false
	foundInvalid := false
	for _, reason := range assessment.Reasons {
		if reason.Code == "unraid_disabled_disks" {
			foundDisabled = true
		}
		if reason.Code == "unraid_invalid_disks" {
			foundInvalid = true
		}
	}
	if !foundDisabled || !foundInvalid {
		t.Fatalf("unexpected reasons %+v", assessment.Reasons)
	}
}

func TestAssessPBSDatastoreUnavailable(t *testing.T) {
	assessment := AssessPBSDatastore(models.PBSDatastore{
		Name:   "backup-store",
		Status: "offline",
		Error:  "I/O error",
	})

	if assessment.Level != RiskCritical {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskCritical)
	}
	if len(assessment.Reasons) < 2 {
		t.Fatalf("expected multiple reasons, got %+v", assessment.Reasons)
	}
	if assessment.Reasons[0].Code != "pbs_datastore_error" && assessment.Reasons[0].Code != "pbs_datastore_state" {
		t.Fatalf("unexpected first reason %+v", assessment.Reasons[0])
	}
}

func TestAssessPBSDatastoreHighUsage(t *testing.T) {
	assessment := AssessPBSDatastore(models.PBSDatastore{
		Name:   "backup-store",
		Status: "online",
		Total:  100,
		Used:   93,
	})

	if assessment.Level != RiskWarning {
		t.Fatalf("Level = %q, want %q", assessment.Level, RiskWarning)
	}
	if len(assessment.Reasons) == 0 || assessment.Reasons[0].Code != "capacity_runway_low" {
		t.Fatalf("unexpected reasons %+v", assessment.Reasons)
	}
}
