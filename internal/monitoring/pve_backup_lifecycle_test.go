package monitoring

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Deliberately returns a successful synthetic read after cancellation. A
// transport cancellation alone cannot admit publication or prove completion.
// No guest command, installed service, backup or database is exercised.
type lifecycleBackupClient struct {
	PVEClientInterface
	stage   string
	label   string
	entered chan context.Context
	release chan struct{}
}

func (c *lifecycleBackupClient) block(ctx context.Context, stage string) {
	if c.stage == stage {
		c.entered <- ctx
		<-c.release
	}
}

func (c *lifecycleBackupClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	c.block(ctx, "task")
	id := "100"
	if c.stage == "task-log" {
		id = ""
	}
	return []proxmox.Task{{UPID: c.label, Node: "node", ID: id, Type: "vzdump", Status: "OK", StartTime: 1700000000, EndTime: 1700000060}}, nil
}

func (c *lifecycleBackupClient) GetTaskLog(ctx context.Context, _, _ string) ([]proxmox.TaskLogLine, error) {
	c.block(ctx, "task-log")
	return []proxmox.TaskLogLine{{LineNumber: 1, Text: "INFO: Starting Backup of VM 100"}, {LineNumber: 2, Text: "INFO: Finished Backup of VM 100 (00:01:00)"}, {LineNumber: 3, Text: "TASK OK"}}, nil
}

func (c *lifecycleBackupClient) GetStorage(ctx context.Context, _ string) ([]proxmox.Storage, error) {
	return []proxmox.Storage{{Storage: "local", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}

func (c *lifecycleBackupClient) GetStorageContent(ctx context.Context, _, _ string) ([]proxmox.StorageContent, error) {
	c.block(ctx, "storage")
	return []proxmox.StorageContent{{Volid: "local:backup/vzdump-qemu-100-" + c.label + ".vma.zst", Content: "backup", VMID: 100, CTime: 1700000060, Size: 1024}}, nil
}

func (c *lifecycleBackupClient) GetVMSnapshots(ctx context.Context, _ string, _ int) ([]proxmox.Snapshot, error) {
	c.block(ctx, "snapshot")
	return []proxmox.Snapshot{{Name: c.label, SnapTime: 1700000060}}, nil
}

func backupLifecycleMonitor(client PVEClientInterface) *Monitor {
	m := &Monitor{state: models.NewState(), config: &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}}
	m.replacePVEClient("site", client)
	m.state.UpdateVMsForInstance("site", []models.VM{{ID: "site:node:100", Instance: "site", Node: "node", VMID: 100, Name: "fixture-guest", Status: "running"}})
	return m
}

func startBackupLifecycleScan(m *Monitor, client PVEClientInterface) {
	m.pollPVEBackupsAsync("site", &config.PVEInstance{MonitorBackups: true}, client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"})
}

func TestPVEBackupLifecycleWithdrawsLateResults(t *testing.T) {
	for _, stage := range []string{"task", "task-log", "storage", "snapshot"} {
		for _, withdrawal := range []string{"replace", "retire", "runtime"} {
			t.Run(stage+"/"+withdrawal, func(t *testing.T) {
				old := &lifecycleBackupClient{stage: stage, label: "withdrawn", entered: make(chan context.Context, 1), release: make(chan struct{})}
				var once sync.Once
				unblock := func() { once.Do(func() { close(old.release) }) }
				defer unblock()
				m := backupLifecycleMonitor(old)
				startBackupLifecycleScan(m, old)
				var readCtx context.Context
				select {
				case readCtx = <-old.entered:
				case <-time.After(5 * time.Second):
					t.Fatal("scan did not reach controlled read")
				}
				fresh := &lifecycleBackupClient{label: "current"}
				switch withdrawal {
				case "replace":
					m.replacePVEClient("site", fresh)
				case "retire":
					m.retirePVEInstanceRuntime("site")
				case "runtime":
					m.setRuntimeContext(context.Background(), nil)
				}
				select {
				case <-readCtx.Done():
				case <-time.After(150 * time.Millisecond):
					t.Error("withdrawal did not cancel old backup read")
				}
				before := m.state.GetSnapshot().PVEBackups
				unblock()
				waitForBackupPollReturn(t, m, "site")
				require.Equal(t, before, m.state.GetSnapshot().PVEBackups, "withdrawn scan must not republish tasks, archives or snapshots")
				m.mu.RLock()
				_, cached := m.vzdumpJobTaskCache["site|withdrawn"]
				m.mu.RUnlock()
				require.False(t, cached, "withdrawn log must not repopulate the job cache")
				m.replacePVEClient("site", fresh)
				m.state.UpdateVMsForInstance("site", []models.VM{{ID: "site:node:100", Instance: "site", Node: "node", VMID: 100, Name: "fixture-guest", Status: "running"}})
				startBackupLifecycleScan(m, fresh)
				waitForBackupPollReturn(t, m, "site")
				after := m.state.GetSnapshot().PVEBackups
				require.Len(t, after.BackupTasks, 1)
				require.Equal(t, "site-current", after.BackupTasks[0].ID)
				require.Len(t, after.StorageBackups, 1)
				require.Contains(t, after.StorageBackups[0].Volid, "current")
				require.Len(t, after.GuestSnapshots, 1)
				require.Equal(t, "current", after.GuestSnapshots[0].Name)
				t.Logf("BACKUP_LIFECYCLE_%s_%s cancelled=true late_effects=0 current=1/1/1", stage, withdrawal)
			})
		}
	}
}

func TestPVEBackupLifecycleRefusesUnregisteredClient(t *testing.T) {
	client := &lifecycleBackupClient{stage: "task", entered: make(chan context.Context, 1), release: make(chan struct{})}
	defer close(client.release)
	m := backupLifecycleMonitor(&lifecycleBackupClient{label: "registered"})
	startBackupLifecycleScan(m, client)
	waitForBackupPollReturn(t, m, "site")
	select {
	case <-client.entered:
		t.Fatal("unregistered client started a backup scan")
	default:
	}
	t.Log(fmt.Sprintf("BACKUP_LIFECYCLE_UNREGISTERED requests=0"))
}
