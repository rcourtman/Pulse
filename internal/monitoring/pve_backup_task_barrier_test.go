package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Closed, read-only fixture: exercise the ordinary scan and its real state,
// backup-age and recovery consumers. No listener, database, guest/QGA command,
// backup, freeze/thaw or installed service is used.
type taskBarrierClient struct {
	fakeSnapshotClient
	mu    sync.Mutex
	phase string
	calls []string
	at    time.Time
}

func (c *taskBarrierClient) record(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *taskBarrierClient) GetBackupTasks(context.Context) ([]proxmox.Task, error) {
	c.record("tasks")
	if c.phase == "unavailable" || c.phase == "unavailable-again" {
		return nil, errors.New("synthetic task inventory unavailable")
	}
	if c.phase == "initial" || c.phase == "empty" {
		return nil, nil
	}
	status, end := "running", int64(0)
	if c.phase == "finished" {
		status, end = "OK", c.at.Add(time.Minute).Unix()
	}
	return []proxmox.Task{{UPID: "current", Node: "node", Type: "vzdump", ID: "100", StartTime: c.at.Unix(), Status: status, EndTime: end}}, nil
}

func (c *taskBarrierClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	c.record("storage")
	return []proxmox.Storage{{Storage: "archive", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}

func (c *taskBarrierClient) GetStorageContent(context.Context, string, string) ([]proxmox.StorageContent, error) {
	c.record("content")
	if c.phase == "empty" {
		return nil, nil
	}
	old := proxmox.StorageContent{Volid: "archive:backup/vzdump-qemu-100-previous.vma.zst", Content: "backup", Format: "vma.zst", VMID: 100, CTime: c.at.Add(-48 * time.Hour).Unix(), Size: 4096, Protected: 1, Notes: "preserved archive"}
	if c.phase == "initial" {
		return []proxmox.StorageContent{old}, nil
	}
	return []proxmox.StorageContent{old, {Volid: "archive:backup/vzdump-qemu-100-current.vma.zst", Content: "backup", Format: "vma.zst", VMID: 100, CTime: c.at.Unix(), Size: 1024}}, nil
}

func (c *taskBarrierClient) GetVMSnapshots(context.Context, string, int) ([]proxmox.Snapshot, error) {
	c.record("snapshots")
	return []proxmox.Snapshot{{Name: "snapshot-" + c.phase, SnapTime: c.at.Add(-72 * time.Hour).Unix()}}, nil
}

// Match the production client's optional scoped-inventory interface without
// making the legacy adapter above accidentally implement it too.
type scopedTaskBarrierClient struct{ *taskBarrierClient }

func (c *scopedTaskBarrierClient) GetBackupTaskInventory(ctx context.Context) (proxmox.BackupTaskInventory, error) {
	tasks, err := c.taskBarrierClient.GetBackupTasks(ctx)
	return proxmox.BackupTaskInventory{Tasks: tasks}, err
}

func TestPVEArchiveScanRequiresCurrentTaskInventory(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("scoped=%t", scoped), func(t *testing.T) {
			at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			c := &taskBarrierClient{at: at}
			var client PVEClientInterface = c
			if scoped {
				client = &scopedTaskBarrierClient{c}
			}
			m := backupLifecycleMonitor(client)
			t.Cleanup(func() { awaitBackupProjectionIngest(m) })
			peer := models.StorageBackup{ID: "independent", Instance: "other", Node: "node", Storage: "archive", VMID: 100, Type: "qemu", Time: at.Add(-time.Hour), CTime: at.Add(-time.Hour).Unix(), Size: 8192, Notes: "independent connection"}
			m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
			frames := map[string]models.PVEBackups{}
			var held models.StorageBackup
			for _, phase := range []string{"initial", "unavailable", "unavailable-again", "running", "finished", "empty"} {
				c.phase, c.calls = phase, nil
				m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, time.Second)
				awaitBackupProjectionIngest(m)
				got := m.PVEBackupsSnapshot()
				var own []models.StorageBackup
				for _, b := range got.StorageBackups {
					if b.Instance == "other" {
						require.Equal(t, peer, b)
					} else {
						own = append(own, b)
					}
				}
				require.Len(t, got.StorageBackups, len(own)+1, "same-VMID independent archive must survive")
				require.Len(t, got.GuestSnapshots, 1, "independent snapshot collection must continue")
				require.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				if phase == "initial" {
					require.Len(t, own, 1)
					held = own[0]
				}
				switch phase {
				case "unavailable", "unavailable-again":
					assert.NotContains(t, c.calls, "content", "failed prerequisite must not request or reconcile archive content")
					assert.Equal(t, []models.StorageBackup{held}, own, "failed read cannot add partial output, erase or alter a known archive")
				case "running", "finished":
					require.Len(t, own, 2)
					for _, b := range own {
						if b.ID != held.ID {
							require.Equal(t, phase == "running", b.InProgress, "ordinary current tasks classify partial output")
						}
					}
				case "empty":
					require.Empty(t, own, "successful empty task/content reads retain removal authority")
				}
				var last time.Time
				for _, vm := range m.state.GetSnapshot().VMs {
					if vm.Instance == "site" && vm.VMID == 100 {
						last = vm.LastBackup
					}
				}
				if phase != "empty" {
					want := held.Time
					if phase == "finished" {
						want = at
					}
					assert.True(t, want.Equal(last), "unqualified output must not renew last-backup age: got %s, want %s", last, want)
				}
				points := proxmoxmapper.FromPVEStorageBackups(own, nil)
				successes := 0
				for _, point := range points {
					if point.Outcome == recovery.OutcomeSuccess {
						successes++
					}
				}
				expected := 1
				if phase == "finished" {
					expected = 2
				} else if phase == "empty" {
					expected = 0
				}
				assert.Equal(t, expected, successes, "recovery consumers must not receive a false successful archive")
				frames[phase] = got
			}
			if scoped {
				wire, err := json.Marshal(frames)
				require.NoError(t, err)
				t.Logf("PVE_ARCHIVE_BARRIER_FRAMES=%s", wire)
			}
		})
	}
}

// A complete retained-history read is independently useful evidence, but it
// cannot substitute for this scan's failed live-task prerequisite. A nil-store
// manager enables the ordinary history branch without opening any database.
type taskBarrierHistoryClient struct{ *scopedTaskBarrierClient }

func (c *taskBarrierHistoryClient) GetBackupTaskHistory(context.Context, time.Time) (proxmox.BackupTaskHistory, error) {
	c.record("history")
	return proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: "node", Complete: true}}, Tasks: []proxmox.Task{{UPID: "history", Node: "node", Type: "vzdump", ID: "100", Status: "OK", StartTime: c.at.Add(-time.Hour).Unix(), EndTime: c.at.Add(-time.Minute).Unix()}}}, nil
}

func TestPVEArchiveReadBarrierColdStartAndHistory(t *testing.T) {
	for _, history := range []bool{false, true} {
		t.Run(fmt.Sprintf("history=%t", history), func(t *testing.T) {
			c := &taskBarrierClient{at: time.Now().Add(-time.Hour), phase: "unavailable"}
			var client PVEClientInterface = &scopedTaskBarrierClient{c}
			if history {
				client = &taskBarrierHistoryClient{&scopedTaskBarrierClient{c}}
			}
			m := backupLifecycleMonitor(client)
			if history {
				m.recoveryManager = recoverymanager.New(nil)
			}
			m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, time.Second)
			awaitBackupProjectionIngest(m)
			got := m.PVEBackupsSnapshot()
			require.Empty(t, got.StorageBackups, "cold start cannot mint completed output from a failed task read")
			require.Empty(t, got.BackupTasks, "history is not live membership")
			require.Len(t, got.GuestSnapshots, 1, "independent snapshot read still completes")
			require.NotContains(t, c.calls, "content")
			if history {
				require.Contains(t, c.calls, "history")
			}
		})
	}
}
