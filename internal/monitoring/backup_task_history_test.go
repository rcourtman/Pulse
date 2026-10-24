package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

type historyPVEClient struct {
	PVEClientInterface
	history proxmox.BackupTaskHistory
}

func TestBackupHistoryGuestLogRequiresCompleteAttribution(t *testing.T) {
	job := models.BackupTask{EndTime: time.Now()}
	guests := []models.BackupTask{{VMID: 100, Status: "OK"}}
	lines := testLogLines("INFO: Starting Backup of VM 100 (qemu)", "INFO: Finished Backup of VM 100 (00:01:00)", "TASK OK")
	require.True(t, backupGuestLogComplete(job, lines, guests))
	require.False(t, backupGuestLogComplete(job, lines, nil))
	require.False(t, backupGuestLogComplete(job, lines[:2], guests))
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

func vzdumpGuestResultFixture() (models.BackupTask, []proxmox.TaskLogLine) {
	start := time.Date(2026, 10, 24, 2, 0, 0, 0, time.UTC)
	job := models.BackupTask{ID: "site-" + testJobUPID, Instance: "site", Node: "pve1", Type: "vzdump", Status: "OK", StartTime: start, EndTime: start.Add(10 * time.Minute)}
	return job, testLogLines(
		"INFO: Starting Backup of VM 101 (qemu)",
		"INFO: Finished Backup of VM 101 (00:01:00)",
		"INFO: Starting Backup of VM 102 (lxc)",
		"ERROR: Backup of VM 102 failed - storage unavailable",
		"INFO: Starting Backup of VM 103 (qemu)",
	)
}

func assertVzdumpGuestResults(t *testing.T, tasks []models.BackupTask, last string) {
	t.Helper()
	require.Len(t, tasks, 3)
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].VMID < tasks[j].VMID })
	require.Equal(t, "OK", tasks[0].Status, "explicit success must survive another guest's missing verdict")
	require.Equal(t, "error", tasks[1].Status, "explicit failure must not be hidden as unknown")
	require.Equal(t, "storage unavailable", tasks[1].Error)
	require.Equal(t, last, tasks[2].Status)
	points := proxmoxmapper.FromPVEBackupTasks(tasks, nil)
	require.Len(t, points, 3)
	require.Equal(t, recovery.OutcomeSuccess, points[0].Outcome)
	require.Equal(t, recovery.OutcomeFailed, points[1].Outcome)
	wantOutcome := recovery.OutcomeUnknown
	if last == "OK" {
		wantOutcome = recovery.OutcomeSuccess
	} else if last == "running" {
		wantOutcome = recovery.OutcomeRunning
	}
	require.Equal(t, wantOutcome, points[2].Outcome)
	for i, task := range tasks {
		require.Equal(t, recovery.KindOther, points[i].Kind, "task results are not restore artifacts")
		require.Equal(t, "site", task.Instance)
		require.Equal(t, "pve1", task.Node)
		require.Equal(t, "site-"+testJobUPID+fmt.Sprintf("-vm%d", task.VMID), task.ID)
	}
}

func TestVzdumpGuestResultMissingVerdictIsUnknown(t *testing.T) {
	for _, status := range []string{"OK", "job errors", "unexpected status", "WARNINGS: 1"} {
		for _, shape := range []string{"prefix", "gapped", "capped", "terminal-ok", "terminal-error", "terminal-warning"} {
			t.Run(status+"/"+shape, func(t *testing.T) {
				job, lines := vzdumpGuestResultFixture()
				job.Status = status
				switch shape {
				case "gapped":
					lines[2].LineNumber++
				case "capped":
					for len(lines) < 20000 {
						lines = append(lines, proxmox.TaskLogLine{LineNumber: int64(len(lines) + 1), Text: "INFO: progress"})
					}
				case "terminal-ok":
					lines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "TASK OK"})
				case "terminal-error":
					lines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "TASK ERROR: job errors"})
				case "terminal-warning":
					lines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "TASK WARNINGS: 1"})
				}
				tasks := parseVzdumpJobLog(job, testJobUPID, lines)
				assertVzdumpGuestResults(t, tasks, "unknown")
				require.Equal(t, "backup job ended without an observed guest result", tasks[2].Error)
				require.Equal(t, job.EndTime, tasks[2].EndTime, "ended parent must not leave a guest apparently running")
				require.False(t, backupGuestLogComplete(job, lines, tasks), "missing verdict must remain retryable even with a terminal marker")
			})
		}
	}
}

func TestVzdumpGuestResultRunningStillRunning(t *testing.T) {
	job, lines := vzdumpGuestResultFixture()
	job.EndTime = time.Time{}
	tasks := parseVzdumpJobLog(job, testJobUPID, lines)
	assertVzdumpGuestResults(t, tasks, "running")
	require.Empty(t, tasks[2].Error)
	require.True(t, tasks[2].EndTime.IsZero())
	require.False(t, backupGuestLogComplete(job, lines, tasks))
}

func TestVzdumpGuestResultCompleteLogStillRequiresEveryVerdict(t *testing.T) {
	job, lines := vzdumpGuestResultFixture()
	lines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "TASK ERROR: job errors"})
	for _, status := range []string{"unknown", "running", ""} {
		t.Run(status, func(t *testing.T) {
			guests := []models.BackupTask{{VMID: 101, Status: "OK"}, {VMID: 102, Status: "error"}, {VMID: 103, Status: status}}
			require.False(t, backupGuestLogComplete(job, lines, guests))
		})
	}
}

func TestVzdumpGuestResultExplicitVerdictsKeepLogBounds(t *testing.T) {
	for _, shape := range []string{"complete-ok", "complete-error", "complete-warning", "missing-terminal", "gapped", "capped"} {
		t.Run(shape, func(t *testing.T) {
			job, lines := vzdumpGuestResultFixture()
			lines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"})
			terminal := "TASK OK"
			if shape == "complete-error" {
				terminal = "TASK ERROR: job errors"
			} else if shape == "complete-warning" {
				terminal = "TASK WARNINGS: 1"
			}
			if shape == "capped" {
				for len(lines) < 19999 {
					lines = append(lines, proxmox.TaskLogLine{LineNumber: int64(len(lines) + 1), Text: "INFO: progress"})
				}
			}
			if shape != "missing-terminal" {
				lines = append(lines, proxmox.TaskLogLine{LineNumber: int64(len(lines) + 1), Text: terminal})
			}
			if shape == "gapped" {
				lines[0].LineNumber = 2
			}
			tasks := parseVzdumpJobLog(job, testJobUPID, lines)
			assertVzdumpGuestResults(t, tasks, "OK")
			require.Equal(t, shape == "complete-ok" || shape == "complete-error" || shape == "complete-warning", backupGuestLogComplete(job, lines, tasks))
		})
	}
}

func TestVzdumpGuestResultPollRetriesUnknownUntilExplicitResult(t *testing.T) {
	job, lines := vzdumpGuestResultFixture()
	client := &jobBackupTaskClient{task: proxmox.Task{UPID: testJobUPID, Node: job.Node, Type: "vzdump", Status: job.Status, StartTime: job.StartTime.Unix(), EndTime: job.EndTime.Unix()}, logLines: lines}
	m := &Monitor{state: models.NewState()}
	other := models.BackupTask{ID: "another-instance", Instance: "other", VMID: 103, Status: "error"}
	m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{other})
	frames := map[string]any{}
	capture := func(phase, last string, complete bool) {
		t.Helper()
		m.pollBackupTasks(context.Background(), "site", client)
		snapshot := m.state.GetSnapshot().PVEBackups
		var guests []models.BackupTask
		var parents []models.BackupTask
		for _, task := range snapshot.BackupTasks {
			if task.Instance == "other" {
				require.Equal(t, other, task, "poll must not cross instance scope")
			} else if task.VMID > 0 {
				guests = append(guests, task)
			} else {
				parents = append(parents, task)
			}
		}
		require.Len(t, snapshot.BackupTasks, 5)
		assertVzdumpGuestResults(t, guests, last)
		points := proxmoxmapper.FromPVEBackupTasks(snapshot.BackupTasks, nil)
		m.qualifyBackupGuestTaskDetails("site", parents, map[string]string{job.ID: testJobUPID}, points)
		for _, point := range points {
			if point.ID == "pve-task:"+job.ID {
				require.Equal(t, complete, point.Details["guestTaskDetailsComplete"])
			}
		}
		entry := m.vzdumpJobTaskCache["site|"+testJobUPID]
		require.Equal(t, complete, entry.historyComplete)
		require.Equal(t, complete, entry.final)
		for _, task := range guests {
			require.False(t, task.ObservedAt.IsZero())
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": snapshot}, "points": points, "complete": complete, "log_requests": client.logRequests}
	}
	capture("prefix", "unknown", false)
	client.logLines = append(client.logLines, proxmox.TaskLogLine{LineNumber: 6, Text: "TASK ERROR: job errors"})
	capture("terminal-unresolved", "unknown", false)
	client.logLines = append(lines, proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"}, proxmox.TaskLogLine{LineNumber: 7, Text: "TASK WARNINGS: 1"})
	capture("resolved", "OK", true)
	client.logLines = nil // final logs are immutable; no new read should happen
	capture("cached", "OK", true)
	require.Equal(t, 3, client.logRequests)
	if os.Getenv("PULSE_VZDUMP_RESULT_EXPORT") == "1" {
		sources := map[string]string{}
		for _, file := range []string{"monitor_backups.go", "backup_task_history.go"} {
			data, err := os.ReadFile(file)
			require.NoError(t, err)
			sources["internal/monitoring/"+file] = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		data, err := json.Marshal(map[string]any{"runtime": sources, "observed_at": job.EndTime.Add(time.Hour), "frames": frames})
		require.NoError(t, err)
		t.Log("VZDUMP_RESULT_PRODUCER=" + string(data))
	}
}
