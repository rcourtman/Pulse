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
