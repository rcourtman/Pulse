package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	recoverymodel "github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

type budgetLogClient struct {
	PVEClientInterface
	read func(context.Context, string) ([]proxmox.TaskLogLine, error)
}

func (c *budgetLogClient) GetTaskLog(ctx context.Context, _, upid string) ([]proxmox.TaskLogLine, error) {
	return c.read(ctx, upid)
}

func budgetLogJobs(count int) ([]models.BackupTask, map[string]string) {
	jobs := make([]models.BackupTask, count)
	upids := make(map[string]string, count)
	for i := range jobs {
		id := fmt.Sprintf("job-%d", i)
		jobs[i] = models.BackupTask{ID: id, Instance: "site", Node: "node", Type: "vzdump", StartTime: time.Unix(1792371600, 0)}
		upids[id] = id
	}
	return jobs, upids
}

func TestVzdumpLogBudgetProgressesAcrossUnfinishedReads(t *testing.T) {
	for _, mode := range []string{"unavailable", "empty", "ambiguous", "running"} {
		t.Run(mode, func(t *testing.T) {
			jobs, upids := budgetLogJobs(2*maxVzdumpJobLogFetchesPerPoll + 3)
			reads := map[string]int{}
			client := &budgetLogClient{read: func(_ context.Context, upid string) ([]proxmox.TaskLogLine, error) {
				reads[upid]++
				switch mode {
				case "unavailable":
					return nil, fmt.Errorf("fixture log unavailable")
				case "empty":
					return nil, nil
				case "ambiguous":
					return testLogLines("INFO: Finished Backup of VM 100 (00:01:00)", "TASK OK"), nil
				default:
					return testLogLines("INFO: Starting Backup of VM 100 (qemu)"), nil
				}
			}}
			m := &Monitor{}
			for cycle := 0; cycle < 3; cycle++ {
				m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids, true, nil)
				total := 0
				for _, count := range reads {
					total += count
				}
				require.Equal(t, (cycle+1)*maxVzdumpJobLogFetchesPerPoll, total, "unchanged request cap")
				// New provider ordering cannot reset progress or bind it to an index.
				slices.Reverse(jobs)
			}
			for _, job := range jobs {
				require.Contains(t, reads, job.ID, "every stable-backlog job must be attempted")
				require.LessOrEqual(t, reads[job.ID], 2, "least-attempted reads precede another retry")
				require.False(t, m.vzdumpJobTaskCache["site|"+job.ID].final)
			}
			t.Logf("BUDGET_%s jobs=%d cycles=3 requests=75 min=1 max=2", mode, len(jobs))
		})
	}
}

func TestVzdumpLogBudgetAdvancesAfterChildCancellation(t *testing.T) {
	jobs, upids := budgetLogJobs(3)
	m := &Monitor{}
	var attempted []string
	for cycle := 0; cycle < len(jobs); cycle++ {
		ctx, cancel := context.WithCancel(context.Background())
		client := &budgetLogClient{read: func(ctx context.Context, upid string) ([]proxmox.TaskLogLine, error) {
			attempted = append(attempted, upid)
			cancel() // Deterministic child-budget exhaustion; no sleep/clock change.
			return nil, ctx.Err()
		}}
		m.synthesizeVzdumpJobGuestTasks(ctx, "site", client, jobs, upids, true, nil)
		cancel()
	}
	require.Equal(t, []string{"job-0", "job-1", "job-2"}, attempted)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.synthesizeVzdumpJobGuestTasks(ctx, "site", nil, jobs, upids, true, nil)
	require.Len(t, attempted, 3, "cancelled-before-read cannot consume a slot")
}

func TestVzdumpLogBudgetPreservesEvidenceAndScope(t *testing.T) {
	jobs, upids := budgetLogJobs(4)
	jobs[0].Node = "offline"
	old := time.Unix(1792371600, 0).UTC()
	retained := models.BackupTask{ID: "retained", VMID: 100, Status: "error", Error: "original failure", ObservedAt: old}
	entry := vzdumpJobTaskCacheEntry{node: "node", tasks: []models.BackupTask{retained}, lastAttempt: old}
	m := &Monitor{vzdumpJobTaskCache: map[string]vzdumpJobTaskCacheEntry{
		"site|job-0":  {node: "offline", tasks: []models.BackupTask{retained}},
		"site|job-1":  {node: "node", tasks: []models.BackupTask{retained}, final: true, historyComplete: true},
		"site|job-2":  entry,
		"other|job-2": entry,
		"site|gone":   {node: "node"},
	}}
	var reads []string
	client := &budgetLogClient{read: func(_ context.Context, upid string) ([]proxmox.TaskLogLine, error) {
		reads = append(reads, upid)
		return nil, fmt.Errorf("fixture unavailable")
	}}
	got := m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, jobs, upids, true, map[string]bool{"offline": true})
	require.Equal(t, []string{"job-3", "job-2"}, reads, "never attempted precedes a retry; offline/final skip reads")
	require.Len(t, got, 2)
	require.True(t, got[0].ObservedAt.After(old), "immutable evidence retains ordinary refresh semantics")
	require.Equal(t, retained, got[1], "failed read keeps status, detail and observation time in provider order")
	require.Equal(t, entry, m.vzdumpJobTaskCache["other|job-2"])
	require.Equal(t, old, m.vzdumpJobTaskCache["site|job-0"].tasks[0].ObservedAt)
	require.NotContains(t, m.vzdumpJobTaskCache, "site|gone")
	require.Empty(t, m.vzdumpJobTaskCache["site|job-3"].tasks, "a failed first attempt cannot mint guest evidence")
	require.False(t, m.vzdumpJobTaskCache["site|job-3"].historyComplete)
	// Empty complete inventory prunes retry metadata along with ordinary cache.
	m.synthesizeVzdumpJobGuestTasks(context.Background(), "site", client, nil, nil, true, nil)
	require.Len(t, m.vzdumpJobTaskCache, 1)
	require.Contains(t, m.vzdumpJobTaskCache, "other|job-2")
}

// Real read-only provider HTTP -> ordinary poll/state/cache -> report reducer.
// No guest/QGA, backup operation, persistent store or installed host is used.
func TestVzdumpLogBudgetHTTPBacklogRecovery(t *testing.T) {
	var phase, logReads atomic.Int32
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case r.URL.Path == "/api2/json/nodes/node/tasks":
			tasks := []proxmox.Task{}
			for i := 0; i < maxVzdumpJobLogFetchesPerPoll+2; i++ {
				tasks = append(tasks, proxmox.Task{UPID: fmt.Sprintf("job-%d", i), Node: "node", Type: "vzdump", Status: "job errors", StartTime: start.Add(-time.Duration(i) * time.Minute).Unix(), EndTime: start.Add(time.Minute).Unix()})
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": tasks}))
		case strings.HasSuffix(r.URL.Path, "/log"):
			logReads.Add(1)
			parts := strings.Split(r.URL.Path, "/")
			index, err := strconv.Atoi(strings.TrimPrefix(parts[len(parts)-2], "job-"))
			require.NoError(t, err)
			if index < maxVzdumpJobLogFetchesPerPoll && phase.Load() < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			vmid := 500 + index
			if index == 0 {
				vmid = 300
			} else if index == maxVzdumpJobLogFetchesPerPoll {
				vmid = 100
			} else if index == maxVzdumpJobLogFetchesPerPoll+1 {
				vmid = 200
			}
			lines := []string{fmt.Sprintf("INFO: Starting Backup of VM %d (qemu)", vmid)}
			if vmid == 200 {
				lines = append(lines, "ERROR: Backup of VM 200 failed - fixture storage unavailable", "TASK ERROR: job errors")
			} else {
				lines = append(lines, fmt.Sprintf("INFO: Finished Backup of VM %d (00:01:00)", vmid), "TASK OK")
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": testLogLines(lines...)}))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	require.NoError(t, err)
	m := &Monitor{state: models.NewState()}
	for p, label := range []string{"deferred", "discovered", "retained", "recovered", "cached"} {
		phase.Store(int32(p))
		before := logReads.Load()
		m.pollBackupTasks(context.Background(), "site", client)
		snapshot := m.state.GetSnapshot().PVEBackups
		byID := map[int]models.BackupTask{}
		for _, task := range snapshot.BackupTasks {
			byID[task.VMID] = task
		}
		wantReads := int32(maxVzdumpJobLogFetchesPerPoll)
		if p == 4 {
			wantReads = 0
		}
		require.Equal(t, wantReads, logReads.Load()-before)
		if p == 0 {
			require.NotContains(t, byID, 100)
			require.NotContains(t, byID, 200)
		} else {
			require.Equal(t, "OK", byID[100].Status)
			require.Equal(t, "error", byID[200].Status)
			require.Equal(t, "fixture storage unavailable", byID[200].Error)
		}
		if p < 3 {
			require.NotContains(t, byID, 300)
		} else {
			require.Equal(t, "OK", byID[300].Status)
		}
		points, err := proxmoxmapper.FromPVEBackupTasksWithEvidence(snapshot.BackupTasks, nil, time.Now().UTC())
		require.NoError(t, err)
		for _, point := range points {
			require.Equal(t, recovery.KindOther, point.Kind, "task result is not a restore artifact")
		}
		summary := recovery.BuildBackupHealth([]recoverymodel.BackupHealthWorkload{{ID: "deferred-success", Instance: "site", VMID: 100}, {ID: "deferred-failure", Instance: "site", VMID: 200}, {ID: "unread-log", Instance: "site", VMID: 300}}, points, recoverymodel.BackupHistoryCoverage{}, start.Add(-time.Hour), start.Add(time.Hour), time.UTC)
		if p == 0 {
			require.Equal(t, 3, summary.Counts.Unknown)
		} else {
			require.Equal(t, 1, summary.Counts.Failed)
			if p < 3 {
				require.Equal(t, 1, summary.Counts.Unknown)
				require.Equal(t, 1, summary.Counts.Success)
			} else {
				require.Zero(t, summary.Counts.Unknown)
				require.Equal(t, 2, summary.Counts.Success)
			}
		}
		wire, err := json.Marshal(map[string]any{"data": snapshot})
		require.NoError(t, err)
		t.Logf("PVE_BUDGET_PROJECTION_%s=%s", label, wire)
		t.Logf("HTTP_BUDGET_%s requests=%d tasks=%d success=%d failed=%d unknown=%d", label, logReads.Load()-before, len(snapshot.BackupTasks), summary.Counts.Success, summary.Counts.Failed, summary.Counts.Unknown)
	}
}
