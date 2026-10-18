package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Task identity is required even when the transport/envelope is complete.
// Dropping a row is not a successful empty list, and duplicate executions must
// not acquire whichever contradictory outcome a downstream map keeps last.
func TestTaskListsRequireUnambiguousIdentity(t *testing.T) {
	for _, body := range []struct{ name, rows string }{
		{"missing-upid", `{"type":"vzdump","id":"100","status":"PRIVATE_FIXTURE"}`},
		{"null-upid", `{"upid":null,"type":"vzdump"}`},
		{"empty-upid", `{"upid":"","type":"vzdump"}`},
		{"blank-upid", `{"upid":" \t","type":"vzdump"}`},
		{"missing-type", `{"upid":"broken","status":"PRIVATE_FIXTURE"}`},
		{"null-type", `{"upid":"broken","type":null}`},
		{"empty-type", `{"upid":"broken","type":""}`},
		{"blank-type", `{"upid":"broken","type":" \t"}`},
		{"duplicate", `{"upid":"broken","type":"vzdump","status":"ERROR"},{"upid":"broken","type":"vzdump","status":"OK"}`},
		{"duplicate-identical", `{"upid":"broken","type":"vzdump"},{"upid":"broken","type":"vzdump"}`},
	} {
		for _, reader := range []string{"node", "live", "history"} {
			t.Run(reader+"/"+body.name, func(t *testing.T) {
				audits := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api2/json/nodes":
						fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
					case "/api2/json/nodes/node/tasks":
						fmt.Fprintf(w, `{"data":[{"upid":"valid-prefix","type":"vzdump","status":"OK"},%s]}`, body.rows)
					case "/api2/json/access/permissions":
						audits++
						fmt.Fprint(w, `{"data":{"/nodes/node":{"Sys.Audit":1}}}`)
					default:
						t.Errorf("unexpected read %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				client := backupTestClient(t, server.URL)
				var tasks []Task
				var err error
				switch reader {
				case "node":
					tasks, err = client.GetNodeTasks(context.Background(), "node")
				case "live":
					var got BackupTaskInventory
					got, err = client.GetBackupTaskInventory(context.Background())
					tasks = got.Tasks
				case "history":
					var got BackupTaskHistory
					got, err = client.GetBackupTaskHistory(context.Background(), time.Now().Add(-time.Hour))
					tasks = got.Tasks
					require.NoError(t, err, "independently readable nodes may still supply evidence")
					require.Equal(t, []BackupTaskHistoryNode{{Node: "node", Reason: "collection_failed"}}, got.Nodes)
				}
				if reader != "history" {
					require.Error(t, err)
				}
				require.Empty(t, tasks, "a malformed identity must not publish the valid prefix")
				if err != nil {
					require.NotContains(t, err.Error(), "PRIVATE_FIXTURE")
				}
				require.Zero(t, audits, "audit permission cannot repair unidentified execution evidence")
			})
		}
	}
}

func TestTaskIdentityPreservesSupportedOptionalFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case "/api2/json/access/permissions":
			fmt.Fprint(w, `{"data":{"/nodes/node":{"Sys.Audit":1}}}`)
		default:
			fmt.Fprint(w, `{"data":[{"upid":"running","type":"vzdump","metadata":{"future":true}},{"upid":"other","type":"qmstart"}]}`)
		}
	}))
	defer server.Close()
	client := backupTestClient(t, server.URL)
	tasks, err := client.GetNodeTasks(context.Background(), "node")
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	live, err := client.GetBackupTaskInventory(context.Background())
	require.NoError(t, err)
	require.Len(t, live.Tasks, 1)
	task := live.Tasks[0]
	require.Equal(t, "running", task.UPID)
	require.Equal(t, "node", task.Node)
	require.Empty(t, task.ID, "scheduled parent jobs have no guest ID")
	require.Empty(t, task.Status)
	require.Zero(t, task.EndTime)
	history, err := client.GetBackupTaskHistory(context.Background(), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, live.Tasks, history.Tasks)
	require.Equal(t, []BackupTaskHistoryNode{{Node: "node", Complete: true}}, history.Nodes)
}

func TestBackupHistoryIdentityGapKeepsEarlierPagesAndPeer(t *testing.T) {
	for _, bad := range []string{`{"type":"vzdump"}`, `{"upid":"repeat","type":"vzdump","status":"OK"},{"upid":"repeat","type":"vzdump","status":"ERROR"}`} {
		t.Run(fmt.Sprint(len(bad)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"node","status":"online"},{"node":"peer","status":"online"}]}`)
				case "/api2/json/nodes/node/tasks":
					if r.URL.Query().Get("start") == "0" {
						tasks := make([]Task, 200)
						for i := range tasks {
							tasks[i] = Task{UPID: fmt.Sprint("prior-", i), Type: "vzdump", Status: "ERROR"}
						}
						json.NewEncoder(w).Encode(map[string]any{"data": tasks})
						return
					}
					fmt.Fprintf(w, `{"data":[{"upid":"invalid-page-prefix","type":"vzdump","status":"OK"},%s]}`, bad)
				case "/api2/json/nodes/peer/tasks":
					fmt.Fprint(w, `{"data":[{"upid":"independent","type":"vzdump","status":"OK"}]}`)
				case "/api2/json/access/permissions":
					require.Equal(t, "/nodes/peer", r.URL.Query().Get("path"))
					fmt.Fprint(w, `{"data":{"/nodes/peer":{"Sys.Audit":1}}}`)
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Now().Add(-time.Hour))
			require.NoError(t, err)
			require.Equal(t, []BackupTaskHistoryNode{{Node: "node", Reason: "collection_failed"}, {Node: "peer", Complete: true}}, history.Nodes)
			require.Len(t, history.Tasks, 201)
			for _, task := range history.Tasks {
				require.True(t, strings.HasPrefix(task.UPID, "prior-") || task.UPID == "independent")
			}
		})
	}
}
