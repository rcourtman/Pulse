package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// These exercise the ordinary client with completed synthetic HTTP responses,
// without a listener, native replication, guest operation or persistent store.
func replicationResponseClient(t *testing.T, reply func(*http.Request) io.ReadCloser) *Client {
	t.Helper()
	c, err := NewClient(ClientConfig{Host: "http://fixture.invalid:8006", TokenName: "reader@pve!fixture", TokenValue: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	c.httpClient.Transport = replicationTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s", r.Method)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: reply(r), Request: r}, nil
	})
	return c
}

const replicationCleanInventory = `{"data":[{"id":"100-0","source":"node-a","target":"node-b","guest":100}]}`
const replicationCleanStatus = `{"last_sync":1792407600,"fail_count":0,"state":"ok"}`

func TestReplicationReadRejectsIncompleteOrAmbiguousInventory(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"trailing-document", `{"data":[]}{"data":[]}`},
		{"trailing-garbage", `{"data":[]}PRIVATE_BODY_MARKER`},
		{"duplicate-data", `{"data":[{"id":"100-0"}],"data":[]}`},
		{"case-data", `{"data":[{"id":"100-0"}],"Data":[]}`},
		{"error-envelope", `{"data":[],"errors":{"reason":"PRIVATE_BODY_MARKER"}}`},
		{"invalid-utf8", "{\"data\":[],\"metadata\":\"\xff\"}"},
		{"null-row", `{"data":[null]}`},
		{"missing-id", `{"data":[{"source":"node-a"}]}`},
		{"empty-id", `{"data":[{"id":"  "}]}`},
		{"nonscalar-id", `{"data":[{"id":{"value":"100-0"}}]}`},
		{"repeated-id", `{"data":[{"id":"100-0"},{"id":"100-0"}]}`},
		{"repeated-id-alias", `{"data":[{"id":"100-0"},{"jobid":"100-0"}]}`},
		{"duplicate-source", `{"data":[{"id":"100-0","source":"node-b","source":"node-a"}]}`},
		{"case-source", `{"data":[{"id":"100-0","source":"node-a","Source":"node-b"}]}`},
		{"contradictory-id-alias", `{"data":[{"id":"100-0","jobid":"999-0"}]}`},
		{"oversized-tail", `{"data":[]}` + strings.Repeat(" ", int(maxResponseBodyBytes))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statusReads := 0
			c := replicationResponseClient(t, func(r *http.Request) io.ReadCloser {
				if strings.HasSuffix(r.URL.Path, "/status") {
					statusReads++
					return io.NopCloser(strings.NewReader(`{"data":` + replicationCleanStatus + `}`))
				}
				return io.NopCloser(strings.NewReader(tc.body))
			})
			jobs, err := c.GetReplicationStatus(context.Background())
			if err == nil || jobs != nil || statusReads != 0 {
				t.Errorf("unsafe inventory was publishable: jobs=%+v error=%v statusReads=%d", jobs, err, statusReads)
			}
			if err != nil && strings.Contains(err.Error(), "PRIVATE_BODY_MARKER") {
				t.Error("private body copied into error")
			}
		})
	}
}

func TestReplicationReadRejectsAmbiguousStatusWithoutHidingConfig(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"duplicate-data", `{"data":{"error":"PRIVATE_BODY_MARKER"},"data":` + replicationCleanStatus + `}`},
		{"case-data", `{"data":` + replicationCleanStatus + `,"Data":{}}`},
		{"error-envelope", `{"data":` + replicationCleanStatus + `,"errors":{"reason":"PRIVATE_BODY_MARKER"}}`},
		{"duplicate-error", `{"data":{"last_sync":1792407600,"fail_count":0,"error":"PRIVATE_BODY_MARKER","error":""}}`},
		{"duplicate-fails", `{"data":{"last_sync":1792407600,"fail_count":3,"fail_count":0}}`},
		{"duplicate-source", `{"data":{"source":"foreign","source":"node-a","last_sync":1792407600,"fail_count":0}}`},
		{"contradictory-fail-alias", `{"data":{"last_sync":1792407600,"fail_count":0,"fail-count":3}}`},
		{"contradictory-outcome-alias", `{"data":{"last_sync":1792407600,"fail_count":0,"last_sync_status":"ok","last-sync-status":"failed"}}`},
		{"contradictory-state-alias", `{"data":{"last_sync":1792407600,"fail_count":0,"state":"ok","status":"failed"}}`},
		{"contradictory-id-alias", `{"data":{"id":"100-0","jobid":"999-0","last_sync":1792407600,"fail_count":0}}`},
		{"case-error", `{"data":{"last_sync":1792407600,"fail_count":0,"Error":"PRIVATE_BODY_MARKER"}}`},
		{"invalid-utf8", "{\"data\":" + replicationCleanStatus + ",\"metadata\":\"\xff\"}"},
		{"null", `{"data":null}`},
		{"duplicate-array-match", `{"data":[{"id":"100-0","last_sync":1792407600,"fail_count":0},{"id":"100-0","last_sync":1792407600,"fail_count":0}]}`},
		{"foreign-job", `{"data":{"id":"999-0","last_sync":1792407600,"fail_count":0}}`},
		{"foreign-source", `{"data":{"source":"foreign","last_sync":1792407600,"fail_count":0}}`},
		{"trailing-document", `{"data":` + replicationCleanStatus + `}{"data":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := replicationResponseClient(t, func(r *http.Request) io.ReadCloser {
				body := replicationCleanInventory
				if strings.HasSuffix(r.URL.Path, "/status") {
					body = tc.body
				}
				return io.NopCloser(strings.NewReader(body))
			})
			jobs, err := c.GetReplicationStatus(context.Background())
			if err != nil || len(jobs) != 1 {
				t.Fatalf("status rejection hid readable job configuration: jobs=%+v error=%v", jobs, err)
			}
			if jobs[0].LastSyncStatus != "" || jobs[0].LastSyncTime != nil || jobs[0].Error != "" || jobs[0].FailCount != 0 {
				t.Errorf("invalid status merged into job: %+v", jobs[0])
			}
		})
	}
}

type replicationBrokenBody struct {
	io.Reader
	failRead bool
	closed   *int
}

func (b replicationBrokenBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF && b.failRead {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

func (b replicationBrokenBody) Close() error {
	(*b.closed)++
	if !b.failRead {
		return fmt.Errorf("synthetic close failure")
	}
	return nil
}

func TestReplicationReadDoesNotLogProviderBodies(t *testing.T) {
	var output bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&output)
	t.Cleanup(func() { log.Logger = previous })
	for _, body := range []string{
		`{"data":{"last_sync":1792407600,"fail_count":0,"private_metadata":"PRIVATE_BODY_MARKER"}}`,
		`{"data":{"last_sync":1792407600,"fail_count":0},"errors":"PRIVATE_BODY_MARKER"}`,
	} {
		c := replicationResponseClient(t, func(r *http.Request) io.ReadCloser {
			if strings.HasSuffix(r.URL.Path, "/status") {
				return io.NopCloser(strings.NewReader(body))
			}
			return io.NopCloser(strings.NewReader(replicationCleanInventory))
		})
		if _, err := c.GetReplicationStatus(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(output.String(), "PRIVATE_BODY_MARKER") {
		t.Error("successful or invalid provider response body escaped into debug logs")
	}
}

// Export the same client responses consumed by the monitor/HTTP/browser
// acceptance. No hand-built verdicts are substituted after this producer.
func TestReplicationReadExportRecoveryFrames(t *testing.T) {
	frames := make(map[string]json.RawMessage)
	for _, phase := range []string{"initial", "short-inventory", "ambiguous-status", "recovered", "empty"} {
		closed := 0
		c := replicationResponseClient(t, func(r *http.Request) io.ReadCloser {
			var body string
			if r.URL.Path == "/api2/json/cluster/replication" {
				body = `{"data":[{"id":"100-0","guest":100,"source":"node-a","target":"node-b","schedule":"*/5"},{"id":"101-0","guest":101,"source":"node-a","target":"node-c","schedule":"*/15"}]}`
				if phase == "short-inventory" {
					return replicationBrokenBody{strings.NewReader(`{"data":[]}`), true, &closed}
				}
				if phase == "empty" {
					body = `{"data":[]}`
				}
			} else if r.URL.Path == "/api2/json/nodes/node-a/replication/100-0/status" {
				body = `{"data":{"id":"100-0","last_sync":1792407600,"next_sync":1792407900,"fail_count":2,"error":"Synthetic source snapshot unavailable","duration":10}}`
				if phase == "ambiguous-status" {
					body = `{"data":{"id":"100-0","last_sync":1792407600,"fail_count":2,"fail_count":0}}`
				} else if phase == "recovered" {
					body = `{"data":{"id":"100-0","last_sync":1792408200,"next_sync":1792408500,"fail_count":0,"state":"ok","duration":8}}`
				}
			} else if r.URL.Path == "/api2/json/nodes/node-a/replication/101-0/status" {
				body = `{"data":[{"id":"101-0","last_sync":1792407600,"next_sync":1792408500,"fail_count":0,"duration":9}]}`
			} else {
				t.Errorf("unexpected endpoint: %s", r.URL.Path)
				body = `{}`
			}
			return io.NopCloser(strings.NewReader(body))
		})
		jobs, err := c.GetReplicationStatus(context.Background())
		if phase == "short-inventory" {
			if err == nil || jobs != nil || closed != 1 {
				t.Fatal("incomplete inventory supplied replacement jobs")
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if phase == "empty" {
				if len(jobs) != 0 {
					t.Fatal("complete empty inventory retained jobs")
				}
			} else {
				want := map[string]string{"initial": "error", "ambiguous-status": "", "recovered": "ok"}[phase]
				if len(jobs) != 2 || jobs[0].LastSyncStatus != want || jobs[1].LastSyncStatus != "ok" {
					t.Fatalf("wrong phase result %s: %+v", phase, jobs)
				}
			}
		}
		frame, marshalErr := json.Marshal(struct {
			Jobs        []ReplicationJob `json:"jobs"`
			Unavailable bool             `json:"unavailable"`
		}{jobs, err != nil})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		frames[phase] = frame
	}
	wire, err := json.Marshal(frames)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("REPLICATION_CLIENT_FRAMES=%s", wire)
}

func TestReplicationReadRequiresEOFAndCloseBeforePublication(t *testing.T) {
	for _, scope := range []string{"inventory", "status"} {
		for _, fault := range []string{"read", "close"} {
			t.Run(scope+"/"+fault, func(t *testing.T) {
				closed := 0
				c := replicationResponseClient(t, func(r *http.Request) io.ReadCloser {
					status := strings.HasSuffix(r.URL.Path, "/status")
					body := replicationCleanInventory
					if status {
						body = `{"data":` + replicationCleanStatus + `}`
					}
					if (scope == "status") == status {
						return replicationBrokenBody{strings.NewReader(body), fault == "read", &closed}
					}
					return io.NopCloser(strings.NewReader(body))
				})
				jobs, err := c.GetReplicationStatus(context.Background())
				if scope == "inventory" {
					if err == nil || jobs != nil {
						t.Errorf("unfinished inventory published: %+v %v", jobs, err)
					}
				} else if err != nil || len(jobs) != 1 || jobs[0].LastSyncStatus != "" {
					t.Errorf("unfinished status supplied verdict or hid config: %+v %v", jobs, err)
				}
				if closed != 1 {
					t.Errorf("body closed %d times, want once", closed)
				}
			})
		}
	}
}

func TestReplicationReadRetainsNativeAndLegacyShapes(t *testing.T) {
	for _, status := range []string{
		`{"data":` + replicationCleanStatus + `,"errors":null,"total":1}`,
		`{"data":[` + replicationCleanStatus + `]}`,
		`{"data":[{"id":"999-0","fail_count":1},{"jobid":"100-0","source":"node-a","last_sync":1792407600,"fail_count":"0"}]}`,
		`{"data":{"id":"100-0","jobid":"100-0","last_sync":1792407600,"fail_count":0,"fail-count":"0","metadata":{"new":true}}}`,
		`{"data":{"last_sync":1792407600,"fail_count":0,"state":"idle","status":"OK","last_sync_status":"Success","last-sync-status":"completed"}}`,
	} {
		c := replicationResponseClient(t, func(r *http.Request) io.ReadCloser {
			body := replicationCleanInventory
			if strings.HasSuffix(r.URL.Path, "/status") {
				body = status
			}
			return io.NopCloser(strings.NewReader(body))
		})
		jobs, err := c.GetReplicationStatus(context.Background())
		if err != nil || len(jobs) != 1 || jobs[0].LastSyncStatus != "ok" || jobs[0].LastSyncUnix != 1792407600 {
			t.Errorf("compatible shape lost: %+v %v", jobs, err)
		}
	}
	for _, inventory := range []string{`{"data":[],"errors":null,"total":0}`, `{"data":[{"jobid":"100-0"}]}`} {
		c := replicationResponseClient(t, func(*http.Request) io.ReadCloser { return io.NopCloser(strings.NewReader(inventory)) })
		jobs, err := c.GetReplicationStatus(context.Background())
		if err != nil || jobs == nil {
			t.Errorf("completed empty/legacy configuration rejected: %+v %v", jobs, err)
		}
	}
}
