package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
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

func (c *historyPVEClient) GetBackupTasks(context.Context) ([]proxmox.Task, error) {
	return c.history.Tasks, nil
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

// Report backfill is a merge of observed executions, not replacement inventory.
// Drive both production HTTP readers and ordinary polling/store consumers. No
// guest, backup, QGA, system clock or installed profile is operated on.
func TestBackupHistoryCollectionCannotReplaceLiveInventory(t *testing.T) {
	var phase atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := phase.Load()
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve-a","status":"online"},{"node":"pve-b","status":"online"}]}`)
		case r.URL.Path == "/api2/json/access/permissions":
			audit := 1
			if p == 4 {
				audit = 0
			}
			fmt.Fprintf(w, `{"data":{%q:{"Sys.Audit":%d}}}`, r.URL.Query().Get("path"), audit)
		case strings.HasSuffix(r.URL.Path, "/tasks"):
			history := r.URL.Query().Has("since")
			if p == 1 && strings.Contains(r.URL.Path, "/pve-a/") || p == 3 && history && strings.Contains(r.URL.Path, "/pve-b/") {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if p == 2 && strings.Contains(r.URL.Path, "/pve-b/") {
				w.Header().Set("Content-Length", "1000")
				fmt.Fprint(w, `{"data":[]}`)
				return
			}
			if p == 6 || p == 5 && history {
				fmt.Fprint(w, `{"data":[]}`)
				return
			}
			node := "pve-a"
			if strings.Contains(r.URL.Path, "/pve-b/") {
				node = "pve-b"
			}
			fmt.Fprintf(w, `{"data":[{"upid":%q,"node":%q,"type":"vzdump","status":"OK","starttime":1792256400,"endtime":1792256460}]}`, "job-"+node, node)
		case strings.HasSuffix(r.URL.Path, "/log"):
			vmid := 101
			if strings.Contains(r.URL.Path, "/pve-b/") {
				vmid = 202
			}
			fmt.Fprintf(w, `{"data":[{"n":1,"t":%q},{"n":2,"t":%q},{"n":3,"t":"TASK OK"}]}`, fmt.Sprintf("INFO: Starting Backup of VM %d (qemu)", vmid), fmt.Sprintf("INFO: Finished Backup of VM %d (00:01:00)", vmid))
		default:
			t.Errorf("unexpected operation %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	require.NoError(t, err)
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	m := &Monitor{state: models.NewState(), recoveryManager: manager}
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	var initial models.PVEBackups
	for _, step := range []struct {
		phase    int32
		label    string
		complete bool
	}{
		{0, "initial", true}, {1, "node-unavailable", false}, {2, "body-incomplete", false},
		{3, "history-unavailable", false}, {4, "audit-unconfirmed", false},
		{5, "history-window-empty", true}, {0, "resumed", true}, {6, "live-empty", true},
	} {
		phase.Store(step.phase)
		m.pollBackupTasks(context.Background(), "fixture-pve", client)
		snapshot := m.state.GetSnapshot().PVEBackups
		if step.label == "initial" {
			initial = snapshot
		}
		if step.phase == 1 || step.phase == 2 {
			require.True(t, reflect.DeepEqual(initial, snapshot), "%s replaced unreadable live rows or original receipts", step.label)
		}
		if step.phase == 6 {
			require.Empty(t, snapshot.BackupTasks, "only a complete live empty read may clear live rows")
			require.Empty(t, m.vzdumpJobTaskCache)
		} else {
			require.Len(t, snapshot.BackupTasks, 4, step.label)
			require.Len(t, m.vzdumpJobTaskCache, 2, "partial history must not prune an unreadable live job's cache")
		}
		coverage, err := store.BackupHistoryCoverage(context.Background(), time.Now().Add(-time.Hour), time.Now())
		require.NoError(t, err)
		require.Len(t, coverage.Sources, 1)
		require.Equal(t, step.complete, coverage.Sources[0].Complete, step.label)
		points, total, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Kind: recovery.KindOther})
		require.NoError(t, err)
		require.Equal(t, 4, total, "incomplete or empty inventory must not erase observed executions")
		for _, point := range points {
			if strings.HasSuffix(point.ID, "job-pve-a") || strings.HasSuffix(point.ID, "job-pve-b") {
				require.Equal(t, true, point.Details["guestTaskDetailsComplete"], "the independent terminal-log requirement remains satisfied")
			}
		}
		wire, err := json.Marshal(map[string]any{"data": snapshot})
		require.NoError(t, err)
		t.Logf("HISTORY_LIVE_SEQUENCE_%s=%s", step.label, wire)
	}
}
