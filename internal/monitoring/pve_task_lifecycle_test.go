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
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Synthetic GET-only HTTP -> direct/cluster clients -> ordinary polling,
// archive/age/recovery/JSON consumers. No database, guest/QGA, backup or native
// service. A nil-persistence manager enables History without opening a store.
func TestPVETaskLifecycleCannotQualifyArchiveOutput(t *testing.T) {
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
					data = map[string]any{"/nodes/node": map[string]int{"Sys.Audit": 1}}
				case "/api2/json/nodes/node/tasks":
					history := r.URL.Query().Has("since")
					c.record(fmt.Sprint("tasks-history=", history))
					row := map[string]any{"upid": "current", "node": "node", "type": "vzdump", "id": "100", "starttime": at.Unix(), "status": "running"}
					switch {
					case c.phase == "initial":
						row["upid"], row["starttime"], row["status"], row["endtime"] = "previous", at.Add(-48*time.Hour).Unix(), "ERROR: fixture failure", at.Add(-48*time.Hour+time.Minute).Unix()
					case c.phase == "empty":
						data = []any{}
					case c.phase == "finished":
						row["status"], row["endtime"] = "OK", at.Add(time.Minute).Unix()
					case strings.HasPrefix(c.phase, "live-") && history:
						data = []any{}
					case strings.HasPrefix(c.phase, "live-") || (strings.HasPrefix(c.phase, "history-") && history):
						switch strings.TrimPrefix(strings.TrimPrefix(c.phase, "live-"), "history-") {
						case "running-end":
							row["endtime"] = at.Add(time.Minute).Unix()
						case "success-no-end":
							row["status"] = "OK"
						case "end-before-start":
							row["status"], row["endtime"] = "OK", at.Add(-time.Second).Unix()
						case "missing-start":
							row["status"], row["endtime"] = "OK", at.Add(time.Minute).Unix()
							delete(row, "starttime")
						}
					}
					if data == nil {
						data = []any{row}
					}
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
				client = proxmox.NewClusterClient("task-lifecycle", cfg, []string{server.URL}, nil)
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
			peerJob := models.BackupTask{ID: "other-job", Instance: "other", Node: "node", Type: "vzdump", VMID: 100, Status: "OK", StartTime: at.Add(-time.Hour), EndTime: at.Add(-time.Minute), ObservedAt: at}
			m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{peerJob})
			phases := []string{"initial", "live-running-end", "live-success-no-end", "live-end-before-start", "live-missing-start", "running", "history-running-end", "history-success-no-end", "history-end-before-start", "history-missing-start", "finished", "empty"}
			frames := map[string]models.PVEBackups{}
			var held models.StorageBackup
			var heldTasks []models.BackupTask
			for _, phase := range phases {
				c.phase, c.calls = phase, nil
				snapshotStarted.Store(false)
				m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, 2*time.Second)
				awaitBackupProjectionIngest(m)
				got := m.PVEBackupsSnapshot()
				var own []models.StorageBackup
				var ownTasks []models.BackupTask
				for _, b := range got.StorageBackups {
					if b.Instance == "other" {
						require.Equal(t, peer, b)
					} else {
						own = append(own, b)
					}
				}
				for _, job := range got.BackupTasks {
					if job.Instance == "other" {
						require.Equal(t, peerJob, job)
					} else {
						ownTasks = append(ownTasks, job)
					}
				}
				assert.Len(t, got.StorageBackups, len(own)+1, phase+": independent connection must survive")
				require.Len(t, got.GuestSnapshots, 1)
				assert.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				switch {
				case phase == "initial":
					require.Len(t, own, 1)
					held = own[0]
					heldTasks = ownTasks
				case strings.HasPrefix(phase, "live-"):
					assert.Equal(t, []models.StorageBackup{held}, own, phase+": unavailable live lifecycle must not admit archives")
					assert.NotContains(t, c.calls, "content", phase)
					assert.Equal(t, heldTasks, ownTasks, phase+": invalid read must not renew or replace prior task evidence")
				case phase == "running" || strings.HasPrefix(phase, "history-") || phase == "finished":
					assert.Len(t, own, 2, phase)
					for _, b := range own {
						if b.ID != held.ID {
							assert.Equal(t, phase != "finished", b.InProgress, phase+": no contradictory History verdict can complete the current archive")
						}
					}
					if assert.Len(t, ownTasks, 1, phase) {
						status := "running"
						if phase == "finished" {
							status = "OK"
						}
						assert.Equal(t, status, ownTasks[0].Status, phase)
						assert.Equal(t, 100, ownTasks[0].VMID)
					}
				case phase == "empty":
					assert.Empty(t, own)
					assert.Empty(t, ownTasks, "ordinary complete emptiness retains reconciliation authority")
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
				assert.Equal(t, want, successes, phase+": recovery consumer cannot receive false success")
				if phase != "empty" {
					age := held.Time
					if phase == "finished" {
						age = at
					}
					assert.True(t, age.Equal(m.state.GetSnapshot().VMs[0].LastBackup), phase+": incomplete task cannot renew backup age")
				}
				frames[phase] = got
			}
			b, err := json.Marshal(map[string]any{"client": name, "frames": frames})
			require.NoError(t, err)
			t.Logf("PVE_TASK_LIFECYCLE_FRAMES=%s", b)
		})
	}
}
