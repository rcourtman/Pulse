package proxmoxmapper

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestFromPVEGuestSnapshots_Empty(t *testing.T) {
	result := FromPVEGuestSnapshots(nil, nil)
	if result != nil {
		t.Errorf("FromPVEGuestSnapshots(nil, nil) = %v, want nil", result)
	}

	result = FromPVEGuestSnapshots([]models.GuestSnapshot{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPVEGuestSnapshots([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPVEGuestSnapshots_Single(t *testing.T) {
	snapshots := []models.GuestSnapshot{
		{
			ID:        "snapshot-1",
			VMID:      100,
			Node:      "pve1",
			Instance:  "pve-cluster",
			Name:      "web-server",
			Type:      "qemu",
			Time:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			SizeBytes: 1024,
		},
	}

	result := FromPVEGuestSnapshots(snapshots, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderProxmoxPVE {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderProxmoxPVE)
	}
	if p.Kind != recovery.KindSnapshot {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindSnapshot)
	}
	if p.Mode != recovery.ModeSnapshot {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeSnapshot)
	}
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
	if p.Details == nil {
		t.Error("expected Details to be set")
	}
}

func TestFromPVEGuestSnapshots_WithGuestInfo(t *testing.T) {
	snapshots := []models.GuestSnapshot{
		{
			ID:       "snapshot-1",
			VMID:     100,
			Node:     "pve1",
			Instance: "pve-cluster",
			Name:     "web-server",
			Type:     "qemu",
			Time:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
	}

	guestInfoByKey := map[string]GuestInfo{
		"pve-cluster|pve1|100": {
			SourceID:     "unified-resource-1",
			ResourceType: unifiedresources.ResourceTypeVM,
			Name:         "web-server",
		},
	}

	result := FromPVEGuestSnapshots(snapshots, guestInfoByKey)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.SubjectResourceID == "" {
		t.Error("expected SubjectResourceID to be set when GuestInfo is provided")
	}
}

func TestFromPVEStorageBackups_Empty(t *testing.T) {
	result := FromPVEStorageBackups(nil, nil)
	if result != nil {
		t.Errorf("FromPVEStorageBackups(nil, nil) = %v, want nil", result)
	}

	result = FromPVEStorageBackups([]models.StorageBackup{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPVEStorageBackups([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPVEStorageBackups_Single(t *testing.T) {
	backups := []models.StorageBackup{
		{
			ID:       "backup-1",
			VMID:     100,
			Node:     "pve1",
			Instance: "pve-cluster",
			Storage:  "local",
			Type:     "qemu",
			Time:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Size:     2048,
		},
	}

	result := FromPVEStorageBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderProxmoxPVE {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderProxmoxPVE)
	}
	if p.Kind != recovery.KindBackup {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindBackup)
	}
	if p.SizeBytes == nil || *p.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %v, want 2048", p.SizeBytes)
	}
}

func TestFromPVEBackupTasks_Empty(t *testing.T) {
	result := FromPVEBackupTasks(nil, nil)
	if result != nil {
		t.Errorf("FromPVEBackupTasks(nil, nil) = %v, want nil", result)
	}

	result = FromPVEBackupTasks([]models.BackupTask{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPVEBackupTasks([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPVEBackupTasks_Success(t *testing.T) {
	tasks := []models.BackupTask{
		{
			ID:        "task-1",
			VMID:      100,
			Node:      "pve1",
			Instance:  "pve-cluster",
			Type:      "vzdump",
			Status:    "ok",
			StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC),
			Size:      4096,
		},
	}

	result := FromPVEBackupTasks(tasks, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
}

func TestFromPVEBackupTasks_Failed(t *testing.T) {
	tasks := []models.BackupTask{
		{
			ID:        "task-1",
			VMID:      100,
			Node:      "pve1",
			Instance:  "pve-cluster",
			Type:      "vzdump",
			Status:    "error",
			StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC),
		},
	}

	result := FromPVEBackupTasks(tasks, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Outcome != recovery.OutcomeFailed {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeFailed)
	}
}

func TestFromPBSBackups_Empty(t *testing.T) {
	result := FromPBSBackups(nil, nil)
	if result != nil {
		t.Errorf("FromPBSBackups(nil, nil) = %v, want nil", result)
	}

	result = FromPBSBackups([]models.PBSBackup{}, nil)
	if result != nil && len(result) != 0 {
		t.Errorf("FromPBSBackups([], nil) = %v, want nil or empty", result)
	}
}

func TestFromPBSBackups_Single(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-1",
			VMID:       "100",
			Instance:   "pbs1",
			Datastore:  "backup-store",
			BackupType: "vm",
			BackupTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Size:       8192,
			Verified:   true,
			Protected:  true,
		},
	}

	result := FromPBSBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderProxmoxPBS {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderProxmoxPBS)
	}
	if p.Kind != recovery.KindBackup {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindBackup)
	}
	if p.Mode != recovery.ModeRemote {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeRemote)
	}
	if p.SizeBytes == nil || *p.SizeBytes != 8192 {
		t.Errorf("SizeBytes = %v, want 8192", p.SizeBytes)
	}
}

func TestFromPBSBackups_PrefersCommentNameWhenGuestIsUnresolved(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-legacy",
			VMID:       "140",
			Instance:   "pbs-docker",
			Namespace:  "pimox",
			Datastore:  "main",
			BackupType: "ct",
			BackupTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Comment:    "pulse-v4-prod, pi, 140",
		},
	}

	result := FromPBSBackups(backups, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}
	if result[0].SubjectRef == nil {
		t.Fatal("expected SubjectRef to be set")
	}
	if got := result[0].SubjectRef.Name; got != "pulse-v4-prod" {
		t.Fatalf("SubjectRef.Name = %q, want %q", got, "pulse-v4-prod")
	}
	if got := result[0].SubjectRef.ID; got != "140" {
		t.Fatalf("SubjectRef.ID = %q, want %q", got, "140")
	}
}

func TestFromPBSBackups_WithCandidates(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-1",
			VMID:       "100",
			Instance:   "pbs1",
			Datastore:  "backup-store",
			BackupType: "vm",
			BackupTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Size:       8192,
		},
	}

	candidatesByKey := map[string][]GuestCandidate{
		"vm:100": {
			{
				SourceID:      "unified-resource-1",
				ResourceType:  unifiedresources.ResourceTypeVM,
				DisplayName:   "web-server",
				InstanceName:  "pbs1",
				NodeName:      "pve1",
				VMID:          100,
				BackupTypeKey: "vm",
			},
		},
	}

	result := FromPBSBackups(backups, candidatesByKey)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.SubjectResourceID == "" {
		t.Error("expected SubjectResourceID to be set when candidate matches")
	}
}

func TestFromPBSBackups_DisambiguatesCandidatesByNamespace(t *testing.T) {
	backups := []models.PBSBackup{
		{
			ID:         "pbs-backup-minipc-ct-112",
			VMID:       "112",
			Instance:   "pbs-docker",
			Namespace:  "minipc",
			Datastore:  "main",
			BackupType: "ct",
			BackupTime: time.Date(2026, 3, 29, 3, 3, 31, 0, time.UTC),
			Comment:    "debian-go",
		},
	}

	candidatesByKey := map[string][]GuestCandidate{
		"ct:112": {
			{
				SourceID:      "system-container-fb42a70d89bd20a6",
				ResourceType:  unifiedresources.ResourceTypeSystemContainer,
				DisplayName:   "debian-go",
				InstanceName:  "delly",
				NodeName:      "minipc",
				VMID:          112,
				BackupTypeKey: "ct",
			},
			{
				SourceID:      "system-container-deadbeefdeadbeef",
				ResourceType:  unifiedresources.ResourceTypeSystemContainer,
				DisplayName:   "other-guest",
				InstanceName:  "other",
				NodeName:      "pve-b",
				VMID:          112,
				BackupTypeKey: "ct",
			},
		},
	}

	result := FromPBSBackups(backups, candidatesByKey)
	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	expectedRID := unifiedresources.SourceSpecificID(
		unifiedresources.ResourceTypeSystemContainer,
		unifiedresources.SourceProxmox,
		"system-container-fb42a70d89bd20a6",
	)

	if got := result[0].SubjectResourceID; got != expectedRID {
		t.Fatalf("SubjectResourceID = %q, want %q", got, expectedRID)
	}
	if result[0].SubjectRef == nil || result[0].SubjectRef.Name != "debian-go" {
		t.Fatalf("SubjectRef = %#v, want linked debian-go guest", result[0].SubjectRef)
	}
}
