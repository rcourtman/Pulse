package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestVzdumpGuestWarningsDoNotBecomeSuccess(t *testing.T) {
	start := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, warning string
	}{
		{"warn", "WARN: synthetic-private-guest-detail"},
		{"warning", "WARNING: synthetic-private-guest-detail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, finished := range []bool{false, true} {
				job := models.BackupTask{Instance: "site", Node: "pve1", Type: "vzdump", StartTime: start}
				lines := []string{"INFO: Starting Backup of VM 100 (qemu)", tc.warning}
				if finished {
					job.EndTime = start.Add(2 * time.Minute)
					lines = append(lines, "INFO: Finished Backup of VM 100 (00:01:00)", "INFO: Starting Backup of VM 200 (lxc)", "INFO: Finished Backup of VM 200 (00:01:00)", "TASK WARNINGS: 1")
				}
				tasks := parseVzdumpJobLog(job, "job", testLogLines(lines...))
				if !finished {
					require.Len(t, tasks, 1)
					require.Equal(t, "running", tasks[0].Status, "a warning cannot finish a running guest backup")
					require.True(t, tasks[0].EndTime.IsZero())
					continue
				}
				require.Len(t, tasks, 2)
				require.Equal(t, "warning", tasks[0].Status, "Finished Backup must not erase this guest's warning")
				require.NotEmpty(t, tasks[0].Error, "the existing detail must direct the operator to the original log")
				require.NotContains(t, tasks[0].Error, "synthetic-private-guest-detail", "warning text is not copied to API/report diagnostics")
				require.Equal(t, "OK", tasks[1].Status, "a sibling guest does not inherit another guest's warning")
				require.Empty(t, tasks[1].Error)
			}
		})
	}
}

func TestVzdumpGuestWarningScopeAndFailurePrecedence(t *testing.T) {
	start := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	job := models.BackupTask{Instance: "site", Node: "pve1", Type: "vzdump", StartTime: start, EndTime: start.Add(time.Minute)}
	for _, tc := range []struct {
		name, status, detail string
		lines                []string
	}{
		{"outside-sections", "OK", "", []string{"WARN: job-level warning", "INFO: Starting Backup of VM 100 (qemu)", "INFO: Finished Backup of VM 100 (00:01:00)", "WARNING: job-level warning", "TASK WARNINGS: 2"}},
		{"quoted-info", "OK", "", []string{"INFO: Starting Backup of VM 100 (qemu)", "INFO: guest name is WARN: text", "INFO: Finished Backup of VM 100 (00:01:00)", "TASK OK"}},
		{"failure-after-warning", "error", "fixture storage unavailable", []string{"INFO: Starting Backup of VM 100 (qemu)", "WARN: log detail", "ERROR: Backup of VM 100 failed - fixture storage unavailable", "TASK ERROR: job errors"}},
		{"warning-after-failure", "error", "fixture storage unavailable", []string{"INFO: Starting Backup of VM 100 (qemu)", "ERROR: Backup of VM 100 failed - fixture storage unavailable", "WARN: log detail", "TASK ERROR: job errors"}},
		{"unfinished-with-warning", "error", "backup job ended before guest backup completed", []string{"INFO: Starting Backup of VM 100 (qemu)", "WARN: log detail", "TASK WARNINGS: 1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tasks := parseVzdumpJobLog(job, "job", testLogLines(tc.lines...))
			require.Len(t, tasks, 1)
			require.Equal(t, tc.status, tasks[0].Status)
			require.Equal(t, tc.detail, tasks[0].Error)
		})
	}
}

// Real read-only HTTP -> ordinary polling -> immutable guest-log cache -> live
// payload and existing recovery/report reducers. No guest/QGA, backup operation,
// database, native host or installed recovery is exercised.
func TestPVEGuestWarningSurvivesPollingCacheAndReports(t *testing.T) {
	var phase, logReads atomic.Int32
	start := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"}]}`)
		case r.URL.Path == "/api2/json/nodes/pve1/tasks":
			upid, status, end := "warn-job", "", int64(0)
			if phase.Load() > 0 {
				status, end = "WARNINGS: 1", start.Add(2*time.Minute).Unix()
			}
			if phase.Load() == 3 {
				upid, status = "next-job", "OK"
			}
			fmt.Fprintf(w, `{"data":[{"upid":%q,"node":"pve1","type":"vzdump","status":%q,"starttime":%d,"endtime":%d}]}`, upid, status, start.Unix(), end)
		case strings.HasSuffix(r.URL.Path, "/log"):
			logReads.Add(1)
			lines := []string{"WARN: job-wide warning is not a guest result", "INFO: Starting Backup of VM 100 (qemu)"}
			if phase.Load() < 3 {
				lines = append(lines, "WARN: synthetic-private-guest-detail /path <img src=unexpected>")
			}
			if phase.Load() > 0 {
				lines = append(lines, "INFO: Finished Backup of VM 100 (00:01:00)", "INFO: Starting Backup of VM 200 (lxc)", "INFO: Finished Backup of VM 200 (00:01:00)", "TASK OK")
				if phase.Load() < 3 {
					lines[len(lines)-1] = "TASK WARNINGS: 1"
				}
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": testLogLines(lines...)}); err != nil {
				t.Errorf("write fixture: %v", err)
			}
		default:
			t.Errorf("unexpected operation %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	require.NoError(t, err)
	m := &Monitor{state: models.NewState()}
	for _, step := range []struct {
		phase              int32
		name, wantStatus   string
		wantReads, wantLen int
	}{
		{0, "running", "running", 1, 2},
		{1, "warning", "warning", 2, 3},
		{2, "cached", "warning", 2, 3},
		{3, "next-clean-job", "OK", 3, 3},
	} {
		phase.Store(step.phase)
		m.pollBackupTasks(context.Background(), "site", client)
		snapshot := m.state.GetSnapshot().PVEBackups
		require.Len(t, snapshot.BackupTasks, step.wantLen, step.name)
		require.Equal(t, int32(step.wantReads), logReads.Load(), step.name)
		byVMID := make(map[int]models.BackupTask)
		for _, task := range snapshot.BackupTasks {
			byVMID[task.VMID] = task
		}
		require.Equal(t, step.wantStatus, byVMID[100].Status, step.name)
		if step.phase > 0 {
			require.Equal(t, "OK", byVMID[200].Status, "clean sibling")
			points, err := proxmoxmapper.FromPVEBackupTasksWithEvidence(snapshot.BackupTasks, nil, time.Now().UTC())
			require.NoError(t, err)
			parent := byVMID[0]
			upid := "warn-job"
			if step.phase == 3 {
				upid = "next-job"
			}
			m.qualifyBackupGuestTaskDetails("site", []models.BackupTask{parent}, map[string]string{parent.ID: upid}, points)
			for _, point := range points {
				require.Equal(t, recovery.KindOther, point.Kind, "tasks cannot mint restore artifacts")
			}
			workloads := []recoverymodel.BackupHealthWorkload{{ID: "vm100", Instance: "site", VMID: 100}, {ID: "ct200", Instance: "site", VMID: 200}}
			summary := recovery.BuildBackupHealth(workloads, points, recoverymodel.BackupHistoryCoverage{}, start.Add(-time.Hour), start.Add(time.Hour), time.UTC)
			if step.phase < 3 {
				require.Equal(t, 1, summary.Counts.Warning, "the existing report must not count the warned guest as successful")
				require.Equal(t, 1, summary.Counts.Success)
				require.Len(t, summary.Failures, 1)
				require.Equal(t, "vm100", summary.Failures[0].WorkloadID)
			} else {
				require.Zero(t, summary.Counts.Warning)
				require.Equal(t, 2, summary.Counts.Success)
				require.Empty(t, summary.Failures)
			}
			for _, workload := range summary.Workloads {
				if workload.ID == "vm100" && step.phase < 3 {
					require.Nil(t, workload.LastSuccessAt, "a warning must not advance last-success")
				} else {
					require.NotNil(t, workload.LastSuccessAt)
				}
			}
		}
		wire, err := json.Marshal(map[string]any{"data": snapshot})
		require.NoError(t, err)
		require.NotContains(t, string(wire), "synthetic-private-guest-detail")
		t.Logf("PVE_GUEST_WARNING_PROJECTION_%s=%s", step.name, wire)
	}
}
