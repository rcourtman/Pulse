package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackupHistoryPaginationAndCollectionQuality(t *testing.T) {
	for _, test := range []struct {
		name       string
		audit      bool
		failSecond bool
		repeat     bool
		expected   int
		reason     string
	}{{"full", true, false, false, 201, ""}, {"permission", false, false, false, 201, "audit_permission_unconfirmed"}, {"denied", true, true, false, 0, "access_denied"}, {"repeated-page", true, false, true, 200, "history_truncated"}} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"node","status":"online"},{"node":"offline","status":"offline"}]}`)
				case "/api2/json/access/permissions":
					value := 0
					if test.audit {
						value = 1
					}
					fmt.Fprintf(w, `{"data":{"/nodes/node":{"Sys.Audit":%d}}}`, value)
				case "/api2/json/nodes/node/tasks":
					require.Equal(t, "vzdump", r.URL.Query().Get("typefilter"))
					require.Equal(t, "all", r.URL.Query().Get("source"))
					require.NotEmpty(t, r.URL.Query().Get("since"))
					require.NotEmpty(t, r.URL.Query().Get("until"))
					second := r.URL.Query().Get("start") == "200"
					if second && test.failSecond {
						w.WriteHeader(403)
						return
					}
					count, offset := 200, 0
					if second && !test.repeat {
						count, offset = 1, 200
					}
					tasks := make([]Task, count)
					for i := range tasks {
						tasks[i] = Task{UPID: fmt.Sprintf("task-%d", i+offset), Type: "vzdump", StartTime: 1700000000}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": tasks})
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "test@pam!token", TokenValue: "fixture", VerifySSL: false})
			require.NoError(t, err)
			history, err := client.GetBackupTaskHistory(context.Background(), time.Now().Add(-time.Hour))
			require.NoError(t, err)
			require.Len(t, history.Tasks, test.expected)
			require.Len(t, history.Nodes, 2)
			require.Equal(t, test.reason, history.Nodes[0].Reason)
			require.Equal(t, test.reason == "", history.Nodes[0].Complete)
			require.Equal(t, "node_offline", history.Nodes[1].Reason)
		})
	}
}
