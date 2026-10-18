package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

type historyPVEClient struct {
	PVEClientInterface
	history proxmox.BackupTaskHistory
}

func TestBackupHistoryGuestLogRequiresCompleteAttribution(t *testing.T) {
	job := models.BackupTask{EndTime: time.Now()}
	guests := []models.BackupTask{{VMID: 100}}
	lines := []proxmox.TaskLogLine{{LineNumber: 1, Text: "INFO: Starting Backup of VM 100 (qemu)"}, {LineNumber: 2, Text: "TASK OK"}}
	require.True(t, backupGuestLogComplete(job, lines, guests))
	require.False(t, backupGuestLogComplete(job, lines, nil))
	require.False(t, backupGuestLogComplete(job, lines[:1], guests))
	lines[0].LineNumber = 2
	require.False(t, backupGuestLogComplete(job, lines, guests))
	lines[0].LineNumber = 1
	job.EndTime = time.Time{}
	require.False(t, backupGuestLogComplete(job, lines, guests))
}

func TestBackupHistoryIncompleteFinishedJobLogIsRetried(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	client := &jobBackupTaskClient{
		task:     proxmox.Task{UPID: testJobUPID, Node: "node", Type: "vzdump", Status: "OK", StartTime: start.Unix(), EndTime: start.Add(time.Minute).Unix()},
		logLines: testLogLines("INFO: Starting Backup of VM 100 (qemu)", "INFO: Finished Backup of VM 100 (00:01:00)"),
	}
	m := &Monitor{state: models.NewState()}
	m.pollBackupTasks(context.Background(), "site", client)
	client.logLines = append(client.logLines, proxmox.TaskLogLine{LineNumber: 3, Text: "TASK OK"})
	m.pollBackupTasks(context.Background(), "site", client)
	require.Equal(t, 2, client.logRequests)
	require.True(t, m.vzdumpJobTaskCache["site|"+testJobUPID].historyComplete)
	m.pollBackupTasks(context.Background(), "site", client)
	require.Equal(t, 2, client.logRequests, "complete terminal logs remain cached")
}

func (c *historyPVEClient) GetBackupTaskHistory(context.Context, time.Time) (proxmox.BackupTaskHistory, error) {
	return c.history, nil
}

func TestBackupHistoryPollPersistsFailuresAcrossEmptySubsequentPoll(t *testing.T) {
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	m := &Monitor{state: models.NewState(), recoveryManager: manager}
	client := &historyPVEClient{history: proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: "node", Complete: true}}, Tasks: []proxmox.Task{{UPID: "UPID:node:1:2:3:vzdump:100:root@pam:", Node: "node", ID: "100", Type: "vzdump", StartTime: time.Now().Add(-time.Minute).Unix(), EndTime: time.Now().Unix(), Status: "ERROR: backup storage unavailable"}}}}
	m.pollBackupTasks(context.Background(), "site", client)
	client.history.Tasks = nil
	m.pollBackupTasks(context.Background(), "site", client)
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Kind: recovery.KindOther})
	require.NoError(t, err)
	require.Len(t, points, 1)
	require.Equal(t, recovery.OutcomeFailed, points[0].Outcome)
	coverage, err := store.BackupHistoryCoverage(context.Background(), time.Now().Add(-time.Hour), time.Now())
	require.NoError(t, err)
	require.Len(t, coverage.Sources, 1)
	require.True(t, coverage.Sources[0].Complete)
}

func TestBackupHistoryBackfillKeepsLiveTaskPayloadBounded(t *testing.T) {
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	m := &Monitor{state: models.NewState(), recoveryManager: manager}
	client := &historyPVEClient{history: proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: "node", Complete: true}}}}
	for i := 0; i < 301; i++ {
		client.history.Tasks = append(client.history.Tasks, proxmox.Task{UPID: fmt.Sprintf("task-%d", i), Node: "node", ID: "100", Type: "vzdump", StartTime: time.Now().Add(-time.Duration(i+2) * time.Minute).Unix(), EndTime: time.Now().Add(-time.Duration(i+1) * time.Minute).Unix(), Status: "OK"})
	}
	m.pollBackupTasks(context.Background(), "site", client)
	require.Len(t, m.state.GetSnapshot().PVEBackups.BackupTasks, 200)
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	_, total, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Kind: recovery.KindOther})
	require.NoError(t, err)
	require.Equal(t, 301, total, "backfilled evidence survives outside the live API window")
}

func TestBackupHistoryTruncationDoesNotHideAnotherNodesFailure(t *testing.T) {
	m := &Monitor{}
	client := &historyPVEClient{history: proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: "failed", Reason: "collection_failed"}, {Node: "capped", Reason: "history_truncated"}}}}
	_, collection, err := m.readBackupTaskHistory(context.Background(), "site", client)
	require.NoError(t, err)
	require.False(t, collection.Complete)
	require.Equal(t, "collection_failed", collection.Reason)
}
