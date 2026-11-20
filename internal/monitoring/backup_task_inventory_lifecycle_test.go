package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Actual HTTP client -> history collector -> monitor -> owning snapshot.
// Fixtures perform reads only and never start a task or contact a real PVE node.
func TestBackupHistoryUnreadNodeRetainsAcceptedTasksWithoutRenewal(t *testing.T) {
	var phase atomic.Int32
	start := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := phase.Load()
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node-a","status":"online"},{"node":"node-b","status":"online"}]}`)
		case "/api2/json/access/permissions":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{r.URL.Query().Get("path"): map[string]int{"Sys.Audit": 1}}})
		case "/api2/json/nodes/node-a/tasks":
			switch p {
			case 1, 3:
				fmt.Fprint(w, `{"data":null}`)
			case 2:
				fmt.Fprint(w, `{"data":[]}`)
			case 5:
				w.WriteHeader(http.StatusForbidden)
			case 6:
				w.WriteHeader(http.StatusServiceUnavailable)
			default:
				json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{{UPID: "task-a", Node: "node-a", Type: "vzdump", ID: "101", StartTime: start.Unix(), Status: "running"}}})
			}
		case "/api2/json/nodes/node-b/tasks":
			status := "OK"
			if p > 0 {
				status = "ERROR: independent archive unavailable"
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{{UPID: "task-b", Node: "node-b", Type: "vzdump", ID: "102", StartTime: start.Unix(), EndTime: start.Add(time.Minute).Unix(), Status: status}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
	require.NoError(t, err)
	m := &Monitor{state: models.NewState()}
	peer := models.BackupTask{ID: "peer-task", Instance: "independent-site", Node: "node-a", VMID: 101, Type: "vzdump", Status: "OK", StartTime: start, EndTime: start.Add(time.Minute), ObservedAt: start}
	m.state.UpdateBackupTasksForInstance(peer.Instance, []models.BackupTask{peer})
	frames := map[string]any{}
	var accepted models.BackupTask
	for i, name := range []string{"accepted", "unread", "empty", "unread-after-empty", "recovered", "denied", "unavailable-after-denial", "recovered-again"} {
		phase.Store(int32(i))
		m.pollBackupTasks(context.Background(), "site", client)
		snapshot := m.state.GetSnapshot()
		byID := map[string]models.BackupTask{}
		for _, task := range snapshot.PVEBackups.BackupTasks {
			byID[task.ID] = task
		}
		require.Equal(t, peer, byID[peer.ID], "same node and VMID on another connection must remain untouched")
		b := byID["site-task-b"]
		require.Equal(t, "node-b", b.Node)
		require.Equal(t, "site", b.Instance)
		require.Equal(t, 102, b.VMID)
		if i > 0 {
			require.Equal(t, "ERROR: independent archive unavailable", b.Status, "usable independent node evidence must advance")
		}
		a, present := byID["site-task-a"]
		switch i {
		case 0, 4, 7:
			require.True(t, present)
			require.Equal(t, "running", a.Status)
			require.True(t, start.Equal(a.StartTime), "preserve the instant regardless of Go's local-zone representation")
			require.True(t, a.EndTime.IsZero())
			require.False(t, a.ObservedAt.IsZero())
			accepted = a
		case 1:
			require.True(t, present, "unread is not an authoritative empty task inventory")
			require.Equal(t, accepted, a, "carry-forward cannot renew time, completion or scope")
		default:
			require.False(t, present, "valid empty/authority withdrawal cannot resurrect from an old unread cache")
		}
		frames[name] = map[string]any{"pve": map[string]any{"data": snapshot.PVEBackups}}
	}
	if os.Getenv("PULSE_TASK_INVENTORY_FRAMES") == "1" {
		b, err := json.Marshal(frames)
		require.NoError(t, err)
		t.Log("TASK_INVENTORY_FRAMES=" + string(b))
	}
}

func TestBackupHistoryRetentionIsNodeScopedAndFreshEvidenceWins(t *testing.T) {
	old := time.Now().UTC().Add(-time.Hour)
	m := &Monitor{state: models.NewState()}
	m.state.UpdateBackupTasksForInstance("site", []models.BackupTask{
		{ID: "site-running", Instance: "site", Node: "unread", Type: "vzdump", VMID: 101, Status: "running", StartTime: old, ObservedAt: old},
		{ID: "site-retained", Instance: "site", Node: "unread", Type: "vzdump", VMID: 102, Status: "OK", StartTime: old, EndTime: old.Add(time.Minute), ObservedAt: old},
		{ID: "site-offline", Instance: "site", Node: "offline", Type: "vzdump", VMID: 103, Status: "running", StartTime: old, ObservedAt: old},
		{ID: "site-removed", Instance: "site", Node: "removed", Type: "vzdump", VMID: 104, Status: "OK", StartTime: old, ObservedAt: old},
	})
	client := &historyPVEClient{history: proxmox.BackupTaskHistory{
		Nodes: []proxmox.BackupTaskHistoryNode{{Node: "unread", Reason: "collection_failed"}, {Node: "offline", Reason: "node_offline"}, {Node: "empty", Complete: true}},
		Tasks: []proxmox.Task{{UPID: "running", Node: "unread", Type: "vzdump", ID: "101", Status: "OK", StartTime: old.Unix(), EndTime: old.Add(time.Minute).Unix()}},
	}}
	m.pollBackupTasks(context.Background(), "site", client)
	byID := map[string]models.BackupTask{}
	for _, task := range m.state.GetSnapshot().PVEBackups.BackupTasks {
		byID[task.ID] = task
	}
	require.Len(t, byID, 3)
	require.Equal(t, "OK", byID["site-running"].Status, "accepted earlier page wins over retained running state")
	require.True(t, byID["site-running"].ObservedAt.After(old))
	require.Equal(t, old, byID["site-retained"].ObservedAt)
	require.Equal(t, old, byID["site-offline"].ObservedAt)
	require.NotContains(t, byID, "site-removed", "a removed node is not a failed read")
}

func TestBackupHistoryUnreadTaskCannotCompleteRunningArchive(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, adverse := range []string{"unread", "ok-without-end", "warning-without-end", "error-without-end", "running-with-end", "active-with-end"} {
			t.Run(kind+"/"+adverse, func(t *testing.T) {
				f, storageClient := localArchiveIdentityFixture(t, kind)
				now := time.Now().UTC().Truncate(time.Second)
				f.client.at = now.Add(-time.Minute)
				var phase atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api2/json/nodes":
						fmt.Fprint(w, `{"data":[{"node":"node-a","status":"online"},{"node":"node-b","status":"online"}]}`)
					case "/api2/json/access/permissions":
						json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{r.URL.Query().Get("path"): map[string]int{"Sys.Audit": 1}}})
					case "/api2/json/nodes/node-a/tasks":
						fmt.Fprint(w, `{"data":[]}`)
					case "/api2/json/nodes/node-b/tasks":
						if phase.Load() == 1 && adverse == "unread" {
							fmt.Fprint(w, `{"data":null}`)
							return
						}
						task := proxmox.Task{UPID: "writer", Node: "node-b", Type: "vzdump", ID: "101", StartTime: now.Add(-10 * time.Minute).Unix(), Status: "running"}
						if phase.Load() == 1 {
							switch adverse {
							case "ok-without-end":
								task.Status = "OK"
							case "warning-without-end":
								task.Status = "WARNINGS: 1"
							case "error-without-end":
								task.Status = "ERROR: interrupted"
							case "running-with-end":
								task.EndTime = now.Unix()
							case "active-with-end":
								task.Status, task.EndTime = "active", now.Unix()
							}
						}
						if phase.Load() == 2 {
							task.Status, task.EndTime = "OK", now.Unix()
						}
						json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{task}})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer server.Close()
				client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
				require.NoError(t, err)
				var before models.BackupTask
				var beforePoint recovery.RecoveryPoint
				for i := range 3 {
					phase.Store(int32(i))
					f.m.pollBackupTasks(context.Background(), "cluster", client)
					f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", storageClient, f.nodes, f.online)
					f.wait(t)
					snapshot, points := f.m.state.GetSnapshot(), f.points(t)
					taskPoint, exists := points["pve-task:cluster-writer"]
					require.True(t, exists, "the actual task must reach retained execution history")
					require.Len(t, snapshot.PVEBackups.BackupTasks, 1)
					if i == 0 {
						before = snapshot.PVEBackups.BackupTasks[0]
						beforePoint = taskPoint
					} else if i == 1 {
						assert.Equal(t, before, snapshot.PVEBackups.BackupTasks[0], "unread task cannot refresh backup intent")
						assert.Equal(t, beforePoint, taskPoint, "carried task cannot renew durable point evidence")
						coverage, err := f.store.BackupHistoryCoverage(context.Background(), now.Add(-time.Hour), time.Now().UTC())
						require.NoError(t, err)
						require.Len(t, coverage.Sources, 1)
						assert.False(t, coverage.Sources[0].Complete, "malformed task reads cannot qualify report history")
					}
					require.Len(t, snapshot.PVEBackups.StorageBackups, 2)
					for _, archive := range snapshot.PVEBackups.StorageBackups {
						point := points["pve-backup:"+archive.ID]
						partial := archive.StorageNode == "node-b" && i < 2
						assert.Equal(t, partial, archive.InProgress)
						if partial {
							assert.Equal(t, recovery.OutcomeRunning, point.Outcome)
							assert.Nil(t, point.CompletedAt, "an unread task list cannot turn the writer's partial archive into success")
						} else {
							require.Equal(t, recovery.OutcomeSuccess, point.Outcome)
							require.NotNil(t, point.CompletedAt, "independent completed archive and ordinary completion stay usable")
						}
					}
				}
			})
		}
	}
}

func TestBackupHistoryCarryForwardCannotAccumulateLiveWindows(t *testing.T) {
	for _, previousCount := range []int{200, 215} {
		t.Run(fmt.Sprint(previousCount), func(t *testing.T) {
			m := &Monitor{state: models.NewState()}
			old := time.Now().UTC().Add(-time.Hour)
			previous := make([]models.BackupTask, previousCount)
			for i := range previous {
				previous[i] = models.BackupTask{ID: fmt.Sprintf("old-%d", i), Instance: "site", Node: "unread", Type: "vzdump", VMID: 101, StartTime: old, ObservedAt: old}
			}
			m.state.UpdateBackupTasksForInstance("site", previous)
			for poll := range 5 {
				fresh := make([]models.BackupTask, 200)
				for i := range fresh {
					fresh[i] = models.BackupTask{ID: fmt.Sprintf("fresh-%d-%d", poll, i), Instance: "site", Node: "unread", Type: "vzdump", VMID: 101, StartTime: old.Add(time.Duration(poll+1) * time.Minute), ObservedAt: time.Now().UTC()}
				}
				retained := m.retainUnreadBackupTasks("site", []proxmox.BackupTaskHistoryNode{{Node: "unread", Reason: "collection_failed"}}, fresh, nil)
				require.Len(t, retained, previousCount, "partial windows cannot grow the live payload on each failed poll")
				require.Equal(t, fresh, retained[:len(fresh)], "accepted page order and exact facts take priority")
				m.state.UpdateBackupTasksForInstance("site", retained)
			}
		})
	}
}

// Exercise the provider reader, the real poller and the next-read checkpoint.
// A new failure beyond a full overlapped page is evidence, but the window
// still has a gap.
func TestBackupHistoryOverlapKeepsFailuresAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	prior := now.Add(-2 * 24 * time.Hour)
	start := now.Add(-10 * time.Minute).Unix()
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.RecordBackupHistory(ctx, nil, []model.BackupHistoryCollection{
		{Provider: recovery.ProviderProxmoxPVE, Instance: "site", Complete: true, ObservedAt: prior, MaxGap: time.Hour},
	}))

	first := make([]proxmox.Task, 200)
	for i := range first {
		first[i] = proxmox.Task{UPID: fmt.Sprintf("task-a-%d", i), Type: "vzdump", ID: "101", StartTime: start, EndTime: start + 60, Status: "OK"}
	}
	completed := slices.Clone(first)
	first[0].EndTime, first[0].Status = 0, "running"
	middle := make([]proxmox.Task, 200)
	for i := range middle {
		middle[i] = proxmox.Task{UPID: fmt.Sprintf("task-middle-%d", i), Type: "vzdump", ID: "101", StartTime: start, EndTime: start + 60, Status: "OK"}
	}
	mixed := slices.Clone(middle)
	mixed[0] = completed[0]
	peer := proxmox.Task{UPID: "task-b", Type: "vzdump", ID: "102", StartTime: start, EndTime: start + 60, Status: "ERROR: fixture backup failed"}
	tail := proxmox.Task{UPID: "tail-failure", Type: "vzdump", ID: "103", StartTime: start, EndTime: start + 60, Status: "ERROR: fixture backup failed"}
	var phase atomic.Int32
	type pageRead struct{ offset, since string }
	var readsMu sync.Mutex
	var reads []pageRead
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.URL.Path {
		case "/api2/json/nodes":
			data = []map[string]string{{"node": "node-a", "status": "online"}, {"node": "node-b", "status": "online"}}
		case "/api2/json/access/permissions":
			data = map[string]any{r.URL.Query().Get("path"): map[string]int{"Sys.Audit": 1}}
		case "/api2/json/nodes/node-a/tasks":
			query := r.URL.Query()
			readsMu.Lock()
			reads = append(reads, pageRead{query.Get("start"), query.Get("since")})
			readsMu.Unlock()
			switch query.Get("start") {
			case "0":
				data = first
				if phase.Load() == 2 {
					data = completed
				}
			case "200":
				switch phase.Load() {
				case 0:
					data = mixed
				case 1:
					data = first
				default:
					data = middle
				}
			case "400":
				data = []proxmox.Task{tail}
			default:
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		case "/api2/json/nodes/node-b/tasks":
			data = []proxmox.Task{peer}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
	require.NoError(t, err)
	m := &Monitor{state: models.NewState(), recoveryManager: manager}

	var retainedTail recovery.RecoveryPoint
	for i, label := range []string{"full-mixed-overlap", "full-repeat", "disjoint-reread"} {
		t.Run(label, func(t *testing.T) {
			phase.Store(int32(i))
			readsMu.Lock()
			reads = nil
			readsMu.Unlock()
			m.pollBackupTasks(ctx, "site", client)
			wantSince := fmt.Sprint(prior.Add(-24 * time.Hour).Unix())
			wantReads := []pageRead{{"0", wantSince}, {"200", wantSince}}
			if i != 1 {
				wantReads = append(wantReads, pageRead{"400", wantSince})
			}
			readsMu.Lock()
			actualReads := slices.Clone(reads)
			readsMu.Unlock()
			assert.Equal(t, wantReads, actualReads, "read progress is independent of checkpoint qualification")
			until := time.Now().UTC().Add(time.Minute)
			points, coverage, err := store.BackupHealthHistory(ctx, prior.Add(-time.Hour), until)
			require.NoError(t, err)
			require.Len(t, coverage.Sources, 1)
			source := coverage.Sources[0]
			require.True(t, source.LastObservedAt.After(prior), "the attempted poll must be recorded")
			coldStart, err := store.BackupHistoryCollectionStart(ctx, recovery.ProviderProxmoxPVE, "site", until)
			require.NoError(t, err)
			live := m.state.GetSnapshot().PVEBackups.BackupTasks
			require.Len(t, live, 201, "the 200-row bound and sibling task are unchanged")
			require.True(t, slices.ContainsFunc(live, func(task models.BackupTask) bool {
				return task.ID == "site-task-b" && task.Node == "node-b" && task.Status == peer.Status
			}), "readable sibling work must still reach live state")
			require.True(t, slices.ContainsFunc(points, func(point recovery.RecoveryPoint) bool {
				return point.ID == "pve-task:site-task-b" && point.Outcome == recovery.OutcomeFailed
			}), "readable sibling failure must still reach saved history")
			tailIndex := slices.IndexFunc(points, func(point recovery.RecoveryPoint) bool {
				return point.ID == "pve-task:site-tail-failure"
			})
			if assert.NotEqual(t, -1, tailIndex, "the distinct failure at offset 400 must reach durable history") {
				point := points[tailIndex]
				assert.Equal(t, recovery.OutcomeFailed, point.Outcome)
				assert.Equal(t, "node-a", point.Details["node"])
				if i == 0 {
					retainedTail = point
				} else if i == 1 {
					assert.Equal(t, retainedTail, point, "the repeat-only reread preserves, not reobserves, the tail")
				}
			}
			if i < 2 {
				assert.Len(t, points, 401, "one execution per UPID, including the distinct tail")
				assert.False(t, source.Complete)
				assert.Equal(t, "collection_failed", source.Reason)
				assert.Equal(t, prior.Add(-24*time.Hour), coldStart, "retry from the last qualified window")
				assert.Len(t, coverage.Intervals, 1, "no new observation coverage was earned")
				assert.True(t, slices.ContainsFunc(points, func(point recovery.RecoveryPoint) bool {
					return point.ID == "pve-task:site-task-a-0" && point.Outcome == recovery.OutcomeRunning && point.CompletedAt == nil
				}), "a repeated UPID cannot turn a running job into a success")
			} else {
				require.Len(t, points, 402, "the disjoint reread also sees the previously missing job")
				require.True(t, source.Complete)
				require.Equal(t, "", source.Reason)
				require.Greater(t, coldStart.Unix(), prior.Unix())
				require.Len(t, coverage.Intervals, 2)
				require.True(t, slices.ContainsFunc(points, func(point recovery.RecoveryPoint) bool {
					return point.ID == "pve-task:site-task-a-0" && point.Outcome == recovery.OutcomeSuccess && point.CompletedAt != nil
				}), "a disjoint reread can now observe the completion")
			}
		})
	}
}
