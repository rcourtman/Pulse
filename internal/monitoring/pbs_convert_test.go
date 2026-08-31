package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestMatchesDatastoreExclude(t *testing.T) {
	tests := []struct {
		name          string
		datastoreName string
		patterns      []string
		expected      bool
	}{
		// Empty patterns
		{"empty patterns returns false", "exthdd1500gb", nil, false},
		{"empty slice returns false", "exthdd1500gb", []string{}, false},

		// Exact match (case-insensitive)
		{"exact match", "exthdd1500gb", []string{"exthdd1500gb"}, true},
		{"exact match case insensitive", "ExtHDD1500GB", []string{"exthdd1500gb"}, true},
		{"exact match no match", "exthdd1500gb", []string{"backup"}, false},

		// Prefix pattern (name*)
		{"prefix pattern match", "exthdd1500gb", []string{"ext*"}, true},
		{"prefix pattern match 2", "backup-external", []string{"backup*"}, true},
		{"prefix pattern no match", "internal-storage", []string{"ext*"}, false},
		{"prefix pattern case insensitive", "EXTHDD1500GB", []string{"ext*"}, true},

		// Suffix pattern (*name)
		{"suffix pattern match", "my-external-hdd", []string{"*hdd"}, true},
		{"suffix pattern match 2", "backup-store", []string{"*store"}, true},
		{"suffix pattern no match", "hdd-backup", []string{"*store"}, false},
		{"suffix pattern case insensitive", "MY-EXTERNAL-HDD", []string{"*hdd"}, true},

		// Contains pattern (*name*)
		{"contains pattern match", "my-external-hdd", []string{"*external*"}, true},
		{"contains pattern match middle", "backup-removable-drive", []string{"*removable*"}, true},
		{"contains pattern no match", "internal-drive", []string{"*external*"}, false},
		{"contains pattern case insensitive", "BACKUP-REMOVABLE-DRIVE", []string{"*removable*"}, true},

		// Multiple patterns (any match)
		{"multiple patterns first match", "exthdd1500gb", []string{"backup*", "ext*"}, true},
		{"multiple patterns second match", "backup-drive", []string{"ext*", "backup*"}, true},
		{"multiple patterns no match", "internal", []string{"ext*", "backup*"}, false},

		// Edge cases
		{"empty pattern in list", "exthdd", []string{"", "ext*"}, true},
		{"whitespace pattern", "exthdd", []string{"   ", "ext*"}, true},
		{"pattern with whitespace", "exthdd", []string{"  ext*  "}, true},
		{"single star", "anything", []string{"*"}, false}, // Single star doesn't match (needs prefix/suffix)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesDatastoreExclude(tt.datastoreName, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesDatastoreExclude(%q, %v) = %t, want %t",
					tt.datastoreName, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestConvertPBSSnapshots(t *testing.T) {
	t.Run("empty input returns empty slice", func(t *testing.T) {
		result := convertPBSSnapshots("pbs-1", "backup-store", "ns1", nil)
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d items", len(result))
		}

		result = convertPBSSnapshots("pbs-1", "backup-store", "ns1", []pbs.BackupSnapshot{})
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d items", len(result))
		}
	})

	t.Run("basic snapshot conversion", func(t *testing.T) {
		backupTime := int64(1700000000)
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "100",
				BackupTime: backupTime,
				Size:       1024000,
				Protected:  true,
				Comment:    "Test backup",
				Owner:      "root@pam",
			},
		}

		result := convertPBSSnapshots("pbs-prod", "datastore1", "production", snapshots)

		if len(result) != 1 {
			t.Fatalf("expected 1 backup, got %d", len(result))
		}

		backup := result[0]
		expectedID := "pbs-pbs-prod-datastore1-production-vm-100-1700000000"
		if backup.ID != expectedID {
			t.Errorf("ID: expected %q, got %q", expectedID, backup.ID)
		}
		if backup.Instance != "pbs-prod" {
			t.Errorf("Instance: expected %q, got %q", "pbs-prod", backup.Instance)
		}
		if backup.Datastore != "datastore1" {
			t.Errorf("Datastore: expected %q, got %q", "datastore1", backup.Datastore)
		}
		if backup.Namespace != "production" {
			t.Errorf("Namespace: expected %q, got %q", "production", backup.Namespace)
		}
		if backup.BackupType != "vm" {
			t.Errorf("BackupType: expected %q, got %q", "vm", backup.BackupType)
		}
		if backup.VMID != "100" {
			t.Errorf("VMID: expected %q, got %q", "100", backup.VMID)
		}
		expectedTime := time.Unix(backupTime, 0)
		if !backup.BackupTime.Equal(expectedTime) {
			t.Errorf("BackupTime: expected %v, got %v", expectedTime, backup.BackupTime)
		}
		if backup.Size != 1024000 {
			t.Errorf("Size: expected %d, got %d", 1024000, backup.Size)
		}
		if !backup.Protected {
			t.Error("Protected: expected true")
		}
		if backup.Verified {
			t.Error("Verified: expected false (no verification data)")
		}
		if backup.Comment != "Test backup" {
			t.Errorf("Comment: expected %q, got %q", "Test backup", backup.Comment)
		}
		if backup.Owner != "root@pam" {
			t.Errorf("Owner: expected %q, got %q", "root@pam", backup.Owner)
		}
	})

	t.Run("files as string array", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "ct",
				BackupID:   "200",
				BackupTime: 1700000000,
				Files:      []interface{}{"file1.img", "file2.pxar", "file3.conf"},
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if len(result[0].Files) != 3 {
			t.Fatalf("expected 3 files, got %d", len(result[0].Files))
		}
		if result[0].Files[0] != "file1.img" {
			t.Errorf("Files[0]: expected %q, got %q", "file1.img", result[0].Files[0])
		}
		if result[0].Files[1] != "file2.pxar" {
			t.Errorf("Files[1]: expected %q, got %q", "file2.pxar", result[0].Files[1])
		}
		if result[0].Files[2] != "file3.conf" {
			t.Errorf("Files[2]: expected %q, got %q", "file3.conf", result[0].Files[2])
		}
	})

	t.Run("files as object array with filename field", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "300",
				BackupTime: 1700000000,
				Files: []interface{}{
					map[string]interface{}{"filename": "disk-0.raw", "size": 10737418240},
					map[string]interface{}{"filename": "vm.conf", "size": 1024},
				},
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if len(result[0].Files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(result[0].Files))
		}
		if result[0].Files[0] != "disk-0.raw" {
			t.Errorf("Files[0]: expected %q, got %q", "disk-0.raw", result[0].Files[0])
		}
		if result[0].Files[1] != "vm.conf" {
			t.Errorf("Files[1]: expected %q, got %q", "vm.conf", result[0].Files[1])
		}
	})

	t.Run("files with missing filename field ignored", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "400",
				BackupTime: 1700000000,
				Files: []interface{}{
					map[string]interface{}{"filename": "valid.raw"},
					map[string]interface{}{"name": "invalid.raw"}, // Wrong field name
					map[string]interface{}{},                      // Empty object
				},
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if len(result[0].Files) != 1 {
			t.Fatalf("expected 1 file (only valid one), got %d", len(result[0].Files))
		}
		if result[0].Files[0] != "valid.raw" {
			t.Errorf("Files[0]: expected %q, got %q", "valid.raw", result[0].Files[0])
		}
	})

	t.Run("verification as string ok", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType:   "vm",
				BackupID:     "500",
				BackupTime:   1700000000,
				Verification: "ok",
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if !result[0].Verified {
			t.Error("Verified: expected true when verification is 'ok'")
		}
	})

	t.Run("verification as string not ok", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType:   "vm",
				BackupID:     "501",
				BackupTime:   1700000000,
				Verification: "failed",
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if result[0].Verified {
			t.Error("Verified: expected false when verification is not 'ok'")
		}
	})

	t.Run("verification as object with state ok", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "600",
				BackupTime: 1700000000,
				Verification: map[string]interface{}{
					"state":    "ok",
					"upid":     "UPID:pbs:00001234",
					"snapshot": "2023-11-14T12:00:00Z",
				},
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if !result[0].Verified {
			t.Error("Verified: expected true when verification.state is 'ok'")
		}
	})

	t.Run("verification as object with state failed", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "601",
				BackupTime: 1700000000,
				Verification: map[string]interface{}{
					"state": "failed",
					"upid":  "UPID:pbs:00005678",
				},
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if result[0].Verified {
			t.Error("Verified: expected false when verification.state is 'failed'")
		}
	})

	t.Run("verification as object without state field", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "602",
				BackupTime: 1700000000,
				Verification: map[string]interface{}{
					"upid": "UPID:pbs:00009999",
				},
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		if result[0].Verified {
			t.Error("Verified: expected false when verification object has no state field")
		}
	})

	t.Run("multiple snapshots", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "100",
				BackupTime: 1700000000,
			},
			{
				BackupType: "ct",
				BackupID:   "200",
				BackupTime: 1700001000,
			},
			{
				BackupType: "vm",
				BackupID:   "100",
				BackupTime: 1700002000, // Same VM, different time
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "ns", snapshots)

		if len(result) != 3 {
			t.Fatalf("expected 3 backups, got %d", len(result))
		}

		// Verify each has unique ID
		ids := make(map[string]bool)
		for _, backup := range result {
			if ids[backup.ID] {
				t.Errorf("duplicate ID found: %s", backup.ID)
			}
			ids[backup.ID] = true
		}
	})

	t.Run("empty namespace in ID", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "vm",
				BackupID:   "100",
				BackupTime: 1700000000,
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "", snapshots)

		expectedID := "pbs-pbs-1-store--vm-100-1700000000"
		if result[0].ID != expectedID {
			t.Errorf("ID with empty namespace: expected %q, got %q", expectedID, result[0].ID)
		}
	})

	t.Run("container backup type", func(t *testing.T) {
		snapshots := []pbs.BackupSnapshot{
			{
				BackupType: "ct",
				BackupID:   "101",
				BackupTime: 1700000000,
				Size:       512000,
			},
		}

		result := convertPBSSnapshots("pbs-1", "store", "ns", snapshots)

		if result[0].BackupType != "ct" {
			t.Errorf("BackupType: expected %q, got %q", "ct", result[0].BackupType)
		}
	})
}

// An in-flight snapshot (backup still being written) is listed by the PBS API
// without a size and without the manifest (index.json.blob) - typically only
// the guest config blob exists yet. It must be flagged InProgress so nothing
// downstream treats it as a completed backup.
func TestConvertPBSSnapshotsInProgress(t *testing.T) {
	tests := []struct {
		name       string
		snapshot   pbs.BackupSnapshot
		inProgress bool
	}{
		{
			name: "no size and only config blob is in-progress",
			snapshot: pbs.BackupSnapshot{
				BackupType: "vm",
				BackupID:   "117",
				BackupTime: 1787221141,
				Files:      []interface{}{map[string]interface{}{"filename": "qemu-server.conf.blob"}},
				Owner:      "admin@pbs!pve-homelab",
			},
			inProgress: true,
		},
		{
			name: "completed snapshot with manifest and size",
			snapshot: pbs.BackupSnapshot{
				BackupType: "vm",
				BackupID:   "117",
				BackupTime: 1787221141,
				Size:       80 << 30,
				Files: []interface{}{
					"qemu-server.conf.blob",
					"drive-scsi0.img.fidx",
					"index.json.blob",
				},
			},
			inProgress: false,
		},
		{
			name: "manifest present but size missing still counts as complete",
			snapshot: pbs.BackupSnapshot{
				BackupType: "ct",
				BackupID:   "105",
				BackupTime: 1787221141,
				Files:      []interface{}{"root.pxar.didx", "index.json.blob"},
			},
			inProgress: false,
		},
		{
			name: "size present without manifest counts as complete",
			snapshot: pbs.BackupSnapshot{
				BackupType: "vm",
				BackupID:   "100",
				BackupTime: 1787221141,
				Size:       2048,
				Files:      []interface{}{"drive-scsi0.img.fidx"},
			},
			inProgress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPBSSnapshots("pbs-1", "main", "pve-pc", []pbs.BackupSnapshot{tt.snapshot})
			if len(result) != 1 {
				t.Fatalf("expected 1 backup, got %d", len(result))
			}
			if result[0].InProgress != tt.inProgress {
				t.Errorf("InProgress: expected %t, got %t", tt.inProgress, result[0].InProgress)
			}
		})
	}
}

func TestApplyPBSWriteActivityClearsTerminalIncompleteSnapshots(t *testing.T) {
	backups := []models.PBSBackup{
		{Datastore: "main", BackupType: "vm", VMID: "117", InProgress: true},
		{Datastore: "main", BackupType: "ct", VMID: "105", InProgress: true},
	}

	applyPBSWriteActivity(backups, nil)

	for i, backup := range backups {
		if !backup.WriteActivityObserved {
			t.Fatalf("backup %d did not record authoritative task observation", i)
		}
		if backup.WriteActive {
			t.Fatalf("backup %d remained active with no running PBS task", i)
		}
	}
}

func TestApplyPBSWriteActivityMatchesBackupSubjectAndSync(t *testing.T) {
	backups := []models.PBSBackup{
		{Datastore: "main", BackupType: "vm", VMID: "117", InProgress: true},
		{Datastore: "main", BackupType: "vm", VMID: "118", InProgress: true},
	}

	applyPBSWriteActivity(backups, []pbs.RunningDataTask{{WorkerType: "backup", WorkerID: "main:vm/117"}})
	if !backups[0].WriteActive {
		t.Fatal("matching backup task did not mark its incomplete snapshot active")
	}
	if backups[1].WriteActive {
		t.Fatal("backup task for vm/117 incorrectly marked vm/118 active")
	}

	applyPBSWriteActivity(backups, []pbs.RunningDataTask{{WorkerType: "syncjob", WorkerID: "offsite"}})
	if !backups[0].WriteActive || !backups[1].WriteActive {
		t.Fatal("running sync task must account for incomplete snapshots across its PBS instance")
	}
}
