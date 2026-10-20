package pbs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type jobResponseBody struct {
	io.Reader
	readError, closeError bool
	closed                bool
}

func (b *jobResponseBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF && b.readError {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}
func (b *jobResponseBody) Close() error {
	b.closed = true
	if b.closeError {
		return errors.New("fixture close failure")
	}
	return nil
}

// No listener or service is involved: request/auth construction and body-limit
// handling remain real; only the HTTP transport supplies synthetic responses.
func TestPBSJobReadersRequireCompleteResponse(t *testing.T) {
	const task = `{"worker-type":"backup","worker-id":"archive:vm/117","upid":"UPID:backup:1","status":"ERROR: fixture job failed"}`
	const config = `{"id":"sync-main","store":"archive","last-run-state":"ERROR: fixture job failed"}`
	for _, reader := range []string{"running", "history", "config", "garbage"} {
		t.Run(reader, func(t *testing.T) {
			good := `{"data":[` + task + `]}`
			if reader == "config" {
				good = `{"data":[` + config + `]}`
			}
			if reader == "garbage" {
				good = `{"data":` + config + `}`
			}
			cases := []struct {
				name, body                   string
				status                       int
				readError, closeError, valid bool
			}{
				{name: "complete", body: good, valid: true},
				{name: "whitespace", body: " \n" + good + "\t", valid: true},
				{name: "additive-metadata", body: strings.Replace(good, `{"data":`, `{"total":1,"errors":null,"data":`, 1), valid: true},
				{name: "missing-data", body: `{}`},
				{name: "null-data", body: `{"data":null}`},
				{name: "null-root", body: `null`},
				{name: "wrong-data-kind", body: `{"data":false}`},
				{name: "case-data", body: strings.Replace(good, `"data"`, `"Data"`, 1)},
				{name: "duplicate-data", body: strings.Replace(good, `{"data":`, `{"data":null,"data":`, 1)},
				{name: "escaped-duplicate-data", body: strings.Replace(good, `{"data":`, `{"d\u0061ta":null,"data":`, 1)},
				{name: "errors-with-data", body: strings.Replace(good, `{"data":`, `{"errors":{"private":"fixture-sensitive-detail"},"data":`, 1)},
				{name: "case-errors", body: strings.Replace(good, `{"data":`, `{"Errors":{"private":"fixture-sensitive-detail"},"data":`, 1)},
				{name: "trailing-object", body: good + ` {"private":"fixture-sensitive-detail"}`},
				{name: "trailing-invalid", body: good + ` fixture-sensitive-detail`},
				{name: "read-failure-after-json", body: good, readError: true},
				{name: "close-failure", body: good, closeError: true},
				{name: "partial-status", body: good, status: http.StatusPartialContent},
				{name: "invalid-utf8", body: strings.Replace(good, "fixture", string([]byte{0xff}), 1)},
				{name: "oversized-tail", body: good + strings.Repeat(" ", int(maxResponseBodyBytes))},
			}
			if reader != "garbage" {
				cases = append(cases, struct {
					name, body                   string
					status                       int
					readError, closeError, valid bool
				}{name: "explicit-empty", body: `{"data":[]}`, valid: true})
				for _, row := range []struct{ name, body string }{
					{"null-row", `null`}, {"empty-row", `{}`}, {"string-row", `"fixture-sensitive-detail"`},
					{"duplicate-identity", strings.Replace(config, `"id":`, `"id":"hidden","id":`, 1)},
				} {
					cases = append(cases, struct {
						name, body                   string
						status                       int
						readError, closeError, valid bool
					}{name: row.name, body: `{"data":[` + row.body + `]}`})
				}
			} else {
				cases = append(cases, struct {
					name, body                   string
					status                       int
					readError, closeError, valid bool
				}{name: "explicit-empty-object", body: `{"data":{}}`, valid: true})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					client, err := NewClient(ClientConfig{Host: "http://pbs-job-fixture.invalid", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
					if err != nil {
						t.Fatal(err)
					}
					bodies := []*jobResponseBody{}
					client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
						if r.Method != http.MethodGet {
							t.Errorf("unexpected mutation: %s", r.Method)
						}
						body := &jobResponseBody{Reader: strings.NewReader(tc.body), readError: tc.readError, closeError: tc.closeError}
						bodies = append(bodies, body)
						status := tc.status
						if status == 0 {
							status = http.StatusOK
						}
						return &http.Response{StatusCode: status, Body: body, Header: make(http.Header), ContentLength: -1}, nil
					})
					var count int
					switch reader {
					case "running":
						var rows []RunningDataTask
						rows, err = client.ListRunningDataTasks(context.Background())
						count = len(rows)
					case "history":
						var rows []JobHealthEvidence
						rows, err = client.listTaskHistoryPage(context.Background(), taskHistoryQuery{}, 0, 200)
						count = len(rows)
					case "config":
						var rows []map[string]interface{}
						rows, err = client.listJobConfigMaps(context.Background(), "/config/sync")
						count = len(rows)
					case "garbage":
						var row map[string]interface{}
						row, err = client.getJobConfigMap(context.Background(), "/admin/datastore/archive/gc")
						count = len(row)
					}
					if tc.valid && err != nil {
						t.Fatalf("complete observation rejected: %v", err)
					}
					if !tc.valid && (err == nil || count != 0) {
						t.Errorf("incomplete response admitted: count=%d err=%v", count, err)
					}
					if err != nil && strings.Contains(err.Error(), "fixture-sensitive") {
						t.Error("private response body leaked into diagnostics")
					}
					for _, body := range bodies {
						if !body.closed {
							t.Error("response body left open")
						}
					}
				})
			}
		})
	}
}

func TestPBSRejectedJobListingsRemainPartialEvidence(t *testing.T) {
	for _, family := range []string{"backup", "sync", "verify", "prune", "garbage"} {
		t.Run(family, func(t *testing.T) {
			client, err := NewClient(ClientConfig{Host: "http://pbs-job-fixture.invalid", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":null}`)), Header: make(http.Header), ContentLength: -1}, nil
			})
			options := JobHealthOptions{MonitorBackups: family == "backup", MonitorSyncJobs: family == "sync", MonitorVerifyJobs: family == "verify", MonitorPruneJobs: family == "prune", MonitorGarbageJobs: family == "garbage"}
			rows, err := client.GetJobHealthEvidence(context.Background(), []string{"archive"}, options)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				t.Fatal("unreadable enabled family vanished instead of partial evidence")
			}
			for _, row := range rows {
				if row.Family != family || row.Error == "" || row.Confidence != JobEvidenceConfidencePartialError || row.EvidenceScope != JobEvidenceScopePartialRead {
					t.Errorf("wrong partial evidence: %+v", row)
				}
			}
			wire, _ := json.Marshal(rows)
			if strings.Contains(string(wire), "partial-permission") {
				t.Error("invalid 200 diagnosed as permission refusal")
			}
		})
	}
}

// The real public reader must not choose a first alias and certify that the
// writer for another alias/subject is absent. Compatibility spellings and an
// unspecified worker scope remain supported without extra requests or retries.
func TestPBSRunningTaskIdentityAliases(t *testing.T) {
	for _, tc := range []struct {
		name, row string
		valid     bool
		worker    string
	}{
		{"canonical", `{"worker-type":"backup","worker-id":"archive:vm/117"}`, true, "archive:vm/117"},
		{"underscore", `{"worker_type":"backup","worker_id":"archive:vm/117"}`, true, "archive:vm/117"},
		{"legacy", `{"type":"backup","id":"archive:vm/117"}`, true, "archive:vm/117"},
		{"equal-aliases", `{"worker-type":" BACKUP ","worker_type":"backup","type":"backup","worker-id":" archive:vm/117 ","worker_id":"archive:vm/117","id":"archive:vm/117"}`, true, "archive:vm/117"},
		{"unscoped", `{"worker-type":"backup"}`, true, ""},
		{"empty-scope", `{"worker-type":"backup","worker-id":""}`, true, ""},
		{"type-shadow", `{"worker-type":"verificationjob","worker_type":"backup","worker-id":"archive:vm/117"}`, false, ""},
		{"legacy-type-shadow", `{"worker-type":"verificationjob","type":"backup","worker-id":"archive:vm/117"}`, false, ""},
		{"identity-shadow", `{"worker-type":"backup","worker-id":"archive:vm/118","worker_id":"archive:vm/117"}`, false, ""},
		{"legacy-identity-shadow", `{"worker-type":"backup","worker-id":"archive:vm/118","id":"archive:vm/117"}`, false, ""},
		{"case-distinct-identity", `{"worker-type":"backup","worker-id":"Archive:vm/117","worker_id":"archive:vm/117"}`, false, ""},
		{"empty-shadow", `{"worker-type":"backup","worker-id":"","worker_id":"archive:vm/117"}`, false, ""},
		{"null-type", `{"worker-type":null,"worker_type":"backup"}`, false, ""},
		{"numeric-type", `{"worker-type":17,"worker_type":"backup"}`, false, ""},
		{"null-identity", `{"worker-type":"backup","worker-id":null}`, false, ""},
		{"numeric-identity", `{"worker-type":"backup","worker-id":117}`, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(ClientConfig{Host: "http://pbs-writer-fixture.invalid", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			reads, closed := 0, 0
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				reads++
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PBSAPIToken=fixture@pbs!reader:fixture-only" {
					t.Error("method or credential changed")
				}
				body := `{"data":[]}`
				if r.URL.Query().Get("typefilter") == "backup" {
					body = `{"data":[` + tc.row + `],"extension":"private-writer-body"}`
				}
				return &http.Response{StatusCode: http.StatusOK, Body: &writerIdentityBody{Reader: strings.NewReader(body), closed: &closed}, Header: make(http.Header), ContentLength: -1}, nil
			})
			tasks, err := client.ListRunningDataTasks(context.Background())
			if tc.valid {
				if err != nil || len(tasks) != 1 || tasks[0].WorkerType != "backup" || tasks[0].WorkerID != tc.worker {
					t.Fatalf("valid aliases lost: tasks=%+v err=%v", tasks, err)
				}
				if reads != 2 {
					t.Errorf("valid read count=%d want=2", reads)
				}
			} else {
				if err == nil || len(tasks) != 0 {
					t.Errorf("ambiguous writer admitted: tasks=%+v err=%v", tasks, err)
				}
				if reads != 1 {
					t.Errorf("rejected read count=%d want=1", reads)
				}
			}
			if closed != reads {
				t.Error("response body not closed")
			}
			if err != nil && strings.Contains(err.Error(), "private-writer-body") {
				t.Error("provider extension leaked")
			}
		})
	}
}

type writerIdentityBody struct {
	io.Reader
	closed *int
}

func (b *writerIdentityBody) Close() error { *b.closed++; return nil }
