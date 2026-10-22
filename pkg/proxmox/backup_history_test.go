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

func TestBackupHistoryPaginationAndCollectionQuality(t *testing.T) {
	for _, test := range []struct {
		name       string
		audit      bool
		failSecond bool
		repeat     bool
		expected   int
		reason     string
	}{{"full", true, false, false, 201, ""}, {"permission", false, false, false, 201, "audit_permission_unconfirmed"}, {"partial", true, true, false, 200, "collection_failed"}, {"repeated-page", true, false, true, 200, "history_truncated"}} {
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

func TestBackupHistoryRejectsIncompletePagesAndAuditEvidence(t *testing.T) {
	for _, test := range []struct {
		name, body, reason string
		audit, truncated   bool
	}{
		{"page-truncated-body", `{"data":[]}`, "collection_failed", false, true},
		{"page-null", `{"data":null}`, "collection_failed", false, false},
		{"page-missing", `{}`, "collection_failed", false, false},
		{"page-duplicate", `{"data":[],"data":[]}`, "collection_failed", false, false},
		{"page-trailing", `{"data":[]} {}`, "collection_failed", false, false},
		{"page-errors", `{"data":[],"errors":{"fixture":"denied"}}`, "collection_failed", false, false},
		{"page-bad-row", `{"data":[null]}`, "collection_failed", false, false},
		{"audit-truncated-body", `{"data":{"/nodes/node":{"Sys.Audit":1}}}`, "audit_permission_unconfirmed", true, true},
		{"audit-duplicate-path", `{"data":{"/nodes/node":{"Sys.Audit":0},"/nodes/node":{"Sys.Audit":1}}}`, "audit_permission_unconfirmed", true, false},
		{"audit-duplicate-privilege", `{"data":{"/nodes/node":{"Sys.Audit":0,"Sys.Audit":1}}}`, "audit_permission_unconfirmed", true, false},
		{"audit-trailing", `{"data":{"/nodes/node":{"Sys.Audit":1}}} {}`, "audit_permission_unconfirmed", true, false},
		{"audit-errors", `{"data":{"/nodes/node":{"Sys.Audit":1}},"errors":{"fixture":"denied"}}`, "audit_permission_unconfirmed", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
				case "/api2/json/nodes/node/tasks":
					if test.audit {
						fmt.Fprint(w, `{"data":[{"endtime":1700000060,"starttime":1700000000,"upid":"valid","type":"vzdump","id":"100","status":"OK"}]}`)
						return
					}
					if test.truncated {
						w.Header().Set("Content-Length", "1000")
					}
					fmt.Fprint(w, test.body)
				case "/api2/json/access/permissions":
					require.True(t, test.audit, "invalid page must not be qualified by audit authority")
					if test.truncated {
						w.Header().Set("Content-Length", "1000")
					}
					fmt.Fprint(w, test.body)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
			require.NoError(t, err)
			history, err := client.GetBackupTaskHistory(context.Background(), time.Now().Add(-time.Hour))
			require.NoError(t, err, "valid earlier observations can still be returned with incomplete quality")
			require.Len(t, history.Nodes, 1)
			require.False(t, history.Nodes[0].Complete)
			require.Equal(t, test.reason, history.Nodes[0].Reason)
			if test.audit {
				require.Len(t, history.Tasks, 1, "failed authority confirmation must not discard observed executions")
			} else {
				require.Empty(t, history.Tasks, "an incomplete page supplies no task evidence")
			}
		})
	}
}

// Offset pages need not be an atomic snapshot. Identical boundary overlap is
// useful, but a contradictory execution must not be resolved by first-wins.
func TestBackupHistoryCrossPageIdentityAdmission(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(*Task)
		conflict bool
	}{
		{"identical", func(*Task) {}, false},
		{"implicit-node", func(task *Task) { task.Node = "" }, false},
		{"status", func(task *Task) { task.Status = "ERROR: archive unavailable" }, true},
		{"guest", func(task *Task) { task.ID = "101" }, true},
		{"kind", func(task *Task) { task.Type = "qmstart" }, true},
		{"pid", func(task *Task) { task.PID++ }, true},
		{"process-start", func(task *Task) { task.PStart++ }, true},
		{"start-time", func(task *Task) { task.StartTime++ }, true},
		{"end-time", func(task *Task) { task.EndTime++ }, true},
		{"user", func(task *Task) { task.User = "other@pve" }, true},
	} {
		for _, lastPage := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/page-%d", tc.name, lastPage+1), func(t *testing.T) {
				first := Task{UPID: "overlapping-execution", Node: "node", PID: 17, PStart: 5, Type: "vzdump", ID: "100", User: "fixture@pve", Status: "OK", StartTime: 1700000000, EndTime: 1700000060}
				second := first
				tc.mutate(&second)
				var requests, audits []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method)
					requests = append(requests, r.URL.RequestURI())
					switch r.URL.Path {
					case "/api2/json/nodes":
						fmt.Fprint(w, `{"data":[{"node":"node","status":"online"},{"node":"peer","status":"online"}]}`)
					case "/api2/json/nodes/node/tasks":
						q := r.URL.Query()
						require.Equal(t, "vzdump", q.Get("typefilter"))
						require.Equal(t, "all", q.Get("source"))
						require.Equal(t, "200", q.Get("limit"))
						var rows []Task
						if q.Get("start") == fmt.Sprint(200*lastPage) {
							rows = []Task{{UPID: "unadmitted-page-prefix", Type: "vzdump", Status: "OK", StartTime: 1700000000, EndTime: 1700000060}, second}
						} else {
							for i := 0; i < 200; i++ {
								rows = append(rows, Task{UPID: fmt.Sprintf("earlier-%s-%d", q.Get("start"), i), Type: "vzdump", Status: "ERROR", StartTime: 1700000000, EndTime: 1700000060})
							}
							if q.Get("start") == "0" {
								rows[0] = first
							}
						}
						json.NewEncoder(w).Encode(map[string]any{"data": rows})
					case "/api2/json/nodes/peer/tasks":
						fmt.Fprint(w, `{"data":[{"endtime":1700000060,"starttime":1700000000,"upid":"independent-peer","type":"vzdump","id":"200","status":"OK"}]}`)
					case "/api2/json/access/permissions":
						audits = append(audits, r.URL.Query().Get("path"))
						fmt.Fprintf(w, `{"data":{%q:{"Sys.Audit":1}}}`, r.URL.Query().Get("path"))
					default:
						t.Errorf("unexpected request %s", r.URL.RequestURI())
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Unix(1699990000, 0))
				require.NoError(t, err)
				reason := ""
				if tc.conflict {
					reason = "collection_failed"
				}
				require.Equal(t, []BackupTaskHistoryNode{{Node: "node", Complete: !tc.conflict, Reason: reason}, {Node: "peer", Complete: true}}, history.Nodes)
				if tc.conflict {
					require.Len(t, history.Tasks, lastPage*200, "only the ambiguous earlier execution and the whole new page are withheld; peer remains")
					require.Equal(t, []string{"/nodes/peer"}, audits)
					for _, task := range history.Tasks {
						require.True(t, strings.HasPrefix(task.UPID, "earlier-") || task.UPID == "independent-peer", "neither version nor new page prefix may enter history")
					}
				} else {
					require.Len(t, history.Tasks, lastPage*200+2, "identical boundary overlap is deduplicated, not dropped or marked incomplete")
					require.Equal(t, []string{"/nodes/node", "/nodes/peer"}, audits)
					require.Equal(t, first, history.Tasks[0], "omitted redundant node is equivalent to the request's node")
				}
				require.Len(t, requests, lastPage+3+len(audits), "no extra retry or page may be requested")
			})
		}
	}
}

func TestBackupHistoryCrossPageConflictRemovesEveryAmbiguousExecution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case "/api2/json/nodes/node/tasks":
			var rows []Task
			if r.URL.Query().Get("start") == "0" {
				for i := 0; i < 200; i++ {
					rows = append(rows, Task{UPID: fmt.Sprint("task-", i), Type: "vzdump", Status: "OK", StartTime: 1700000000, EndTime: 1700000060})
				}
			} else {
				rows = []Task{{UPID: "task-0", Type: "vzdump", Status: "ERROR", StartTime: 1700000000, EndTime: 1700000060}, {UPID: "task-199", Type: "vzdump", ID: "101", Status: "OK", StartTime: 1700000000, EndTime: 1700000060}}
			}
			json.NewEncoder(w).Encode(map[string]any{"data": rows})
		default:
			t.Errorf("ambiguous history must not be qualified with another read: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Unix(1699990000, 0))
	require.NoError(t, err)
	require.Equal(t, []BackupTaskHistoryNode{{Node: "node", Reason: "collection_failed"}}, history.Nodes)
	require.Len(t, history.Tasks, 198)
	for _, task := range history.Tasks {
		require.NotContains(t, []string{"task-0", "task-199"}, task.UPID)
	}
}
