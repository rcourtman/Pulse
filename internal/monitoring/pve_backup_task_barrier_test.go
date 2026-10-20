package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Closed, read-only fixture: exercise the ordinary scan and its real state,
// backup-age and recovery consumers. No listener, database, guest/QGA command,
// backup, freeze/thaw or installed service is used.
type taskBarrierClient struct {
	fakeSnapshotClient
	mu    sync.Mutex
	phase string
	calls []string
	at    time.Time
}

func (c *taskBarrierClient) record(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *taskBarrierClient) GetBackupTasks(context.Context) ([]proxmox.Task, error) {
	c.record("tasks")
	if c.phase == "unavailable" || c.phase == "unavailable-again" {
		return nil, errors.New("synthetic task inventory unavailable")
	}
	if c.phase == "initial" || c.phase == "empty" {
		return nil, nil
	}
	status, end := "running", int64(0)
	if c.phase == "finished" {
		status, end = "OK", c.at.Add(time.Minute).Unix()
	}
	return []proxmox.Task{{UPID: "current", Node: "node", Type: "vzdump", ID: "100", StartTime: c.at.Unix(), Status: status, EndTime: end}}, nil
}

func (c *taskBarrierClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	c.record("storage")
	return []proxmox.Storage{{Storage: "archive", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}

func (c *taskBarrierClient) GetStorageContent(context.Context, string, string) ([]proxmox.StorageContent, error) {
	c.record("content")
	if c.phase == "empty" {
		return nil, nil
	}
	old := proxmox.StorageContent{Volid: "archive:backup/vzdump-qemu-100-previous.vma.zst", Content: "backup", Format: "vma.zst", VMID: 100, CTime: c.at.Add(-48 * time.Hour).Unix(), Size: 4096, Protected: 1, Notes: "preserved archive"}
	if c.phase == "initial" {
		return []proxmox.StorageContent{old}, nil
	}
	return []proxmox.StorageContent{old, {Volid: "archive:backup/vzdump-qemu-100-current.vma.zst", Content: "backup", Format: "vma.zst", VMID: 100, CTime: c.at.Unix(), Size: 1024}}, nil
}

func (c *taskBarrierClient) GetVMSnapshots(context.Context, string, int) ([]proxmox.Snapshot, error) {
	c.record("snapshots")
	return []proxmox.Snapshot{{Name: "snapshot-" + c.phase, SnapTime: c.at.Add(-72 * time.Hour).Unix()}}, nil
}

// Match the production client's optional scoped-inventory interface without
// making the legacy adapter above accidentally implement it too.
type scopedTaskBarrierClient struct{ *taskBarrierClient }

func (c *scopedTaskBarrierClient) GetBackupTaskInventory(ctx context.Context) (proxmox.BackupTaskInventory, error) {
	tasks, err := c.taskBarrierClient.GetBackupTasks(ctx)
	return proxmox.BackupTaskInventory{Tasks: tasks}, err
}

func TestPVEArchiveScanRequiresCurrentTaskInventory(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("scoped=%t", scoped), func(t *testing.T) {
			at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			c := &taskBarrierClient{at: at}
			var client PVEClientInterface = c
			if scoped {
				client = &scopedTaskBarrierClient{c}
			}
			m := backupLifecycleMonitor(client)
			t.Cleanup(func() { awaitBackupProjectionIngest(m) })
			peer := models.StorageBackup{ID: "independent", Instance: "other", Node: "node", Storage: "archive", VMID: 100, Type: "qemu", Time: at.Add(-time.Hour), CTime: at.Add(-time.Hour).Unix(), Size: 8192, Notes: "independent connection"}
			m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
			frames := map[string]models.PVEBackups{}
			var held models.StorageBackup
			for _, phase := range []string{"initial", "unavailable", "unavailable-again", "running", "finished", "empty"} {
				c.phase, c.calls = phase, nil
				m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, time.Second)
				awaitBackupProjectionIngest(m)
				got := m.PVEBackupsSnapshot()
				var own []models.StorageBackup
				for _, b := range got.StorageBackups {
					if b.Instance == "other" {
						require.Equal(t, peer, b)
					} else {
						own = append(own, b)
					}
				}
				require.Len(t, got.StorageBackups, len(own)+1, "same-VMID independent archive must survive")
				require.Len(t, got.GuestSnapshots, 1, "independent snapshot collection must continue")
				require.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				if phase == "initial" {
					require.Len(t, own, 1)
					held = own[0]
				}
				switch phase {
				case "unavailable", "unavailable-again":
					assert.NotContains(t, c.calls, "content", "failed prerequisite must not request or reconcile archive content")
					assert.Equal(t, []models.StorageBackup{held}, own, "failed read cannot add partial output, erase or alter a known archive")
				case "running", "finished":
					require.Len(t, own, 2)
					for _, b := range own {
						if b.ID != held.ID {
							require.Equal(t, phase == "running", b.InProgress, "ordinary current tasks classify partial output")
						}
					}
				case "empty":
					require.Empty(t, own, "successful empty task/content reads retain removal authority")
				}
				var last time.Time
				for _, vm := range m.state.GetSnapshot().VMs {
					if vm.Instance == "site" && vm.VMID == 100 {
						last = vm.LastBackup
					}
				}
				if phase != "empty" {
					want := held.Time
					if phase == "finished" {
						want = at
					}
					assert.True(t, want.Equal(last), "unqualified output must not renew last-backup age: got %s, want %s", last, want)
				}
				points := proxmoxmapper.FromPVEStorageBackups(own, nil)
				successes := 0
				for _, point := range points {
					if point.Outcome == recovery.OutcomeSuccess {
						successes++
					}
				}
				expected := 1
				if phase == "finished" {
					expected = 2
				} else if phase == "empty" {
					expected = 0
				}
				assert.Equal(t, expected, successes, "recovery consumers must not receive a false successful archive")
				frames[phase] = got
			}
			if scoped {
				wire, err := json.Marshal(frames)
				require.NoError(t, err)
				t.Logf("PVE_ARCHIVE_BARRIER_FRAMES=%s", wire)
			}
		})
	}
}

// A complete retained-history read is independently useful evidence, but it
// cannot substitute for this scan's failed live-task prerequisite. A nil-store
// manager enables the ordinary history branch without opening any database.
type taskBarrierHistoryClient struct{ *scopedTaskBarrierClient }

func (c *taskBarrierHistoryClient) GetBackupTaskHistory(context.Context, time.Time) (proxmox.BackupTaskHistory, error) {
	c.record("history")
	return proxmox.BackupTaskHistory{Nodes: []proxmox.BackupTaskHistoryNode{{Node: "node", Complete: true}}, Tasks: []proxmox.Task{{UPID: "history", Node: "node", Type: "vzdump", ID: "100", Status: "OK", StartTime: c.at.Add(-time.Hour).Unix(), EndTime: c.at.Add(-time.Minute).Unix()}}}, nil
}

func TestPVEArchiveReadBarrierColdStartAndHistory(t *testing.T) {
	for _, history := range []bool{false, true} {
		t.Run(fmt.Sprintf("history=%t", history), func(t *testing.T) {
			c := &taskBarrierClient{at: time.Now().Add(-time.Hour), phase: "unavailable"}
			var client PVEClientInterface = &scopedTaskBarrierClient{c}
			if history {
				client = &taskBarrierHistoryClient{&scopedTaskBarrierClient{c}}
			}
			m := backupLifecycleMonitor(client)
			if history {
				m.recoveryManager = recoverymanager.New(nil)
			}
			m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, time.Second)
			awaitBackupProjectionIngest(m)
			got := m.PVEBackupsSnapshot()
			require.Empty(t, got.StorageBackups, "cold start cannot mint completed output from a failed task read")
			require.Empty(t, got.BackupTasks, "history is not live membership")
			require.Len(t, got.GuestSnapshots, 1, "independent snapshot read still completes")
			require.NotContains(t, c.calls, "content")
			if history {
				require.Contains(t, c.calls, "history")
			}
		})
	}
}

// Actual read-only HTTP clients -> ordinary scan -> archive/age/recovery
// consumers. A 200 response with own-task visibility is not negative evidence
// of other users' writers; a saturated recent page needs its active census.
func TestPVEArchiveScanRequiresConfirmedTaskVisibility(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		t.Run(fmt.Sprintf("cluster=%t", cluster), func(t *testing.T) {
			at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			c := &taskBarrierClient{at: at}
			var snapshotStarted atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				var data any
				switch r.URL.Path {
				case "/api2/json/nodes":
					data = []proxmox.Node{{Node: "node", Status: "online"}}
				case "/api2/json/access/permissions":
					require.Equal(t, "/nodes/node", r.URL.Query().Get("path"))
					audit := 1
					if c.phase == "own-only" || c.phase == "own-empty" {
						audit = 0
					}
					data = map[string]any{"/nodes/node": map[string]int{"Sys.Audit": audit}}
				case "/api2/json/nodes/node/tasks":
					c.record("tasks")
					if c.phase == "active-failed" || c.phase == "active-bounded" || c.phase == "running" {
						if r.URL.Query().Get("source") == "active" && c.phase != "active-bounded" {
							if c.phase == "active-failed" {
								w.WriteHeader(http.StatusForbidden)
								return
							}
							data = []proxmox.Task{{UPID: "current", Type: "vzdump", ID: "100", StartTime: at.Unix()}}
						} else {
							tasks := make([]proxmox.Task, 200)
							for i := range tasks {
								tasks[i] = proxmox.Task{UPID: fmt.Sprint("recent-", i), Type: "vzdump", ID: "101", Status: "OK", StartTime: at.Add(time.Minute).Unix(), EndTime: at.Add(2 * time.Minute).Unix()}
							}
							data = tasks
						}
					} else if c.phase == "own-only" || c.phase == "own-empty" {
						data = []proxmox.Task{}
					} else {
						tasks, err := c.GetBackupTasks(r.Context())
						require.NoError(t, err)
						data = tasks
						if tasks == nil {
							data = []proxmox.Task{}
						}
					}
				case "/api2/json/nodes/node/storage":
					if snapshotStarted.Load() {
						// Independent snapshot sizing has no eligible disk store;
						// do not confuse its read with archive reconciliation.
						data = []proxmox.Storage{}
					} else {
						data, _ = c.GetStorage(r.Context(), "node")
					}
				case "/api2/json/nodes/node/storage/archive/content":
					data, _ = c.GetStorageContent(r.Context(), "node", "archive")
					if c.phase == "empty" {
						data = []proxmox.StorageContent{}
					}
				case "/api2/json/nodes/node/qemu/100/snapshot":
					snapshotStarted.Store(true)
					data, _ = c.GetVMSnapshots(r.Context(), "node", 100)
				default:
					t.Errorf("unexpected provider request %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 2 * time.Second}
			var client PVEClientInterface
			if cluster {
				client = proxmox.NewClusterClient("visibility", cfg, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(cfg)
				require.NoError(t, err)
			}
			m := backupLifecycleMonitor(client)
			t.Cleanup(func() { awaitBackupProjectionIngest(m) })
			peer := models.StorageBackup{ID: "independent", Instance: "other", Node: "node", Storage: "archive", VMID: 100, Type: "qemu", Time: at.Add(-time.Hour), CTime: at.Add(-time.Hour).Unix(), Size: 8192}
			m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
			frames := map[string]models.PVEBackups{}
			var held []models.StorageBackup
			for _, phase := range []string{"initial", "own-only", "own-empty", "active-failed", "active-bounded", "running", "finished", "empty"} {
				c.phase, c.calls = phase, nil
				snapshotStarted.Store(false)
				m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, 2*time.Second)
				awaitBackupProjectionIngest(m)
				got := m.PVEBackupsSnapshot()
				var own []models.StorageBackup
				for _, b := range got.StorageBackups {
					if b.Instance == "other" {
						require.Equal(t, peer, b)
					} else {
						own = append(own, b)
					}
				}
				require.Len(t, got.GuestSnapshots, 1)
				require.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				if phase == "initial" {
					require.Len(t, own, 1)
					held = own
				}
				switch phase {
				case "own-only", "own-empty", "active-failed", "active-bounded":
					require.Equal(t, held, own, "uncertain task visibility cannot add, renew or erase archives")
					require.NotContains(t, c.calls, "content")
				case "running", "finished":
					require.Len(t, own, 2)
					for _, b := range own {
						if b.ID != held[0].ID {
							require.Equal(t, phase == "running", b.InProgress)
						}
					}
				case "empty":
					require.Empty(t, own, "confirmed empty recovery retains removal authority")
				}
				if phase == "active-failed" || phase == "active-bounded" || phase == "running" {
					require.Len(t, got.BackupTasks, 200, "keep the existing bounded positive task view")
				}
				if phase == "running" {
					found := false
					for _, task := range got.BackupTasks {
						found = found || task.ID == "site-current"
					}
					require.True(t, found, "older active writer must survive the live projection cap")
				}
				if phase != "empty" {
					want := held[0].Time
					if phase == "finished" {
						want = at
					}
					require.True(t, want.Equal(m.state.GetSnapshot().VMs[0].LastBackup), "only a completed archive can advance backup age")
				}
				points := proxmoxmapper.FromPVEStorageBackups(own, nil)
				successes := 0
				for _, point := range points {
					if point.Outcome == recovery.OutcomeSuccess {
						successes++
					}
				}
				want := 1
				if phase == "finished" {
					want = 2
				} else if phase == "empty" {
					want = 0
				}
				require.Equal(t, want, successes)
				frames[phase] = got
			}
			if !cluster {
				wire, err := json.Marshal(frames)
				require.NoError(t, err)
				t.Logf("PVE_ARCHIVE_VISIBILITY_FRAMES=%s", wire)
			}
		})
	}
}

type limitedTaskLogClient struct{ lifecycleBackupClient }

func (c *limitedTaskLogClient) GetBackupTaskInventory(ctx context.Context) (proxmox.BackupTaskInventory, error) {
	tasks, err := c.GetBackupTasks(ctx)
	tasks[0].ID = "" // Observed scheduled parent, not an individual guest task.
	return proxmox.BackupTaskInventory{Tasks: tasks, UnconfirmedNodes: []string{"node"}}, err
}

func TestPVEUnconfirmedTaskVisibilityKeepsObservedGuestLogs(t *testing.T) {
	c := &limitedTaskLogClient{lifecycleBackupClient{label: "parent"}}
	m := backupLifecycleMonitor(c)
	// A scheduled parent has no guest ID. Exercise its normal expansion while
	// archive admission remains unqualified, without requiring broader grants.
	ready := m.pollBackupTasks(context.Background(), "site", c)
	require.False(t, ready)
	got := m.PVEBackupsSnapshot()
	require.Len(t, got.BackupTasks, 2)
	byID := map[string]models.BackupTask{}
	for _, task := range got.BackupTasks {
		byID[task.ID] = task
	}
	require.Equal(t, 100, byID["site-parent-vm100"].VMID)
	require.Equal(t, "OK", byID["site-parent-vm100"].Status)
	require.True(t, m.vzdumpJobTaskCache["site|parent"].final)
}

// GET-only direct/cluster readers exercise the actual live/History merge.
// The nil-persistence manager selects that branch without opening a database.
func TestPVEBackupHistoryExecutionIdentity(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		for _, change := range []string{"identical", "completion", "implicit-node", "guest", "parent", "pid", "process-start", "start-time", "owner"} {
			t.Run(fmt.Sprintf("cluster=%t/%s", cluster, change), func(t *testing.T) {
				live := proxmox.Task{UPID: "opaque-execution", Node: "node", Type: "vzdump", ID: "100", PID: 17, PStart: 5, StartTime: 1792350000, User: "fixture@pve", Status: "ERROR: archive unavailable", EndTime: 1792350060}
				later := live
				if change != "identical" {
					later.Status = "OK"
				}
				switch change {
				case "completion":
					later.EndTime++
				case "implicit-node":
					later.Node = ""
				case "guest":
					later.ID = "101"
				case "parent":
					later.ID = ""
				case "pid":
					later.PID++
				case "process-start":
					later.PStart++
				case "start-time":
					later.StartTime++
				case "owner":
					later.User = "other@pve"
				}
				peer := live
				peer.UPID, peer.ID, peer.Status = "independent", "200", "OK"
				backfill := live
				backfill.UPID, backfill.ID = "history-only", "201"
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method)
					var data any
					switch r.URL.Path {
					case "/api2/json/nodes":
						data = []proxmox.Node{{Node: "node", Status: "online"}}
					case "/api2/json/access/permissions":
						data = map[string]any{"/nodes/node": map[string]int{"Sys.Audit": 1}}
					case "/api2/json/nodes/node/tasks":
						if r.URL.Query().Has("since") {
							data = []proxmox.Task{later, peer, backfill}
						} else {
							data = []proxmox.Task{live, peer}
						}
					case "/api2/json/nodes/node/tasks/opaque-execution/log":
						data = []proxmox.TaskLogLine{}
					default:
						t.Errorf("unexpected GET %s", r.URL)
						http.NotFound(w, r)
						return
					}
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
				}))
				defer server.Close()
				cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 2 * time.Second}
				var client PVEClientInterface
				if cluster {
					client = proxmox.NewClusterClient("history-identity", cfg, []string{server.URL}, nil)
				} else {
					var err error
					client, err = proxmox.NewClient(cfg)
					require.NoError(t, err)
				}
				m := &Monitor{state: models.NewState(), recoveryManager: recoverymanager.New(nil)}
				other := models.BackupTask{ID: "another-site", Instance: "other", Node: "node", VMID: 100, Status: "OK", Type: "vzdump", ObservedAt: time.Unix(1792350000, 0)}
				m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{other})
				consistent := change == "identical" || change == "completion" || change == "implicit-node"
				assert.Equal(t, consistent, m.pollBackupTasks(context.Background(), "site", client), "only matching execution identity may qualify the archive pass")
				got := m.PVEBackupsSnapshot()
				require.Len(t, got.BackupTasks, 3, "history-only tasks cannot become live membership")
				for _, task := range got.BackupTasks {
					switch task.ID {
					case "another-site":
						require.Equal(t, other, task)
					case "site-independent":
						assert.Equal(t, 200, task.VMID)
						assert.Equal(t, "OK", task.Status)
					case "site-opaque-execution":
						assert.Equal(t, 100, task.VMID, "History cannot move a live failure to another guest or parent")
						assert.Equal(t, "node", task.Node)
						assert.Equal(t, live.StartTime, task.StartTime.Unix())
						status, end := live.Status, live.EndTime
						if consistent {
							status, end = later.Status, later.EndTime
						}
						assert.Equal(t, status, task.Status, "contradictory History cannot erase the observed live failure")
						assert.Equal(t, end, task.EndTime.Unix())
					default:
						t.Errorf("unexpected task %+v", task)
					}
				}
			})
		}
	}
}

// Ordinary HTTP scan -> current tasks, archives, backup age and recovery mapper.
// Safe synthetic observations only: no guest, QGA, database or native service.
func TestPVEArchiveScanRejectsHistoryExecutionIdentity(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		name := "direct"
		if cluster {
			name = "cluster"
		}
		t.Run(name, func(t *testing.T) {
			at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			c := &taskBarrierClient{at: at}
			var snapshotStarted atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				var data any
				switch r.URL.Path {
				case "/api2/json/nodes":
					data = []proxmox.Node{{Node: "node", Status: "online"}}
				case "/api2/json/access/permissions":
					c.record("audit")
					data = map[string]any{"/nodes/node": map[string]int{"Sys.Audit": 1}}
				case "/api2/json/nodes/node/tasks":
					history := r.URL.Query().Has("since")
					c.record(fmt.Sprint("tasks-history=", history))
					task := proxmox.Task{UPID: "current", Node: "node", Type: "vzdump", ID: "100", PID: 17, PStart: 5, StartTime: at.Unix(), User: "fixture@pve", Status: "running"}
					if c.phase == "initial" || c.phase == "empty" {
						data = []proxmox.Task{}
					} else {
						if history {
							switch c.phase {
							case "conflict-guest", "conflict-again":
								task.ID = "101"
							case "conflict-parent":
								task.ID = ""
							case "conflict-start":
								task.StartTime++
							}
							if c.phase != "running" {
								task.Status = "OK"
								task.EndTime = at.Add(time.Minute).Unix()
							}
						}
						data = []proxmox.Task{task}
					}
				case "/api2/json/nodes/node/tasks/current/log":
					data = []proxmox.TaskLogLine{}
				case "/api2/json/nodes/node/storage":
					if snapshotStarted.Load() {
						data = []proxmox.Storage{}
					} else {
						data, _ = c.GetStorage(r.Context(), "node")
					}
				case "/api2/json/nodes/node/storage/archive/content":
					data, _ = c.GetStorageContent(r.Context(), "node", "archive")
					if c.phase == "empty" {
						data = []proxmox.StorageContent{}
					}
				case "/api2/json/nodes/node/qemu/100/snapshot":
					snapshotStarted.Store(true)
					data, _ = c.GetVMSnapshots(r.Context(), "node", 100)
				default:
					t.Errorf("unexpected GET %s", r.URL)
					http.NotFound(w, r)
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 2 * time.Second}
			var client PVEClientInterface
			if cluster {
				client = proxmox.NewClusterClient("history-identity", cfg, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(cfg)
				require.NoError(t, err)
			}
			m := backupLifecycleMonitor(client)
			m.recoveryManager = recoverymanager.New(nil)
			t.Cleanup(func() { awaitBackupProjectionIngest(m) })
			peer := models.StorageBackup{ID: "independent", Instance: "other", Node: "node", Storage: "archive", VMID: 100, Type: "qemu", Time: at.Add(-time.Hour), CTime: at.Add(-time.Hour).Unix(), Size: 8192, Notes: "independent connection"}
			m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
			frames := map[string]models.PVEBackups{}
			var held models.StorageBackup
			for _, phase := range []string{"initial", "conflict-guest", "conflict-again", "conflict-parent", "conflict-start", "running", "finished", "empty"} {
				c.phase, c.calls = phase, nil
				snapshotStarted.Store(false)
				m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, 2*time.Second)
				awaitBackupProjectionIngest(m)
				got := m.PVEBackupsSnapshot()
				var own []models.StorageBackup
				for _, b := range got.StorageBackups {
					if b.Instance == "other" {
						require.Equal(t, peer, b)
					} else {
						own = append(own, b)
					}
				}
				require.Len(t, got.StorageBackups, len(own)+1)
				require.Len(t, got.GuestSnapshots, 1, "independent snapshot collection still progresses")
				require.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				switch {
				case phase == "initial":
					require.Len(t, own, 1)
					held = own[0]
				case strings.HasPrefix(phase, "conflict-"):
					require.Equal(t, []models.StorageBackup{held}, own, "contradictory History must not qualify new output or renew backup age")
					require.NotContains(t, c.calls, "content")
					require.Len(t, got.BackupTasks, 1)
					require.Equal(t, 100, got.BackupTasks[0].VMID)
					require.Equal(t, "running", got.BackupTasks[0].Status, "preserve the actual live observation")
				case phase == "running" || phase == "finished":
					require.Len(t, own, 2)
					for _, b := range own {
						if b.ID != held.ID {
							require.Equal(t, phase == "running", b.InProgress)
						}
					}
				case phase == "empty":
					require.Empty(t, own, "complete ordinary emptiness still reconciles absence")
				}
				successes := 0
				for _, p := range proxmoxmapper.FromPVEStorageBackups(own, nil) {
					if p.Outcome == recovery.OutcomeSuccess {
						successes++
					}
				}
				want := 1
				if phase == "finished" {
					want = 2
				} else if phase == "empty" {
					want = 0
				}
				require.Equal(t, want, successes)
				if phase != "empty" {
					age := held.Time
					if phase == "finished" {
						age = at
					}
					require.True(t, age.Equal(m.state.GetSnapshot().VMs[0].LastBackup))
				}
				require.Contains(t, c.calls, "tasks-history=true")
				frames[phase] = got
			}
			b, err := json.Marshal(map[string]any{"client": name, "frames": frames})
			require.NoError(t, err)
			t.Logf("PVE_HISTORY_IDENTITY_FRAMES=%s", b)
		})
	}
}
