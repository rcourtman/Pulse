package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupTaskInventoryScopesOfflineNodes(t *testing.T) {
	for _, state := range []string{"offline", "unknown", ""} {
		t.Run(state, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprintf(w, `{"data":[{"node":"a","status":"online"},{"node":"b","status":%q}]}`, state)
				case "/api2/json/nodes/a/tasks":
					requests++
					require.Equal(t, "all", r.URL.Query().Get("source"))
					require.Equal(t, "200", r.URL.Query().Get("limit"))
					require.Equal(t, "vzdump", r.URL.Query().Get("typefilter"))
					fmt.Fprint(w, `{"data":[{"upid":"job","type":"vzdump","id":"100","status":"OK"}]}`)
				default:
					t.Errorf("unread node probed: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			c, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"})
			require.NoError(t, err)
			result, err := c.GetBackupTaskInventory(context.Background())
			require.NoError(t, err)
			require.Equal(t, []string{"b"}, result.OfflineNodes)
			require.Len(t, result.Tasks, 1)
			require.Equal(t, "a", result.Tasks[0].Node)
			require.Equal(t, 1, requests)
			tasks, err := c.GetBackupTasks(context.Background())
			require.Error(t, err, "legacy all-or-nothing callers cannot erase an unread scope")
			require.Nil(t, tasks)
		})
	}
}

func TestBackupTaskInventoryRejectsUnobservedOrMisattributedScopes(t *testing.T) {
	for _, tc := range []struct{ name, nodes, tasks string }{
		{"no-nodes", `[]`, `[]`},
		{"wrong-task-node", `[{"node":"a","status":"online"}]`, `[{"upid":"job","node":"b","type":"vzdump"}]`},
		{"failed-online-node", `[{"node":"a","status":"online"},{"node":"b","status":"online"}]`, `[{"upid":"job","node":"a","type":"vzdump"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprintf(w, `{"data":%s}`, tc.nodes)
				case "/api2/json/nodes/a/tasks":
					fmt.Fprintf(w, `{"data":%s}`, tc.tasks)
				case "/api2/json/nodes/b/tasks":
					w.WriteHeader(http.StatusServiceUnavailable)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			c, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"})
			require.NoError(t, err)
			got, err := c.GetBackupTaskInventory(context.Background())
			require.Error(t, err)
			require.Empty(t, got.Tasks, "failed collection must not leak a prefix as authoritative replacement")
			require.Empty(t, got.OfflineNodes)
		})
	}
}
