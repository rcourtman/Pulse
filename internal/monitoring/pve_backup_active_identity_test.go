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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Closed GET-only provider -> actual direct/cluster task reader -> ordinary
// archive scan, backup age and recovery mapper. No database or guest operation.
func TestPVEArchiveScanRejectsChangedActiveExecutionIdentity(t *testing.T) {
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
					source := r.URL.Query().Get("source")
					c.record("tasks-" + source)
					current := proxmox.Task{UPID: "current", Type: "vzdump", ID: "100", PID: 17, PStart: 5, StartTime: at.Unix(), User: "fixture@pve"}
					if c.phase == "initial" || c.phase == "empty" {
						data = []proxmox.Task{}
					} else if c.phase == "finished" {
						current.Status = "OK"
						current.EndTime = at.Add(time.Minute).Unix()
						data = []proxmox.Task{current}
					} else if source == "all" {
						tasks := make([]proxmox.Task, 200)
						for i := range tasks {
							tasks[i] = proxmox.Task{UPID: fmt.Sprint("recent-", i), Type: "vzdump", ID: "101", Status: "OK", StartTime: at.Add(time.Minute).Unix(), EndTime: at.Add(2 * time.Minute).Unix()}
						}
						tasks[199] = current
						data = tasks
					} else {
						require.Equal(t, "active", source)
						switch c.phase {
						case "conflict-guest":
							current.ID = "101"
						case "conflict-parent":
							current.ID = ""
						case "conflict-start":
							current.StartTime++
						}
						data = []proxmox.Task{current}
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
					t.Errorf("unexpected request %s", r.URL)
					http.NotFound(w, r)
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 2 * time.Second}
			var client PVEClientInterface
			if cluster {
				client = proxmox.NewClusterClient("active-identity", cfg, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(cfg)
				require.NoError(t, err)
			}
			m := backupLifecycleMonitor(client)
			t.Cleanup(func() { awaitBackupProjectionIngest(m) })
			peer := models.StorageBackup{ID: "independent", Instance: "other", Node: "node", Storage: "archive", VMID: 100, Type: "qemu", Time: at.Add(-time.Hour), CTime: at.Add(-time.Hour).Unix(), Size: 8192, Notes: "independent connection"}
			m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
			frames := map[string]models.PVEBackups{}
			var held models.StorageBackup
			for _, phase := range []string{"initial", "conflict-guest", "conflict-parent", "conflict-start", "running", "finished", "empty"} {
				c.phase, c.calls = phase, nil
				snapshotStarted.Store(false)
				prior := m.PVEBackupsSnapshot().BackupTasks
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
				require.Len(t, got.StorageBackups, len(own)+1, "independent same-VMID archive is unchanged")
				require.Len(t, got.GuestSnapshots, 1, "independent snapshot refresh continues")
				require.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				switch phase {
				case "initial":
					require.Len(t, own, 1)
					held = own[0]
				case "conflict-guest", "conflict-parent", "conflict-start":
					require.Equal(t, []models.StorageBackup{held}, own, "contradictory execution identity cannot qualify a new archive")
					require.Equal(t, prior, got.BackupTasks, "contradiction is not live replacement authority")
					require.NotContains(t, c.calls, "content")
					require.NotContains(t, c.calls, "audit")
				case "running", "finished":
					require.Len(t, own, 2)
					for _, b := range own {
						if b.ID != held.ID {
							require.Equal(t, phase == "running", b.InProgress)
						}
					}
				case "empty":
					require.Empty(t, own, "ordinary complete emptiness can remove old archives")
				}
				wantSuccess := 1
				if phase == "finished" {
					wantSuccess = 2
				} else if phase == "empty" {
					wantSuccess = 0
				}
				successes := 0
				for _, p := range proxmoxmapper.FromPVEStorageBackups(own, nil) {
					if p.Outcome == recovery.OutcomeSuccess {
						successes++
					}
				}
				require.Equal(t, wantSuccess, successes, "archive ambiguity cannot create a successful recovery point")
				if phase != "empty" {
					want := held.Time
					if phase == "finished" {
						want = at
					}
					require.True(t, want.Equal(m.state.GetSnapshot().VMs[0].LastBackup), "only ordinary complete recovery advances backup age")
				}
				if strings.HasPrefix(phase, "conflict-") {
					require.Contains(t, c.calls, "tasks-all")
					require.Contains(t, c.calls, "tasks-active")
				}
				frames[phase] = got
			}
			b, err := json.Marshal(map[string]any{"client": name, "frames": frames})
			require.NoError(t, err)
			t.Logf("PVE_ACTIVE_IDENTITY_FRAMES=%s", b)
		})
	}
}
