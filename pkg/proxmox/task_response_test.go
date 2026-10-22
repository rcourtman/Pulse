package proxmox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskReadsRequireCompleteResponses(t *testing.T) {
	for _, read := range []string{"node", "backup", "log"} {
		for _, bad := range []string{"truncated", "trailing", "missing", "null", "case-envelope", "duplicate-data", "error-envelope", "duplicate-row", "case-row", "invalid-utf8", "oversized"} {
			t.Run(read+"/"+bad, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api2/json/nodes" {
						fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"}]}`)
						return
					}
					row := `{"endtime":1700000060,"starttime":1700000000,"upid":"task","type":"vzdump","status":"OK"}`
					if read == "log" {
						row = `{"n":1,"t":"INFO: Finished Backup of VM 100 (00:01:00)"}`
					}
					body := `{"data":[` + row + `]}`
					switch bad {
					case "truncated":
						w.Header().Set("Content-Length", "999999")
					case "trailing":
						body += `{"data":[]}`
					case "missing":
						body = `{}`
					case "null":
						body = `{"data":null}`
					case "case-envelope":
						body = `{"Data":[]}`
					case "duplicate-data":
						body = `{"data":[` + row + `],"data":[]}`
					case "error-envelope":
						body = `{"data":[` + row + `],"errors":"PRIVATE_FIXTURE_ERROR"}`
					case "duplicate-row":
						if read == "log" {
							body = `{"data":[{"n":1,"t":"ERROR: failed","t":"INFO: Finished Backup of VM 100 (00:01:00)"}]}`
						} else {
							body = `{"data":[{"endtime":1700000060,"starttime":1700000000,"upid":"task","type":"vzdump","status":"ERROR","status":"OK"}]}`
						}
					case "case-row":
						if read == "log" {
							body = `{"data":[{"n":1,"t":"ERROR: failed","T":"INFO: Finished Backup of VM 100 (00:01:00)"}]}`
						} else {
							body = `{"data":[{"endtime":1700000060,"starttime":1700000000,"upid":"task","type":"vzdump","status":"ERROR","Status":"OK"}]}`
						}
					case "invalid-utf8":
						body = strings.Replace(body, "}", ",\"metadata\":\"\xff\"}", 1)
					case "oversized":
						fmt.Fprint(w, body)
						w.(http.Flusher).Flush()
						fmt.Fprint(w, strings.Repeat(" ", int(maxResponseBodyBytes)))
						return
					}
					fmt.Fprint(w, body)
				}))
				defer server.Close()
				c := backupTestClient(t, server.URL)
				c.httpClient.Timeout = time.Minute // body-bound test, not a latency assertion
				var err error
				var count int
				switch read {
				case "node":
					v, e := c.GetNodeTasks(context.Background(), "pve1")
					err = e
					count = len(v)
				case "backup":
					v, e := c.GetBackupTasks(context.Background())
					err = e
					count = len(v)
				case "log":
					v, e := c.GetTaskLog(context.Background(), "pve1", "task")
					err = e
					count = len(v)
				}
				if err == nil || count != 0 {
					t.Errorf("unsafe task response accepted: rows=%d error=%v", count, err)
				}
				if err != nil && strings.Contains(err.Error(), "PRIVATE_FIXTURE_ERROR") {
					t.Error("successful-response error payload escaped diagnostics")
				}
			})
		}
	}
}

func TestTaskResponseCompletionAndLimits(t *testing.T) {
	t.Run("prefix-waits-for-cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":[]}`)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		tasks, err := backupTestClient(t, server.URL).GetNodeTasks(ctx, "pve1")
		if err == nil || len(tasks) != 0 {
			t.Fatalf("unfinished prefix published: %+v %v", tasks, err)
		}
	})
	t.Run("completed-empty-and-running", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api2/json/nodes":
				fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"},{"node":"offline","status":"offline"}]}`)
			case "/api2/json/nodes/pve1/tasks":
				fmt.Fprint(w, `{"data":[{"starttime":1700000000,"upid":"running","type":"vzdump","metadata":1},{"starttime":1700000000,"upid":"other","type":"qmstart"}],"total":2,"errors":null}`)
			default:
				fmt.Fprint(w, `{"data":[],"total":0}`)
			}
		}))
		defer server.Close()
		c := backupTestClient(t, server.URL)
		tasks, err := c.GetBackupTasks(context.Background())
		if err != nil || len(tasks) != 1 || tasks[0].UPID != "running" || tasks[0].EndTime != 0 || tasks[0].Status != "" {
			t.Fatalf("running/additive/filter/offline semantics changed: %+v %v", tasks, err)
		}
		lines, err := c.GetTaskLog(context.Background(), "pve1", "task")
		if err != nil || len(lines) != 0 {
			t.Fatalf("completed empty log rejected: %+v %v", lines, err)
		}
	})
	t.Run("bounded-log-cannot-be-final", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":[`)
			for i := 0; i < taskLogLineLimit; i++ {
				if i > 0 {
					fmt.Fprint(w, ",")
				}
				fmt.Fprintf(w, `{"n":%d,"t":"fixture"}`, i+1)
			}
			fmt.Fprint(w, `]}`)
		}))
		defer server.Close()
		lines, err := backupTestClient(t, server.URL).GetTaskLog(context.Background(), "pve1", "task")
		if err == nil || lines != nil {
			t.Fatalf("bounded log prefix accepted as complete: %d %v", len(lines), err)
		}
	})
}

type taskCloseFailure struct{ io.Reader }

func (taskCloseFailure) Close() error { return errors.New("fixture close failure") }
func TestTaskRowsCloseFailureIsNotCompletion(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Body: taskCloseFailure{strings.NewReader(`{"data":[]}`)}}
	if rows, err := readTasks(resp); err == nil || rows != nil {
		t.Fatalf("close failure published: %+v %v", rows, err)
	}
}

// Whole-response admission, not a provider/guest operation. Running tasks may
// omit status/endtime; terminal verdicts need a coherent, completed interval.
func TestTaskLifecycleAdmission(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any)
		valid  bool
	}{
		{"completed", func(map[string]any) {}, true},
		{"last-json-year", func(r map[string]any) { r["starttime"] = int64(253402300798); r["endtime"] = int64(253402300799) }, true},
		{"year-ten-thousand-start", func(r map[string]any) { r["starttime"] = int64(253402300800); r["endtime"] = int64(253402300860) }, false},
		{"year-ten-thousand-end", func(r map[string]any) { r["endtime"] = int64(253402300800) }, false},
		{"millisecond-start", func(r map[string]any) { r["starttime"] = int64(1700000000000); r["endtime"] = int64(1700000060000) }, false},
		{"millisecond-end", func(r map[string]any) { r["endtime"] = int64(1700000060000) }, false},
		{"max-int64-start", func(r map[string]any) { r["starttime"] = int64(1<<63 - 1); r["endtime"] = int64(1<<63 - 1) }, false},
		{"max-int64-end", func(r map[string]any) { r["endtime"] = int64(1<<63 - 1) }, false},
		{"zero-duration", func(r map[string]any) { r["endtime"] = r["starttime"] }, true},
		{"failed", func(r map[string]any) { r["status"] = "ERROR: fixture failure" }, true},
		{"warnings", func(r map[string]any) { r["status"] = "WARNINGS: 1" }, true},
		{"active-omitted", func(r map[string]any) { delete(r, "status"); delete(r, "endtime") }, true},
		{"active-explicit", func(r map[string]any) { r["status"] = "running"; r["endtime"] = 0 }, true},
		{"active-null-end", func(r map[string]any) { r["status"] = ""; r["endtime"] = nil }, true},
		{"missing-start", func(r map[string]any) { delete(r, "starttime") }, false},
		{"null-start", func(r map[string]any) { r["starttime"] = nil }, false},
		{"zero-start", func(r map[string]any) { r["starttime"] = 0 }, false},
		{"negative-start", func(r map[string]any) { r["starttime"] = -1 }, false},
		{"negative-end", func(r map[string]any) { r["endtime"] = -1 }, false},
		{"end-before-start", func(r map[string]any) { r["endtime"] = 1699999999 }, false},
		{"running-with-end", func(r map[string]any) { r["status"] = "running" }, false},
		{"spaced-running-with-end", func(r map[string]any) { r["status"] = " Running " }, false},
		{"missing-result-with-end", func(r map[string]any) { delete(r, "status") }, false},
		{"null-result-with-end", func(r map[string]any) { r["status"] = nil }, false},
		{"empty-result-with-end", func(r map[string]any) { r["status"] = " " }, false},
		{"success-without-end", func(r map[string]any) { delete(r, "endtime") }, false},
		{"success-null-end", func(r map[string]any) { r["endtime"] = nil }, false},
		{"success-zero-end", func(r map[string]any) { r["endtime"] = 0 }, false},
		{"failure-without-end", func(r map[string]any) { r["status"] = "ERROR: fixture failure"; delete(r, "endtime") }, false},
	}
	for _, tc := range cases {
		for _, reader := range []string{"node", "live", "active", "history"} {
			t.Run(tc.name+"/"+reader, func(t *testing.T) {
				row := map[string]any{"upid": "current", "type": "vzdump", "id": "100", "starttime": 1700000000, "status": "OK", "endtime": 1700000060, "metadata": "additive"}
				tc.change(row)
				audits := 0
				c := backupTestClient(t, "https://fixture.invalid")
				c.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
					require.Equal(t, http.MethodGet, r.Method)
					var body []byte
					switch r.URL.Path {
					case "/api2/json/nodes":
						body = []byte(`{"data":[{"node":"node","status":"online"}]}`)
					case "/api2/json/access/permissions":
						audits++
						body = []byte(`{"data":{"/nodes/node":{"Sys.Audit":1}}}`)
					case "/api2/json/nodes/node/tasks":
						rows := []any{map[string]any{"upid": "valid-prefix", "type": "vzdump", "id": "101", "starttime": 1699900000, "status": "OK", "endtime": 1699900060}, row}
						if reader == "active" && r.URL.Query().Get("source") == "all" {
							rows = make([]any, 200)
							for i := range rows {
								rows[i] = Task{UPID: fmt.Sprint("recent-", i), Type: "vzdump", ID: "101", StartTime: 1699900000, Status: "OK", EndTime: 1699900060}
							}
						}
						var err error
						body, err = json.Marshal(map[string]any{"data": rows})
						require.NoError(t, err)
					default:
						t.Fatalf("unexpected read %s", r.URL)
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
				})
				switch reader {
				case "node":
					rows, err := c.GetNodeTasks(context.Background(), "node")
					if tc.valid {
						require.NoError(t, err)
						assert.Len(t, rows, 2)
					} else {
						assert.Error(t, err)
						assert.Empty(t, rows, "bad suffix cannot publish a prefix")
					}
				case "live", "active":
					got, err := c.GetBackupTaskInventory(context.Background())
					if reader == "active" {
						require.NoError(t, err, "failed active census preserves valid recent observations")
						if tc.valid {
							assert.Empty(t, got.UnconfirmedNodes)
							assert.Len(t, got.Tasks, 202)
						} else {
							assert.Equal(t, []string{"node"}, got.UnconfirmedNodes)
							assert.Len(t, got.Tasks, 200)
							assert.Zero(t, audits)
						}
					} else if tc.valid {
						require.NoError(t, err)
						assert.Len(t, got.Tasks, 2)
						assert.Empty(t, got.UnconfirmedNodes)
					} else {
						assert.Error(t, err)
						assert.Empty(t, got.Tasks)
						assert.Zero(t, audits)
					}
				case "history":
					got, err := c.GetBackupTaskHistory(context.Background(), time.Unix(1699800000, 0))
					require.NoError(t, err, "collection quality retains node-specific failure")
					require.Len(t, got.Nodes, 1)
					assert.Equal(t, tc.valid, got.Nodes[0].Complete)
					if tc.valid {
						assert.Len(t, got.Tasks, 2)
					} else {
						assert.Empty(t, got.Tasks)
						assert.Equal(t, "collection_failed", got.Nodes[0].Reason)
						assert.Zero(t, audits)
					}
				}
			})
		}
	}
}
