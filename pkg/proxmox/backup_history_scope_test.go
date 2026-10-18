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

// Synthetic HTTP only: no guest, backup, QGA, database or installed operation.
// A foreign node label cannot turn a page (even a deduplicated row) into
// complete history or supply a task that can replace live execution evidence.
func TestBackupHistoryRejectsForeignNodePages(t *testing.T) {
	for _, later := range []bool{false, true} {
		for _, duplicate := range []bool{false, true} {
			if duplicate && !later {
				continue
			}
			t.Run(fmt.Sprintf("later-%t/duplicate-%t", later, duplicate), func(t *testing.T) {
				var badNodeAudits int
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api2/json/nodes":
						fmt.Fprint(w, `{"data":[{"node":"pve-a","status":"online"},{"node":"pve-b","status":"online"}]}`)
					case "/api2/json/nodes/pve-a/tasks":
						if later && r.URL.Query().Get("start") == "0" {
							tasks := make([]Task, 200)
							for i := range tasks {
								tasks[i] = Task{UPID: fmt.Sprintf("prior-%d", i), Node: "pve-a", Type: "vzdump", ID: "100", Status: "ERROR: retained failure"}
							}
							json.NewEncoder(w).Encode(map[string]any{"data": tasks})
							return
						}
						foreign := "foreign"
						if duplicate {
							foreign = "prior-0"
						}
						// Reject the whole page, not whichever prefix preceded
						// the foreign row. Do this before cross-page deduplication.
						fmt.Fprintf(w, `{"data":[{"upid":"page-prefix","node":"pve-a","type":"vzdump","id":"100","status":"OK"},{"upid":%q,"node":"pve-b","type":"vzdump","id":"100","status":"OK"}]}`, foreign)
					case "/api2/json/nodes/pve-b/tasks":
						fmt.Fprint(w, `{"data":[{"upid":"independent","type":"vzdump","id":"200","status":"OK"}]}`)
					case "/api2/json/access/permissions":
						if r.URL.Query().Get("path") == "/nodes/pve-a" {
							badNodeAudits++
						}
						fmt.Fprintf(w, `{"data":{%q:{"Sys.Audit":1}}}`, r.URL.Query().Get("path"))
					default:
						t.Errorf("unexpected operation %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Now().Add(-time.Hour))
				require.NoError(t, err, "a failed node page does not discard independently observed history")
				require.Len(t, history.Nodes, 2)
				require.Equal(t, BackupTaskHistoryNode{Node: "pve-a", Reason: "collection_failed"}, history.Nodes[0])
				require.Equal(t, BackupTaskHistoryNode{Node: "pve-b", Complete: true}, history.Nodes[1])
				want := 1
				if later {
					want += 200
				}
				require.Len(t, history.Tasks, want)
				for _, task := range history.Tasks {
					if task.UPID == "independent" {
						require.Equal(t, "pve-b", task.Node, "missing redundant node is supplied from the observed request scope")
					} else {
						require.True(t, strings.HasPrefix(task.UPID, "prior-"))
						require.Equal(t, "pve-a", task.Node)
						require.Equal(t, "ERROR: retained failure", task.Status)
					}
				}
				require.Zero(t, badNodeAudits, "audit authority cannot repair a foreign page")
			})
		}
	}
}
