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
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Synthetic HTTP -> ordinary direct/cluster clients -> live reconciliation ->
// served backup projection. No database, guest/backup command or native target.
func TestPVEBackupTaskIdentityGapCannotEraseFailure(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		name := "direct"
		if cluster {
			name = "cluster"
		}
		t.Run(name, func(t *testing.T) {
			var phase atomic.Int32
			primaryUPID := "UPID:pve-a:00000100:00000001:67000000:vzdump:100:fixture@pve:"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"pve-a","status":"online"},{"node":"pve-b","status":"online"}]}`)
				case r.URL.Path == "/api2/json/access/permissions":
					fmt.Fprintf(w, `{"data":{%q:{"Sys.Audit":1}}}`, r.URL.Query().Get("path"))
				case strings.HasSuffix(r.URL.Path, "/tasks"):
					task := map[string]any{"upid": primaryUPID, "node": "pve-a", "type": "vzdump", "id": "100", "status": "ERROR: archive unavailable", "starttime": 1792350000, "endtime": 1792350060}
					peer := strings.Contains(r.URL.Path, "/pve-b/")
					if peer {
						task["upid"], task["node"], task["id"], task["status"] = "peer-job", "pve-b", "200", "OK"
					}
					rows := []map[string]any{task}
					p := phase.Load()
					history := r.URL.Query().Has("since")
					if !peer && p == 7 {
						task["status"] = "OK"
					}
					if !peer && ((p >= 1 && p <= 3 && !history) || (p >= 4 && p <= 6 && history)) {
						task["status"] = "OK"
						if history {
							invalid := map[string]any{"upid": "unidentified", "type": "vzdump", "id": "101", "status": "OK"}
							rows = append(rows, invalid)
							switch p {
							case 4:
								delete(invalid, "type")
							case 5:
								delete(invalid, "upid")
							case 6:
								invalid["upid"] = primaryUPID
								invalid["status"] = "ERROR: contradictory reply"
							}
						} else {
							switch p {
							case 1:
								delete(task, "type")
							case 2:
								delete(task, "upid")
							case 3:
								rows = append(rows, map[string]any{"upid": primaryUPID, "type": "vzdump", "id": "100", "node": "pve-a", "status": "ERROR: contradictory reply"})
							}
						}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": rows})
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second}
			var client PVEClientInterface
			if cluster {
				client = proxmox.NewClusterClient("task-identity", cfg, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(cfg)
				require.NoError(t, err)
			}
			m := &Monitor{state: models.NewState(), recoveryManager: recoverymanager.New(nil)}
			other := models.BackupTask{ID: "other-job", Instance: "other", Node: "pve-a", VMID: 100, Type: "vzdump", Status: "OK", StartTime: time.Unix(1792350000, 0), EndTime: time.Unix(1792350060, 0), ObservedAt: time.Unix(1792350120, 0)}
			m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{other})
			project := func(label string) []models.BackupTask {
				snapshot := m.state.GetSnapshot().PVEBackups
				wire, err := json.Marshal(map[string]any{"data": snapshot})
				require.NoError(t, err)
				t.Logf("TASK_IDENTITY_%s_%s=%s", name, label, wire)
				return snapshot.BackupTasks
			}
			for i, label := range []string{"live-missing-type", "live-missing-upid", "live-duplicate", "history-missing-type", "history-missing-upid", "history-duplicate"} {
				phase.Store(0)
				m.pollBackupTasks(context.Background(), "site", client)
				initial := m.state.GetSnapshot().PVEBackups.BackupTasks
				if i == 0 {
					project("initial")
				}
				phase.Store(int32(i + 1))
				if i >= 3 {
					_, observation, err := m.readBackupTaskHistory(context.Background(), "site", client)
					require.NoError(t, err)
					if observation.Complete || observation.Reason != "collection_failed" {
						t.Errorf("%s: unidentified history became complete: %+v", label, observation)
					}
				}
				m.pollBackupTasks(context.Background(), "site", client)
				tasks := project(label)
				if len(tasks) != 3 {
					t.Errorf("%s: task identity gap erased or added tasks: %+v", label, tasks)
				}
				found := false
				for _, task := range tasks {
					if task.Instance == "other" {
						require.Equal(t, other, task)
						continue
					}
					if task.VMID == 200 {
						require.Equal(t, "OK", task.Status)
						require.Equal(t, "pve-b", task.Node)
						continue
					}
					if task.VMID != 100 || task.ID != "site-"+primaryUPID || task.Status != "ERROR: archive unavailable" || task.Node != "pve-a" {
						t.Errorf("%s: incomplete task identity replaced failure/provenance: %+v", label, task)
					}
					found = true
					if i < 3 {
						for _, before := range initial {
							if before.Instance == "site" && before.VMID == 100 && !before.ObservedAt.Equal(task.ObservedAt) {
								t.Errorf("%s: invalid live read renewed observation time", label)
							}
						}
					}
				}
				if !found {
					t.Errorf("%s: original failed guest task disappeared", label)
				}
			}
			phase.Store(7)
			m.pollBackupTasks(context.Background(), "site", client)
			for _, task := range project("recovered") {
				if task.Instance == "site" && task.VMID == 100 {
					require.Equal(t, "OK", task.Status)
					require.Equal(t, "site-"+primaryUPID, task.ID)
				}
			}
		})
	}
}
