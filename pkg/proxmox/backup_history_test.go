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
						tasks[i] = Task{UPID: fmt.Sprintf("task-%d", i+offset), Type: "vzdump"}
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
						fmt.Fprint(w, `{"data":[{"upid":"valid","type":"vzdump","id":"100","status":"OK"}]}`)
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
