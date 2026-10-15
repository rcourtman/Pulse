package proxmox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
					row := `{"upid":"task","type":"vzdump","status":"OK"}`
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
							body = `{"data":[{"upid":"task","type":"vzdump","status":"ERROR","status":"OK"}]}`
						}
					case "case-row":
						if read == "log" {
							body = `{"data":[{"n":1,"t":"ERROR: failed","T":"INFO: Finished Backup of VM 100 (00:01:00)"}]}`
						} else {
							body = `{"data":[{"upid":"task","type":"vzdump","status":"ERROR","Status":"OK"}]}`
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
				fmt.Fprint(w, `{"data":[{"upid":"running","type":"vzdump","metadata":1},{"upid":"other","type":"qmstart"}],"total":2,"errors":null}`)
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
