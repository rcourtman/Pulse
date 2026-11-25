package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

type backupRefreshCapture struct {
	*unifiedresources.MonitorAdapter
	snapshot models.StateSnapshot
}

func (c *backupRefreshCapture) PopulateSnapshotAndSupplemental(snapshot models.StateSnapshot, records map[unifiedresources.DataSource][]unifiedresources.IngestRecord) {
	c.snapshot = snapshot
	c.MonitorAdapter.PopulateSnapshotAndSupplemental(snapshot, records)
}

func TestBackupReadStateForInstance_RefreshesOnlyWhenGuestArrives(t *testing.T) {
	for _, guestType := range []string{"vm", "container"} {
		t.Run(guestType, func(t *testing.T) {
			now := time.Now().UTC()
			state := models.NewState()
			state.UpdateVMsForInstance("peer", []models.VM{{
				ID: "peer-vm-100", Instance: "peer", Node: "peer-node", VMID: 100, Name: "peer", Status: "running", LastSeen: now,
			}})
			state.UpdateStorage([]models.Storage{{
				ID: "target-local", Instance: "target", Node: "target-node", Name: "local", Type: "dir", Content: "backup", Active: true, Enabled: true,
			}})
			state.UpdatePBSBackups("pbs", []models.PBSBackup{{
				ID: "pbs-vm-100", Instance: "pbs", Datastore: "archive", BackupType: "vm", VMID: "100",
				Files: []string{"index.json.blob", "drive.fidx"}, VerificationRaw: map[string]any{"state": "ok"},
			}})
			store := &backupRefreshCapture{MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
			store.MonitorAdapter.PopulateFromSnapshot(state.GetSnapshot())
			m := &Monitor{state: state, resourceStore: store}

			// No guest on target; still use the published storage view and
			// leave the unrelated peer and its archives alone.
			read := m.backupReadStateForInstance("target")
			require.Same(t, store, read)
			require.Zero(t, store.snapshot.LastUpdate, "an empty target must not republish the registry")
			require.Equal(t, []string{"local"}, storageNamesForNode(read, "target", "target-node"))
			nodes := map[int]string{}
			populateGuestNodeMapFromReadState(read, "target", nodes)
			require.Empty(t, nodes, "a same-ID peer must not stand in for target inventory")

			// The next read after a real collection must still catch up the
			// registry, with either kind of guest and no negative cache.
			if guestType == "vm" {
				state.UpdateVMsForInstance("target", []models.VM{{
					ID: "target-vm-100", Instance: "target", Node: "target-node", VMID: 100, Name: "target", Status: "running", LastSeen: now,
				}})
			} else {
				state.UpdateContainersForInstance("target", []models.Container{{
					ID: "target-ct-100", Instance: "target", Node: "target-node", VMID: 100, Name: "target", Status: "running", LastSeen: now,
				}})
			}
			full := state.GetSnapshot()
			require.False(t, backupReadStateHasGuestForInstance(store, "target"), "registry is deliberately one update behind")
			read = m.backupReadStateForInstance("target")
			require.Same(t, store, read)
			populateGuestNodeMapFromReadState(read, "target", nodes)
			require.Equal(t, map[int]string{100: "target-node"}, nodes)
			require.Equal(t, []string{"local"}, storageNamesForNode(read, "target", "target-node"))
			require.Len(t, store.VMs(), len(full.VMs), "peer VM must survive catch-up")
			require.Len(t, store.Containers(), len(full.Containers))
			require.Equal(t, full, store.snapshot, "catch-up must publish the same complete snapshot")
			store.snapshot.PBSBackups[0].Files[0] = "local edit"
			require.Equal(t, full.PBSBackups, state.GetBackups().PBS, "published snapshot must stay detached")
		})
	}
}

func TestSyncGuestBackupTimesAndResourceStore_UsesLightweightPBSRows(t *testing.T) {
	now := time.Now().UTC()
	state := models.NewState()
	state.UpdateNodes([]models.Node{
		{ID: "lab-node", Name: "node", Instance: "lab", Status: "online", LastSeen: now},
		{ID: "peer-node", Name: "peer-node", Instance: "peer", Status: "online", LastSeen: now},
	})
	state.UpdateVMsForInstance("lab", []models.VM{{ID: "lab-vm100", VMID: 100, Name: "vm100", Instance: "lab", Node: "node", Status: "running", LastSeen: now}})
	state.UpdateVMsForInstance("peer", []models.VM{{ID: "peer-vm100", VMID: 100, Name: "peer100", Instance: "peer", Node: "peer-node", Status: "running", LastSeen: now}})
	state.UpdateContainersForInstance("lab", []models.Container{{ID: "lab-ct101", VMID: 101, Name: "ct101", Instance: "lab", Node: "node", Status: "running", LastSeen: now}})
	state.UpdatePBSInstances([]models.PBSInstance{{
		ID: "pbs", Name: "pbs", Status: "online", LastSeen: now,
		Datastores: []models.PBSDatastore{{Name: "daily", Total: 1000, Used: 400, Free: 600}},
	}})
	state.UpdateStorageBackupsForInstance("lab", []models.StorageBackup{{
		ID: "local-vm100", Instance: "lab", Node: "node", Storage: "local", VMID: 100, Type: "qemu", Time: now.Add(-time.Hour),
	}})
	state.UpdateBackupTasksForInstance("lab", []models.BackupTask{{
		ID: "vzdump-101", Instance: "lab", Node: "node", VMID: 101, Status: "running", StartTime: now,
	}})
	pbsDate := now.Add(-30 * time.Minute)
	state.UpdatePBSBackups("pbs", []models.PBSBackup{{
		ID: "pbs-vm100", Instance: "pbs", Datastore: "daily", BackupType: "vm", VMID: "100", BackupTime: pbsDate,
		Files: []string{"index.json.blob", "drive.fidx"}, VerificationRaw: map[string]any{"state": "ok"}, Verified: true,
	}})
	// The two connections share the same typed VMID. The exclusive storage
	// view is needed to attribute this PBS snapshot to exactly one guest.
	state.UpdatePBSGuestConfirmationsForInstance("lab", []models.PBSGuestConfirmation{{
		Storage: "pbs-archive", BackupType: "vm", VMID: 100, Time: pbsDate.Unix(),
	}})
	state.UpdateActiveAlerts([]models.Alert{{ID: "test-alert", ResourceID: "lab-node", StartTime: now}})
	archives := state.GetBackups()
	require.Len(t, archives.PVE.StorageBackups, 1)
	require.Len(t, archives.PVE.BackupTasks, 1)
	require.Len(t, archives.PBS, 1)

	store := &backupRefreshCapture{MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
	m := &Monitor{state: state, resourceStore: store}
	m.syncGuestBackupTimesAndResourceStore()

	full := state.GetSnapshot()
	require.Equal(t, archives, full.Backups, "archive readers still get the complete history")
	require.Equal(t, full.PVEBackups, store.snapshot.PVEBackups)
	require.Len(t, store.snapshot.PBSBackups, 1)
	light := archives.PBS[0]
	light.Files = []string{}
	light.VerificationRaw = nil
	require.Equal(t, light, store.snapshot.PBSBackups[0])
	require.Equal(t, []models.PBSGuestRef{{Instance: "lab", BackupType: "vm", VMID: 100}}, store.snapshot.PBSBackupGuests())
	require.Equal(t, full.LastUpdate, store.snapshot.LastUpdate)
	require.Equal(t, full.ActiveAlerts, store.snapshot.ActiveAlerts)

	reference := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	reference.PopulateFromSnapshot(full)
	want, got := reference.GetAll(), store.GetAll()
	// Each rebuild stamps its own publication time; source observation
	// timestamps and the rest of the resource payload must still agree.
	for i := range want {
		want[i].UpdatedAt = time.Time{}
	}
	for i := range got {
		got[i].UpdatedAt = time.Time{}
	}
	require.ElementsMatch(t, want, got, "registry output must match a full snapshot rebuild")
	pbsInstances := store.PBSInstances()
	require.Len(t, pbsInstances, 1)
	require.Equal(t, 1, pbsInstances[0].ProtectedWorkloadCount())

	vms, containers := store.VMs(), store.Containers()
	require.Len(t, vms, 2)
	for _, vm := range vms {
		if vm.Instance() == "lab" {
			require.True(t, vm.LastBackup().Equal(pbsDate), "newest completed backup must reach canonical resources")
		} else {
			require.True(t, vm.LastBackup().IsZero(), "same-typed peer VMID must not inherit the backup")
		}
	}
	require.Len(t, containers, 1)
	require.True(t, containers[0].BackupInProgress(), "running task must reach canonical resources")
	require.True(t, containers[0].LastBackup().IsZero(), "running task must not become a completed backup")
	store.snapshot.PBSBackups[0].Comment = "local edit"
	require.Equal(t, archives, state.GetBackups(), "projected rows must not alias live archives")
}
