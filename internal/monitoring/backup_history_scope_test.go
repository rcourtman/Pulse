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

// Exercise both production HTTP readers, the ordinary history/live merge,
// collected state and JSON consumed by Coverage. The manager has no persistence:
// no database is opened, no durable-write success is claimed, and no guest,
// backup, QGA, native install or stopped operation is exercised.
func TestPVEBackupHistoryForeignNodeCannotReplaceLiveFailure(t *testing.T) {
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
					task := proxmox.Task{UPID: primaryUPID, Node: "pve-a", Type: "vzdump", ID: "100", Status: "ERROR: archive unavailable", StartTime: 1792350000, EndTime: 1792350060}
					if strings.Contains(r.URL.Path, "/pve-b/") {
						task.UPID, task.Node, task.ID, task.Status = "UPID:pve-b:00000200:00000001:67000000:vzdump:200:fixture@pve:", "pve-b", "200", "OK"
					} else if r.URL.Query().Has("since") {
						switch phase.Load() {
						case 1:
							// Same UPID as the live failed task, but contradictory
							// origin. It must not become a live OK task on pve-b.
							task.Node, task.Status = "pve-b", "OK"
						case 2:
							// An ordinary later, correctly scoped history reply may
							// still update the live task's outcome.
							task.Status = "OK"
						case 3:
							// PVE may omit the redundant node field. Bind it to
							// this request rather than inventing missing evidence.
							task.Node, task.Status = "", "OK"
						}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{task}})
				default:
					t.Errorf("unexpected operation %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second}
			var client PVEClientInterface
			if cluster {
				client = proxmox.NewClusterClient("history-scope", cfg, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(cfg)
				require.NoError(t, err)
			}
			m := &Monitor{state: models.NewState(), recoveryManager: recoverymanager.New(nil)}
			other := models.BackupTask{ID: "other-instance-task", Instance: "other", Node: "pve-a", VMID: 100, Type: "vzdump", Status: "OK", StartTime: time.Unix(1792350000, 0), EndTime: time.Unix(1792350060, 0), ObservedAt: time.Unix(1792350120, 0)}
			m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{other})
			for _, step := range []struct {
				phase    int32
				label    string
				complete bool
				status   string
			}{{0, "initial", true, "ERROR: archive unavailable"}, {1, "foreign", false, "ERROR: archive unavailable"}, {2, "recovered", true, "OK"}, {3, "implicit-node", true, "OK"}} {
				phase.Store(step.phase)
				_, observation, err := m.readBackupTaskHistory(context.Background(), "site", client)
				require.NoError(t, err)
				if observation.Complete != step.complete {
					t.Errorf("%s collection complete=%t, want %t", step.label, observation.Complete, step.complete)
				}
				if step.phase == 1 && observation.Reason != "collection_failed" {
					t.Errorf("foreign page reason=%q, want collection_failed", observation.Reason)
				}
				m.pollBackupTasks(context.Background(), "site", client)
				snapshot := m.state.GetSnapshot().PVEBackups
				require.Len(t, snapshot.BackupTasks, 3, "independent tasks cannot disappear during history admission")
				for _, task := range snapshot.BackupTasks {
					switch {
					case task.Instance == "other":
						require.Equal(t, other, task, "same VMID in another connection is unchanged")
					case task.VMID == 100:
						if task.Status != step.status || task.Node != "pve-a" || task.ID != "site-"+primaryUPID {
							t.Errorf("%s: foreign history replaced live result/provenance: %+v", step.label, task)
						}
					case task.VMID == 200:
						require.Equal(t, "OK", task.Status)
						require.Equal(t, "pve-b", task.Node)
					default:
						t.Errorf("unexpected task %+v", task)
					}
				}
				wire, err := json.Marshal(map[string]any{"data": snapshot})
				require.NoError(t, err)
				t.Logf("HISTORY_SCOPE_%s_%s=%s", name, step.label, wire)
			}
		})
	}
}
