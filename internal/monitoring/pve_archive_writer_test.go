package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the real task, log and storage clients, then live dates and durable
// protection. No provider write is needed and the live window stays at 200.
func TestPVEArchiveWritersOutsideDisplayWindow(t *testing.T) {
	for _, kind := range []string{"direct", "scheduled"} {
		t.Run(kind, func(t *testing.T) { testPVEArchiveWritersOutsideDisplayWindow(t, kind == "scheduled") })
	}
}

func testPVEArchiveWritersOutsideDisplayWindow(t *testing.T, scheduled bool) {
	t.Helper()
	f := newPVEReconciliationFixture(t)
	now := time.Now().UTC().Truncate(24 * time.Hour)
	startA := now.Add(-2 * time.Hour)
	startB := now.Add(-75 * time.Minute)
	archive := func(guest int, at time.Time) proxmox.StorageContent {
		return proxmox.StorageContent{
			Volid: fmt.Sprintf("archive:backup/vzdump-qemu-%d-%s.vma.zst", guest, at.UTC().Format("2006_01_02-15_04_05")),
			VMID:  guest, CTime: at.Unix(), Size: 2048, Content: "backup", Format: "vma.zst",
		}
	}
	older := archive(102, now.Add(-8*24*time.Hour))
	peer := archive(101, now.Add(-25*time.Hour))
	current := archive(102, startA.Add(time.Minute))
	unrelated := archive(102, startB.Add(time.Minute))
	independent := archive(102, startB.Add(20*time.Minute))
	writer := proxmox.Task{UPID: "writer-a", Node: "node-b", Type: "vzdump", ID: "102", Status: "running", StartTime: startA.Unix()}
	if scheduled {
		writer.ID = ""
	}
	success := proxmox.Task{UPID: "job-b", Node: "node-b", Type: "vzdump", ID: "102",
		Status: "OK", StartTime: startB.Unix(), EndTime: startB.Add(10 * time.Minute).Unix()}
	page := make([]proxmox.Task, 200)
	wantIDs := make([]string, len(page))
	for i := range page {
		at := now.Add(-time.Duration(i+1) * time.Second)
		page[i] = proxmox.Task{UPID: fmt.Sprintf("recent-%03d", i), Node: "node-b", Type: "vzdump", ID: "999",
			Status: "OK", StartTime: at.Add(-time.Second).Unix(), EndTime: at.Unix()}
		wantIDs[i] = "cluster-" + page[i].UPID
	}

	const (
		writing = iota
		unreadPage
		emptyLog
		soleFile
		unreadLog
		sameWriter
		otherGuestLog
		guestFinished
		ended
	)
	var phase atomic.Int32
	var mu sync.Mutex
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		key := path
		switch path {
		case "/api2/json/access/permissions":
			key += ":" + r.URL.Query().Get("path")
		case "/api2/json/nodes/node-b/tasks":
			key += ":" + r.URL.Query().Get("start")
		}
		mu.Lock()
		requests[key]++
		mu.Unlock()
		assert.Equal(t, http.MethodGet, r.Method)
		p := phase.Load()
		var data any
		switch path {
		case "/api2/json/nodes":
			data = []proxmox.Node{{Node: "node-a", Status: "online"}, {Node: "node-b", Status: "online"}}
		case "/api2/json/access/permissions":
			data = map[string]any{r.URL.Query().Get("path"): map[string]int{"Sys.Audit": 1}}
		case "/api2/json/nodes/node-a/tasks":
			data = []proxmox.Task{}
		case "/api2/json/nodes/node-b/tasks":
			switch r.URL.Query().Get("start") {
			case "0":
				data = page
			case "200":
				switch {
				case p == unreadPage:
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				case p == soleFile || p == unreadLog && !scheduled:
					// B's native OK is accepted, but overlap leaves the
					// listing incomplete. B's real output is not listed.
					data = []proxmox.Task{page[len(page)-1], success}
				case p == ended:
					done := writer
					done.Status, done.EndTime = "OK", now.Add(-time.Minute).Unix()
					data = []proxmox.Task{done}
				default:
					data = []proxmox.Task{writer}
				}
			default:
				t.Errorf("unexpected task offset: %s", r.URL.Query().Get("start"))
				http.NotFound(w, r)
				return
			}
		case "/api2/json/nodes/node-b/tasks/writer-a/log":
			switch p {
			case unreadLog:
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			case emptyLog:
				// A successful, explicitly empty response is accepted for
				// live detail, but cannot resolve the known archive writer.
				data = []proxmox.TaskLogLine{}
			case otherGuestLog:
				// Real parsed rows still don't settle a different guest.
				data = testLogLines("INFO: Starting Backup of VM 103 (qemu)", "INFO: Finished Backup of VM 103 (00:02:00)")
			default:
				lines := testLogLines("INFO: Starting Backup of VM 102 (qemu)")
				if p == guestFinished || p == ended {
					lines = append(lines, proxmox.TaskLogLine{LineNumber: 2, Text: "INFO: Finished Backup of VM 102 (01:59:00)"})
				}
				if p == ended {
					lines = append(lines, proxmox.TaskLogLine{LineNumber: 3, Text: "TASK OK"})
				}
				data = lines
			}
		case "/api2/json/nodes/node-a/storage", "/api2/json/nodes/node-b/storage":
			// Equal storage names do not make these independent disks shared.
			data = []proxmox.Storage{{Storage: "archive", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}
		case "/api2/json/nodes/node-a/storage/archive/content":
			files := []proxmox.StorageContent{peer}
			if p >= unreadLog {
				files = append(files, independent)
			}
			data = files
		case "/api2/json/nodes/node-b/storage/archive/content":
			files := []proxmox.StorageContent{older, current}
			if p >= soleFile && p < guestFinished {
				files = append(files, unrelated)
			}
			data = files
		case "/api2/json/nodes/node-a/qemu/101/snapshot", "/api2/json/nodes/node-b/qemu/102/snapshot":
			data = []proxmox.Snapshot{}
		default:
			t.Errorf("unexpected fixture path %s", path)
			http.NotFound(w, r)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
	}))
	t.Cleanup(server.Close)
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
	require.NoError(t, err)
	phases := [...]string{
		writing: "writing", unreadPage: "unread-page", emptyLog: "empty-running-log",
		soleFile: "sole-unrelated", unreadLog: "unread-log", sameWriter: "same-running-log",
		otherGuestLog: "other-guest-running-log", guestFinished: "guest-finished-parent-running", ended: "completed",
	}
	var previousGuest recovery.RecoveryPoint
	polls := 0
	for i, name := range phases {
		if !scheduled && (i == emptyLog || i == otherGuestLog || i == guestFinished) {
			continue
		}
		polls++
		phase.Store(int32(i))
		f.m.pollPVEBackupsAndSnapshots(context.Background(), "cluster", client, f.nodes, f.online, 10*time.Second)
		f.wait(t)
		snapshot := f.m.state.GetSnapshot()
		ids := make([]string, 0, len(snapshot.PVEBackups.BackupTasks))
		for _, task := range snapshot.PVEBackups.BackupTasks {
			ids = append(ids, task.ID)
		}
		require.Equal(t, wantIDs, ids, "%s: display window", name)
		points, postures := f.points(t), f.postures(t)
		wantRows := map[string]bool{older.Volid: false, peer.Volid: false, current.Volid: i < guestFinished}
		if i >= soleFile && i < guestFinished {
			wantRows[unrelated.Volid] = true
		}
		if i >= unreadLog {
			wantRows[independent.Volid] = false
		}
		require.Len(t, snapshot.PVEBackups.StorageBackups, len(wantRows))
		for _, row := range snapshot.PVEBackups.StorageBackups {
			incomplete, ok := wantRows[row.Volid]
			require.True(t, ok, "%s: unexpected file %s", name, row.Volid)
			delete(wantRows, row.Volid)
			point, exists := points["pve-backup:"+row.ID]
			require.True(t, exists, "%s: native file stays inspectable", name)
			assert.Equal(t, incomplete, row.InProgress, "%s: archive classification %s", name, row.Volid)
			if incomplete {
				assert.Equal(t, recovery.OutcomeRunning, point.Outcome, "%s: %s", name, row.Volid)
				assert.Nil(t, point.CompletedAt, "%s: %s may not certify protection", name, row.Volid)
			} else {
				archiveTime := time.Unix(row.CTime, 0).UTC()
				assert.Equal(t, recovery.OutcomeSuccess, point.Outcome, "%s: eligible file %s", name, row.Volid)
				assert.Equal(t, &archiveTime, point.CompletedAt)
			}
			if row.Volid == independent.Volid {
				assert.Equal(t, "node-a", row.StorageNode)
				assert.Equal(t, "node-b", row.Node, "guest placement is not storage provenance")
			}
		}
		require.Empty(t, wantRows)
		latest := older.CTime
		if i >= unreadLog {
			latest = independent.CTime
		}
		for _, vm := range snapshot.VMs {
			want := peer.CTime
			if vm.VMID == 102 {
				want = latest
			}
			assert.Equal(t, want, vm.LastBackup.Unix(), "%s: %s last backup", name, vm.Name)
		}
		for _, posture := range postures {
			want := peer.CTime
			if posture.SubjectResourceID == f.ids[1] {
				want = latest
			}
			require.NotNil(t, posture.LastSuccessfulPointAt, "%s: independent protection was lost", name)
			assert.Equal(t, want, posture.LastSuccessfulPointAt.Unix(), "%s: %s protection date", name, posture.SubjectResourceID)
		}
		if i >= soleFile {
			assert.Equal(t, recovery.OutcomeSuccess, points["pve-task:cluster-job-b"].Outcome, "%s: B retains its separate execution result", name)
		}
		task := points["pve-task:cluster-writer-a"]
		if i == ended {
			assert.Equal(t, recovery.OutcomeSuccess, task.Outcome)
		} else {
			assert.Equal(t, recovery.OutcomeRunning, task.Outcome)
			assert.Nil(t, task.CompletedAt)
		}
		if scheduled {
			guest := points["pve-task:cluster-writer-a-vm102"]
			if i == emptyLog || i == otherGuestLog {
				assert.Equal(t, previousGuest.Evidence, guest.Evidence, "%s: no new observation for guest 102", name)
			}
			if i == otherGuestLog {
				assert.Equal(t, recovery.OutcomeSuccess, points["pve-task:cluster-writer-a-vm103"].Outcome, name)
			}
			if i == guestFinished || i == ended {
				assert.Equal(t, recovery.OutcomeSuccess, guest.Outcome, name)
				assert.NotNil(t, guest.CompletedAt, name)
			} else {
				assert.Equal(t, recovery.OutcomeRunning, guest.Outcome, name)
				assert.Nil(t, guest.CompletedAt, name)
			}
			previousGuest = guest
		}
	}
	n := polls
	wantRequests := map[string]int{
		"/api2/json/nodes": n,
		"/api2/json/access/permissions:/nodes/node-a": n,
		"/api2/json/access/permissions:/nodes/node-b": n - 3,
		"/api2/json/nodes/node-a/tasks":               n, "/api2/json/nodes/node-b/tasks:0": n, "/api2/json/nodes/node-b/tasks:200": n,
		"/api2/json/nodes/node-a/storage": n, "/api2/json/nodes/node-b/storage": n,
		"/api2/json/nodes/node-a/storage/archive/content": n, "/api2/json/nodes/node-b/storage/archive/content": n,
		"/api2/json/nodes/node-a/qemu/101/snapshot": n, "/api2/json/nodes/node-b/qemu/102/snapshot": n,
	}
	if scheduled {
		wantRequests["/api2/json/access/permissions:/nodes/node-b"]++
		wantRequests["/api2/json/nodes/node-b/tasks/writer-a/log"] = n - 2
	}
	mu.Lock()
	assert.Equal(t, wantRequests, requests, "unchanged task/log/storage/snapshot reads")
	mu.Unlock()
}

func TestPVEArchiveWriterWithdrawal(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	writer := models.BackupTask{ID: "site-job-vm101", Instance: "site", Node: "member",
		VMID: 101, Type: "vzdump", Status: "running", StartTime: start}
	done := writer
	done.Status, done.EndTime = "OK", start.Add(time.Minute)
	parent := models.BackupTask{ID: "site-job", Status: "running", StartTime: start}
	endedParent := parent
	endedParent.Status, endedParent.EndTime = "OK", done.EndTime
	for _, tc := range []struct {
		name         string
		node         proxmox.BackupTaskHistoryNode
		jobs         []models.BackupTask
		resolved     bool
		replace      bool
		wantRetained bool
	}{
		{name: "failed-page", node: proxmox.BackupTaskHistoryNode{Node: "member", Reason: "collection_failed"}, wantRetained: true},
		{name: "own-tasks-only", node: proxmox.BackupTaskHistoryNode{Node: "member", Reason: "audit_permission_unconfirmed"}, wantRetained: true},
		{name: "running-parent-without-guest-result", node: proxmox.BackupTaskHistoryNode{Node: "member", Complete: true}, jobs: []models.BackupTask{parent}, wantRetained: true},
		{name: "running-parent-with-exact-result", node: proxmox.BackupTaskHistoryNode{Node: "member", Reason: "collection_failed"}, jobs: []models.BackupTask{parent}, resolved: true},
		{name: "ended-parent-overrides-incomplete-page", node: proxmox.BackupTaskHistoryNode{Node: "member", Reason: "collection_failed"}, jobs: []models.BackupTask{endedParent}},
		{name: "complete-absence", node: proxmox.BackupTaskHistoryNode{Node: "member", Complete: true}},
		{name: "removed-node", node: proxmox.BackupTaskHistoryNode{Node: "other", Complete: true}},
		{name: "access-denied", node: proxmox.BackupTaskHistoryNode{Node: "member", Reason: "access_denied"}},
		{name: "replaced-connection", node: proxmox.BackupTaskHistoryNode{Node: "member", Reason: "collection_failed"}, replace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Monitor{}
			peer := writer
			peer.ID, peer.Instance = "peer-job-vm101", "peer"
			m.recordPVEArchiveWriters("site", nil, []models.BackupTask{writer}, nil)
			m.recordPVEArchiveWriters("peer", nil, []models.BackupTask{peer}, nil)
			if tc.replace {
				m.replacePVEClient("site", nil)
			}
			var tasks []models.BackupTask
			if tc.resolved {
				tasks = []models.BackupTask{done}
			}
			m.recordPVEArchiveWriters("site", []proxmox.BackupTaskHistoryNode{tc.node}, tasks, tc.jobs)
			// A later missing page must not bring withdrawn evidence back.
			m.recordPVEArchiveWriters("site", []proxmox.BackupTaskHistoryNode{{Node: "member", Reason: "collection_failed"}}, nil, nil)
			wantStarts := map[int]map[string]time.Time{101: {"member": start}}
			if tc.wantRetained {
				require.Equal(t, wantStarts, m.pveArchiveWriterStarts("site"))
			} else {
				require.Empty(t, m.pveArchiveWriterStarts("site"))
			}
			require.Equal(t, wantStarts, m.pveArchiveWriterStarts("peer"))
		})
	}
}
