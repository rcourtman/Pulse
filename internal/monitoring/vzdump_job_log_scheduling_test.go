package monitoring

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

type scheduledVzdumpClient struct {
	PVEClientInterface
	history  proxmox.BackupTaskHistory
	requests []string
	read     func(string) ([]proxmox.TaskLogLine, error)
}

func (c *scheduledVzdumpClient) GetBackupTaskHistory(context.Context, time.Time) (proxmox.BackupTaskHistory, error) {
	return c.history, nil
}

func (c *scheduledVzdumpClient) GetTaskLog(_ context.Context, _, upid string) ([]proxmox.TaskLogLine, error) {
	c.requests = append(c.requests, upid)
	return c.read(upid)
}

func scheduledVzdumpJobs(count int, instance string) ([]models.BackupTask, map[string]string) {
	jobs := make([]models.BackupTask, count)
	upids := make(map[string]string, count)
	start := time.Date(2026, 10, 29, 2, 0, 0, 0, time.UTC)
	for i := range jobs {
		upid := fmt.Sprintf("UPID:pve1:%08X:00000001:69016420:vzdump::root@pam:", i+1)
		id := instance + "-" + upid
		jobs[i] = models.BackupTask{ID: id, Instance: instance, Node: "pve1", Type: "vzdump", Status: "OK", StartTime: start, EndTime: start.Add(time.Minute)}
		upids[id] = upid
	}
	return jobs, upids
}

func TestVzdumpJobLogSchedulingPersistentReadsDoNotStarveHistory(t *testing.T) {
	for _, shape := range []string{"error", "incomplete", "empty", "capped"} {
		t.Run(shape, func(t *testing.T) {
			jobs, upids := scheduledVzdumpJobs(3*maxVzdumpJobLogFetchesPerPoll+1, "site")
			original := append([]models.BackupTask(nil), jobs...)
			lines := testLogLines("INFO: Starting Backup of VM 101 (qemu)")
			if shape == "capped" {
				for len(lines) < 19999 {
					lines = append(lines, proxmox.TaskLogLine{LineNumber: int64(len(lines) + 1), Text: "INFO: progress"})
				}
				lines = append(lines, proxmox.TaskLogLine{LineNumber: 20000, Text: "TASK OK"})
			}
			client := &scheduledVzdumpClient{read: func(string) ([]proxmox.TaskLogLine, error) {
				switch shape {
				case "error":
					return nil, fmt.Errorf("synthetic task log unavailable")
				case "empty":
					return nil, nil
				default:
					return lines, nil
				}
			}}
			m := &Monitor{}
			for cycle := 0; cycle < 4; cycle++ {
				before := len(client.requests)
				m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids)
				require.Len(t, client.requests[before:], maxVzdumpJobLogFetchesPerPoll)
			}
			counts := make(map[string]int)
			for _, upid := range client.requests {
				counts[upid]++
			}
			require.Len(t, counts, len(jobs), "persistent early failures must not hide later jobs")
			for _, count := range counts {
				require.LessOrEqual(t, count, 2, "retry only after every pending job has had a turn")
			}
			require.Equal(t, original, jobs, "selection must not reorder provider history")
			for _, entry := range m.vzdumpJobTaskCache {
				require.False(t, entry.final, "an attempt is not complete guest evidence")
				require.False(t, entry.historyComplete)
			}
		})
	}
}

func TestVzdumpJobLogSchedulingPollReachesLaterGuestResults(t *testing.T) {
	jobs, upids := scheduledVzdumpJobs(maxVzdumpJobLogFetchesPerPoll+2, "site")
	client := &scheduledVzdumpClient{history: proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: "pve1", Complete: true}}}}
	for _, job := range jobs {
		client.history.Tasks = append(client.history.Tasks, proxmox.Task{UPID: upids[job.ID], Node: job.Node, Type: job.Type, Status: job.Status, StartTime: job.StartTime.Unix(), EndTime: job.EndTime.Unix()})
	}
	good, failed := upids[jobs[len(jobs)-2].ID], upids[jobs[len(jobs)-1].ID]
	client.read = func(upid string) ([]proxmox.TaskLogLine, error) {
		switch upid {
		case good:
			return testLogLines("INFO: Starting Backup of VM 501 (qemu)", "INFO: Finished Backup of VM 501 (00:00:30)", "TASK OK"), nil
		case failed:
			return testLogLines("INFO: Starting Backup of VM 502 (lxc)", "ERROR: Backup of VM 502 failed - storage unavailable", "TASK ERROR: job errors"), nil
		default:
			return nil, fmt.Errorf("synthetic task log unavailable")
		}
	}
	m := &Monitor{state: models.NewState()}
	other := models.BackupTask{ID: "other-task", Instance: "other", VMID: 501, Status: "error"}
	m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{other})
	m.pollBackupTasks(context.Background(), "site", client)
	require.Len(t, m.state.GetBackupTasks(), len(jobs)+1, "no guest outcome can be inferred from an unread log")
	m.pollBackupTasks(context.Background(), "site", client)
	m.pollBackupTasks(context.Background(), "site", client)
	byID := make(map[string]models.BackupTask)
	for _, task := range m.state.GetBackupTasks() {
		byID[task.ID] = task
	}
	require.Len(t, byID, len(jobs)+3)
	require.Equal(t, other, byID[other.ID])
	require.Equal(t, "OK", byID[jobs[len(jobs)-2].ID+"-vm501"].Status)
	require.Equal(t, "error", byID[jobs[len(jobs)-1].ID+"-vm502"].Status)
	require.Equal(t, "storage unavailable", byID[jobs[len(jobs)-1].ID+"-vm502"].Error)
	for _, upid := range []string{good, failed} {
		count := 0
		for _, read := range client.requests {
			if read == upid {
				count++
			}
		}
		require.Equal(t, 1, count, "complete terminal logs retain their immutable cache")
		require.True(t, m.vzdumpJobTaskCache["site|"+upid].historyComplete)
	}
}

func TestVzdumpJobLogSchedulingCancellationKeepsUnspentTurns(t *testing.T) {
	jobs, upids := scheduledVzdumpJobs(maxVzdumpJobLogFetchesPerPoll+1, "site")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &scheduledVzdumpClient{read: func(string) ([]proxmox.TaskLogLine, error) {
		cancel()
		return nil, context.Canceled
	}}
	m := &Monitor{}
	m.synthesizeVzdumpJobGuestTasks(ctx, "site", client, jobs, upids)
	require.Equal(t, []string{upids[jobs[0].ID]}, client.requests)
	client.requests = nil
	client.read = func(string) ([]proxmox.TaskLogLine, error) { return nil, nil }
	m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids)
	require.Len(t, client.requests, maxVzdumpJobLogFetchesPerPoll)
	require.NotContains(t, client.requests, upids[jobs[0].ID], "only an attempted read spends its turn")
}

func TestVzdumpJobLogSchedulingPrunesAndSeparatesConnections(t *testing.T) {
	jobs, upids := scheduledVzdumpJobs(maxVzdumpJobLogFetchesPerPoll+1, "site")
	client := &scheduledVzdumpClient{read: func(string) ([]proxmox.TaskLogLine, error) { return nil, nil }}
	m := &Monitor{}
	m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids)
	client.requests = nil
	m.synthesizeVzdumpJobGuestTasks(context.Background(), "other", client, jobs, upids)
	require.Equal(t, upids[jobs[0].ID], client.requests[0], "another connection has its own request turns")
	m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, nil, nil)
	require.Len(t, m.vzdumpJobTaskCache, maxVzdumpJobLogFetchesPerPoll, "removed jobs discard scheduling state, not another connection")
	client.requests = nil
	m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids)
	require.Equal(t, upids[jobs[0].ID], client.requests[0], "a new history window has no stale attempt priority")
}

func TestVzdumpJobLogSchedulingConcurrentConnections(t *testing.T) {
	m := &Monitor{}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(instance string) {
			defer wg.Done()
			jobs, upids := scheduledVzdumpJobs(maxVzdumpJobLogFetchesPerPoll+1, instance)
			client := &scheduledVzdumpClient{read: func(string) ([]proxmox.TaskLogLine, error) { return nil, nil }}
			for cycle := 0; cycle < 3; cycle++ {
				m.synthesizeVzdumpJobGuestTasks(context.Background(), instance, client, jobs, upids)
			}
		}(fmt.Sprintf("site-%d", i))
	}
	wg.Wait()
	require.Len(t, m.vzdumpJobTaskCache, 3*(maxVzdumpJobLogFetchesPerPoll+1))
}

func TestVzdumpJobLogSchedulingKeepsLiveReadsAndHistoryMoving(t *testing.T) {
	for _, liveCount := range []int{1, maxVzdumpJobLogFetchesPerPoll + 1} {
		t.Run(fmt.Sprint(liveCount), func(t *testing.T) {
			jobs, upids := scheduledVzdumpJobs(2*maxVzdumpJobLogFetchesPerPoll+liveCount, "site")
			live := make(map[string]bool)
			for i := 2 * maxVzdumpJobLogFetchesPerPoll; i < len(jobs); i++ {
				jobs[i].EndTime = time.Time{}
				jobs[i].Status = "running"
				live[upids[jobs[i].ID]] = true
			}
			client := &scheduledVzdumpClient{read: func(string) ([]proxmox.TaskLogLine, error) { return nil, nil }}
			m := &Monitor{}
			seen := make(map[string]bool)
			// Even a saturated live queue leaves one history turn per poll.
			for cycle := 0; cycle < 2*maxVzdumpJobLogFetchesPerPoll; cycle++ {
				before := len(client.requests)
				m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids)
				reads := client.requests[before:]
				require.Len(t, reads, maxVzdumpJobLogFetchesPerPoll)
				liveReads := 0
				for _, upid := range reads {
					seen[upid] = true
					if live[upid] {
						liveReads++
					}
				}
				if liveCount == 1 {
					require.Equal(t, 1, liveReads, "ordinary live work is not delayed behind history")
				} else {
					require.Equal(t, maxVzdumpJobLogFetchesPerPoll-1, liveReads, "saturated live work still reserves history progress")
				}
			}
			require.Len(t, seen, len(jobs), "both live and ended queues must make progress")
		})
	}
}

func TestVzdumpJobLogSchedulingContextBudgetRotatesInsideAllowance(t *testing.T) {
	jobs, upids := scheduledVzdumpJobs(2, "site")
	client := &scheduledVzdumpClient{}
	m := &Monitor{}
	for cycle := 0; cycle < 2; cycle++ {
		ctx, cancel := context.WithCancel(context.Background())
		client.read = func(string) ([]proxmox.TaskLogLine, error) {
			cancel() // one slow request consumed this cycle's context budget
			return nil, context.DeadlineExceeded
		}
		m.synthesizeVzdumpJobGuestTasks(ctx, "site", client, jobs, upids)
		cancel()
	}
	require.Equal(t, []string{upids[jobs[0].ID], upids[jobs[1].ID]}, client.requests,
		"a small queue must rotate actual execution, not only admission")
}

func TestVzdumpJobLogSchedulingOutputRetainsProviderOrder(t *testing.T) {
	jobs, upids := scheduledVzdumpJobs(2, "site")
	index := map[string]int{upids[jobs[0].ID]: 0, upids[jobs[1].ID]: 1}
	client := &scheduledVzdumpClient{read: func(upid string) ([]proxmox.TaskLogLine, error) {
		return testLogLines(fmt.Sprintf("INFO: Starting Backup of VM %d (qemu)", 501+index[upid])), nil
	}}
	m := &Monitor{}
	for cycle := 0; cycle < 2; cycle++ {
		tasks := m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids)
		require.Len(t, tasks, 2)
		require.Equal(t, []int{501, 502}, []int{tasks[0].VMID, tasks[1].VMID})
		// Force a different scheduling order without changing accepted tasks.
		m.mu.Lock()
		entry := m.vzdumpJobTaskCache["site|"+upids[jobs[1].ID]]
		entry.lastAttempt = time.Time{}
		m.vzdumpJobTaskCache["site|"+upids[jobs[1].ID]] = entry
		m.mu.Unlock()
	}
	require.Equal(t, []string{upids[jobs[0].ID], upids[jobs[1].ID], upids[jobs[1].ID], upids[jobs[0].ID]}, client.requests)
}
