package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
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

// Ordinary PVE HTTP reads -> poll publication -> live payload and execution
// history. Inventory and log availability can recover on different polls.
func TestBackupHistoryGuestLogsSurviveUnreadInventory(t *testing.T) {
	const instance = "site"
	job, prefix := vzdumpGuestResultFixture()
	job.StartTime = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	job.EndTime = job.StartTime.Add(10 * time.Minute)
	full := append(append([]proxmox.TaskLogLine{}, prefix...),
		proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"},
		proxmox.TaskLogLine{LineNumber: 7, Text: "TASK ERROR: one guest failed"})
	short := append(append([]proxmox.TaskLogLine{}, prefix[:2]...),
		proxmox.TaskLogLine{LineNumber: 3, Text: "TASK ERROR: one guest failed"})
	var phase, reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		p := phase.Load()
		var data any
		switch r.URL.Path {
		case "/api2/json/nodes":
			status := "online"
			if p == 5 {
				status = "offline"
			}
			data = []proxmox.Node{{Node: "pve1", Status: status}, {Node: "pve2", Status: "online"}}
		case "/api2/json/access/permissions":
			data = map[string]any{r.URL.Query().Get("path"): map[string]int{"Sys.Audit": 1}}
		case "/api2/json/nodes/pve1/tasks":
			if p == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			task := proxmox.Task{UPID: testJobUPID, Node: "pve1", Type: "vzdump", StartTime: job.StartTime.Unix()}
			if p >= 2 {
				task.EndTime, task.Status = job.EndTime.Unix(), "job errors"
			}
			data = []proxmox.Task{task}
		case "/api2/json/nodes/pve2/tasks":
			data = []proxmox.Task{{UPID: "sibling", Node: "pve2", Type: "vzdump", ID: "104",
				StartTime: job.StartTime.Unix(), EndTime: job.StartTime.Add(time.Minute).Unix(), Status: "OK"}}
		case "/api2/json/nodes/pve1/tasks/" + testJobUPID + "/log":
			reads.Add(1)
			switch p {
			case 0:
				data = prefix
			case 3:
				data = short
			case 4:
				data = full
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		default:
			t.Errorf("unexpected read %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
	require.NoError(t, err)

	ctx := context.Background()
	dataDir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dataDir, "absent-test-legacy.key"))
	manager := recoverymanager.New(config.NewMultiTenantPersistence(dataDir))
	m := &Monitor{state: models.NewState(), recoveryManager: manager}
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	key := instance + "|" + testJobUPID
	parent := instance + "-" + testJobUPID
	peer := models.BackupTask{ID: "fixture-peer", Instance: "other-site", Node: "pve1", VMID: 102,
		Type: "vzdump", Status: "OK", StartTime: job.StartTime, EndTime: job.EndTime, ObservedAt: job.EndTime}
	m.state.UpdateBackupTasksForInstance(peer.Instance, []models.BackupTask{peer})

	frames := map[string]any{}
	var accepted, completed map[string]models.BackupTask
	for p, label := range []string{"accepted", "inventory-unread", "listed-log-unread", "listed-log-short", "complete", "node-offline", "final-listed"} {
		t.Run(label, func(t *testing.T) {
			phase.Store(int32(p))
			coverage, err := store.BackupHistoryCoverage(ctx, job.StartTime, time.Now().UTC())
			require.NoError(t, err)
			for _, source := range coverage.Sources {
				if wait := time.Until(source.LastObservedAt.Add(time.Millisecond)); wait > 0 {
					time.Sleep(wait)
				}
			}
			m.pollBackupTasks(ctx, instance, client)
			snapshot := m.PVEBackupsSnapshot()
			frames[label] = map[string]any{"data": snapshot}
			live := map[string]models.BackupTask{}
			for _, task := range snapshot.BackupTasks {
				live[task.ID] = task
			}
			assert.Equal(t, peer, live[peer.ID])
			assert.Equal(t, "OK", live[instance+"-sibling"].Status)
			firstID, failedID, lastID := parent+"-vm101", parent+"-vm102", parent+"-vm103"
			first, failed, last := live[firstID], live[failedID], live[lastID]
			assert.Equal(t, "OK", first.Status)
			assert.Equal(t, "error", failed.Status)
			assert.Equal(t, "storage unavailable", failed.Error)
			assert.Equal(t, []string{"running", "running", "unknown", "unknown", "OK", "OK", "OK"}[p], last.Status)
			assert.EqualValues(t, []int{1, 1, 2, 3, 4, 4, 4}[p], reads.Load())
			assert.Equal(t, p >= 4, m.vzdumpJobTaskCache[key].final)
			switch p {
			case 0:
				accepted = live
			case 1:
				assert.Equal(t, accepted[failedID], failed, "unread inventory carries without renewal")
				assert.Equal(t, accepted[lastID], last)
			case 2, 3:
				assert.Equal(t, accepted[firstID], first, "retain the known finish, including observation time")
				assert.Equal(t, accepted[failedID], failed, "retain failure without inventing a finish")
				assert.Equal(t, accepted[lastID].ObservedAt, last.ObservedAt)
				assert.True(t, last.EndTime.IsZero(), "the parent end does not measure the guest")
				_, active := m.resolveBackupIntentContext("", instance, job.Node, 103, accepted[lastID].ObservedAt.Add(time.Second))
				assert.False(t, active, "an ended parent withdraws running intent")
			case 4:
				completed = live
				assert.True(t, job.StartTime.Add(3*time.Minute).Equal(last.EndTime))
				assert.Equal(t, accepted[lastID].ID, last.ID)
			case 5:
				assert.Equal(t, completed[lastID], last, "offline does not reobserve a complete log")
			}
			points, _, err := store.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
			require.NoError(t, err)
			byID := map[string]recovery.RecoveryPoint{}
			for _, point := range points {
				byID[point.ID] = point
			}
			assert.Equal(t, p >= 4, byID["pve-task:"+parent].Details["guestTaskDetailsComplete"])
			assert.Equal(t, recovery.OutcomeFailed, byID["pve-task:"+failedID].Outcome)
			assert.Equal(t, []recovery.Outcome{
				recovery.OutcomeRunning, recovery.OutcomeRunning, recovery.OutcomeUnknown,
				recovery.OutcomeUnknown, recovery.OutcomeSuccess, recovery.OutcomeSuccess, recovery.OutcomeSuccess,
			}[p], byID["pve-task:"+lastID].Outcome)
		})
	}
	if os.Getenv("PULSE_BACKUP_LOG_RECOVERY_FRAMES") == "1" {
		b, err := json.Marshal(frames)
		require.NoError(t, err)
		t.Log("TASK_LOG_RECOVERY_FRAMES=" + string(b))
	}
}

func TestBackupHistoryGuestLogFallbackRespectsWithdrawal(t *testing.T) {
	for _, withdrawal := range []string{"empty-inventory", "access-denied", "removed-node", "empty-log-on-partial-list"} {
		t.Run(withdrawal, func(t *testing.T) {
			ctx := context.Background()
			job, logLines := vzdumpGuestResultFixture()
			task := proxmox.Task{UPID: testJobUPID, Node: job.Node, Type: job.Type, StartTime: job.StartTime.Unix()}
			listed := proxmox.BackupTaskHistory{
				Nodes: []proxmox.BackupTaskHistoryNode{{Node: job.Node, Complete: true}},
				Tasks: []proxmox.Task{task},
			}
			client := &scheduledVzdumpClient{history: listed, read: func(string) ([]proxmox.TaskLogLine, error) { return logLines, nil }}
			m := &Monitor{state: models.NewState()}
			m.pollBackupTasks(ctx, job.Instance, client)
			require.Len(t, m.state.GetBackupTasks(), 4)
			unread := proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: job.Node, Reason: "collection_failed"}}}
			client.history = unread
			m.pollBackupTasks(ctx, job.Instance, client)
			require.Len(t, m.state.GetBackupTasks(), 4, "unread is not withdrawal")

			client.history = proxmox.BackupTaskHistory{}
			wantRows := 0
			switch withdrawal {
			case "empty-inventory":
				client.history.Nodes = listed.Nodes
			case "access-denied":
				client.history.Nodes = []proxmox.BackupTaskHistoryNode{{Node: job.Node, Reason: "access_denied"}}
			case "empty-log-on-partial-list":
				client.history = unread
				client.history.Tasks = listed.Tasks
				client.read = func(string) ([]proxmox.TaskLogLine, error) { return []proxmox.TaskLogLine{}, nil }
				wantRows = 1
			}
			m.pollBackupTasks(ctx, job.Instance, client)
			assert.Len(t, m.state.GetBackupTasks(), wantRows)
			if wantRows == 0 {
				assert.NotContains(t, m.vzdumpJobTaskCache, job.Instance+"|"+testJobUPID)
			}

			client.history = listed
			client.read = func(string) ([]proxmox.TaskLogLine, error) { return nil, fmt.Errorf("fixture log unavailable") }
			m.pollBackupTasks(ctx, job.Instance, client)
			assert.Len(t, m.state.GetBackupTasks(), 1, "a later failed log cannot resurrect withdrawn children")
			client.read = func(string) ([]proxmox.TaskLogLine, error) { return logLines, nil }
			m.pollBackupTasks(ctx, job.Instance, client)
			assert.Len(t, m.state.GetBackupTasks(), 4, "the next actual log recovers the children")
		})
	}
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

type backupHistoryDeadlineClient struct {
	historyPVEClient
	readLog func(context.Context, string) ([]proxmox.TaskLogLine, error)
}

func (c *backupHistoryDeadlineClient) GetTaskLog(ctx context.Context, _, upid string) ([]proxmox.TaskLogLine, error) {
	return c.readLog(ctx, upid)
}

func TestBackupHistoryRetainsAcceptedResultsAfterLogDeadline(t *testing.T) {
	const instance, node = "fixture-site", "pve"
	const first, later = "fixture-observed-job", "fixture-unread-job"

	for _, phase := range []string{"log-deadline", "runtime-cancelled", "connection-replaced"} {
		t.Run(phase, func(t *testing.T) {
			parent, stop := context.WithCancel(context.Background())
			defer stop()
			start := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
			listed := []proxmox.Task{
				{UPID: first, Node: node, Type: "vzdump", Status: "job errors", StartTime: start.Unix(), EndTime: start.Add(5 * time.Minute).Unix()},
				{UPID: later, Node: node, Type: "vzdump", Status: "OK", StartTime: start.Add(time.Minute).Unix(), EndTime: start.Add(6 * time.Minute).Unix()},
			}
			c := &backupHistoryDeadlineClient{historyPVEClient: historyPVEClient{history: proxmox.BackupTaskHistory{
				Tasks: listed, Nodes: []proxmox.BackupTaskHistoryNode{{Node: node, Complete: true}},
			}}}
			manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
			store, err := manager.StoreForOrg("default")
			require.NoError(t, err)
			defer store.Close()
			m := &Monitor{
				state: models.NewState(), recoveryManager: manager,
				pveClients: map[string]PVEClientInterface{instance: c}, runtimeCtx: parent,
			}
			peer := models.BackupTask{ID: "peer-task", Instance: "peer-site", VMID: 101, Status: "OK"}
			m.state.UpdateBackupTasksForInstance(peer.Instance, []models.BackupTask{peer})
			var reads []string
			c.readLog = func(ctx context.Context, upid string) ([]proxmox.TaskLogLine, error) {
				reads = append(reads, upid)
				if upid == first {
					return testLogLines(
						"INFO: Starting Backup of VM 101 (qemu)",
						"ERROR: Backup of VM 101 failed - fixture archive unavailable",
						"INFO: Starting Backup of VM 102 (qemu)",
						"INFO: Finished Backup of VM 102 (00:01:00)",
						"TASK ERROR: job errors",
					), nil
				}
				require.Equal(t, later, upid)
				switch phase {
				case "log-deadline":
					<-ctx.Done()
					require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
				case "runtime-cancelled":
					stop()
					require.ErrorIs(t, ctx.Err(), context.Canceled)
				case "connection-replaced":
					m.replacePVEClient(instance, &backupHistoryDeadlineClient{})
					require.ErrorIs(t, ctx.Err(), context.Canceled)
				}
				// A provider can return a decoded result after cancellation.
				// That is not an accepted guest observation.
				return testLogLines(
					"INFO: Starting Backup of VM 103 (qemu)",
					"INFO: Finished Backup of VM 103 (00:01:00)",
					"TASK OK",
				), nil
			}

			owner := m.beginPVEBackupPoll(instance, c, time.Time{}, time.Now())
			require.NotNil(t, owner)
			poll := context.WithValue(owner.ctx, pveBackupPollContextKey{}, owner)
			budget := time.Minute
			if phase == "log-deadline" {
				budget = 2 * time.Second
			}
			m.pollPVEBackupsAndSnapshots(poll, instance, c, nil, nil, budget)
			m.finishPVEBackupPoll(instance, owner)
			require.Equal(t, []string{first, later}, reads)

			query := context.Background()
			from, to := start.Add(-time.Minute), time.Now().UTC()
			points, coverage, err := store.BackupHealthHistory(query, from, to)
			require.NoError(t, err)
			if phase != "log-deadline" {
				require.Equal(t, []models.BackupTask{peer}, m.state.GetBackupTasks())
				require.Empty(t, points)
				require.Empty(t, coverage.Sources)
				return
			}

			live := make(map[string]models.BackupTask)
			for _, task := range m.state.GetBackupTasks() {
				live[task.ID] = task
			}
			require.Len(t, live, 5)
			require.Equal(t, peer, live[peer.ID])
			require.Equal(t, "error", live[instance+"-"+first+"-vm101"].Status)
			require.Equal(t, "OK", live[instance+"-"+first+"-vm102"].Status)
			require.Len(t, points, 4, "accepted live results must also reach durable history")
			require.Len(t, coverage.Sources, 1, "the source observation belongs with the accepted task evidence")

			outcomes := make(map[string]recovery.RecoveryPoint)
			for _, point := range points {
				require.Equal(t, recovery.KindOther, point.Kind, "job outcomes are not restore artifacts")
				outcomes[point.ID] = point
			}
			id := "pve-task:" + instance + "-"
			require.Equal(t, recovery.OutcomeFailed, outcomes[id+first+"-vm101"].Outcome)
			require.Equal(t, recovery.OutcomeSuccess, outcomes[id+first+"-vm102"].Outcome)
			require.Equal(t, true, outcomes[id+first].Details["guestTaskDetailsComplete"])
			require.Equal(t, false, outcomes[id+later].Details["guestTaskDetailsComplete"])
			require.NotContains(t, outcomes, id+later+"-vm103")

			c.history.Tasks = nil
			m.pollBackupTasks(query, instance, c)
			require.Equal(t, []models.BackupTask{peer}, m.state.GetBackupTasks(), "the next accepted empty inventory only removes live rows")
			retained, _, err := store.BackupHealthHistory(query, from, time.Now().UTC())
			require.NoError(t, err)
			require.ElementsMatch(t, points, retained)
		})
	}
}
