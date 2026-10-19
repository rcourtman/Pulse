package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// No listener or provider: the older running task is outside the ordinary
// newest-200 view, but still writing its archive. A full recent page is not a
// census of active writers, even with Sys.Audit.
func TestBackupTaskInventoryIncludesRunningBeyondRecentWindow(t *testing.T) {
	reads := []string{}
	c := backupTestClient(t, "https://fixture.invalid")
	c.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		reads = append(reads, r.URL.RequestURI())
		body := ""
		switch r.URL.Path {
		case "/api2/json/nodes":
			body = `{"data":[{"node":"node","status":"online"}]}`
		case "/api2/json/access/permissions":
			body = `{"data":{"/nodes/node":{"Sys.Audit":1}}}`
		case "/api2/json/nodes/node/tasks":
			require.Equal(t, "vzdump", r.URL.Query().Get("typefilter"))
			require.Equal(t, "200", r.URL.Query().Get("limit"))
			if r.URL.Query().Get("source") == "active" {
				body = `{"data":[{"upid":"older-running","type":"vzdump","id":"100","starttime":1700000000}]}`
			} else {
				require.Equal(t, "all", r.URL.Query().Get("source"))
				tasks := make([]Task, 200)
				for i := range tasks {
					tasks[i] = Task{UPID: fmt.Sprint("finished-", i), Type: "vzdump", ID: "101", Status: "OK", StartTime: 1700000060, EndTime: 1700000120}
				}
				b, err := json.Marshal(map[string]any{"data": tasks})
				require.NoError(t, err)
				body = string(b)
			}
		default:
			t.Fatalf("unexpected read %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	got, err := c.GetBackupTaskInventory(context.Background())
	require.NoError(t, err)
	require.Len(t, got.Tasks, 201, "recent terminal tasks must not crowd out an older active writer")
	require.Equal(t, "older-running", got.Tasks[0].UPID, "active tasks take priority in the bounded live projection")
	require.Equal(t, "node", got.Tasks[0].Node)
	require.Len(t, reads, 4, "bounded recent, active and permission reads, plus node discovery")
}

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
				case "/api2/json/access/permissions":
					require.Equal(t, "/nodes/a", r.URL.Query().Get("path"))
					fmt.Fprint(w, `{"data":{"/nodes/a":{"Sys.Audit":1}}}`)
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
			require.NoError(t, err, "the compatibility helper retains its readable-node view")
			require.Len(t, tasks, 1)
			require.Equal(t, "a", tasks[0].Node)
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
				case "/api2/json/access/permissions":
					fmt.Fprint(w, `{"data":{"/nodes/a":{"Sys.Audit":1}}}`)
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

func TestBackupTaskInventoryQualifiesOtherUserVisibility(t *testing.T) {
	for _, tc := range []struct {
		name, permission string
		status           int
		confirmed        bool
	}{
		{"audit", `{"data":{"/nodes/node":{"Sys.Audit":1}}}`, 200, true},
		{"own-only", `{"data":{"/nodes/node":{"Sys.Audit":0}}}`, 200, false},
		{"missing", `{"data":{"/nodes/node":{}}}`, 200, false},
		{"wrong-path", `{"data":{"/nodes/peer":{"Sys.Audit":1}}}`, 200, false},
		{"denied", `{"data":null}`, 403, false},
		{"ambiguous", `{"data":{"/nodes/node":{"Sys.Audit":0,"Sys.Audit":1}}}`, 200, false},
		{"trailing", `{"data":{"/nodes/node":{"Sys.Audit":1}}} {}`, 200, false},
		{"wrong-type", `{"data":{"/nodes/node":{"Sys.Audit":"1"}}}`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := backupTestClient(t, "https://fixture.invalid")
			reads := 0
			c.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, r.Method)
				body, status := "", 200
				switch r.URL.Path {
				case "/api2/json/nodes":
					body = `{"data":[{"node":"node","status":"online"}]}`
				case "/api2/json/nodes/node/tasks":
					body = `{"data":[{"upid":"visible","type":"vzdump","id":"100","status":"ERROR"}]}`
				case "/api2/json/access/permissions":
					require.Equal(t, "/nodes/node", r.URL.Query().Get("path"))
					reads++
					body, status = tc.permission, tc.status
				default:
					t.Fatalf("unexpected request %s", r.URL)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			got, err := c.GetBackupTaskInventory(context.Background())
			require.NoError(t, err, "a visibility limitation must not discard valid positive observations")
			require.Equal(t, []Task{{UPID: "visible", Node: "node", Type: "vzdump", ID: "100", Status: "ERROR"}}, got.Tasks)
			if tc.confirmed {
				require.Empty(t, got.UnconfirmedNodes)
			} else {
				require.Equal(t, []string{"node"}, got.UnconfirmedNodes)
			}
			require.Equal(t, 1, reads)
		})
	}
}

func TestBackupTaskInventoryQualifiesSaturatedActiveReads(t *testing.T) {
	for _, mode := range []string{"empty", "overlap", "denied", "null", "foreign", "bounded"} {
		t.Run(mode, func(t *testing.T) {
			c := backupTestClient(t, "https://fixture.invalid")
			audits := 0
			c.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				body, status := "", 200
				switch r.URL.Path {
				case "/api2/json/nodes":
					body = `{"data":[{"node":"node","status":"online"}]}`
				case "/api2/json/access/permissions":
					audits++
					body = `{"data":{"/nodes/node":{"Sys.Audit":1}}}`
				case "/api2/json/nodes/node/tasks":
					active := r.URL.Query().Get("source") == "active"
					if active && mode != "bounded" {
						switch mode {
						case "empty":
							body = `{"data":[]}`
						case "overlap":
							body = `{"data":[{"upid":"recent-0","type":"vzdump","id":"100","status":"running"}]}`
						case "denied":
							body, status = `{"data":null}`, 403
						case "null":
							body = `{"data":null}`
						case "foreign":
							body = `{"data":[{"upid":"foreign","type":"vzdump","node":"peer"}]}`
						}
					} else {
						tasks := make([]Task, 200)
						for i := range tasks {
							tasks[i] = Task{UPID: fmt.Sprint("recent-", i), Type: "vzdump", ID: "100", Status: "OK"}
						}
						b, err := json.Marshal(map[string]any{"data": tasks})
						require.NoError(t, err)
						body = string(b)
					}
				default:
					t.Fatalf("unexpected read %s", r.URL)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			got, err := c.GetBackupTaskInventory(context.Background())
			require.NoError(t, err)
			require.Len(t, got.Tasks, 200, "keep valid positives without duplicating overlapping executions")
			confirmed := mode == "empty" || mode == "overlap"
			require.Equal(t, confirmed, len(got.UnconfirmedNodes) == 0)
			if mode == "overlap" {
				require.Equal(t, "running", got.Tasks[0].Status, "later active evidence supersedes the earlier recent row")
			}
			if confirmed {
				require.Equal(t, 1, audits)
			} else {
				require.Zero(t, audits, "audit permission cannot repair an incomplete active census")
			}
		})
	}
}
