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

func TestVzdumpGuestLogMarkersRequireOneMatchingSection(t *testing.T) {
	start := "INFO: Starting Backup of VM 100 (qemu)"
	finish := "INFO: Finished Backup of VM 100 (00:01:00)"
	fail := "ERROR: Backup of VM 100 failed - fixture failure"
	at := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, status string
		attributed   bool
		lines        []string
	}{
		{"ordinary", "OK", true, []string{start, finish}},
		{"legacy-start", "OK", true, []string{"INFO: Starting Backup of VM 100", finish}},
		{"orphan-finish", "unknown", false, []string{finish}},
		{"foreign-finish", "unknown", false, []string{"INFO: Starting Backup of VM 999 (lxc)", finish}},
		{"repeated-start", "unknown", false, []string{start, start, finish}},
		{"repeated-finish", "unknown", false, []string{start, finish, finish}},
		{"second-attempt", "unknown", false, []string{start, finish, start, finish}},
		{"reversed", "unknown", false, []string{finish, start, finish}},
		{"unfinished-overlap", "unknown", false, []string{start}},
		{"explicit-failure", "error", true, []string{start, fail}},
		{"orphan-failure", "error", false, []string{fail}},
		{"failure-then-finish", "error", false, []string{start, fail, finish}},
		{"failure-then-retry", "error", false, []string{start, fail, start, finish}},
		{"failure-after-ambiguity", "error", false, []string{start, start, fail}},
		{"invalid-start-suffix", "unknown", false, []string{"INFO: Starting Backup of VM 100junk", finish}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := models.BackupTask{Instance: "site", Node: "pve1", Type: "vzdump", StartTime: at, EndTime: at.Add(3 * time.Minute)}
			// An independent, well-formed sibling retains its own successful result.
			lines := append(append([]string(nil), tc.lines...), "INFO: Starting Backup of VM 200 (lxc)", "INFO: Finished Backup of VM 200 (00:01:00)", "TASK OK")
			tasks, attributed := parseVzdumpJobLog(job, "job", testLogLines(lines...))
			require.Equal(t, tc.attributed, attributed)
			byID := map[int]models.BackupTask{}
			for _, task := range tasks {
				byID[task.VMID] = task
			}
			require.Equal(t, tc.status, byID[100].Status)
			require.Equal(t, "OK", byID[200].Status)
			if tc.status == "unknown" {
				require.Contains(t, byID[100].Error, "task log markers do not match")
			}
			if strings.Contains(tc.name, "failure") {
				require.Equal(t, "fixture failure", byID[100].Error)
			}
		})
	}
}

// Ordinary read-only HTTP -> polling -> cache -> existing reporting reducers.
// No native guest/QGA/backup, database, installed recovery or clock change.
func TestPVEGuestLogAttributionRecoversWithoutSealingAmbiguity(t *testing.T) {
	var phase, logReads atomic.Int32
	start := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"}]}`)
		case r.URL.Path == "/api2/json/nodes/pve1/tasks":
			fmt.Fprintf(w, `{"data":[{"upid":"attribution-job","node":"pve1","type":"vzdump","status":"job errors","starttime":%d,"endtime":%d}]}`, start.Unix(), start.Add(3*time.Minute).Unix())
		case strings.HasSuffix(r.URL.Path, "/log"):
			logReads.Add(1)
			if phase.Load() == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			lines := []string{}
			if phase.Load() >= 2 {
				lines = append(lines, "INFO: Starting Backup of VM 100 (qemu)")
			}
			lines = append(lines, "INFO: Finished Backup of VM 100 (00:01:00)", "INFO: Starting Backup of VM 200 (lxc)", "INFO: Finished Backup of VM 200 (00:01:00)", "INFO: Starting Backup of VM 300 (qemu)", "ERROR: Backup of VM 300 failed - fixture storage unavailable", "TASK ERROR: job errors")
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
	var firstObservation time.Time
	for _, step := range []struct {
		phase        int32
		name, status string
		reads        int32
		final        bool
	}{
		{0, "unmatched", "unknown", 1, false}, {1, "unavailable", "unknown", 2, false}, {2, "corrected", "OK", 3, true}, {3, "cached", "OK", 3, true},
	} {
		phase.Store(step.phase)
		m.pollBackupTasks(context.Background(), "site", client)
		snapshot := m.state.GetSnapshot().PVEBackups
		require.Len(t, snapshot.BackupTasks, 4)
		byID := map[int]models.BackupTask{}
		for _, task := range snapshot.BackupTasks {
			byID[task.VMID] = task
		}
		require.Equal(t, step.status, byID[100].Status)
		require.Equal(t, "OK", byID[200].Status)
		require.Equal(t, "error", byID[300].Status, "explicit sibling failure survives every phase")
		require.Equal(t, "fixture storage unavailable", byID[300].Error)
		require.Equal(t, step.reads, logReads.Load())
		entry := m.vzdumpJobTaskCache["site|attribution-job"]
		require.Equal(t, step.final, entry.final, "ambiguous logs cannot become immutable")
		require.Equal(t, step.final, entry.historyComplete)
		if step.phase == 0 {
			firstObservation = byID[100].ObservedAt
		}
		if step.phase == 1 {
			require.Equal(t, firstObservation, byID[100].ObservedAt, "failure cannot renew a prior observation")
		}
		if step.phase >= 2 {
			require.Empty(t, byID[100].Error, "ordinary corrected read clears uncertainty")
		}
		points, err := proxmoxmapper.FromPVEBackupTasksWithEvidence(snapshot.BackupTasks, nil, time.Now().UTC())
		require.NoError(t, err)
		parent := byID[0]
		m.qualifyBackupGuestTaskDetails("site", []models.BackupTask{parent}, map[string]string{parent.ID: "attribution-job"}, points)
		for _, p := range points {
			require.Equal(t, recovery.KindOther, p.Kind, "task cannot mint a restore artifact")
		}
		summary := recovery.BuildBackupHealth([]recoverymodel.BackupHealthWorkload{{ID: "vm100", Instance: "site", VMID: 100}, {ID: "ct200", Instance: "site", VMID: 200}, {ID: "vm300", Instance: "site", VMID: 300}}, points, recoverymodel.BackupHistoryCoverage{}, start.Add(-time.Hour), start.Add(time.Hour), time.UTC)
		require.Equal(t, 1, summary.Counts.Failed)
		if !step.final {
			require.Equal(t, 1, summary.Counts.Unknown)
			require.Equal(t, 1, summary.Counts.Success)
		} else {
			require.Zero(t, summary.Counts.Unknown)
			require.Equal(t, 2, summary.Counts.Success)
		}
		for _, w := range summary.Workloads {
			if w.ID == "vm300" || (w.ID == "vm100" && !step.final) {
				require.Nil(t, w.LastSuccessAt)
			} else {
				require.NotNil(t, w.LastSuccessAt)
			}
			if !step.final {
				require.Equal(t, "partial", w.HistoryState)
			}
		}
		wire, err := json.Marshal(map[string]any{"data": snapshot})
		require.NoError(t, err)
		t.Logf("PVE_ATTRIBUTION_PROJECTION_%s=%s", step.name, wire)
	}
}
