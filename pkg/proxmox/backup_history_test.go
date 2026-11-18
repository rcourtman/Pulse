package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	}{
		{"full", true, false, false, 201, ""},
		{"permission", false, false, false, 201, "audit_permission_unconfirmed"},
		{"denied", true, true, false, 0, "access_denied"},
		{"repeated-page", true, false, true, 200, "collection_failed"},
	} {
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

func TestBackupHistoryQualifiesOnlyDisjointPages(t *testing.T) {
	taskAt := func(i int) Task {
		return Task{UPID: fmt.Sprintf("task-%d", i), Type: "vzdump", ID: "101", StartTime: 1700000000}
	}
	page := func(start, count int) []Task {
		rows := make([]Task, count)
		for i := range rows {
			rows[i] = taskAt(start + i)
		}
		return rows
	}
	// A completion for a UPID already seen running must not replace that
	// observation. A new failure on the very same page is still evidence.
	failed := taskAt(200)
	failed.EndTime, failed.Status = 1700000060, "ERROR: fixture backup failed"
	later := taskAt(0)
	later.EndTime, later.Status = 1700000060, "OK"
	beyondOverlap := taskAt(399)
	beyondOverlap.EndTime, beyondOverlap.Status = 1700000120, "ERROR: later fixture backup failed"
	fullMixed := func() []Task { return append([]Task{failed, later}, page(201, 198)...) }
	for _, tc := range []struct {
		name     string
		tail     func(int) []Task
		offsets  []int
		count    int
		failures map[int]Task
		complete bool
		reason   string
		denyAt   int
	}{
		{
			name: "disjoint-short-tail", tail: func(int) []Task { return []Task{failed} },
			offsets: []int{0, 200}, count: 201, failures: map[int]Task{200: failed}, complete: true,
		},
		{
			name: "disjoint-empty-tail", tail: func(int) []Task { return page(200, 0) },
			offsets: []int{0, 200}, count: 200, complete: true,
		},
		{
			name: "short-repeat", tail: func(int) []Task { return page(199, 1) },
			offsets: []int{0, 200}, count: 200, reason: "collection_failed",
		},
		{
			name: "short-mixed-overlap", tail: func(int) []Task { return []Task{later, failed} },
			offsets: []int{0, 200}, count: 201, failures: map[int]Task{200: failed}, reason: "collection_failed",
		},
		{
			name: "full-mixed-overlap",
			tail: func(offset int) []Task {
				if offset == 200 {
					return fullMixed()
				}
				return []Task{beyondOverlap}
			},
			offsets: []int{0, 200, 400}, count: 400, failures: map[int]Task{200: failed, 399: beyondOverlap}, reason: "collection_failed",
		},
		{
			name: "full-mixed-then-denied", tail: func(int) []Task { return fullMixed() },
			offsets: []int{0, 200, 400}, reason: "access_denied", denyAt: 400,
		},
		{
			name: "non-adjacent-overlap",
			tail: func(offset int) []Task {
				if offset == 200 {
					return page(200, 200)
				}
				return []Task{taskAt(0), taskAt(400)}
			},
			offsets: []int{0, 200, 400}, count: 401, reason: "collection_failed",
		},
		{
			name: "mixed-overlap-at-cap",
			tail: func(offset int) []Task {
				if offset == 5800 {
					return append(page(0, 1), page(offset, 199)...)
				}
				return page(offset, 200)
			},
			count: 5999, reason: "collection_failed",
		},
		{
			name: "earlier-overlap-stays-partial-at-cap",
			tail: func(offset int) []Task {
				if offset == 200 {
					return append([]Task{later}, page(offset, 199)...)
				}
				return page(offset-1, 200)
			},
			count: 5999, reason: "collection_failed",
		},
		{
			name: "real-cap", tail: func(offset int) []Task { return page(offset, 200) },
			count: 6000, reason: "history_truncated",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := backupTestClient(t, "http://127.0.0.1:18080")
			var offsets []int
			var cutoff string
			auditReads := 0
			client.httpClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				var data any
				switch req.URL.Path {
				case "/api2/json/nodes":
					data = []map[string]string{{"node": "member", "status": "online"}}
				case "/api2/json/nodes/member/tasks":
					query := req.URL.Query()
					require.Equal(t, "200", query.Get("limit"))
					require.Equal(t, "1700000000", query.Get("since"))
					require.Equal(t, "vzdump", query.Get("typefilter"))
					require.Equal(t, "all", query.Get("source"))
					require.NotEmpty(t, query.Get("until"))
					if cutoff == "" {
						cutoff = query.Get("until")
					}
					require.Equal(t, cutoff, query.Get("until"))
					offset, err := strconv.Atoi(query.Get("start"))
					require.NoError(t, err)
					offsets = append(offsets, offset)
					if tc.denyAt > 0 && offset == tc.denyAt {
						return &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
					}
					if offset == 0 {
						data = page(0, 200)
					} else {
						data = tc.tail(offset)
					}
				case "/api2/json/access/permissions":
					auditReads++
					require.Equal(t, "/nodes/member", req.URL.Query().Get("path"))
					data = map[string]any{"/nodes/member": map[string]int{"Sys.Audit": 1}}
				default:
					t.Fatalf("unexpected fixture read: %s", req.URL.Path)
				}
				body, err := json.Marshal(map[string]any{"data": data})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
			})}
			history, err := client.GetBackupTaskHistory(context.Background(), time.Unix(1700000000, 0))
			require.NoError(t, err)
			assert.Equal(t, []BackupTaskHistoryNode{{Node: "member", Complete: tc.complete, Reason: tc.reason}}, history.Nodes)
			assert.Len(t, history.Tasks, tc.count)
			for i, task := range history.Tasks {
				want := taskAt(i)
				if failed, ok := tc.failures[i]; ok {
					want = failed
				}
				want.Node = "member"
				require.Equal(t, want, task, "keep distinct rows in order without promoting the repeated UPID")
			}
			if tc.complete {
				require.Equal(t, 1, auditReads)
			} else {
				require.Zero(t, auditReads, "unread/capped paging cannot be qualified by audit permission")
			}
			if tc.offsets != nil {
				require.Equal(t, tc.offsets, offsets)
			} else {
				require.Len(t, offsets, 30)
				for i, offset := range offsets {
					require.Equal(t, i*200, offset)
				}
			}
		})
	}
}
