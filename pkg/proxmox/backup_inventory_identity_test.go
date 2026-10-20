package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Independent recent/active GETs may advance completion, not move the same
// opaque execution to a different guest, process, start time or owner.
func TestBackupTaskInventoryOverlappingExecutionIdentity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*Task)
		conflict bool
	}{
		{"identical", func(*Task) {}, false},
		{"implicit-node", func(t *Task) { t.Node = "" }, false},
		{"status-progress", func(t *Task) { t.Status = "running" }, false},
		{"completion-progress", func(t *Task) { t.Status = "OK"; t.EndTime = 1700000060 }, false},
		{"guest", func(t *Task) { t.ID = "101" }, true},
		{"scheduled-parent", func(t *Task) { t.ID = "" }, true},
		{"pid", func(t *Task) { t.PID++ }, true},
		{"process-start", func(t *Task) { t.PStart++ }, true},
		{"start-time", func(t *Task) { t.StartTime++ }, true},
		{"user", func(t *Task) { t.User = "other@pve" }, true},
	} {
		for _, index := range []int{0, 199} {
			t.Run(fmt.Sprintf("%s/recent-%d", tc.name, index), func(t *testing.T) {
				recent := Task{UPID: "shared-execution", Node: "node", PID: 17, PStart: 5, StartTime: 1700000000, Type: "vzdump", ID: "100", User: "fixture@pve"}
				active := recent
				tc.change(&active)
				reads, audits := 0, 0
				c := backupTestClient(t, "https://fixture.invalid")
				c.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
					require.Equal(t, http.MethodGet, r.Method)
					reads++
					var data any
					switch r.URL.Path {
					case "/api2/json/nodes":
						data = []Node{{Node: "node", Status: "online"}}
					case "/api2/json/nodes/node/tasks":
						require.Equal(t, "vzdump", r.URL.Query().Get("typefilter"))
						require.Equal(t, "200", r.URL.Query().Get("limit"))
						switch r.URL.Query().Get("source") {
						case "all":
							tasks := make([]Task, 200)
							for i := range tasks {
								tasks[i] = Task{UPID: fmt.Sprint("independent-", i), Type: "vzdump", ID: "200", Status: "OK"}
							}
							tasks[index] = recent
							data = tasks
						case "active":
							data = []Task{{UPID: "independent-active", Type: "vzdump", ID: "200"}, active}
						default:
							t.Fatalf("unexpected source: %s", r.URL)
						}
					case "/api2/json/access/permissions":
						audits++
						data = map[string]any{"/nodes/node": map[string]int{"Sys.Audit": 1}}
					default:
						t.Fatalf("unexpected read: %s", r.URL)
					}
					b, err := json.Marshal(map[string]any{"data": data})
					require.NoError(t, err)
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
				})
				got, err := c.GetBackupTaskInventory(context.Background())
				if tc.conflict {
					require.ErrorContains(t, err, "identity")
					require.Equal(t, BackupTaskInventory{}, got, "no prefix or arbitrary identity winner has replacement authority")
					require.Zero(t, audits, "permissions cannot reconcile contradictory identities")
					require.Equal(t, 3, reads, "no retry or wider inventory query")
				} else {
					require.NoError(t, err)
					require.Empty(t, got.UnconfirmedNodes)
					require.Len(t, got.Tasks, 201, "independent writers and terminal rows survive with one shared execution")
					active.Node = "node"
					require.Equal(t, active, got.Tasks[1], "later completion evidence is preserved, not frozen")
					require.Equal(t, 1, audits)
					require.Equal(t, 4, reads)
				}
			})
		}
	}
}
