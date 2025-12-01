package monitoring

import (
	"slices"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPreserveFailedStorageBackups(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{
		{
			ID:       "pve01-volid-new",
			Instance: instance,
			Storage:  "local-lvm",
			Volid:    "local-lvm:backup/new.vma.zst",
		},
	}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{
					ID:       "pve01-volid-old",
					Instance: instance,
					Storage:  "nas-share",
					Volid:    "nas-share:backup/old.vma.zst",
				},
				{
					ID:       "pve01-volid-new",
					Instance: instance,
					Storage:  "local-lvm",
					Volid:    "local-lvm:backup/new.vma.zst",
				},
			},
		},
	}
	toPreserve := map[string]struct{}{
		"nas-share": {},
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, toPreserve, current)

	if len(merged) != 2 {
		t.Fatalf("expected 2 backups after merge, got %d", len(merged))
	}

	if !slices.Contains(storages, "nas-share") {
		t.Fatalf("expected nas-share to be reported as preserved, got %v", storages)
	}

	if !slices.ContainsFunc(merged, func(b models.StorageBackup) bool {
		return b.Storage == "nas-share"
	}) {
		t.Fatalf("expected preserved backup for nas-share to be present")
	}
}

func TestPreserveFailedStorageBackupsSkipsDuplicates(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{
		{
			ID:       "pve01-volid-old",
			Instance: instance,
			Storage:  "nas-share",
			Volid:    "nas-share:backup/old.vma.zst",
		},
	}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{
					ID:       "pve01-volid-old",
					Instance: instance,
					Storage:  "nas-share",
					Volid:    "nas-share:backup/old.vma.zst",
				},
			},
		},
	}
	toPreserve := map[string]struct{}{
		"nas-share": {},
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, toPreserve, current)

	if len(merged) != 1 {
		t.Fatalf("expected duplicate backup to be ignored, got %d entries", len(merged))
	}
	if len(storages) != 0 {
		t.Fatalf("expected no storages to be reported because nothing new was preserved, got %v", storages)
	}
}

func TestPreserveFailedStorageBackupsEmptyPreserveMap(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{
		{ID: "backup1", Instance: instance, Storage: "local"},
	}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{ID: "backup2", Instance: instance, Storage: "nas-share"},
			},
		},
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, nil, current)

	if len(merged) != 1 {
		t.Fatalf("expected current unchanged with 1 backup, got %d", len(merged))
	}
	if storages != nil {
		t.Fatalf("expected nil storages list, got %v", storages)
	}

	// Also test empty map (not nil)
	merged2, storages2 := preserveFailedStorageBackups(instance, snapshot, map[string]struct{}{}, current)
	if len(merged2) != 1 {
		t.Fatalf("expected current unchanged with 1 backup, got %d", len(merged2))
	}
	if storages2 != nil {
		t.Fatalf("expected nil storages list for empty map, got %v", storages2)
	}
}

func TestPreserveFailedStorageBackupsNoMatchingBackups(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{
		{ID: "backup1", Instance: instance, Storage: "local"},
	}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{ID: "backup2", Instance: instance, Storage: "other-storage"},
			},
		},
	}
	toPreserve := map[string]struct{}{
		"nas-share": {}, // Storage not in snapshot
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, toPreserve, current)

	if len(merged) != 1 {
		t.Fatalf("expected current unchanged with 1 backup, got %d", len(merged))
	}
	if storages != nil {
		t.Fatalf("expected nil storages list when no matches, got %v", storages)
	}
}

func TestPreserveFailedStorageBackupsWrongInstance(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{
		{ID: "backup1", Instance: instance, Storage: "local"},
	}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{ID: "backup2", Instance: "pve02", Storage: "nas-share"}, // Wrong instance
			},
		},
	}
	toPreserve := map[string]struct{}{
		"nas-share": {},
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, toPreserve, current)

	if len(merged) != 1 {
		t.Fatalf("expected current unchanged (wrong instance skipped), got %d backups", len(merged))
	}
	if storages != nil {
		t.Fatalf("expected nil storages list, got %v", storages)
	}
}

func TestPreserveFailedStorageBackupsStorageNotInPreserveMap(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{
		{ID: "backup1", Instance: instance, Storage: "local"},
	}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{ID: "backup2", Instance: instance, Storage: "nas-share"},
				{ID: "backup3", Instance: instance, Storage: "other-storage"},
			},
		},
	}
	toPreserve := map[string]struct{}{
		"nas-share": {}, // Only nas-share should be preserved
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, toPreserve, current)

	if len(merged) != 2 {
		t.Fatalf("expected 2 backups (original + nas-share), got %d", len(merged))
	}
	if len(storages) != 1 || storages[0] != "nas-share" {
		t.Fatalf("expected [nas-share], got %v", storages)
	}
	// Verify other-storage was not added
	for _, b := range merged {
		if b.Storage == "other-storage" {
			t.Fatal("other-storage should not have been preserved")
		}
	}
}

func TestPreserveFailedStorageBackupsSortedStorageNames(t *testing.T) {
	instance := "pve01"
	current := []models.StorageBackup{}
	snapshot := models.StateSnapshot{
		PVEBackups: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{ID: "backup1", Instance: instance, Storage: "zebra-storage"},
				{ID: "backup2", Instance: instance, Storage: "alpha-storage"},
				{ID: "backup3", Instance: instance, Storage: "middle-storage"},
			},
		},
	}
	toPreserve := map[string]struct{}{
		"zebra-storage":  {},
		"alpha-storage":  {},
		"middle-storage": {},
	}

	merged, storages := preserveFailedStorageBackups(instance, snapshot, toPreserve, current)

	if len(merged) != 3 {
		t.Fatalf("expected 3 backups, got %d", len(merged))
	}
	expected := []string{"alpha-storage", "middle-storage", "zebra-storage"}
	if !slices.Equal(storages, expected) {
		t.Fatalf("expected sorted storages %v, got %v", expected, storages)
	}
}

func TestStorageNamesForNode(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		nodeName     string
		snapshot     models.StateSnapshot
		want         []string
	}{
		{
			name:         "empty nodeName returns nil",
			instanceName: "pve1",
			nodeName:     "",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve1", Name: "local-backup", Node: "node1", Content: "backup"},
				},
			},
			want: nil,
		},
		{
			name:         "empty snapshot returns nil",
			instanceName: "pve1",
			nodeName:     "node1",
			snapshot:     models.StateSnapshot{},
			want:         nil,
		},
		{
			name:         "storage with wrong instance is skipped",
			instanceName: "pve1",
			nodeName:     "node1",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve2", Name: "local-backup", Node: "node1", Content: "backup"},
				},
			},
			want: nil,
		},
		{
			name:         "storage with empty name is skipped",
			instanceName: "pve1",
			nodeName:     "node1",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve1", Name: "", Node: "node1", Content: "backup"},
				},
			},
			want: nil,
		},
		{
			name:         "storage without backup in Content is skipped",
			instanceName: "pve1",
			nodeName:     "node1",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve1", Name: "local", Node: "node1", Content: "images,rootdir"},
				},
			},
			want: nil,
		},
		{
			name:         "storage where Node matches nodeName is included",
			instanceName: "pve1",
			nodeName:     "node1",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve1", Name: "backup-storage", Node: "node1", Content: "backup"},
				},
			},
			want: []string{"backup-storage"},
		},
		{
			name:         "storage where nodeName is in Nodes slice is included",
			instanceName: "pve1",
			nodeName:     "node2",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve1", Name: "shared-backup", Node: "node1", Nodes: []string{"node1", "node2", "node3"}, Content: "backup"},
				},
			},
			want: []string{"shared-backup"},
		},
		{
			name:         "multiple matching storages are returned",
			instanceName: "pve1",
			nodeName:     "node1",
			snapshot: models.StateSnapshot{
				Storage: []models.Storage{
					{Instance: "pve1", Name: "local-backup", Node: "node1", Content: "backup"},
					{Instance: "pve1", Name: "nfs-backup", Node: "node1", Content: "backup,images"},
					{Instance: "pve1", Name: "shared-backup", Node: "node2", Nodes: []string{"node1", "node2"}, Content: "backup"},
				},
			},
			want: []string{"local-backup", "nfs-backup", "shared-backup"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storageNamesForNode(tt.instanceName, tt.nodeName, tt.snapshot)
			if !slices.Equal(got, tt.want) {
				t.Errorf("storageNamesForNode() = %v, want %v", got, tt.want)
			}
		})
	}
}
