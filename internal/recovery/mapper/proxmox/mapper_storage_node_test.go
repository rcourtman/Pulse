package proxmoxmapper

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A non-shared storage is a different repository on each node. A backup row's
// Node follows its guest after a migration, so the repository records the
// node whose disk holds the archive.
func TestPVEStorageBackupRepositoryIsTheDisksNode(t *testing.T) {
	at := time.Unix(1800, 0).UTC()
	points := FromPVEStorageBackups([]models.StorageBackup{
		{ID: "migrated", Storage: "local", Node: "node2", StorageNode: "node1", Instance: "pve1", Type: "lxc", VMID: 102, Time: at},
		{ID: "shared", Storage: "nfs", Node: "node2", Instance: "pve1", Type: "lxc", VMID: 102, Time: at},
	}, nil)
	if len(points) != 2 {
		t.Fatalf("points = %d, want 2", len(points))
	}
	for _, want := range []struct{ id, class string }{{"pve-backup:migrated", "node1"}, {"pve-backup:shared", "node2"}} {
		var found bool
		for _, p := range points {
			if p.ID != want.id {
				continue
			}
			found = true
			if p.RepositoryRef == nil || p.RepositoryRef.Class != want.class {
				t.Errorf("%s repository = %+v, want class %q", want.id, p.RepositoryRef, want.class)
			}
		}
		if !found {
			t.Errorf("no point %s", want.id)
		}
	}
}
