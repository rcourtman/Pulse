package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

type scheduledBackupLifecycleClient struct{ backupLifecycleClient }

func (c *scheduledBackupLifecycleClient) GetNodes(context.Context) ([]proxmox.Node, error) {
	return []proxmox.Node{{Node: "node-a", Status: "online"}}, nil
}

func TestPVEBackupLifecycleSchedulerLeavesBackgroundOwnerAlive(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var readCtx context.Context
	c := &scheduledBackupLifecycleClient{backupLifecycleClient: backupLifecycleClient{label: "scheduled", read: func(ctx context.Context, phase string) {
		if phase == "tasks" {
			readCtx = ctx
			close(started)
			<-release
		}
	}}}
	// This scheduler also performs ordinary node reads. Supply the complete
	// existing no-op adapter for methods the focused backup fake does not own,
	// rather than forwarding those methods through a nil embedded interface.
	c.mockPVEClientExtra.mockPVEClient.PVEClientInterface = &stubPVEClient{}
	m := newReplicationLifecycleMonitor(t, c, false)
	m.config.EnableBackupPolling = true
	m.config.BackupPollingCycles = 1
	m.config.PVEInstances[0].MonitorBackups = true
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.setRuntimeContext(runCtx, nil)
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	runReplicationCycle(m)
	waitReplication(t, started, "scheduled backup read")
	require.NoError(t, readCtx.Err(), "finished core-cycle context canceled its background scan")
	m.mu.RLock()
	owner := m.pveBackupPolls["site-a"]
	m.mu.RUnlock()
	require.NotNil(t, owner)
	cancel()
	require.Error(t, readCtx.Err(), "runtime cancellation did not reach the background owner")
	close(release)
	released = true
	waitReplication(t, owner.done, "scheduled backup completion")
	m.mu.RLock()
	remaining := m.pveBackupPolls["site-a"]
	m.mu.RUnlock()
	require.Nil(t, remaining)
	require.Empty(t, m.state.GetSnapshot().PVEBackups.BackupTasks, "canceled scheduler published late tasks")
	// The stopped runtime remains stopped even if another ordinary cycle is due.
	m.mu.Lock()
	m.lastPVEBackupPoll["site-a"] = time.Time{}
	m.mu.Unlock()
	scheduleBackupLifecycle(m, c)
	m.mu.RLock()
	remaining = m.pveBackupPolls["site-a"]
	m.mu.RUnlock()
	require.Nil(t, remaining)
}
