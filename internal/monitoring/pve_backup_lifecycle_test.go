package monitoring

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// No guest-agent, task operation, external connection or installed target.
// The same behavioural tests also compile against the exact original source.
type backupLifecycleClient struct {
	mockPVEClientExtra
	label string
	read  func(context.Context, string)
	calls atomic.Int32
}

func (c *backupLifecycleClient) before(ctx context.Context, phase string) {
	c.calls.Add(1)
	if c.read != nil {
		c.read(ctx, phase)
	}
}
func (c *backupLifecycleClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	c.before(ctx, "tasks")
	return []proxmox.Task{{UPID: testJobUPID, Node: "node-a", Type: "vzdump", StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(-time.Minute).Unix(), Status: "job errors"}}, nil
}
func (c *backupLifecycleClient) GetTaskLog(ctx context.Context, _, _ string) ([]proxmox.TaskLogLine, error) {
	c.before(ctx, "log")
	return []proxmox.TaskLogLine{{LineNumber: 1, Text: "INFO: Starting Backup of VM 101 (qemu)"}, {LineNumber: 2, Text: "ERROR: Backup of VM 101 failed - " + c.label + " result"}, {LineNumber: 3, Text: "TASK ERROR: job errors"}}, nil
}
func (c *backupLifecycleClient) GetStorage(ctx context.Context, _ string) ([]proxmox.Storage, error) {
	c.before(ctx, "storage")
	return []proxmox.Storage{{Storage: "archive", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}
func (c *backupLifecycleClient) GetStorageContent(ctx context.Context, _, _ string) ([]proxmox.StorageContent, error) {
	c.before(ctx, "content")
	return []proxmox.StorageContent{{Volid: "archive:backup/vzdump-qemu-101-" + c.label + ".vma.zst", VMID: 101, CTime: time.Now().Add(-2 * time.Hour).Unix(), Size: 1024, Content: "backup"}}, nil
}
func (c *backupLifecycleClient) GetVMSnapshots(ctx context.Context, _ string, _ int) ([]proxmox.Snapshot, error) {
	c.before(ctx, "snapshots")
	return []proxmox.Snapshot{{Name: c.label, SnapTime: time.Now().Add(-time.Hour).Unix()}}, nil
}
func (c *backupLifecycleClient) GetContainerSnapshots(ctx context.Context, node string, id int) ([]proxmox.Snapshot, error) {
	return c.GetVMSnapshots(ctx, node, id)
}
func newBackupLifecycleMonitor(c PVEClientInterface, kind string) *Monitor {
	m := &Monitor{state: models.NewState(), pveClients: map[string]PVEClientInterface{"site-a": c}, config: &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}, lastPVEBackupPoll: map[string]time.Time{}, runtimeCtx: context.Background()}
	if kind == "qemu" {
		m.state.UpdateVMsForInstance("site-a", []models.VM{{ID: "site-a-node-a-101", VMID: 101, Node: "node-a", Instance: "site-a", Name: "workload", Status: "running"}})
	} else {
		m.state.UpdateContainersForInstance("site-a", []models.Container{{ID: "site-a-node-a-101", VMID: 101, Node: "node-a", Instance: "site-a", Name: "workload", Status: "running"}})
	}
	m.state.UpdateStorageBackupsForInstance("site-b", []models.StorageBackup{{ID: "peer-file", Instance: "site-b", Node: "node-a", Type: "qemu", VMID: 101, Storage: "archive", Time: time.Now().Add(-time.Hour)}})
	m.state.UpdateGuestSnapshotsForInstance("site-b", []models.GuestSnapshot{{ID: "peer-snap", Instance: "site-b", Node: "node-a", Type: "qemu", VMID: 101, Name: "independent", Time: time.Now().Add(-time.Hour)}})
	return m
}
func scheduleBackupLifecycle(m *Monitor, c PVEClientInterface) {
	m.pollPVEBackupsAsync("site-a", &config.PVEInstance{MonitorBackups: true}, c, []proxmox.Node{{Node: "node-a", Status: "online"}}, map[string]string{"node-a": "online"})
}

func TestPVEBackupLifecycleSingleFlight(t *testing.T) {
	for _, interval := range []time.Duration{0, time.Minute} {
		t.Run(interval.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				release := make(chan struct{})
				var tasks atomic.Int32
				c := &backupLifecycleClient{label: "accepted", read: func(_ context.Context, phase string) {
					if phase == "tasks" {
						tasks.Add(1)
						<-release
					}
				}}
				m := newBackupLifecycleMonitor(c, "qemu")
				m.config.BackupPollingInterval = interval
				scheduleBackupLifecycle(m, c)
				synctest.Wait()
				// A slow scan outlasts cadence, but it still owns the connection.
				time.Sleep(90 * time.Second)
				for i := 0; i < 8; i++ {
					scheduleBackupLifecycle(m, c)
				}
				synctest.Wait()
				got := tasks.Load()
				close(release)
				synctest.Wait()
				require.Equal(t, int32(1), got, "cadence must not stack scans")
				completed := m.lastPVEBackupPoll["site-a"]
				require.Equal(t, time.Now(), completed, "cadence starts at accepted completion")
				time.Sleep(interval + time.Second)
				scheduleBackupLifecycle(m, c)
				synctest.Wait()
				require.Equal(t, int32(2), tasks.Load(), "ordinary next eligible cycle recovers")
			})
		})
	}
}

func TestPVEBackupLifecycleLateReadsCannotReplaceCurrentEvidence(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, phase := range []string{"tasks", "log", "storage", "content", "snapshots", "sizing"} {
			for _, transition := range []string{"client", "runtime", "retire"} {
				t.Run(kind+"/"+phase+"/"+transition, func(t *testing.T) {
					synctest.Test(t, func(t *testing.T) {
						release := make(chan struct{})
						released := false
						defer func() {
							if !released {
								close(release)
							}
						}()
						var reached atomic.Bool
						var storageReads atomic.Int32
						var oldCtx context.Context
						old := &backupLifecycleClient{label: "old", read: func(ctx context.Context, p string) {
							sizing := p == "storage" && storageReads.Add(1) == 2
							if p == phase || (phase == "sizing" && sizing) {
								oldCtx = ctx
								reached.Store(true)
								<-release
							}
						}}
						m := newBackupLifecycleMonitor(old, kind)
						scheduleBackupLifecycle(m, old)
						synctest.Wait()
						require.True(t, reached.Load())
						fresh := &backupLifecycleClient{label: "current"}
						switch transition {
						case "retire":
							m.retirePVEInstanceRuntime("site-a")
						case "runtime":
							m.setRuntimeContext(context.Background(), nil) // Deliberately don't cancel the old parent.
						}
						m.replacePVEClient("site-a", fresh)
						if transition == "retire" {
							// Fresh configuration rebuilds its guest inventory without a live guest operation.
							source := newBackupLifecycleMonitor(fresh, kind).state.GetSnapshot()
							m.state.UpdateVMsForInstance("site-a", source.VMs)
							m.state.UpdateContainersForInstance("site-a", source.Containers)
						}
						scheduleBackupLifecycle(m, fresh)
						synctest.Wait()
						before := m.state.GetSnapshot().PVEBackups
						require.NotZero(t, fresh.calls.Load(), "replacement must not wait for the late client")
						currentStamp := m.lastPVEBackupPoll["site-a"]
						cache := m.vzdumpJobTaskCache["site-a|"+testJobUPID]
						close(release)
						released = true
						synctest.Wait()
						require.NotNil(t, oldCtx.Err(), "binding transition must cancel the old read")
						require.Equal(t, before, m.state.GetSnapshot().PVEBackups, "late read changed accepted inventories")
						require.Equal(t, currentStamp, m.lastPVEBackupPoll["site-a"], "old finish rewrote replacement cadence")
						require.Equal(t, cache, m.vzdumpJobTaskCache["site-a|"+testJobUPID], "old result poisoned the immutable cache")
						require.Len(t, before.StorageBackups, 2)
						require.Len(t, before.GuestSnapshots, 2)
						found := false
						for _, task := range before.BackupTasks {
							if task.VMID == 101 {
								require.Equal(t, "current result", task.Error)
								found = true
							}
						}
						require.True(t, found, "new client must read its own guest log even for a reused UPID")
						if os.Getenv("PULSE_BACKUP_LIFECYCLE_FRAMES") == "1" && kind == "qemu" && phase == "tasks" && transition == "client" {
							data, err := json.Marshal(map[string]any{"accepted": before, "after_late": m.state.GetSnapshot().PVEBackups, "vms": m.state.GetSnapshot().VMs, "containers": m.state.GetSnapshot().Containers})
							require.NoError(t, err)
							t.Log("BACKUP_LIFECYCLE_FRAME=" + string(data))
						}
					})
				})
			}
		}
	}
}

func TestPVEBackupLifecycleRetirementDoesNotResurrectState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		old := &backupLifecycleClient{label: "retired", read: func(_ context.Context, p string) {
			if p == "tasks" {
				<-release
			}
		}}
		m := newBackupLifecycleMonitor(old, "qemu")
		scheduleBackupLifecycle(m, old)
		synctest.Wait()
		m.retirePVEInstanceRuntime("site-a")
		before := m.state.GetSnapshot().PVEBackups
		close(release)
		synctest.Wait()
		require.Equal(t, before, m.state.GetSnapshot().PVEBackups)
		_, exists := m.lastPVEBackupPoll["site-a"]
		require.False(t, exists, "retired completion recreated scheduling state")
		require.Empty(t, m.vzdumpJobTaskCache)
	})
}

func TestPVEBackupLifecycleConcurrentAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var tasks atomic.Int32
		c := &backupLifecycleClient{label: "accepted", read: func(_ context.Context, p string) {
			if p == "tasks" {
				tasks.Add(1)
				<-release
			}
		}}
		m := newBackupLifecycleMonitor(c, "qemu")
		var callers sync.WaitGroup
		for i := 0; i < 32; i++ {
			callers.Add(1)
			go func() { defer callers.Done(); scheduleBackupLifecycle(m, c) }()
		}
		callers.Wait()
		synctest.Wait()
		count := tasks.Load()
		close(release)
		synctest.Wait()
		require.Equal(t, int32(1), count)
	})
}

func TestPVEBackupLifecycleCopiesSchedulingInventory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		c := &backupLifecycleClient{label: "accepted", read: func(_ context.Context, p string) {
			if p == "tasks" {
				<-release
			}
		}}
		m := newBackupLifecycleMonitor(c, "qemu")
		nodes := []proxmox.Node{{Node: "node-a", Status: "online"}}
		statuses := map[string]string{"node-a": "online"}
		m.pollPVEBackupsAsync("site-a", &config.PVEInstance{MonitorBackups: true}, c, nodes, statuses)
		synctest.Wait()
		nodes[0].Node = "reused-core-buffer"
		statuses["node-a"] = "offline"
		close(release)
		synctest.Wait()
		found := false
		for _, row := range m.state.GetSnapshot().PVEBackups.StorageBackups {
			if row.Instance == "site-a" {
				found = true
				require.Equal(t, "node-a", row.StorageNode)
			}
		}
		require.True(t, found, "next core cycle mutated the owned storage origin")
	})
}

func TestPVEBackupLifecycleCanceledCollectorsKeepEvidence(t *testing.T) {
	for _, phase := range []string{"tasks", "log", "snapshots"} {
		for _, kind := range []string{"qemu", "lxc"} {
			t.Run(phase+"/"+kind, func(t *testing.T) {
				c := &backupLifecycleClient{label: "accepted"}
				m := newBackupLifecycleMonitor(c, kind)
				m.pollBackupTasks(context.Background(), "site-a", c)
				m.pollGuestSnapshots(context.Background(), "site-a", c)
				before := m.state.GetSnapshot().PVEBackups
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				c.label = "late"
				c.read = func(_ context.Context, p string) {
					if p == phase {
						cancel()
					}
				}
				// Non-finalise the prior cache to force another real log read.
				key := "site-a|" + testJobUPID
				entry := m.vzdumpJobTaskCache[key]
				entry.final = false
				m.vzdumpJobTaskCache[key] = entry
				cache := m.vzdumpJobTaskCache[key]
				if phase == "snapshots" {
					m.pollGuestSnapshots(ctx, "site-a", c)
				} else {
					m.pollBackupTasks(ctx, "site-a", c)
				}
				observed := m.state.GetSnapshot().PVEBackups
				if phase == "log" {
					// The parent listing was accepted before this log exhausted
					// the budget. Its observation may advance; cached guest facts
					// may not. A canceled read supplied no new guest verdict.
					var beforeGuests, afterGuests []models.BackupTask
					for _, row := range before.BackupTasks {
						if row.VMID > 0 {
							beforeGuests = append(beforeGuests, row)
						}
					}
					for _, row := range observed.BackupTasks {
						if row.VMID > 0 {
							afterGuests = append(afterGuests, row)
						}
					}
					require.Equal(t, beforeGuests, afterGuests)
					require.Equal(t, before.StorageBackups, observed.StorageBackups)
					require.Equal(t, before.GuestSnapshots, observed.GuestSnapshots)
				} else {
					require.Equal(t, before, observed)
				}
				after := m.vzdumpJobTaskCache[key]
				require.Equal(t, cache.tasks, after.tasks, "cancellation replaced accepted log evidence")
				require.Equal(t, cache.final, after.final)
			})
		}
	}
}

func TestPVEBackupLifecycleSizingBudgetKeepsAcceptedListings(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var storageReads atomic.Int32
				c := &backupLifecycleClient{label: "accepted", read: func(ctx context.Context, phase string) {
					if phase == "storage" && storageReads.Add(1) == 2 {
						// The listing succeeded before this optional enrichment.
						// Exhaust only the snapshot child, not its runtime owner.
						<-ctx.Done()
					}
				}}
				m := newBackupLifecycleMonitor(c, kind)
				scheduleBackupLifecycle(m, c)
				synctest.Wait()
				require.Equal(t, int32(2), storageReads.Load(), "fixture must reach snapshot sizing")
				time.Sleep(61 * time.Second)
				synctest.Wait()
				state := m.state.GetSnapshot()
				require.Len(t, state.PVEBackups.GuestSnapshots, 2, "accepted listing and independent peer survive sizing timeout")
				for _, snap := range state.PVEBackups.GuestSnapshots {
					if snap.Instance == "site-a" {
						require.Equal(t, "accepted", snap.Name)
						require.Zero(t, snap.SizeBytes, "missing enrichment must not invent a size")
					}
				}
				require.Empty(t, m.pveBackupPolls, "timed-out sizing must release the completed owner")
				if os.Getenv("PULSE_BACKUP_LIFECYCLE_FRAMES") == "1" && kind == "qemu" {
					data, err := json.Marshal(map[string]any{"accepted": state.PVEBackups, "vms": state.VMs, "containers": state.Containers})
					require.NoError(t, err)
					t.Log("BACKUP_SNAPSHOT_BUDGET_FRAME=" + string(data))
				}
			})
		})
	}
}

func TestPVEBackupLifecycleCompletedReplacementRefreshesImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		old := &backupLifecycleClient{label: "old"}
		m := newBackupLifecycleMonitor(old, "qemu")
		m.config.BackupPollingInterval = 24 * time.Hour
		scheduleBackupLifecycle(m, old)
		synctest.Wait()
		fresh := &backupLifecycleClient{label: "current"}
		m.replacePVEClient("site-a", fresh)
		scheduleBackupLifecycle(m, fresh)
		synctest.Wait()
		require.NotZero(t, fresh.calls.Load(), "a completed previous binding must not delay the new client's first scan")
		for _, row := range m.state.GetSnapshot().PVEBackups.GuestSnapshots {
			if row.Instance == "site-a" {
				require.Equal(t, "current", row.Name)
			}
		}
	})
}

func TestPVEBackupLifecycleRuntimeReplacementKeepsSameClientSafe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var taskReads atomic.Int32
		var oldCtx context.Context
		c := &backupLifecycleClient{label: "accepted", read: func(ctx context.Context, p string) {
			if p == "tasks" && taskReads.Add(1) == 1 {
				oldCtx = ctx
				<-release
			}
		}}
		m := newBackupLifecycleMonitor(c, "qemu")
		scheduleBackupLifecycle(m, c)
		synctest.Wait()
		m.setRuntimeContext(context.Background(), nil)
		scheduleBackupLifecycle(m, c)
		synctest.Wait()
		before := m.state.GetSnapshot().PVEBackups
		require.Equal(t, int32(2), taskReads.Load())
		close(release)
		synctest.Wait()
		require.Error(t, oldCtx.Err())
		require.Equal(t, before, m.state.GetSnapshot().PVEBackups)
	})
}
