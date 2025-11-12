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

func TestStorageNamesForNode(t *testing.T) {
	instance := "pve01"
	snapshot := models.StateSnapshot{
		Storage: []models.Storage{
			{
				Name:     "local-zfs",
				Instance: instance,
				Node:     "pve-node1",
				Content:  "images,rootdir,backup",
			},
			{
				Name:     "nas-share",
				Instance: instance,
				Nodes:    []string{"pve-node2", "pve-node3"},
				Content:  "backup,iso",
			},
			{
				Name:     "other-instance",
				Instance: "pve02",
				Node:     "pve-node1",
				Content:  "backup",
			},
		},
	}

	found := storageNamesForNode(instance, "pve-node2", snapshot)
	if !slices.Equal(found, []string{"nas-share"}) {
		t.Fatalf("unexpected storages for node2: %v", found)
	}

	found = storageNamesForNode(instance, "pve-node1", snapshot)
	if !slices.Equal(found, []string{"local-zfs"}) {
		t.Fatalf("unexpected storages for node1: %v", found)
	}
}
